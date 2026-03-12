# Workflows

## release.yml — Build and Release

Builds Go bindings and publishes a GitHub Release.

- **Trigger**: automatically via `repository_dispatch` from `rgb-lib`, or manually
- **Input**: `rgb_lib_version` (e.g. `v0.3.0-beta.15`)
- **What it does**:
  1. Auto-generates `rgb_lib.go` and `rgb_lib.h` from `rgb-lib` source using `uniffi-bindgen-go`
  2. Commits updated bindings to `main`
  3. Downloads the native library from `rgb-lib` release
  4. Builds for Linux x64 and macOS ARM64
  5. Creates a GitHub Release with the tag and platform zips

## test.yml — Test Library (local)

Tests the Go bindings using **local** (checked out) code.

- **Trigger**: push to `main`, PRs to `main`, after release workflow, or manually
- **What it does**:
  1. Downloads native library from latest `rgb-lib` release
  2. Builds the Go package
  3. Runs `lib_test` against the local code (`go mod edit -replace`)
  4. Tests on Ubuntu and macOS

## test_tag.yml — Test Library (published tag)

Tests the **published** Go module version — simulates what end users do.

- **Trigger**: push of a `v*` tag (i.e. right after a release)
- **What it does**:
  1. Downloads native library from latest `rgb-lib` release
  2. Installs the tagged version via `go get @<tag>` with `GOPROXY=direct`
  3. Runs `lib_test` against the published module
  4. Tests on Ubuntu and macOS
