# Plan: Runtime auto-install .desktop entry and icon on Linux

## Problem
When running `crocson` from `~/go/bin/` (via `go install`), the window has no icon on Debian/X11 because no `.desktop` file or icon is installed in system directories. The WM looks up icons via `.desktop` → `Icon=` → freedesktop icon theme spec.

## Solution
At startup on Linux, check if `.desktop` entry and icon exist in standard locations. If not — install them locally to `~/.local/share/`. Everything runs in-process using already embedded data.

## Files to create/modify

### 1. New: `install_desktop_linux.go` (`//go:build linux && !android`)

```go
//go:build linux && !android

package main

import (
    "fmt"
    "os"
    "os/exec"
    "path/filepath"

    "github.com/adrg/xdg"
    log "github.com/schollz/logger"
)

func ensureDesktopEntry() {
    appID := "com.github.abakum.crocson"

    // 1. Check if .desktop exists in standard locations
    //    xdg.DataDirs = [/usr/local/share, /usr/share, ~/.local/share]
    desktopRel := filepath.Join("applications", appID+".desktop")
    if _, err := xdg.SearchDataFile(desktopRel); err == nil {
        log.Debugf("found %s", desktopRel)
        return
    }

    // 2. Get actual binary path
    exe, err := os.Executable()
    if err != nil {
        log.Errorf("os.Executable: %v", err)
        return
    }
    exe, err = filepath.EvalSymlinks(exe)
    if err != nil {
        log.Errorf("EvalSymlinks: %v", err)
        return
    }

    // 3. Write .desktop to ~/.local/share/applications/
    desktopDir := filepath.Join(xdg.DataHome, "applications")
    desktopPath := filepath.Join(desktopDir, appID+".desktop")

    // 4. Write icon to ~/.local/share/pixmaps/
    pixmapsDir := filepath.Join(xdg.DataHome, "pixmaps")
    iconPath := filepath.Join(pixmapsDir, appID+".png")

    // Write icon from embedded data
    if _, err := os.Stat(iconPath); err != nil {
        if err := os.MkdirAll(pixmapsDir, 0755); err != nil { ... }
        if err := os.WriteFile(iconPath, iconData, 0644); err != nil { ... }
        log.Infof("installed icon to %s", iconPath)
    }

    // Write .desktop
    if err := os.MkdirAll(desktopDir, 0755); err != nil { ... }
    desktop := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=crocson
GenericName=GUI for croc
Exec=%s
Icon=%s
Comment=A simple GUI for croc
Categories=Network;FileTransfer;
`, exe, appID)
    if err := os.WriteFile(desktopPath, []byte(desktop), 0644); err != nil { ... }
    log.Infof("installed %s", desktopPath)

    // 5. Update desktop database (best-effort)
    if p, err := exec.LookPath("update-desktop-database"); err == nil {
        cmd := exec.Command(p, desktopDir)
        cmd.Run()
    }
}
```

### 2. New: `install_desktop_other.go` (`//go:build !linux || android`)

```go
//go:build !linux || android

package main

func ensureDesktopEntry() {}
```

### 3. Modify: `main.go`

Add call in `main()` before `w.ShowAndRun()`, in the `default:` case (desktop path):

```go
    case "linux":
        ensureDesktopEntry()  // <-- add here, before DISPLAY check
        // ... existing code
    fallthrough
    default:
```

Or better — call it after window creation but before show, in the `default:` branch, after `setOut(GUI)`:

```go
    default:
        setOut(GUI)
    }

    ensureDesktopEntry()  // no-op on non-linux
```

## Lookup order (xdg.SearchDataFile)
1. `$XDG_DATA_HOME/applications/` (default `~/.local/share/applications/`)
2. `$XDG_DATA_DIRS/applications/` (default `/usr/local/share/applications/`, `/usr/share/applications/`)

## Icon lookup (WM behavior)
WM resolves `Icon=com.github.abakum.crocson` via freedesktop Icon Theme Specification:
1. `~/.local/share/icons/<theme>/...` (recursive)
2. `~/.local/share/pixmaps/com.github.abakum.crocson.png`
3. `/usr/share/pixmaps/com.github.abakum.crocson.png`

We install to `pixmaps` — simplest, theme-independent location.

## Edge cases
- **Symlink**: `filepath.EvalSymlinks(exe)` resolves `~/go/bin/crocson` → actual binary
- **Already installed**: `xdg.SearchDataFile` returns nil → skip, no writes
- **Icon exists, .desktop missing**: only writes .desktop
- **Permission denied**: log error, continue — app works without icon
- **Concurrency**: not an issue — single-process, early in main()
- **update-desktop-database missing**: best-effort `Run()`, ignore error

## Testing
```bash
# 1. Remove existing
rm -f ~/.local/share/applications/com.github.abakum.crocson.desktop
rm -f ~/.local/share/pixmaps/com.github.abakum.crocson.png

# 2. Run from source
go install && crocson

# 3. Verify files created
ls -la ~/.local/share/applications/com.github.abakum.crocson.desktop
ls -la ~/.local/share/pixmaps/com.github.abakum.crocson.png

# 4. Verify Exec= points to actual binary
grep Exec ~/.local/share/applications/com.github.abakum.crocson.desktop

# 5. Second run — no re-install (check logs for "found" message)
crocson
```
