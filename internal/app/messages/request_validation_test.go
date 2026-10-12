package messages

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseRequestValidation(t *testing.T) {
	for _, tc := range []struct {
		name, model, role string
		budget            int
	}{
		{"empty model", "", "user", 10},
		{"blank model", " ", "user", 10},
		{"invalid role", "claude-sonnet-4-6", "system", 10},
		{"negative budget", "claude-sonnet-4-6", "user", -1},
		{"missing budget", "claude-sonnet-4-6", "user", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := fmt.Sprintf(`{"model":%q,"max_tokens":%d,"messages":[{"role":%q,"content":"hi"}]}`, tc.model, tc.budget, tc.role)
			r := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(body))
			if _, err := parseAndValidateRequest(t.Context(), httptest.NewRecorder(), r); err == nil {
				t.Fatal("invalid request accepted")
			}
		})
	}
	t.Run("count tokens needs no output budget", func(t *testing.T) {
		body := `{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"hi"}]}`
		r := httptest.NewRequest("POST", "/v1/messages/count_tokens", strings.NewReader(body))
		if _, err := parseAndValidateRequest(t.Context(), httptest.NewRecorder(), r); err != nil {
			t.Fatal(err)
		}
	})
}
