# Fix: saving directories-with-files in the recv basket (recv.go)

## Problem

When the recv basket contains a directory with files, saving is broken on both
desktop and Android. After `reload()` (recv.go:533) the basket holds BOTH a
directory entry `join/<dir>` AND nested file entries
`join/<dir>/a.txt`, `join/<dir>/sub/b.txt`. `filesSave` (recv.go:1070) iterates
every entry and:

- computes `child := filepath.Base(src)` (recv.go:1084), stripping the directory
  → structure lost, files scattered/colliding;
- processes the directory entry and its nested file entries **independently** →
  on desktop a map-order race (whoever runs first wins; the others hit a
  vanished `src`), on Android the async `ZipDirectoryProgress` races
  `removeEntry` on its own child files.

Note: desktop `ChildDownload` (for_mobile0.go:35) already supports subdirs but
never receives them because `filepath.Base` flattens them first.

## Decision

Desktop = preserve directory tree at destination; Android = keep current
zip-of-directory behaviour, but eliminate duplication/races.

## Tasks

### 1. Helpers (recv.go)

- Add `relFromRoot(src string) string`: `filepath.Rel(join(), src)`, fallback to
  `filepath.Base(src)` on error.
- Add `isCoveredByDir(src string, dirEntries []string) bool`: true if a cleaned
  `src` starts with some dir entry path + separator (reuse the `isCached`
  prefix idea from send.go:2716, or inline a `strings.HasPrefix` after
  `filepath.Clean`).

### 2. New platform helper `DownloadDir()` (fyne.URI, error)

- `for_mobile0.go` (desktop): return `storage.NewFileURI(xdg.UserDirs.Download)`.
- `for_android.go` / `for_ios.go`: return `(nil, fmt.Errorf("DownloadDir not applicable"))`.
  Android directories go through zip + `createFileInDownloads`, so this is never
  used there.

### 3. Desktop directory-copy routine

Add a helper (recv.go) used by both `filesSave` and `dialogFileSave`:

```
saveDirDesktop(srcDir string, destDir string, fe *fyne.Container, done func(ok bool))
```

- `os.MkdirAll(destDir, 0700)`.
- `copyFiles(storage.NewFileURI(srcDir), destDir, copyFileCB)` where `copyFileCB`
  mirrors the existing recv.go:1135-1180 pattern: per file create a temp progress
  entry via `addEntry`, `CopyFileProgress(src, dstPath, feCopy, cb)`, on success
  `removeEntry(src, feCopy, true)` and finally remove the empty source dirs +
  `removeEntry(srcDir, fe, false)`.
- `done(ok)` lets the caller clear the directory entry and its nested entries.

### 4. Refactor `filesSave` (recv.go:1070)

At the top of the `forEachFileEntry` body, snapshot entries first (the existing
`forEachFileEntry` already builds a temp map). Then per entry:

a. If `isLinkDir(src)` (directory entry):
   - **Skip nested file entries**: collect dir entries up front; if current `src`
     is `isCoveredByDir(src, dirEntries)`, skip it entirely (it is saved with its
     directory).
   - **Android** (`isMobile || asMobile`): keep the existing zip block
     (recv.go:1215-1241): `ZipDirectoryProgress(pathZip, src, fe, cb)` →
     `copyFrom(pathZip)`. In the zip-success path additionally `removeEntry` the
     nested file entries under `src`.
   - **Desktop**: resolve `destDir`:
     - `lu != nil` (folder picker) → `destDir = lu.Path()`.
     - `lu == nil` (Download) → `destDir = DownloadDir().Path()`.
     Call `saveDirDesktop(src, destDir, fe, ...)`.

b. Else (file entry, not covered by a directory entry):
   - **Desktop**: `child = relFromRoot(src)`; before `Rename(src, dst)` do
     `os.MkdirAll(filepath.Dir(dst), 0700)`; keep the existing `copyFiles`
     fallback (recv.go:1135-1180) which already does `MkdirAll` per file.
   - **Android**: keep `child = filepath.Base(src)` (MediaStore/SAF flat name)
     and the existing `copyFrom` path.

### 5. `dialogFileSave` directory case (recv.go:382)

- Mobile: unchanged (save button already zips then calls `dialogFileSave(pathZip)`
  on a real file).
- Desktop: when `isLinkDir(src)`, do NOT open a file picker; instead route to
  `saveDirDesktop(src, DownloadDir().Path(), fe, ...)` (or open a folder picker
  via `ShowFolderOpen` and copy there). Prefer Download destination to keep it
  one-tap.

### 6. Cleanup / basket refresh

- After a successful directory save, ensure `removeEntry(srcDir, feDir, false)`
  and removal of nested file entries (covered entries), then `recvRefresh()`.
- `topline` "Saved all files to ..." should fire only when `mapEmpty(&fileentries)`.

## Affed boundaries / files

- `recv.go` (filesSave, dialogFileSave, new helpers).
- `for_mobile0.go`, `for_android.go`, `for_ios.go` (new `DownloadDir`).
- No changes to `copy.go`, `zip.go`, `symlink.go` (reuse as-is).

## Risks

- `DownloadDir()` on android/ios must error cleanly (never reached for dirs —
  those zip) to avoid build/log noise.
- `copyFiles` recursion respects `visited`/symlinks already; verify nested
  symlinked subdirs resolve under the temp tree.
- Concurrent saves (user taps Download + Save-All): acceptable — each entry is
  processed once; the `covered` skip prevents double work.

## Validation

- `go build` (desktop) and `go build -tags=android` (android) must pass.
- Manual:
  1. Receive a folder with nested files/subfolders.
  2. Save via **Download** button → desktop: `<Download>/<dir>/...` tree intact;
     android: `<dir>.zip` in Download, no loose duplicate files.
  3. Save via **Save-All** (folder picker) → same expectations at chosen folder.
  4. Save a directory entry via its **save** button → desktop tree copy, android
     zip.
  5. Basket empties correctly after each save; no leftover nested entries.
  6. Receive a mix of standalone files + a folder → standalone files saved by
     basename, folder saved as tree/zip, no cross-contamination.

## Out of scope

- Live mid-transfer saving (entries exist only as files before `reload()`); the
  `relFromRoot` change already keeps structure for desktop in this case, but it
  is not a primary scenario.
- iOS-specific SAF/MediaStore nuances beyond matching the Android zip path.

---

# Round 2 — follow-up fixes after manual test (confirmed via log)

Manual test (folder `MAX` → Download on Windows desktop) revealed two bugs in
the `saveDirDesktop` routine added in Round 1.

## Bug 2.A — source directory name dropped (structure lost)

Symptom (log): `walk copyFile .../recv/MAX/<file>.mp3 C:\Users\KAbak\Downloads\<file>.mp3`
— files land directly in `Downloads\`, not `Downloads\MAX\`.

Cause: `copyFiles(srcDir, destDir)` treats `srcDir` as the root and writes its
**contents** into `destDir`, dropping the source dir's own basename (`MAX`). The
Round-1 `saveDirDesktop` passed `destDir = lu.Path()` / `DownloadDir().Path()`
without the dir name.

### Fix 2.A (recv.go, `saveDirDesktop`)
After resolving `destDir` and before `os.MkdirAll`/`copyFiles`, append the dir
basename:
```
destDir = filepath.Join(destDir, filepath.Base(srcDir))
```
Applies to BOTH the Download (`lu == nil`) and folder-picker (`lu != nil`) paths.

## Bug 2.B — cleanup races in-flight copies (`unlinkat ... file in use`)

Symptom (log order): the dir-level `removeEntry(src, fe, true)` (which calls
`os.RemoveAll(srcDir)`) runs at `recv.go:237` BEFORE the per-file copies finish
at `recv.go:442`, producing
`remove dir MAX: unlinkat ... .mp3: The process cannot access the file because it
is being used by another process`.

Cause: `CopyFileProgress` is **asynchronous** (progress.go:338 spawns a
goroutine). `copyFiles` returns as soon as all copies are *scheduled*, so the
`done(true)` callback fires while copies are still writing → `RemoveAll(srcDir)`
collides with open file handles.

### Fix 2.B (recv.go, `saveDirDesktop`)
Track completion of all async copies and only call `done` after they finish:

- Add import `"sync/atomic"` to recv.go (`sync` already imported).
- In `saveDirDesktop` use `var wg sync.WaitGroup` and `var failed atomic.Bool`.
- In the per-file `copyFile` callback:
  - `wg.Add(1)` before each `CopyFileProgress`.
  - `CopyFileProgress` `onComplete`: `defer wg.Done()`; on error
    `failed.Store(true)` + `removeEntry(src, feCopy, false)`; on success
    `removeEntry(src, feCopy, false)` (UI only — do **not** `os.Remove` the
    source file individually; the dir-level cleanup deletes everything after all
    copies close).
- After `copyFiles` returns: `wg.Wait()` (all `wg.Add` happen synchronously in
  the same goroutine, so there is no Add/Wait race), then
  `done(!failed.Load())`.
- The caller's `done` callback (`removeEntry(srcDir, fe, true)`) now runs only
  after all file handles are closed, so `RemoveAll(srcDir)` succeeds.

## Affed boundaries (Round 2)

- `recv.go` only: `saveDirDesktop` (Fix 2.A + 2.B) and one new import
  (`sync/atomic`).

## Validation (Round 2)

- `make wsl install amd64` must pass.
- Repeat the `MAX` folder → Download test on Windows:
  1. Files appear under `Downloads\MAX\<file>` (tree preserved), not flat.
  2. No `unlinkat ... file in use` / `remove dir ... ` errors in the log.
  3. Temp `recv\MAX` is removed cleanly after save; basket cleared.
- Folder-picker path (`saveAllButton`) with the same folder: tree preserved at
  chosen location.

## Out of scope (Round 2)

- Android/iOS unchanged (zip path, no `copyFiles` race there — `copyFrom` uses
  the synchronous-within-its-own-goroutine `copyToUWCProgress` and removes the
  dir only from the zip-success callback).

---

# Round 3 — directory save via diskette must open a picker (desktop)

Manual test (Windows desktop) confirms Round 2 works (tree preserved, no race).
New finding: clicking the **diskette** (per-entry save button) on a directory
does **not** open any picker — it saves immediately to `Download\MAX\`.

Cause: the Round-1 desktop-dir branch in `dialogFileSave` (recv.go:475-488)
calls `saveDirDesktop(src, nil, fe, ...)` directly, bypassing any picker.

### Fix 3.A (recv.go, `dialogFileSave`, lines 475-488)

Replace the direct-to-Download save with a folder picker, then copy the tree to
the chosen folder:

```go
// Десктоп: каталог сохраняем деревом в выбранную через каталогпикер папку
if !(isMobile || asMobile) && isLinkDir(src) {
    ShowFolderOpen(func(lu fyne.ListableURI, err error) {
        if err != nil {
            log.Errorf("folder selection: %v", err)
            return
        }
        if lu == nil {
            log.Debug("folder selection canceled")
            return
        }
        saveDirDesktop(src, lu, fe, func(ok bool) {
            if ok {
                removeEntry(src, fe, true)
                fyne.Do(func() {
                    if mapEmpty(&fileentries) {
                        topline.SetText(fmt.Sprintf("%s %s", lp("Saved all files to"), lu.Path()))
                    }
                })
            }
        })
    }, parent)
    return
}
```

Notes:
- `ShowFolderOpen` is a package-level function (recv.go:1316), always in scope.
- The diskette for a directory now opens the **folder picker** (consistent with
  `saveAllButton`), letting the user choose the destination; the tree is copied
  to `<chosen>\<dir>\`.
- Mobile unchanged: the diskette still zips then opens a file picker for the
  `.zip`.

## Affed boundaries (Round 3)

- `recv.go` only: the desktop-dir branch of `dialogFileSave` (lines 475-488).

## Validation (Round 3)

- `make wsl install amd64` must pass.
- Windows desktop: click diskette on a received directory → folder picker opens;
  choose a folder → directory tree copied to `<chosen>\<dir>\`, basket cleared.
- Cancel the picker → nothing saved, no error in log.
- Mobile: diskette on a directory still produces a `.zip` via the file picker.

---

# Round 4 — unify single-entry save with the directory save (reuse working code)

Manual test (Windows desktop): pressing the diskette on a **file inside** a
directory (`MAX/foo`) opens the **file picker**; choosing folder `AAB` saves to
`AAB\foo` instead of `AAB\MAX\foo`. The directory diskette (`MAX/`, Round 3)
already preserves structure correctly. Rather than patching the file-picker path,
reuse that working directory-save code for nested files too.

Confirmed via log: `recv.go:559: move ...\recv\MAX\foo C:/.../AAB/foo` (the
`dialogFileSave` → `fileSave` desktop branch uses `child := filepath.Base(src)`).

### Decision

- Generalize `saveDirDesktop` → `saveEntryDesktop` so it preserves the
  **relative path from the basket root** (`relFromRoot`) instead of just
  `filepath.Base`. This makes one routine handle both a whole directory and a
  single nested file.
- The desktop diskette routes **directories and nested files** to the folder
  picker + `saveEntryDesktop`. A **top-level file** (no parent dir relative to
  the basket root) keeps the file picker (rename still possible).

### Fix 4.A — generalize the save routine (recv.go, `saveDirDesktop` → `saveEntryDesktop`)

In the routine currently named `saveDirDesktop`:
- Compute the destination from the relative path:
  `dstPath := filepath.Join(destDir, relFromRoot(srcDir))`
  (was `filepath.Join(destDir, filepath.Base(srcDir))`).
- `copyFiles` top-level `MkdirAll` only runs when the source is a directory, so
  ensure the parent exists for the file case:
  `os.MkdirAll(filepath.Dir(dstPath), 0700)` before `copyFiles`.
- Pass `dstPath` to `copyFiles(storage.NewFileURI(srcDir), dstPath, ...)`. For a
  directory this is the target dir (tree copied); for a file this is the target
  file path (single copy). The existing `sync.WaitGroup`/`atomic.Bool` gating and
  the `removeEntry` semantics are unchanged.
- Rename the symbol `saveDirDesktop` → `saveEntryDesktop` and update its callers
  (`filesSave` desktop-dir branch, `dialogFileSave`).

Effect:
- `MAX/` (dir) → `lu\MAX\{foo,bar}` (unchanged behavior).
- `MAX/foo` (nested file) → `lu\MAX\foo` (NEW — now correct).
- `foo` (top-level file) is NOT routed here.

### Fix 4.B — route nested files to the folder picker (recv.go, `dialogFileSave`)

Replace the Round-3 desktop-dir guard with one that also covers nested files:

```go
// Десктоп: каталог или файл внутри каталога — каталогпикер с сохранением структуры
if !(isMobile || asMobile) && (isLinkDir(src) || filepath.Dir(relFromRoot(src)) != ".") {
    ShowFolderOpen(func(lu fyne.ListableURI, err error) {
        if err != nil {
            log.Errorf("folder selection: %v", err)
            return
        }
        if lu == nil {
            log.Debug("folder selection canceled")
            return
        }
        saveEntryDesktop(src, lu, fe, func(ok bool) {
            if ok {
                removeEntry(src, fe, true)
                fyne.Do(func() {
                    if mapEmpty(&fileentries) {
                        topline.SetText(fmt.Sprintf("%s %s", lp("Saved all files to"), lu.Path()))
                    }
                })
            }
        })
    }, parent)
    return
}
child := filepath.Base(src)   // top-level file → file picker (existing path)
```

Notes:
- `relFromRoot` and `saveEntryDesktop` are closures declared before
  `dialogFileSave` (in scope). `ShowFolderOpen` is package-level.
- A top-level file has `filepath.Dir(relFromRoot(src)) == "."` → falls through to
  the existing file picker (`newFileSave`) with rename.
- `filesSave` is unaffected: its `coveredBy` skip means nested files are saved by
  their directory entry; only directory entries call `saveEntryDesktop` there.

## Affed boundaries (Round 4)

- `recv.go` only:
  - rename/generalize `saveDirDesktop` → `saveEntryDesktop` (relFromRoot dest +
    `MkdirAll(Dir(dstPath))`);
  - `dialogFileSave` desktop guard widened to nested files;
  - update `filesSave` call site name.

## Validation (Round 4)

- `make wsl install amd64` must pass.
- Windows desktop:
  1. Diskette on `MAX/foo` (nested file) → folder picker → choose `AAB` →
     `AAB\MAX\foo`.
  2. Diskette on `MAX/` (dir) → folder picker → choose `AAB` →
     `AAB\MAX\{foo,bar}` (regression check).
  3. Diskette on top-level `foo` → **file picker** → choose/rename → flat save
     (unchanged).
  4. Deeper nesting `MAX/sub/x` → `AAB\MAX\sub\x`.
  5. Cancel the folder picker → nothing saved.
- Mobile: unchanged (directory zips; nested single-file save not the primary
  path).

---

# Round 5 — Android: directory zip is incomplete (race with reload)

Manual test (Android): the **Download** save writes a complete `croc-received.zip`,
but the **folder picker** save produced an incomplete `A/croc-received.zip` (only
2 of 4 files). Log shows, during the zip walk:
```
zip.go:94 Adding croc-received/            <- walk starts (zip file already created)
recv.go:702 removeEntry zipped ...mp3      <- reload() marks source inZip
zip.go:228 Added file ...flac
recv.go:245 remove file ...mp3             <- reload() DELETES the source file
zip.go:102 Error walking ...mp3: no such file   <- walk hits deleted file
zip.go:102 Error walking ...jpg: no such file
```

### Root cause

- `ZipDirectoryProgress` → `zipDirectoryWithOverallProgress` (zip.go:30) creates
  the `.zip` via `os.Create` at the **start** (zip.go:46) and only finishes the
  async `filepath.Walk` later.
- `reload()` (recv.go:662-711) scans `lsr(join())` for files with
  `filepath.Ext == ".zip"`; for each it builds a prefix and marks every source
  path under it `inZip`, then `removeEntry(path, fe, true)` **deletes those
  source files from disk**.
- A refresh that triggers `reload()` during the walk window therefore deletes
  sources the walk has not yet read → `lstat ... no such file` → incomplete zip.
- Desktop is unaffected (uses `copyFiles`, not zip).

### Decision

Write the zip to a `.part` temp file and `os.Rename` to the final `.zip` **only
after the walk completes**. `reload()`'s `filepath.Ext == ".zip"` scan then
cannot match the temp file during zipping, so sources are not deleted
prematurely. This is robust regardless of what triggers `reload()`.

### Fix 5.A (zip.go, `zipDirectoryWithOverallProgress`)

In `zipDirectoryWithOverallProgress(destination, source, c)`:

1. Keep the existing "already exists" check against the **final** `destination`.
2. Add `partFile := destination + ".part"` and an `ok := false` flag.
3. Register a deferred finalizer **first** (so it runs last, after the
   `zipWriter.Close` and `file.Close` defers):
   - if `ok`: `os.Rename(partFile, destination)` (log+propagate error on
     failure);
   - else: `os.Remove(partFile)` if it exists (clean up partial).
4. Create `partFile` (not `destination`) with `os.Create`.
5. Keep the existing `file.Close` and `zipWriter.Close` defers (they now close
   the part file).
6. After the walk returns `nil`, call `restore()`, set `ok = true`, then
   `return nil`. The finalizer then renames the now-complete archive to
   `destination`.
7. On any walk error (`err != nil`), `ok` stays false → finalizer removes the
   partial `.part`.

No signature change: `ZipDirectoryProgress` and its callers (recv.go save block,
`addEntry` saveButton) are unchanged — by the time `onComplete` fires, the final
`.zip` exists (renamed), so the existing `os.Stat(pathZip)` checks work.

### Notes / edge cases

- `reload()` showing a stale `.part` (from a crash) as a basket entry: minor;
  the existing delete button removes it. Optional hardening: skip `.part` paths
  in `reload()`'s `lsr` loop. Out of scope unless desired.
- A concurrent second zip of the same dir would still race on the same
  `partFile` (pre-existing concurrency limitation; not made worse).
- Download and folder-picker saves on Android share this zip path, so both are
  fixed.

## Affed boundaries (Round 5)

- `zip.go` only: `zipDirectoryWithOverallProgress` (temp `.part` + rename).

## Validation (Round 5)

- `make wsl install amd64` must pass.
- Android (`make amd64`): receive a folder with several files; trigger a refresh
  (e.g. switch tabs / background) during/around the folder-picker save → the
  resulting zip must contain **all** files; no `lstat ... no such file` errors in
  logcat; sources cleaned up after.
- Repeat with the Download save → complete zip (regression).
- Desktop unchanged (no zip path).

---

# Round 6 — stale `croc-received.zip.part` shown in basket (Android)

Manual test (Android): the zip now contains all files (Round 5 works), but after
the save a `croc-received.zip.part` entry lingers in the basket; it only
disappears after switching tabs (Recv→Send→Recv).

### Root cause

`reload()` (recv.go:648) calls `lsr(join())`, which lists every file in the recv
dir — including the in-progress `croc-received.zip.part` written by Round 5. In
the loop (recv.go:670) the `.part` is not skipped (ext `.part` ≠ `.zip`, not
`inZip`), so it is `addEntry`'d into the basket. When the zip finishes, the
`.part` is renamed to `.zip` (gone from disk), but the map entry persists until
the next `reload()` (the tab switch) — whose `forEachFileEntry` then sees
`os.Stat ... no such file` and removes it (the `recv.go:705` log line).

### Fix 6.A (main.go): constant

Add `PART = ".part"` next to `DOTZIP` (main.go:49).

### Fix 6.B (recv.go, `reload`)

1. **Loop** (recv.go:670 `loop:`): extend the skip condition to ignore `.part`
   paths so they are never added as entries — treat like `crocRemovalFile`:
   ```go
   if name == "" ||
       path == fpath ||
       name == crocRemovalFile ||
       strings.HasSuffix(strings.ToLower(path), PART) {
       continue
   }
   ```
2. **Cleanup** (`forEachFileEntry`, recv.go:697): add a case so any lingering
   `.part` entry is removed UI-only (never delete the file from `reload` — the
   zip finalizer owns it):
   ```go
   switch {
   case exists[path]:
       return
   case inZip[path]:
       log.Debugf("removeEntry zipped %s", path)
   case strings.HasSuffix(strings.ToLower(path), PART):
       log.Debugf("removeEntry part %s", path)
   default:
       if _, err := os.Stat(path); err != nil {
           log.Debugf("removeEntry %s: %v", path, err)
       } else {
           return
       }
   }
   removeEntry(path, fe, false)   // UI-only for .part; del stays true for others
   ```
   Note: the existing fall-through already calls `removeEntry(path, fe, true)`.
   For the `.part` case we must use `del=false` (don't `os.Remove` a possibly
   mid-zip file). Implement by branching the `del` argument: `false` when the
   path ends in `PART`, otherwise `true` (current behavior).

### Effect

`.part` files never appear in the basket; a stale `.part` entry is removed on the
next `reload` without touching the file. `reload` never deletes a `.part`
(deleting it would corrupt a concurrent zip).

## Affed boundaries (Round 6)

- `main.go` (new `PART` const).
- `recv.go` (`reload`: skip `.part` in loop; clean stale `.part` UI-only).

## Validation (Round 6)

- `make wsl install amd64` must pass.
- Android folder-picker + Download save of a directory: after save the basket is
  clean immediately (no `.part` entry, no need to switch tabs).
- Trigger a refresh mid-zip → still no `.part` entry; zip completes with all
  files.





