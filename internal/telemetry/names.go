package telemetry

// AgentGateway metric names (verified empirically against v1.6.0).
const (
	GWRequests       = "agentgateway_requests_total"
	GWTokens         = "agentgateway_gen_ai_client_token_usage_sum" //nolint:gosec // G101 false positive: Prometheus metric name
	GWTokenTypeLabel = "gen_ai_token_type"                          //nolint:gosec // G101 false positive: Prometheus label name
)
