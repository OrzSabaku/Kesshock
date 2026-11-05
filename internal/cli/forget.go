package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func forgetCookies(out io.Writer) error {
	path, err := cookiePath()
	if err != nil {
		return err
	}
	for _, p := range []string{path, filepath.Join(filepath.Dir(path), "cookies.txt")} {
		if err = os.Remove(p); err != nil && !os.IsNotExist(err) {
			return errors.New("Cookieを削除できません。")
		}
	}
	fmt.Fprintln(out, "ローカルCookieを削除しました。")
	return nil
}
