package insights

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// FetchRequests connects to the postgres request-log database and reads logs completed since `since`.
func FetchRequests(ctx context.Context, dsn string, since time.Duration) ([]RequestRecord, error) {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connecting to request log database: %w", err)
	}
	defer func() {
		_ = conn.Close(ctx)
	}()

	cutoff := time.Now().Add(-since)

	query := `
SELECT 
	l.id, l.started_at, l.completed_at, l.duration_ms, l.http_status, l.error,
	COALESCE(l.gen_ai_operation_name, ''),
	COALESCE(l.gen_ai_provider_name, ''),
	COALESCE(l.gen_ai_request_model, ''),
	COALESCE(l.gen_ai_response_model, ''),
	COALESCE(l.input_tokens, 0),
	COALESCE(l.output_tokens, 0),
	COALESCE(l.total_tokens, 0),
	COALESCE(l.cost, 0.0),
	COALESCE(l.agentgateway_user, ''),
	COALESCE(l.agentgateway_group, ''),
	COALESCE(l.user_agent_name, ''),
	l.has_payload,
	l.attributes_json,
	p.request_prompt_json,
	p.response_completion_json
FROM request_logs l
LEFT JOIN request_log_payloads p ON l.id = p.log_id
WHERE l.completed_at >= $1
ORDER BY l.completed_at ASC
`

	rows, err := conn.Query(ctx, query, cutoff)
	if err != nil {
		return nil, fmt.Errorf("querying request_logs: %w", err)
	}
	defer rows.Close()

	var records []RequestRecord
	for rows.Next() {
		var r RequestRecord
		var attrBytes []byte
		var promptBytes []byte
		var compBytes []byte

		err := rows.Scan(
			&r.ID,
			&r.StartedAt,
			&r.CompletedAt,
			&r.DurationMS,
			&r.HTTPStatus,
			&r.Error,
			&r.GenAIOperationName,
			&r.GenAIProviderName,
			&r.GenAIRequestModel,
			&r.GenAIResponseModel,
			&r.InputTokens,
			&r.OutputTokens,
			&r.TotalTokens,
			&r.Cost,
			&r.AgentgatewayUser,
			&r.AgentgatewayGroup,
			&r.UserAgentName,
			&r.HasPayload,
			&attrBytes,
			&promptBytes,
			&compBytes,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning request_log row: %w", err)
		}

		if len(attrBytes) > 0 {
			var attrs map[string]any
			if err := json.Unmarshal(attrBytes, &attrs); err == nil {
				r.Attributes = attrs
				if tier, ok := attrs["pinball.tier"].(string); ok {
					r.PinballTier = tier
				}
				if model, ok := attrs["pinball.model"].(string); ok {
					r.PinballModel = model
				}
				if reason, ok := attrs["pinball.reason"].(string); ok {
					r.PinballReason = reason
				}
			}
		}

		if len(promptBytes) > 0 {
			var v any
			if err := json.Unmarshal(promptBytes, &v); err == nil {
				r.RequestPrompt = v
				r.UserPromptPreview = ExtractPromptText(v)
			}
		}

		if len(compBytes) > 0 {
			var v any
			if err := json.Unmarshal(compBytes, &v); err == nil {
				r.ResponseCompletion = v
				r.AssistantCompletion = ExtractCompletionText(v)
			}
		}

		records = append(records, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating request_logs rows: %w", err)
	}

	return records, nil
}
