# Android upload: stream directly via uploadFromURC

## Changes

### 1. New function `uploadFromURC` in `webdavclient.go`

Similar to `copyFromURCProgress` but uploads directly via PUT instead of writing to local file. No progress bar, no caching.

```go
func uploadFromURC(source fyne.URIReadCloser, targetURL *url.URL, onFileDone func(name string, err error)) {
    if source == nil {
        if onFileDone != nil { onFileDone("", fmt.Errorf("nil source")) }
        return
    }
    defer source.Close()

    name := uriBase(source.URI())
    if name == "" || name == "/" {
        name = "file"
    }
    uploadURL := fmt.Sprintf("%s://%s%s", targetURL.Scheme, targetURL.Host,
        path.Join("/", targetURL.Path, url.PathEscape(name)))

    req, err := http.NewRequest(http.MethodPut, uploadURL, source)
    if err != nil {
        if onFileDone != nil { onFileDone(name, err) }
        return
    }

    resp, err := uploadHTTPClient.Do(req)
    if err != nil {
        if onFileDone != nil { onFileDone(name, err) }
        return
    }
    defer resp.Body.Close()

    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        if onFileDone != nil { onFileDone(name, fmt.Errorf("HTTP %d", resp.StatusCode)) }
        return
    }

    if onFileDone != nil { onFileDone(name, nil) }
}
```

### 2. Modify Android intent handler in `send.go` (~line 1495)

Replace the `davServer.IsRemote()` block. New logic:

- **`file://` scheme**: use `uploadToWebDAV(u.Path(), ...)` directly (no caching, already works with filesystem paths)
- **Content URI (non-file)**: use `uploadFromURC(source, proxyURL, onUploaded)` — stream via `Reader()` directly to PUT, no caching

```go
if davServer.IsRemote() && link.URL != nil {
    u, err := storage.ParseURI(uriString)
    if err != nil { ... continue }
    _, _, proxyURL, _ := isDAV(link.URL.String())
    if proxyURL == nil { continue }
    onUploaded := func(name string, ferr error) {
        fyne.Do(func() {
            if ferr != nil {
                topline.SetText(fmt.Sprintf("upload %s: %v", name, ferr))
            } else {
                NewToast(w, "Uploaded "+name).Show()
                topline.SetText("Uploaded " + name)
            }
        })
    }

    if u.Scheme() == "file" {
        go func() {
            if err := uploadToWebDAV(u.Path(), proxyURL, onUploaded); err != nil {
                log.Errorf("upload %s: %v", u.Path(), err)
            }
            scRefresh()
        }()
        continue
    }

    // Content URI — stream directly via Reader()
    if IsDirectory(u) { continue }
    source, err := Reader(u)
    if err != nil {
        log.Errorf("reader: %v", err)
        continue
    }
    go uploadFromURC(source, proxyURL, func(name string, ferr error) {
        onUploaded(name, ferr)
        scRefresh()
    })
    continue
}
```

### Files to modify

| File | Change |
|------|--------|
| `webdavclient.go` | Add `uploadFromURC` function |
| `send.go` | Simplify Android `davServer.IsRemote()` block — `file://` direct, content URI via `uploadFromURC` |
