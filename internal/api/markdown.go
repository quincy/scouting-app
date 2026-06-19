package api

import (
	"bytes"
	"log"
	"net/http"

	"github.com/yuin/goldmark"
)

func renderMarkdown(source string) (string, error) {
	md := goldmark.New()
	var buf bytes.Buffer
	if err := md.Convert([]byte(source), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (h *EventHandler) MarkdownPreview(w http.ResponseWriter, r *http.Request) {
	markdown := r.FormValue("markdown")
	html, err := renderMarkdown(markdown)
	if err != nil {
		log.Printf("renderMarkdown: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}
