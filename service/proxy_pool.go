package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

type proxyAssignment struct {
	ProxyURL   string
	AssignedAt time.Time
}

type proxyHealthState struct {
	Available   bool
	Checked     bool
	LastChecked time.Time
	LastError   string
}

type ProxyPoolRuntimeStatus struct {
	Enabled                    bool                     `json:"enabled"`
	HealthCheckURL             string                   `json:"health_check_url"`
	HealthCheckIntervalSeconds int                      `json:"health_check_interval_seconds"`
	AssignmentCooldownSeconds  int                      `json:"assignment_cooldown_seconds"`
	Total                      int                      `json:"total"`
	Usable                     int                      `json:"usable"`
	Resources                  []ProxyPoolResourceState `json:"resources"`
}

type ProxyPoolResourceState struct {
	Name                     string `json:"name"`
	URL                      string `json:"url"`
	Enabled                  bool   `json:"enabled"`
	Available                bool   `json:"available"`
	Checked                  bool   `json:"checked"`
	LastCheckedAt            int64  `json:"last_checked_at"`
	LastError                string `json:"last_error"`
	LastAssignedAt           int64  `json:"last_assigned_at"`
	CooldownUntil            int64  `json:"cooldown_until"`
	CooldownRemainingSeconds int64  `json:"cooldown_remaining_seconds"`
	AssignmentCount          int    `json:"assignment_count"`
}

type globalProxyPoolManager struct {
	sync.Mutex
	assignments  map[string]proxyAssignment
	health       map[string]proxyHealthState
	lastAssigned map[string]time.Time
	nextIndex    int
	monitorOnce  sync.Once
}

var globalProxyPool = &globalProxyPoolManager{
	assignments:  make(map[string]proxyAssignment),
	health:       make(map[string]proxyHealthState),
	lastAssigned: make(map[string]time.Time),
}

func StartGlobalProxyPoolMonitor() {
	globalProxyPool.monitorOnce.Do(func() {
		go globalProxyPool.monitorLoop()
	})
}

func ResetGlobalProxyPoolRuntime() {
	globalProxyPool.Lock()
	defer globalProxyPool.Unlock()
	globalProxyPool.assignments = make(map[string]proxyAssignment)
	globalProxyPool.health = make(map[string]proxyHealthState)
	globalProxyPool.lastAssigned = make(map[string]time.Time)
	globalProxyPool.nextIndex = 0
}

func GetGlobalProxyPoolRuntimeStatus() ProxyPoolRuntimeStatus {
	setting := operation_setting.GetProxyPoolSetting()
	operation_setting.NormalizeProxyPoolSetting(setting)
	now := time.Now()
	cooldown := time.Duration(setting.AssignmentCooldownSeconds) * time.Second

	globalProxyPool.Lock()
	defer globalProxyPool.Unlock()
	globalProxyPool.pruneRuntimeStateLocked(setting.Proxies)

	assignmentCounts := make(map[string]int)
	for _, assignment := range globalProxyPool.assignments {
		assignmentCounts[assignment.ProxyURL]++
	}

	status := ProxyPoolRuntimeStatus{
		Enabled:                    setting.Enabled,
		HealthCheckURL:             setting.HealthCheckURL,
		HealthCheckIntervalSeconds: setting.HealthCheckIntervalSeconds,
		AssignmentCooldownSeconds:  setting.AssignmentCooldownSeconds,
		Resources:                  make([]ProxyPoolResourceState, 0, len(setting.Proxies)),
	}

	for _, resource := range setting.Proxies {
		proxyURL := strings.TrimSpace(resource.URL)
		if proxyURL == "" {
			continue
		}
		health := globalProxyPool.health[proxyURL]
		state := ProxyPoolResourceState{
			Name:            resource.Name,
			URL:             proxyURL,
			Enabled:         resource.Enabled,
			Available:       resource.Enabled && (!health.Checked || health.Available),
			Checked:         health.Checked,
			LastError:       health.LastError,
			AssignmentCount: assignmentCounts[proxyURL],
		}
		if !health.LastChecked.IsZero() {
			state.LastCheckedAt = health.LastChecked.Unix()
		}
		if lastAssigned, ok := globalProxyPool.lastAssigned[proxyURL]; ok && !lastAssigned.IsZero() {
			state.LastAssignedAt = lastAssigned.Unix()
			if cooldown > 0 {
				cooldownUntil := lastAssigned.Add(cooldown)
				if cooldownUntil.After(now) {
					state.CooldownUntil = cooldownUntil.Unix()
					state.CooldownRemainingSeconds = int64(time.Until(cooldownUntil).Seconds())
					if state.CooldownRemainingSeconds < 1 {
						state.CooldownRemainingSeconds = 1
					}
				}
			}
		}
		status.Total++
		if state.Enabled && state.Available && state.CooldownRemainingSeconds == 0 {
			status.Usable++
		}
		status.Resources = append(status.Resources, state)
	}

	return status
}

func ApplyGlobalProxyPoolToChannelSetting(setting *dto.ChannelSettings, channelID int, keyIndex int, apiKey string) error {
	if setting == nil || !setting.UseGlobalProxyPool {
		return nil
	}
	proxyURL, err := globalProxyPool.assign(channelID, keyIndex, apiKey)
	if err != nil {
		return err
	}
	setting.Proxy = proxyURL
	return nil
}

func ReportGlobalProxyPoolFailure(setting dto.ChannelSettings, channelID int, keyIndex int, apiKey string, reason string) (dto.ChannelSettings, bool, error) {
	if !setting.UseGlobalProxyPool || strings.TrimSpace(setting.Proxy) == "" {
		return setting, false, nil
	}
	globalProxyPool.markUnavailable(setting.Proxy, reason)
	return SwitchGlobalProxyPoolProxy(setting, channelID, keyIndex, apiKey)
}

func SwitchGlobalProxyPoolProxy(setting dto.ChannelSettings, channelID int, keyIndex int, apiKey string) (dto.ChannelSettings, bool, error) {
	if !setting.UseGlobalProxyPool || strings.TrimSpace(setting.Proxy) == "" {
		return setting, false, nil
	}
	proxyURL, err := globalProxyPool.reassign(channelID, keyIndex, apiKey, setting.Proxy)
	if err != nil {
		return setting, false, err
	}
	if proxyURL == "" || proxyURL == setting.Proxy {
		return setting, false, nil
	}
	setting.Proxy = proxyURL
	return setting, true, nil
}

func (m *globalProxyPoolManager) monitorLoop() {
	for {
		setting := operation_setting.GetProxyPoolSetting()
		interval := time.Duration(setting.HealthCheckIntervalSeconds) * time.Second
		if interval <= 0 {
			interval = 5 * time.Minute
		}
		if setting.Enabled {
			m.runHealthChecks(*setting)
		}
		time.Sleep(interval)
	}
}

func (m *globalProxyPoolManager) runHealthChecks(setting operation_setting.ProxyPoolSetting) {
	operation_setting.NormalizeProxyPoolSetting(&setting)
	if len(setting.Proxies) == 0 {
		return
	}
	for _, proxyResource := range setting.Proxies {
		if !proxyResource.Enabled || strings.TrimSpace(proxyResource.URL) == "" {
			continue
		}
		available, errText := checkProxyResource(proxyResource.URL, setting.HealthCheckURL, setting.HealthCheckTimeoutSeconds)
		m.Lock()
		m.health[proxyResource.URL] = proxyHealthState{
			Available:   available,
			Checked:     true,
			LastChecked: time.Now(),
			LastError:   errText,
		}
		m.Unlock()
	}
	m.pruneRuntimeState(setting.Proxies)
}

func checkProxyResource(proxyURL string, targetURL string, timeoutSeconds int) (bool, string) {
	client, err := GetHttpClientWithProxy(proxyURL)
	if err != nil {
		return false, err.Error()
	}
	if timeoutSeconds <= 0 {
		timeoutSeconds = 10
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, targetURL, nil)
	if err != nil {
		return false, err.Error()
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, err.Error()
	}
	if resp.Body != nil {
		_ = resp.Body.Close()
	}
	if resp.StatusCode == http.StatusMethodNotAllowed {
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
		if err != nil {
			return false, err.Error()
		}
		resp, err = client.Do(req)
		if err != nil {
			return false, err.Error()
		}
		if resp.Body != nil {
			_ = resp.Body.Close()
		}
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 500 {
		return true, ""
	}
	return false, fmt.Sprintf("health check returned status %d", resp.StatusCode)
}

func (m *globalProxyPoolManager) assign(channelID int, keyIndex int, apiKey string) (string, error) {
	setting := operation_setting.GetProxyPoolSetting()
	if !setting.Enabled {
		return "", errors.New("global proxy pool is disabled")
	}
	operation_setting.NormalizeProxyPoolSetting(setting)
	if len(setting.Proxies) == 0 {
		return "", errors.New("global proxy pool has no proxy resources")
	}
	assignmentKey := buildProxyAssignmentKey(channelID, keyIndex, apiKey)
	m.Lock()
	defer m.Unlock()
	m.pruneRuntimeStateLocked(setting.Proxies)
	if assignment, ok := m.assignments[assignmentKey]; ok && m.isProxyUsableLocked(setting.Proxies, assignment.ProxyURL) {
		return assignment.ProxyURL, nil
	}
	proxyURL := m.pickProxyLocked(setting.Proxies, time.Duration(setting.AssignmentCooldownSeconds)*time.Second, "")
	if proxyURL == "" {
		return "", errors.New("global proxy pool has no usable proxy resources")
	}
	m.assignments[assignmentKey] = proxyAssignment{ProxyURL: proxyURL, AssignedAt: time.Now()}
	m.lastAssigned[proxyURL] = time.Now()
	return proxyURL, nil
}

func (m *globalProxyPoolManager) reassign(channelID int, keyIndex int, apiKey string, failedProxy string) (string, error) {
	setting := operation_setting.GetProxyPoolSetting()
	if !setting.Enabled {
		return "", errors.New("global proxy pool is disabled")
	}
	operation_setting.NormalizeProxyPoolSetting(setting)
	assignmentKey := buildProxyAssignmentKey(channelID, keyIndex, apiKey)
	m.Lock()
	defer m.Unlock()
	m.pruneRuntimeStateLocked(setting.Proxies)
	proxyURL := m.pickProxyLocked(setting.Proxies, time.Duration(setting.AssignmentCooldownSeconds)*time.Second, failedProxy)
	if proxyURL == "" {
		delete(m.assignments, assignmentKey)
		return "", errors.New("global proxy pool has no replacement proxy resources")
	}
	m.assignments[assignmentKey] = proxyAssignment{ProxyURL: proxyURL, AssignedAt: time.Now()}
	m.lastAssigned[proxyURL] = time.Now()
	return proxyURL, nil
}

func (m *globalProxyPoolManager) markUnavailable(proxyURL string, reason string) {
	proxyURL = strings.TrimSpace(proxyURL)
	if proxyURL == "" {
		return
	}
	m.Lock()
	defer m.Unlock()
	m.health[proxyURL] = proxyHealthState{
		Available:   false,
		Checked:     true,
		LastChecked: time.Now(),
		LastError:   strings.TrimSpace(reason),
	}
}

func (m *globalProxyPoolManager) pickProxyLocked(resources []operation_setting.ProxyPoolResource, cooldown time.Duration, exclude string) string {
	if len(resources) == 0 {
		return ""
	}
	now := time.Now()
	var oldestProxy string
	var oldestAssigned time.Time
	for _, resource := range resources {
		proxyURL := strings.TrimSpace(resource.URL)
		if proxyURL == "" || proxyURL == exclude || !resource.Enabled {
			continue
		}
		if !m.isProxyHealthyLocked(proxyURL) {
			continue
		}
		lastAssigned, assignedBefore := m.lastAssigned[proxyURL]
		if !assignedBefore {
			return proxyURL
		}
		if cooldown > 0 && now.Sub(lastAssigned) < cooldown {
			continue
		}
		if oldestProxy == "" || lastAssigned.Before(oldestAssigned) {
			oldestProxy = proxyURL
			oldestAssigned = lastAssigned
		}
	}
	return oldestProxy
}

func (m *globalProxyPoolManager) isProxyUsableLocked(resources []operation_setting.ProxyPoolResource, proxyURL string) bool {
	for _, resource := range resources {
		if resource.Enabled && strings.TrimSpace(resource.URL) == proxyURL {
			return m.isProxyHealthyLocked(proxyURL)
		}
	}
	return false
}

func (m *globalProxyPoolManager) isProxyHealthyLocked(proxyURL string) bool {
	health, ok := m.health[proxyURL]
	if !ok || !health.Checked {
		return true
	}
	return health.Available
}

func (m *globalProxyPoolManager) pruneRuntimeState(resources []operation_setting.ProxyPoolResource) {
	m.Lock()
	defer m.Unlock()
	m.pruneRuntimeStateLocked(resources)
}

func (m *globalProxyPoolManager) pruneRuntimeStateLocked(resources []operation_setting.ProxyPoolResource) {
	active := make(map[string]struct{}, len(resources))
	for _, resource := range resources {
		proxyURL := strings.TrimSpace(resource.URL)
		if proxyURL != "" && resource.Enabled {
			active[proxyURL] = struct{}{}
		}
	}
	for key, assignment := range m.assignments {
		if _, ok := active[assignment.ProxyURL]; !ok {
			delete(m.assignments, key)
		}
	}
	for proxyURL := range m.health {
		if _, ok := active[proxyURL]; !ok {
			delete(m.health, proxyURL)
		}
	}
	for proxyURL := range m.lastAssigned {
		if _, ok := active[proxyURL]; !ok {
			delete(m.lastAssigned, proxyURL)
		}
	}
}

func buildProxyAssignmentKey(channelID int, keyIndex int, apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey))
	return fmt.Sprintf("%d:%d:%s", channelID, keyIndex, hex.EncodeToString(hash[:8]))
}

func logGlobalProxyPoolFailure(err error) {
	if err != nil {
		common.SysLog("global proxy pool: " + err.Error())
	}
}
