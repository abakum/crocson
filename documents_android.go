//go:build android

package main

import (
	log "github.com/schollz/logger"
)

func IsIntentSupported(action, mimeType string) (bool, error) {
	if noDialogs {
		return false, nil
	}

	result, err := callBooleanString2("isIntentSupported", action, mimeType)
	if err != nil {
		log.Error("Error in IsIntentSupported: ", err.Error())
		return false, err
	}

	return result, nil
}

// IsFilePickerSupported проверяет поддержку диалога выбора файлов
func IsFilePickerSupported() (bool, error) {
	supported, err := IsIntentSupported("android.intent.action.GET_CONTENT", "*/*")
	if err != nil {
		log.Error("File picker support check failed: ", err.Error())
	}
	return supported, err
}

// IsSaveDialogSupported проверяет поддержку диалога сохранения файлов
func IsSaveDialogSupported() (bool, error) {
	supported, err := IsIntentSupported("android.intent.action.CREATE_DOCUMENT", "*/*")
	if err != nil {
		log.Error("Save dialog support check failed: ", err.Error())
	}
	return supported, err
}

// IsFolderPickerSupported проверяет поддержку диалога выбора папки
func IsFolderPickerSupported() (bool, error) {
	supported, err := IsIntentSupported("android.intent.action.OPEN_DOCUMENT_TREE", "")
	if err != nil {
		log.Error("Folder picker support check failed: ", err.Error())
	}
	return supported, err
}
