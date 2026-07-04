# Plan: Fix `file://` share permission drop on API 29–32

## Context
Sharing a `file://` URI into crocson triggers `sendIntentURIs` (`GoNativeActivity.java:1022`), which requests `[READ_EXTERNAL_STORAGE, WRITE_EXTERNAL_STORAGE]` and defers delivery to Go via `pendingIntentURIs` until `onRequestPermissionsResult` (`:1048`).

Single delivery is already structural: the permission branch `return`s without delivering; only the callback delivers, then nulls `pendingIntentURIs` (`:1064`). So **double-send is not a risk** — the fix keeps that guarantee.

**The bug (API 29–32):** `WRITE_EXTERNAL_STORAGE` is undeclared there (`maxSdkVersion="28"`, `AndroidManifest.xml:108`) → auto-denied at runtime. `onRequestPermissionsResult` requires **all** `grantResults==0` (`:1053-1056`) → `granted=false` → `pendingIntentURIs` dropped (`:1064`) → file silently lost; re-share never succeeds.

- API **≤28**: both perms are in the "Storage" group (grant/deny together) → works (the user's device shows no problem).
- API **33+**: inter-app sharing is `content://` (FileProvider forbids `file://` since Android 7); the gate triggers only on `file://` (`:1026`), and `content://` is read via Go `Reader`/content-resolver with **no storage permission** (`for_android.go:1026`). So no `READ_MEDIA_*` is needed for the share path.

## Decision
Minimal, policy-clean fix: in `onRequestPermissionsResult`, deliver pending URIs if **any** requested permission is granted (instead of requiring all). This unblocks 29–32 (READ granted → deliver; the auto-denied undeclared WRITE no longer blocks). No manifest change, no `READ_MEDIA_*`, no Go change, ≤28 behavior unchanged.

## Tasks
1. **`GoNativeActivity.java` — `onRequestPermissionsResult`** (`:1052-1065`): change the grant determination from all-required to any-granted:
   - `boolean granted = false;`
   - loop: `if (result == PackageManager.PERMISSION_GRANTED /* 0 */) granted = true;`
   - keep the delivery loop (`logIntentURI`) and `pendingIntentURIs = null;` unchanged.
2. No other edits.

## Risks / notes
- **≤28:** no behavioral change — the pair grants/denies together, so any-granted ≡ all-required.
- **33+:** realistic share is `content://` (gate skipped, no permission). The artificial `file://` path still drops (no storage permission can help under scoped storage) — acceptable, non-real-world.
- `requestCode == 123` is shared with `createFileInDownloads`, but that path does not set `pendingIntentURIs`, so the callback branch is share-only — no routing conflict.
- Defensive: an empty `grantResults` leaves `granted=false` (drop) — acceptable.

## Validation
- **API ≤28 (regression):** share `file://` image → grant → file appears once.
- **API 29–32 emulator:** `adb shell pm revoke com.github.abakum.crocson android.permission.READ_EXTERNAL_STORAGE`; then
  `adb shell am start -a android.intent.action.VIEW -d "file:///sdcard/Picture/test.jpg" -n com.github.abakum.crocson/org.golang.app.GoNativeActivity`;
  expect READ dialog → grant → file appears once (previously lost).
- **API 33+ emulator:** share a file from the system Files app (`content://`) → appears with no permission dialog.
- Re-share the same file after grant → no duplicate entries.

## Out of scope
- `createFileInDownloads` re-press after grant (`:352-358`, download/save path) — separate issue.
- `READ_MEDIA_*` / SAF adoption for arbitrary non-media `file://` on 33+ — not needed for the share path.

## Status (post-implementation & testing on API 28 emulator)
**Done:**
- Task 1 implemented: `GoNativeActivity.java:1053-1055` now uses any-granted (`PackageManager.PERMISSION_GRANTED`). Compiles vs `android-36` (`javac`, exit 0).
- Built (`make 386`, exit 0) and installed (`make adb`, `Success`) on the API 28 (Android 9, x86) emulator.
- End-to-end verified on API 28: sharing `file:///sdcard/Download/test.txt` triggers the permission dialog (`needs permission for file://` → `requesting permissions` → `permissionDialog`); after grant, the file reaches the **send basket** ("корзина на передачу"). No regression on the already-granted path (`sendIntentURIs sending 1 URIs to Go` → `intent: URI ...`).

**Validated on API 31 (the fix's target range):**
- `file://` share (`file:///sdcard/Download/...jpg`) → gate fires (`needs permission for file://`, `requesting permissions`, `permissionDialog`). Activity **survives** the dialog on 31 (`SDK > 28` → `onUserLeaveHint` does not call `finishActivity`; observed `onResume` after grant), so the callback path is used.
- On grant: `onRequestPermissionsResult requestCode=123, grantResults=0` → `permissionResult granted=true, pending URIs=1` → `sending URI` → `intent: URI ...` → Go copies to `files/fyne/send/...jpg` (success log at `send.go:1834`). File reaches the send basket.
- Decisive: the request array is `[READ, WRITE]`; WRITE is undeclared on 31 → auto-denied. The **old** all-required check would have set `granted=false` and dropped the file; the new **any-granted** delivers on READ=granted. Confirmed.
- A prior denied attempt logged `grantResults=-1` → `granted=false`, no delivery — correct behavior on denial.

**Minor / out of scope:**
- `content://` share on 33+ and re-share-no-duplicate are not separately exercised; both follow the already-structural single-delivery path (immediate `logIntentURI` for `content://`; `pendingIntentURIs` nulled after the callback for `file://`).

**Note on the API 28 lifecycle (clarified — NOT a defect):**
- On API ≤28 the crocson activity is **intentionally finished** when the permission dialog appears: `onUserLeaveHint` calls `finishActivity()` (→ `super.onBackPressed()`) when `SDK_INT <= 28 && !expectingResult` (`GoNativeActivity.java:969-977, 942`). The permission request path does not raise `expectingResult`, so the activity is finished (a deliberate workaround: on ≤28 the render engine otherwise crashes while behind the dialog). Hence on ≤28 the instance field `pendingIntentURIs` is lost and delivery happens via **intent re-delivery on activity recreation** (after grant, `onCreate` re-runs `processIntentData`, READ is now granted → direct delivery) — observed: file reaches the send basket.
- On **API 29+** (the fix's actual target) this `SDK_INT <= 28` branch is skipped, the activity **survives** the dialog, the callback `onRequestPermissionsResult` fires with `pendingIntentURIs` intact, and the any-granted change delivers. So the fix is effective where it matters; no follow-up needed for lifecycle robustness.
