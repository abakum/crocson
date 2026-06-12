# Keep pickers enabled during remote WebDAV + upload clipboard

## Problem

When receiver connects to sender via tunnel:
- `allEnabled(false, cosED...)` disables everything including `addClipButton`, `addFileButton`, `addFolderButton`
- These should stay enabled when `davServer.IsRemote()` so receiver can upload files back to sender

## Changes — `send.go` only

### 1. Create `cosPickers` group, exclude from `cosED`

After the three buttons are created, instead of:
```go
cosED = append(cosED, addClipButton)
cosDAV = append(cosDAV, addClipButton)
```
Do:
```go
cosPickers = append(cosPickers, addClipButton)
cosDAV = append(cosDAV, addClipButton)
```

Same for `addFileButton` and `addFolderButton`.

Add `var cosPickers []fyne.CanvasObject` declaration alongside existing `cosED`, `cosSH`, etc.

### 2. Add `cosPickers` to `cosDAVremote`

```go
cosDAVremote = append(cosDAVremote, addClipButton, addFileButton, addFolderButton)
```

This ensures they get re-enabled when remote proxy activates (line 759).

### 3. Re-enable pickers after `allEnabled(false, cosED...)` when remote

At each place where `allEnabled(false, cosED...)` is called during active sending (lines 826, 948, 1207), add right after:
```go
if davServer.IsRemote() {
    allEnabled(true, cosPickers...)
}
```

This keeps pickers enabled even during active croc send if connected remotely.

### 4. Add clipboard upload in `addClipButton`

In the `addClipButton` callback, after writing clipboard text to local file `src`, add remote upload:
```go
source.Close()

if davServer.IsRemote() && link.URL != nil {
    _, _, proxyURL, _ := isDAV(link.URL.String())
    if proxyURL != nil {
        go uploadToWebDAV(src, proxyURL, func(name string, ferr error) {
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
    }
}
```

### Summary of edits

| Location | Change |
|----------|--------|
| Line ~62 | Add `cosPickers` variable |
| Line ~1820 | `addClipButton`: remove from `cosED`, add to `cosPickers` and `cosDAVremote` |
| Line ~1906 | `addFileButton`: remove from `cosED`, add to `cosPickers` and `cosDAVremote` |
| Line ~2024 | `addFolderButton`: remove from `cosED`, add to `cosPickers` and `cosDAVremote` |
| Lines 826, 948, 1207 | After `allEnabled(false, cosED...)`, add remote picker re-enable |
| `addClipButton` callback | After writing clipboard text, upload via WebDAV if remote |
