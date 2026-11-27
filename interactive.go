package main

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// prompter reads answers line by line. Any read error (EOF, Ctrl-D) ends the session.
type prompter struct {
	in  *bufio.Reader
	out io.Writer
}

func (p *prompter) line(prompt string) (string, error) {
	fmt.Fprint(p.out, prompt)
	s, err := p.in.ReadString('\n')
	if err != nil && s == "" {
		return "", err
	}
	return strings.TrimSpace(s), nil
}

// askInt prompts until a number in [lo, hi] is given. Empty keeps def,
// pulled into range first (a command-line -l or -n may be outside it).
func (p *prompter) askInt(label string, def, lo, hi int) (int, error) {
	def = max(lo, min(def, hi))
	for {
		s, err := p.line(fmt.Sprintf("%s [%d]: ", label, def))
		if err != nil {
			return 0, err
		}
		if s == "" {
			return def, nil
		}
		v, convErr := strconv.Atoi(s)
		if convErr != nil {
			fmt.Fprintln(p.out, "  enter a number")
			continue
		}
		if v < lo || v > hi {
			fmt.Fprintf(p.out, "  must be %d-%d\n", lo, hi)
			continue
		}
		return v, nil
	}
}

// askChoice prompts until one of choices is given. Empty keeps def.
func (p *prompter) askChoice(label, def string, choices []string) (string, error) {
	hint := strings.Join(choices, "/")
	for {
		s, err := p.line(fmt.Sprintf("%s (%s) [%s]: ", label, hint, def))
		if err != nil {
			return "", err
		}
		if s == "" {
			return def, nil
		}
		s = strings.ToLower(s)
		for _, c := range choices {
			if s == c {
				return s, nil
			}
		}
		fmt.Fprintf(p.out, "  pick one of: %s\n", hint)
	}
}

// askYes prompts for y/n. Empty keeps def.
func (p *prompter) askYes(label string, def bool) (bool, error) {
	hint := "y/N"
	if def {
		hint = "Y/n"
	}
	for {
		s, err := p.line(fmt.Sprintf("%s [%s]: ", label, hint))
		if err != nil {
			return false, err
		}
		switch strings.ToLower(s) {
		case "":
			return def, nil
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		}
	}
}

// session holds the settings that interactive prompts can change.
type session struct {
	opts       Options
	complexity string
	count      int
}

// configure asks for every setting, offering the current values as defaults.
func (s *session) configure(p *prompter) error {
	var err error
	if s.opts.Length, err = p.askInt("Length", s.opts.Length, 1, 1024); err != nil {
		return err
	}
	if s.count, err = p.askInt("How many", s.count, 1, 100); err != nil {
		return err
	}
	fmt.Fprintln(p.out, "Complexity: low = a-z 0-9 | medium = + A-Z | high = + symbols")
	if s.complexity, err = p.askChoice("Complexity", s.complexity, complexityLevels); err != nil {
		return err
	}
	// The interactive answer replaces any per-set flags from the command line.
	_ = s.opts.ApplyPreset(s.complexity)
	if s.opts.Symbols {
		for _, name := range symbolModes {
			fmt.Fprintf(p.out, "  %-9s %s\n", name, symbolSets[name])
		}
		s.opts.SymbolSet = ""
		if _, ok := symbolSets[s.opts.SymbolMode]; !ok {
			s.opts.SymbolMode = "safe"
		}
		if s.opts.SymbolMode, err = p.askChoice("Symbol mode", s.opts.SymbolMode, symbolModes); err != nil {
			return err
		}
	}
	s.opts.NoAmbiguous, err = p.askYes("Exclude look-alike characters (Il1O0o etc)?", s.opts.NoAmbiguous)
	return err
}

func runInteractive(opts Options, complexity string, count int, stdin io.Reader, stdout, stderr io.Writer) int {
	if _, ok := presets[complexity]; !ok {
		complexity = "high"
	}
	if count < 1 {
		count = 1
	}
	s := &session{opts: opts, complexity: complexity, count: count}
	p := &prompter{in: bufio.NewReader(stdin), out: stdout}

	fmt.Fprintln(stdout, "passgen interactive. Enter keeps [default]. Ctrl-C to quit.")
	fmt.Fprintln(stdout)
	if err := s.configure(p); err != nil {
		fmt.Fprintln(stdout)
		return 0
	}

	for {
		if err := s.opts.Validate(); err != nil {
			fmt.Fprintf(stdout, "\n%v. Change settings.\n\n", err)
			if err := s.configure(p); err != nil {
				fmt.Fprintln(stdout)
				return 0
			}
			continue
		}

		fmt.Fprintln(stdout)
		passwords := make([]string, s.count)
		for i := range passwords {
			passwords[i] = Generate(s.opts)
			if s.count > 1 {
				fmt.Fprintf(stdout, "  %d) %s\n", i+1, passwords[i])
			} else {
				fmt.Fprintf(stdout, "  %s\n", passwords[i])
			}
		}
		fmt.Fprintf(stdout, "\n  %s\n", strengthLine(s.opts))

		copyHint := "c = copy"
		if len(passwords) > 1 {
			copyHint = fmt.Sprintf("c / 1-%d = copy", len(passwords))
		}

	menu:
		for {
			cmd, err := p.line(fmt.Sprintf("\n[Enter] = regenerate, %s, s = settings, q = quit: ", copyHint))
			if err != nil {
				fmt.Fprintln(stdout)
				return 0
			}
			switch cmd = strings.ToLower(cmd); cmd {
			case "":
				break menu
			case "q":
				return 0
			case "s":
				fmt.Fprintln(stdout)
				if err := s.configure(p); err != nil {
					fmt.Fprintln(stdout)
					return 0
				}
				break menu
			}
			idx := 0
			if cmd == "c" {
				idx = 1
			} else if n, convErr := strconv.Atoi(cmd); convErr == nil {
				idx = n
			}
			if idx < 1 || idx > len(passwords) {
				fmt.Fprintln(stdout, "  unknown command")
				continue
			}
			if err := copyToClipboard(passwords[idx-1]); err != nil {
				fmt.Fprintf(stderr, "  copy failed: %v\n", err)
				continue
			}
			fmt.Fprintf(stdout, "  copied #%d to clipboard\n", idx)
		}
	}
}
