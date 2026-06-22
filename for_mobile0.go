//go:build !android && !ios

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
	"github.com/adrg/xdg"
	log "github.com/schollz/logger"
)

func Child(parent fyne.URI, component string) (child fyne.URI, cleanup func(), err error) {
	cleanup = func() {}
	child, err = storage.Child(parent, component)
	return
}

func DownloadDir() (fyne.URI, error) {
	downloads := xdg.UserDirs.Download
	if downloads == "" {
		return nil, fmt.Errorf("failed to get Downloads directory")
	}
	return storage.NewFileURI(downloads), nil
}

func ChildDownload(component string) (child fyne.URI, cleanup func(), err error) {
	cleanup = func() {}

	downloads := xdg.UserDirs.Download
	if downloads == "" {
		err = fmt.Errorf("failed to get Downloads directory")
		return
	}

	u := storage.NewFileURI(downloads)

	dirPath := filepath.Dir(component)

	// Проверяем, есть ли реальные поддиректории
	hasSubdirs := dirPath != "." && dirPath != string(filepath.Separator)

	// Создаем полный путь к файлу
	if hasSubdirs {
		dirToCreate := filepath.Join(downloads, dirPath)
		err = os.MkdirAll(dirToCreate, 0755)
		if err != nil {
			err = fmt.Errorf("failed to create directory %s: %v", dirToCreate, err)
			return
		}
		u = storage.NewFileURI(dirToCreate)
	}
	lu, err := storage.ListerForURI(u)
	if err != nil {
		err = fmt.Errorf("create lister for %s: %v", u, err)
		return
	}

	child, err = storage.Child(lu, filepath.Base(component))
	return
}

// fixRecordingFile запускает ffmpeg для ремукса записанного файла:
// WebM — дописывает Cues + Duration, MP4 — перемещает moov в начало.
// Вызывается в горутине, ошибки только логируются.
func fixRecordingFile(root, fileName string) {
	srcPath := filepath.Join(root, fileName)
	ext := strings.ToLower(filepath.Ext(fileName))

	// Проверяем наличие ffmpeg
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		log.Debugf("fixRecordingFile: ffmpeg not found, skip fix for %s", fileName)
		return
	}

	fixedPath := srcPath + ".fixed" + ext
	defer os.Remove(fixedPath) // очистка временного файла

	args := []string{ffmpegPath, "-y", "-i", srcPath, "-c", "copy"}
	if ext == ".mp4" {
		args = append(args, "-movflags", "+faststart")
	}
	args = append(args, fixedPath)

	cmd := exec.Command(args[0], args[1:]...)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Debugf("fixRecordingFile: ffmpeg failed for %s: %v (%s)", fileName, err, string(out))
		return
	}

	if err := os.Rename(fixedPath, srcPath); err != nil {
		log.Debugf("fixRecordingFile: rename failed for %s: %v", fileName, err)
		return
	}

	log.Debugf("fixRecordingFile: fixed %s", fileName)
}
