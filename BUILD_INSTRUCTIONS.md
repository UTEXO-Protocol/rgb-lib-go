# RGB Go Bindings — Build Instructions

This branch builds the native library directly from the
[`rgb-lib`](https://github.com/RGB-Tools/rgb-lib) git submodule
instead of downloading pre-built binaries.

## Prerequisites

- Rust toolchain (`rustup install stable`)
- Go 1.22+

## Quick start

```bash
git clone --recurse-submodules https://github.com/UTEXO-Protocol/rgb-lib-go.git
cd rgb-lib-go
make          # build native lib + generate Go bindings
make build    # verify Go package compiles
```

## Individual steps

```bash
make build-lib   # cargo build --release in rgb-lib/bindings/uniffi
make generate    # uniffi-bindgen-go → rgb_lib.go + rgb_lib.h
make build       # go build ./...
```

## Updating to a newer rgb-lib version

```bash
make update-submodule     # pull latest upstream commit
# or pin to a specific tag:
git -C rgb-lib checkout v0.3.0-beta.8
git add rgb-lib
git commit -m "chore: bump rgb-lib submodule to v0.3.0-beta.8"
# then push — CI will rebuild and commit the new bindings
```

## CI

The workflow (`.github/workflows/release.yml`) triggers on pushes to
`feat/submodule-build` that update `.gitmodules` or the `rgb-lib` submodule
pointer. It:

1. Checks out with `submodules: recursive`
2. Builds `librgblibuniffi` from source for each platform
3. Generates Go bindings from `rgb-lib/bindings/uniffi/src/rgb-lib.udl`
4. Commits the result back to the branch and uploads platform artifacts
