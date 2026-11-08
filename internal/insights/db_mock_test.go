package insights

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"testing"
	"time"
)

type mockDriver struct {
	rows []mockRow
}

type mockRow struct {
	id                  string
	startedAt           time.Time
	completedAt         time.Time
	durationMS          int64
	httpStatus          int
	err                 *string
	opName              string
	providerName        string
	reqModel            string
	respModel           string
	inTokens            int64
	outTokens           int64
	totTokens           int64
	cost                float64
	user                string
	group               string
	userAgent           string
	hasPayload          bool
	attributesJSON      []byte
	requestPromptJSON   []byte
	responsePayloadJSON []byte
}

func (m *mockDriver) Open(name string) (driver.Conn, error) {
	return &mockConn{driver: m}, nil
}

type mockConn struct {
	driver *mockDriver
}

func (c *mockConn) Prepare(query string) (driver.Stmt, error) {
	return &mockStmt{driver: c.driver}, nil
}

func (c *mockConn) Close() error {
	return nil
}

func (c *mockConn) Begin() (driver.Tx, error) {
	return nil, fmt.Errorf("not implemented")
}

type mockStmt struct {
	driver *mockDriver
}

func (s *mockStmt) Close() error {
	return nil
}

func (s *mockStmt) NumInput() int {
	return 1
}

func (s *mockStmt) Exec(args []driver.Value) (driver.Result, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *mockStmt) Query(args []driver.Value) (driver.Rows, error) {
	return &mockRows{rows: s.driver.rows}, nil
}

type mockRows struct {
	rows []mockRow
	pos  int
}

func (r *mockRows) Columns() []string {
	return []string{
		"id", "started_at", "completed_at", "duration_ms", "http_status", "error",
		"gen_ai_operation_name", "gen_ai_provider_name", "gen_ai_request_model", "gen_ai_response_model",
		"input_tokens", "output_tokens", "total_tokens", "cost",
		"agentgateway_user", "agentgateway_group", "user_agent_name",
		"has_payload", "attributes_json", "request_prompt_json", "response_completion_json",
	}
}

func (r *mockRows) Close() error {
	return nil
}

func (r *mockRows) Next(dest []driver.Value) error {
	if r.pos >= len(r.rows) {
		return io.EOF
	}
	row := r.rows[r.pos]
	r.pos++

	dest[0] = row.id
	dest[1] = row.startedAt
	dest[2] = row.completedAt
	dest[3] = row.durationMS
	dest[4] = int64(row.httpStatus)
	if row.err != nil {
		dest[5] = *row.err
	} else {
		dest[5] = nil
	}
	dest[6] = row.opName
	dest[7] = row.providerName
	dest[8] = row.reqModel
	dest[9] = row.respModel
	dest[10] = row.inTokens
	dest[11] = row.outTokens
	dest[12] = row.totTokens
	dest[13] = row.cost
	dest[14] = row.user
	dest[15] = row.group
	dest[16] = row.userAgent
	dest[17] = row.hasPayload
	dest[18] = row.attributesJSON
	dest[19] = row.requestPromptJSON
	dest[20] = row.responsePayloadJSON
	return nil
}

func init() {
	sql.Register("mock_pg", &mockDriver{
		rows: []mockRow{
			{
				id:                  "mock-1",
				startedAt:           time.Now().Add(-5 * time.Minute),
				completedAt:         time.Now().Add(-5 * time.Minute),
				durationMS:          120,
				httpStatus:          200,
				opName:              "chat",
				providerName:        "openai",
				reqModel:            "auto",
				respModel:           "gpt-4o",
				inTokens:            100,
				outTokens:           50,
				totTokens:           150,
				hasPayload:          true,
				attributesJSON:      []byte(`{"pinball.tier":"standard","pinball.model":"claude-sonnet-5.5","pinball.reason":"tools"}`),
				requestPromptJSON:   []byte(`[{"role":"user","content":"Hello world"}]`),
				responsePayloadJSON: []byte(`{"choices":[{"message":{"role":"assistant","content":"Hi there!"}}]}`),
			},
		},
	})
}

func TestQueryRequestsWithMockDB(t *testing.T) {
	db, err := sql.Open("mock_pg", "")
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	records, err := QueryRequests(context.Background(), db, 24*time.Hour)
	if err != nil {
		t.Fatalf("QueryRequests failed: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	r := records[0]
	if r.ID != "mock-1" {
		t.Errorf("expected ID mock-1, got %q", r.ID)
	}
	if r.PinballTier != "standard" {
		t.Errorf("expected PinballTier standard, got %q", r.PinballTier)
	}
	if r.PinballModel != "claude-sonnet-5.5" {
		t.Errorf("expected PinballModel claude-sonnet-5.5, got %q", r.PinballModel)
	}
	if r.UserPromptPreview != "Hello world" {
		t.Errorf("expected UserPromptPreview 'Hello world', got %q", r.UserPromptPreview)
	}
	if r.AssistantCompletion != "Hi there!" {
		t.Errorf("expected AssistantCompletion 'Hi there!', got %q", r.AssistantCompletion)
	}
}
