package search

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type WebSearchTool struct {
	client *http.Client
}

func New() *WebSearchTool {
	return &WebSearchTool{client: &http.Client{}}
}

var snippetRe = regexp.MustCompile(`(?s)<a class="result__snippet"[^>]*>(.*?)</a>`)
var tagRe = regexp.MustCompile(`<[^>]+>`)

func (w *WebSearchTool) Search(query string) (string, error) {
	req, err := http.NewRequest("GET", "https://html.duckduckgo.com/html/?q="+url.QueryEscape(query), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Sprintf("Search failed: %v", err), nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	matches := snippetRe.FindAllSubmatch(body, -1)
	if len(matches) == 0 {
		return "No results found.", nil
	}

	var parts []string
	for i, m := range matches {
		if i >= 3 {
			break
		}
		snippet := tagRe.ReplaceAllString(string(m[1]), "")
		parts = append(parts, strings.TrimSpace(snippet))
	}
	return strings.Join(parts, "\n---\n"), nil
}
