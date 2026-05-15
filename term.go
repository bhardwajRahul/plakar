//go:build !windows

package main

import (
	"os"

	"github.com/charmbracelet/x/term"
)

func isTerminal() bool {
	if t, ok := os.LookupEnv("TERM"); ok && t == "dumb" {
		return false
	}

	return term.IsTerminal(1)
}
