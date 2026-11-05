package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// Read only the current directory's .env, without changing process environment
// or evaluating shell commands, variable substitutions, or escapes.
func credentialsFromDotEnv(path string) (user, password string, found bool, err error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", true, errors.New(".envを読み取れません。")
	}
	defer f.Close()
	values := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(scanner.Text())
		if line == 1 {
			text = strings.TrimPrefix(text, "\ufeff")
		}
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		text = strings.TrimSpace(strings.TrimPrefix(text, "export "))
		key, raw, ok := strings.Cut(text, "=")
		if !ok {
			return "", "", true, fmt.Errorf(".envの%d行目はKEY=VALUE形式で指定してください。", line)
		}
		key = strings.TrimSpace(key)
		// Ignore unrelated settings, including values with other dotenv syntax.
		if key != "TAKUMA_STUDENT_ID" && key != "TAKUMA_PASSWORD" {
			continue
		}
		value, valid := dotenvValue(raw)
		if !valid {
			return "", "", true, fmt.Errorf(".envの%d行目の引用符が不正です。", line)
		}
		values[key] = value
	}
	if scanner.Err() != nil {
		return "", "", true, errors.New(".envを読み取れません。")
	}
	user, password = values["TAKUMA_STUDENT_ID"], values["TAKUMA_PASSWORD"]
	if strings.TrimSpace(user) == "" || password == "" {
		return "", "", true, errors.New(".envにTAKUMA_STUDENT_IDとTAKUMA_PASSWORDの両方を設定してください。")
	}
	return user, password, true, nil
}

func dotenvValue(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	if len(s) > 0 && (s[0] == '\'' || s[0] == '"') {
		end := strings.IndexByte(s[1:], s[0])
		if end < 0 {
			return "", false
		}
		end++
		rest := strings.TrimSpace(s[end+1:])
		if rest != "" && !strings.HasPrefix(rest, "#") {
			return "", false
		}
		return s[1:end], true
	}
	for i := 1; i < len(s); i++ {
		if s[i] == '#' && (s[i-1] == ' ' || s[i-1] == '\t') {
			return strings.TrimSpace(s[:i]), true
		}
	}
	return s, true
}

func loginCredentials(path string, in *bufio.Reader, out io.Writer, secret func() (string, error)) (string, string, error) {
	user, password, found, err := credentialsFromDotEnv(path)
	if err != nil || found {
		return user, password, err
	}
	fmt.Fprint(out, "学籍番号: ")
	user, err = readLine(in)
	if err != nil {
		return "", "", err
	}
	password, err = secret()
	return user, password, err
}
