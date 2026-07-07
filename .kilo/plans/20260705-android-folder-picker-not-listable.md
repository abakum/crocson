# Android: native folder picker to bypass Fyne's listability gate

## Problem
On **API 29 (Android 10)** picking a folder for "Save all" with the **stock** SAF Documents UI
(`ACTION_OPEN_DOCUMENT_TREE`) logs `folder selection: specified URI is not listable` and the files
end up in **Downloads** instead of the chosen folder. The single-file **log export** works.
**API 31 and API 34 (Android 14) work.** Boundary is between API 29 and 31 (API 30 untested).

Note on the attached log: the `content://com.ghisler.files/...` URI (`send.go:1767`) is the
**Send-side file READ** (file picked via Total Commander) and is **unrelated** to this bug. The
folder chosen for saving was the **stock** picker; its URI is not in the log because Fyne discards
it on gate failure (`filesSave` logs only the error, recv.go:1195). `content://media/external/...`
in the log is the Downloads **fallback** write target, not the chosen folder.

## Root cause (verified in vendored Fyne v2.7.4)
- `dialog.ShowFolderOpen` (recv.go:1457) → Fyne `ShowFolderOpenPicker` (`mobile/file.go:67`) →
  `listerForURI` (`mobile/folder.go:17`) → `canListContentURI` (`mobile/android.c:234`).
- `canListContentURI` calls `getContentResolver().getType(DocumentsContract.buildDocumentUriUsingTree(uri, docId))`
  and requires it to equal exactly `vnd.android.document/directory`.
- On API 29, for a stock `OPEN_DOCUMENT_TREE` result, `getType()` on that built document URI returns
  null / non-directory → Fyne returns `callback(nil, "specified URI is not listable")` and
  **discards the raw tree URI**. On API 31+ `getType()` returns the directory MIME → works.
  (The exact Android-framework reason `getType()` differs on API 29 is not pinned down; Fyne's gate
  is the proximate cause we bypass.)
- `filesSave` (recv.go:1193) then sees `err != nil`, `lu == nil` → falls into the
  `ChildDownload` branch (recv.go:1310) → files saved to Downloads (works, wrong place).
- Log export works because it uses `ACTION_CREATE_DOCUMENT` (file save), which returns a file URI
  and uses `storage.Writer` — no listability gate.

**Key assumption (verify in validation):** the stock tree URI is **writable** on API 29 even though
`getType()` fails — `OPEN_DOCUMENT_TREE` grants write, and `DocumentsContract.createDocument` does
not depend on `getType()`. So bypassing the gate should let writes succeed. If, contrary to
expectation, the URI is not writable on API 29, the bypass would pick but fail at write time and the
plan would need revisiting (see Risks).

Saving multiple received files never calls `List()` (each file goes through
`ChildViaMediaStore`/`CreateFileInTree` → `DocumentsContract.createDocument`). Listing IS used by
Send "add folder" (`walkDir`→`List`, send.go:2172), so the fix makes the returned URI genuinely
listable via the app's existing `list()` (for_android.go:855, calls native `getChildrenURIs`, which
queries SAF children — independent of Fyne's gate).

## Decision
On **Android only**, route `ShowFolderOpen` (recv.go:1455) through a **native**
`ACTION_OPEN_DOCUMENT_TREE` picker that returns the raw tree URI, bypassing Fyne's gate. Reuse the
existing Java→Go callback pattern (`lifecycleEvent`/`intentURI`/`intentText` in for_android.go /
for_android.c). iOS keeps `dialog.ShowFolderOpen`; desktop keeps its custom path.

This fixes both Receive "save all" and Send "add folder" on API 29 via one choke point, and is a
no-op regression on API 31+/iOS/desktop.

## Implementation

### 1. Java — `GoNativeActivity.java`
- Add constant `private static final int FOLDER_OPEN_CODE = 4;` (existing: FILE_OPEN=1, FILE_SAVE=2, INTENT_OPEN=3).
- Add `private native void folderPickerReturned(String uri);` (next to line 50 `filePickerReturned`).
- Add static launcher mirroring `doShowFileOpen` (line 741):
  ```java
  static void pickFolder() { goNativeActivity.doPickFolder(); }
  void doPickFolder() {
      Intent intent = new Intent(Intent.ACTION_OPEN_DOCUMENT_TREE);
      intent.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION
                    | Intent.FLAG_GRANT_WRITE_URI_PERMISSION
                    | Intent.FLAG_GRANT_PERSISTABLE_URI_PERMISSION);
      expectingResult = true;
      startActivityForResult(intent, FOLDER_OPEN_CODE); // no createChooser (tree intent)
  }
  ```
  Run on UI thread same as `doShowFileOpen`.
- In `onActivityResult` (line 911) handle `FOLDER_OPEN_CODE` **before** the existing
  `requestCode != FILE_OPEN_CODE && != FILE_SAVE_CODE` early-return (line 917):
  ```java
  if (requestCode == FOLDER_OPEN_CODE) {
      if (resultCode == Activity.RESULT_OK && data != null && data.getData() != null) {
          Uri uri = data.getData();
          try {
              getContentResolver().takePersistableUriPermission(uri,
                  Intent.FLAG_GRANT_READ_URI_PERMISSION | Intent.FLAG_GRANT_WRITE_URI_PERMISSION);
          } catch (SecurityException e) { Log.w(TAG, "takePersistableUriPermission: " + e); }
          folderPickerReturned(uri.toString());
      } else {
          folderPickerReturned("");
      }
      return;
  }
  ```
  Wrap persist in try/catch (some providers reject it; not fatal).
- AndroidManifest already declares handling `android.intent.action.OPEN_DOCUMENT_TREE` (line 89) and
  the provider `<intent-filter>`; no manifest change expected. (`IsFolderPickerSupported` already
  queries `OPEN_DOCUMENT_TREE` resolve — recv.go:1392 guard stays.)

### 2. C bridge — `for_android.h` + `for_android.c`
- Header: declare `jint pickFolder(JNIEnv* env, jobject context);`
- Source: add `pickFolder(...)` calling the static `pickFolder` method (clone of `callVoid` body,
  method signature `()V`, name `"pickFolder"`).
- Add JNI entry mirroring `Java_org_golang_app_GoNativeActivity_intentURI` (for_android.c:462):
  ```c
  JNIEXPORT void Java_org_golang_app_GoNativeActivity_folderPickerReturned(JNIEnv *env, jobject thiz, jstring uri) {
      if (uri == NULL) { folderPickerReturnedNotify((char*)""); return; }
      const char *curi = (*env)->GetStringUTFChars(env, uri, NULL);
      if (caseException(env, "folderPickerReturned GetStringUTFChars") || curi == NULL) return;
      folderPickerReturnedNotify((char*)curi);
      (*env)->ReleaseStringUTFChars(env, uri, curi);
  }
  ```

### 3. Go Android — `for_android.go`
- Channel + exported notify (mirror `lifecycleFromJava`/`lifecycleEventNotify`, lines 1066–1077):
  ```go
  var folderFromJava = make(chan string, 1)
  //export folderPickerReturnedNotify
  func folderPickerReturnedNotify(uri *C.char) {
      goURI := C.GoString(uri) // "" == cancelled
      select { case folderFromJava <- goURI: default: } // keep latest; drop stale
  }
  ```
- `pickFolder()` wrapper (mirror `callVoid`, for_android.go top helpers):
  ```go
  func pickFolder() {
      driver.RunNative(func(ctx interface{}) error {
          ac, ok := ctx.(*driver.AndroidContext)
          if !ok { return errJNI }
          C.pickFolder((*C.JNIEnv)(unsafe.Pointer(ac.Env)), (C.jobject)(unsafe.Pointer(ac.Ctx)))
          return nil
      })
  }
  ```
- `ListableURI` wrapper (real listing via `list()`):
  ```go
  type androidListable struct{ fyne.URI }
  func (a androidListable) List() ([]fyne.URI, error) {
      if a.URI == nil { return nil, nil }
      if a.URI.Scheme() == "content" { return list(a.URI) }   // for_android.go:855
      return storage.List(a.URI)
  }
  ```
- Async entry consumed by `ShowFolderOpen`:
  ```go
  // pickFolderAsync launches the native folder picker. Returns true if handled (Android).
  func pickFolderAsync(cb func(fyne.ListableURI, error)) bool {
      // drain any stale result
      select { case <-folderFromJava: default: }
      pickFolder()
      go func() {
          s := <-folderFromJava
          fyne.Do(func() {
              if s == "" { cb(nil, nil); return }   // cancelled
              u, err := storage.ParseURI(s)
              if err != nil { cb(nil, err); return }
              cb(androidListable{u}, nil)
          })
      }()
      return true
  }
  ```
  Notes: drain stale before launching; wait on goroutine; deliver via `fyne.Do` (callback must run
  on Fyne main goroutine, like dialog callbacks).

### 4. Go non-Android stub — `for_android0.go` (`//go:build !android`)
- Add stub so `ShowFolderOpen` compiles for desktop+iOS:
  ```go
  func pickFolderAsync(cb func(fyne.ListableURI, error)) bool { return false }
  ```

### 5. `ShowFolderOpen` — `recv.go:1455`
- Change the `isMobile` branch:
  ```go
  func ShowFolderOpen(callback func(fyne.ListableURI, error), parent fyne.Window) {
      if isMobile {
          if pickFolderAsync(callback) { return }   // Android: native, bypass Fyne gate
          dialog.ShowFolderOpen(callback, parent)   // iOS
          return
      }
      // ... desktop path unchanged
  }
  ```
- No change to `filesSave`, `ChildViaMediaStore`, `CreateFileInTree`, or `ShowFilesSave`'s
  `IsFolderPickerSupported` guard — they already operate on a raw tree URI.

## Edge cases / failure modes
- **Cancel picker** → `folderPickerReturned("")` → `cb(nil, nil)` → existing "folder selection
  canceled" path (recv.go:1196). No Downloads fallback.
- **No OPEN_DOCUMENT_TREE** → `IsFolderPickerSupported()` false → `ShowFilesSave` Download fallback
  + toast (recv.go:1397) — unchanged.
- **Existing file in target** → `createDocument` auto-suffixes; no overwrite, no listing needed.
- **`takePersistableUriPermission` rejected** → caught; grant still valid for process lifetime;
  `LastFolder` write (recv.go:1315) stays consistent.
- **Send "add folder" on API 29** → wrapper `List()` uses `list()`/`getChildrenURIs` (SAF query,
  not `getType`); if provider can't list, `walkDir` already logs+returns (send.go:2173).
- **Concurrent picks** — UI doesn't allow them; buffered channel drained per launch.
- **Stale result in channel** — drained before each launch.

## Validation
1. Build: `fyne package -os android/arm64` (and 32-bit as needed) — cgo export + JNI entry compile.
2. **API 29, confirm writability assumption first**: with the **stock** SAF folder picker, pick a
   folder for "Save all" after receiving ≥1 file.
   - If files land in the chosen folder (not Downloads) → assumption holds, fix works.
   - Log should show `copy ... content://...` to the chosen tree and **no**
     `specified URI is not listable`.
   - Multi-file save: each file written into the same tree.
3. **If API 29 stock-pick write fails** (assumption violated): capture the exact chosen tree URI from
   the log (add a temporary `log.Debugf` of the raw URI in `pickFolderAsync`/`filesSave`), report it,
   and pause — the fix approach must be revisited.
4. **API 29**: Send → add folder via **stock** picker → folder uploads, or degrades gracefully.
5. **API 31 / Android 14**: regression — folder pick still works through the new native path.
6. **Cancel** on API 29 and 14: no error, no Downloads fallback.
7. **iOS**: still uses `dialog.ShowFolderOpen` (unchanged).
8. **gofmt / `go vet`** for android + non-android builds (two build configs).

## Risks / open notes
- **Writability assumption** (most important): the fix presumes the stock tree URI is writable on
  API 29 via `createDocument` despite `getType()` failing. If validation step 3 triggers, the plan
  must be revisited (possibly the URI is malformed on API 29, not merely "not listable").
- Must add `FLAG_GRANT_PERSISTABLE_URI_PERMISSION` at launch for `takePersistableUriPermission` to
  succeed (included above).
- `pickFolder` must dispatch on the UI thread (same pattern as `doShowFileOpen`).
- `onActivityResult` ordering: `FOLDER_OPEN_CODE` branch must precede the FILE_OPEN/FILE_SAVE guard.
- If a future Fyne upgrade changes the listability gate, this bypass becomes unnecessary but stays
  harmless (returns a genuinely listable URI).

---
 https://github.com/fyne-io/fyne/pull/6402/changes/afc3082cacc1be619665bb54ff1e3ff44ac768c3