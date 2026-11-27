package main

import (
	"flag"
	"io"
	"math"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckStrengthWeakPatterns(t *testing.T) {
	cases := []struct {
		pw      string
		maxBits float64
		warn    string
	}{
		{"password", 15, "common password"},
		{"P@ssw0rd", 15, "common password"},
		{"MyPassword2024!", 40, "year"},
		{"qwerty123", 25, "common password"},
		{"123456", 15, "common password"},
		{"aaaaaaaaaaaaaaaa", 25, "repeated"},
		{"abcdefgh", 15, "sequence"},
		{"87654321", 15, "sequence"},
		{"asdfghjk", 15, "common password"},
		{"zxcvbnm,", 20, "keyboard run"},
		{"hi", 15, "shorter than 12"},
		// Length must not rescue a pattern: these were "very strong" once.
		{strings.Repeat("a", 100), 20, "repeated"},
		{strings.Repeat("ab", 50), 20, "sequence"},
		{strings.Repeat("abcdefghijklmnopqrstuvwxyz", 3), 25, "sequence"},
		{"Password!Password!Password!Password!", 35, "repeats an earlier part"},
		{"xK9#mQ2$xK9#mQ2$xK9#mQ2$", 65, "repeats an earlier part"},
		{strings.Repeat("qwerty", 10), 25, "keyboard run"},
	}
	for _, c := range cases {
		s := CheckStrength(c.pw)
		if s.Bits > c.maxBits {
			t.Errorf("%q: %.1f bits, want <= %.0f", c.pw, s.Bits, c.maxBits)
		}
		if !strings.Contains(strings.Join(s.Warnings, "\n"), c.warn) {
			t.Errorf("%q: warnings %q, want one about %q", c.pw, s.Warnings, c.warn)
		}
	}
}

// passgen's own output must always rate as very strong.
func TestCheckStrengthGeneratedPasswords(t *testing.T) {
	o := defaultOpts(t, "high")
	o.Length = 20
	for i := 0; i < 500; i++ {
		pw := Generate(o)
		s := CheckStrength(pw)
		// Real entropy is ~124 bits. Patterns that happen by chance ("ab",
		// "11", "90-=") shave some off, but it must never look less than strong.
		if s.Bits < 80 {
			t.Fatalf("%q: only %.1f bits (warnings %q)", pw, s.Bits, s.Warnings)
		}
	}
}

func TestCheckStrengthEdgeCases(t *testing.T) {
	if s := CheckStrength(""); s.Bits != 0 || len(s.Warnings) == 0 {
		t.Errorf("empty: %+v", s)
	}
	// Non-ASCII counts, and doesn't break the rune bookkeeping.
	s := CheckStrength("ñandú-パスワード-password")
	if s.Length != 20 || !strings.Contains(strings.Join(s.Warnings, ","), "common password") {
		t.Errorf("unicode: %+v", s)
	}
	if math.IsNaN(s.Bits) || s.Bits <= 0 {
		t.Errorf("unicode bits: %v", s.Bits)
	}
}

// The prompt hides the password, so the report must not show any of it.
func TestCheckWarningsNeverShowThePassword(t *testing.T) {
	for _, pw := range []string{"xK9qwerty#mQ2vL", "MyPassword2024!", "aaaaZq!", "Zq!abcdefZq!", "hunter2hunter2"} {
		for _, w := range CheckStrength(pw).Warnings {
			for i := 0; i+3 <= len(pw); i++ {
				if chunk := pw[i : i+3]; strings.Contains(strings.ToLower(w), strings.ToLower(chunk)) && !strings.Contains(strings.ToLower(knownWarningText), strings.ToLower(chunk)) {
					t.Errorf("%q: warning %q shows %q", pw, w, chunk)
				}
			}
		}
	}
}

// Text the warnings may legitimately contain (examples like "abcd").
const knownWarningText = "empty password repeated characters sequence like abcd or 4321 keyboard run like qwerty " +
	"looks like a year contains a common password or word repeats an earlier part of itself " +
	"digits only, like a pin or date shorter than 12 characters"

func TestCheckCLI(t *testing.T) {
	code, out, _ := runCLI(t, "P@ssw0rd2024!\n", "check")
	if code != 0 || !strings.Contains(out, "weak") || !strings.Contains(out, "common password") || strings.Contains(out, "ssw0rd") {
		t.Errorf("exit %d, out %q", code, out)
	}
	code, out, _ = runCLI(t, "abc\n", "check", "-q")
	if code != 0 || strings.TrimSpace(out) == "" || strings.Contains(out, "Warnings") {
		t.Errorf("quiet: exit %d, out %q", code, out)
	}
	if code, _, _ := runCLI(t, "abc\n", "check", "--min-bits", "60"); code != exitBelowMin {
		t.Errorf("min-bits weak: exit %d, want %d", code, exitBelowMin)
	}
	if code, _, _ := runCLI(t, "h_nX#3TsmGp6PdKJ\n", "check", "--min-bits", "60"); code != 0 {
		t.Errorf("min-bits strong: exit %d, want 0", code)
	}
	// Password as an argument is refused: it would land in shell history.
	code, _, errOut := runCLI(t, "", "check", "hunter2")
	if code != 2 || !strings.Contains(errOut, "history") {
		t.Errorf("arg: exit %d, stderr %q", code, errOut)
	}
	if code, _, _ := runCLI(t, "", "check"); code != 1 {
		t.Errorf("no input: exit %d, want 1", code)
	}
	// Input is capped, so piping a huge file in can't eat memory.
	code, _, errOut = runCLI(t, strings.Repeat("a", maxCheckRunes+1)+"\n", "check")
	if code != 1 || !strings.Contains(errOut, "longer than") {
		t.Errorf("overlong: exit %d, stderr %q", code, errOut)
	}
	if code, _, _ := runCLI(t, strings.Repeat("é", maxCheckRunes)+"\n", "check", "-q"); code != 0 {
		t.Errorf("max length multibyte: exit %d, want 0", code)
	}
	if code, _, _ := runCLI(t, strings.Repeat("a", 10<<20), "check", "-q"); code != 1 {
		t.Errorf("10 MB without newline: exit %d, want 1", code)
	}
	// Windows line endings are trimmed.
	_, crlf, _ := runCLI(t, "password\r\n", "check", "-q")
	_, lf, _ := runCLI(t, "password\n", "check", "-q")
	if crlf != lf {
		t.Errorf("CRLF %q != LF %q", crlf, lf)
	}
}

// Every flag must appear in the help text, the man page and every completion
// script, so none of them can fall behind.
func TestDocsCoverEveryFlag(t *testing.T) {
	_, _, help := runCLI(t, "", "--help")
	_, manOut, _ := runCLI(t, "", "man")
	man := strings.ReplaceAll(manOut, `\-`, "-")
	scripts := map[string]string{}
	for _, sh := range completionShells {
		code, out, _ := runCLI(t, "", "completion", sh)
		if code != 0 || out == "" {
			t.Fatalf("completion %s: exit %d", sh, code)
		}
		scripts[sh] = out
	}
	for _, f := range flagSpecs() {
		long := "--" + f.long
		if !strings.Contains(help, long) {
			t.Errorf("--help is missing %s", long)
		}
		if !strings.Contains(man, long) {
			t.Errorf("man page is missing %s", long)
		}
		for sh, s := range scripts {
			name := long
			if sh == "fish" {
				name = "-l " + f.long
			}
			if !strings.Contains(s, name) {
				t.Errorf("%s completion is missing %s", sh, long)
			}
		}
	}
	for _, c := range subcommands {
		if !strings.Contains(help, "passgen "+c[0]) || !strings.Contains(man, c[0]) {
			t.Errorf("subcommand %s missing from help or man page", c[0])
		}
	}
}

func TestManVersionStamped(t *testing.T) {
	old := version
	version = "v9.8.7"
	defer func() { version = old }()
	_, out, _ := runCLI(t, "", "man")
	if !strings.Contains(out, `"passgen v9.8.7"`) {
		t.Errorf("version not in man header: %q", strings.SplitN(out, "\n", 2)[0])
	}
	if strings.Contains(out, "@VERSION@") || strings.Contains(out, "@DATE@") {
		t.Errorf("unfilled placeholder in man page: %q", strings.SplitN(out, "\n", 2)[0])
	}
}

// Every flag the parser knows must be in the list completion is built from.
func TestFlagSpecsListEveryFlag(t *testing.T) {
	fs, _, specs := newFlagSet(io.Discard)
	names := map[string]bool{}
	for _, s := range specs {
		names[s.long] = true
		if s.short != "" {
			names[s.short] = true
		}
	}
	fs.VisitAll(func(f *flag.Flag) {
		if !names[f.Name] {
			t.Errorf("flag %q missing from flagSpecs", f.Name)
		}
		delete(names, f.Name)
	})
	for n := range names {
		t.Errorf("flagSpecs lists %q, which the parser doesn't know", n)
	}
}

// The scripts must at least parse in the shells that are installed.
func TestCompletionScriptsParse(t *testing.T) {
	for _, sh := range completionShells {
		path, err := exec.LookPath(sh)
		if err != nil {
			t.Logf("%s not installed, skipping", sh)
			continue
		}
		_, out, _ := runCLI(t, "", "completion", sh)
		file := filepath.Join(t.TempDir(), "passgen."+sh)
		if err := writeFile(file, out); err != nil {
			t.Fatal(err)
		}
		args := []string{"-n", file}
		if sh == "fish" {
			args = []string{"--no-execute", file}
		}
		if msg, err := exec.Command(path, args...).CombinedOutput(); err != nil { //nolint:gosec,noctx // test: fixed shell
			t.Errorf("%s: %v\n%s", sh, err, msg)
		}
	}
	if code, _, _ := runCLI(t, "", "completion", "powershell"); code != 2 {
		t.Errorf("unknown shell: exit %d, want 2", code)
	}
	if code, _, _ := runCLI(t, "", "completion"); code != 2 {
		t.Errorf("no shell: exit %d, want 2", code)
	}
}
