# Plan: `status` checkbox for `.github/workflows/fdroid4.yml`

## Goal
Add a `status` boolean input to the F-Droid metadata workflow. When `status=true`,
the run is **read-only** and reports two facts about the version currently in
`FyneApp.toml`:

1. **New recipe detected** — is that `Version` present in the recipe on fdroiddata `master`?
2. **Build outcome** — for that version's `versionCode`s: SUCCESS (APK published) /
   FAILED (build log present, no APK) / PENDING (neither present).

Always exits 0 (report only). Prints direct APK/log links.

## Confirmed decisions
- Detect «new version» by **recipe-vs-FyneApp.toml** (version in recipe on master).
- Failure handling: **report only, exit 0**.
- Extras: **direct `<code>.apk` / `<code>.log.gz` links** (no MR state, no reproducibility).

## Changes (file: `.github/workflows/fdroid4.yml`)

### 1. Add input (in `on.workflow_dispatch.inputs`)
```yaml
      status:
        description: 'Status only: is the FyneApp.toml version in the fdroiddata recipe, and did its build succeed/fail? (read-only)'
        required: false
        default: false
        type: boolean
```

### 2. Make tag lookup non-fatal in «Read version info»
status mode only needs `Version`/`Build`; avoid aborting if the tag isn't fetched:
```bash
COMMIT_SHA=$(git rev-list -n 1 "v${VERSION}" || true)
```

### 3. Gate all mutating steps to skip when `status=true`
Add `&& github.event.inputs.status != 'true'` to the `if:` of:
- «Generate recipe from FyneApp.toml»
- «Verify recipe exists»
- «Create Merge Request to fdroiddata»

And give «Commit and push to GitHub» (currently unconditional) a new condition:
```yaml
        if: github.event.inputs.status != 'true'
```
Leave «Checkout», «Read version info», «Load recipe» untouched (status fetches the
recipe itself).

### 4. New step «Check F-Droid status»
`if: github.event.inputs.status == 'true'`. Self-contained, **secret-free** (public raw
endpoint + repo HEAD). Sketch:

```bash
set -u
APP_ID="${APP_ID}"
VER="${{ steps.info.outputs.version }}"
RECIPE_URL="https://gitlab.com/fdroid/fdroiddata/-/raw/master/metadata/${APP_ID}.yml"
REPO="https://f-droid.org/repo"

http() { curl -sI -o /dev/null -w "%{http_code}" "$1"; }   # HEAD; 200/206=present, 404=absent

code=$(http "$RECIPE_URL")
if [ "$code" != "200" ]; then
  echo "::notice::Recipe not on fdroiddata master (HTTP $code)."
  { echo "### F-Droid status"; echo "- Recipe on master: **not found** (HTTP $code)"; } >> "$GITHUB_STEP_SUMMARY"
  exit 0
fi

# Pair versionName/versionCode (versionName immediately precedes versionCode per build block)
mapfile -t PAIRS < <(paste -d'|' \
  <(grep -E '^\s*versionName:' recipe.yml | sed -E 's/[^:]*:\s*"?([^"]+)"?/\1/') \
  <(grep -E '^\s*versionCode:' recipe.yml | sed -E 's/[^:]*:\s*([0-9]+).*/\1/'))

CODES=()
for p in "${PAIRS[@]}"; do [ "${p%%|*}" = "$VER" ] && CODES+=("${p##*|}"); done

# Summary to step summary + log
{
  echo "### F-Droid status for v${VER}"
  if [ "${#CODES[@]}" -eq 0 ]; then
    echo "- New recipe: **NOT detected** — v${VER} is absent from the recipe on master."
    echo "  (recipe newest version: ${PAIRS[-1]%%|*})"
  else
    echo "- New recipe: **detected** — v${VER} is on master, codes: ${CODES[*]}"
  fi
} | tee -a "$GITHUB_STEP_SUMMARY"

for c in "${CODES[@]}"; do
  apk="$REPO/${APP_ID}_${c}.apk"; log="$REPO/${APP_ID}_${c}.log.gz"
  if [ "$(http "$apk")" = "200" ]; then st="SUCCESS";
  elif [ "$(http "$log")" = "200" ]; then st="FAILED";
  else st="PENDING"; fi
  echo "- code ${c}: **${st}** — [apk](${apk}) · [log](${log})" | tee -a "$GITHUB_STEP_SUMMARY"
done
exit 0
```
(If a CDN HEAD is unreliable, swap `http()` to `curl -s -o /dev/null -w "%{http_code}" --range 0-0`
and treat 200/206 as present.)

## Affected boundaries
- Single file: `.github/workflows/fdroid4.yml`.
- 1 new input; `if`-gating on 4 existing steps; 1 new step; 1-line non-fatal tag tweak.
- No repo source changes; status mode needs no secrets.

## Validation
- Branch + run with `status=true`: expect `detected: v1.11.77`, codes 2011–2014 = PENDING
  (apk+log 404, as observed today).
- After F-Droid publishes 1.11.77, re-run `status=true`: codes → SUCCESS (apk 200).
- Confirm mutating steps did **not** run in status mode: no new commit, no MR (check job
  step list / `git log` / GitLab MRs).
- Regression: `status=false` (default) + e.g. `load=true` behaves exactly as before.

## Risks / caveats
- For `binary:` builds a `.log.gz` may not always be published on failure → a real
  failure could show as PENDING; SUCCESS (APK present) is reliable.
- Recipe pairing assumes `versionName:` immediately precedes `versionCode:` per build
  block (true for this recipe and fdroid standard format).
- status mode needs the `info` step to succeed — the non-fatal tag tweak (§2) covers the
  common case.

## Out of scope
- Repo-index comparison, MR-state, reproducibility status (per user choice).
- Changing generation / MR logic.
