package main

// passgen check: estimate how strong an existing password is, offline.
//
// The estimate starts from brute force (length x log2(pool)), then charges
// far less for the parts a cracking tool tries first: common passwords and
// words (also with l33t swaps), years, repeated characters, sequences like
// "abcd" or "4321", keyboard runs like "qwerty", and repeats of an earlier
// part. It is an upper bound: real tools use much bigger word lists and
// rules, so treat it as a best case.

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
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

// maxCheckRunes caps what check reads and scores, like the generator's length.
const maxCheckRunes = maxLength

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
	Warnings []string // never contain any part of the password
}

func (s *Strength) warn(msg string) {
	for _, w := range s.Warnings {
		if w == msg {
			return
		}
	}
	s.Warnings = append(s.Warnings, msg)
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

// patternStep reports whether b continues a run from a: the same character
// again, the next or previous letter or digit, or the next key on the row.
func patternStep(a, b rune) (kind string, ok bool) {
	if a == b {
		return "repeated characters", true
	}
	la, lb := unicode.ToLower(a), unicode.ToLower(b)
	ca, _ := charClass(la)
	cb, _ := charClass(lb)
	if ca == cb && (ca == "lowercase" || ca == "digits") && (lb-la == 1 || la-lb == 1) {
		return "sequence like abcd or 4321", true
	}
	ra, ia := keyboardPos(a)
	rb, ib := keyboardPos(b)
	if ra >= 0 && ra == rb && (ib-ia == 1 || ia-ib == 1) {
		return "keyboard run like qwerty", true
	}
	return "", false
}

// span is a stretch of the password, [start, end) in runes.
type span struct{ start, end int }

// commonWordSpans finds every common password or word in pw, ignoring case
// and l33t swaps (p@ssw0rd), so "MyPassword2024!" is caught too. Matches
// don't overlap; at each position the longest word wins.
func commonWordSpans(pw []rune) []span {
	lower := make([]rune, len(pw))
	plain := make([]rune, len(pw)) // lowered, l33t undone; same length as pw
	for i, r := range pw {
		lower[i] = unicode.ToLower(r)
		plain[i] = lower[i]
		if u, isLeet := unleet[lower[i]]; isLeet {
			plain[i] = u
		}
	}
	matchAt := func(text []rune, i int, w string) bool {
		j := i
		for _, c := range w {
			if j >= len(text) || text[j] != c {
				return false
			}
			j++
		}
		return true
	}
	var spans []span
	for i := 0; i < len(pw); {
		best := 0
		for _, w := range commonWords {
			n := utf8.RuneCountInString(w)
			// Short words only count as the whole password: "god" inside a
			// random string is luck, not a pattern.
			if n <= best || (n < 4 && n != len(pw)) {
				continue
			}
			if matchAt(lower, i, w) || matchAt(plain, i, w) {
				best = n
			}
		}
		if best > 0 {
			spans = append(spans, span{i, i + best})
			i += best
		} else {
			i++
		}
	}
	return spans
}

func sum(xs []float64) float64 {
	t := 0.0
	for _, x := range xs {
		t += x
	}
	return t
}

// CheckStrength estimates the entropy of pw.
//
// Every character starts at the brute-force cost, log2(pool). Then whatever
// a cracking tool would guess early gets re-priced at what it really adds:
// a run (aaaa, abcd, qwer) costs ~log2(length)+1 bits for "keep going", a
// year ~8 bits, a common word ~9 bits, and a repeat of an earlier part of
// the password ~log2(where)+log2(length)+1 bits.
func CheckStrength(pw string) Strength {
	runes := []rune(pw)
	n := len(runes)
	s := Strength{Length: n}
	if n == 0 {
		s.warn("empty password")
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
	cost := make([]float64, n)
	for i := range cost {
		cost[i] = perChar
	}

	// Runs. The first character keeps its cost; continuing the run L-1 more
	// times is one choice of length, not L-1 fresh characters.
	for i := 0; i < n-1; {
		kind, ok := patternStep(runes[i], runes[i+1])
		if !ok {
			i++
			continue
		}
		j := i + 1
		for j+1 < n {
			if k, ok := patternStep(runes[j], runes[j+1]); !ok || k != kind {
				break
			}
			j++
		}
		cont := j - i // characters after the first
		each := (math.Log2(float64(cont)) + 1) / float64(cont)
		for k := i + 1; k <= j; k++ {
			cost[k] = math.Min(cost[k], each)
		}
		if cont >= 2 {
			s.warn(kind)
		}
		i = j
	}

	// Years (1900-2099) are among the first things tried: ~200 choices.
	for i := 0; i+4 <= n; i++ {
		y := string(runes[i : i+4])
		before := i == 0 || !unicode.IsDigit(runes[i-1])
		after := i+4 == n || !unicode.IsDigit(runes[i+4])
		if before && after && (strings.HasPrefix(y, "19") || strings.HasPrefix(y, "20")) && isDigits(y) {
			s.warn("looks like a year")
			for j := i; j < i+4; j++ {
				cost[j] = math.Log2(200) / 4
			}
		}
	}

	// Common words: picking one from the list, plus a few bits for case and l33t.
	wordBits := math.Log2(float64(len(commonWords))) + 2
	for _, sp := range commonWordSpans(runes) {
		s.warn("contains a common password or word")
		each := wordBits / float64(sp.end-sp.start)
		for i := sp.start; i < sp.end; i++ {
			cost[i] = math.Min(cost[i], each)
		}
	}

	// Repeats of an earlier part (Password!Password!): the copy costs only
	// where it starts and how long it is. Case is ignored, for one extra bit.
	lower := make([]rune, n)
	for i, r := range runes {
		lower[i] = unicode.ToLower(r)
	}
	for i := 1; i < n; {
		bestLen, bestFrom := 0, 0
		for j := 0; j < i; j++ {
			m := 0
			for i+m < n && lower[j+m] == lower[i+m] {
				m++
			}
			if m > bestLen {
				bestLen, bestFrom = m, j
			}
		}
		if bestLen >= 3 {
			ref := math.Log2(float64(i-bestFrom)) + math.Log2(float64(bestLen)) + 1
			if sum(cost[i:i+bestLen]) > ref {
				s.warn("repeats an earlier part of itself")
				for k := i; k < i+bestLen; k++ {
					cost[k] = ref / float64(bestLen)
				}
			}
			i += bestLen
			continue
		}
		i++
	}

	if isDigits(pw) && n <= 8 {
		s.warn("digits only, like a PIN or date")
	}
	if n < 12 {
		s.warn("shorter than 12 characters")
	}
	s.Bits = math.Min(sum(cost), float64(n)*perChar)
	return s
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// readSecret reads one line from stdin, at most maxCheckRunes characters.
// On a terminal it hides what is typed (where the OS lets us), so the
// password does not end up on screen.
func readSecret(stdin io.Reader, stderr io.Writer) (string, error) {
	if f, ok := stdin.(*os.File); ok && isTerminal(f) {
		if runtime.GOOS == "windows" {
			fmt.Fprint(stderr, "Password to check (input is visible): ")
		} else {
			fmt.Fprint(stderr, "Password to check (not shown): ")
			defer hideInput(f, stderr)()
		}
	}
	// A rune is at most 4 bytes; read one byte more to spot overlong input.
	limit := int64(maxCheckRunes*utf8.UTFMax + 1)
	line, err := bufio.NewReader(io.LimitReader(stdin, limit)).ReadString('\n')
	if err != nil && line == "" {
		if errors.Is(err, io.EOF) {
			return "", errors.New("no password given on stdin")
		}
		return "", err
	}
	line = strings.TrimRight(line, "\r\n")
	if int64(len(line)) >= limit-1 || utf8.RuneCountInString(line) > maxCheckRunes {
		return "", fmt.Errorf("password longer than %d characters", maxCheckRunes)
	}
	return line, nil
}

func isTerminal(f *os.File) bool {
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

// Swappable in tests.
var (
	echoFn = setEcho
	exitFn = os.Exit
)

// hideInput turns terminal echo off and returns a function that turns it
// back on. Until then, Ctrl-C or SIGTERM also restores echo before exiting,
// so the terminal is never left silent.
func hideInput(tty *os.File, stderr io.Writer) (restore func()) {
	if echoFn(tty, false) != nil {
		return func() {}
	}
	var once sync.Once
	restoreEcho := func() {
		once.Do(func() {
			_ = echoFn(tty, true)
			fmt.Fprintln(stderr)
		})
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		select {
		case <-sig:
			restoreEcho()
			exitFn(130) // the usual exit status after Ctrl-C
		case <-done:
		}
	}()
	return func() {
		signal.Stop(sig)
		close(done)
		restoreEcho()
	}
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
