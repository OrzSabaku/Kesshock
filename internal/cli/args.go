package cli

import (
	"errors"
	"flag"
	"io"
	"time"
)

type commandOptions struct {
	Command, Operation, Date, Meal, HTML string
	Remark                               *string
	Apply                                bool
	Yes                                  bool
}

func parseArgs(args []string) (commandOptions, error) {
	o := commandOptions{}
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
