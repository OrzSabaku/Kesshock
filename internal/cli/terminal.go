package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
)

func readLine(r *bufio.Reader) (string, error) {
	s, err := r.ReadString('\n')
	if err != nil && !(err == io.EOF && len(s) > 0) {
		return "", errors.New("入力を読み取れません。")
	}
	return strings.TrimSuffix(strings.TrimSuffix(s, "\n"), "\r"), nil
}

// stty operates on /dev/tty, so the password never falls back to echoed stdin.
func readPassword() (string, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return "", errors.New("パスワード入力には端末が必要です。")
	}
	defer tty.Close()
	run := func(args ...string) ([]byte, error) {
		cmd := exec.Command("stty", args...)
		cmd.Stdin = tty
		return cmd.Output()
	}
	saved, err := run("-g")
	if err != nil {
		return "", errors.New("端末設定を取得できません。sttyを確認してください。")
	}
	if _, err = run("-echo"); err != nil {
		return "", errors.New("パスワードの非表示入力を設定できません。")
	}
	interrupts := make(chan os.Signal, 1)
	done := make(chan struct{})
	signal.Notify(interrupts, os.Interrupt, syscall.SIGTERM)
	restore := func() { run(strings.TrimSpace(string(saved))); fmt.Fprintln(tty) }
	go func() {
		select {
		case <-interrupts:
			restore()
			os.Exit(130)
		case <-done:
		}
	}()
	defer func() { signal.Stop(interrupts); close(done); restore() }()
	fmt.Fprint(tty, "パスワード: ")
	return readLine(bufio.NewReader(tty))
}
