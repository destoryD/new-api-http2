package dto

import "strings"

const (
	ChannelEndpointTypeMessages            = "messages"
	ChannelEndpointTypeChatCompletions     = "chat/completions"
	ChannelEndpointTypeCompletions         = "completions"
	ChannelEndpointTypeResponses           = "responses"
	ChannelEndpointTypeResponsesCompact    = "responses/compact"
	ChannelEndpointTypeEmbeddings          = "embeddings"
	ChannelEndpointTypeRerank              = "rerank"
	ChannelEndpointTypeImagesGenerations   = "images/generations"
	ChannelEndpointTypeImagesEdits         = "images/edits"
	ChannelEndpointTypeAudioSpeech         = "audio/speech"
	ChannelEndpointTypeAudioTranscriptions = "audio/transcriptions"
	ChannelEndpointTypeAudioTranslations   = "audio/translations"
	ChannelEndpointTypeModerations         = "moderations"
	ChannelEndpointTypeRealtime            = "realtime"
	ChannelEndpointTypeGemini              = "gemini"
)

func NormalizeChannelEndpointType(endpointType string) string {
	endpointType = strings.TrimSpace(strings.ToLower(endpointType))
	endpointType = strings.TrimPrefix(endpointType, "/")
	endpointType = strings.TrimPrefix(endpointType, "v1/")
	endpointType = strings.TrimPrefix(endpointType, "pg/")
	endpointType = strings.TrimPrefix(endpointType, "v1beta/")
	endpointType = strings.NewReplacer("_", "/", "-", "/").Replace(endpointType)

	switch endpointType {
	case "message", "anthropic", ChannelEndpointTypeMessages:
		return ChannelEndpointTypeMessages
	case "openai", "chat/completion", ChannelEndpointTypeChatCompletions:
		return ChannelEndpointTypeChatCompletions
	case "completion", ChannelEndpointTypeCompletions:
		return ChannelEndpointTypeCompletions
	case "openai/response", "response", ChannelEndpointTypeResponses:
		return ChannelEndpointTypeResponses
	case "openai/response/compact", "response/compact", ChannelEndpointTypeResponsesCompact:
		return ChannelEndpointTypeResponsesCompact
	case "embedding", ChannelEndpointTypeEmbeddings:
		return ChannelEndpointTypeEmbeddings
	case "jina/rerank", ChannelEndpointTypeRerank:
		return ChannelEndpointTypeRerank
	case "image/generation", ChannelEndpointTypeImagesGenerations:
		return ChannelEndpointTypeImagesGenerations
	case "image/edit", ChannelEndpointTypeImagesEdits:
		return ChannelEndpointTypeImagesEdits
	case ChannelEndpointTypeAudioSpeech:
		return ChannelEndpointTypeAudioSpeech
	case "audio/transcription", ChannelEndpointTypeAudioTranscriptions:
		return ChannelEndpointTypeAudioTranscriptions
	case "audio/translation", ChannelEndpointTypeAudioTranslations:
		return ChannelEndpointTypeAudioTranslations
	case "moderation", ChannelEndpointTypeModerations:
		return ChannelEndpointTypeModerations
	case ChannelEndpointTypeRealtime:
		return ChannelEndpointTypeRealtime
	case "models", "model", ChannelEndpointTypeGemini:
		return ChannelEndpointTypeGemini
	default:
		return endpointType
	}
}

func NormalizeChannelEndpointTypes(endpointTypes []string) []string {
	if len(endpointTypes) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(endpointTypes))
	normalized := make([]string, 0, len(endpointTypes))
	for _, endpointType := range endpointTypes {
		endpointType = NormalizeChannelEndpointType(endpointType)
		if endpointType == "" {
			continue
		}
		if _, ok := seen[endpointType]; ok {
			continue
		}
		seen[endpointType] = struct{}{}
		normalized = append(normalized, endpointType)
	}
	return normalized
}

func IsChannelEndpointAllowed(allowedEndpointTypes []string, endpointType string) bool {
	allowedEndpointTypes = NormalizeChannelEndpointTypes(allowedEndpointTypes)
	if len(allowedEndpointTypes) == 0 {
		return true
	}
	endpointType = NormalizeChannelEndpointType(endpointType)
	if endpointType == "" {
		return false
	}
	for _, allowed := range allowedEndpointTypes {
		if allowed == endpointType {
			return true
		}
	}
	return false
}

func ChannelEndpointTypeFromPath(path string) string {
	path = strings.TrimSpace(strings.ToLower(path))
	switch {
	case strings.HasPrefix(path, "/v1/messages"):
		return ChannelEndpointTypeMessages
	case strings.HasPrefix(path, "/v1/chat/completions"), strings.HasPrefix(path, "/pg/chat/completions"):
		return ChannelEndpointTypeChatCompletions
	case strings.HasPrefix(path, "/v1/completions"):
		return ChannelEndpointTypeCompletions
	case strings.HasPrefix(path, "/v1/responses/compact"):
		return ChannelEndpointTypeResponsesCompact
	case strings.HasPrefix(path, "/v1/responses"):
		return ChannelEndpointTypeResponses
	case strings.HasPrefix(path, "/v1/embeddings"), strings.HasSuffix(path, "/embeddings"):
		return ChannelEndpointTypeEmbeddings
	case strings.HasPrefix(path, "/v1/rerank"):
		return ChannelEndpointTypeRerank
	case strings.HasPrefix(path, "/v1/images/generations"):
		return ChannelEndpointTypeImagesGenerations
	case strings.HasPrefix(path, "/v1/images/edits"):
		return ChannelEndpointTypeImagesEdits
	case strings.HasPrefix(path, "/v1/audio/speech"):
		return ChannelEndpointTypeAudioSpeech
	case strings.HasPrefix(path, "/v1/audio/transcriptions"):
		return ChannelEndpointTypeAudioTranscriptions
	case strings.HasPrefix(path, "/v1/audio/translations"):
		return ChannelEndpointTypeAudioTranslations
	case strings.HasPrefix(path, "/v1/moderations"):
		return ChannelEndpointTypeModerations
	case strings.HasPrefix(path, "/v1/realtime"):
		return ChannelEndpointTypeRealtime
	case strings.HasPrefix(path, "/v1beta/models"), strings.HasPrefix(path, "/v1/models"):
		return ChannelEndpointTypeGemini
	default:
		return NormalizeChannelEndpointType(path)
	}
}
