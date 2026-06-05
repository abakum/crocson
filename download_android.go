//go:build android

package main

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
	log "github.com/schollz/logger"
)

// CreateFileInDownloads создает файл в папке Downloads с поддержкой всех версий Android
func CreateFileInDownloads(fileName, mimeType string) (string, error) {
	log.Debug("Creating file in Downloads: ", fileName)

	if mimeType == "" {
		mimeType = detectMimeType(fileName)
	}

	result, err := callStringString2("createFileInDownloads", fileName, mimeType)
	if err != nil {
		log.Error("Failed to create file: ", err.Error())
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	if result == "" {
		return "", fmt.Errorf("empty result from createFileInDownloads")
	}

	return result, nil
}

// ChildDownload создает файл и возвращает его для последующего наполнения данными
func ChildDownload(component string) (child fyne.URI, cleanup func(), err error) {
	cleanup = func() {}

	// Создаем файл и получаем только URI
	uri, err := CreateFileInDownloads(component, "")
	if err != nil {
		err = fmt.Errorf("createFileInDownloads failed: %v", err)
		return
	}

	child, err = storage.ParseURI(uri)
	if err != nil {
		err = fmt.Errorf("parse URI failed: %v", err)
		return
	}

	return
}
