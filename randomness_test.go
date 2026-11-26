package main

// Statistical checks that generation is unbiased. A password generator can
// look fine and still quietly favour some characters (modulo bias, a broken
// shuffle, a bad refactor); these tests catch that.
//
// Each test runs a chi-square goodness-of-fit test. The pass threshold is set
// at p = 1e-6, so a correct generator fails by bad luck about once in a
// million runs, while a real bias of a few percent fails every time.

import (
	"math"
	"strconv"
	"testing"
)

// zCrit is the standard normal quantile for an upper tail of 1e-6.
const zCrit = 4.753

// chiSquareCritical approximates the chi-square value with upper tail 1e-6
// for df degrees of freedom (Wilson-Hilferty, accurate for df >= 1).
func chiSquareCritical(df int) float64 {
	d := float64(df)
	h := 2 / (9 * d)
	return d * math.Pow(1-h+zCrit*math.Sqrt(h), 3)
}

// chiSquare compares observed counts with expected counts.
func chiSquare(observed []int, expected []float64) float64 {
	var x float64
	for i, o := range observed {
		diff := float64(o) - expected[i]
		x += diff * diff / expected[i]
	}
	return x
}

func assertFits(t *testing.T, what string, observed []int, expected []float64) {
	t.Helper()
	x := chiSquare(observed, expected)
	limit := chiSquareCritical(len(observed) - 1)
	if x > limit {
		t.Errorf("%s: biased, chi-square %.1f > limit %.1f (df %d)", what, x, limit, len(observed)-1)
	}
}

func uniform(n int, total float64) []float64 {
	e := make([]float64, n)
	for i := range e {
		e[i] = total / float64(n)
	}
	return e
}

// randIndex must be uniform for every n, including ones that don't divide
// 2^k evenly (where naive modulo would be biased).
func TestRandIndexUniform(t *testing.T) {
	for _, n := range []int{2, 3, 7, 10, 11, 36, 73, 94} {
		samples := n * 2000
		counts := make([]int, n)
		for i := 0; i < samples; i++ {
			counts[randIndex(n)]++
		}
		assertFits(t, "randIndex n="+strconv.Itoa(n), counts, uniform(n, float64(samples)))
	}
}

// Without the one-per-set rule, every character in the pool is equally likely.
func TestCharFrequencyUniform(t *testing.T) {
	for _, mode := range symbolModes {
		o := Options{Length: 32, SymbolMode: mode}
		if err := o.ApplyPreset("high"); err != nil {
			t.Fatal(err)
		}
		alpha := alphabet(o.Pools())
		index := map[rune]int{}
		for i, c := range alpha {
			index[c] = i
		}
		counts := make([]int, len(alpha))
		passwords := len(alpha) * 1500 / o.Length
		for i := 0; i < passwords; i++ {
			for _, c := range Generate(o) {
				counts[index[c]]++
			}
		}
		assertFits(t, "char frequency, mode "+mode, counts, uniform(len(alpha), float64(passwords*o.Length)))
	}
}

// With the one-per-set rule on (the default), each password gets one forced
// char from every set plus the rest drawn from the whole pool. Check the
// counts match exactly that mix: no extra bias beyond what the rule implies.
func TestCharFrequencyRequireEach(t *testing.T) {
	o := Options{Length: 16, SymbolMode: "safe", RequireEach: true}
	if err := o.ApplyPreset("high"); err != nil {
		t.Fatal(err)
	}
	pools := o.Pools()
	alpha := alphabet(pools)
	index := map[rune]int{}
	for i, c := range alpha {
		index[c] = i
	}
	passwords := 12000
	free := float64(o.Length - len(pools)) // slots drawn from the whole pool

	expected := make([]float64, len(alpha))
	for _, p := range pools {
		for _, c := range p {
			perPassword := 1/float64(len(p)) + free/float64(len(alpha))
			expected[index[c]] = perPassword * float64(passwords)
		}
	}
	counts := make([]int, len(alpha))
	for i := 0; i < passwords; i++ {
		for _, c := range Generate(o) {
			counts[index[c]]++
		}
	}
	assertFits(t, "char frequency with require-each", counts, expected)
}

// The forced characters are shuffled in, so every kind of character must be
// equally likely at every position (not, say, symbols always first).
func TestPositionUnbiased(t *testing.T) {
	o := Options{Length: 16, SymbolMode: "safe", RequireEach: true}
	if err := o.ApplyPreset("high"); err != nil {
		t.Fatal(err)
	}
	pools := o.Pools()
	setOf := map[rune]int{}
	for i, p := range pools {
		for _, c := range p {
			setOf[c] = i
		}
	}
	// counts[set][position]
	counts := make([][]int, len(pools))
	for i := range counts {
		counts[i] = make([]int, o.Length)
	}
	for i := 0; i < 12000; i++ {
		for pos, c := range []rune(Generate(o)) {
			counts[setOf[c]][pos]++
		}
	}
	names := []string{"lowercase", "uppercase", "digits", "symbols"}
	for s, byPos := range counts {
		total := 0
		for _, n := range byPos {
			total += n
		}
		assertFits(t, names[s]+" across positions", byPos, uniform(o.Length, float64(total)))
	}
}

// Sanity check against a stuck or reseeded source: no repeats in a batch.
func TestNoDuplicatePasswords(t *testing.T) {
	o := Options{Length: 16, SymbolMode: "safe", RequireEach: true}
	if err := o.ApplyPreset("high"); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for i := 0; i < 20000; i++ {
		pw := Generate(o)
		if seen[pw] {
			t.Fatalf("duplicate password after %d: %q", i, pw)
		}
		seen[pw] = true
	}
}

// The tests themselves must be able to fail: a biased generator must trip them.
func TestChiSquareDetectsBias(t *testing.T) {
	n, samples := 73, 73*1500
	counts := make([]int, n)
	for i := 0; i < samples; i++ {
		v := randIndex(n + 7)
		if v >= n {
			v = 0 // fold the extra values onto char 0: a ~10x bias on one char
		}
		counts[v]++
	}
	if x := chiSquare(counts, uniform(n, float64(samples))); x <= chiSquareCritical(n-1) {
		t.Errorf("planted bias not detected: chi-square %.1f <= %.1f", x, chiSquareCritical(n-1))
	}

	// Subtle modulo bias: index = randIndex(256) % 73, the classic mistake.
	counts = make([]int, n)
	for i := 0; i < samples*4; i++ {
		counts[randIndex(256)%n]++
	}
	if x := chiSquare(counts, uniform(n, float64(samples*4))); x <= chiSquareCritical(n-1) {
		t.Errorf("modulo bias not detected: chi-square %.1f <= %.1f", x, chiSquareCritical(n-1))
	}
}
