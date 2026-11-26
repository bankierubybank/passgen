// passgen generates secure random passwords locally, using the OS's
// cryptographic random source. It never touches the network.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

// version is set at build time: -ldflags "-X main.version=v1.2.3".
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run is main without the process exit, so tests can drive it.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("passgen", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		length, count                     int
		complexity, symbolMode, symbolSet string
		exclude                           string
		lower, upper, digits, symbols     bool
		noLower, noUpper, noDigits, noSym bool
		noAmbiguous, noRequireEach        bool
		doCopy, quiet, interactive        bool
		listSymbols, showVersion          bool
	)
	intFlag := func(p *int, short, long string, def int, usage string) {
		fs.IntVar(p, long, def, usage)
		fs.IntVar(p, short, def, usage)
	}
	strFlag := func(p *string, short, long, def, usage string) {
		fs.StringVar(p, long, def, usage)
		if short != "" {
			fs.StringVar(p, short, def, usage)
		}
	}
	boolFlag := func(p *bool, short, long, usage string) {
		fs.BoolVar(p, long, false, usage)
		if short != "" {
			fs.BoolVar(p, short, false, usage)
		}
	}

	intFlag(&length, "l", "length", 16, "password length")
	intFlag(&count, "n", "count", 1, "number of passwords")
	strFlag(&complexity, "c", "complexity", "high", "preset: low, medium, high")
	strFlag(&symbolMode, "s", "symbol-mode", "safe", "symbol set: safe, standard, complex")
	strFlag(&symbolSet, "", "symbol-set", "", "custom symbol characters")
	strFlag(&exclude, "", "exclude", "", "characters to exclude")
	boolFlag(&lower, "", "lower", "include a-z")
	boolFlag(&upper, "", "upper", "include A-Z")
	boolFlag(&digits, "", "digits", "include 0-9")
	boolFlag(&symbols, "", "symbols", "include symbols")
	boolFlag(&noLower, "", "no-lower", "exclude a-z")
	boolFlag(&noUpper, "", "no-upper", "exclude A-Z")
	boolFlag(&noDigits, "", "no-digits", "exclude 0-9")
	boolFlag(&noSym, "", "no-symbols", "exclude symbols")
	boolFlag(&noAmbiguous, "", "no-ambiguous", "exclude look-alike characters")
	boolFlag(&noRequireEach, "", "no-require-each", "don't force one char from each set")
	boolFlag(&doCopy, "", "copy", "copy the first password to the clipboard")
	boolFlag(&quiet, "q", "quiet", "print passwords only")
	boolFlag(&interactive, "i", "interactive", "interactive mode")
	boolFlag(&listSymbols, "", "list-symbols", "show symbol sets")
	boolFlag(&showVersion, "v", "version", "show version")

	fs.Usage = func() { printUsage(stderr) }

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "passgen: unexpected argument %q (see passgen --help)\n", fs.Arg(0))
		return 2
	}

	if showVersion {
		fmt.Fprintln(stdout, "passgen", version)
		return 0
	}
	if listSymbols {
		for _, name := range symbolModes {
			fmt.Fprintf(stdout, "%-9s %s\n", name, symbolSets[name])
		}
		return 0
	}

	opts := Options{
		Length:      length,
		SymbolMode:  symbolMode,
		SymbolSet:   symbolSet,
		NoAmbiguous: noAmbiguous,
		Exclude:     exclude,
		RequireEach: !noRequireEach,
	}
	if err := opts.ApplyPreset(complexity); err != nil {
		return fail(stderr, err)
	}
	// Per-set flags override the preset. Only flags the user actually gave count.
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "lower":
			opts.Lower = lower
		case "upper":
			opts.Upper = upper
		case "digits":
			opts.Digits = digits
		case "symbols":
			opts.Symbols = symbols
		case "no-lower":
			opts.Lower = !noLower
		case "no-upper":
			opts.Upper = !noUpper
		case "no-digits":
			opts.Digits = !noDigits
		case "no-symbols":
			opts.Symbols = !noSym
		}
	})

	if interactive {
		return runInteractive(opts, complexity, count, stdin, stdout, stderr)
	}

	if count < 1 {
		return fail(stderr, errors.New("count must be at least 1"))
	}
	if err := opts.Validate(); err != nil {
		return fail(stderr, err)
	}

	passwords := make([]string, count)
	for i := range passwords {
		passwords[i] = Generate(opts)
		fmt.Fprintln(stdout, passwords[i])
	}
	if !quiet {
		fmt.Fprintf(stderr, "\n%s\n", strengthLine(opts))
	}
	if doCopy {
		if err := copyToClipboard(passwords[0]); err != nil {
			return fail(stderr, err)
		}
		if !quiet {
			fmt.Fprintln(stderr, "[copied to clipboard]")
		}
	}
	return 0
}

func fail(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "passgen: %v\n", err)
	return 1
}

func printUsage(w io.Writer) {
	var sym strings.Builder
	for _, name := range symbolModes {
		fmt.Fprintf(&sym, "                          %-9s %s\n", name, symbolSets[name])
	}
	fmt.Fprintf(w, `passgen %s - generate secure random passwords locally

Usage:
  passgen [options]
  passgen -i              interactive mode

Options:
  -l, --length N          password length (default 16)
  -n, --count N           number of passwords (default 1)
  -c, --complexity LEVEL  low = a-z 0-9, medium = + A-Z, high = + symbols (default high)
  -s, --symbol-mode MODE  symbol set (default safe):
%s      --symbol-set CHARS  custom symbols (overrides --symbol-mode)
      --lower, --no-lower       include/exclude a-z     (override preset)
      --upper, --no-upper       include/exclude A-Z
      --digits, --no-digits     include/exclude 0-9
      --symbols, --no-symbols   include/exclude symbols
      --no-ambiguous      exclude look-alikes: %s
      --exclude CHARS     extra characters to exclude
      --no-require-each   don't force one char from each enabled set
      --copy              copy the first password to the clipboard
  -q, --quiet             print passwords only, no strength info
  -i, --interactive       prompt for options, then regenerate/copy in a loop
      --list-symbols      show symbol sets and exit
  -v, --version           show version
  -h, --help              show this help

Examples:
  passgen -l 24 -n 5
  passgen -c medium --no-ambiguous
  passgen -s complex -l 32 --copy
`, version, sym.String(), ambiguousChars)
}
