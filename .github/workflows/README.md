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

### Submodule build mode — runbook: build a release from one commit

Use this whenever someone asks for Go bindings built from a specific
upstream `rgb-lib` commit that has **no release yet** — e.g. "build from the
head of PR #124" or "build from commit `<sha>`".

#### Before you start — two things to check

1. **The commit must be public**, i.e. it must exist in
   [`RGB-Tools/rgb-lib`](https://github.com/RGB-Tools/rgb-lib) — that's where
   this repo's `rgb-lib` submodule points (see `.gitmodules`). A commit that
   only exists in someone's fork won't be fetchable.
2. **No private patches.** This mode doesn't set up any SSH keys/deploy
   keys, so the commit's dependencies must all resolve from crates.io on
   their own. Run this — both commands must print nothing:
   ```bash
   gh api "repos/RGB-Tools/rgb-lib/contents/Cargo.toml?ref=<sha>" --jq '.content' | base64 -d | grep -A5 '\[patch'
   gh api "repos/RGB-Tools/rgb-lib/contents/bindings/uniffi/Cargo.toml?ref=<sha>" --jq '.content' | base64 -d | grep -A5 '\[patch'
   ```
   If either prints a `[patch.crates-io]` block, stop — this mode will fail;
   use the download-mode pipeline via rgb-lib-bfa instead.

#### Step by step

1. **Copy the commit hash.** Open the commit (or the PR, then its latest
   commit) on GitHub and copy the full 40-character SHA — the long hash from
   the commit page URL or `git log`, not a shortened one.

2. **Pick a tag name.** Convention so far: `v<rgb-lib-version>-upstream-dev-bfa.<n>`
   — e.g. the previous one was `v0.3.0-beta.8-upstream-dev-bfa.1`, so the
   next build against the same base version is
   `v0.3.0-beta.8-upstream-dev-bfa.2`. Bump `<n>` for another build against
   the same base version; bump the version part for a new one.

   Make sure it's not already taken:
   ```bash
   gh api repos/UTEXO-Protocol/rgb-lib-go/git/refs/tags/<your-tag>
   ```
   This must print `Not Found` (404). If it prints a ref, pick another tag —
   tags are never reused/moved.

3. **Open the workflow page**: go to
   https://github.com/UTEXO-Protocol/rgb-lib-go/actions/workflows/release.yml

4. Click the **"Run workflow"** button (top right of the runs list, opens a
   dropdown form).

5. In the dropdown form, fill in exactly these fields:
   - **Use workflow from**: select the branch `release/upstream`
   - **rgb_lib_version**: leave **empty**
   - **source_repo**: leave on the default (`rgb-lib`) — it's ignored in this mode
   - **branch**: leave **empty** (defaults to `release/upstream`)
   - **source_sha**: paste the commit hash you copied in step 1
   - **tag**: type the tag name you picked in step 2

6. Click the green **"Run workflow"** button to submit the form.

7. A new run appears in the list within a few seconds — click it to watch
   progress. It has these jobs: `Resolve build mode` → `Resolve submodule
   build context` → `Generate Go bindings (submodule)` +
   `Build native (×3, in parallel)` → `Commit bindings, libraries and tag` →
   `Create Release`. (You'll also see `Generate Go bindings`, `Build`,
   `Create Release` *without* "submodule" in grey/skipped — that's the
   other mode's jobs, correctly not running. Not an error.)

8. **Cold builds take ~8-10 minutes** (no Cargo cache) — the three
   `Build native (...)` jobs run in parallel and are the slowest part.

9. When it's green, your release is at:
   `https://github.com/UTEXO-Protocol/rgb-lib-go/releases/tag/<your-tag>`
   with three zips attached (`rgb-lib-go-linux-amd64.zip`,
   `rgb-lib-go-linux-arm64.zip`, `rgb-lib-go-darwin-arm64.zip`), and the
   commit it was built from is tagged on `release/upstream`.

#### Same thing via `gh` CLI, if you prefer not to click through the UI

```bash
gh workflow run release.yml --repo UTEXO-Protocol/rgb-lib-go --ref release/upstream \
  -f source_sha=<40-char commit sha from RGB-Tools/rgb-lib> \
  -f tag=<new tag, e.g. v0.3.0-beta.8-upstream-dev-bfa.2> \
  -f branch=release/upstream
```

#### Verifying it worked

```bash
gh run view --repo UTEXO-Protocol/rgb-lib-go <run-id>           # all jobs green
gh release view <tag> --repo UTEXO-Protocol/rgb-lib-go          # release + 3 assets
gh api repos/UTEXO-Protocol/rgb-lib-go/commits/<tag> --jq '.files[].filename'
# → lib/librgblibuniffi.dylib, lib/librgblibuniffi.so, lib/librgblibuniffi_arm64.so, rgb-lib, rgb_lib.go, rgb_lib.h
```

#### Known pitfall (already fixed once, mentioning in case it regresses)

The bindings-generation step must run with its working directory set to
`rgb-lib/bindings/uniffi` (where that commit's `Cargo.toml` lives) — running
`uniffi-bindgen-go` from the repo root fails with `could not find
Cargo.toml`, because `cargo metadata` resolves relative to the current
directory. If you see that error, check the `Generate bindings` step's
`working-directory` in the `sub-bindings` job.

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
