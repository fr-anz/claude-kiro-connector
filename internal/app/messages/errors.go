package messages

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/fr-anz/claudecode-kiro/internal/httpx"
	"github.com/fr-anz/claudecode-kiro/internal/kiroclient"
)

func writeRequestValidationError(w http.ResponseWriter, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		httpx.WriteError(w, http.StatusRequestEntityTooLarge, errTypeInvalidRequest,
			fmt.Sprintf("request body exceeds %d MiB limit; reduce image size or conversation history", maxRequestBytes>>20))
		return
	}
	httpx.WriteError(w, http.StatusBadRequest, errTypeInvalidRequest, err.Error())
}

// Re-exports of httpx error type constants so in-package callers stay concise.
const (
	errTypeInvalidRequest = httpx.ErrTypeInvalidRequest
	errTypeAPI            = httpx.ErrTypeAPI
	ErrTypeAuthentication = httpx.ErrTypeAuthentication
	errTypeStreamError    = httpx.ErrTypeStream
	errTypeRateLimit      = httpx.ErrTypeRateLimit
	errTypeOverloaded     = httpx.ErrTypeOverloaded
)

// reasonContentLengthExceeded is Kiro's invalidStateEvent reason for a
// conversation that exceeds the model's context window.
const reasonContentLengthExceeded = "CONTENT_LENGTH_EXCEEDS_THRESHOLD"

// retryableInvalidStateReasons are invalidStateEvent reasons that can be resolved
// by clearing the conversation ID and retrying.
var retryableInvalidStateReasons = map[string]struct{}{
	reasonContentLengthExceeded:  {},
	"INVALID_CONVERSATION_STATE": {},
	"STALE_CONVERSATION":         {},
}

type upstreamErrorClassification struct {
	retryReason string
	final       streamFinalError
}

type streamFinalError struct {
	status      int
	jsonType    string
	jsonMessage string
	sseType     string
	sseMessage  string
}

func newStreamFinalError(status int, errType, message string) streamFinalError {
	return streamFinalError{
		status:      status,
		jsonType:    errType,
		jsonMessage: message,
		sseType:     errType,
		sseMessage:  message,
	}
}

// classifyUpstreamError separates upstream event classification from response
// writing so streaming callers can choose JSON or SSE based on session state.
// For exception frames, invalidReason carries the :exception-type header.
func classifyUpstreamError(isException bool, invalidReason, upstreamMessage string) upstreamErrorClassification {
	if isException {
		final, ok := classifyAWSException(invalidReason)
		if !ok {
			final = newStreamFinalError(http.StatusBadGateway, errTypeAPI, "upstream exception")
		}
		if upstreamMessage != "" {
			final.sseMessage = upstreamMessage
		}
		return upstreamErrorClassification{final: final}
	}
	final := newStreamFinalError(
		http.StatusBadRequest,
		errTypeInvalidRequest,
		"invalid state: request rejected by upstream",
	)
	final.sseType = "invalid_state"
	if upstreamMessage != "" {
		final.sseMessage = upstreamMessage
	}
	classification := upstreamErrorClassification{
		final: final,
	}
	if _, ok := retryableInvalidStateReasons[invalidReason]; ok {
		classification.retryReason = invalidReason
	}
	return classification
}

// classifyAWSException maps Kiro's AWS exception classes to the Anthropic
// errors Claude Code reacts to: rate_limit_error (429) and overloaded_error
// (529). Other exceptions return false and keep the caller's default.
func classifyAWSException(exception string) (streamFinalError, bool) {
	switch exception {
	case "ThrottlingException", "TooManyRequestsException":
		return newStreamFinalError(http.StatusTooManyRequests, errTypeRateLimit, "Kiro rate limit exceeded"), true
	case "ServiceUnavailableException":
		return newStreamFinalError(httpx.StatusOverloaded, errTypeOverloaded, "Kiro is temporarily overloaded"), true
	}
	return streamFinalError{}, false
}

// upstreamCallError maps a failed Kiro request (after the client's own
// retries) to a downstream error. Unclassified failures stay 502 with message.
func upstreamCallError(err error, message string) streamFinalError {
	var ue *kiroclient.UpstreamError
	if errors.As(err, &ue) {
		if final, ok := classifyAWSException(ue.Exception); ok {
			return final
		}
		switch ue.Status {
		case http.StatusTooManyRequests:
			final, _ := classifyAWSException("ThrottlingException")
			return final
		case http.StatusServiceUnavailable:
			final, _ := classifyAWSException("ServiceUnavailableException")
			return final
		}
	}
	return newStreamFinalError(http.StatusBadGateway, errTypeAPI, message)
}

// writeFinalJSONError writes a classified error as a non-streaming response.
func writeFinalJSONError(w http.ResponseWriter, final streamFinalError) {
	httpx.WriteError(w, final.status, final.jsonType, final.jsonMessage)
}

// promptTooLongError reports context overflow in the exact wording Claude Code
// matches to trigger reactive auto-compaction.
func promptTooLongError(promptTokens, contextWindowSize int) streamFinalError {
	promptTokens = max(promptTokens, contextWindowSize+1)
	return newStreamFinalError(http.StatusBadRequest, errTypeInvalidRequest,
		fmt.Sprintf("prompt is too long: %d tokens > %d maximum", promptTokens, contextWindowSize))
}

// logUpstreamError logs a "kiro api error" with structured attributes when the
// error is an *UpstreamError. Falls back to plain err logging otherwise.
func logUpstreamError(ctx context.Context, short string, err error, extra ...any) {
	attrs := []any{"trace_id", short, "err", err}
	attrs = append(attrs, extra...)
	var ue *kiroclient.UpstreamError
	if errors.As(err, &ue) {
		attrs = append(attrs,
			"status", ue.Status,
			"content_type", ue.ContentType,
			"exception", ue.Exception,
			"body", ue.Body,
		)
	}
	slog.ErrorContext(ctx, "kiro api error", attrs...)
}
