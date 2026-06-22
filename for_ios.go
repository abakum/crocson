//go:build ios

package main

/*
#include <stdlib.h>
#include "for_ios.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
	"fyne.io/fyne/v2/storage"
	log "github.com/schollz/logger"
)

func caffeinate(i int32) int32 {
	old := atomic.LoadInt32(&sleepCounter)
	var newVal int32

	if i == 0 {
		atomic.StoreInt32(&sleepCounter, 0)
		newVal = 0
	} else {
		newVal = atomic.AddInt32(&sleepCounter, i)
	}

	// Управляем флагом через очередь Fyne для потокобезопасности
	fyne.Do(func() {
		if old <= 0 && newVal > 0 {
			// Включаем блокировку сна
			C.setIdleTimerDisabled(C.YES)
		} else if old > 0 && newVal <= 0 {
			// Выключаем блокировку сна
			C.setIdleTimerDisabled(C.NO)
		}
	})

	return newVal
}

func SleepAllowed() bool {
	return atomic.LoadInt32(&sleepCounter) <= 0
}

// CreateBookmarkFromURLDownload создает security-scoped bookmark для папки Downloads
func CreateBookmarkFromURLDownload() (string, error) {
	var result string
	var err error

	driver.RunNative(func(ctx interface{}) error {
		cResult := C.CreateBookmarkFromURLDownload()
		if cResult == nil {
			err = errors.New("неизвестная ошибка при создании bookmark")
			return nil
		}

		defer C.free(unsafe.Pointer(cResult))
		resultStr := C.GoString(cResult)

		if strings.HasPrefix(resultStr, "error:") {
			err = errors.New(resultStr)
		} else {
			result = resultStr
		}
		return nil
	})

	if err != nil {
		return "", err
	}
	return result, nil
}

// CreateFileInDownloads создает файл в папке Downloads
func CreateFileInDownloads(fileName, mimeType string) (string, error) {
	log.Debug("Creating file in iOS Downloads: ", fileName)

	var result string
	var err error

	// Получаем bookmark для папки Downloads
	bookmarkData, err := CreateBookmarkFromURLDownload()
	if err != nil {
		return "", fmt.Errorf("failed to get Downloads bookmark: %v", err)
	}

	driver.RunNative(func(ctx interface{}) error {
		cBookmarkData := C.CString(bookmarkData)
		defer C.free(unsafe.Pointer(cBookmarkData))

		cFileName := C.CString(fileName)
		defer C.free(unsafe.Pointer(cFileName))

		cMimeType := C.CString(mimeType)
		defer C.free(unsafe.Pointer(cMimeType))

		cResult := C.CreateFileInDownloads(cBookmarkData, cFileName, cMimeType)
		if cResult == nil {
			err = errors.New("unknown error in native function")
			return nil
		}

		defer C.free(unsafe.Pointer(cResult))
		resultStr := C.GoString(cResult)

		if strings.HasPrefix(resultStr, "error:") {
			err = errors.New(strings.TrimPrefix(resultStr, "error: "))
		} else {
			result = resultStr
		}
		return nil
	})

	if err != nil {
		log.Error("Failed to create file: ", err.Error())
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	if result == "" {
		return "", errors.New("empty result from file creation")
	}

	return result, nil
}

// DownloadDir на iOS неприменим: каталоги сохраняются как .zip.
func DownloadDir() (fyne.URI, error) {
	return nil, fmt.Errorf("DownloadDir not applicable on ios")
}

// ChildDownload создает файл и возвращает его для последующего наполнения данными
func ChildDownload(component string) (child fyne.URI, cleanup func(), err error) {
	cleanup = func() {}

	// Создаем файл в папке Downloads
	newFileBookmark, err := CreateFileInDownloads(component, "")
	if err != nil {
		err = fmt.Errorf("CreateFileInDownloads failed: %v", err)
		return
	}

	// Разрешаем bookmark нового файла
	resolvedURL, isStale, err := ResolveBookmarkToURL(newFileBookmark)
	if err != nil {
		err = fmt.Errorf("resolveBookmarkToURL failed: %v", err)
		return
	}

	if isStale {
		StopAccessingSecurityScopedResource(resolvedURL)
		err = fmt.Errorf("bookmark is stale for %s", resolvedURL)
		return
	}

	// Конвертируем в fyne.URI
	child, err = storage.ParseURI(resolvedURL)
	if err != nil {
		StopAccessingSecurityScopedResource(resolvedURL)
		err = fmt.Errorf("parse URI failed: %v", err)
		return
	}

	cleanup = func() {
		StopAccessingSecurityScopedResource(resolvedURL)
	}

	return
}

// CreateBookmarkFromURL создает security-scoped bookmark из URL
func CreateBookmarkFromURL(url string) (string, error) {
	var result string
	var err error

	driver.RunNative(func(ctx interface{}) error {
		cUrl := C.CString(url)
		defer C.free(unsafe.Pointer(cUrl))

		cResult := C.CreateBookmarkFromURL(cUrl)
		if cResult == nil {
			err = errors.New("CreateBookmarkFromURL is nil")
			return nil
		}

		defer C.free(unsafe.Pointer(cResult))
		resultStr := C.GoString(cResult)

		if strings.HasPrefix(resultStr, "error:") {
			err = errors.New(resultStr)
		} else {
			result = resultStr
		}
		return nil
	})

	if err != nil {
		return "", err
	}
	return result, nil
}

// ResolveBookmarkToURL разрешает security-scoped bookmark в URL
func ResolveBookmarkToURL(bookmarkData string) (string, bool, error) {
	var result string
	var isStale bool
	var err error

	driver.RunNative(func(ctx interface{}) error {
		cBookmarkData := C.CString(bookmarkData)
		defer C.free(unsafe.Pointer(cBookmarkData))

		var cIsStale C.bool
		cResult := C.ResolveBookmarkToURL(cBookmarkData, &cIsStale)
		if cResult == nil {
			err = errors.New("bookmark is nil")
			return nil
		}

		defer C.free(unsafe.Pointer(cResult))
		resultStr := C.GoString(cResult)
		isStale = bool(cIsStale)

		if strings.HasPrefix(resultStr, "error:") {
			err = errors.New(resultStr)
		} else {
			result = resultStr
		}
		return nil
	})

	if err != nil {
		return "", false, err
	}
	return result, isStale, nil
}

// StopAccessingSecurityScopedResource останавливает security-scoped доступ
func StopAccessingSecurityScopedResource(url string) {
	driver.RunNative(func(ctx interface{}) error {
		cUrl := C.CString(url)
		defer C.free(unsafe.Pointer(cUrl))

		C.StopAccessingSecurityScopedResource(cUrl)
		return nil
	})
}

// CreateFileInTree создает файл в указанной через bookmark директории на iOS
func CreateFileInTree(bookmarkData, fileName, mimeType string) (string, error) {
	var result string
	var err error

	if mimeType == "" {
		mimeType = detectMimeType(fileName)
	}

	driver.RunNative(func(ctx interface{}) error {
		cBookmarkData := C.CString(bookmarkData)
		defer C.free(unsafe.Pointer(cBookmarkData))
		cFileName := C.CString(fileName)
		defer C.free(unsafe.Pointer(cFileName))
		cMimeType := C.CString(mimeType)
		defer C.free(unsafe.Pointer(cMimeType))

		cResult := C.CreateFileInTreeIOS(cBookmarkData, cFileName, cMimeType)
		if cResult == nil {
			err = errors.New("createFileInTreeIOS is nil")
			return nil
		}

		defer C.free(unsafe.Pointer(cResult))
		resultStr := C.GoString(cResult)

		if strings.HasPrefix(resultStr, "error:") {
			err = errors.New(resultStr)
		} else {
			result = resultStr
		}
		return nil
	})

	if err != nil {
		return "", err
	}
	if result == "" {
		return "", errors.New("result is empty")
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

	// 2. Проверяем, что parent является директорией
	canList, err := storage.CanList(parent)
	if err != nil {
		err = fmt.Errorf("cannot check if listable: %v", err)
		return
	}
	if !canList {
		err = fmt.Errorf("URI is not a directory: %s", parent.String())
		return
	}

	// 3. iOS-specific логика
	bookmarkData, err := CreateBookmarkFromURL(parent.String())
	if err != nil {
		err = fmt.Errorf("createBookmarkFromURL failed: %v", err)
		return
	}

	// 4. Создаём component в parent
	newFileURL, err := CreateFileInTree(bookmarkData, component, "")
	if err != nil {
		err = fmt.Errorf("CreateFileInTree failed: %v", err)
		return
	}

	// 5. Создаем security-scoped доступ
	newFileBookmark, err := CreateBookmarkFromURL(newFileURL)
	if err != nil {
		err = fmt.Errorf("create bookmark for new file failed: %v", err)
		return
	}

	// 6. Конвертируем в URL
	resolvedURL, isStale, err := ResolveBookmarkToURL(newFileBookmark)
	if err != nil {
		err = fmt.Errorf("resolveBookmarkToURL failed: %v", err)
		return
	}

	if isStale {
		StopAccessingSecurityScopedResource(resolvedURL)
		err = fmt.Errorf("bookmark is stale for %s", resolvedURL)
		return
	}

	// 7. Конвертируем в fyne.URI
	child, err = storage.ParseURI(resolvedURL)
	if err != nil {
		StopAccessingSecurityScopedResource(resolvedURL)
		err = fmt.Errorf("parse URI failed: %v", err)
		return
	}

	cleanup = func() {
		StopAccessingSecurityScopedResource(resolvedURL)
	}

	return
}
