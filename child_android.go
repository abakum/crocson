//go:build android

package main

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
)

func CreateFileInTree(treeUri, fileName, mimeType string) (string, error) {
	if mimeType == "" {
		mimeType = detectMimeType(fileName)
	}

	result, err := callStringStringString("createFileInTree", treeUri, fileName, mimeType)
	if err != nil {
		return "", fmt.Errorf("createFileInTree failed: %w", err)
	}
	if result == "" {
		return "", fmt.Errorf("empty result from createFileInTree")
	}
	if strings.HasPrefix(result, "error:") {
		return "", fmt.Errorf("createFileInTree: %s", result)
	}

	return result, nil
}

func Child(parent fyne.URI, component string) (child fyne.URI, cleanup func(), err error) {
	cleanup = func() {}

	// 1. Пробуем стандартный способ
	child, err = storage.Child(parent, component)
	if err == nil {
		return
	}

	// 2. Создаём component в parent
	newFileURL, err := CreateFileInTree(parent.String(), component, "")
	if err != nil {
		err = fmt.Errorf("CreateFileInTree failed: %v", err)
		return
	}

	// 3. Конвертируем в fyne.URI
	child, err = storage.ParseURI(newFileURL)
	if err != nil {
		err = fmt.Errorf("parse URI failed: %v", err)
		return
	}

	return
}
