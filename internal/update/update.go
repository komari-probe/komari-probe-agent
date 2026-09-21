// Package update implements explicitly requested Agent binary updates.
// It never starts a timer and is only invoked by the `komari-agent update` command.
package update

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	goupdate "github.com/inconshreveable/go-update"
)

const (
	Repo          = "komari-probe/komari-probe-agent"
	githubAPIBase = "https://api.github.com"
	checksumName  = "checksums.txt"
)

type Result struct {
	CurrentVersion, TargetVersion string
	Updated                       bool
}

type Client struct {
	HTTPClient *http.Client
	APIBaseURL string
	Executable func() (string, error)
	Apply      func(io.Reader, goupdate.Options) error
}

func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	return &Client{HTTPClient: httpClient, APIBaseURL: githubAPIBase, Executable: os.Executable, Apply: goupdate.Apply}
}

// Run updates only when this explicitly invoked command finds a newer release.
func (c *Client) Run(currentVersion string) (Result, error) {
	candidate, found, err := c.Find(currentVersion)
	if err != nil {
		return Result{}, err
	}
	if !found {
		return Result{CurrentVersion: currentVersion, TargetVersion: currentVersion}, nil
	}
	path, err := c.Executable()
	if err != nil {
		return Result{}, fmt.Errorf("resolve current executable: %w", err)
	}
	if err := c.downloadVerifyAndApply(candidate, path); err != nil {
		return Result{}, err
	}
	return Result{CurrentVersion: currentVersion, TargetVersion: candidate.Release.TagName, Updated: true}, nil
}
