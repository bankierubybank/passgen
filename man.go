package main

import (
	_ "embed"
	"fmt"
	"io"
	"strings"
)

// The man page ships inside the binary, so `passgen man` works for every
// install method. A test checks it documents every flag.
//
//go:embed passgen.1
var manPage string

func runMan(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintln(stderr, "usage: passgen man   (install: passgen man > /usr/local/share/man/man1/passgen.1)")
		return 2
	}
	// Stamp the real version into the header line.
	fmt.Fprint(stdout, strings.Replace(manPage, `"passgen dev"`, fmt.Sprintf("%q", "passgen "+version), 1))
	return 0
}
