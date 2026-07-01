package operation_setting

import (
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

type ProxyPoolResource struct {
	Name    string `json:"name,omitempty"`
	URL     string `json:"url"`
	Enabled bool   `json:"enabled"`
}

type ProxyPoolSetting struct {
	Enabled                    bool                `json:"enabled"`
	Proxies                    []ProxyPoolResource `json:"proxies"`
	HealthCheckURL             string              `json:"health_check_url"`
	HealthCheckIntervalSeconds int                 `json:"health_check_interval_seconds"`
	HealthCheckTimeoutSeconds  int                 `json:"health_check_timeout_seconds"`
	AssignmentCooldownSeconds  int                 `json:"assignment_cooldown_seconds"`
}

var proxyPoolSetting = ProxyPoolSetting{
	Enabled:                    false,
	Proxies:                    []ProxyPoolResource{},
	HealthCheckURL:             "https://api.openai.com",
	HealthCheckIntervalSeconds: 300,
	HealthCheckTimeoutSeconds:  10,
	AssignmentCooldownSeconds:  60,
}

func init() {
	config.GlobalConfig.Register("proxy_pool_setting", &proxyPoolSetting)
}

func GetProxyPoolSetting() *ProxyPoolSetting {
	NormalizeProxyPoolSetting(&proxyPoolSetting)
	return &proxyPoolSetting
}

func NormalizeProxyPoolSetting(setting *ProxyPoolSetting) {
	if setting == nil {
		return
	}
	if strings.TrimSpace(setting.HealthCheckURL) == "" {
		setting.HealthCheckURL = "https://api.openai.com"
	}
	if setting.HealthCheckIntervalSeconds <= 0 {
		setting.HealthCheckIntervalSeconds = 300
	}
	if setting.HealthCheckTimeoutSeconds <= 0 {
		setting.HealthCheckTimeoutSeconds = 10
	}
	if setting.AssignmentCooldownSeconds < 0 {
		setting.AssignmentCooldownSeconds = 0
	}
	seen := make(map[string]struct{}, len(setting.Proxies))
	normalized := make([]ProxyPoolResource, 0, len(setting.Proxies))
	for _, proxyResource := range setting.Proxies {
		proxyResource.URL = strings.TrimSpace(proxyResource.URL)
		proxyResource.Name = strings.TrimSpace(proxyResource.Name)
		if proxyResource.URL == "" {
			continue
		}
		if _, exists := seen[proxyResource.URL]; exists {
			continue
		}
		seen[proxyResource.URL] = struct{}{}
		normalized = append(normalized, proxyResource)
	}
	setting.Proxies = normalized
}

func ValidateProxyPoolResources(resources []ProxyPoolResource) error {
	for _, proxyResource := range resources {
		proxyURL := strings.TrimSpace(proxyResource.URL)
		if proxyURL == "" {
			continue
		}
		parsedURL, err := url.Parse(proxyURL)
		if err != nil {
			return err
		}
		switch strings.ToLower(parsedURL.Scheme) {
		case "http", "https", "socks5", "socks5h":
		default:
			return &url.Error{Op: "parse", URL: proxyURL, Err: errUnsupportedProxyScheme}
		}
		if parsedURL.Host == "" {
			return &url.Error{Op: "parse", URL: proxyURL, Err: errProxyHostRequired}
		}
	}
	return nil
}

type proxyPoolValidationError string

func (e proxyPoolValidationError) Error() string {
	return string(e)
}

const (
	errUnsupportedProxyScheme proxyPoolValidationError = "unsupported proxy scheme, must be http, https, socks5 or socks5h"
	errProxyHostRequired      proxyPoolValidationError = "proxy host is required"
)
