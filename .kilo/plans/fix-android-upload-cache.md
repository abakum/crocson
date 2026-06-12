# Fix: Android upload — cache to sandbox first, then upload

## Problem

On Android, `u.Path()` returns a content URI like `/tree/primary%3A/document/...` which is not a filesystem path. `os.Stat()` / `os.Open()` fail. Need to cache to sandbox (`join()`) first via Android content resolver, then upload the cached file.

## Fix

Replace the Android `davServer.IsRemote()` block in `send.go` (~line 1495-1525).

### New flow for Android intent when `davServer.IsRemote()`:

1. Parse URI → `storage.ParseURI(uriString)`
2. Normalize proxy URL → `isDAV(link.URL.String())`
3. Determine name → `uriBase(u)`, dst → `join(name)`
4. Cache to sandbox (reuse existing patterns):
   - `file://` + directory: `copyFiles` to `dst`, collect all cached paths
   - `file://` + file: `CopyFileProgress` to `dst`
   - Content URI (non-file): `Reader(u)` + `copyFromURCProgress` to `dst`
5. After caching completes: `uploadToWebDAV(dst, proxyURL, onFileDone)`
6. After upload: `scRefresh()` to refresh tree

### Implementation

Replace the current block:

```go
if davServer.IsRemote() && link.URL != nil {
    u, err := storage.ParseURI(uriString)
    if err != nil { ... continue }
    _, _, proxyURL, _ := isDAV(link.URL.String())
    if proxyURL == nil { continue }
    name := uriBase(u)
    dst := join(name)

    if u.Scheme() == "file" {
        if fi, err := os.Stat(u.Path()); err == nil {
            if fi.IsDir() {
                // Directory: copy all to cache, then upload each
                go func() {
                    var cached []string
                    copyFiles(storage.NewFileURI(u.Path()), dst, func(fileURI fyne.URI, dstPath string) error {
                        cached = append(cached, dstPath)
                        CopyFileProgress(fileURI.Path(), dstPath, nil, func(err error) {
                            if err != nil {
                                log.Errorf("cache %s: %v", dstPath, err)
                            }
                        })
                        return nil
                    })
                    for _, c := range cached {
                        uploadToWebDAV(c, proxyURL, func(name string, ferr error) {
                            fyne.Do(func() { ... toast/topline ... })
                        })
                    }
                    scRefresh()
                }()
                continue
            }
            // File: copy to cache, then upload
            CopyFileProgress(u.Path(), dst, nil, func(err error) {
                if err != nil { ... return }
                go uploadToWebDAV(dst, proxyURL, onFileDone)
                scRefresh()
            })
            continue
        }
    }
    // Content URI: Reader → cache, then upload
    source, err := Reader(u)
    if err != nil { ... continue }
    copyFromURCProgress(source, dst, nil, func(err error) {
        if err != nil { ... return }
        go uploadToWebDAV(dst, proxyURL, onFileDone)
        scRefresh()
    })
    continue
}
```

### Key points

- Reuse existing `CopyFileProgress`, `copyFiles`, `copyFromURCProgress` for caching
- After cache complete, call `uploadToWebDAV(dst, proxyURL, ...)` on the local cached path
- For directories, collect all cached file paths and upload each
- Toast/topline feedback after each uploaded file
- No changes to `webdavclient.go` — `uploadToWebDAV` already works with local filesystem paths
- Only modify `send.go` Android `uriFromIntent` handler (~lines 1495-1525)

### File to modify

| File | Lines | Change |
|------|-------|--------|
| `send.go` | 1495-1525 | Replace direct `uploadToWebDAV(u.Path(), ...)` with cache-then-upload flow |
