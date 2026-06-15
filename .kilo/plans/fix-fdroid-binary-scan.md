# Fix F-Droid build failure: source scanner rejects committed binaries

## Root cause
GitLab job `14847865012` failed during `fdroid build com.github.abakum.crocson:1950`
at the source-scanning step:

```
ERROR: Found binary at apks
ERROR: Found Java JAR file at rustore/pepk.jar
ERROR: Could not build app ... Can't build due to 2 errors while scanning
```

F-Droid forbids prebuilt binaries in the source tree at the pinned commit.
Both files are tracked in git and are present at commit `89eb354`:

- `apks` — ELF x86-64 executable. A committed build artifact of `cmd/apks/main.go`
  (an `.apks` filter tool). It is NOT used by the F-Droid build steps.
- `rustore/pepk.jar` — Google's PEPK tool (third-party JAR), used only by the
  manual RuStore signing workflow (`.github/workflows/pepk.yml`). Not used by F-Droid.

A full scan of all tracked files confirms these are the **only two** binaries the
scanner will reject (`.ttf.xz` font resources passed the scan).

## Plan

### 1. Remove the binaries from git tracking
- `git rm --cached apks rustore/pepk.jar` (remove from index; keep working copies
  locally if desired) — or plain `git rm` to delete them from the tree.

### 2. Prevent re-committing via `.gitignore`
Append to `.gitignore`:
```
/apks
rustore/pepk.jar
```

### 3. Keep RuStore PEPK workflow working (RESOLVED)
`pepk.jar` has been moved out of this repo into the `abakum/homebrew-tap`
repository and is downloadable via a pinned raw URL (verified: HTTP 200,
9136653 bytes, `application/octet-stream`):
```
https://raw.githubusercontent.com/abakum/homebrew-tap/8c8a9386f34cc39a1e4dec3314942eb5dd19435b/rustore/pepk.jar
```
In `.github/workflows/pepk.yml`, add a step before "Run pepk.jar" that fetches it:
```yaml
- name: Download pepk.jar
  run: wget -q https://raw.githubusercontent.com/abakum/homebrew-tap/8c8a9386f34cc39a1e4dec3314942eb5dd19435b/rustore/pepk.jar -O rustore/pepk.jar
```
The existing `java -jar rustore/pepk.jar` step then works unchanged. No secret needed.

### 4. Commit the removal
Commit message e.g. `chore: remove committed binaries for F-Droid`.
Note the new commit SHA.

### 5. Update F-Droid metadata pin
In `metadata/com.github.abakum.crocson.yml`, replace the `commit:` value
`89eb354d32e27978ae6e08bfb1981e6e16c48b20` with the new commit SHA in **all
four** build entries (versionCode 1950, 1951, 1952, 1953).
Version codes stay the same (the build never succeeded, so nothing was published).

### 6. Re-run the GitLab CI job for `crocson-v1.11.59` (or open a new MR with the
updated metadata) and confirm the scanner passes and the build proceeds.

## Notes
- `pepk.jar` now lives in `abakum/homebrew-tap` and is fetched by CI via `wget`
  (no GitHub Actions secret required). The pinned commit SHA
  `8c8a9386f34cc39a1e4dec3314942eb5dd19435b` keeps the download reproducible.
