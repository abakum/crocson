# Plan: `generate_recipe.sh` — preserve F-Droid build history (idempotent)

## Goal
Make `metadata/generate_recipe.sh` preserve the historical `Builds:` entries that
arrive via `load=true` (recipe downloaded from fdroiddata), while still emitting the
4 fresh current-version builds. The script must be **idempotent** (re-running for the
same version never duplicates builds).

## Context — why this is needed
- `.github/workflows/fdroid4.yml`:
  - `first=true` → runs `generate_recipe.sh` (fresh recipe, no history). Today this works.
  - `load=true` → downloads `metadata/com.github.abakum.crocson.yml` from fdroiddata
    (full build history) and commits it. The generate step does **not** run on a
    successful load (`fdroid4.yml:57`), so the loaded recipe is committed as-is.
- `metadata/generate_recipe.sh` (current): always regenerates the entire `Builds:`
  block from scratch for the current version (`generate_builds`, lines 56–92) and
  discards any existing build entries. `com.github.abakum.crocson.yml` currently holds
  only the 4 builds of `1.11.76` (versionCodes 2006–2009) — no older versions.
- User's manual flow after the fix: run `generate_recipe.sh` locally, then trigger the
  workflow with `first=false load=false` to open the MR.

## Scope
- **In scope:** `metadata/generate_recipe.sh` only.
- **Out of scope:** the workflow file (`fdroid4.yml`), srclibs, changelogs, and any
  Go/Java build. `make arm64 wsl` is unrelated here (the recipe is F-Droid metadata,
  not built by `make`); do not run it for this change.

## Design — variant A: version-based dedup (one unified branch)
Core operation the script will perform:
> The recipe must contain exactly the 4 fresh current-version builds **plus** all
> historical (non-current) build entries that already exist in `$YML`.

Steps inside the script:
1. Compute `VERSION`, `BUILD`, `COMMIT_SHA`, `TOOLS_SHA`, `GO_VERSION` as today
   (lines 8–28 unchanged).
2. `HEADER`: if `$YML` exists, everything before `Builds:` via
   `sed '/^Builds:/q' "$YML" | sed '$d'`; else `HEADER_DEFAULT`. (unchanged behavior)
3. `HISTORY`: from the existing `$YML`, take the `Builds:` block body (entries only,
   excluding the `Builds:` line and the tail starting at `AllowedAPKSigningKeys:`),
   and **drop every entry whose `versionName` value == current `$VERSION`**. Keep the
   rest verbatim in their existing order. If `$YML` is missing → `HISTORY=""`.
4. `NEW_BUILDS`: regenerate the 4 ABI entries (arm/arm64/386/amd64) for the current
   version with latest `COMMIT_SHA`/`TOOLS_SHA`/`GO_VERSION`, versionCodes
   `BUILD+1..BUILD+4`. (the body of today's `generate_builds`, but **without** the
   leading `Builds:` line)
5. Assemble and write `$YML`:
   ```
   HEADER
   <blank>
   Builds:
   [HISTORY entries]            # only if non-empty, followed by one blank line
   NEW_BUILDS (4 entries)
   <blank>
   TAIL                         # unchanged hardcoded TAIL
   CurrentVersion: <VERSION>
   CurrentVersionCode: <BUILD+4>
   ```
6. Print a one-line summary: how many existing entries were kept as HISTORY, how many
   current-version entries were dropped, 4 added.

### Why this is idempotent & risk-free
- Re-run for the same version: the 4 current entries are in `HISTORY`'s "drop" set, so
  they are removed then re-added exactly once → no duplicates, deterministic output.
- Loaded recipe already containing the current version (rare, post-merge re-run): those
  entries are dropped and replaced with fresh SHAs; history preserved.
- Older versions are never touched (no silent data loss).

### Mapping back to the user's original count rule
- ">4 → preserve + add": satisfied (HISTORY preserved, 4 fresh appended).
- "exactly 4 → overwrite": satisfied when those 4 are the current version (always true
  in the repo right after a generate) — they are dropped and rewritten as 4 fresh.
- Only deviation from a literal count rule: a file with exactly 4 builds of an **older**
  version (never occurs in the repo's working flow) would be preserved instead of
  overwritten. This is the safer, data-preserving choice and matches "eliminate risks".

## Recommended implementation detail (awk, paragraph mode)
- Extract block body:
  `sed -n '/^Builds:/,/^AllowedAPKSigningKeys:/p' "$YML" | sed -e '1d' -e '/^AllowedAPKSigningKeys:/d'`
- Filter current-version paragraphs with `awk -v RS='' -v ORS='\n\n' -v v="$VERSION"`,
  comparing the parsed `versionName` value (strip `^  - versionName:\s*` and trailing
  spaces) to `v` **by exact string equality** (not regex) so `.`/digits in the version
  can't cause false matches. Skip paragraphs whose value == `v`; print the rest.
- Trim trailing blank lines from `HISTORY` (e.g. `sed -e :a -e '/^\n*$/{$d;N;ba}'`),
  so the join with `NEW_BUILDS` yields exactly one blank-line separator between the last
  kept entry and the first new entry, and the block ends with the last new entry's
  `ndk:` line (the assembler adds the single blank before `TAIL`).
- Refactor `generate_builds` into `generate_new_builds` (body only); the assembler
  emits the `Builds:` line itself.

## Edge cases
- `$YML` missing → `HEADER_DEFAULT`, `HISTORY=""`, output = fresh 4-build recipe (== today's first-time output).
- All entries are current version → `HISTORY=""` → output = 4 fresh current builds (refresh SHAs).
- `versionName` value comparison must trim trailing CR/whitespace; FyneApp `Version` has no `v` prefix and entries use `versionName: 1.11.76` (no `v`), so they compare cleanly.
- Blank-line normalization between entries: exactly one blank line separating entries (matches existing file style).

## Residual risks / notes
- The hardcoded `TAIL` is still used (current behavior). Any extra top-level fields that
  fdroiddata might add after the builds block (e.g. `AntiFeatures:`) would be dropped.
  The crocson recipe has none today; flagged but intentionally unchanged.
- `HEADER` is still taken from the existing file (so header fields like `Categories:`,
  `License:`, descriptions are preserved).
- No `versionCode`-based dedup; matching is by `versionName`. Sufficient because all 4
  ABIs of a version share one `versionName`.

## Validation (manual, against copies of the real recipe)
1. **Idempotency / overwrite (current state):** snapshot `metadata/com.github.abakum.crocson.yml`
   (4 current builds), run `bash metadata/generate_recipe.sh`, re-run it → output stable;
   build count stays 4; only SHAs may refresh.
2. **History preserved:** make a temp copy, prepend a fake older-version group (e.g.
   `versionName: 1.11.75`, lower versionCodes) so the file has 8 entries. Run the script
   for the current version → result contains the 1.11.75 group unchanged **plus** the 4
   current builds (8 entries). Re-run → identical (no duplicate current builds).
3. **Loaded recipe already has current version:** in a temp copy, set the top group's
   `versionName` to the current version with a stale `commit`. Run the script → that
   group is replaced with fresh-SHA builds; any older group preserved.
4. **First-time:** delete the yml, run the script → fresh recipe from `HEADER_DEFAULT`
   with exactly 4 builds.
5. **YAML sanity:** `python3 -c 'import yaml,sys; yaml.safe_load(open(sys.argv[1]))' metadata/com.github.abakum.crocson.yml`
   parses without error.
6. **Lint:** `bash -n metadata/generate_recipe.sh` and (if available) `shellcheck metadata/generate_recipe.sh`.
7. Final smoke: trigger workflow `first=false load=false` to confirm the MR step picks up the corrected recipe.

## Open questions
None — design confirmed (variant A). Implementation-only.
