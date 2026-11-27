package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"runtime"
	"strconv"
	"time"
)

// The only network call passgen ever makes, and only on --check-update.
// It sends no data about you or your passwords: a plain GET to GitHub's API.
const repo = "bankierubybank/passgen"

// latestReleaseURL is a variable so tests can point it at a fake server.
var latestReleaseURL = "https://api.github.com/repos/" + repo + "/releases/latest"

// Only the tag is used. Links are built from it here rather than taken from
// the response, so nothing GitHub (or anyone in between) sends is printed
// unchecked.
type release struct {
	TagName string `json:"tag_name"`
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

// versionPattern is "v1.2.3" plus an optional suffix such as "-rc1" or the
// "-3-gabc123-dirty" that git describe adds. The suffix allows only letters,
// digits, dots and dashes: a release tag is printed inside a shell command
// for the user to copy, so quotes, $, ;, spaces or control characters must
// never get through.
var versionPattern = regexp.MustCompile(`^v?(\d{1,9})\.(\d{1,9})\.(\d{1,9})(?:-([0-9A-Za-z][0-9A-Za-z.-]{0,63}))?$`)

// parseVersion returns the numbers, whether there was a suffix, and whether
// v is a well-formed version at all.
func parseVersion(v string) (nums [3]int, suffix bool, ok bool) {
	m := versionPattern.FindStringSubmatch(v)
	if m == nil {
		return nums, false, false
	}
	for i := range nums {
		nums[i], _ = strconv.Atoi(m[i+1]) // at most 9 digits: always fits
	}
	return nums, m[4] != "", true
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
		client = &http.Client{
			// The request's own 15s deadline (fetchLatest) bounds the time.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if req.URL.Scheme != "https" {
					return errors.New("refusing a redirect away from https")
				}
				if len(via) >= 5 {
					return errors.New("too many redirects")
				}
				return nil
			},
		}
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
		fmt.Fprintf(stdout, "Release notes: https://github.com/%s/releases/tag/%s\n", repo, latest.TagName)
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
