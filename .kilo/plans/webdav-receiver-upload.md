# WebDAV: Receiver drag & drop upload to sender

## Problem

In WebDAV mode with a tunnel connection:
- Sender drag & drop → files appear in WebDAV → receiver sees them (works)
- Receiver drag & drop → files go to local SEND directory, never reach sender (broken)

## Root cause

`SetOnDropped` (desktop, `send.go:1687`) and `uriFromIntent` handler (Android, `send.go:1463`) always call `addPath` which creates local symlinks/copies in the SEND directory. They don't check `davServer.IsRemote()` to detect that the user is a receiver connected through a tunnel.

## Solution

When `davServer.IsRemote()` is true, upload dropped files to the sender's WebDAV server via HTTP PUT through the tunnel proxy (`link.URL`). No progress bar — just upload silently and refresh the tree.

## Implementation

### 1. New function: `uploadToWebDAV` in `webdavclient.go`

```go
func uploadToWebDAV(localPath string, targetURL *url.URL, onFileDone func(name string, err error)) error
```

- `onFileDone` callback вызывается после завершения каждого файла с именем и ошибкой (nil если успех)
- If `localPath` is a file: `PUT` to `scheme://host/path/baseName`
- If `localPath` is a directory: `filepath.Walk` → `MKCOL` for subdirs, `PUT` for files
- Use a dedicated `http.Client` **without timeout** (large files) with `InsecureSkipVerify: true`
- No progress bar

### 2. Desktop: modify `SetOnDropped` in `send.go:1687`

Insert **before** `entry.Disabled()` check:

```go
if davServer.IsRemote() && link.URL != nil {
    go func() {
        for _, uri := range uris {
            p := uri.Path()
            err := uploadToWebDAV(p, link.URL, func(name string, ferr error) {
                fyne.Do(func() {
                    if ferr != nil {
                        topline.SetText(fmt.Sprintf("upload %s: %v", name, ferr))
                    } else {
                        NewToast(w, "Uploaded "+name).Show()
                        topline.SetText("Uploaded " + name)
                    }
                })
            })
            if err != nil {
                log.Errorf("upload %s: %v", p, err)
                fyne.Do(func() {
                    topline.SetText(fmt.Sprintf("upload %s: %v", filepath.Base(p), err))
                })
            }
        }
        scRefresh()
    }()
    return
}
```

### 3. Android: modify `uriFromIntent` handler in `send.go:1463`

After existing deepLink/DAV checks, before `entry.Disabled()` check:

```go
if davServer.IsRemote() && link.URL != nil {
    u, err := storage.ParseURI(uriString)
    if err != nil { continue }
    go func() {
        err := uploadToWebDAV(u.Path(), link.URL, func(name string, ferr error) {
            fyne.Do(func() {
                if ferr != nil {
                    topline.SetText(fmt.Sprintf("upload %s: %v", name, ferr))
                } else {
                    NewToast(w, "Uploaded "+name).Show()
                    topline.SetText("Uploaded " + name)
                }
            })
        })
        if err != nil {
            log.Errorf("upload %s: %v", u.Path(), err)
            fyne.Do(func() {
                topline.SetText(fmt.Sprintf("upload %v", err))
            })
        }
        scRefresh()
    }()
    continue
}
```

### 4. Files to modify

| File | Changes |
|------|---------|
| `webdavclient.go` | Add `uploadToWebDAV` function with `onFileDone` callback |
| `send.go` | Modify `SetOnDropped` (~line 1687) and `uriFromIntent` handler (~line 1463) |

### 5. Key details

- **URL construction**: Upload URL = `link.URL.Scheme + "://" + link.URL.Host + "/" + filepath.Base(filePath)`. The `link.URL.Host` points to the local proxy which forwards through the tunnel to the sender.
- **HTTP client**: Dedicated upload client without timeout with `InsecureSkipVerify: true`
- **Toast + topline**: После каждого файла — тост "Uploaded filename" и запись в `topline`. При ошибке — текст ошибки в `topline`.
- **MKCOL for dirs**: Before uploading files inside a subdirectory, send `MKCOL` to create the directory on the sender's WebDAV server
- **Refresh tree**: After all uploads complete, call `scRefresh()` to refresh the WebDAV file tree
- **No `entry.Disabled()` gate**: The remote upload path must bypass the `entry.Disabled()` check since the receiver's entry may be disabled during active transfer
