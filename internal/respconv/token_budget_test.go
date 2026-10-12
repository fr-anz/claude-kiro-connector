package respconv

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/fr-anz/claude-kiro-connector/internal/kiroproto"
	"github.com/fr-anz/claude-kiro-connector/internal/tokencount"
)

func TestTokenBudget_UsesTokenizerAndKeepsUTF8(t *testing.T) {
	for _, text := range []string{"internationalization", strings.Repeat("你好世界", 10)} {
		budget := 3
		acc := newAccumulator(200000, nil, budget, 1)
		acc.ProcessEvent(kiroproto.Event{Type: kiroproto.EventAssistantResponse, Content: text})
		out := acc.TextBuf.String()
		n, err := tokencount.CountBytes([]byte(out))
		if err != nil {
			t.Fatal(err)
		}
		if !utf8.ValidString(out) || n > budget {
			t.Fatalf("invalid or over-budget output: tokens=%d", n)
		}
		original, _ := tokencount.CountBytes([]byte(text))
		if original <= budget && out != text {
			t.Fatal("text truncated even though its tokens fit")
		}
	}
}

func TestTokenBudget_DoesNotEmitOversizedToolJSON(t *testing.T) {
	acc := newAccumulator(200000, nil, 1, 1)
	d := acc.ProcessEvent(kiroproto.Event{Type: kiroproto.EventToolUse, ToolUseID: "call", ToolName: "Bash", ToolInput: `{"command":"echo hello"}`, ToolStop: true})
	if d.ToolStop || len(acc.ToolCalls) != 0 || !acc.LocalStop {
		t.Fatal("oversized tool was emitted")
	}
}
