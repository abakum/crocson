//go:build linux && !android

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/adrg/xdg"
	log "github.com/schollz/logger"
)

func ensureDesktopEntry() {
	appID := ID

	desktopRel := filepath.Join("applications", appID+".desktop")
	if _, err := xdg.SearchDataFile(desktopRel); err == nil {
		log.Debugf("found %s", desktopRel)
		return
	}

	var data FyneApp
	if _, err := toml.Decode(fyneApp, &data); err != nil {
		log.Errorf("decode FyneApp.toml: %v", err)
		return
	}

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

	pixmapsDir := filepath.Join(xdg.DataHome, "pixmaps")
	iconPath := filepath.Join(pixmapsDir, appID+".png")
	if _, err := os.Stat(iconPath); err != nil {
		if err := os.MkdirAll(pixmapsDir, 0755); err != nil {
			log.Errorf("MkdirAll %s: %v", pixmapsDir, err)
			return
		}
		if err := os.WriteFile(iconPath, iconData, 0644); err != nil {
			log.Errorf("WriteFile %s: %v", iconPath, err)
			return
		}
		log.Infof("installed icon %s", iconPath)
	}

	name := data.Details.Name
	if name == "" {
		name = appID
	}

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
		desktop += "\n[X-Fyne Source]\n" +
			"Repo=" + data.Source.Repo + "\n" +
			"Dir=" + data.Source.Dir + "\n"
	}

	desktopDir := filepath.Join(xdg.DataHome, "applications")
	desktopPath := filepath.Join(desktopDir, appID+".desktop")
	if err := os.MkdirAll(desktopDir, 0755); err != nil {
		log.Errorf("MkdirAll %s: %v", desktopDir, err)
		return
	}
	if err := os.WriteFile(desktopPath, []byte(desktop), 0644); err != nil {
		log.Errorf("WriteFile %s: %v", desktopPath, err)
		return
	}
	log.Infof("installed %s", desktopPath)

	if p, err := exec.LookPath("update-desktop-database"); err == nil {
		_ = exec.Command(p, desktopDir).Run()
	}
}
