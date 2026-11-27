package main

// passgen check: estimate how strong an existing password is, offline.
//
// The estimate starts from brute force (length x log2(pool)), then charges
// far less for the parts a cracking tool tries first: common passwords and
// words (also with l33t swaps), repeated characters, sequences like "abcd"
// or "4321", and keyboard runs like "qwerty". It is an upper bound: real
// tools use much bigger word lists and rules, so treat it as a best case.

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Exit code when the password is below --min-bits.
const exitBelowMin = 3

// commonWords are passwords and words found at the top of every leaked
// password list. A password built on one of these is cracked in seconds,
// however long the rest of it is.
var commonWords = []string{
	"password", "passwd", "pass", "qwerty", "letmein", "welcome", "admin",
	"administrator", "root", "login", "master", "secret", "default", "guest",
	"user", "test", "changeme", "iloveyou", "love", "lovely", "hello", "hi",
	"monkey", "dragon", "shadow", "sunshine", "princess", "football",
	"baseball", "soccer", "hockey", "basketball", "superman", "batman",
	"starwars", "pokemon", "ninja", "mustang", "jordan", "michael", "charlie",
	"jessica", "ashley", "daniel", "thomas", "robert", "matthew", "andrew",
	"joshua", "hunter", "ranger", "killer", "trustno", "freedom", "whatever",
	"computer", "internet", "service", "access", "flower", "summer", "winter",
	"spring", "autumn", "january", "february", "march", "april", "june",
	"july", "august", "september", "october", "november", "december",
	"monday", "friday", "sunday", "cookie", "cheese", "chocolate", "banana",
	"orange", "apple", "pepper", "ginger", "tiger", "lion", "eagle", "falcon",
	"angel", "devil", "money", "god", "jesus", "blessed", "family", "friend",
	"happy", "lucky", "magic", "music", "pussy", "sexy", "fuck", "asshole",
	"thailand", "bangkok", "london", "paris", "america", "google",
	"facebook", "microsoft", "samsung", "iphone", "android",
	"abc", "abcd", "abcdef", "qwertyuiop", "asdf", "asdfgh", "zxcvbn",
	"qazwsx", "q1w2e3", "1q2w3e", "aaaa", "test123",
	"123456", "1234567", "12345678", "123456789", "1234567890", "111111",
	"000000", "123123", "654321", "112233", "121212", "696969", "1234",
}

var commonSet = func() map[string]bool {
	m := map[string]bool{}
	for _, w := range commonWords {
		m[w] = true
	}
	return m
}()

// Keyboard rows, for spotting runs like "qwer" or "asdf".
var keyboardRows = []string{"`1234567890-=", "qwertyuiop[]\\", "asdfghjkl;'", "zxcvbnm,./"}

// l33t swaps undone before the word check: p@ssw0rd -> password.
var unleet = map[rune]rune{'0': 'o', '1': 'i', '3': 'e', '4': 'a', '5': 's', '7': 't', '@': 'a', '$': 's', '!': 'i'}

// Strength is the result of checking one password.
type Strength struct {
	Length   int
	Classes  []string // kinds of characters used
	Pool     int      // brute-force pool those kinds imply
	Bits     float64  // estimated entropy
	Warnings []string
}

// charClass names the kind of a character and the size of its pool.
func charClass(r rune) (string, int) {
	switch {
	case r >= 'a' && r <= 'z':
		return "lowercase", 26
	case r >= 'A' && r <= 'Z':
		return "uppercase", 26
	case r >= '0' && r <= '9':
		return "digits", 10
	case r < utf8.RuneSelf:
		return "symbols", 33 // printable ASCII specials, plus space
	default:
		return "other", 100 // non-ASCII: a rough guess
	}
}

func keyboardPos(r rune) (row, col int) {
	r = unicode.ToLower(r)
	for i, k := range keyboardRows {
		if j := strings.IndexRune(k, r); j >= 0 {
			return i, j
		}
	}
	return -1, -1
}

// patternStep reports whether b continues a sequence or keyboard run from a.
func patternStep(a, b rune) (kind string, ok bool) {
	la, lb := unicode.ToLower(a), unicode.ToLower(b)
	ca, _ := charClass(la)
	cb, _ := charClass(lb)
	if ca == cb && (ca == "lowercase" || ca == "digits") && (lb-la == 1 || la-lb == 1) {
		return "sequence", true
	}
	ra, ia := keyboardPos(a)
	rb, ib := keyboardPos(b)
	if ra >= 0 && ra == rb && (ib-ia == 1 || ia-ib == 1) {
		return "keyboard run", true
	}
	return "", false
}

// commonCore finds the longest common password or word inside pw, ignoring
// case and l33t swaps (p@ssw0rd), so "MyPassword2024!" is caught too. It
// returns the word and the rune range it covers in pw.
func commonCore(pw []rune) (word string, start, end int, ok bool) {
	lower := make([]rune, len(pw))
	plain := make([]rune, len(pw)) // lowered, l33t undone; same length as pw
	for i, r := range pw {
		lower[i] = unicode.ToLower(r)
		plain[i] = lower[i]
		if u, isLeet := unleet[lower[i]]; isLeet {
			plain[i] = u
		}
	}
	for _, text := range []string{string(lower), string(plain)} {
		for _, w := range commonWords {
			n := len([]rune(w))
			// Short words only count as the whole password: "god" inside a
			// random string is luck, not a pattern.
			if n < 4 && n != len(pw) {
				continue
			}
			if n <= len([]rune(word)) {
				continue
			}
			if i := strings.Index(text, w); i >= 0 {
				word = w
				start = utf8.RuneCountInString(text[:i])
				end = start + n
				ok = true
			}
		}
	}
	return word, start, end, ok
}

// CheckStrength estimates the entropy of pw.
func CheckStrength(pw string) Strength {
	runes := []rune(pw)
	s := Strength{Length: len(runes)}
	if len(runes) == 0 {
		s.Warnings = append(s.Warnings, "empty password")
		return s
	}

	seen := map[string]int{}
	for _, r := range runes {
		name, size := charClass(r)
		seen[name] = size
	}
	for _, name := range []string{"lowercase", "uppercase", "digits", "symbols", "other"} {
		if size, ok := seen[name]; ok {
			s.Classes = append(s.Classes, name)
			s.Pool += size
		}
	}
	perChar := math.Log2(float64(s.Pool))

	// Cost of each character. Guessable continuations cost about 1 bit.
	cost := make([]float64, len(runes))
	cost[0] = perChar
	runKind, runLen := "", 1
	flush := func(end int) {
		if runLen >= 3 {
			s.Warnings = append(s.Warnings, fmt.Sprintf("%s %q", runKind, string(runes[end-runLen:end])))
		}
	}
	for i := 1; i < len(runes); i++ {
		kind, ok := "", false
		if runes[i] == runes[i-1] {
			kind, ok = "repeated characters", true
		} else {
			kind, ok = patternStep(runes[i-1], runes[i])
		}
		if ok && (runLen == 1 || kind == runKind) {
			cost[i] = 1
			runKind = kind
			runLen++
			continue
		}
		flush(i)
		runKind, runLen = "", 1
		if ok { // a new run starting at i-1
			cost[i] = 1
			runKind, runLen = kind, 2
			continue
		}
		cost[i] = perChar
	}
	flush(len(runes))

	// Years (1900-2099) are among the first things tried: ~200 choices.
	for i := 0; i+4 <= len(runes); i++ {
		y := string(runes[i : i+4])
		before := i == 0 || !unicode.IsDigit(runes[i-1])
		after := i+4 == len(runes) || !unicode.IsDigit(runes[i+4])
		if before && after && (strings.HasPrefix(y, "19") || strings.HasPrefix(y, "20")) && isDigits(y) {
			s.Warnings = append(s.Warnings, fmt.Sprintf("looks like a year (%q)", y))
			for j := i; j < i+4; j++ {
				cost[j] = math.Log2(200) / 4
			}
		}
	}

	bits := 0.0
	if word, start, end, ok := commonCore(runes); ok {
		s.Warnings = append(s.Warnings, fmt.Sprintf("based on a common password or word (%q)", word))
		for i := start; i < end; i++ {
			cost[i] = 0
		}
		// Picking a word from a big list, plus a few bits for case and l33t.
		bits += math.Log2(float64(len(commonWords))) + 2
	}
	if isDigits(pw) && len(runes) <= 8 {
		s.Warnings = append(s.Warnings, "digits only, like a PIN or date")
	}
	for _, c := range cost {
		bits += c
	}
	s.Bits = math.Min(bits, float64(len(runes))*perChar)
	if len(runes) < 12 {
		s.Warnings = append(s.Warnings, "shorter than 12 characters")
	}
	return s
}

// readSecret reads one line from stdin. On a terminal it hides what is typed
// (where the OS lets us), so the password does not end up on screen.
func readSecret(stdin io.Reader, stderr io.Writer) (string, error) {
	f, isFile := stdin.(*os.File)
	tty := false
	if isFile {
		if st, err := f.Stat(); err == nil && st.Mode()&os.ModeCharDevice != 0 {
			tty = true
		}
	}
	if tty {
		if runtime.GOOS == "windows" {
			fmt.Fprint(stderr, "Password to check (input is visible): ")
		} else {
			fmt.Fprint(stderr, "Password to check (not shown): ")
			if setEcho(f, false) == nil {
				defer func() {
					_ = setEcho(f, true)
					fmt.Fprintln(stderr)
				}()
			}
		}
	}
	line, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && line == "" {
		if errors.Is(err, io.EOF) {
			return "", errors.New("no password given on stdin")
		}
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// setEcho turns terminal echo on or off with stty (macOS, Linux, BSD).
func setEcho(tty *os.File, on bool) error {
	arg := "-echo"
	if on {
		arg = "echo"
	}
	cmd := exec.Command("stty", arg) //nolint:gosec,noctx // G204: fixed command; stty returns at once
	cmd.Stdin = tty
	return cmd.Run()
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

func runCheck(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("passgen check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	minBits := fs.Float64("min-bits", 0, "exit 3 if the estimate is below this")
	var quiet bool
	fs.BoolVar(&quiet, "quiet", false, "print the bits only")
	fs.BoolVar(&quiet, "q", false, "print the bits only")
	fs.Usage = func() {
		fmt.Fprint(stderr, `Usage:
  passgen check [options]        reads the password from stdin

Estimates a password's strength offline. Nothing is sent anywhere.
Type it at the prompt (hidden), or pipe it in:  pbpaste | passgen check
Never pass a password as an argument: it would land in your shell history.

Options:
      --min-bits N   exit 3 if the estimate is below N bits
  -q, --quiet        print the estimated bits only
`)
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintln(stderr, "passgen check: don't pass the password as an argument (it ends up in shell history); type it at the prompt or pipe it in")
		return 2
	}
	pw, err := readSecret(stdin, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "passgen check: %v\n", err)
		return 1
	}
	s := CheckStrength(pw)
	if quiet {
		fmt.Fprintf(stdout, "%.0f\n", s.Bits)
	} else {
		printStrength(stdout, s)
	}
	if *minBits > 0 && s.Bits < *minBits {
		if !quiet {
			fmt.Fprintf(stderr, "passgen check: below the minimum of %.0f bits\n", *minBits)
		}
		return exitBelowMin
	}
	return 0
}

func printStrength(w io.Writer, s Strength) {
	fmt.Fprintf(w, "Length:     %d\n", s.Length)
	if len(s.Classes) > 0 {
		fmt.Fprintf(w, "Characters: %s (pool %d)\n", strings.Join(s.Classes, ", "), s.Pool)
	}
	fmt.Fprintf(w, "Estimate:   ~%.0f bits: %s\n", s.Bits, StrengthLabel(s.Bits))
	if len(s.Warnings) > 0 {
		fmt.Fprintln(w, "Warnings:")
		for _, m := range s.Warnings {
			fmt.Fprintf(w, "  - %s\n", m)
		}
	}
	fmt.Fprintln(w, "\nThis is a best case. Cracking tools use bigger word lists; a random\npassword from passgen avoids every pattern above.")
}
