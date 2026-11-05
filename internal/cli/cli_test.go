package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOfflineStatusAndArgs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "form.html")
	if err := os.WriteFile(path, []byte(fixture), 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Run([]string{"status", "--html", path}, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "2026/11/13") {
		t.Fatal(&out)
	}
	for _, args := range [][]string{{"kesshoku", "add", "2026-11-14"}, {"gaihaku", "add", "2026-02-30"}, {"login", "extra"}, {"status", "--unknown"}} {
		if _, err := parseArgs(args); err == nil {
			t.Fatal(args)
		}
	}
}
