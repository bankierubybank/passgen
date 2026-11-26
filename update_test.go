package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseVersion(t *testing.T) {
	cases := []struct {
		in     string
		nums   [3]int
		suffix bool
		ok     bool
	}{
		{"v1.2.3", [3]int{1, 2, 3}, false, true},
		{"0.10.0", [3]int{0, 10, 0}, false, true},
		{"v0.1.0-2-gff1110d-dirty", [3]int{0, 1, 0}, true, true},
		{"v2.0.0-rc1", [3]int{2, 0, 0}, true, true},
		{"dev", [3]int{}, false, false},
		{"v1.2", [3]int{}, false, false},
		{"v1.x.3", [3]int{}, false, false},
		{"ff1110d", [3]int{}, false, false},
	}
	for _, c := range cases {
		nums, suffix, ok := parseVersion(c.in)
		if ok != c.ok || (ok && (nums != c.nums || suffix != c.suffix)) {
			t.Errorf("parseVersion(%q) = %v %v %v, want %v %v %v", c.in, nums, suffix, ok, c.nums, c.suffix, c.ok)
		}
	}
}

func TestNewer(t *testing.T) {
	if !newer([3]int{0, 2, 0}, [3]int{0, 1, 9}) {
		t.Error("0.2.0 should be newer than 0.1.9")
	}
	if !newer([3]int{0, 10, 0}, [3]int{0, 9, 0}) {
		t.Error("0.10.0 should be newer than 0.9.0 (numeric, not string)")
	}
	if newer([3]int{1, 0, 0}, [3]int{1, 0, 0}) {
		t.Error("equal versions are not newer")
	}
}

// fakeGitHub serves a canned response for the latest-release endpoint.
func fakeGitHub(t *testing.T, status int, body string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ua := r.Header.Get("User-Agent"); !strings.HasPrefix(ua, "passgen/") {
			t.Errorf("User-Agent %q", ua)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	old := latestReleaseURL
	latestReleaseURL = srv.URL
	t.Cleanup(func() { latestReleaseURL = old })
}

func setVersion(t *testing.T, v string) {
	t.Helper()
	old := version
	version = v
	t.Cleanup(func() { version = old })
}

func TestCheckUpdate(t *testing.T) {
	const rel = `{"tag_name":"v0.2.0","html_url":"https://example.test/r/v0.2.0"}`
	cases := []struct {
		name, current string
		status        int
		body          string
		wantCode      int
		want          string
	}{
		{"newer available", "v0.1.0", 200, rel, 0, "New version available: v0.2.0 (you have v0.1.0)"},
		{"up to date", "v0.2.0", 200, rel, 0, "Up to date: v0.2.0 is the latest release."},
		{"ahead of latest", "v0.3.0", 200, rel, 0, "Up to date"},
		{"git describe build", "v0.2.0-4-gabc1234", 200, rel, 0, "development build of the latest release v0.2.0"},
		{"dev build", "dev", 200, rel, 0, "can't be compared"},
		{"no releases", "v0.1.0", 404, `{}`, 1, "no releases published yet"},
		{"rate limited", "v0.1.0", 403, `{}`, 1, "rate limit"},
		{"server error", "v0.1.0", 500, `{}`, 1, "500"},
		{"garbage json", "v0.1.0", 200, `not json`, 1, "bad response"},
		{"weird tag", "v0.1.0", 200, `{"tag_name":"latest"}`, 1, "unexpected release tag"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fakeGitHub(t, c.status, c.body)
			setVersion(t, c.current)
			var out, errOut bytes.Buffer
			code := checkUpdate(&out, &errOut, http.DefaultClient)
			if code != c.wantCode {
				t.Errorf("exit %d, want %d", code, c.wantCode)
			}
			if all := out.String() + errOut.String(); !strings.Contains(all, c.want) {
				t.Errorf("output missing %q:\n%s", c.want, all)
			}
		})
	}
}

func TestCheckUpdateShowsAssetForThisSystem(t *testing.T) {
	fakeGitHub(t, 200, `{"tag_name":"v9.9.9","html_url":"https://example.test"}`)
	setVersion(t, "v0.1.0")
	var out bytes.Buffer
	checkUpdate(&out, &bytes.Buffer{}, http.DefaultClient)
	if want := assetName("v9.9.9"); !strings.Contains(out.String(), want) {
		t.Errorf("output missing asset %q:\n%s", want, out.String())
	}
}

func TestGeneratingMakesNoNetworkCalls(t *testing.T) {
	// Any request to the update URL fails the test: normal use must stay offline.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected network call: %s", r.URL)
	}))
	defer srv.Close()
	old := latestReleaseURL
	latestReleaseURL = srv.URL
	defer func() { latestReleaseURL = old }()

	if code, _, _ := runCLI(t, "", "-l", "20", "-n", "5"); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if code, _, _ := runCLI(t, "20\n1\nhigh\nsafe\nn\nq\n", "-i"); code != 0 {
		t.Fatalf("interactive exit %d", code)
	}
}
