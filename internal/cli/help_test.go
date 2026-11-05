package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHelp(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	for _, args := range [][]string{nil, {"--help"}, {"-h"}, {"help"}, {"login", "--help"}, {"status", "-h"}} {
		var out bytes.Buffer
		if err := Run(args, strings.NewReader(""), &out); err != nil {
			t.Fatal(err)
		}
		if out.String() != usage+"\n" {
			t.Fatalf("unexpected help without toilet for %v: %q", args, out.String())
		}
	}

}

func TestHelpWithToilet(t *testing.T) {
	for _, tc := range []struct {
		name, script, want string
	}{
		{"success", "printf 'test logo'", "test logo\n\n" + usage + "\n"},
		{"failure", "echo failed >&2; exit 1", usage + "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "toilet"), []byte("#!/bin/sh\n"+tc.script+"\n"), 0755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir)
			var out bytes.Buffer
			printHelp(&out)
			if out.String() != tc.want {
				t.Fatalf("unexpected help: %q", out.String())
			}
		})
	}
}
