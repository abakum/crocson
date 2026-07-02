# Fix Darwin (macOS) Local Peer Discovery

## Problem

When local mode is enabled in settings.go (`force-local` checked), Windows and Android find peers successfully, but Darwin (macOS) does not find peers.

## Analysis of peerdiscovery Library

After examining `/home/koka/src/peerdiscovery/`:

1. **`peerdiscovery.go:166-168` and `listener.go:114-116`**: The library calls `JoinGroup` on all filtered interfaces:
   ```go
   for i := range ifaces {
       p2.JoinGroup(&ifaces[i], &net.UDPAddr{IP: group, Port: portNum})
   }
   ```

2. **`internal.go:73-80`**: Interface filtering logic:
   ```go
   // Interface must be up and either support multicast or be a loopback interface.
   if iface.Flags&net.FlagUp == 0 {
       continue
   }
   if iface.Flags&net.FlagLoopback == 0 && iface.Flags&net.FlagMulticast == 0 {
       continue
   }
   ```

3. **`anet` library**: On Darwin (`//go:build !android`), `anet` just delegates to stdlib `net.Interfaces()` - no special handling.

4. **Platform comparison**:
   - **Android**: Uses `anet` with netlink (Android 11+), AND requires `WifiManager.MulticastLock` to receive multicast packets
   - **Windows**: Uses stdlib `net.Interfaces()`, multicast works without special handling
   - **Darwin**: Uses stdlib `net.Interfaces()`, multicast should work without special handling **in theory**

## Root Cause Analysis

### Issue 1: Darwin-specific `acquireMulticastLock()` is a no-op

Currently, Darwin inherits the no-op `acquireMulticastLock()` from `for_android0.go`:
```go
//go:build !android
func acquireMulticastLock() bool { return true }
```

Unlike Android which needs a MulticastLock to receive multicast, Darwin should theoretically work without it. However, the fact that Windows works and Darwin doesn't suggests there's something platform-specific.

### Issue 2: macOS Local Network Privacy (macOS 11+)

macOS 11+ has Local Network Privacy which may block multicast. However, this typically prompts the user for permission, so if this were the issue, the user would have seen a prompt.

### Issue 3: Virtual/Tunnel Interfaces on macOS

macOS has many virtual interfaces that may have multicast flags but don't actually support multicast:
- `utun*` - VPN tunnel interfaces
- `awdl*` - Apple Wireless Direct Link
- `anpi*` - Apple network interface
- `tun*`, `tap*` - Generic tunnel interfaces
- `gif*`, `stf*` - Generic tunnel interfaces
- `ppp*` - Point-to-point protocol

The `filterInterfaces()` function doesn't exclude these, and `JoinGroup` on a virtual interface may fail silently or cause issues.

### Issue 4: Missing Error Handling in JoinGroup

In `peerdiscovery.go:167`, `JoinGroup` errors are ignored:
```go
p2.JoinGroup(&ifaces[i], &net.UDPAddr{IP: group, Port: portNum})
```

This means if some interfaces fail to join the group, discovery continues with the remaining interfaces. If ALL interfaces fail, discovery silently finds nothing.

### Issue 5: `anet` vs `net.Interfaces()` on Darwin

While Android needed `anet` because `net.Interfaces()` is broken on Android 11+, on Darwin, `anet` just delegates to stdlib. However, there might be differences in how Darwin returns interface flags compared to Windows.

## Proposed Solution

### Primary Solution: Add `acquireMulticastLock()` to Existing `for_darwin.go`

Since Darwin already has a dedicated `for_darwin.go` file for platform-specific code, we'll add multicast handling there. The implementation will:

1. **Add `isVirtualInterface()` helper** - Filter out macOS virtual interfaces (utun*, awdl*, etc.)
2. **Add `acquireMulticastLock()`** - Log valid interfaces for debugging, return false if none found
3. **Add `releaseMulticastLock()`** - No-op cleanup function
4. **Update `for_android0.go`** - Change build constraint so Darwin doesn't inherit the no-op

This approach:
- ✅ Keeps Darwin-specific code in one file
- ✅ Doesn't change the peerdiscovery library
- ✅ Provides debug logging to diagnose issues
- ✅ Minimal changes required

### Secondary Solution: Patch `peerdiscovery` Library (Optional)

If primary solution doesn't work, we can patch the `peerdiscovery` library to:
1. Filter virtual interfaces on Darwin in `filterInterfaces()`
2. Add error handling for `JoinGroup` failures
3. Add logging for interface discovery

## Implementation Steps

### Primary Solution: Add to Existing `for_darwin.go`

1. **Add `"net"` import** to existing `for_darwin.go`:
   ```go
   import (
       "fmt"
       "net"           // Add this
       "os/exec"
       "sync/atomic"
       "fyne.io/fyne/v2"
       log "github.com/schollz/logger"
   )
   ```

2. **Add multicast functions** to end of `for_darwin.go`:

   ```go
   // isVirtualInterface checks if interface name indicates a virtual/tunnel interface on Darwin
   func isVirtualInterface(name string) bool {
       virtualPrefixes := []string{"utun", "tun", "tap", "gif", "stf", "ppp", "awdl", "anpi"}
       for _, prefix := range virtualPrefixes {
           if len(name) >= len(prefix) && name[:len(prefix)] == prefix {
               return true
           }
       }
       return false
   }

   // acquireMulticastLock for Darwin logs interface info for debugging
   // Unlike Android, macOS doesn't require a lock for multicast, but we add
   // debugging to help diagnose issues with peerdiscovery
   func acquireMulticastLock() bool {
       interfaces, err := net.Interfaces()
       if err != nil {
           log.Errorf("Failed to get interfaces: %v", err)
           return false
       }

       log.Debugf("Darwin acquireMulticastLock: checking %d interfaces", len(interfaces))
       validIfaces := 0
       for _, iface := range interfaces {
           if iface.Flags&net.FlagUp == 0 {
               log.Debugf("  Interface %s: DOWN, skipping", iface.Name)
               continue
           }
           if iface.Flags&net.FlagLoopback != 0 {
               log.Debugf("  Interface %s: LOOPBACK, skipping", iface.Name)
               continue
           }
           if iface.Flags&net.FlagMulticast == 0 {
               log.Debugf("  Interface %s: NO MULTICAST FLAG, skipping", iface.Name)
               continue
           }
           if isVirtualInterface(iface.Name) {
               log.Debugf("  Interface %s: VIRTUAL, skipping", iface.Name)
               continue
           }
           log.Debugf("  Interface %s: VALID (flags: %v)", iface.Name, iface.Flags)
           validIfaces++
       }

       if validIfaces == 0 {
           log.Warn("No valid multicast interfaces found on Darwin")
           return false
       }

       log.Debugf("Found %d valid multicast interfaces on Darwin", validIfaces)
       return true
   }

   // releaseMulticastLock is a no-op on Darwin
   func releaseMulticastLock() bool {
       log.Debug("Release multicast lock on Darwin (no-op)")
       return true
   }
   ```

3. **Modify `for_android0.go`** build constraint:
   - Change from `//go:build !android` to `//go:build !android && !darwin`
   - This ensures Darwin uses its own implementation instead of the no-op

4. **Build and test**:
   ```bash
   GOOS=darwin GOARCH=amd64 go build -o crocson-darwin-amd64
   # Or use Makefile if available
   make darwin
   ```

5. **Test on macOS**:
   - Enable debug logging in settings (set debug level to debug/trace)
   - Enable `force-local` in settings
   - Start sender on macOS and receiver on Windows/Android (or vice versa)
   - Check logs for interface discovery messages
   - Verify peers are discovered

### Secondary Solution: Patch `peerdiscovery` Library (If Primary Fails)

If primary solution doesn't work, patch `/home/koka/src/peerdiscovery/internal.go`:

1. **Add `import "runtime"`** at top
2. **Add helper function**:
   ```go
   // isVirtualInterfaceDarwin checks if interface name indicates a virtual/tunnel interface on Darwin
   func isVirtualInterfaceDarwin(name string) bool {
       virtualPrefixes := []string{"utun", "tun", "tap", "gif", "stf", "ppp", "awdl", "anpi"}
       for _, prefix := range virtualPrefixes {
           if len(name) >= len(prefix) && name[:len(prefix)] == prefix {
               return true
           }
       }
       return false
   }
   ```
3. **Modify `filterInterfaces()`** to add Darwin-specific filtering:
   ```go
   // Add after checking flags:
   if runtime.GOOS == "darwin" && isVirtualInterfaceDarwin(iface.Name) {
       continue
   }
   ```

1. **Modify `/home/koka/src/peerdiscovery/internal.go`**:
   - Add `import "runtime"` at top
   - Add `isVirtualInterfaceDarwin()` helper function
   - Modify `filterInterfaces()` to skip virtual interfaces on Darwin

2. **Add error handling** in `/home/koka/src/peerdiscovery/peerdiscovery.go`:
   ```go
   for i := range ifaces {
       if err := p2.JoinGroup(&ifaces[i], &net.UDPAddr{IP: group, Port: portNum}); err != nil {
           log.Printf("Failed to join multicast group on %s: %v", ifaces[i].Name, err)
       }
   }
   ```

3. **Add error handling** in `/home/koka/src/peerdiscovery/listener.go` similarly

4. **Test on macOS** with other platforms (Windows/Android)

## Testing

- Enable `force-local` in settings on macOS
- Start sender on macOS and receiver on Windows/Android (or vice versa)
- Check logs for interface discovery and multicast join messages
- Verify peers are discovered
- Test with different network configurations (Wi-Fi, Ethernet, with/without VPN)

## Files to Modify

### Option 1 (Add to existing for_darwin.go - Recommended):

1. **Modify**: `for_darwin.go`
   - Add `"net"` to imports
   - Add `isVirtualInterface()` function
   - Add `acquireMulticastLock()` with logging
   - Add `releaseMulticastLock()` as no-op

2. **Modify**: `for_android0.go`
   - Change build constraint from `//go:build !android` to `//go:build !android && !darwin`

### Option 2 (Patch peerdiscovery for additional filtering):

1. **Modify**: `/home/koka/src/peerdiscovery/internal.go`
   - Add `import "runtime"` at top
   - Add `isVirtualInterfaceDarwin()` helper function
   - Modify `filterInterfaces()` to skip virtual interfaces on Darwin

2. **Modify**: `/home/koka/src/peerdiscovery/peerdiscovery.go`
   - Add error handling for `JoinGroup`

3. **Modify**: `/home/koka/src/peerdiscovery/listener.go`
   - Add error handling for `JoinGroup`

## Notes

- The `peerdiscovery` library is already forked locally at `/home/koka/src/peerdiscovery`
- Default multicast address is `239.255.255.250` (set in `main.go`)
- The crocson repo already uses local forks via `make local` in Makefile
- Android's MulticastLock approach is different from Darwin's - Android needs it to receive multicast, Darwin should work without it

## Questions for User

1. **Testing environment**: Do you have access to a macOS machine for testing?
2. **VPN status**: Is the macOS machine using a VPN when testing? (Virtual interfaces from VPN may interfere)
3. **Firewall**: Is macOS Firewall enabled? Does it block incoming connections?
4. **Local Network Privacy**: Has the macOS app been granted Local Network permission? (System Settings → Privacy & Security → Local Network)
5. **Network topology**: Are macOS and other devices on the same local network / same subnet?