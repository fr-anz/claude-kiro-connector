package respconv

import (
	"strings"

	"github.com/fr-anz/claude-kiro-connector/internal/tokencount"
)

// estimatedOutputTokens returns an output token count for responses where
// Kiro sent no usage metadata. It tokenizes the accumulated text, thinking,
// and tool input; if the tokenizer is unavailable it falls back to the
// incrementally tracked ~4 runes per token heuristic.
// Returns at least 1 for non-empty output to avoid reporting 0 tokens for short responses.
func (a *responseAccumulator) estimatedOutputTokens() int {
	if a.outputRuneCount == 0 {
		return 0
	}
	var sb strings.Builder
	sb.WriteString(a.TextBuf.String())
	sb.WriteString(a.ThinkingBuf.String())
	for _, tc := range a.ToolCalls {
		sb.WriteString(tc.Name)
		sb.WriteString(tc.Input)
	}
	if sb.Len() > 0 {
		if n, err := tokencount.CountBytes([]byte(sb.String())); err == nil && n > 0 {
			return n
		}
	}
	return max(1, a.outputRuneCount/4)
}

// resolvedUsage returns the best available input and output token counts.
// Priority: metadata/metering > pre-counted (tiktoken) > contextUsage estimate > 0.
func (a *responseAccumulator) resolvedUsage() (inputTokens, outputTokens int) {
	if a.HasMetadata || a.InputTokens > 0 || a.OutputTokens > 0 {
		return a.InputTokens, a.OutputTokens
	}
	if a.PreCountedInputTokens > 0 {
		return a.PreCountedInputTokens, a.estimatedOutputTokens()
	}
	if a.HasContextUsage && a.ContextWindowSize > 0 {
		pct := max(0, min(100, a.ContextUsagePercentage))
		estOutput := a.estimatedOutputTokens()
		total := int(pct / 100 * float64(a.ContextWindowSize))
		estInput := max(0, total-estOutput)
		return estInput, estOutput
	}
	return 0, 0
}

// UsageMap builds an Anthropic-compatible usage map from the given token counts.
func (a *responseAccumulator) UsageMap(inputTokens, outputTokens int) map[string]any {
	return map[string]any{
		"input_tokens":                inputTokens,
		"output_tokens":               outputTokens,
		"cache_read_input_tokens":     a.CacheReadInputTokens,
		"cache_creation_input_tokens": a.CacheWriteInputTokens,
	}
}
