package main

// Fuzz tests. `go test` runs the seed inputs below; CI also runs each target
// with random inputs for a short while:
//
//	go test -run '^$' -fuzz '^FuzzGenerate$' -fuzztime 30s .

import (
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

func writeFile(path, s string) error { return os.WriteFile(path, []byte(s), 0o600) }

// Any options that pass Validate must give a password of the right length,
// using only allowed characters, with every set present when required.
func FuzzGenerate(f *testing.F) {
	f.Add(16, true, true, true, true, false, true, "", "", "safe")
	f.Add(1, false, false, true, false, true, true, "", "0123456789", "safe")
	f.Add(4, true, false, false, true, false, true, "日本", "abc", "complex")
	f.Add(3, true, true, true, true, true, true, "\xff\xfe", "", "nope")
	f.Fuzz(func(t *testing.T, length int, lower, upper, digits, symbols, noAmb, req bool, symSet, exclude, mode string) {
		if length > 512 {
			length %= 512 // keep each run fast
		}
		o := Options{Length: length, Lower: lower, Upper: upper, Digits: digits, Symbols: symbols,
			SymbolMode: mode, SymbolSet: symSet, NoAmbiguous: noAmb, Exclude: exclude, RequireEach: req}
		if o.Validate() != nil {
			return
		}
		pw := Generate(o)
		if n := utf8.RuneCountInString(pw); n != length {
			t.Fatalf("length %d, want %d", n, length)
		}
		allowed := map[rune]bool{}
		for _, c := range alphabet(o.Pools()) {
			allowed[c] = true
		}
		for _, c := range pw {
			if !allowed[c] {
				t.Fatalf("char %q not allowed", c)
			}
		}
		if req {
			for _, p := range o.Pools() {
				if !strings.ContainsAny(pw, p) {
					t.Fatalf("no char from set %q in %q", p, pw)
				}
			}
		}
		if bits := o.EntropyBits(); math.IsNaN(bits) || bits < 0 {
			t.Fatalf("entropy %v", bits)
		}
	})
}

// Any command line must exit cleanly with a known code, never panic.
func FuzzCLI(f *testing.F) {
	for _, s := range []string{
		"-l 20 -n 3", "-c low --no-ambiguous", "--symbol-set=!! --exclude !",
		"--no-lower --no-upper --no-digits --no-symbols", "-l 0", "-l -5", "-n 0",
		"--list-symbols", "-v", "check -q", "completion zsh", "man", "-x", "--",
		"-l 3 --symbols --no-require-each", "-i", "-c",
	} {
		f.Add(s, "password\n")
	}
	f.Fuzz(func(t *testing.T, cmdline, stdin string) {
		args := strings.Fields(cmdline)
		for _, a := range args {
			name, val, _ := strings.Cut(strings.TrimLeft(a, "-"), "=")
			// No network, no clipboard, nothing that only repeats slow work.
			if strings.Contains(name, "update") || strings.Contains(name, "copy") {
				return
			}
			for _, v := range []string{name, val} {
				if n, err := strconv.ParseInt(v, 0, 64); err == nil && n > 300 {
					return
				}
			}
		}
		code := run(args, strings.NewReader(stdin), io.Discard, io.Discard)
		if code < 0 || code > 3 {
			t.Fatalf("%q: exit %d", cmdline, code)
		}
	})
}

// The strength estimate must stay a sane number for any input.
func FuzzCheckStrength(f *testing.F) {
	for _, s := range []string{"", "password", "P@ssw0rd2024!", "aaaa", "abcd", "qwerty",
		"\xff\xfe\xfd", "ñandú", "2024", "1900x2099", "h_nX#3TsmGp6PdKJ"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, pw string) {
		s := CheckStrength(pw)
		if math.IsNaN(s.Bits) || math.IsInf(s.Bits, 0) || s.Bits < 0 {
			t.Fatalf("%q: bits %v", pw, s.Bits)
		}
		if s.Length != utf8.RuneCountInString(pw) {
			t.Fatalf("%q: length %d", pw, s.Length)
		}
		if s.Length > 0 && s.Bits > float64(s.Length)*math.Log2(float64(s.Pool))+1e-9 {
			t.Fatalf("%q: %.1f bits beats brute force", pw, s.Bits)
		}
	})
}

// Release tags come from the network: parsing them must never panic, and
// comparison must be consistent.
func FuzzParseVersion(f *testing.F) {
	for _, s := range []string{"v1.2.3", "1.2.3", "v0.2.3-rc1", "v", "", "v1.2", "v99999999999999999999.0.0", "v1.2.3.4"} {
		f.Add(s, "v0.2.3")
	}
	f.Fuzz(func(t *testing.T, a, b string) {
		va, _, okA := parseVersion(a)
		vb, _, okB := parseVersion(b)
		if !okA || !okB {
			return
		}
		if newer(va, va) {
			t.Fatalf("%q newer than itself", a)
		}
		if newer(va, vb) && newer(vb, va) {
			t.Fatalf("%q and %q both newer than each other", a, b)
		}
	})
}
