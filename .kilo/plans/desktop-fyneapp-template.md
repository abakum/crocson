# Plan: Use FyneApp.toml as template for .desktop generation

## Problem
`install_desktop_linux.go` hardcodes `.desktop` content (Name, GenericName, Comment, Categories). These values are already in `FyneApp.toml` under `[Details]` and `[LinuxAndBSD]`. Fyne's own `fyne package` reads them and uses the same template. We should do the same at runtime.

## What's already available
- `fyneApp` — embed of `FyneApp.toml` (`about.go:28`)
- `FyneApp` / `LinuxAndBSD` structs with `toml` tags (`about.go:166-199`)
- `toml.Decode` already imported in `about.go`
- `iconData` — embedded icon PNG (`main.go:102`)
- Fyne's own `.desktop` template uses fields: `Name`, `Exec`, `Icon`, `GenericName`, `Categories`, `Comment`, `Keywords`, `ExecParams`, `SourceRepo`, `SourceDir`

## Changes

### 0. Modify: `FyneApp.toml`

Add `Keywords` to `[LinuxAndBSD]`:

```toml
[LinuxAndBSD]
GenericName = "GUI for croc"
Categories = ["Network", "FileTransfer", "Telephony", "Security"]
Comment = "A simple GUI for croc"
Keywords = ["croc", "fyne", "p2p", "webdav", "webchat", "gui", "cli", "magic-wormhole", "qr", "media-recording", "screen-sharing"]
```

### 1. Modify: `install_desktop_linux.go`

Parse `fyneApp` (embedded `FyneApp.toml`) and build `.desktop` from it, matching Fyne's template:

```go
func ensureDesktopEntry() {
    appID := ID

    // Check if .desktop already exists
    desktopRel := filepath.Join("applications", appID+".desktop")
    if _, err := xdg.SearchDataFile(desktopRel); err == nil {
        return
    }

    // Parse embedded FyneApp.toml
    var data FyneApp
    if _, err := toml.Decode(fyneApp, &data); err != nil {
        log.Errorf("decode FyneApp.toml: %v", err)
        return
    }

    // Resolve binary path
    exe, err := os.Executable()
    ...
    exe, err = filepath.EvalSymlinks(exe)
    ...

    // Install icon
    pixmapsDir := filepath.Join(xdg.DataHome, "pixmaps")
    iconPath := filepath.Join(pixmapsDir, appID+".png")
    ...same as before...

    // Build .desktop from FyneApp.toml data
    name := data.Details.Name
    if name == "" { name = appID }

    desktop := "[Desktop Entry]\n" +
        "Type=Application\n" +
        "Name=" + name + "\n"

    if data.LinuxAndBSD != nil {
        if data.LinuxAndBSD.GenericName != "" {
            desktop += "GenericName=" + data.LinuxAndBSD.GenericName + "\n"
        }
    }

    desktop += "Exec=" + exe
    if data.LinuxAndBSD != nil && data.LinuxAndBSD.ExecParams != "" {
        desktop += " " + data.LinuxAndBSD.ExecParams
    }
    desktop += "\n"

    desktop += "Icon=" + appID + "\n"

    if data.LinuxAndBSD != nil {
        if data.LinuxAndBSD.Comment != "" {
            desktop += "Comment=" + data.LinuxAndBSD.Comment + "\n"
        }
        if len(data.LinuxAndBSD.Categories) > 0 {
            desktop += "Categories=" + strings.Join(data.LinuxAndBSD.Categories, ";") + ";\n"
        }
        if len(data.LinuxAndBSD.Keywords) > 0 {
            desktop += "Keywords=" + strings.Join(data.LinuxAndBSD.Keywords, ";") + ";\n"
        }
    }

    if data.Source != nil && (data.Source.Repo != "" || data.Source.Dir != "") {
        desktop += "\n[X-Fyne Source]\n"
        desktop += "Repo=" + data.Source.Repo + "\n"
        desktop += "Dir=" + data.Source.Dir + "\n"
    }

    ...write desktopPath, update-desktop-database...
}
```

New imports needed: `"strings"`, `"github.com/BurntSushi/toml"`

### No other files need changes

- `fyneApp` (embed of `FyneApp.toml`) is already a `var` in `about.go` — accessible since same package
- `FyneApp` and `LinuxAndBSD` structs already defined in `about.go`
- `toml` package already a dependency (`go.mod`)
- `install_desktop_other.go` — no changes needed
- `main.go` — no changes needed

### Result: generated .desktop from FyneApp.toml
```ini
[Desktop Entry]
Type=Application
Name=crocson
GenericName=GUI for croc
Exec=/home/koka/go/bin/crocson
Icon=com.github.abakum.crocson
Comment=A simple GUI for croc
Categories=Network;FileTransfer;Telephony;Security;
Keywords=croc;fyne;p2p;webdav;webchat;gui;cli;magic-wormhole;qr;media-recording;screen-sharing;

[X-Fyne Source]
Repo=https://github.com/abakum/crocson
Dir=
```

This matches exactly what `fyne package` generates.
