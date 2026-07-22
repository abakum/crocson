//go:build android

package main

/*
#include <stdlib.h>
#include "for_android.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
	"fyne.io/fyne/v2/storage"

	log "github.com/schollz/logger"
)

var errJNI = errors.New("JNI call failed")

var (
	fgStopMu    sync.Mutex
	fgStopTimer *time.Timer
)

func callVoid(method string) error {
	return driver.RunNative(func(ctx interface{}) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok {
			return errJNI
		}
		cMethod := C.CString(method)
		defer C.free(unsafe.Pointer(cMethod))
		if C.callVoid((*C.JNIEnv)(unsafe.Pointer(ac.Env)), (C.jobject)(unsafe.Pointer(ac.Ctx)), cMethod) != 0 {
			return fmt.Errorf("callVoid(%s): %w", method, errJNI)
		}
		return nil
	})
}

func callVoidString(method, arg string) error {
	return driver.RunNative(func(ctx interface{}) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok {
			return errJNI
		}
		cMethod := C.CString(method)
		cArg := C.CString(arg)
		defer C.free(unsafe.Pointer(cMethod))
		defer C.free(unsafe.Pointer(cArg))
		if C.callVoidString((*C.JNIEnv)(unsafe.Pointer(ac.Env)), (C.jobject)(unsafe.Pointer(ac.Ctx)), cMethod, cArg) != 0 {
			return fmt.Errorf("callVoidString(%s): %w", method, errJNI)
		}
		return nil
	})
}

func callInt(method string) (int, error) {
	var result int
	err := driver.RunNative(func(ctx interface{}) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok {
			return errJNI
		}
		cMethod := C.CString(method)
		defer C.free(unsafe.Pointer(cMethod))
		r := C.callInt((*C.JNIEnv)(unsafe.Pointer(ac.Env)), (C.jobject)(unsafe.Pointer(ac.Ctx)), cMethod)
		if r < 0 {
			return fmt.Errorf("callInt(%s): %w", method, errJNI)
		}
		result = int(r)
		return nil
	})
	return result, err
}

func callBoolean(method string) (bool, error) {
	var result bool
	err := driver.RunNative(func(ctx interface{}) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok {
			return errJNI
		}
		cMethod := C.CString(method)
		defer C.free(unsafe.Pointer(cMethod))
		r := C.callBoolean((*C.JNIEnv)(unsafe.Pointer(ac.Env)), (C.jobject)(unsafe.Pointer(ac.Ctx)), cMethod)
		if r < 0 {
			return fmt.Errorf("callBoolean(%s): %w", method, errJNI)
		}
		result = r > 0
		return nil
	})
	return result, err
}

func callStringString(method, arg string) (string, error) {
	var result string
	err := driver.RunNative(func(ctx interface{}) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok {
			return errJNI
		}
		cMethod := C.CString(method)
		cArg := C.CString(arg)
		defer C.free(unsafe.Pointer(cMethod))
		defer C.free(unsafe.Pointer(cArg))
		r := C.callStringString((*C.JNIEnv)(unsafe.Pointer(ac.Env)), (C.jobject)(unsafe.Pointer(ac.Ctx)), cMethod, cArg)
		if r == nil {
			return fmt.Errorf("callStringString(%s): %w", method, errJNI)
		}
		defer C.free(unsafe.Pointer(r))
		result = C.GoString(r)
		return nil
	})
	return result, err
}

func callBooleanString(method, arg string) (bool, error) {
	var result bool
	err := driver.RunNative(func(ctx interface{}) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok {
			return errJNI
		}
		cMethod := C.CString(method)
		cArg := C.CString(arg)
		defer C.free(unsafe.Pointer(cMethod))
		defer C.free(unsafe.Pointer(cArg))
		r := C.callBooleanString((*C.JNIEnv)(unsafe.Pointer(ac.Env)), (C.jobject)(unsafe.Pointer(ac.Ctx)), cMethod, cArg)
		if r < 0 {
			return fmt.Errorf("callBooleanString(%s): %w", method, errJNI)
		}
		result = r > 0
		return nil
	})
	return result, err
}

func callStringString2(method, arg1, arg2 string) (string, error) {
	var result string
	err := driver.RunNative(func(ctx interface{}) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok {
			return errJNI
		}
		cMethod := C.CString(method)
		cArg1 := C.CString(arg1)
		cArg2 := C.CString(arg2)
		defer C.free(unsafe.Pointer(cMethod))
		defer C.free(unsafe.Pointer(cArg1))
		defer C.free(unsafe.Pointer(cArg2))
		r := C.callStringString2((*C.JNIEnv)(unsafe.Pointer(ac.Env)), (C.jobject)(unsafe.Pointer(ac.Ctx)), cMethod, cArg1, cArg2)
		if r == nil {
			return fmt.Errorf("callStringString2(%s): %w", method, errJNI)
		}
		defer C.free(unsafe.Pointer(r))
		result = C.GoString(r)
		return nil
	})
	return result, err
}

func callLongString(method, arg string) (int64, error) {
	var result int64
	err := driver.RunNative(func(ctx interface{}) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok {
			return errJNI
		}
		cMethod := C.CString(method)
		cArg := C.CString(arg)
		defer C.free(unsafe.Pointer(cMethod))
		defer C.free(unsafe.Pointer(cArg))
		r := C.callLongString((*C.JNIEnv)(unsafe.Pointer(ac.Env)), (C.jobject)(unsafe.Pointer(ac.Ctx)), cMethod, cArg)
		if r < 0 {
			return fmt.Errorf("callLongString(%s): %w", method, errJNI)
		}
		result = int64(r)
		return nil
	})
	return result, err
}

func callIntString(method, arg string) (int, error) {
	var result int
	err := driver.RunNative(func(ctx interface{}) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok {
			return errJNI
		}
		cMethod := C.CString(method)
		cArg := C.CString(arg)
		defer C.free(unsafe.Pointer(cMethod))
		defer C.free(unsafe.Pointer(cArg))
		r := C.callIntString((*C.JNIEnv)(unsafe.Pointer(ac.Env)), (C.jobject)(unsafe.Pointer(ac.Ctx)), cMethod, cArg)
		if r < 0 {
			return fmt.Errorf("callIntString(%s): %w", method, errJNI)
		}
		result = int(r)
		return nil
	})
	return result, err
}

func callStringStringString(method, arg1, arg2, arg3 string) (string, error) {
	var result string
	err := driver.RunNative(func(ctx interface{}) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok {
			return errJNI
		}
		cMethod := C.CString(method)
		cArg1 := C.CString(arg1)
		cArg2 := C.CString(arg2)
		cArg3 := C.CString(arg3)
		defer C.free(unsafe.Pointer(cMethod))
		defer C.free(unsafe.Pointer(cArg1))
		defer C.free(unsafe.Pointer(cArg2))
		defer C.free(unsafe.Pointer(cArg3))
		r := C.callStringStringString((*C.JNIEnv)(unsafe.Pointer(ac.Env)), (C.jobject)(unsafe.Pointer(ac.Ctx)), cMethod, cArg1, cArg2, cArg3)
		if r == nil {
			return fmt.Errorf("callStringStringString(%s): %w", method, errJNI)
		}
		defer C.free(unsafe.Pointer(r))
		result = C.GoString(r)
		return nil
	})
	return result, err
}

func callBooleanString2(method, arg1, arg2 string) (bool, error) {
	var result bool
	err := driver.RunNative(func(ctx interface{}) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok {
			return errJNI
		}
		cMethod := C.CString(method)
		cArg1 := C.CString(arg1)
		cArg2 := C.CString(arg2)
		defer C.free(unsafe.Pointer(cMethod))
		defer C.free(unsafe.Pointer(cArg1))
		defer C.free(unsafe.Pointer(cArg2))
		r := C.callBooleanString2((*C.JNIEnv)(unsafe.Pointer(ac.Env)), (C.jobject)(unsafe.Pointer(ac.Ctx)), cMethod, cArg1, cArg2)
		if r < 0 {
			return fmt.Errorf("callBooleanString2(%s): %w", method, errJNI)
		}
		result = r > 0
		return nil
	})
	return result, err
}

func callVoidInt(method string, arg int32) error {
	return driver.RunNative(func(ctx interface{}) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok {
			return errJNI
		}
		cMethod := C.CString(method)
		defer C.free(unsafe.Pointer(cMethod))
		if C.callVoidInt((*C.JNIEnv)(unsafe.Pointer(ac.Env)), (C.jobject)(unsafe.Pointer(ac.Ctx)), cMethod, C.jint(arg)) != 0 {
			return fmt.Errorf("callVoidInt(%s): %w", method, errJNI)
		}
		return nil
	})
}

func setAppThemeMode(mode int32) {
	if err := callVoidInt("setAppThemeMode", mode); err != nil {
		log.Errorf("setAppThemeMode: %v", err)
	}
}

func callBooleanStringLong(method string, strArg string, longArg int64) (bool, error) {
	var result bool
	err := driver.RunNative(func(ctx interface{}) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok {
			return errJNI
		}
		cMethod := C.CString(method)
		cArg := C.CString(strArg)
		defer C.free(unsafe.Pointer(cMethod))
		defer C.free(unsafe.Pointer(cArg))
		r := C.callBooleanStringLong((*C.JNIEnv)(unsafe.Pointer(ac.Env)), (C.jobject)(unsafe.Pointer(ac.Ctx)), cMethod, cArg, C.jlong(longArg))
		if r < 0 {
			return fmt.Errorf("callBooleanStringLong(%s): %w", method, errJNI)
		}
		result = r > 0
		return nil
	})
	return result, err
}

func setModTimeUsingFD(uriStr string, modTimeMs int64) bool {
	var result bool
	driver.RunNative(func(ctx interface{}) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok {
			return nil
		}
		cUriStr := C.CString(uriStr)
		defer C.free(unsafe.Pointer(cUriStr))
		r := C.setModTimeUsingFD((*C.JNIEnv)(unsafe.Pointer(ac.Env)), (C.jobject)(unsafe.Pointer(ac.Ctx)), cUriStr, C.jlong(modTimeMs))
		result = r > 0
		return nil
	})
	return result
}

// --- callers ---

func setModTime(uri fyne.URI, mtime time.Time) {
	if uri == nil {
		return
	}
	if uri.Scheme() == "file" {
		log.Debugf("Chtimes %s %v: %v", uri.Path(), mtime,
			os.Chtimes(uri.Path(), time.Time{}, mtime))
		return
	}
	if uri.Scheme() != "content" {
		return
	}
	modTimeMs := mtime.UnixMilli()
	ok := setModTimeUsingFD(uri.String(), modTimeMs)
	log.Debugf("setModTime FD %s %v: %v", uri, mtime, ok)
}

func callStringString4(method, arg1, arg2, arg3, arg4 string) (string, error) {
	var result string
	err := driver.RunNative(func(ctx interface{}) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok {
			return errJNI
		}
		cMethod := C.CString(method)
		cArg1 := C.CString(arg1)
		cArg2 := C.CString(arg2)
		cArg3 := C.CString(arg3)
		cArg4 := C.CString(arg4)
		defer C.free(unsafe.Pointer(cMethod))
		defer C.free(unsafe.Pointer(cArg1))
		defer C.free(unsafe.Pointer(cArg2))
		defer C.free(unsafe.Pointer(cArg3))
		defer C.free(unsafe.Pointer(cArg4))
		r := C.callStringString4((*C.JNIEnv)(unsafe.Pointer(ac.Env)), (C.jobject)(unsafe.Pointer(ac.Ctx)), cMethod, cArg1, cArg2, cArg3, cArg4)
		if r == nil {
			return fmt.Errorf("callStringString4(%s): %w", method, errJNI)
		}
		defer C.free(unsafe.Pointer(r))
		result = C.GoString(r)
		return nil
	})
	return result, err
}

func isMediaStorePath(uri fyne.URI) bool {
	if uri == nil || uri.Scheme() != "content" {
		return false
	}
	uriStr := uri.String()
	if strings.Contains(uriStr, "content://media/") {
		return true
	}
	decoded, err := url.QueryUnescape(uriStr)
	if err != nil {
		decoded = uriStr
	}
	path := ""
	switch {
	case strings.Contains(decoded, "primary:"):
		idx := strings.Index(decoded, "primary:")
		path = decoded[idx+len("primary:"):]
	case strings.Contains(decoded, "primary%3A"):
		idx := strings.Index(decoded, "primary%3A")
		path = decoded[idx+len("primary%3A"):]
		path, _ = url.QueryUnescape(path)
	case strings.Contains(decoded, "raw:"):
		idx := strings.Index(decoded, "raw:")
		path = decoded[idx+len("raw:"):]
	case strings.Contains(decoded, "raw%3A"):
		idx := strings.Index(decoded, "raw%3A")
		path = decoded[idx+len("raw%3A"):]
		path, _ = url.QueryUnescape(path)
	default:
		return false
	}
	path = strings.TrimPrefix(path, "/storage/emulated/0/")
	return strings.HasPrefix(path, "Download") ||
		strings.HasPrefix(path, "Pictures") ||
		strings.HasPrefix(path, "DCIM") ||
		strings.HasPrefix(path, "Movies") ||
		strings.HasPrefix(path, "Music") ||
		strings.HasPrefix(path, "Alarms") ||
		strings.HasPrefix(path, "Podcasts") ||
		strings.HasPrefix(path, "Ringtones")
}

func resolveMediaStoreURI(uriStr string) string {
	result, err := callStringString("resolveMediaStoreUri", uriStr)
	if err != nil || result == "" {
		return ""
	}
	return result
}

func setModTimeMediaStore(uri fyne.URI, mtime time.Time) {
	if uri == nil || uri.Scheme() != "content" || !isMediaStorePath(uri) {
		return
	}
	if apiLevel() < 29 {
		return
	}
	uriStr := uri.String()
	if strings.Contains(uriStr, "content://media/") {
		return
	}
	mediaUri := resolveMediaStoreURI(uriStr)
	if mediaUri == "" {
		return
	}
	modTimeMs := mtime.UnixMilli()
	ok := setModTimeUsingFD(mediaUri, modTimeMs)
	log.Debugf("setModTimeMediaStore %s → %s %v: %v", uriStr, mediaUri, mtime, ok)
}

func parseMediaStoreInfo(uriStr string) (collectionType, relativePath string) {
	decoded, err := url.QueryUnescape(uriStr)
	if err != nil {
		decoded = uriStr
	}
	path := ""
	switch {
	case strings.Contains(decoded, "primary:"):
		idx := strings.Index(decoded, "primary:")
		path = decoded[idx+len("primary:"):]
	case strings.Contains(decoded, "primary%3A"):
		idx := strings.Index(decoded, "primary%3A")
		path = decoded[idx+len("primary%3A"):]
		path, _ = url.QueryUnescape(path)
	case strings.Contains(decoded, "raw:"):
		idx := strings.Index(decoded, "raw:")
		path = decoded[idx+len("raw:"):]
	case strings.Contains(decoded, "raw%3A"):
		idx := strings.Index(decoded, "raw%3A")
		path = decoded[idx+len("raw%3A"):]
		path, _ = url.QueryUnescape(path)
	default:
		return "downloads", ""
	}
	path = strings.TrimPrefix(path, "/storage/emulated/0/")

	switch {
	case strings.HasPrefix(path, "Download"):
		collectionType = "downloads"
		relativePath = "Download"
	case strings.HasPrefix(path, "Pictures"):
		collectionType = "images"
		relativePath = "Pictures"
	case strings.HasPrefix(path, "DCIM"):
		collectionType = "images"
		relativePath = "DCIM"
	case strings.HasPrefix(path, "Movies"):
		collectionType = "video"
		relativePath = "Movies"
	case strings.HasPrefix(path, "Music"):
		collectionType = "audio"
		relativePath = "Music"
	default:
		return "downloads", ""
	}

	sub := strings.TrimPrefix(path, relativePath)
	sub = strings.TrimPrefix(sub, "/")
	if sub != "" {
		if lastSlash := strings.LastIndex(sub, "/"); lastSlash >= 0 {
			relativePath += "/" + sub[:lastSlash]
		} else {
			relativePath += "/" + sub
		}
	}
	return
}

func ChildViaMediaStore(parent fyne.URI, component string) (child fyne.URI, cleanup func(), err error) {
	cleanup = func() {}
	if apiLevel() < 29 {
		err = fmt.Errorf("MediaStore not available on API < 29")
		return
	}
	if !isMediaStorePath(parent) {
		err = fmt.Errorf("not a MediaStore path")
		return
	}

	collectionType, relativePath := parseMediaStoreInfo(parent.String())
	mimeType := detectMimeType(component)

	result, err := callStringString4("createFileViaMediaStore", collectionType, relativePath, component, mimeType)
	if err != nil || result == "" {
		err = fmt.Errorf("createFileViaMediaStore failed: %v", err)
		return
	}

	child, err = storage.ParseURI(result)
	if err != nil {
		err = fmt.Errorf("parse URI failed: %v", err)
		return
	}

	return
}

func createViaMediaStoreFromFileURI(safURI fyne.URI) (fyne.URI, error) {
	if !isMediaStorePath(safURI) {
		return nil, fmt.Errorf("not a MediaStore path")
	}
	uriStr := safURI.String()

	decoded, err := url.QueryUnescape(uriStr)
	if err != nil {
		decoded = uriStr
	}
	path := ""
	switch {
	case strings.Contains(decoded, "primary:"):
		idx := strings.Index(decoded, "primary:")
		path = decoded[idx+len("primary:"):]
	case strings.Contains(decoded, "primary%3A"):
		idx := strings.Index(decoded, "primary%3A")
		path = decoded[idx+len("primary%3A"):]
		path, _ = url.QueryUnescape(path)
	default:
		return nil, fmt.Errorf("cannot extract path from %s", uriStr)
	}
	path = strings.TrimPrefix(path, "/storage/emulated/0/")

	fileName := filepath.Base(path)
	collectionType, relativePath := parseMediaStoreInfo(uriStr)
	mimeType := detectMimeType(fileName)

	result, err := callStringString4("createFileViaMediaStore", collectionType, relativePath, fileName, mimeType)
	if err != nil || result == "" {
		return nil, fmt.Errorf("createFileViaMediaStore failed: %v", err)
	}

	return storage.ParseURI(result)
}

func apiLevel() int {
	n, _ := callInt("getApiLevel")
	return n
}

func caffeinate(i int32) int32 {
	old := atomic.LoadInt32(&sleepCounter)
	var newVal int32

	if i == 0 {
		atomic.StoreInt32(&sleepCounter, 0)
		newVal = 0
	} else {
		newVal = atomic.AddInt32(&sleepCounter, i)
	}

	if old <= 0 && newVal > 0 {
		fgStopMu.Lock()
		if fgStopTimer != nil {
			fgStopTimer.Stop()
			fgStopTimer = nil
		}
		fgStopMu.Unlock()
		startForegroundService()
	} else if old > 0 && newVal <= 0 {
		fgStopMu.Lock()
		if fgStopTimer != nil {
			fgStopTimer.Stop()
		}
		if i == 0 {
			fgStopTimer = nil
			fgStopMu.Unlock()
			stopForegroundService()
		} else {
			fgStopTimer = time.AfterFunc(fgStopDelay, func() {
				stopForegroundService()
			})
			fgStopMu.Unlock()
		}
	}

	return newVal
}

const fgStopDelay = 3 * time.Second

func SleepAllowed() bool {
	return atomic.LoadInt32(&sleepCounter) <= 0
}

func startForegroundService() {
	if err := callVoid("startCrocsonService"); err != nil {
		log.Errorf("Foreground service start failed: %v", err)
		return
	}
	log.Debugf("Foreground service started")
}

func stopForegroundService() {
	if err := callVoid("stopCrocsonService"); err != nil {
		log.Errorf("Foreground service stop failed: %v", err)
		return
	}
	log.Debugf("Foreground service stopped")
}

// acquireMulticastLock holds a WifiManager.MulticastLock so that the wifi
// driver delivers inbound multicast datagrams to the app. Required for croc's
// local peer discovery (peerdiscovery) on Android. No-op on other platforms.
// Returns true on success; logs failures on the Go side.
func acquireMulticastLock() bool {
	ok, err := callBoolean("acquireMulticastLock")
	if err != nil {
		log.Errorf("acquireMulticastLock JNI failed: %v", err)
		return false
	}
	if !ok {
		log.Warn("acquireMulticastLock failed (WifiManager unavailable or security exception)")
		return false
	}
	log.Debug("MulticastLock acquired")
	return true
}

// releaseMulticastLock releases the previously acquired MulticastLock.
// Returns true on success; logs failures on the Go side.
func releaseMulticastLock() bool {
	ok, err := callBoolean("releaseMulticastLock")
	if err != nil {
		log.Errorf("releaseMulticastLock JNI failed: %v", err)
		return false
	}
	if !ok {
		log.Warn("releaseMulticastLock failed")
		return false
	}
	log.Debug("MulticastLock released")
	return true
}

func MimeType(uri fyne.URI) string {
	if uri == nil {
		return ""
	}
	s, _ := callStringString("getMimeType", uri.String())
	return s
}

func OpenURL(intentStr string) error {
	ok, err := callBooleanString("openIntent", intentStr)
	if err != nil {
		return fmt.Errorf("intent failed: %s: %w", intentStr, err)
	}
	if !ok {
		return fmt.Errorf("intent failed: %s", intentStr)
	}
	return nil
}

func OpenDAV(s string) error {
	u, err := url.Parse(s)
	if err != nil {
		return err
	}

	if schemes, _, _, ok := isDAV(s); ok {
		u.Scheme = schemes[0]
	}

	return OpenURL(u.String())
}

// ResolveIntent возвращает описание того, кто обработал бы intent-URI
// (default-компонент + кандидаты), не запуская activity.
func ResolveIntent(intentStr string) (string, error) {
	return callStringString("resolveIntent", intentStr)
}

func CanList(u fyne.URI) (bool, error) {
	if u == nil {
		return false, nil
	}

	if apiLevel() > 28 {
		return storage.CanList(u)
	}

	return callBooleanString("canListDirectory", u.String())
}

func uriBase(uri fyne.URI) string {
	name, err := callStringString("getFileName", uri.String())
	if err == nil && name != "" {
		return sanitizeFileName(name)
	}
	return base(uri.Path())
}

func uriPath(uri fyne.URI) string {
	path := uri.Path()
	decoded, err := url.PathUnescape(path)
	if err != nil {
		return path
	}
	return decoded
}

func base(path string) string {
	decoded, err := url.PathUnescape(path)
	if err != nil {
		decoded = strings.ReplaceAll(path, "%2F", "/")
		decoded = strings.ReplaceAll(decoded, "%3A", "/")
	}
	decoded = strings.ReplaceAll(decoded, ":", "/")

	lastSlash := strings.LastIndex(decoded, "/")
	if lastSlash < 0 {
		return replace(decoded)
	}

	return replace(decoded[lastSlash+1:])
}

func replace(s string) string {
	return strings.NewReplacer(
		"?", "_",
		":", "_",
	).Replace(s)
}

type Toast struct {
	message string
}

const (
	ToastShort        = 3 * time.Second
	ToastLong         = 4 * time.Second
	DefaultPadding    = 10.0
	AnimationDuration = 300 * time.Millisecond
)

func NewToast(_ fyne.Window, message string) *Toast {
	return &Toast{message: message}
}

func (t *Toast) SetIcon(_ fyne.Resource) *Toast    { return t }
func (t *Toast) SetText(message string) *Toast     { t.message = message; return t }
func (t *Toast) SetTimeout(_ time.Duration) *Toast { return t }
func (t *Toast) SetPadding(_ float32) *Toast       { return t }
func (t *Toast) SetAnimation(_ bool) *Toast        { return t }
func (t *Toast) Short() *Toast                     { return t }
func (t *Toast) Long() *Toast                      { return t }
func (t *Toast) Hide()                             {}

func (t *Toast) Show() {
	callVoidString("showToast", t.message)
}

// --- count children ---

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

// --- get size ---

func getSize(uri fyne.URI) (size int64, err error) {
	if uri == nil {
		return 0, fmt.Errorf("uri is nil")
	}

	size, err = callLongString("getSize", uri.String())
	if err != nil {
		return 0, fmt.Errorf("failed to get size: %w", err)
	}
	return size, nil
}

// --- get mod time ---

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

// --- get children uri ---

func list(uri fyne.URI) (children []fyne.URI, err error) {
	if uri == nil {
		return nil, fmt.Errorf("uri is nil")
	}

	childrenStr, err := callStringString("getChildrenURIs", uri.String())
	if err != nil {
		return nil, fmt.Errorf("getChildrenURIs failed: %w", err)
	}

	if childrenStr == "" {
		return []fyne.URI{}, nil
	}

	uriStrs := strings.Split(childrenStr, "|")
	children = make([]fyne.URI, 0, len(uriStrs))

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

// --- documents ---

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

func IsFilePickerSupported() (bool, error) {
	supported, err := IsIntentSupported("android.intent.action.GET_CONTENT", "*/*")
	if err != nil {
		log.Error("File picker support check failed: ", err.Error())
	}
	return supported, err
}

func IsSaveDialogSupported() (bool, error) {
	supported, err := IsIntentSupported("android.intent.action.CREATE_DOCUMENT", "*/*")
	if err != nil {
		log.Error("Save dialog support check failed: ", err.Error())
	}
	return supported, err
}

func IsFolderPickerSupported() (bool, error) {
	supported, err := IsIntentSupported("android.intent.action.OPEN_DOCUMENT_TREE", "")
	if err != nil {
		log.Error("Folder picker support check failed: ", err.Error())
	}
	return supported, err
}

// --- child ---

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

	child, err = storage.Child(parent, component)
	if err == nil {
		return
	}

	newFileURL, err := CreateFileInTree(parent.String(), component, "")
	if err != nil {
		err = fmt.Errorf("CreateFileInTree failed: %v", err)
		return
	}

	child, err = storage.ParseURI(newFileURL)
	if err != nil {
		err = fmt.Errorf("parse URI failed: %v", err)
		return
	}

	return
}

// DownloadDir на Android неприменим: каталоги сохраняются как .zip через
// createFileInDownloads, поэтому здесь возвращается ошибка.
func DownloadDir() (fyne.URI, error) {
	return nil, fmt.Errorf("DownloadDir not applicable on android")
}

// --- download ---

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
		return "", ErrPermissionPending
	}

	return result, nil
}

func AddPendingSave(src, dest string, lu fyne.ListableURI, fe *fyne.Container, w fyne.Window) {
	ps := &PendingSave{
		Src:  src,
		Dest: dest,
		LU:   lu,
		FE:   fe,
		W:    w,
	}
	pendingSaves.Store(src, ps)
	log.Debug("Added pending save: ", src)
}

func ChildDownload(component string) (child fyne.URI, cleanup func(), err error) {
	cleanup = func() {}

	uri, err := CreateFileInDownloads(component, "")
	if err != nil {
		err = fmt.Errorf("createFileInDownloads failed: %w", err)
		return
	}

	child, err = storage.ParseURI(uri)
	if err != nil {
		err = fmt.Errorf("parse URI failed: %v", err)
		return
	}

	return
}

// --- reader ---

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

// --- log ---

func LogD(message string) {
	cmessage := C.CString(message)
	defer C.free(unsafe.Pointer(cmessage))

	C.LogD(cmessage)
}

// --- lifecycle ---

var lifecycleFromJava = make(chan string, 10)

//export lifecycleEventNotify
func lifecycleEventNotify(event *C.char) {
	goEvent := C.GoString(event)
	LogD("lifecycle: " + goEvent)
	select {
	case lifecycleFromJava <- goEvent:
	default:
		LogD("lifecycle: channel full, dropping " + goEvent)
	}
}

// --- intent ---

//export intentURINotify
func intentURINotify(uri *C.char) {
	if uri != nil {
		goURI := C.GoString(uri)
		LogD("intent: URI " + goURI)
		select {
		case uriFromIntent <- goURI:
		default:
			LogD("intent: URI channel full, dropping")
		}
	}
}

//export intentTextNotify
func intentTextNotify(text *C.char) {
	if text != nil {
		goText := C.GoString(text)
		LogD("intent: text received")
		select {
		case textFromIntent <- goText:
		default:
			LogD("intent: text channel full, dropping")
		}
	}
}

// One NV21 preview frame from the built-in QR scanner (Java -> Go).
// Returns true to keep streaming, false to stop feeding frames.
//
//export cameraFrameNotify
func cameraFrameNotify(data *C.char, length, w, h C.int) bool {
	if data == nil || length <= 0 || w <= 0 || h <= 0 {
		return true // ignore bad frame, keep camera going
	}
	b := C.GoBytes(unsafe.Pointer(data), length)
	return cameraFrameReceived(b, int(w), int(h))
}
