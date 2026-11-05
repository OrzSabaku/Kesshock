package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

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
