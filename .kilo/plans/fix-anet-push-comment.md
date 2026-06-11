# Fix: push new commit to anet fork so Go proxy picks it up

## Problem
`go list -m github.com/abakum/anet@main` resolves to commit `5501d40` (before our fix). The Go module proxy has cached the old state. A new commit will force a new pseudo-version.

## Steps

1. Add a comment to `interface_android.go` in `/home/koka/src/anet` explaining why removing `go:linkname` is safe for crocson — something like:
   ```
   // NOTE: go:linkname directives for net.zoneCache and golang.org/x/net/internal/socket.zoneCache
   // were removed for Go 1.25 compatibility (see -checklinkname). The zone cache sync calls
   // were also removed. This does not affect crocson because it does not rely on IPv6 zone
   // resolution from anet — croc and WebRTC use IP addresses directly.
   ```

2. Commit and push:
   ```
   cd /home/koka/src/anet
   git add -A && git commit -m "doc: explain go:linkname removal safety for Go 1.25+"
   git push origin main
   ```
