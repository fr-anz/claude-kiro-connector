package respconv

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fr-anz/claude-kiro-connector/internal/kiroproto"
)

func TestDeduplicateToolCalls(t *testing.T) {
	tests := []struct {
		name      string
		calls     []ToolCall
		wantLen   int
		wantInput string // optional: check first result's Input
	}{
		{
			name: "no_duplicates",
			calls: []ToolCall{
				{ID: "1", Name: "a", Input: `{"x":1}`},
				{ID: "2", Name: "b", Input: `{"y":2}`},
			},
			wantLen: 2,
		},
		{
			name: "id_dedup_keep_longer",
			calls: []ToolCall{
				{ID: "1", Name: "a", Input: `{"x":1}`},
				{ID: "1", Name: "a", Input: `{"x":1,"y":2}`},
			},
			wantLen:   1,
			wantInput: `{"x":1,"y":2}`,
		},
		{
			name: "distinct_ids_same_arguments",
			calls: []ToolCall{
				{ID: "1", Name: "a", Input: `{"x": 1}`},
				{ID: "2", Name: "a", Input: `{"x":1}`}, // same after normalization
			},
			wantLen: 2,
		},
		{
			name:    "empty",
			calls:   nil,
			wantLen: 0,
		},
		{
			name:    "single",
			calls:   []ToolCall{{ID: "1", Name: "a", Input: `{}`}},
			wantLen: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeduplicateToolCalls(tt.calls)
			if len(got) != tt.wantLen {
				t.Fatalf("got %d, want %d", len(got), tt.wantLen)
			}
			if tt.wantInput != "" && got[0].Input != tt.wantInput {
				t.Fatalf("input = %s, want %s", got[0].Input, tt.wantInput)
			}
		})
	}
}

func TestRepeatedToolCalls_StreamParity(t *testing.T) {
	events := []kiroproto.Event{
		{Type: kiroproto.EventToolUse, ToolUseID: "first", ToolName: "Bash", ToolInput: `{"command":"echo hello"}`, ToolStop: true},
		{Type: kiroproto.EventToolUse, ToolUseID: "second", ToolName: "Bash", ToolInput: `{"command":"echo hello"}`, ToolStop: true},
	}
	response, _ := BuildNonStreamingResponse(events, "test", 200000, nil, 0, 0)
	content := response["content"].([]any)
	if len(content) != 2 {
		t.Fatalf("non-streaming calls=%d, want 2", len(content))
	}
	w := httptest.NewRecorder()
	sw := NewSSEWriter(t.Context(), w, "test", 200000, nil, 0, 0)
	for _, event := range events {
		sw.HandleEvent(event)
	}
	if err := sw.Finish(); err != nil {
		t.Fatal(err)
	}
	for i, id := range []string{"first", "second"} {
		if content[i].(map[string]any)["id"] != id || !strings.Contains(w.Body.String(), `"id":"`+id+`"`) {
			t.Fatalf("missing tool call %s", id)
		}
	}
}
