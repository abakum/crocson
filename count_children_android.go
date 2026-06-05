//go:build android

package main

import (
	"fmt"
	"os"

	"fyne.io/fyne/v2"
	log "github.com/schollz/logger"
)

func countChild(uri fyne.URI) (count int, err error) {
	if uri == nil {
		return 0, fmt.Errorf("uri is nil")
	}

	count, err = callIntString("countChildren", uri.String())
	if err != nil {
		return 0, fmt.Errorf("countChildren failed: %w", err)
	}

	log.Debugf("countChildren: successfully counted %d children for URI: %s", count, uri.String())
	return count, nil
}

func IsDirectory(uri fyne.URI) bool {
	if uri == nil {
		return false
	}
	if uri.Scheme() == "file" {
		if fi, err := os.Stat(uri.Path()); err == nil {
			return fi.IsDir()
		}
	}
	switch MimeType(uri) {
	case MIME_TYPE_DIR:
		return true
	case MIME_TYPE_OCTET_STREAM:
		fallthrough
	case "":
		_, err := countChild(uri)
		return err == nil
	default:
		return false
	}
}
