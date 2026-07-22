# Fix: sent folder named "25" instead of its real name ("A"/"B")

## Symptom
User picks a folder (real name e.g. `A`/`B`) from the Android **Downloads**
provider and sends it. The created send directory is named `25`, not `A`/`B`.

Log evidence (latest run):
```
send.go: symlink /tree/msd%3A25 .../files/fyne/send/25: isMobile
copyFile content://com.android.providers.downloads.documents/tree/msd%3A25/document/msf%3A56 .../send/25/c88a0b907419a70c.txt
```
Note: `%3A` == `:`. The child file keeps its real name (`c88a0b907419a70c.txt`);
only the **folder** is misnamed.

## Root cause
- Picked URI is a SAF **tree-root** URI:
  `content://com.android.providers.downloads.documents/tree/msd%3A25`.
- Folder name comes from `uriBase(u)` (for_android.go:713): Java `getFileName`
  (GoNativeActivity.java:1058); if that returns `null`/`""` → falls back to
  `base(uri.Path())` (for_android.go:730).
- `base("/tree/msd%3A25")`: decodes `%3A`→`:`, `replace()` turns `:`→`/`, takes
  last `/`-segment → `25`.
- The Downloads provider does NOT resolve `OpenableColumns.DISPLAY_NAME` for a
  **raw tree-root** URI. It must be queried via the **document** URI built from
  `DocumentsContract.getTreeDocumentId` + `buildDocumentUriUsingTree`, exactly as
  `countChildren`/`getChildrenURIs`/`createFileInTree` already do.

## What went wrong in the two prior attempts (must not regress)
1. First attempt: **always** rebuilt via `buildDocumentUriUsingTree`. Folder → `B`
   ✓, BUT for a **child document URI** (`…/document/msf:56`), `getTreeDocumentId`
   returns the *root* id, so every child collapsed onto the root name → child
   copied as `…/send/B/B`. ✗
2. Second attempt (current, GoNativeActivity.java:1058-1097): query the URI
   directly first, then rebuild. The **direct query on the raw tree root THROWS**
   (caught by the single outer `catch` at line 1093), which aborts the method
   **before** the rebuild at line 1079 runs → returns `null` → folder `25`. ✗

Three runs confirm the model:
- query raw root (single try) → throws → `25`
- always rebuild (never query raw root) → `B` (but children broken)
- query-then-rebuild in ONE outer try → step-1 throw skips step 2 → `25`

## Correct fix
**Isolate each query attempt in its own try/catch** so an exception on the raw
tree-root direct query cannot skip the rebuild. Add a private helper and have
`getFileName` try direct, then rebuild:

```java
static String getFileName(String uriStr) {
    // 1) Direct query — works for document URIs (.../document/msf:56) and
    //    MediaStore URIs, returning each child's OWN name. For a raw tree root
    //    this throws/returns nothing; the helper swallows that so we fall through.
    String name = queryDisplayName(uriStr);
    if (name != null && !name.isEmpty()) return name;

    // 2) Bare SAF tree-root: rebuild the document URI from the tree document id
    //    and query that. Child document URIs already returned at step 1, so
    //    getTreeDocumentId never collapses a child onto the root.
    try {
        android.net.Uri uri = android.net.Uri.parse(uriStr);
        String treeDocId = android.provider.DocumentsContract.getTreeDocumentId(uri);
        android.net.Uri docUri = android.provider.DocumentsContract.buildDocumentUriUsingTree(uri, treeDocId);
        if (docUri != null) {
            name = queryDisplayName(docUri.toString());
            if (name != null && !name.isEmpty()) return name;
        }
    } catch (Exception ignored) {
        // not a tree URI -> nothing else to try
    }
    return null;
}

private static String queryDisplayName(String uriStr) {
    try {
        android.net.Uri uri = android.net.Uri.parse(uriStr);
        String[] projection = {android.provider.OpenableColumns.DISPLAY_NAME};
        android.database.Cursor cursor = goNativeActivity.getContentResolver().query(uri, projection, null, null, null);
        if (cursor != null) {
            try {
                if (cursor.moveToFirst() && cursor.getString(0) != null) return cursor.getString(0);
            } finally { cursor.close(); }
        }
    } catch (Exception e) {
        Log.e(TAG, "Java: queryDisplayName failed: " + e.getMessage());
    }
    return null;
}
```

### Why this is correct for every case
| URI type | step 1 (direct) | step 2 (rebuild) | result |
|---|---|---|---|
| tree root `…/tree/msd:25` | throws/null → null (isolated) | rebuild → `B` | **B** ✓ |
| child doc `…/document/msf:56` | real child name | (not reached) | **real child name** ✓ |
| MediaStore `content://media/…/123` | real name | not a tree URI (getTreeDocumentId throws, caught) | **real name** ✓ |
| `file://` | throws → null | not a tree → null → Go `base()` fallback | unchanged ✓ |

## Scope / safety
- `getFileName` has one caller: `uriBase` (for_android.go:714) → `send.go` folder
  name + `copy.go` child names. Behavior for document/regular URIs is identical to
  the original (direct query only); only bare tree roots gain correct resolution.
- No Go change; `base()` stays the last-resort fallback.
- `DocumentsContract` API 19+, tree helpers API 21+; minSdk 23 (AndroidManifest.xml:117) — fine.

## Validation
- `make arm64 wsl` (AGENTS.md) — `make arm64` compiles the `.java`, so a Java
  error surfaces there.
- On device, pick folder `A`/`B` from Downloads → Send: expect entry named `A`/`B`
  and contents under `…/send/A/<real child names>` (NOT `A/A`, NOT `25/...`).
- Regression guard: send a single regular file from Downloads → still its real name.

## Out of scope
- Go `base()`/`replace()` heuristic unchanged (final fallback).
- Receive-side naming untouched.
