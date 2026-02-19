package output

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

var unsafeChars = regexp.MustCompile(`[^a-zA-Z0-9\-_.]`)

func sanitizeContextName(name string) string {
	if name == "" {
		return "default"
	}
	return unsafeChars.ReplaceAllString(name, "_")
}

// saveMarkdownFile writes a markdown file to output/<context>/<command>_<timestamp>.md.
func saveMarkdownFile(command, contextName string, ts time.Time, tableMarkdown string) {
	dir := filepath.Join("output", sanitizeContextName(contextName))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to create output directory %s: %v\n", dir, err)
		return
	}

	filename := fmt.Sprintf("%s_%s.md", command, ts.Format("20060102_150405"))
	path := filepath.Join(dir, filename)

	header := fmt.Sprintf("# kusa %s — %s\n\n_Generated at %s_\n\n",
		command, contextName, ts.UTC().Format("2006-01-02 15:04:05 UTC"))
	content := header + tableMarkdown + "\n"

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to write markdown file %s: %v\n", path, err)
		return
	}

	fmt.Printf("Saved: %s\n", path)
}
