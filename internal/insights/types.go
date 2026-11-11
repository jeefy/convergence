package insights

import (
	"encoding/json"
	"time"
)

// RequestRecord holds one logged request/response transaction from agentgateway.
type RequestRecord struct {
	ID                  string
	StartedAt           time.Time
	CompletedAt         time.Time
	DurationMS          int64
	HTTPStatus          int
	Error               *string
	GenAIOperationName  string
	GenAIProviderName   string
	GenAIRequestModel   string
	GenAIResponseModel  string
	InputTokens         int64
	OutputTokens        int64
	TotalTokens         int64
	Cost                float64
	Priced              bool // cost column was non-NULL
	AgentgatewayUser    string
	AgentgatewayGroup   string
	UserAgentName       string
	HasPayload          bool
	Attributes          map[string]any
	PinballTier         string
	PinballModel        string
	PinballReason       string
	RequestPrompt       any
	ResponseCompletion  any
	UserPromptPreview   string
	AssistantCompletion string
}

// ExtractPromptText attempts to find the primary user prompt text from prompt JSON.
func ExtractPromptText(v any) string {
	if v == nil {
		return ""
	}
	// Case 1: Array of messages (e.g. [{"role": "user", "content": "..."}, ...])
	if msgs, ok := v.([]any); ok {
		for i := len(msgs) - 1; i >= 0; i-- {
			if m, ok := msgs[i].(map[string]any); ok {
				if role, _ := m["role"].(string); role == "user" {
					return extractContentString(m["content"])
				}
			}
		}
	}
	// Case 2: Object with messages field or prompt field
	if m, ok := v.(map[string]any); ok {
		if msgs, ok := m["messages"].([]any); ok {
			for i := len(msgs) - 1; i >= 0; i-- {
				if msg, ok := msgs[i].(map[string]any); ok {
					if role, _ := msg["role"].(string); role == "user" {
						return extractContentString(msg["content"])
					}
				}
			}
		}
		if p, ok := m["prompt"].(string); ok {
			return p
		}
	}
	// Fallback to json string if short
	b, err := json.Marshal(v)
	if err == nil && len(b) < 1000 {
		return string(b)
	}
	return ""
}

func extractContentString(content any) string {
	if s, ok := content.(string); ok {
		return s
	}
	if parts, ok := content.([]any); ok {
		var full string
		for _, part := range parts {
			if pm, ok := part.(map[string]any); ok {
				if text, ok := pm["text"].(string); ok {
					if full != "" {
						full += "\n"
					}
					full += text
				}
			}
		}
		return full
	}
	return ""
}

// ExtractCompletionText attempts to find the primary assistant completion text.
func ExtractCompletionText(v any) string {
	if v == nil {
		return ""
	}
	// Case 1: Standard OpenAI completion response structure
	if m, ok := v.(map[string]any); ok {
		if choices, ok := m["choices"].([]any); ok && len(choices) > 0 {
			if c, ok := choices[0].(map[string]any); ok {
				if msg, ok := c["message"].(map[string]any); ok {
					return extractContentString(msg["content"])
				}
				if text, ok := c["text"].(string); ok {
					return text
				}
			}
		}
	}
	// Case 2: Array or string directly
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
