# GitHub Actions

Komari Probe Agent uses three workflows with non-overlapping responsibilities.

| Workflow | Trigger | Output |
| --- | --- | --- |
| `ci.yaml` | Pull requests to `main`, manual dispatch | Tests, vet, and native builds on Linux, Windows, and macOS |
| `nightly.yaml` | Pushes to `main`, manual dispatch from `main` | Latest nightly prerelease and `ghcr.io/...:nightly` |
| `release.yaml` | Published stable release, or a manual existing tag | Binaries, checksums, Docker images, and notes |

## CI

`ci.yaml` is the branch-protection workflow. It does not publish artifacts and
should be configured as a required check for `main`.

## Nightly releases

`nightly.yaml` accepts only the current `main` commit. It creates a prerelease
tagged `nightly-YYYYMMDD-HHMM` in UTC, removes older `nightly-*` prereleases,
and publishes the mutable `:nightly` Docker tag. Komari Probe Agent does not self-update;
container users must recreate or update their container.

## Stable releases

Create a stable GitHub Release first, then publish it. `release.yaml` validates
that the target is not a prerelease, checks out its tag, and uses GoReleaser as
the single source of binary assets. It uploads binaries and `checksums.txt`,
publishes the tag plus `:latest` Docker images, then generates release notes.

Manual stable publishing requires an existing release tag and never substitutes
the branch head. Do not use it for nightly tags.

## Asset conventions

- Binaries are named `komari-agent-${GOOS}-${GOARCH}`; Windows adds `.exe`.
- The Dockerfile requires `komari-agent-linux-amd64` and
  `komari-agent-linux-arm64` in its build context.
- The version is embedded through `internal/version.CurrentVersion`.
- Workflows build with Go 1.27.1, the current patch release that fixes the
  reachable standard-library vulnerabilities found by `govulncheck`; `go.mod`
  still records the minimum supported Go version.
- `checksums.txt` is regenerated after GoReleaser's layout is flattened, so it
  hashes the exact files uploaded to GitHub Releases.
- Release and nightly binaries receive signed GitHub artifact attestations;
  published container images receive signed registry attestations. Verify a
  downloaded binary with `gh attestation verify <file> --repo
  komari-probe/komari-probe-agent`.
