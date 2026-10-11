package messages

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/fr-anz/claudecode-kiro/internal/httpx"
	"github.com/fr-anz/claudecode-kiro/internal/kiroclient"
)

func TestUpstreamCallError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantType   string
	}{
		{"throttling exception", &kiroclient.UpstreamError{Status: 400, Exception: "ThrottlingException"}, http.StatusTooManyRequests, errTypeRateLimit},
		{"too many requests exception", &kiroclient.UpstreamError{Status: 200, Exception: "TooManyRequestsException"}, http.StatusTooManyRequests, errTypeRateLimit},
		{"bare 429", &kiroclient.UpstreamError{Status: http.StatusTooManyRequests}, http.StatusTooManyRequests, errTypeRateLimit},
		{"service unavailable exception", &kiroclient.UpstreamError{Status: 500, Exception: "ServiceUnavailableException"}, httpx.StatusOverloaded, errTypeOverloaded},
		{"bare 503", &kiroclient.UpstreamError{Status: http.StatusServiceUnavailable}, httpx.StatusOverloaded, errTypeOverloaded},
		{"wrapped throttling", fmt.Errorf("call: %w", &kiroclient.UpstreamError{Status: 400, Exception: "ThrottlingException"}), http.StatusTooManyRequests, errTypeRateLimit},
		{"validation stays 502", &kiroclient.UpstreamError{Status: 400, Exception: "ValidationException"}, http.StatusBadGateway, errTypeAPI},
		{"internal 500 stays 502", &kiroclient.UpstreamError{Status: 500}, http.StatusBadGateway, errTypeAPI},
		{"transport error stays 502", errors.New("connection reset"), http.StatusBadGateway, errTypeAPI},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := upstreamCallError(tt.err, "upstream API error")
			if got.status != tt.wantStatus || got.jsonType != tt.wantType || got.sseType != tt.wantType {
				t.Fatalf("got status=%d json=%q sse=%q, want %d %q", got.status, got.jsonType, got.sseType, tt.wantStatus, tt.wantType)
			}
		})
	}
}

func TestClassifyUpstreamError_ExceptionFrame(t *testing.T) {
	c := classifyUpstreamError(true, "ThrottlingException", "slow down")
	if c.final.status != http.StatusTooManyRequests || c.final.sseType != errTypeRateLimit || c.final.sseMessage != "slow down" {
		t.Fatalf("throttling frame = %+v", c.final)
	}
	c = classifyUpstreamError(true, "InternalServerException", "boom")
	if c.final.status != http.StatusBadGateway || c.final.sseType != errTypeAPI {
		t.Fatalf("internal frame = %+v", c.final)
	}
}

func TestPromptTooLongError(t *testing.T) {
	got := promptTooLongError(250_000, 200_000)
	want := "prompt is too long: 250000 tokens > 200000 maximum"
	if got.status != http.StatusBadRequest || got.jsonType != errTypeInvalidRequest || got.jsonMessage != want || got.sseMessage != want {
		t.Fatalf("got %+v, want 400 %q", got, want)
	}
	// An unknown or under-counted prompt still reports a value above the
	// window so Claude Code's parser sees an overflow.
	if got := promptTooLongError(0, 200_000); got.jsonMessage != "prompt is too long: 200001 tokens > 200000 maximum" {
		t.Fatalf("zero count message = %q", got.jsonMessage)
	}
}
