package tokencount

import (
	"bytes"
	"encoding/base64"
	"encoding/json/v2"
	"image"
	"image/png"
	"strings"
	"testing"

	"github.com/fr-anz/claude-kiro-connector/internal/kiroproto"
)

func imagePrompt(t testing.TB, padding int) []byte {
	t.Helper()
	var pngBytes bytes.Buffer
	if err := png.Encode(&pngBytes, image.NewRGBA(image.Rect(0, 0, 200, 200))); err != nil {
		t.Fatal(err)
	}
	pngBytes.WriteString(strings.Repeat("x", padding))
	img := kiroproto.Image{Format: "png", Source: kiroproto.ImageSource{Bytes: base64.StdEncoding.EncodeToString(pngBytes.Bytes())}}
	var payload kiroproto.Payload
	payload.ConversationState.CurrentMessage.UserInputMessage = kiroproto.UserInputMessage{Content: "describe", Images: []kiroproto.Image{img}}
	payload.ConversationState.History = []kiroproto.HistoryEntry{{UserInputMessage: &kiroproto.HistoryUserInputMessage{Content: "earlier", Images: []kiroproto.Image{img}}}}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestCountPromptBytes_ImageEncodingDoesNotInflateTokens(t *testing.T) {
	small, err := CountPromptBytes(imagePrompt(t, 0))
	if err != nil {
		t.Fatal(err)
	}
	large, err := CountPromptBytes(imagePrompt(t, 2<<20))
	if err != nil {
		t.Fatal(err)
	}
	if small != large || small < 128 || small > 1000 {
		t.Fatalf("image estimates small=%d large=%d", small, large)
	}
	if n := estimatedImageTokens("invalid-image"); n != 4784 {
		t.Fatalf("invalid image estimate=%d", n)
	}
}

func BenchmarkCountPromptBytes_Image(b *testing.B) {
	data := imagePrompt(b, 2<<20)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := CountPromptBytes(data); err != nil {
			b.Fatal(err)
		}
	}
}
