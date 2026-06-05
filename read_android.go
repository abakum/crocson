//go:build android

package main

import (
	"fmt"

	log "github.com/schollz/logger"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
)

func Reader(u fyne.URI) (r fyne.URIReadCloser, err error) {
	if u == nil {
		err = fmt.Errorf("uri is nul")
		return
	}
	if !canRead(u) {
		err = fmt.Errorf("uri not readable")
		return
	}
	return storage.Reader(u)
}

func canRead(uri fyne.URI) bool {
	if uri == nil {
		return false
	}
	switch MimeType(uri) {
	case MIME_TYPE_DIR:
		return false
	case MIME_TYPE_OCTET_STREAM:
	}
	ok, err := storage.CanRead(uri)
	if err != nil {
		log.Errorf("canRead: %v", err)
		return false
	}
	return ok
}
