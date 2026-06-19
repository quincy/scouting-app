package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRenderMarkdown_Basic(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		notWant []string
	}{
		{
			name:  "headers",
			input: "# Hello\n## World",
			want:  "<h1>Hello</h1>\n<h2>World</h2>",
		},
		{
			name:  "bold",
			input: "**bold text**",
			want:  "<strong>bold text</strong>",
		},
		{
			name:  "list",
			input: "- item 1\n- item 2",
			want:  "<li>item 1</li>\n<li>item 2</li>",
		},
		{
			name:  "code block",
			input: "```\ncode\n```",
			want:  "<code>code\n</code>",
		},
		{
			name:  "link",
			input: "[link](https://example.com)",
			want:  "<a href=\"https://example.com\">link</a>",
		},
		{
			name:  "xss safety",
			input: "<script>alert('xss')</script>",
			want:  "raw HTML omitted",
		},
		{
			name:     "raw html omitted",
			input:    "<div>raw</div>",
			notWant: []string{"<div>raw</div>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := renderMarkdown(tt.input)
			if err != nil {
				t.Fatalf("renderMarkdown returned error: %v", err)
			}
			if tt.want != "" && !strings.Contains(got, tt.want) {
				t.Errorf("renderMarkdown(%q) = %q, want it to contain %q", tt.input, got, tt.want)
			}
			for _, nw := range tt.notWant {
				if strings.Contains(got, nw) {
					t.Errorf("renderMarkdown(%q) = %q, should not contain %q", tt.input, got, nw)
				}
			}
		})
	}
}

func TestEventHandler_MarkdownPreview(t *testing.T) {
	_, _, _, _, handler, _ := setupEventTest(t)

	body := "markdown=**bold** and _italic_"
	req := httptest.NewRequest("POST", "/admin/markdown-preview", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	handler.MarkdownPreview(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("MarkdownPreview returned status %d, want %d", rr.Code, http.StatusOK)
	}

	got := rr.Body.String()
	if !strings.Contains(got, "<strong>bold</strong>") {
		t.Errorf("expected <strong>bold</strong> in response, got:\n%s", got)
	}
	if !strings.Contains(got, "<em>italic</em>") {
		t.Errorf("expected <em>italic</em> in response, got:\n%s", got)
	}

	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("expected Content-Type text/html, got %q", ct)
	}
}

func TestEventHandler_MarkdownPreview_XSS(t *testing.T) {
	_, _, _, _, handler, _ := setupEventTest(t)

	body := "markdown=<script>alert('xss')</script>"
	req := httptest.NewRequest("POST", "/admin/markdown-preview", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	handler.MarkdownPreview(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("MarkdownPreview returned status %d, want %d", rr.Code, http.StatusOK)
	}

	got := rr.Body.String()
	if strings.Contains(got, "<script>") {
		t.Errorf("expected raw <script> to be escaped, got:\n%s", got)
	}
}
