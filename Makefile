UNIFFI_BINDGEN_GO_GIT := https://github.com/kixelated/uniffi-bindgen-go.git
UNIFFI_BINDGEN_GO_REV := 4f79e52bd8f518e5fa4d7acff9e586aee21e12a0

OS   := $(shell uname -s)
ARCH := $(shell uname -m)

ifeq ($(OS),Darwin)
  LIB_NAME := librgblibuniffi.dylib
else
  LIB_NAME := librgblibuniffi.so
endif

.PHONY: all build-lib generate build clean

all: build-lib generate

## Build the native library from the rgb-lib submodule
build-lib:
	@echo "==> Building rgb-lib uniffi ($(OS)/$(ARCH))..."
	cd rgb-lib/bindings/uniffi && cargo build --release
	mkdir -p lib
	cp rgb-lib/bindings/uniffi/target/release/$(LIB_NAME) lib/
ifeq ($(OS),Darwin)
	install_name_tool -id @rpath/$(LIB_NAME) lib/$(LIB_NAME)
endif
	@echo "==> Native library ready: lib/$(LIB_NAME)"

## Generate Go bindings from the UDL file
generate: build-lib
	@echo "==> Installing uniffi-bindgen-go..."
	cargo install --git $(UNIFFI_BINDGEN_GO_GIT) --rev $(UNIFFI_BINDGEN_GO_REV) uniffi-bindgen-go
	@echo "==> Generating Go bindings..."
	uniffi-bindgen-go rgb-lib/bindings/uniffi/src/rgb-lib.udl --out-dir /tmp/go-bindings
	cp $$(find /tmp/go-bindings -name "rgb_lib.go" | head -1) rgb_lib.go
	cp $$(find /tmp/go-bindings -name "rgb_lib.h"  | head -1) rgb_lib.h
	python3 - <<'PY'
import pathlib
p = pathlib.Path("rgb_lib.go"); s = p.read_text()
old = '// #include <rgb_lib.h>\nimport "C"'
new = ('/*\n#cgo LDFLAGS: -lrgblibuniffi -L$${SRCDIR}/lib -Wl,-rpath,$${SRCDIR}/lib\n'
       '#include <rgb_lib.h>\n*/\nimport "C"')
if s.count(old) == 1: p.write_text(s.replace(old, new))
PY
	@echo "==> Bindings ready: rgb_lib.go rgb_lib.h"

## Build and verify the Go package
build:
	CGO_ENABLED=1 go build ./...

## Update the submodule to latest upstream
update-submodule:
	git submodule update --remote rgb-lib
	@echo "==> Submodule updated to: $$(git -C rgb-lib rev-parse --short HEAD)"

clean:
	rm -f lib/librgblibuniffi.*
	rm -rf rgb-lib/bindings/uniffi/target
