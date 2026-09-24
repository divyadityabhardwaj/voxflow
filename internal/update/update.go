// Package update checks GitHub for a newer release. It never installs one:
// the app is unsigned, so the user downloads it themselves.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const latestURL = "https://api.github.com/repos/divyadityabhardwaj/voxflow/releases/latest"

type Info struct {
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	URL       string `json:"url"` // the release page
	Available bool   `json:"available"`
}

// The default transport already honours HTTPS_PROXY and friends.
var client = &http.Client{Timeout: 10 * time.Second}

// Check asks GitHub for the latest release and compares it with current.
func Check(ctx context.Context, current string) (Info, error) {
	info := Info{Current: current}
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
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return info, fmt.Errorf("update check: %w", err)
	}
	info.Latest = strings.TrimPrefix(release.TagName, "v")
	info.URL = release.HTMLURL
	info.Available = Newer(info.Latest, current)
	return info, nil
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
