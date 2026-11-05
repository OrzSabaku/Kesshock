package cli

import (
	"bufio"
	"fmt"
	"io"
)

// Run executes one command using the supplied input and output streams.
// It returns errors to the caller so only the executable decides when to exit.
func Run(args []string, in io.Reader, out io.Writer) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		fmt.Fprintln(out, usage)
		return nil
	}
	opts, err := parseArgs(args)
	if err != nil {
		return err
	}
	if opts.Command == "forget" {
		return forgetCookies(out)
	}
	if opts.Command == "status" && opts.HTML != "" {
		return showSavedStatus(opts.HTML, out)
	}

	session, err := newClient()
	if err != nil {
		return err
	}
	reader := bufio.NewReader(in)
	if opts.Command == "login" {
		return session.login(reader, out, readPassword)
	}
	registrationForm, err := session.fetchRegistration()
	if err != nil {
		return err
	}
	if opts.Command == "status" {
		return showStatus(registrationForm, out)
	}
	return session.applyChange(opts, registrationForm, reader, out)
}
