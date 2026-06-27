package dto

import "strings"

type ChannelSettings struct {
	ForceFormat               bool           `json:"force_format,omitempty"`
	ThinkingToContent         bool           `json:"thinking_to_content,omitempty"`
	Proxy                     string         `json:"proxy"`
	ProxyPool                 []string       `json:"proxy_pool,omitempty"`
	ProxyPoolRetryStatusCodes []int          `json:"proxy_pool_retry_status_codes,omitempty"`
	PassThroughBodyEnabled    bool           `json:"pass_through_body_enabled,omitempty"`
	SystemPrompt              string         `json:"system_prompt,omitempty"`
	SystemPromptOverride      bool           `json:"system_prompt_override,omitempty"`
	EnableHttp2               bool           `json:"enable_http2,omitempty"`
	DisableHttp2              bool           `json:"disable_http2,omitempty"`
	DisableConnectionReuse    bool           `json:"disable_connection_reuse,omitempty"`
	ModelNameOverride         bool           `json:"model_name_override,omitempty"`
	NonStreamToStream         bool           `json:"non_stream_to_stream,omitempty"`
	AllowedEndpointTypes      []string       `json:"allowed_endpoint_types,omitempty"`
	RPMLimit                  int            `json:"rpm_limit,omitempty"`
	ModelRPMLimits            map[string]int `json:"model_rpm_limits,omitempty"`
	MultiKeyRPMLimit          int            `json:"multi_key_rpm_limit,omitempty"`
	MultiKey429SkipSeconds    int            `json:"multi_key_429_skip_seconds,omitempty"`
	OverrideErrorAs429        bool           `json:"override_error_as_429,omitempty"`
}

func (s ChannelSettings) GetMultiKey429SkipSeconds() int {
	if s.MultiKey429SkipSeconds <= 0 {
		return 60
	}
	return s.MultiKey429SkipSeconds
}

func NormalizeProxyPool(proxyPool []string) []string {
	if len(proxyPool) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(proxyPool))
	for _, proxyURL := range proxyPool {
		proxyURL = strings.TrimSpace(proxyURL)
		if proxyURL != "" {
			normalized = append(normalized, proxyURL)
		}
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func (s ChannelSettings) WithProxyPoolIndex(index int) ChannelSettings {
	proxyPool := NormalizeProxyPool(s.ProxyPool)
	if len(proxyPool) == 0 || index < 0 {
		return s
	}
	s.Proxy = proxyPool[index%len(proxyPool)]
	return s
}

func (s ChannelSettings) ShouldRetryWithProxyPoolStatusCode(statusCode int) bool {
	if statusCode < 100 || statusCode > 599 || len(s.ProxyPoolRetryStatusCodes) == 0 {
		return false
	}
	for _, code := range s.ProxyPoolRetryStatusCodes {
		if code == statusCode {
			return true
		}
	}
	return false
}

type VertexKeyType string

const (
	VertexKeyTypeJSON   VertexKeyType = "json"
	VertexKeyTypeAPIKey VertexKeyType = "api_key"
)

type AwsKeyType string

const (
	AwsKeyTypeAKSK   AwsKeyType = "ak_sk" // 默认
	AwsKeyTypeApiKey AwsKeyType = "api_key"
)

type ChannelOtherSettings struct {
	AzureResponsesVersion                 string        `json:"azure_responses_version,omitempty"`
	VertexKeyType                         VertexKeyType `json:"vertex_key_type,omitempty"` // "json" or "api_key"
	OpenRouterEnterprise                  *bool         `json:"openrouter_enterprise,omitempty"`
	NativeMessagesEnabled                 bool          `json:"native_messages_enabled,omitempty"`   // OpenAI-compatible channels: forward /v1/messages to upstream native Messages API
	ClaudeBetaQuery                       bool          `json:"claude_beta_query,omitempty"`         // Claude 渠道是否强制追加 ?beta=true
	AllowServiceTier                      bool          `json:"allow_service_tier,omitempty"`        // 是否允许 service_tier 透传（默认过滤以避免额外计费）
	AllowInferenceGeo                     bool          `json:"allow_inference_geo,omitempty"`       // 是否允许 inference_geo 透传（仅 Claude，默认过滤以满足数据驻留合规
	AllowSpeed                            bool          `json:"allow_speed,omitempty"`               // 是否允许 speed 透传（仅 Claude，默认过滤以避免意外切换推理速度模式）
	AllowSafetyIdentifier                 bool          `json:"allow_safety_identifier,omitempty"`   // 是否允许 safety_identifier 透传（默认过滤以保护用户隐私）
	DisableStore                          bool          `json:"disable_store,omitempty"`             // 是否禁用 store 透传（默认允许透传，禁用后可能导致 Codex 无法使用）
	AllowIncludeObfuscation               bool          `json:"allow_include_obfuscation,omitempty"` // 是否允许 stream_options.include_obfuscation 透传（默认过滤以避免关闭流混淆保护）
	AwsKeyType                            AwsKeyType    `json:"aws_key_type,omitempty"`
	UpstreamModelUpdateCheckEnabled       bool          `json:"upstream_model_update_check_enabled,omitempty"`        // 是否检测上游模型更新
	UpstreamModelUpdateAutoSyncEnabled    bool          `json:"upstream_model_update_auto_sync_enabled,omitempty"`    // 是否自动同步上游模型更新
	UpstreamModelUpdateLastCheckTime      int64         `json:"upstream_model_update_last_check_time,omitempty"`      // 上次检测时间
	UpstreamModelUpdateLastDetectedModels []string      `json:"upstream_model_update_last_detected_models,omitempty"` // 上次检测到的可加入模型
	UpstreamModelUpdateLastRemovedModels  []string      `json:"upstream_model_update_last_removed_models,omitempty"`  // 上次检测到的可删除模型
	UpstreamModelUpdateIgnoredModels      []string      `json:"upstream_model_update_ignored_models,omitempty"`       // 手动忽略的模型
}

func (s *ChannelOtherSettings) IsOpenRouterEnterprise() bool {
	if s == nil || s.OpenRouterEnterprise == nil {
		return false
	}
	return *s.OpenRouterEnterprise
}
