package main

import (
	_ "embed"
	"fmt"
	"io"
	"runtime/debug"
	"strings"
)

// The man page ships inside the binary, so `passgen man` works for every
// install method. passgen.1 is a template: @VERSION@ and @DATE@ are filled
// in here. A test checks it documents every flag.
//
//go:embed passgen.1
var manPage string

func runMan(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintln(stderr, "usage: passgen man   (install: passgen man > /usr/local/share/man/man1/passgen.1)")
		return 2
	}
	fmt.Fprint(stdout, strings.NewReplacer("@VERSION@", version, "@DATE@", buildDate()).Replace(manPage))
	return 0
}

// buildDate is the date of the commit this binary was built from (Go records
// it when building inside a git checkout), so the man page date is right
// and builds stay reproducible. Empty when unknown, e.g. after go install.
func buildDate() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.time" && len(s.Value) >= 10 {
				return s.Value[:10]
			}
		}
	}
	return ""
}
