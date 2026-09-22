package update

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type Release struct {
	TagName    string    `json:"tag_name"`
	Draft      bool      `json:"draft"`
	Prerelease bool      `json:"prerelease"`
	Published  time.Time `json:"published_at"`
	Assets     []Asset   `json:"assets"`
}
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}
type Candidate struct {
	Release  Release
	Binary   Asset
	Checksum Asset
}

func (c *Client) Find(current string) (Candidate, bool, error) {
	asset := expectedAssetName(runtime.GOOS, runtime.GOARCH)
	if isNightly(current) {
		releases, err := c.listReleases()
		if err != nil {
			return Candidate{}, false, err
		}
		candidate, ok := latestNightly(releases, asset)
		return candidate, ok && candidate.Release.TagName != current, nil
	}
	release, err := c.latestStable()
	if err != nil {
		return Candidate{}, false, err
	}
	candidate, ok := releaseCandidate(release, asset)
	if !ok {
		return Candidate{}, false, fmt.Errorf("release %s does not contain %s and %s", release.TagName, asset, checksumName)
	}
	needed, err := stableNeedsUpdate(current, release.TagName)
	return candidate, needed, err
}

func (c *Client) latestStable() (Release, error) {
	var r Release
	if err := c.getJSON("/repos/"+Repo+"/releases/latest", &r); err != nil {
		return r, err
	}
	if r.Draft || r.Prerelease {
		return r, errors.New("GitHub latest release must be a published stable release")
	}
	return r, nil
}
func (c *Client) listReleases() ([]Release, error) {
	var r []Release
	return r, c.getJSON("/repos/"+Repo+"/releases?per_page=100", &r)
}
func (c *Client) getJSON(path string, target any) error {
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(c.APIBaseURL, "/")+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "sonar-agent-updater")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request GitHub release metadata: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GitHub release metadata returned HTTP %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode GitHub release metadata: %w", err)
	}
	return nil
}
func expectedAssetName(goos, goarch string) string {
	n := "sonar-agent-" + goos + "-" + goarch
	if goos == "windows" {
		n += ".exe"
	}
	return n
}
func isNightly(v string) bool {
	return strings.HasPrefix(v, "nightly-") || strings.HasPrefix(v, "Snapshot-")
}
func latestNightly(rs []Release, name string) (Candidate, bool) {
	var latest Candidate
	found := false
	for _, r := range rs {
		if r.Draft || !r.Prerelease || !isNightly(r.TagName) {
			continue
		}
		c, ok := releaseCandidate(r, name)
		if !ok {
			continue
		}
		if !found || r.Published.After(latest.Release.Published) || (r.Published.Equal(latest.Release.Published) && r.TagName > latest.Release.TagName) {
			latest, found = c, true
		}
	}
	return latest, found
}
func releaseCandidate(r Release, name string) (Candidate, bool) {
	var b, s Asset
	for _, a := range r.Assets {
		if a.Name == name {
			b = a
		}
		if a.Name == checksumName {
			s = a
		}
	}
	return Candidate{r, b, s}, b.URL != "" && s.URL != ""
}
func stableNeedsUpdate(current, latest string) (bool, error) {
	a, e := versionParts(current)
	if e != nil {
		return false, fmt.Errorf("parse current version %q: %w", current, e)
	}
	b, e := versionParts(latest)
	if e != nil {
		return false, fmt.Errorf("parse latest version %q: %w", latest, e)
	}
	for i := range a {
		if a[i] != b[i] {
			return b[i] > a[i], nil
		}
	}
	return false, nil
}
func versionParts(v string) ([3]int, error) {
	var out [3]int
	v = strings.SplitN(strings.TrimPrefix(strings.TrimPrefix(v, "v"), "V"), "-", 2)[0]
	p := strings.Split(v, ".")
	if len(p) != 3 {
		return out, errors.New("expected major.minor.patch")
	}
	for i, x := range p {
		n, e := strconv.Atoi(x)
		if e != nil || n < 0 {
			return out, errors.New("invalid numeric version")
		}
		out[i] = n
	}
	return out, nil
}
