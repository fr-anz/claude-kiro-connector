package respconv

import (
	"math"
	"strings"
	"unicode/utf8"

	"github.com/fr-anz/claude-kiro-connector/internal/tokencount"
)

// applyMaxTokensBudget checks whether adding delta would exceed the max_tokens budget.
// Uses the same tokenizer as fallback usage. Rune estimation is only a fallback
// when tokenizer initialization is unavailable.
// If the budget is exceeded, sets LocalStop and StopReason.
func (a *responseAccumulator) applyMaxTokensBudget(delta string) string {
	if a.maxTokensBudget <= 0 {
		a.outputRuneCount += utf8.RuneCountInString(delta)
		return delta
	}
	previous := a.outputBudgetText.String()
	combined := previous + delta
	prefix, n, err := tokencount.LimitText(combined, a.maxTokensBudget)
	if err != nil {
		out := a.applyRuneBudget(delta)
		a.outputBudgetText.WriteString(out)
		return out
	}
	if !strings.HasPrefix(prefix, previous) {
		prefix = previous
	}
	out := prefix[len(previous):]
	a.outputBudgetText.WriteString(out)
	a.outputRuneCount += utf8.RuneCountInString(out)
	a.outputTokenCount = n
	if prefix != combined || n >= a.maxTokensBudget {
		a.LocalStop = true
		a.StopReason = StopReasonMaxTokens
	}
	return out
}

func (a *responseAccumulator) applyRuneBudget(delta string) string {
	runesInDelta := utf8.RuneCountInString(delta)
	newTotal := a.outputRuneCount + runesInDelta
	if newTotal/4 < a.maxTokensBudget {
		a.outputRuneCount = newTotal
		return delta
	}
	// Budget exceeded — find the cutoff point.
	// Allow up to (maxTokensBudget * 4 - outputRuneCount) more runes.
	maxRunes := math.MaxInt
	if a.maxTokensBudget <= math.MaxInt/4 {
		maxRunes = a.maxTokensBudget * 4
	}
	remaining := maxRunes - a.outputRuneCount
	if remaining <= 0 {
		a.LocalStop = true
		a.StopReason = StopReasonMaxTokens
		return ""
	}
	// Truncate delta to remaining runes.
	count := 0
	for i := range delta {
		count++
		if count > remaining {
			a.outputRuneCount += remaining
			a.LocalStop = true
			a.StopReason = StopReasonMaxTokens
			return delta[:i]
		}
	}
	// All runes fit (edge case: exactly at boundary).
	a.outputRuneCount += runesInDelta
	if a.outputRuneCount/4 >= a.maxTokensBudget {
		a.LocalStop = true
		a.StopReason = StopReasonMaxTokens
	}
	return delta
}
