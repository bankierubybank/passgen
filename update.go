package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// The only network call passgen ever makes, and only on --check-update.
// It sends no data about you or your passwords: a plain GET to GitHub's API.
const repo = "bankierubybank/passgen"

// latestReleaseURL is a variable so tests can point it at a fake server.
var latestReleaseURL = "https://api.github.com/repos/" + repo + "/releases/latest"

type release struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

func fetchLatest(client *http.Client) (release, error) {
	var r release
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestReleaseURL, nil)
	if err != nil {
		return r, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "passgen/"+version)

	resp, err := client.Do(req)
	if err != nil {
		return r, fmt.Errorf("could not reach GitHub: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return r, errors.New("no releases published yet")
	case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests:
		return r, errors.New("GitHub API rate limit hit, try again later")
	case resp.StatusCode != http.StatusOK:
		return r, fmt.Errorf("GitHub API returned %s", resp.Status)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&r); err != nil {
		return r, fmt.Errorf("bad response from GitHub: %w", err)
	}
	if _, _, ok := parseVersion(r.TagName); !ok {
		return r, fmt.Errorf("unexpected release tag %q", r.TagName)
	}
	return r, nil
}

// parseVersion reads "v1.2.3" plus any suffix (e.g. "-rc1" or the
// "-3-gabc123-dirty" that git describe adds). Returns the numbers, whether
// there was a suffix, and whether it parsed at all.
func parseVersion(v string) (nums [3]int, suffix bool, ok bool) {
	v = strings.TrimPrefix(v, "v")
	core, rest, hasRest := strings.Cut(v, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return nums, false, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return nums, false, false
		}
		nums[i] = n
	}
	return nums, hasRest && rest != "", true
}

// newer reports whether version a is higher than b (numbers only).
func newer(a, b [3]int) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

// assetName is the release archive for the machine passgen is running on.
func assetName(tag string) string {
	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("passgen_%s_%s_%s%s", tag, runtime.GOOS, runtime.GOARCH, ext)
}

// checkUpdate prints whether a newer release exists. It never downloads or installs anything.
func checkUpdate(stdout, stderr io.Writer, client *http.Client) int {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	fmt.Fprintln(stderr, "Checking github.com/"+repo+" for the latest release...")
	latest, err := fetchLatest(client)
	if err != nil {
		return fail(stderr, err)
	}
	latestNums, _, _ := parseVersion(latest.TagName)

	cur, devSuffix, ok := parseVersion(version)
	switch {
	case !ok:
		fmt.Fprintf(stdout, "Latest release: %s\nThis is a development build (%s), so it can't be compared.\n", latest.TagName, version)
	case newer(latestNums, cur):
		asset := assetName(latest.TagName)
		fmt.Fprintf(stdout, "New version available: %s (you have %s)\n\n", latest.TagName, version)
		fmt.Fprintf(stdout, "Release notes: %s\n", latest.HTMLURL)
		fmt.Fprintf(stdout, "Download for this system: https://github.com/%s/releases/download/%s/%s\n", repo, latest.TagName, asset)
		fmt.Fprintf(stdout, "\nOr with the GitHub CLI:\n  gh release download %s -R %s -p '%s' -p checksums.txt\n", latest.TagName, repo, asset)
		fmt.Fprintln(stdout, "\nVerify the download against checksums.txt before installing.")
	case devSuffix && !newer(cur, latestNums):
		fmt.Fprintf(stdout, "Up to date: %s is a development build of the latest release %s.\n", version, latest.TagName)
	default:
		fmt.Fprintf(stdout, "Up to date: %s is the latest release.\n", version)
	}
	return 0
}
