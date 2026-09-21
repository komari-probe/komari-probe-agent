package update

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	goupdate "github.com/inconshreveable/go-update"
)

func (c *Client) downloadVerifyAndApply(candidate Candidate, executable string) error {
	expected, err := c.downloadChecksum(candidate.Checksum.URL, candidate.Binary.Name)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp("", "komari-agent-update-*")
	if err != nil {
		return fmt.Errorf("create update download: %w", err)
	}
	path := temporary.Name()
	defer os.Remove(path)
	actual, err := c.downloadAsset(temporary, candidate.Binary)
	closeErr := temporary.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return fmt.Errorf("close update download: %w", closeErr)
	}
	if actual != expected {
		return fmt.Errorf("SHA-256 verification failed for %s", candidate.Binary.Name)
	}
	verified, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open verified update: %w", err)
	}
	defer verified.Close()
	info, err := os.Stat(executable)
	if err != nil {
		return fmt.Errorf("stat current executable: %w", err)
	}
	if err := c.Apply(verified, goupdate.Options{TargetPath: executable, TargetMode: info.Mode()}); err != nil {
		return fmt.Errorf("replace current executable: %w", err)
	}
	return nil
}

func (c *Client) downloadChecksum(url, binary string) (string, error) {
	resp, err := c.get(url)
	if err != nil {
		return "", fmt.Errorf("download checksum manifest: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read checksum manifest: %w", err)
	}
	return checksumFor(string(body), binary)
}
func (c *Client) downloadAsset(dst *os.File, a Asset) (string, error) {
	resp, err := c.get(a.URL)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", a.Name, err)
	}
	defer resp.Body.Close()
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(dst, h), io.LimitReader(resp.Body, a.Size+1))
	if err != nil {
		return "", fmt.Errorf("save %s: %w", a.Name, err)
	}
	if a.Size > 0 && n != a.Size {
		return "", fmt.Errorf("downloaded %s has size %d, want %d", a.Name, n, a.Size)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func (c *Client) get(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "komari-probe-agent-updater")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return resp, nil
}
func checksumFor(manifest, name string) (string, error) {
	var match string
	for _, line := range strings.Split(manifest, "\n") {
		f := strings.Fields(line)
		if len(f) != 2 || strings.TrimPrefix(f[1], "*") != name || len(f[0]) != sha256.Size*2 {
			continue
		}
		if _, err := hex.DecodeString(f[0]); err != nil {
			continue
		}
		if match != "" {
			return "", fmt.Errorf("checksum manifest contains multiple entries for %s", name)
		}
		match = strings.ToLower(f[0])
	}
	if match == "" {
		return "", fmt.Errorf("checksum manifest has no SHA-256 entry for %s", name)
	}
	return match, nil
}
