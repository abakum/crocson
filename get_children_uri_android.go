//go:build android

package main

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
	log "github.com/schollz/logger"
)

func list(uri fyne.URI) (children []fyne.URI, err error) {
	if uri == nil {
		return nil, fmt.Errorf("uri is nil")
	}

	childrenStr, err := callStringString("getChildrenURIs", uri.String())
	if err != nil {
		return nil, fmt.Errorf("getChildrenURIs failed: %w", err)
	}

	// Если строка пустая, возвращаем пустой слайс
	if childrenStr == "" {
		return []fyne.URI{}, nil
	}

	// Разбиваем строку по разделителю | на слайс строк
	uriStrs := strings.Split(childrenStr, "|")
	children = make([]fyne.URI, 0, len(uriStrs))

	// Конвертируем каждую строку в fyne.URI
	for _, uriStr := range uriStrs {
		if uriStr != "" {
			childURI := storage.NewURI(uriStr)
			children = append(children, childURI)
		}
	}

	log.Debugf("getChildrenURIs: parsed %d children URIs for parent URI: %s", len(children), uri.String())

	return children, nil
}

func List(u fyne.URI) (c []fyne.URI, err error) {
	if u == nil {
		err = fmt.Errorf("uri is nul")
		return
	}
	if u.Scheme() == "content" {
		return list(u)
	}
	return storage.List(u)
}
