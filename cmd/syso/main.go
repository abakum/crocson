package main

import (
	"fmt"
	"image"
	_ "image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	ico "github.com/Kodeworks/golang-image-ico"
	"github.com/BurntSushi/toml"
	"github.com/josephspurrier/goversioninfo"
)

type FyneApp struct {
	Website string `toml:"Website"`
	Details struct {
		Icon    string `toml:"Icon"`
		Name    string `toml:"Name"`
		ID      string `toml:"ID"`
		Version string `toml:"Version"`
		Build   int    `toml:"Build"`
	} `toml:"Details"`
	Source struct {
		Repo string `toml:"Repo"`
	} `toml:"Source"`
	LinuxAndBSD struct {
		Comment string `toml:"Comment"`
	} `toml:"LinuxAndBSD"`
}

func main() {
	dir := "."
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}

	tomlPath := filepath.Join(dir, "FyneApp.toml")
	archs := []string{"amd64"}
	if len(os.Args) > 2 {
		archs = os.Args[2:]
	}

	var app FyneApp
	if _, err := toml.DecodeFile(tomlPath, &app); err != nil {
		fmt.Fprintf(os.Stderr, "parse %s: %v\n", tomlPath, err)
		os.Exit(1)
	}

	iconPath := filepath.Join(dir, app.Details.Icon)

	needGenerate := false
	tomlStat, err := os.Stat(tomlPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "stat %s: %v\n", tomlPath, err)
		os.Exit(1)
	}
	iconStat, iconErr := os.Stat(iconPath)
	if iconErr != nil {
		fmt.Fprintf(os.Stderr, "stat %s: %v\n", iconPath, iconErr)
		os.Exit(1)
	}
	for _, arch := range archs {
		out := filepath.Join(dir, fmt.Sprintf("resource_windows_%s.syso", arch))
		sysoStat, err := os.Stat(out)
		if err != nil {
			needGenerate = true
			break
		}
		if sysoStat.ModTime().Before(tomlStat.ModTime()) || sysoStat.ModTime().Before(iconStat.ModTime()) {
			needGenerate = true
			break
		}
	}
	if !needGenerate {
		return
	}

	parts := strings.SplitN(app.Details.Version, ".", 4)
	major, _ := strconv.Atoi(or(parts, 0, "0"))
	minor, _ := strconv.Atoi(or(parts, 1, "0"))
	patch, _ := strconv.Atoi(or(parts, 2, "0"))
	build := app.Details.Build
	if b, _ := strconv.Atoi(or(parts, 3, "0")); b != 0 {
		build = b
	}

	icoPath, err := pngToIco(iconPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ico: %v\n", err)
		os.Exit(1)
	}
	defer os.Remove(icoPath)

	vi := &goversioninfo.VersionInfo{}
	vi.IconPath = icoPath
	vi.FixedFileInfo.FileVersion.Major = major
	vi.FixedFileInfo.FileVersion.Minor = minor
	vi.FixedFileInfo.FileVersion.Patch = patch
	vi.FixedFileInfo.FileVersion.Build = build
	vi.FixedFileInfo.ProductVersion.Major = major
	vi.FixedFileInfo.ProductVersion.Minor = minor
	vi.FixedFileInfo.ProductVersion.Patch = patch
	vi.FixedFileInfo.ProductVersion.Build = build
	vi.StringFileInfo.ProductName = app.Details.Name
	vi.StringFileInfo.FileDescription = app.LinuxAndBSD.Comment
	vi.StringFileInfo.FileVersion = fmt.Sprintf("%d.%d.%d.%d", major, minor, patch, build)
	vi.StringFileInfo.ProductVersion = app.Details.Version
	vi.StringFileInfo.InternalName = app.Details.Name
	vi.StringFileInfo.OriginalFilename = app.Details.Name + ".exe"
	vi.StringFileInfo.CompanyName = app.Details.ID
	vi.StringFileInfo.LegalCopyright = app.Source.Repo

	vi.Build()
	vi.Walk()

	for _, arch := range archs {
		out := filepath.Join(dir, fmt.Sprintf("resource_windows_%s.syso", arch))
		if err := vi.WriteSyso(out, arch); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", out, err)
			os.Exit(1)
		}
		fmt.Println("wrote", out)
	}
}

func pngToIco(pngPath string) (string, error) {
	f, err := os.Open(pngPath)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", pngPath, err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return "", fmt.Errorf("decode %s: %w", pngPath, err)
	}

	tmp, err := os.CreateTemp("", "icon-*.ico")
	if err != nil {
		return "", err
	}

	if err := ico.Encode(tmp, img); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", fmt.Errorf("encode ico: %w", err)
	}
	tmp.Close()
	return tmp.Name(), nil
}

func or(ss []string, i int, def string) string {
	if i < len(ss) && ss[i] != "" {
		return ss[i]
	}
	return def
}
