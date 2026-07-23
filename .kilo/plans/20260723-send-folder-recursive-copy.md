# Plan: recursive (multi-level) folder copy via SAF picker on Android

## Problem
When a folder is picked for the **send** basket (`addFolderButton` → `ShowFolderOpen` →
`copyFiles`), only files at **depth 1** are copied; nested subdirectories are skipped.

This is **not** a fundamental Android limitation — `ACTION_OPEN_DOCUMENT_TREE` grants
recursive read access to the whole subtree. The depth-1 cap comes from two places:

1. `GoNativeActivity.getChildrenURIs` / `countChildren` compute the parent document id
   via `getTreeDocumentId(uri)`. For a **nested** document URI
   (`…/tree/<root>/document/<child>`) this returns the **root** id, so listing at
   depth 2 would re-list the root's children (wrong + would loop without the guard).
2. `copy.go:104` — an explicit workaround for #1 that aborts recursion:
   ```go
   if isAndroid && deep > 1 {
       return fmt.Errorf("walk deep %d", deep)
   }
   ```

## Decisions (confirmed)
- **Guard**: removed completely; rely on the existing `visited` sync.Map for cycle
  protection (SAF returns a distinct document URI per document, so no false collisions).
- **Empty subdirs**: out of scope — keep current behavior (a dir whose `List` yields 0
  children returns `count == 0` and is skipped). croc accounts for empty folders
  separately via `GetFilesInfo`.

## Changes

### 1. `GoNativeActivity.java` — correct parent doc-id for nested dirs
Add a helper near `countChildren` (~line 1298) that returns the directory's **own**
document id, falling back to the tree id only for a bare tree-root:

```java
// document-id каталога, ЧЬИХ детей нужно получить:
//   вложенный документ (.../tree/<root>/document/<child>) -> getDocumentId -> <child>
//   голый tree-root      (.../tree/<root>)                -> getTreeDocumentId -> <root>
private static String listParentDocumentId(android.net.Uri uri) {
    try {
        return android.provider.DocumentsContract.getDocumentId(uri);
    } catch (Exception ignored) {
        return android.provider.DocumentsContract.getTreeDocumentId(uri);
    }
}
```

Use it in **both** `countChildren` (line 1304) and `getChildrenURIs` (line 1332):

```java
String parentDocId = listParentDocumentId(uri);
childUri = android.provider.DocumentsContract.buildChildDocumentsUriUsingTree(uri, parentDocId);
```

`buildChildDocumentsUriUsingTree` extracts the tree root from `uri` itself, so passing the
nested child URI + its own doc-id yields `…/tree/<root>/document/<child>/children` — correct
at every depth. The existing `catch (e1)` fallback (for non-tree / MediaStore URIs) is
unchanged, so behavior for those URIs is preserved.

Note: `processChildrenCursor` already builds grandchild URIs via
`buildDocumentUriUsingTree(treeUri, docId)`, which is depth-agnostic → no change needed there.

### 2. `copy.go` — remove the depth guard
Delete lines 104-106 in `walk`:
```go
if isAndroid && deep > 1 {
    return fmt.Errorf("walk deep %d", deep)
}
```
`deep` is still used for `finalRelPath` computation; keep it. The `visited.Store` +
`visited.Load` cycle protection remains.

## Why this is correct for every case
| URI passed to getChildrenURIs | `listParentDocumentId` | children listed |
|---|---|---|
| bare tree root `…/tree/msd:25` | `getDocumentId` throws → `getTreeDocumentId` → `msd:25` | root's children ✓ |
| nested subdir `…/tree/msd:25/document/msf:56` | `getDocumentId` → `msf:56` | subdir's children ✓ (new) |
| grandchild `…/tree/msd:25/document/msf:56/document/msf:99` | `getDocumentId` → `msf:99` | grandchild's children ✓ |
| non-tree MediaStore doc | `listParentDocumentId` throws → existing `catch(e1)` fallback | unchanged ✓ |

## Affected call sites (all benefit, all safe)
- `send.go` folder picker copy (`copyFiles`, line ~2238) — **primary target**.
- `send.go` intent `file://` dir copy (line ~1800) — uses local fs, unaffected path but shares `copyFiles`.
- `recv.go` `copyFiles` (lines ~475, ~694).
- `for_android.go` `IsDirectory` fallback + `storageChild` via `countChild`.

## Risks / notes
- `IsDirectory` (for_android.go:806) falls back to `countChild` when MIME type is empty /
  `application/octet-stream`. A file whose provider omits its MIME could be misdetected as a
  directory → `List` returns 0 children → `count == 0` error → that file is skipped (no crash).
  Pre-existing behavior at depth 1; not worsened by this change. Downloads/MediaStore files
  normally expose a MIME type, so `MimeType` returns it and the file is handled correctly.
- Large/very deep trees produce one progress-bar entry per **file** (with a rel-path label) —
  existing UX, just more entries.
- API: `getDocumentId`/`getTreeDocumentId`/`buildChildDocumentsUriUsingTree` are API 21+;
  minSdk 23 (AndroidManifest.xml) — fine.

## Validation
- `make arm64 wsl` (AGENTS.md) — `make arm64` compiles the `.java`, so a Java error surfaces there.
- On device: pick a folder containing nested subdirs (≥2 levels) with files at each level →
  Send. Expect every nested file cached under `…/send/<folder>/<subdir>/…/<file>` and shown
  with its relative path in the basket; no `walk deep N` errors in logcat.
- Regression: pick a flat folder (depth 1 only) → same result as before. Pick a single
  regular file → unchanged.

## Out of scope
- Empty subdirectory creation in the send cache.
- `copy.go` `deep`/`finalRelPath` path-join logic (unchanged).
- Receive-side naming and `getFileName` tree-root resolution
  (covered by `.kilo/plans/20260723-send-tree-folder-name.md`).
