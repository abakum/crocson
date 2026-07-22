# Fix Cyrillic File Path Issue in Android Intent Handling

## Problem Summary

When handling Android intents with Cyrillic filenames, `os.Stat(u.Path())` fails because the path contains URL-encoded characters (e.g., `%D0%92%D0%B5...` instead of actual Cyrillic letters). This causes the code to skip file copying, and `reload()` subsequently removes the UI entry.

### Symptoms
- Files with Cyrillic names are not copied from Download to send/ directory
- UI entry is added then removed by `reload()`
- Only affects Cyrillic filenames; ASCII files work fine

### Root Cause
`uri.Path()` returns URL-encoded paths for Cyrillic characters. The code at `send.go:1793` calls `os.Stat(u.Path())` directly without decoding, causing `os.Stat` to fail.

The `uriBase()` function already handles URL decoding via `url.PathUnescape()`, but we need a `uriPath()` function for full paths (not just the filename).

## Solution

Create `uriPath()` function similar to the existing `base()` function in `for_android.go`, which properly decodes URL-encoded paths before use.

### Implementation

#### 1. Add `uriPath()` function to `for_android.go`

```go
func uriPath(uri fyne.URI) string {
    path := uri.Path()
    decoded, err := url.PathUnescape(path)
    if err != nil {
        decoded = strings.ReplaceAll(path, "%2F", "/")
        decoded = strings.ReplaceAll(decoded, "%3A", ":")
    }
    return decoded
}
```

#### 2. Update `send.go` at line 1793

Replace:
```go
if fi, err := os.Stat(u.Path()); err == nil {
```

With:
```go
srcPath := uriPath(u)
if fi, err := os.Stat(srcPath); err == nil {
```

#### 3. Update all `CopyFileProgress` calls to use `srcPath`

Replace `u.Path()` with `srcPath` in:
- Line 1811: `CopyFileProgress(src, dstPath, ...)`
- Line 1839: `CopyFileProgress(u.Path(), dst, fe, ...)`
- Any other file operations using the source path

## Validation Plan

1. Test with Cyrillic filenames from Download folder
2. Test with ASCII filenames (regression test)
3. Test with mixed ASCII/Cyrillic filenames
4. Test on both Android (apiLevel 28+) and Windows
5. Verify logs show successful copying instead of errors

## Edge Cases

- `url.PathUnescape()` handles already-decoded strings correctly (idempotent)
- Invalid encoding falls back to string replacement (like `base()` does)
- Error cases log details for debugging