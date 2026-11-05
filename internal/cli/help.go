package cli

import (
	"fmt"
	"io"
)

// Generated with: toilet -f ascii9 -w 120 Kesshock
// Embed the result so displaying help never requires toilet on the user's machine.
const logo = ` m    m                      #                    #
 #  m"   mmm    mmm    mmm   # mm    mmm    mmm   #   m
 #m#    #"  #  #   "  #   "  #"  #  #" "#  #"  "  # m"
 #  #m  #""""   """m   """m  #   #  #   #  #      #"#
 #   "m "#mm"  "mmm"  "mmm"  #   #  "#m#"  "#mm"  #  "m`

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
