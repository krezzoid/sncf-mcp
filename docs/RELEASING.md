# Releasing

A release is cut by pushing a semver tag. CI then runs GoReleaser to build
cross-platform binaries + checksums and attach them to a GitHub Release.

## Cut a release

```sh
git checkout main && git pull
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0
```

The `release` job in [`.github/workflows/ci.yml`](../.github/workflows/ci.yml)
runs `goreleaser release --clean` on tags, after `test`, `lint`, and `vuln`
pass. The tag version is stamped into the binary via ldflags
(`internal/server.Version`), so `sncf-mcp` reports the real version.

## Dry run locally

```sh
goreleaser check                       # validate .goreleaser.yaml
goreleaser release --snapshot --clean  # build dist/ without publishing
```

## Container image

The image is built from the [`Dockerfile`](../Dockerfile) (distroless, nonroot,
~10 MB) and is **not auto-published** yet:

```sh
docker build --build-arg VERSION=v0.1.0 -t sncf-mcp:v0.1.0 .
```

## Notes

- The Go module path is `github.com/krezzoid/sncf-mcp`. For public `go install`
  to work, the hosting repository must be public and match that path.
- Public distribution is served from a dedicated public mirror repository;
  publishing is intentionally not wired into this (private) repo.
