package reqconv

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/fr-anz/claudecode-kiro/internal/anthropic"
)

// minimalPDF builds a one-page PDF whose content stream draws text, with a
// correct xref table so strict parsers accept it.
func minimalPDF(text string) []byte {
	stream := fmt.Sprintf("BT /F1 12 Tf 72 720 Td (%s) Tj ET", text)
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}
	var b strings.Builder
	b.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objs))
	for i, o := range objs {
		offsets[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, off := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return []byte(b.String())
}

func TestDocumentText(t *testing.T) {
	pdfData := base64.StdEncoding.EncodeToString(minimalPDF("Quarterly revenue grew"))
	tests := []struct {
		name  string
		block anthropic.ContentBlock
		want  []string
	}{
		{
			name:  "pdf base64",
			block: anthropic.ContentBlock{Type: "document", Title: "report.pdf", Source: &anthropic.ImageSource{Type: "base64", MediaType: "application/pdf", Data: pdfData}},
			want:  []string{`<document title="report.pdf">`, "Quarterly revenue grew", "</document>"},
		},
		{
			name:  "plain text source",
			block: anthropic.ContentBlock{Type: "document", Context: "from the wiki", Source: &anthropic.ImageSource{Type: "text", MediaType: "text/plain", Data: "hello doc"}},
			want:  []string{"from the wiki", "hello doc"},
		},
		{
			name:  "base64 text/plain",
			block: anthropic.ContentBlock{Type: "document", Source: &anthropic.ImageSource{Type: "base64", MediaType: "text/plain", Data: base64.StdEncoding.EncodeToString([]byte("encoded text"))}},
			want:  []string{"encoded text"},
		},
		{
			name: "content source",
			block: anthropic.ContentBlock{Type: "document", Source: &anthropic.ImageSource{Type: "content", Content: anthropic.MessageContent{Blocks: []anthropic.ContentBlock{
				{Type: "text", Text: "chunk one"}, {Type: "text", Text: "chunk two"},
			}}}},
			want: []string{"chunk one chunk two"},
		},
		{
			name:  "unreadable pdf",
			block: anthropic.ContentBlock{Type: "document", Source: &anthropic.ImageSource{Type: "base64", MediaType: "application/pdf", Data: base64.StdEncoding.EncodeToString([]byte("not a pdf"))}},
			want:  []string{"no extractable text"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := documentText(tt.block)
			for _, w := range tt.want {
				if !strings.Contains(got, w) {
					t.Fatalf("documentText() = %q, missing %q", got, w)
				}
			}
		})
	}
}

func TestBuildPayload_DocumentInCurrentMessage(t *testing.T) {
	req := &anthropic.Request{
		Model: "claude-sonnet-4-6",
		Messages: []anthropic.Message{{Role: "user", Content: anthropic.MessageContent{Blocks: []anthropic.ContentBlock{
			{Type: "document", Source: &anthropic.ImageSource{Type: "text", MediaType: "text/plain", Data: "the secret word is plover"}},
			{Type: "text", Text: "What is the secret word?"},
		}}}},
	}
	payload, _, err := BuildPayload(req, BuildOptions{ModelID: "claude-sonnet-4.6"})
	if err != nil {
		t.Fatal(err)
	}
	content := payload.ConversationState.CurrentMessage.UserInputMessage.Content
	if !strings.Contains(content, "the secret word is plover") || strings.Contains(content, "[document]") {
		t.Fatalf("current message content = %q", content)
	}
}

func TestBuildPayload_DocumentInToolResult(t *testing.T) {
	pdfData := base64.StdEncoding.EncodeToString(minimalPDF("Invoice total 42"))
	req := &anthropic.Request{
		Model: "claude-sonnet-4-6",
		Tools: []anthropic.Tool{{Name: "Read", InputSchema: map[string]any{"type": "object"}}},
		Messages: []anthropic.Message{
			{Role: "user", Content: anthropic.MessageContent{Text: "read the invoice"}},
			{Role: "assistant", Content: anthropic.MessageContent{Blocks: []anthropic.ContentBlock{{Type: "tool_use", ID: "t1", Name: "Read", Input: map[string]any{"file_path": "invoice.pdf"}}}}},
			{Role: "user", Content: anthropic.MessageContent{Blocks: []anthropic.ContentBlock{{Type: "tool_result", ToolUseID: "t1", Content: anthropic.MessageContent{Blocks: []anthropic.ContentBlock{
				{Type: "document", Source: &anthropic.ImageSource{Type: "base64", MediaType: "application/pdf", Data: pdfData}},
			}}}}}},
		},
	}
	payload, _, err := BuildPayload(req, BuildOptions{ModelID: "claude-sonnet-4.6"})
	if err != nil {
		t.Fatal(err)
	}
	results := payload.ConversationState.CurrentMessage.UserInputMessage.UserInputMessageContext.ToolResults
	if len(results) != 1 {
		t.Fatalf("tool results = %d, want 1", len(results))
	}
	stdout, _ := results[0].Content[0].JSON["stdout"].(string)
	if !strings.Contains(stdout, "Invoice total 42") {
		t.Fatalf("tool result stdout = %q", stdout)
	}
}

func TestBuildPayload_ToolChoice(t *testing.T) {
	tools := []anthropic.Tool{{Name: "get_weather", InputSchema: map[string]any{"type": "object"}}}
	tests := []struct {
		name   string
		choice *anthropic.ToolChoice
		tools  []anthropic.Tool
		want   string
	}{
		{"auto adds nothing", &anthropic.ToolChoice{Type: "auto"}, tools, ""},
		{"any", &anthropic.ToolChoice{Type: "any"}, tools, "must respond by calling one of the available tools"},
		{"specific tool", &anthropic.ToolChoice{Type: "tool", Name: "get_weather"}, tools, "calling the `get_weather` tool"},
		{"none", &anthropic.ToolChoice{Type: "none"}, tools, "Do not call any tools"},
		{"ignored without tools", &anthropic.ToolChoice{Type: "any"}, nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &anthropic.Request{
				Model:      "claude-sonnet-4-6",
				System:     anthropic.SystemPrompt{Text: "base system"},
				Tools:      tt.tools,
				ToolChoice: tt.choice,
				Messages:   []anthropic.Message{{Role: "user", Content: anthropic.MessageContent{Text: "weather?"}}},
			}
			payload, _, err := BuildPayload(req, BuildOptions{ModelID: "claude-sonnet-4.6"})
			if err != nil {
				t.Fatal(err)
			}
			system := payload.ConversationState.History[0].UserInputMessage.Content
			if !strings.HasPrefix(system, "base system") {
				t.Fatalf("system prompt = %q", system)
			}
			if tt.want == "" {
				if strings.Contains(system, "<tool_choice>") {
					t.Fatalf("unexpected tool_choice directive: %q", system)
				}
				return
			}
			if !strings.Contains(system, tt.want) {
				t.Fatalf("system prompt = %q, missing %q", system, tt.want)
			}
		})
	}
}
