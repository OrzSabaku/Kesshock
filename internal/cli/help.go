package cli

import (
	"fmt"
	"io"
	"os/exec"
)

const usage = `Kesshock - 詫間寮 外泊・欠食 CLI（非公式）
使い方:
  kesshock login
  kesshock status [--html saved-form.html]
  kesshock kesshoku add|remove YYYY-MM-DD --meal morning|lunch|dinner|all [--remark 備考] [--apply] [--yes|-y]
  kesshock gaihaku add|remove YYYY-MM-DD [--remark 備考] [--apply] [--yes|-y]
  kesshock forget
--apply省略時は表示のみ。指定時は確認入力後に送信します。--yes / -yで確認入力を省略します。`

func callToilet(font string, filter string) (string, error) {
	cmd := exec.Command("toilet", "Kesshock", "-f", font, "-F", filter)

	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func printHelp(out io.Writer) {
	if logo, err := callToilet("pagga", "metal"); err == nil && logo != "" {
		fmt.Fprintln(out, logo)
		fmt.Fprintln(out)
	}
	fmt.Fprintln(out, usage)
}

func isHelpRequest(args []string) bool {
	if len(args) == 0 {
		return true
	}
	switch args[0] {
	case "--help", "-h", "help":
		return true
	}
	// Only recognize help before positional values, so a remark whose value is
	// "--help" can still be submitted unchanged.
	return len(args) == 2 && (args[1] == "--help" || args[1] == "-h")
}
