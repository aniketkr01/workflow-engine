package queue

import (
	"testing"

	"github.com/redis/go-redis/v9"
)

func TestParseMessage(t *testing.T) {
	tests := []struct {
		name      string
		message   redis.XMessage
		wantErr   bool
		wantID    string
		wantInput map[string]any
	}{
		{
			name: "valid message",
			message: redis.XMessage{
				ID: "1-0",
				Values: map[string]interface{}{
					"payload": `{"task_execution_id":"id","execution_id":"exec","workflow_id":"wf","task_def_id":"task","attempt":1,"input":{"foo":"bar"},"timeout_sec":60,"idempotency_key":"key"}`,
				},
			},
			wantErr: false,
			wantID:  "id",
			wantInput: map[string]any{
				"foo": "bar",
			},
		},
		{
			name: "missing payload",
			message: redis.XMessage{Values: map[string]interface{}{}},
			wantErr: true,
		},
		{
			name: "invalid json",
			message: redis.XMessage{Values: map[string]interface{}{"payload": "not-json"}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := ParseMessage(tt.message)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if msg.TaskExecutionID != tt.wantID {
				t.Fatalf("expected id %q, got %q", tt.wantID, msg.TaskExecutionID)
			}
			for k, want := range tt.wantInput {
				if got := msg.Input[k]; got != want {
					t.Fatalf("expected input[%q]=%v, got %v", k, want, got)
				}
			}
		})
	}
}
