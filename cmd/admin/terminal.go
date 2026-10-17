package main

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

// The RTK proxy can capture standard streams even when the operator is in a
// terminal. Keep passwords and TOTP enrollment material on /dev/tty instead.
func privateTerminal() (func(), error) {
	if term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd())) && term.IsTerminal(int(os.Stderr.Fd())) {
		return func() {}, nil
	}
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("execute em um terminal privado interativo")
	}
	if !term.IsTerminal(int(tty.Fd())) {
		tty.Close()
		return nil, fmt.Errorf("terminal privado indisponível")
	}
	stdin, stdout, stderr := os.Stdin, os.Stdout, os.Stderr
	os.Stdin, os.Stdout, os.Stderr = tty, tty, tty
	return func() {
		os.Stdin, os.Stdout, os.Stderr = stdin, stdout, stderr
		tty.Close()
	}, nil
}
