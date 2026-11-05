package cli

import (
	"fmt"
	"io"
)

const logo = `  K   K  EEEEE   SSSS   SSSS  H   H   OOO    CCCC  K   K
  K  K   E      S      S      H   H  O   O  C      K  K
  KKK    EEEE    SSS    SSS   HHHHH  O   O  C      KKK
  K  K   E          S      S  H   H  O   O  C      K  K
  K   K  EEEEE  SSSS   SSSS   H   H   OOO    CCCC  K   K`

const usage = `Kesshock - 詫間寮 外泊・欠食 CLI（非公式）
使い方:
  kesshock login
  kesshock status [--html saved-form.html]
  kesshock kesshoku add|remove YYYY-MM-DD --meal morning|lunch|dinner|all [--remark 備考] [--apply] [--yes|-y]
  kesshock gaihaku add|remove YYYY-MM-DD [--remark 備考] [--apply] [--yes|-y]
  kesshock forget
--apply省略時は表示のみ。指定時は確認入力後に送信します。--yes / -yで確認入力を省略します。`

func printHelp(out io.Writer) {
	fmt.Fprintln(out, logo)
	fmt.Fprintln(out)
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
