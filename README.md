# RGB Go Bindings (UniFFI)

This package provides Go bindings for the RGB Rust library via [UniFFI](https://mozilla.github.io/uniffi-rs/). It supports macOS and Linux.

## Installation

```bash
go get github.com/UTEXO-Protocol/rgb-lib-go@latest
```

## Automatic Releases

This package is automatically rebuilt when a new version of [rgb-lib](https://github.com/UTEXO-Protocol/rgb-lib) is released. Pre-built binaries are available in the [Releases](https://github.com/UTEXO-Protocol/rgb-lib-go/releases) section.

## Manual Build Instructions

### 1. Clone the Repository

```bash
git clone https://github.com/UTEXO-Protocol/rgb-lib.git
cd rgb-lib/bindings/uniffi
```

### 2. Build the Rust Library

```bash
cargo build --release
```

### 3. Copy the Compiled Library

Copy the compiled `.dylib` (macOS) or `.so` (Linux) file from `target/release` into the `lib` directory:

```bash
mkdir -p lib
cp ../../target/release/librgblibuniffi.* lib/
```

### 4. Generate Go Bindings

```bash
uniffi-bindgen-go src/rgb-lib.udl
```

Copy the generated files (e.g., `rgb_lib.go`, `rgb_lib.h`) to the root of your Go package.

### 5. Add cgo Linker Flags

At the top of `rgb_lib.go`, insert:

```go
/*
#cgo LDFLAGS: -lrgblibuniffi -L${SRCDIR}/lib -Wl,-rpath,${SRCDIR}/lib
*/
```

### 6. Link the Shared Library

#### On macOS:

```bash
install_name_tool -id @rpath/librgblibuniffi.dylib lib/librgblibuniffi.dylib
```

#### On Linux:

```bash
patchelf --set-rpath '$ORIGIN/lib' lib/librgblibuniffi.so
```

### 7. Publish

Tag the release and push:

```bash
git tag v0.3.5
git push origin v0.3.5
```

