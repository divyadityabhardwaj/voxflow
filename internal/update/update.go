// Package update discovers, verifies and installs VoxFlow releases from GitHub.
package update

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const repository = "divyadityabhardwaj/voxflow"
const latestURL = "https://api.github.com/repos/" + repository + "/releases/latest"

type Info struct {
	Current    string   `json:"current"`
	Latest     string   `json:"latest"`
	URL        string   `json:"url"` // the release page
	Available  bool     `json:"available"`
	Summary    []string `json:"summary"`
	CanInstall bool     `json:"can_install"`
	Reason     string   `json:"reason"`
	State      string   `json:"state"`
	Downloaded int64    `json:"downloaded"`
	Total      int64    `json:"total"`
	Error      string   `json:"error"`
	asset      releaseAsset
	tag        string
}

// The default transport already honours HTTPS_PROXY and friends.
var client = &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 || req.URL.Scheme != "https" {
		return fmt.Errorf("unsafe update redirect")
	}
	return nil
}}

type releaseAsset struct {
	Name   string `json:"name"`
	URL    string `json:"browser_download_url"`
	Digest string `json:"digest"`
	Size   int64  `json:"size"`
}

// Check asks GitHub for the latest release and compares it with current.
func Check(ctx context.Context, current string) (Info, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	info := Info{Current: current, State: "idle", Summary: []string{}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestURL, nil)
	if err != nil {
		return info, err
	}
	req.Header.Set("User-Agent", "VoxFlow/"+current)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return info, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return info, fmt.Errorf("update check: GitHub returned %s", resp.Status)
	}
	var release struct {
		TagName    string         `json:"tag_name"`
		Body       string         `json:"body"`
		Draft      bool           `json:"draft"`
		Prerelease bool           `json:"prerelease"`
		Assets     []releaseAsset `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&release); err != nil {
		return info, fmt.Errorf("update check: %w", err)
	}
	info.Latest = strings.TrimPrefix(release.TagName, "v")
	info.tag = release.TagName
	info.URL = "https://github.com/" + repository + "/releases/tag/" + url.PathEscape(release.TagName)
	info.Available = !release.Draft && !release.Prerelease && Newer(info.Latest, current)
	info.Summary = summarize(release.Body)
	if !info.Available {
		return info, nil
	}
	info.Reason = "This release doesn't include an in-app update. Download it from the release page."
	for _, asset := range release.Assets {
		if validAsset(asset, release.TagName, runtime.GOOS, runtime.GOARCH) {
			info.asset = asset
			info.Total = asset.Size
			info.CanInstall = true
			info.Reason = ""
			break
		}
	}
	return info, nil
}

func validAsset(a releaseAsset, tag, goos, arch string) bool {
	name := "voxflow-" + goos + "-" + arch + ".zip"
	expected := "https://github.com/" + repository + "/releases/download/" + url.PathEscape(tag) + "/" + name
	digest, err := hex.DecodeString(strings.TrimPrefix(a.Digest, "sha256:"))
	return goos == "darwin" && a.Name == name && a.URL == expected &&
		strings.HasPrefix(a.Digest, "sha256:") && err == nil && len(digest) == 32 && a.Size > 0 && a.Size <= maxDownload
}

var markdownLink = regexp.MustCompile(`\[([^\]]+)\]\([^)]*\)`)

// Show a bounded plain-text summary, never HTML supplied by a release author.
func summarize(body string) []string {
	result := []string{}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- ") && !strings.HasPrefix(line, "* ") {
			continue
		}
		line = strings.TrimSpace(line[2:])
		line = markdownLink.ReplaceAllString(line, "$1")
		line = strings.NewReplacer("**", "", "`", "").Replace(line)
		if line == "" {
			continue
		}
		if runes := []rune(line); len(runes) > 180 {
			line = string(runes[:177]) + "…"
		}
		result = append(result, line)
		if len(result) == 5 {
			break
		}
	}
	return result
}

// Newer reports whether version a is later than b. Both are dotted numbers
// with an optional leading "v"; anything after "-" or "+" is ignored, and
// unparsable versions are never newer.
func Newer(a, b string) bool {
	pa, okA := parse(a)
	pb, okB := parse(b)
	if !okA || !okB {
		return false
	}
	for i := range max(len(pa), len(pb)) {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x != y {
			return x > y
		}
	}
	return false
}

func parse(v string) ([]int, bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	if v == "" {
		return nil, false
	}
	var parts []int
	for _, s := range strings.Split(v, ".") {
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 {
			return nil, false
		}
		parts = append(parts, n)
	}
	return parts, true
}
