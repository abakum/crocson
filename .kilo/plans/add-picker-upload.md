# Add upload to file picker and folder picker when remote

## Context

`addFileButton` (file picker, ~line 1823) and `addFolderButton` (folder picker, ~line 1890) currently always cache files locally. When `davServer.IsRemote()` they should upload to sender via WebDAV instead.

## Changes — `send.go` only

### 1. `addFileButton` callback (~line 1838)

Inside `ShowFileOpen(func(source fyne.URIReadCloser, e error) {`, after the nil/error checks, add remote upload branch before existing symlink/cache logic:

```go
ShowFileOpen(func(source fyne.URIReadCloser, e error) {
    // ... nil/error checks ...
    u := source.URI()

    // Remote upload
    if davServer.IsRemote() && link.URL != nil {
        _, _, proxyURL, _ := isDAV(link.URL.String())
        if proxyURL != nil {
            go uploadFromURC(source, proxyURL, func(name string, ferr error) {
                fyne.Do(func() {
                    if ferr != nil {
                        topline.SetText(fmt.Sprintf("upload %s: %v", name, ferr))
                    } else {
                        NewToast(w, "Uploaded "+name).Show()
                        topline.SetText("Uploaded " + name)
                    }
                })
                scRefresh()
            })
            return
        }
    }

    // ... existing symlink/cache logic unchanged ...
}, w)
```

Note: `uploadFromURC` consumes and closes the `source`, so `return` after it — no fallthrough to existing code.

### 2. `addFolderButton` callback (~line 1891)

Inside `folderOpen := func(u fyne.ListableURI, e error) {`, after nil/error checks, add remote upload branch:

```go
folderOpen := func(u fyne.ListableURI, e error) {
    // ... nil/error checks ...

    // Remote upload
    if davServer.IsRemote() && link.URL != nil {
        _, _, proxyURL, _ := isDAV(link.URL.String())
        if proxyURL != nil {
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
            go func() {
                copyFiles(u, "", func(src fyne.URI, dstPath string) error {
                    if IsDirectory(src) {
                        return nil
                    }
                    source, err := Reader(src)
                    if err != nil {
                        log.Errorf("reader %s: %v", src, err)
                        return nil
                    }
                    uploadFromURC(source, proxyURL, onUploaded)
                    return nil
                })
                scRefresh()
            }()
            return
        }
    }

    // ... existing symlink/cache logic unchanged ...
}
```

Note: For folder upload via `copyFiles`, `dstPath` parameter is not needed (we pass `""`) since `uploadFromURC` determines the filename from `source.URI()`. Directories are skipped — `uploadFromURC` doesn't handle dirs, and `MKCOL` for remote subdirs would need the full relative path which isn't available in this callback. Files inside subdirs would upload to root of WebDAV.

### File to modify

| File | Change |
|------|--------|
| `send.go` | Add `davServer.IsRemote()` branches in `addFileButton` and `addFolderButton` callbacks |
