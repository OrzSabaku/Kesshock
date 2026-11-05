package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestHelp(t *testing.T) {
	for _, args := range [][]string{nil, {"--help"}, {"-h"}, {"help"}, {"login", "--help"}, {"status", "-h"}} {
		var out bytes.Buffer
		if err := Run(args, strings.NewReader(""), &out); err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(out.String(), logo+"\n\n") || !strings.Contains(out.String(), "kesshock login") {
			t.Fatalf("missing logo or command list for %v", args)
		}
	}
	for _, char := range logo {
		if char > 127 {
			t.Fatal("logo contains non-ASCII characters")
		}
	}
}
