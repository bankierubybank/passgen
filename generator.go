package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
)

const (
	lowerChars = "abcdefghijklmnopqrstuvwxyz"
	upperChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digitChars = "0123456789"
	// Characters easily confused with each other when read or typed.
	ambiguousChars = "Il1O0o|`'\""
)

// Symbol sets, from most to least compatible.
var symbolSets = map[string]string{
	// Accepted by nearly every password policy; no quoting/escaping trouble
	// in shell, JSON, XML/HTML, URLs, CSV, SQL or .env files.
	"safe": "-_.+=@#%*?^",
	// Wider set; still avoids quotes, backslash, backtick, pipe and space,
	// which many systems reject or mangle.
	"standard": "!@#$%^&*()-_=+[]{};:,.<>?/~",
	// Every printable ASCII special character (OWASP list, minus space).
	// Some systems reject " ' ` \ | < > & ; or need them escaped.
	"complex": "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~",
}

// Ordered names, for help text and prompts.
var symbolModes = []string{"safe", "standard", "complex"}

// Complexity presets: which character sets are on.
type preset struct{ lower, upper, digits, symbols bool }

var presets = map[string]preset{
	"low":    {lower: true, digits: true},
	"medium": {lower: true, upper: true, digits: true},
	"high":   {lower: true, upper: true, digits: true, symbols: true},
}

var complexityLevels = []string{"low", "medium", "high"}

// Options fully describe how passwords are generated.
type Options struct {
	Length      int
	Lower       bool
	Upper       bool
	Digits      bool
	Symbols     bool
	SymbolMode  string // key of symbolSets
	SymbolSet   string // custom symbols; overrides SymbolMode when not empty
	NoAmbiguous bool
	Exclude     string
	RequireEach bool // at least one char from every enabled set
}

// ApplyPreset turns character sets on or off to match a complexity level.
func (o *Options) ApplyPreset(level string) error {
	p, ok := presets[level]
	if !ok {
		return fmt.Errorf("unknown complexity %q (want low, medium or high)", level)
	}
	o.Lower, o.Upper, o.Digits, o.Symbols = p.lower, p.upper, p.digits, p.symbols
	return nil
}

func (o Options) symbols() string {
	if o.SymbolSet != "" {
		return o.SymbolSet
	}
	return symbolSets[o.SymbolMode]
}

// Pools returns the enabled character sets, with excluded characters removed.
// Sets left empty after exclusion are dropped.
func (o Options) Pools() []string {
	var raw []string
	if o.Lower {
		raw = append(raw, lowerChars)
	}
	if o.Upper {
		raw = append(raw, upperChars)
	}
	if o.Digits {
		raw = append(raw, digitChars)
	}
	if o.Symbols {
		raw = append(raw, o.symbols())
	}

	exclude := o.Exclude
	if o.NoAmbiguous {
		exclude += ambiguousChars
	}

	var pools []string
	for _, p := range raw {
		var b strings.Builder
		for _, c := range p {
			if !strings.ContainsRune(exclude, c) {
				b.WriteRune(c)
			}
		}
		if b.Len() > 0 {
			pools = append(pools, b.String())
		}
	}
	return pools
}

// Validate reports a problem with the options, or nil.
func (o Options) Validate() error {
	if o.SymbolSet == "" && o.Symbols {
		if _, ok := symbolSets[o.SymbolMode]; !ok {
			return fmt.Errorf("unknown symbol mode %q (want safe, standard or complex)", o.SymbolMode)
		}
	}
	if o.Length < 1 {
		return errors.New("length must be at least 1")
	}
	pools := o.Pools()
	if len(pools) == 0 {
		return errors.New("no character sets enabled")
	}
	if o.RequireEach && o.Length < len(pools) {
		return fmt.Errorf("length %d too short to include all %d enabled character sets", o.Length, len(pools))
	}
	return nil
}

// alphabet merges pools into one string of unique runes, order kept.
func alphabet(pools []string) []rune {
	seen := map[rune]bool{}
	var out []rune
	for _, p := range pools {
		for _, c := range p {
			if !seen[c] {
				seen[c] = true
				out = append(out, c)
			}
		}
	}
	return out
}

// randIndex returns a uniform random int in [0, n) from the OS CSPRNG.
func randIndex(n int) int {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		// The OS random source failing is not recoverable; never fall back to weak randomness.
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return int(v.Int64()) //nolint:gosec // G115: v < n, and n is an int, so it fits
}

// Generate makes one password. Call Validate first.
func Generate(o Options) string {
	pools := o.Pools()
	alpha := alphabet(pools)

	chars := make([]rune, 0, o.Length)
	if o.RequireEach {
		for _, p := range pools {
			r := []rune(p)
			chars = append(chars, r[randIndex(len(r))])
		}
	}
	for len(chars) < o.Length {
		chars = append(chars, alpha[randIndex(len(alpha))])
	}
	// Fisher-Yates shuffle so the forced characters are not at the front.
	for i := len(chars) - 1; i > 0; i-- {
		j := randIndex(i + 1)
		chars[i], chars[j] = chars[j], chars[i]
	}
	return string(chars)
}

// PoolSize is the number of distinct characters a password is drawn from.
func (o Options) PoolSize() int { return len(alphabet(o.Pools())) }

// EntropyBits estimates the strength of a password made with these options.
func (o Options) EntropyBits() float64 {
	return float64(o.Length) * math.Log2(float64(o.PoolSize()))
}

// StrengthLabel turns entropy bits into a word.
func StrengthLabel(bits float64) string {
	switch {
	case bits < 40:
		return "weak"
	case bits < 60:
		return "fair"
	case bits < 80:
		return "strong"
	default:
		return "very strong"
	}
}

func strengthLine(o Options) string {
	bits := o.EntropyBits()
	return fmt.Sprintf("[%d chars pool, ~%.0f bits entropy: %s]", o.PoolSize(), bits, StrengthLabel(bits))
}
