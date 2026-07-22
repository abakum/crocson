# Plan: Cap READ_EXTERNAL_STORAGE at API 28

## Context

`AndroidManifest.xml:111` declares `READ_EXTERNAL_STORAGE` with `maxSdkVersion="32"`;
`WRITE_EXTERNAL_STORAGE` is already capped at `28` (`:110`). The asymmetry (READ 32 /
WRITE 28) was kept so the `file://` share-in path could request READ on API 29–32 (see
`.kilo/plans/20260704-share-file-uri-permissions.md`).

Post-pull analysis shows the `32` cap buys nothing real — READ is only effective on API ≤28:

- `createFileInDownloads` (`GoNativeActivity.java:1154`) requests storage permission **only
  on API ≤28** (`Build.VERSION.SDK_INT >= 29` → `createFileInDownloadsModern` via MediaStore,
  no permission). This is why Android 10 never prompts for READ on saves/downloads — the
  observation that prompted this change.
- `content://` shares are read via the content resolver — no storage permission needed. Only
  `file://` URIs hit the permission gate in `sendIntentURIs` (`:1854`).
- `file://` is only reliably readable on **API ≤28** (legacy storage). With `targetSdk="36"`
  (`:118`) and no `requestLegacyExternalStorage`, scoped storage blocks reading another app's
  `file://` path on API 29+ even with READ granted — Go's `os.Open`/`CopyFileProgress`
  (`send.go`, per commit `7e47c31`) get `EACCES`.
- Commit `7e47c31` confirms `file://` is real in the wild (Total Commander on Android 9 =
  API 28, where READ works and stays declared).

So READ is effective only on API ≤28, identical to WRITE. Cap it at 28.

## Change (single edit)

**`AndroidManifest.xml:111`** —
`android:maxSdkVersion="32"` → `android:maxSdkVersion="28"`.

```xml
<uses-permission android:name="android.permission.READ_EXTERNAL_STORAGE" android:maxSdkVersion="28" />
```

No Java or Go change required.

## Behavioral effect (post-change)

- **API ≤28:** unchanged. READ is declared and granted together with WRITE (same permission
  group). `createFileInDownloads` legacy saves and `file://` share-in both keep working.
- **API 29–32:** READ no longer declared. `sendIntentURIs` (`:1864`) still calls
  `requestPermissions([READ,WRITE],123)`; both undeclared → system returns an immediate
  auto-deny (**no dialog**) → `onRequestPermissionsResult` (`:1891`) sees
  `grantResults[0] == -1` → `granted=false` → emits `storagePermissionDenied` (Go shows the
  existing "Storage permission required" toast via `send.go:1645-1651`) and drops
  `pendingIntentURIs`. Net: a `file://` share-in is not received — identical to current
  API 33+ behavior and to the scoped-storage reality (the file was unreadable anyway).
  `case "permissionDialog":` (`send.go:1640`) is a no-op, so that emitted event has no effect.
- **API 33+:** no change (READ was already undeclared at cap=32).

## Why no Java change is needed

- `createFileInDownloads` is already guarded by `SDK_INT < 29`.
- `onRequestPermissionsResult` derives `granted` from `grantResults[0]` (READ, the first
  element of the request array), so an undeclared READ cleanly yields `granted=false`.
- The 20260704/20260721 handling remains correct for API ≤28, where it actually matters.

## Out of scope (optional, not required)

`sendIntentURIs` (`:1864`) still issues a no-op `requestPermissions` on API 29–32 — harmless
(immediate auto-deny, no dialog) but it triggers the "Storage permission required" toast for
a `file://` share-in, which is mildly misleading (the real blocker is scoped storage, not the
permission). A `Build.VERSION.SDK_INT <= 28` guard there would skip the request and deliver
the `file://` straight to Go (which fails under scoped storage via its own error path, with no
permission toast). Defer unless that edge UX matters; the manifest-only change is sufficient
and matches the request.

## Validation

1. `make arm64 wsl` — both targets build (`AGENTS.md`).
2. **API ≤28 emulator (regression):** mass save → READ/WRITE prompt → grant → files save;
   share `file:///sdcard/Download/t.txt` → file received.
3. **API 29–32 device:** launch + mass save → **no** storage prompt (Downloads via
   MediaStore); confirm no stray READ prompt in normal use (the original observation).
4. **API 33+ device:** normal use → no storage prompt (already true; sanity check only).
5. Optional (API 29–32): share a `file://` → no dialog, app does not crash, file not received
   (expected under scoped storage).
