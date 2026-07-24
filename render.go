package cachefp

import (
	"embed"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

//go:embed templates/*.html
var templateFS embed.FS

var htmlEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	`"`, "&quot;",
	"'", "&#039;",
)

// renderTemplate loads templates/<name>, substitutes {{key}} placeholders
// with HTML-escaped values from options, and writes it as the response.
func renderTemplate(w http.ResponseWriter, name string, options map[string]string) {
	content, err := templateFS.ReadFile("templates/" + name)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	out := string(content)
	for key, value := range options {
		re := regexp.MustCompile(`\{\{` + regexp.QuoteMeta(key) + `\}\}`)
		out = re.ReplaceAllString(out, htmlEscaper.Replace(value))
	}
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, out)
}
