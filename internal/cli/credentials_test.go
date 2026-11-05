package cli

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDotEnvCredentials(t *testing.T) {
	for _, tc := range []struct {
		name, text, user, pass string
		invalid                bool
	}{
		{name: "plain", text: "TAKUMA_STUDENT_ID=012345\nTAKUMA_PASSWORD=p=a#ss$word\\x", user: "012345", pass: `p=a#ss$word\x`},
		{name: "quoted", text: "# comment\r\nexport TAKUMA_STUDENT_ID = \"012345\"\r\nTAKUMA_PASSWORD=' p# $word\\x ' # comment", user: "012345", pass: ` p# $word\x `},
		{name: "comments", text: "TAKUMA_STUDENT_ID=012345 # id\nTAKUMA_PASSWORD=test # password\nOTHER=anything", user: "012345", pass: "test"},
		{name: "missing", text: "TAKUMA_STUDENT_ID=012345", invalid: true},
		{name: "empty", text: "TAKUMA_STUDENT_ID=012345\nTAKUMA_PASSWORD=''", invalid: true},
		{name: "unclosed", text: "TAKUMA_STUDENT_ID=012345\nTAKUMA_PASSWORD='private-value", invalid: true},
		{name: "trailing", text: "TAKUMA_STUDENT_ID=012345\nTAKUMA_PASSWORD='private-value' garbage", invalid: true},
		{name: "malformed", text: "private-value", invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), ".env")
			if err := os.WriteFile(path, []byte(tc.text), 0600); err != nil {
				t.Fatal(err)
			}
			user, pass, found, err := credentialsFromDotEnv(path)
			if !found || (err != nil) != tc.invalid {
				t.Fatalf("found=%v err=%v", found, err)
			}
			if err != nil {
				if strings.Contains(err.Error(), "private-value") {
					t.Fatal("secret leaked")
				}
				return
			}
			if user != tc.user || pass != tc.pass {
				t.Fatal("credentials differ")
			}
		})
	}
}
func TestLoginCredentialSelection(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	var out bytes.Buffer
	calls := 0
	secret := func() (string, error) { calls++; return "interactive-password", nil }
	user, pass, err := loginCredentials(path, bufio.NewReader(strings.NewReader("interactive-user\n")), &out, secret)
	if err != nil || user != "interactive-user" || pass != "interactive-password" || calls != 1 {
		t.Fatal("interactive fallback failed", err)
	}
	if err = os.WriteFile(path, []byte("TAKUMA_STUDENT_ID=file-user\nTAKUMA_PASSWORD=file-password"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TAKUMA_STUDENT_ID", "environment-user")
	t.Setenv("TAKUMA_PASSWORD", "environment-password")
	out.Reset()
	user, pass, err = loginCredentials(path, bufio.NewReader(strings.NewReader("")), &out, secret)
	if err != nil || user != "file-user" || pass != "file-password" || calls != 1 || out.Len() != 0 {
		t.Fatal("file precedence failed", err)
	}
	if err = os.WriteFile(path, []byte("TAKUMA_STUDENT_ID=file-user"), 0600); err != nil {
		t.Fatal(err)
	}
	_, _, err = loginCredentials(path, bufio.NewReader(strings.NewReader("")), &out, secret)
	if err == nil || calls != 1 {
		t.Fatal("incomplete file should fail without prompting")
	}
}
