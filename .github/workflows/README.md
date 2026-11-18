# Workflows

## release.yml — Build and Release

Builds Go bindings and publishes a GitHub Release. Has two independent modes,
selected automatically by the `mode` job:

- **download mode** — the default. Builds bindings from a release already
  published in a mirror repo (`rgb-lib` or `rgb-lib-bfa`) and downloads the
  pre-built native libraries from that release's assets instead of compiling
  them.
- **submodule build mode** — builds everything from source, straight out of
  the `rgb-lib` git submodule, pinned to an arbitrary commit. Use this when
  you need bindings for a commit that has **no** published release yet (e.g.
  an open upstream PR).

### Download mode (default)

- **Trigger**: automatically via `repository_dispatch` from `rgb-lib` /
  `rgb-lib-bfa`, or manually via `workflow_dispatch`
- **Inputs**: `rgb_lib_version` (e.g. `v0.3.0-beta.15`), `source_repo`
  (`rgb-lib` or `rgb-lib-bfa`), `branch` (optional, auto-selected if empty)
- **What it does**:
  1. Clones the mirror repo (`UTEXO-Protocol/rgb-lib` or
     `UTEXO-Protocol/rgb-lib-bfa`) at the given release tag
  2. Generates `rgb_lib.go` / `rgb_lib.h` with `uniffi-bindgen-go`
  3. Downloads the pre-built native libraries from that release's assets
     (linux x64, linux arm64, darwin arm64)
  4. Commits bindings + libraries to the target branch
  5. Builds and tests the Go package for linux/amd64 and darwin/arm64
  6. Publishes a GitHub Release with per-platform zips

This mode **requires a release to already exist** in the mirror repo with the
native library assets attached. It cannot build an arbitrary commit — for
that, use submodule build mode below.

### Submodule build mode — how to run it

**→ See [`RUNBOOK.md`](../../RUNBOOK.md) for the step-by-step guide** (how to
get the commit SHA, pick a tag, run the workflow, verify the result,
troubleshooting). The rest of this section documents the job graph itself,
for whoever maintains the pipeline rather than just runs it.

It's triggered via `workflow_dispatch` with two inputs:
`source_sha` (commit in `RGB-Tools/rgb-lib`) and `tag` (new tag to create).
When `source_sha` is set, the `mode` job routes the whole run to this path
and skips `generate`/`build`/`release` (the download-mode jobs):

1. `sub-meta` resolves and validates the inputs (branch, source_sha, tag)
2. `sub-bindings` checks out `release/upstream` with the submodule, pins
   `rgb-lib` to `source_sha`, and generates `rgb_lib.go` / `rgb_lib.h` —
   **note**: this must run with `working-directory:
   rgb-lib/bindings/uniffi`, because `cargo metadata` (invoked internally by
   `uniffi-bindgen-go`) resolves relative to the current directory, not the
   path passed on the command line. Running it from the repo root fails with
   `could not find Cargo.toml` (bug hit once already, now fixed — don't
   reintroduce it)
3. `sub-build-native` builds all three native libraries in parallel
   (`x86_64-unknown-linux-gnu`, `aarch64-unknown-linux-gnu` via cross-gcc,
   `aarch64-apple-darwin` on a macOS runner) — no caching, ~5-7 min per
   target on a cold run
4. `sub-finalize` downloads all four artifacts, updates the submodule
   pointer, and pushes **one commit** to `release/upstream` containing
   `rgb_lib.go`, `rgb_lib.h`, `lib/librgblibuniffi.so`,
   `lib/librgblibuniffi_arm64.so`, `lib/librgblibuniffi.dylib`, and the
   bumped submodule pointer — then creates and pushes the git tag on that
   commit
5. `sub-release` packages the same bindings + libraries into three
   per-platform zips and publishes a prerelease GitHub Release on the tag

This mode intentionally sets up no SSH keys or deploy keys — the target
commit's dependencies must resolve from crates.io on their own, or the
`sub-bindings`/`sub-build-native` jobs will fail. (`RUNBOOK.md` has the
exact check to run before triggering a build.)

#### Interaction with `watch-upstream.yml`

`watch-upstream.yml` (below) runs every 6 hours and unconditionally bumps the
`rgb-lib` submodule pointer on `release/upstream` to the latest *published,
non-draft* upstream release — regardless of what submodule build mode last
pinned it to. This is expected and harmless: each submodule-mode run pins
and commits its own exact `source_sha` at build time, and the tag + Release
it creates are permanent — a later cron bump never changes anything already
tagged/released. You'll just see a `chore: bump rgb-lib submodule to
upstream <tag>` commit show up shortly after any submodule-mode build in
`git log release/upstream` — that's the cron job doing its normal job, not a
conflict.

## watch-upstream.yml — Watch upstream rgb-lib

Keeps the `rgb-lib` submodule pointer on a branch tracking the latest
**published, non-draft** release of
[`RGB-Tools/rgb-lib`](https://github.com/RGB-Tools/rgb-lib) — it does not
track branches, PRs, or draft/unpublished releases.

- **Trigger**: `schedule` (every 6 hours), or manually via `workflow_dispatch`
  (`target_branch` input, default `release/upstream`)
- **What it does**:
  1. Looks up the latest non-draft release tag in `RGB-Tools/rgb-lib` via the
     GitHub API and resolves it to a commit SHA
  2. Checks out the target branch with the submodule
  3. If the submodule isn't already at that SHA, checks it out and pushes a
     `chore: bump rgb-lib submodule to upstream <tag>` commit

This is the *only* fully automatic path — it needs no human input, but it
can only ever reach commits that have an actual published GitHub Release
upstream. For anything else (an open PR, an unreleased commit on `next`),
use submodule build mode above.

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
