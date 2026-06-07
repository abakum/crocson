//go:build !android

package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
)

var lifecycleFromJava chan string

func uriBase(uri fyne.URI) string {
	return uri.Name()
}

func IsFilePickerSupported() (bool, error) { return !noDialogs, nil }
func IsSaveDialogSupported() (bool, error) { return !noDialogs, nil }

func IsFolderPickerSupported() (bool, error) { return !noDialogs, nil }

// func IsFolderPickerSupported() (bool, error) { return false, nil }

func CanList(u fyne.URI) (bool, error) {
	return storage.CanList(u)
}

func IsDirectory(u fyne.URI) (ok bool) {
	if u == nil {
		return
	}
	switch u.Scheme() {
	case "file", "":
		if fi, _ := os.Stat(u.Path()); fi == nil {
			return
		} else {
			return fi.IsDir()
		}
	}
	ok, _ = storage.CanList(u)
	return
}

func MimeType(u fyne.URI) string { return u.MimeType() }
func apiLevel() int              { return 29 }

func sendNotification(a fyne.App, title, content string) {
	// Стандартное уведомление для других платформ
	notification := fyne.NewNotification(title, content)
	a.SendNotification(notification)
}

func LogD(string) {}

func setModTime(uri fyne.URI, mtime time.Time) {
	if uri == nil {
		return
	}
	os.Chtimes(uri.Path(), time.Time{}, mtime)
}

func setModTimeMediaStore(uri fyne.URI, mtime time.Time) {}

func ChildViaMediaStore(parent fyne.URI, component string) (child fyne.URI, cleanup func(), err error) {
	cleanup = func() {}
	err = fmt.Errorf("not supported")
	return
}

func createViaMediaStoreFromFileURI(safURI fyne.URI) (fyne.URI, error) {
	return nil, fmt.Errorf("not supported")
}

func isMediaStorePath(uri fyne.URI) bool {
	return false
}

// getSize возвращает размер файла в байтах
func getSize(uri fyne.URI) (size int64, err error) {
	if uri == nil {
		return 0, ErrNilURI
	}
	switch uri.Scheme() {
	case "file", "":
		if fi, err := os.Stat(uri.Path()); err != nil {
			return 0, err
		} else {
			return fi.Size(), nil
		}
	}
	return 0, fmt.Errorf("uri is not file")
}

func start() {
	cmd := exec.Command(os.Args[0])
	cmd.Env = os.Environ()
	cmd.Dir = wd
	cmd.Start()
}

func Reader(u fyne.URI) (r fyne.URIReadCloser, err error) {
	if u == nil {
		return nil, ErrNilURI
	}
	switch u.Scheme() {
	case "file", "":
		return reader(u)
	}

	ok, err := storage.CanRead(u)
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, fmt.Errorf("uri not readable")
	}
	return storage.Reader(u)
}

var (
	ErrNilReader = errors.New("reader is nil")
	ErrNilFile   = errors.New("file is nil")
)

type fileReader struct {
	uri  fyne.URI
	file *os.File
}

func reader(uri fyne.URI) (fyne.URIReadCloser, error) {
	if uri == nil {
		return nil, ErrNilURI
	}

	path := uri.Path()
	if path == "" {
		return nil, os.ErrInvalid
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	return &fileReader{
		uri:  uri,
		file: file,
	}, nil
}

// Read читает данные из файла
func (r *fileReader) Read(p []byte) (n int, err error) {
	if r == nil {
		return 0, ErrNilReader
	}
	if r.file == nil {
		return 0, ErrNilFile
	}
	if len(p) == 0 {
		return 0, nil
	}

	return r.file.Read(p)
}

// Close закрывает файл
func (r *fileReader) Close() error {
	if r == nil {
		return ErrNilReader
	}
	if r.file == nil {
		return nil // уже закрыт
	}

	err := r.file.Close()
	r.file = nil // помечаем как закрытый
	return err
}

// URI возвращает URI файла
func (r *fileReader) URI() fyne.URI {
	if r == nil {
		return nil
	}
	return r.uri
}

func List(u fyne.URI) (c []fyne.URI, err error) {
	if u == nil {
		err = fmt.Errorf("uri is nul")
		return
	}
	switch u.Scheme() {
	case "file", "":
		return list(u)

	}
	return storage.List(u)
}

func list(u fyne.URI) ([]fyne.URI, error) {
	path := u.Path()

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var uris []fyne.URI
	for _, entry := range entries {
		childPath := filepath.Join(path, entry.Name())
		childURI := storage.NewFileURI(childPath)
		uris = append(uris, childURI)
	}

	return uris, nil
}

func ModTime(uri fyne.URI) (time.Time, error) {
	if uri == nil {
		return time.Time{}, ErrNilURI
	}
	return fileModTime(uri.Path())
}

// Если зарегистрированы схемы то через них
// иначе регистрируем для юникс
// иначе монтируем для дарвина и виндовс
// иначе через браузер
func OpenDAV(s string) error {
	u, err := url.Parse(s)
	if err != nil {
		return err
	}

	if schemes, _, _, ok := isDAV(s); ok {
		// Если зарегистрированы схемы то через них
		for _, scheme := range schemes[1:] {
			if registered(scheme) {
				u.Scheme = scheme
				return fyne.CurrentApp().OpenURL(u)
			}
		}
		switch runtime.GOOS {
		case "darwin":
			// иначе монтируем
			u.Scheme = schemes[0]
			script := fmt.Sprintf(`tell app "Finder" to mount volume "%s"`, u)
			err := exec.Command("osascript", "-e", script).Run()
			if err == nil {
				cleanups = append(cleanups, func() {
					exec.Command("diskutil", "eject", u.Hostname()).Start()
				})
				return nil
			}
			return err
		case "unix":
			// иначе регистрируем для юникс
			if err := registerScheme(u.Scheme); err == nil {
				return fyne.CurrentApp().OpenURL(u)
			}
		case "windows":
			// иначе монтируем
			u.Scheme = schemes[0]
			err := netUse(u, false)
			if err == nil {
				cleanups = append(cleanups, func() {
					netUse(u, true)
				})
				return nil
			}
		}
		u.Scheme = schemes[0]
	}

	return fyne.CurrentApp().OpenURL(u)

}

func OpenURL(s string) error {
	u, err := url.Parse(s)
	if err != nil {
		return err
	}
	return fyne.CurrentApp().OpenURL(u)
}
