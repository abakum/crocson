# Fix: upload URL scheme and path separator

## Bug

From logs:
```
mkcol dav://127.0.0.1:8080mp3: parse "dav://127.0.0.1:8080mp3": invalid port ":8080mp3" after host
```

Two issues:
1. `link.URL` has scheme `dav` — HTTP requests need `http`/`https`
2. `path.Join(targetURL.Path, ...)` when `targetURL.Path` is `""` produces `"mp3"` without leading `/`, so URL becomes `dav://host:portmp3`

## Fix

### 1. In `send.go` — normalize URL before passing to `uploadToWebDAV`

Use existing `isDAV()` helper (applinks.go:237) which converts `dav`→`http`, `davs`→`https`.

**Desktop** (`SetOnDropped`, ~line 1700):
```go
// вместо:
uploadToWebDAV(p, link.URL, ...)
// сделать:
_, _, proxyURL, _ := isDAV(link.URL.String())
if proxyURL == nil { continue }
uploadToWebDAV(p, proxyURL, ...)
```

**Android** (`uriFromIntent`, ~line 1495):
```go
// аналогично:
_, _, proxyURL, _ := isDAV(link.URL.String())
if proxyURL == nil { continue }
uploadToWebDAV(u.Path(), proxyURL, ...)
```

### 2. In `webdavclient.go` — fix path separator

In `uploadFileToWebDAV` and `uploadDirToWebDAV`, ensure path always starts with `/`:

Replace:
```go
path.Join(targetURL.Path, url.PathEscape(base))
```
With:
```go
path.Join("/", targetURL.Path, url.PathEscape(base))
```

Same for all other `path.Join(targetURL.Path, ...)` calls in these functions.

No new helpers needed — `isDAV` already exists.
