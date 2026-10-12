package respconv

// ToolCall represents a finalized tool call from the response stream.
type ToolCall struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Input string `json:"input"` // raw JSON string
}

// DeduplicateToolCalls removes replayed calls sharing an ID, keeping the longest
// input. Distinct IDs remain distinct even when their arguments are identical.
func DeduplicateToolCalls(calls []ToolCall) []ToolCall {
	if len(calls) <= 1 {
		return calls
	}
	byID := make(map[string]ToolCall)
	var order []string
	for _, c := range calls {
		if existing, ok := byID[c.ID]; ok {
			if len(c.Input) > len(existing.Input) {
				byID[c.ID] = c
			}
		} else {
			byID[c.ID] = c
			order = append(order, c.ID)
		}
	}
	result := make([]ToolCall, 0, len(order))
	for _, id := range order {
		result = append(result, byID[id])
	}
	return result
}
