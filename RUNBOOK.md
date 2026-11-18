# Runbook: build rgb-lib-go from a specific RGB-Tools/rgb-lib commit

**When to use this:** someone needs Go bindings + native libraries built
from a specific commit in the public
[`RGB-Tools/rgb-lib`](https://github.com/RGB-Tools/rgb-lib) repo — usually
the head of an open PR — that has **no published release** yet. If a
release already exists upstream, you don't need this; the regular
`repository_dispatch` pipeline (or `watch-upstream.yml`) handles it on its
own.

Two real examples already done this way:
`v0.3.0-beta.8-upstream-dev-bfa.1` (built from PR #124) and
`v0.3.0-beta.8-upstream-dev-bfa.2` (built from a later commit on the same
PR).

## What you need

- `gh` CLI, authenticated with push access to `UTEXO-Protocol/rgb-lib-go`
  (`gh auth status`)
- The commit SHA you want to build, from `RGB-Tools/rgb-lib`

## Step 1 — Get the commit SHA

Open the commit (or the PR → its latest/relevant commit) on
`RGB-Tools/rgb-lib` and copy the full 40-character hash. Example:
`286baf80c9f043afb5834094adf6a7efb2dfda3e`.

## Step 2 — Confirm it's buildable with this pipeline

This pipeline does **not** set up any SSH keys or private deploy keys. It
only works if the commit's Rust dependencies resolve entirely from
crates.io. Run both of these — **both must print nothing**:

```bash
SHA=286baf80c9f043afb5834094adf6a7efb2dfda3e   # replace with your sha

gh api "repos/RGB-Tools/rgb-lib/contents/Cargo.toml?ref=$SHA" \
  --jq '.content' | base64 -d | grep -A5 '\[patch'

gh api "repos/RGB-Tools/rgb-lib/contents/bindings/uniffi/Cargo.toml?ref=$SHA" \
  --jq '.content' | base64 -d | grep -A5 '\[patch'
```

If either prints a `[patch.crates-io]` block, **stop** — this commit
depends on private forks and this pipeline will fail at the bindings step.
That needs the separate `rgb-lib-bfa` download-mode pipeline instead (ask
before improvising there).

## Step 3 — Pick a tag name

Convention: `v<rgb-lib-version>-upstream-dev-bfa.<n>`.

- Same base version as the last build → bump `<n>` (`...bfa.1` → `...bfa.2`)
- New base version (e.g. upstream bumped `Cargo.toml` to `0.3.0-beta.9`) →
  use that version with `.1`

Confirm it's free — this **must** print `Not Found` (404):

```bash
TAG=v0.3.0-beta.8-upstream-dev-bfa.2   # replace with your tag
gh api repos/UTEXO-Protocol/rgb-lib-go/git/refs/tags/$TAG
```

If it prints a ref instead of 404, pick a different tag. Tags are never
reused or moved once created.

## Step 4 — Run the build

Pick whichever of these two is easier for you — they do the exact same
thing.

### Option A — GitHub UI (click-by-click)

1. Open https://github.com/UTEXO-Protocol/rgb-lib-go/actions/workflows/release.yml
2. Click the **"Run workflow"** button — top right of the runs list, just
   under the page title. It opens a dropdown form.
3. In that dropdown form:
   - **Use workflow from**: click this dropdown and select the branch
     `release/upstream` (it defaults to the repo's default branch, which is
     *not* `release/upstream` — you must change it)
   - **rgb_lib_version**: leave the text box **empty**
   - **source_repo**: leave the dropdown on its default (`rgb-lib`) —
     ignored in this mode either way
   - **branch**: leave the text box **empty** (it auto-fills to
     `release/upstream`)
   - **source_sha**: paste the 40-character commit hash from Step 1
   - **tag**: type the tag name you picked in Step 3 (e.g.
     `v0.3.0-beta.8-upstream-dev-bfa.2`)
4. Click the green **"Run workflow"** button at the bottom of the dropdown
   to submit it.
5. The dropdown closes and nothing visibly happens for a few seconds —
   reload the page (or wait ~10s) and a new run with a yellow/spinning icon
   appears at the top of the runs list. Click it to watch progress (see
   Step 5 below).

### Option B — `gh` CLI

```bash
gh workflow run release.yml --repo UTEXO-Protocol/rgb-lib-go --ref release/upstream \
  -f source_sha=$SHA \
  -f tag=$TAG \
  -f branch=release/upstream
```

Leave `rgb_lib_version` and `source_repo` unset — they only apply to the
other (download) mode and are ignored here.

## Step 5 — Watch it run

```bash
gh run list --repo UTEXO-Protocol/rgb-lib-go --workflow release.yml --limit 1
# copy the run id, then:
gh run watch <run-id> --repo UTEXO-Protocol/rgb-lib-go
```

Expect ~8-10 minutes on a cold run (no Cargo cache). You'll see these jobs
run: `Resolve build mode` → `Resolve submodule build context` →
`Generate Go bindings (submodule)` + `Build native (×3, in parallel)` →
`Commit bindings, libraries and tag` → `Create Release`. The *other* set of
jobs (`Generate Go bindings`, `Build`, `Create Release` without
"submodule") will show as grey/skipped — that's correct, that's the
download-mode path not running.

## Step 6 — Verify

```bash
gh release view $TAG --repo UTEXO-Protocol/rgb-lib-go
# → should show 3 assets: rgb-lib-go-linux-amd64.zip, rgb-lib-go-linux-arm64.zip,
#   rgb-lib-go-darwin-arm64.zip

gh api repos/UTEXO-Protocol/rgb-lib-go/commits/$TAG --jq '.files[].filename'
# → lib/librgblibuniffi.dylib, lib/librgblibuniffi.so, lib/librgblibuniffi_arm64.so,
#   rgb-lib, rgb_lib.go, rgb_lib.h
```

Done — the release is at
`https://github.com/UTEXO-Protocol/rgb-lib-go/releases/tag/<tag>`, and
`go get github.com/UTEXO-Protocol/rgb-lib-go@<tag>` picks it up.

## Troubleshooting

- **"release not found" / "error running cargo metadata ... could not find
  Cargo.toml"** — these are old bugs from the first two runs, already
  fixed. If either reappears, check `.github/workflows/release.yml`: the
  `Generate bindings` step in the `sub-bindings` job must have
  `working-directory: rgb-lib/bindings/uniffi`.
- **A `chore: bump rgb-lib submodule to upstream <tag>` commit appears on
  `release/upstream` right after your build** — that's `watch-upstream.yml`
  (runs every 6h), resetting the submodule pointer back to the latest
  *official* upstream release. Harmless — your tag and Release are already
  created and don't change.
- **Step 2's checks come back non-empty** — don't try to work around it by
  adding secrets yourself; this pipeline is intentionally keyless. Ask
  before extending it.

See `.github/workflows/README.md` for how the workflow's jobs fit together
if you need to change the pipeline itself rather than just run it.
