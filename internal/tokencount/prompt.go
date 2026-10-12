package tokencount

import (
	"encoding/base64"
	"encoding/json/v2"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"strings"

	"github.com/fr-anz/claude-kiro-connector/internal/kiroproto"
)

// CountPromptBytes counts the text/tool envelope separately from image bytes.
// Images use a bounded dimension estimate, not a tokenizer pass over base64.
func CountPromptBytes(data []byte) (int, error) {
	var payload kiroproto.Payload
	if err := json.Unmarshal(data, &payload); err != nil {
		return 0, err
	}
	imageTokens := 0
	stripImages := func(images []kiroproto.Image) {
		for _, img := range images {
			imageTokens += estimatedImageTokens(img.Source.Bytes)
		}
	}
	current := &payload.ConversationState.CurrentMessage.UserInputMessage
	stripImages(current.Images)
	current.Images = nil
	for _, entry := range payload.ConversationState.History {
		if entry.UserInputMessage != nil {
			stripImages(entry.UserInputMessage.Images)
			entry.UserInputMessage.Images = nil
		}
	}
	if imageTokens == 0 {
		return CountBytes(data)
	}
	text, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	n, err := CountBytes(text)
	return n + imageTokens, err
}

// Claude vision uses 28px patches, with up to 4784 tokens on high-resolution
// models: https://platform.claude.com/docs/en/build-with-claude/vision
// Kiro processing may differ; upstream usage remains authoritative.
func estimatedImageTokens(data string) int {
	const fallback = 4784
	r := base64.NewDecoder(base64.StdEncoding, strings.NewReader(data))
	config, _, err := image.DecodeConfig(io.LimitReader(r, 1<<20))
	if err != nil || config.Width <= 0 || config.Height <= 0 {
		return fallback
	}
	patches := math.Ceil(float64(config.Width)/28) * math.Ceil(float64(config.Height)/28)
	return int(min(float64(fallback), patches))
}
