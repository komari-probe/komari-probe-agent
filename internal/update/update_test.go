package update

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	goupdate "github.com/inconshreveable/go-update"
)

func TestRunDownloadsVerifiedReleaseBeforeApplying(t *testing.T) {
	binary := []byte("verified replacement binary")
	hash := sha256.Sum256(binary)
	assetName := expectedAssetName(runtime.GOOS, runtime.GOARCH)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/" + Repo + "/releases/latest":
			_ = json.NewEncoder(w).Encode(Release{TagName: "v0.0.2", Assets: []Asset{
				{Name: assetName, URL: serverURL(r, "/binary"), Size: int64(len(binary))},
				{Name: checksumName, URL: serverURL(r, "/checksums")},
			}})
		case "/checksums":
			_, _ = w.Write([]byte(hex.EncodeToString(hash[:]) + "  " + assetName + "\n"))
		case "/binary":
			_, _ = w.Write(binary)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	target := filepath.Join(t.TempDir(), "komari-agent")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	applied := false
	client := NewClient(server.Client())
	client.APIBaseURL = server.URL
	client.Executable = func() (string, error) { return target, nil }
	client.Apply = func(reader io.Reader, options goupdate.Options) error {
		applied = true
		if options.TargetPath != target {
			t.Errorf("TargetPath = %q, want %q", options.TargetPath, target)
		}
		got, err := io.ReadAll(reader)
		if err != nil {
			return err
		}
		if !bytes.Equal(got, binary) {
			t.Errorf("replacement = %q, want %q", got, binary)
		}
		return nil
	}

	result, err := client.Run("0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if !applied || !result.Updated || result.TargetVersion != "v0.0.2" {
		t.Fatalf("result = %#v, applied = %v", result, applied)
	}
}

func TestRunRejectsChecksumMismatch(t *testing.T) {
	binary := []byte("untrusted")
	assetName := expectedAssetName(runtime.GOOS, runtime.GOARCH)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/" + Repo + "/releases/latest":
			_ = json.NewEncoder(w).Encode(Release{TagName: "0.0.2", Assets: []Asset{{Name: assetName, URL: serverURL(r, "/binary"), Size: int64(len(binary))}, {Name: checksumName, URL: serverURL(r, "/checksums")}}})
		case "/checksums":
			_, _ = w.Write([]byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  " + assetName + "\n"))
		case "/binary":
			_, _ = w.Write(binary)
		}
	}))
	defer server.Close()
	client := NewClient(server.Client())
	client.APIBaseURL = server.URL
	client.Executable = func() (string, error) { return filepath.Join(t.TempDir(), "agent"), nil }
	client.Apply = func(io.Reader, goupdate.Options) error {
		t.Fatal("Apply must not run after checksum failure")
		return nil
	}
	if _, err := client.Run("0.0.1"); err == nil {
		t.Fatal("Run() succeeded with an invalid checksum")
	}
}

func TestFindSkipsCurrentStableVersion(t *testing.T) {
	assetName := expectedAssetName(runtime.GOOS, runtime.GOARCH)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Release{TagName: "v1.2.3", Assets: []Asset{{Name: assetName, URL: "https://example.invalid/agent"}, {Name: checksumName, URL: "https://example.invalid/checksums"}}})
	}))
	defer server.Close()
	client := NewClient(server.Client())
	client.APIBaseURL = server.URL
	_, found, err := client.Find("1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("Find() reported an update for the current version")
	}
}

func TestLatestNightlyUsesNewestCompatiblePrerelease(t *testing.T) {
	assetName := expectedAssetName("linux", "amd64")
	old := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	newer := old.Add(time.Hour)
	candidate, found := latestNightly([]Release{
		{TagName: "nightly-old", Prerelease: true, Published: old, Assets: []Asset{{Name: assetName, URL: "binary"}, {Name: checksumName, URL: "checksums"}}},
		{TagName: "nightly-new", Prerelease: true, Published: newer, Assets: []Asset{{Name: assetName, URL: "binary"}, {Name: checksumName, URL: "checksums"}}},
		{TagName: "nightly-incomplete", Prerelease: true, Published: newer.Add(time.Hour), Assets: []Asset{{Name: assetName, URL: "binary"}}},
	}, assetName)
	if !found || candidate.Release.TagName != "nightly-new" {
		t.Fatalf("candidate = %#v, found = %v", candidate, found)
	}
}

func TestChecksumFor(t *testing.T) {
	valid := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	got, err := checksumFor(valid+" *komari-agent-linux-amd64\n", "komari-agent-linux-amd64")
	if err != nil || got != valid {
		t.Fatalf("checksumFor() = %q, %v", got, err)
	}
	if _, err := checksumFor(valid+" a\n"+valid+" a\n", "a"); err == nil {
		t.Fatal("checksumFor accepted duplicate entries")
	}
}

func serverURL(request *http.Request, path string) string { return "http://" + request.Host + path }
