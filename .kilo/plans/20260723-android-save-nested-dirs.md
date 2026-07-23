# Android: save received folders as a real directory tree (no zip)

## Problem

When saving from the Receive basket (`recv.go`) on Android, files that arrive
inside nested directories are written out as **`.zip` archives**, while desktops
preserve the directory structure. Root causes (all in the mobile branch of
`filesSave`):

1. `child := filepath.Base(src)` (`recv.go:1313`) strips the relative path, so the
   Java side never receives the subdirectory information.
2. Directory entries (`isLinkDir(src)` → a received folder) are zipped at save time
   (`recv.go:1382-1407`) instead of being written as a tree.
3. The basket lists a received folder as **one directory entry** (`lsr`,
   `symlink.go:271` lists only the first level), so its nested files are never
   individual entries and ride along with the zip.

Desktop already does the right thing: `saveEntryDesktop` (`recv.go:434`) walks the
tree, preserves `relFromRoot`, `MkdirAll`s, and copies each file.

**Decision:** mirror desktop behavior on Android — preserve the relative directory
tree, no zipping, across all save paths (folder picker, Download button,
single-file save button, and pending saves).

## Feasibility (confirmed from code)

- **Download path (MediaStore):** already supports nesting in Java.
  `createFileInDownloadsModern` (`GoNativeActivity.java:1205`) splits `fileName`
  on `/`, calls `createDirectoriesInMediaStore`, and sets
  `relative_path = "Download/<subdirs>"`. Go only needs to pass the full relative
  path instead of the base name.
- **Folder picker (SAF tree):** `createFileInTree` (`GoNativeActivity.java:1413`)
  creates a single document. To nest, walk the relative path, creating each
  intermediate directory via
  `createDocument(parent, "vnd.android.document/directory", part)`, then the file.
  Works on any SAF tree URI (incl. external SD). `ACTION_OPEN_DOCUMENT_TREE`
  already grants the needed write access.

## Implementation tasks

### 1. Java — `GoNativeActivity.java`

- Add `findChildDocument(parentUri, name)`:
  - Builds the child-documents URI for `parentUri` (use
    `buildChildDocumentsUriUsingTree` for tree URIs, fall back to
    `buildChildDocumentsUri`).
  - Queries children for `Document.COLUMN_DISPLAY_NAME == name` (and prefer
    `COLUMN_MIME_TYPE == Document.MIME_TYPE_DIR` when looking for a dir).
  - Returns the matching child document URI string, or `null` if not found.
- Add `createFileInTreeNested(treeUri, relPath, mimeType)`:
  - Start from the tree's child-doc URI.
  - Split `relPath` on `/`. For every component except the last, **find-or-create**
    a directory document (`createDocument(parent, "vnd.android.document/directory",
    part)`; reuse it if `findChildDocument` already finds it). Descend into it.
  - For the last component, `createDocument(parent, mimeType, part)` and return the
    file URI (use the file's mime, detected in Go).
  - Return `"error: ..."` on failure (consistent with existing `createFileInTree`).
- Fix `createFileInDownloadsLegacy` (`GoNativeActivity.java:1253`): when `fileName`
  contains `/`, call `file.getParentFile().mkdirs()` before `createNewFile()`.
- Verify `createDirectoriesInMediaStore` (`GoNativeActivity.java:1232`): the current
  logic inserts cumulative `_display_name` entries (`"a"`, then `"a/b"`) all with
  `relative_path = "Download"`, which is likely wrong/unnecessary. Prefer relying
  solely on the file's `relative_path = "Download/<subdirs>"` (MediaStore
  auto-creates intermediate dirs on API 29+). Remove or correct the helper
  accordingly and confirm via test.

### 2. cgo bridge — `for_android.go`

- Add `ChildTreeNested(parent fyne.URI, relPath string) (child fyne.URI, cleanup func(), err error)`:
  - Calls `callStringStringString("createFileInTreeNested", parent.String(), relPath, mime)`
    (mime via `detectMimeType(filepath.Base(relPath))`).
  - Parses the returned URI; translate `"error: ..."` to an error (mirror
    `CreateFileInTree`).
- Keep `CreateFileInDownloads(fileName, mime)` as-is (it already passes `fileName`
  through to Java, which splits on `/`). Callers will now pass the full
  `relFromRoot` path.
- No change needed to `ChildDownload` (it wraps `CreateFileInDownloads`).

### 3. Non-Android parity — `for_android0.go`

- Add no-op/`fmt.Errorf("not supported")` stubs for `ChildTreeNested` so the
  non-Android build compiles (mirror the existing `ChildViaMediaStore` stub).

### 4. Save logic — `recv.go`

All mobile save paths below must compute `rel := relFromRoot(src)` and write a
tree instead of zipping. Extract a shared helper to avoid duplication:

- Add `saveEntryMobile(src string, lu fyne.ListableURI, fe *fyne.Container, done func(ok bool))`
  mirroring `saveEntryDesktop` (`recv.go:434`):
  - **Single file:** `rel := relFromRoot(src)`.
  - **Directory (`isLinkDir`):** enumerate files under `src` (e.g. via `lsr2(src)`,
    `symlink.go:175`) and compute `rel := relFromRoot(eachFile)` for each.
  - For each target file, create the dest URI:
    - `lu != nil` (picker): `ChildTreeNested(lu, rel)`.
    - `lu == nil` (download): `ChildDownload(rel)`.
  - Write via `storage.Writer(u)` + `copyToUWCProgress` (reuse the existing
    `copyFrom` pattern at `recv.go:1359`), with per-file progress entries (like
    desktop's `addEntry` per copied file).
  - Use a `sync.WaitGroup` + `atomic.Bool` to aggregate success; on full success,
    `removeEntry(src, fe, true)` and update `topline` (mirror desktop).
  - **On any failure:** log the error and leave the entry in the basket so the user
    can retry — **no zip fallback** (mirrors desktop).
- `filesSave` mobile branch (`recv.go:1312-1410`): replace the `child :=
  filepath.Base(src)` + zip logic with calls to `saveEntryMobile`. Keep
  `coveredBy`/`dirEntries` handling.
- Single-file save button (`recv.go:335-375`, the `isLinkDir` mobile zip block):
  call `saveEntryMobile` instead of `ZipDirectoryProgress` + `dialogFileSave`.
- `processPendingSaves` (`recv.go:1434+`): for `isLinkDir` pending entries, call
  `saveEntryMobile` instead of zipping. (`PendingSave` already carries `Src`,
  `Dest`, `LU`, `FE`, `W`.)
- Remove now-dead zip-on-save code paths for mobile (the `ZipDirectoryProgress`
  blocks specific to mobile save). Do **not** touch the send-side / transfer zip
  (`zip-unzip` preference, `send.go`) — that is out of scope.

## Failure modes / edge cases

- **Re-save / existing dirs:** `findChildDocument` dedupes SAF dir creation so
  `createDocument` does not produce duplicates (`(1)` suffixes).
- **Empty subdirectories:** lost on the Download/MediaStore path (dirs only exist
  when a file is inserted with that `relative_path`). Preserved via SAF picker.
  Acceptable — matches desktop's `copyFiles`, which also only `MkdirAll`s a dir
  when it contains a file.
- **API < 29:** legacy Download path now `mkdirs()`; SAF `createDocument` is
  available since API 21, so the picker path works.
- **Scoped storage (API 29+):** Download via MediaStore `relative_path`; picker via
  SAF tree — both compliant.
- **External SD / 3rd-party providers:** `ChildTreeNested` uses generic SAF, not
  MediaStore, so it works on any tree URI.
- **Filename with `/` or unicode:** handled by SAF/MediaStore document naming;
  keep existing sanitization where present.

## Validation

- `make arm64 wsl` — builds Android (compiles `GoNativeActivity.java`) + Windows.
  A Java compile error surfaces in `make arm64`.
- Manual (Android): send a folder with nested subdirs (`a/b/c.txt`, `a/d.txt`);
  receive; then:
  - Save via **folder picker** → confirm the full tree is recreated (test an
    external-SD provider too).
  - Save via **Download** → confirm `Download/a/b/c.txt`, `Download/a/d.txt`.
- Manual: re-save the same folder → no duplicate dirs (dedupe works).
- Manual: single-file save button and pending-save paths preserve the tree.
- Confirm a single top-level file still saves correctly (no regression).

## Out of scope

- Send-side / transfer-time zipping and the `zip-unzip` preference (`send.go`,
  `zip.go`) — unchanged.
- iOS save paths.
