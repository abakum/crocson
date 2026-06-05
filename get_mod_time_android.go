//go:build android

package main

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
)

func getModTime(uri fyne.URI) (modTime int64, err error) {
	if uri == nil {
		return 0, fmt.Errorf("uri is nil")
	}

	modTime, err = callLongString("getModTime", uri.String())
	if err != nil {
		return 0, fmt.Errorf("failed to get modification time: %w", err)
	}
	return modTime, nil
}

func ModTime(uri fyne.URI) (time.Time, error) {
	if uri == nil {
		return time.Time{}, fmt.Errorf("uri is nil")
	}

	// Для обычных файлов используем стандартный подход
	if uri.Scheme() != "content" {
		return fileModTime(uri.Path())
	}

	modTimeMs, err := getModTime(uri)
	if err != nil {
		return time.Time{}, err
	}

	if modTimeMs <= 0 {
		return time.Time{}, fmt.Errorf("modification time not available")
	}

	return time.UnixMilli(modTimeMs), nil
}
