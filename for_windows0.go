//go:build !windows

package main

import (
	"fmt"
	"net/url"
	"os"
)

func netUse(_ *url.URL, _ bool) error { return nil }

func clearConsole() {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return
	}
	if fi.Mode()&os.ModeCharDevice == 0 {
		return
	}
	fmt.Fprint(os.Stdout, "\x1b[2J\x1b[3J\x1b[H")
}
