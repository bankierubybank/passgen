package main

import (
	"bytes"
	"math"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

func defaultOpts(t *testing.T, level string) Options {
	t.Helper()
	o := Options{Length: 16, SymbolMode: "safe", RequireEach: true}
	if err := o.ApplyPreset(level); err != nil {
		t.Fatal(err)
	}
	return o
}

func allowedSet(o Options) map[rune]bool {
	m := map[rune]bool{}
	for _, c := range alphabet(o.Pools()) {
		m[c] = true
	}
	return m
}

func TestGenerateUsesOnlyAllowedChars(t *testing.T) {
	for _, mode := range symbolModes {
		o := defaultOpts(t, "high")
		o.SymbolMode = mode
		o.Length = 32
		allowed := allowedSet(o)
		used := map[rune]bool{}
		for i := 0; i < 2000; i++ {
			pw := Generate(o)
			if n := utf8.RuneCountInString(pw); n != o.Length {
				t.Fatalf("%s: length %d, want %d", mode, n, o.Length)
			}
			for _, c := range pw {
				if !allowed[c] {
					t.Fatalf("%s: char %q not allowed in %q", mode, c, pw)
				}
				used[c] = true
			}
		}
		// Every symbol in the set should show up across 64k samples.
		for _, c := range symbolSets[mode] {
			if !used[c] {
				t.Errorf("%s: symbol %q never generated", mode, c)
			}
		}
	}
}

func TestRequireEachIncludesEverySet(t *testing.T) {
	o := defaultOpts(t, "high")
	o.Length = 4 // exactly one slot per set
	for i := 0; i < 500; i++ {
		pw := Generate(o)
		for _, pool := range o.Pools() {
			if !strings.ContainsAny(pw, pool) {
				t.Fatalf("%q missing a char from %q", pw, pool)
			}
		}
	}
}

func TestPresets(t *testing.T) {
	cases := map[string]int{"low": 36, "medium": 62, "high": 62 + len(symbolSets["safe"])}
	for level, want := range cases {
		if got := defaultOpts(t, level).PoolSize(); got != want {
			t.Errorf("%s: pool %d, want %d", level, got, want)
		}
	}
	var o Options
	if err := o.ApplyPreset("extreme"); err == nil {
		t.Error("unknown preset accepted")
	}
}

func TestNoAmbiguousAndExclude(t *testing.T) {
	o := defaultOpts(t, "high")
	o.SymbolMode = "complex"
	o.NoAmbiguous = true
	o.Exclude = "abc"
	allowed := allowedSet(o)
	for _, c := range ambiguousChars + "abc" {
		if allowed[c] {
			t.Errorf("excluded char %q still in pool", c)
		}
	}
}

func TestCustomSymbolSet(t *testing.T) {
	o := defaultOpts(t, "high")
	o.SymbolSet = "#!"
	if got := o.Pools()[3]; got != "#!" {
		t.Errorf("symbol pool %q, want %q", got, "#!")
	}
}

func TestValidate(t *testing.T) {
	o := defaultOpts(t, "high")
	if err := o.Validate(); err != nil {
		t.Fatalf("default options invalid: %v", err)
	}

	short := o
	short.Length = 3
	if short.Validate() == nil {
		t.Error("length below set count accepted")
	}
	short.RequireEach = false
	if err := short.Validate(); err != nil {
		t.Errorf("length 3 without require-each rejected: %v", err)
	}

	none := o
	none.Lower, none.Upper, none.Digits, none.Symbols = false, false, false, false
	if none.Validate() == nil {
		t.Error("no sets accepted")
	}

	allExcluded := defaultOpts(t, "low")
	allExcluded.Upper, allExcluded.Symbols = false, false
	allExcluded.Exclude = lowerChars + digitChars
	if allExcluded.Validate() == nil {
		t.Error("everything excluded accepted")
	}

	bad := o
	bad.SymbolMode = "bogus"
	if bad.Validate() == nil {
		t.Error("unknown symbol mode accepted")
	}
}

func TestStrengthLabel(t *testing.T) {
	cases := map[float64]string{10: "weak", 50: "fair", 70: "strong", 100: "very strong"}
	for bits, want := range cases {
		if got := StrengthLabel(bits); got != want {
			t.Errorf("%v bits: %q, want %q", bits, got, want)
		}
	}
}

func runCLI(t *testing.T, stdin string, args ...string) (code int, out, errOut string) {
	t.Helper()
	var o, e bytes.Buffer
	code = run(args, strings.NewReader(stdin), &o, &e)
	return code, o.String(), e.String()
}

func TestCLIFlags(t *testing.T) {
	code, out, _ := runCLI(t, "", "-l", "20", "-n", "3", "-q")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	lines := strings.Fields(out)
	if len(lines) != 3 {
		t.Fatalf("got %d passwords, want 3: %q", len(lines), out)
	}
	for _, l := range lines {
		if len(l) != 20 {
			t.Errorf("%q: length %d, want 20", l, len(l))
		}
	}

	// Long flags with -- and per-set override on top of the preset.
	code, out, _ = runCLI(t, "", "--length=30", "--complexity", "low", "--upper", "-q")
	if code != 0 || strings.ToLower(out) == out {
		t.Errorf("--upper over low preset: exit %d, out %q", code, out)
	}
	code, out, _ = runCLI(t, "", "--no-symbols", "--no-digits", "--no-upper", "-q", "-l", "40")
	if code != 0 || strings.Trim(strings.TrimSpace(out), lowerChars) != "" {
		t.Errorf("lowercase only: exit %d, out %q", code, out)
	}
}

func TestCLIErrors(t *testing.T) {
	cases := [][]string{
		{"-l", "2"},
		{"-n", "0"},
		{"-c", "extreme"},
		{"-s", "bogus"},
		{"--no-lower", "--no-upper", "--no-digits", "--no-symbols"},
		{"stray"},
		{"--nope"},
	}
	for _, args := range cases {
		if code, _, _ := runCLI(t, "", args...); code == 0 {
			t.Errorf("%v: exit 0, want error", args)
		}
	}
	if code, _, _ := runCLI(t, "", "-h"); code != 0 {
		t.Errorf("-h: exit %d, want 0", code)
	}
}

func TestInteractive(t *testing.T) {
	// length 20, 3 passwords, high, complex symbols, exclude ambiguous,
	// regenerate once, bad command, then quit.
	input := "20\n3\nhigh\ncomplex\ny\n\nzzz\nq\n"
	code, out, _ := runCLI(t, input, "-i")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if n := strings.Count(out, "  1) "); n != 2 {
		t.Errorf("want 2 rounds of passwords, got %d:\n%s", n, out)
	}
	if !strings.Contains(out, "unknown command") {
		t.Error("bad command not reported")
	}
}

func TestInteractiveRetriesBadInputAndEOF(t *testing.T) {
	// Bad length, bad complexity, then EOF mid-session: must exit cleanly.
	input := "abc\n5000\n12\n1\nextreme\nlow\nn\n"
	code, out, _ := runCLI(t, input, "-i")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{"enter a number", "must be 1-1024", "pick one of"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output", want)
		}
	}
}

// Duplicates in a custom symbol set, or symbols that are also letters or
// digits, must not make any character likelier.
func TestPoolsHaveNoDuplicates(t *testing.T) {
	o := defaultOpts(t, "high")
	o.SymbolSet = "!!!!#a1"
	pools := o.Pools()
	if got := pools[len(pools)-1]; got != "!#" {
		t.Errorf("symbol pool %q, want %q", got, "!#")
	}
	seen := map[rune]bool{}
	for _, p := range pools {
		for _, c := range p {
			if seen[c] {
				t.Errorf("%q appears twice across pools %q", c, pools)
			}
			seen[c] = true
		}
	}
	// A symbol set made only of letters adds no new set to require.
	o.SymbolSet = "abc"
	if n := len(o.Pools()); n != 3 {
		t.Errorf("%d pools, want 3", n)
	}
}

// With one char forced from each set, the count of possible passwords is
// exactly computable for short lengths.
func TestEntropyBitsRequireEach(t *testing.T) {
	o := defaultOpts(t, "high") // 26 + 26 + 10 + 11 safe symbols
	o.Length = 4
	// One from each set, in any of 4! orders.
	want := math.Log2(26 * 26 * 10 * 11 * 24)
	if got := o.EntropyBits(); math.Abs(got-want) > 1e-9 {
		t.Errorf("length 4: %.4f bits, want %.4f", got, want)
	}
	// Long passwords: the rule barely matters.
	o.Length = 64
	if got, plain := o.EntropyBits(), 64*math.Log2(73); plain-got > 0.01 || got > plain {
		t.Errorf("length 64: %.3f bits, want just under %.3f", got, plain)
	}
	// Huge lengths must not overflow.
	o.Length = maxLength
	if got := o.EntropyBits(); math.IsNaN(got) || math.IsInf(got, 0) {
		t.Errorf("max length: %v", got)
	}
	o.RequireEach = false
	o.Length = 4
	if got, want := o.EntropyBits(), 4*math.Log2(73); math.Abs(got-want) > 1e-9 {
		t.Errorf("no require-each: %.4f, want %.4f", got, want)
	}
}

// Interactive mode has its own limits; command-line defaults outside them
// are pulled in instead of being accepted as-is.
func TestInteractiveClampsCommandLineDefaults(t *testing.T) {
	code, out, _ := runCLI(t, "\n\n\n\nq\n", "-i", "-l", "2000", "-n", "150", "--no-symbols", "-c", "low")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out, "Length [1024]") || !strings.Contains(out, "How many [100]") {
		t.Errorf("defaults not clamped:\n%s", out)
	}
	if n := len(regexp.MustCompile(`(?m)^ +\d+\) `).FindAllString(out, -1)); n != 100 {
		t.Errorf("printed %d passwords, want 100", n)
	}
}

func TestInteractiveWarnsAboutIgnoredFlags(t *testing.T) {
	_, _, errOut := runCLI(t, "q\n", "-i", "--copy")
	if !strings.Contains(errOut, "don't apply to -i") {
		t.Errorf("stderr %q", errOut)
	}
}
