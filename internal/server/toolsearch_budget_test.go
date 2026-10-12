package server

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/fr-anz/claude-kiro-connector/internal/kiroclient"
	"github.com/fr-anz/claude-kiro-connector/internal/kiroproto"
	"github.com/fr-anz/claude-kiro-connector/internal/tokencount"
)

func TestToolSearch_CumulativeBudgetAndPromptUsage(t *testing.T) {
	firstTokens, err := tokencount.CountBytes([]byte(`HelloToolSearch{"query":"Read"}`))
	if err != nil {
		t.Fatal(err)
	}
	budget := firstTokens + 1
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprintf("stream=%t", stream), func(t *testing.T) {
			calls := 0
			client := &mockKiroClient{handler: func(context.Context, string, *kiroproto.Payload, string) (*kiroclient.Response, error) {
				calls++
				body := buildEventStream("assistantResponseEvent", mustJSON(map[string]string{"content": "internationalization"}))
				if calls == 1 {
					body = buildEventStream("assistantResponseEvent", mustJSON(map[string]string{"content": "Hello"}), "toolUseEvent", mustJSON(map[string]any{"name": "ToolSearch", "toolUseId": "search", "input": map[string]string{"query": "Read"}, "stop": true}))
				}
				return &kiroclient.Response{StatusCode: 200, Body: body, PromptTokens: calls * 111}, nil
			}}
			srv := newTestServer(t, "", client)
			defer srv.Close()
			body := fmt.Sprintf(`{"model":"claude-sonnet-4-6","max_tokens":%d,"stream":%t,"messages":[{"role":"user","content":"find a tool"}],"tools":[{"type":"tool_search_tool_regex_20251119","name":"tool_search_tool_regex"},{"name":"Read","description":"Read a file","input_schema":{"type":"object"},"defer_loading":true}]}`, budget, stream)
			resp := postMessages(t, srv.URL, body)
			defer resp.Body.Close()
			data, _ := io.ReadAll(resp.Body)
			var result map[string]any
			if stream {
				for _, line := range strings.Split(string(data), "\n") {
					if strings.HasPrefix(line, "data: ") && strings.Contains(line, `"type":"message_delta"`) {
						if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &result); err != nil {
							t.Fatal(err)
						}
					}
				}
			} else if err := json.Unmarshal(data, &result); err != nil {
				t.Fatal(err)
			}
			usage, ok := result["usage"].(map[string]any)
			if !ok {
				t.Fatalf("missing usage: %s", data)
			}
			if calls != 2 || usage["input_tokens"] != float64(333) {
				t.Fatalf("calls=%d input=%v, want 2/333", calls, usage["input_tokens"])
			}
			if out := usage["output_tokens"].(float64); out > float64(budget) || out < float64(firstTokens) {
				t.Fatalf("output=%v budget=%d", out, budget)
			}
			stop := result["stop_reason"]
			if stream {
				stop = result["delta"].(map[string]any)["stop_reason"]
			}
			if stop != "max_tokens" {
				t.Fatalf("stop=%v", stop)
			}
		})
	}
}
