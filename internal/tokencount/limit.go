package tokencount

import "unicode/utf8"

// LimitText returns a valid UTF-8 prefix within the local tokenizer budget.
func LimitText(text string, budget int) (string, int, error) {
	e, err := getEncoding()
	if err != nil {
		return "", 0, err
	}
	tokens := e.Encode(text, nil, nil)
	if len(tokens) <= budget {
		return text, len(tokens), nil
	}
	if budget <= 0 {
		return "", 0, nil
	}
	prefix := e.Decode(tokens[:budget])
	for !utf8.ValidString(prefix) {
		prefix = prefix[:len(prefix)-1]
	}
	return prefix, len(e.Encode(prefix, nil, nil)), nil
}
