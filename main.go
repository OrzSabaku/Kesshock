package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const usage = `詫間寮 外泊・欠食 CLI（非公式）
使い方:
  takuma login
  takuma status [--html saved-form.html]
  takuma kesshoku add|remove YYYY-MM-DD --meal morning|lunch|dinner|all [--remark 備考] [--apply] [--yes|-y]
  takuma gaihaku add|remove YYYY-MM-DD [--remark 備考] [--apply] [--yes|-y]
  takuma forget
--apply省略時は表示のみ。指定時は確認入力後に送信します。--yes / -yで確認入力を省略します。`

type options struct {
	Command, Operation, Date, Meal, HTML string
	Remark                               *string
	Apply                                bool
	Yes                                  bool
}

func parseArgs(args []string) (options, error) {
	o := options{}
	if len(args) == 0 {
		return o, errors.New(usage)
	}
	o.Command = args[0]
	args = args[1:]
	fs := flag.NewFlagSet(o.Command, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	switch o.Command {
	case "login", "forget":
		if len(args) != 0 {
			return o, errors.New("余分な引数があります。")
		}
		return o, nil
	case "status":
		fs.StringVar(&o.HTML, "html", "", "保存HTML")
	case "kesshoku", "gaihaku":
		if len(args) < 2 {
			return o, errors.New(usage)
		}
		o.Operation = args[0]
		if o.Operation != "add" && o.Operation != "remove" {
			return o, errors.New("操作はaddまたはremoveを指定してください。")
		}
		d, err := time.Parse("2006-01-02", args[1])
		if err != nil {
			return o, errors.New("日付はYYYY-MM-DDで指定してください。")
		}
		o.Date = d.Format("2006/01/02")
		args = args[2:]
		if o.Command == "kesshoku" {
			fs.StringVar(&o.Meal, "meal", "", "morning/lunch/dinner/all")
		}
		fs.BoolVar(&o.Apply, "apply", false, "登録する")
		fs.BoolVar(&o.Yes, "yes", false, "確認入力を省略する")
		fs.BoolVar(&o.Yes, "y", false, "確認入力を省略する")
		fs.Func("remark", "備考", func(v string) error { o.Remark = &v; return nil })
	default:
		return o, errors.New(usage)
	}
	if err := fs.Parse(args); err != nil {
		return o, errors.New("オプションが不正です。\n" + usage)
	}
	if fs.NArg() != 0 {
		return o, errors.New("余分な引数があります。")
	}
	if o.Command == "kesshoku" && o.Meal != "morning" && o.Meal != "lunch" && o.Meal != "dinner" && o.Meal != "all" {
		return o, errors.New("--meal morning/lunch/dinner/allを指定してください。")
	}
	return o, nil
}
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
func show(w io.Writer, rows []row) {
	fmt.Fprintln(w, "Date        B  L  D  O  Remarks")
	fmt.Fprintln(w, "----------  -  -  -  -  -------")
	fmt.Fprintln(w, "B/L/D: breakfast/lunch/dinner; O: overnight")
	fmt.Fprintln(w, "X: requested; .: not requested; -: unavailable")
	for _, r := range rows {
		values := []string{}
		for _, field := range fieldNames {
			v, ok := r.Fields[field]
			s := "."
			if !ok {
				s = "-"
			} else if v.Checked {
				s = "X"
			}
			values = append(values, s)
		}
		fmt.Fprintln(w, r.Date+"  "+strings.Join(values, "  ")+"  "+strconv.QuoteToASCII(r.Remark))
	}
}
func run(args []string, in io.Reader, out io.Writer) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		fmt.Fprintln(out, usage)
		return nil
	}
	o, err := parseArgs(args)
	if err != nil {
		return err
	}
	if o.Command == "status" && o.HTML != "" {
		b, err := os.ReadFile(o.HTML)
		if err != nil {
			return errors.New("HTMLファイルを読めません。")
		}
		f, err := parseForm(string(b), "regist")
		if err != nil {
			return err
		}
		rows, err := formState(f)
		if err != nil {
			return err
		}
		show(out, rows)
		return nil
	}
	if o.Command == "forget" {
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
	c, err := newClient()
	if err != nil {
		return err
	}
	reader := bufio.NewReader(in)
	if o.Command == "login" {
		s, err := c.request(loginURL, nil)
		if err != nil {
			return err
		}
		f, err := parseForm(s, "login")
		if err != nil {
			return err
		}
		userField, passwordField := false, false
		for _, a := range f.Inputs {
			if a["name"] == "gakusekiNo" && !has(a, "disabled") {
				userField = true
			}
			if a["name"] == "loginPass" && !has(a, "disabled") {
				passwordField = true
			}
		}
		if !userField || !passwordField {
			return errors.New("ログイン入力欄が見つかりません。画面変更の可能性があります。")
		}
		user, password, err := loginCredentials(".env", reader, out, readPassword)
		if err != nil {
			return err
		}
		for _, a := range f.Inputs {
			switch a["name"] {
			case "gakusekiNo":
				a["value"] = user
			case "loginPass":
				a["value"] = password
			case "rememberMe":
				delete(a, "checked")
			}
		}
		target, err := resolveURL(loginURL, f.Action)
		if err != nil {
			return err
		}
		s, err = c.request(target.String(), f.payload())
		if err != nil {
			return err
		}
		if err = c.confirmOTP(s, reader, out); err != nil {
			return err
		}
		if _, err = c.registration(); err != nil {
			return err
		}
		fmt.Fprintln(out, "ログインを確認しました。")
		return nil
	}
	f, err := c.registration()
	if err != nil {
		return err
	}
	if o.Command == "status" {
		rows, err := formState(f)
		if err != nil {
			return err
		}
		show(out, rows)
		return nil
	}
	fields := []string{fieldNames[3]}
	if o.Command == "kesshoku" {
		switch o.Meal {
		case "morning":
			fields = fieldNames[:1]
		case "lunch":
			fields = fieldNames[1:2]
		case "dinner":
			fields = fieldNames[2:3]
		case "all":
			fields = fieldNames[:3]
		}
	}
	before, after, err := change(f, o.Date, fields, o.Operation == "add", o.Remark)
	if err != nil {
		return err
	}
	if sameState(before, after) {
		fmt.Fprintln(out, "変更はありません。")
		return nil
	}
	fmt.Fprintln(out, "変更前:")
	for _, r := range before {
		if r.Date == o.Date {
			show(out, []row{r})
		}
	}
	fmt.Fprintln(out, "変更後:")
	for _, r := range after {
		if r.Date == o.Date {
			show(out, []row{r})
		}
	}
	if !o.Apply {
		fmt.Fprintln(out, "表示のみ。登録するには同じコマンドに --apply を付けてください。")
		return nil
	}
	if !o.Yes {
		fmt.Fprint(out, "この変更を登録しますか？ [y/N]: ")
		answer, err := readLine(reader)
		if err != nil {
			return err
		}
		if strings.ToLower(answer) != "y" {
			return nil
		}
	}
	latest, err := c.registration()
	if err != nil {
		return err
	}
	current, err := formState(latest)
	if err != nil {
		return err
	}
	if !sameState(current, before) {
		return errors.New("確認中に状態が変わりました。再実行してください。")
	}
	if _, _, err = change(latest, o.Date, fields, o.Operation == "add", o.Remark); err != nil {
		return err
	}
	target, err := resolveURL(registURL, latest.Action)
	if err != nil {
		return err
	}
	if _, err = c.request(target.String(), latest.payload()); err != nil {
		return errors.New("送信結果を確認できません。自動再送しません。statusまたはブラウザで確認してください。")
	}
	actualForm, err := c.registration()
	if err != nil {
		return errors.New("送信結果を確認できません。自動再送しません。statusまたはブラウザで確認してください。")
	}
	actual, err := formState(actualForm)
	if err != nil || !sameState(actual, after) {
		return errors.New("再取得した状態が一致しません。ブラウザで登録結果を確認してください。自動再送しません。")
	}
	fmt.Fprintln(out, "登録後の状態を再取得し、一致を確認しました。")
	return nil
}
func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "エラー:", err)
		os.Exit(1)
	}
}
