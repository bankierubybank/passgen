// passgen generates secure random passwords locally, using the OS's
// cryptographic random source. It never touches the network, except
// when you ask it to with --check-update.
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

// cliFlags holds every option of the main command.
type cliFlags struct {
	length, count                     int
	complexity, symbolMode, symbolSet string
	exclude                           string
	lower, upper, digits, symbols     bool
	noLower, noUpper, noDigits, noSym bool
	noAmbiguous, noRequireEach        bool
	doCopy, quiet, interactive        bool
	listSymbols, showVersion          bool
	checkForUpdate                    bool
}

// subcommands, with a one-line description each (used by completion too).
var subcommands = [][2]string{
	{"check", "estimate the strength of a password from stdin"},
	{"completion", "print a shell completion script (bash, zsh, fish)"},
	{"man", "print the man page"},
}

// flagSpec is one option with its long and short names.
type flagSpec struct {
	long, short, usage string
	value              bool // takes an argument
}

// newFlagSet defines the main command's flags, and lists them for the
// completion scripts so those can't drift from what passgen accepts.
func newFlagSet(stderr io.Writer) (*flag.FlagSet, *cliFlags, []flagSpec) {
	fs := flag.NewFlagSet("passgen", flag.ContinueOnError)
	fs.SetOutput(stderr)
	f := &cliFlags{}
	var specs []flagSpec

	intFlag := func(p *int, short, long string, def int, usage string) {
		fs.IntVar(p, long, def, usage)
		fs.IntVar(p, short, def, usage)
		specs = append(specs, flagSpec{long, short, usage, true})
	}
	strFlag := func(p *string, short, long, def, usage string) {
		fs.StringVar(p, long, def, usage)
		if short != "" {
			fs.StringVar(p, short, def, usage)
		}
		specs = append(specs, flagSpec{long, short, usage, true})
	}
	boolFlag := func(p *bool, short, long, usage string) {
		fs.BoolVar(p, long, false, usage)
		if short != "" {
			fs.BoolVar(p, short, false, usage)
		}
		specs = append(specs, flagSpec{long, short, usage, false})
	}

	intFlag(&f.length, "l", "length", 16, "password length")
	intFlag(&f.count, "n", "count", 1, "number of passwords")
	strFlag(&f.complexity, "c", "complexity", "high", "preset: low, medium, high")
	strFlag(&f.symbolMode, "s", "symbol-mode", "safe", "symbol set: safe, standard, complex")
	strFlag(&f.symbolSet, "", "symbol-set", "", "custom symbol characters")
	strFlag(&f.exclude, "", "exclude", "", "characters to exclude")
	boolFlag(&f.lower, "", "lower", "include a-z")
	boolFlag(&f.upper, "", "upper", "include A-Z")
	boolFlag(&f.digits, "", "digits", "include 0-9")
	boolFlag(&f.symbols, "", "symbols", "include symbols")
	boolFlag(&f.noLower, "", "no-lower", "exclude a-z")
	boolFlag(&f.noUpper, "", "no-upper", "exclude A-Z")
	boolFlag(&f.noDigits, "", "no-digits", "exclude 0-9")
	boolFlag(&f.noSym, "", "no-symbols", "exclude symbols")
	boolFlag(&f.noAmbiguous, "", "no-ambiguous", "exclude look-alike characters")
	boolFlag(&f.noRequireEach, "", "no-require-each", "don't force one char from each set")
	boolFlag(&f.doCopy, "", "copy", "copy the first password to the clipboard")
	boolFlag(&f.quiet, "q", "quiet", "print passwords only")
	boolFlag(&f.interactive, "i", "interactive", "interactive mode")
	boolFlag(&f.listSymbols, "", "list-symbols", "show symbol sets")
	boolFlag(&f.showVersion, "v", "version", "show version")
	boolFlag(&f.checkForUpdate, "", "check-update", "check GitHub for a newer release")
	return fs, f, specs
}

// run is main without the process exit, so tests can drive it.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "check":
			return runCheck(args[1:], stdin, stdout, stderr)
		case "completion":
			return runCompletion(args[1:], stdout, stderr)
		case "man":
			return runMan(args[1:], stdout, stderr)
		}
	}

	fs, f, _ := newFlagSet(stderr)
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

	if f.checkForUpdate {
		return checkUpdate(stdout, stderr, nil)
	}
	if f.showVersion {
		fmt.Fprintln(stdout, "passgen", version)
		return 0
	}
	if f.listSymbols {
		for _, name := range symbolModes {
			fmt.Fprintf(stdout, "%-9s %s\n", name, symbolSets[name])
		}
		return 0
	}

	opts := Options{
		Length:      f.length,
		SymbolMode:  f.symbolMode,
		SymbolSet:   f.symbolSet,
		NoAmbiguous: f.noAmbiguous,
		Exclude:     f.exclude,
		RequireEach: !f.noRequireEach,
	}
	if err := opts.ApplyPreset(f.complexity); err != nil {
		return fail(stderr, err)
	}
	// Per-set flags override the preset. Only flags the user actually gave count.
	fs.Visit(func(fl *flag.Flag) {
		switch fl.Name {
		case "lower":
			opts.Lower = f.lower
		case "upper":
			opts.Upper = f.upper
		case "digits":
			opts.Digits = f.digits
		case "symbols":
			opts.Symbols = f.symbols
		case "no-lower":
			opts.Lower = !f.noLower
		case "no-upper":
			opts.Upper = !f.noUpper
		case "no-digits":
			opts.Digits = !f.noDigits
		case "no-symbols":
			opts.Symbols = !f.noSym
		}
	})

	if f.interactive {
		if f.doCopy || f.quiet {
			fmt.Fprintln(stderr, "passgen: --copy and --quiet don't apply to -i (press c to copy)")
		}
		return runInteractive(opts, f.complexity, f.count, stdin, stdout, stderr)
	}

	if f.count < 1 || f.count > maxCount {
		return fail(stderr, fmt.Errorf("count must be 1-%d", maxCount))
	}
	if err := opts.Validate(); err != nil {
		return fail(stderr, err)
	}

	passwords := make([]string, f.count)
	for i := range passwords {
		passwords[i] = Generate(opts)
		fmt.Fprintln(stdout, passwords[i])
	}
	if !f.quiet {
		fmt.Fprintf(stderr, "\n%s\n", strengthLine(opts))
	}
	if f.doCopy {
		if err := copyToClipboard(passwords[0]); err != nil {
			return fail(stderr, err)
		}
		if !f.quiet {
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
  passgen check           estimate a password's strength (reads stdin)
  passgen completion SHELL  print a completion script: bash, zsh, fish
  passgen man             print the man page

Options:
  -l, --length N          password length, 1-4096 (default 16)
  -n, --count N           number of passwords, 1-10000 (default 1)
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
      --check-update      check GitHub for a newer release (the only network use)
  -h, --help              show this help

Examples:
  passgen -l 24 -n 5
  passgen -c medium --no-ambiguous
  passgen -s complex -l 32 --copy
  pbpaste | passgen check
`, version, sym.String(), ambiguousChars)
}
