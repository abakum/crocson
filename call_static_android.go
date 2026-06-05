//go:build android

package main

/*
#include <jni.h>
#include <stdlib.h>
#include <string.h>
#include <android/log.h>

#define LogE(...) __android_log_print(ANDROID_LOG_ERROR, "croc", __VA_ARGS__)

static jint callVoid(JNIEnv* env, jobject context, const char* method) {
    jclass cls = (*env)->GetObjectClass(env, context);
    if (cls == NULL) { LogE("GetObjectClass failed for callVoid(%s)", method); return -1; }
    jmethodID mid = (*env)->GetStaticMethodID(env, cls, method, "()V");
    if (mid == NULL) {
        LogE("static method not found: %s", method);
        (*env)->DeleteLocalRef(env, cls);
        return -1;
    }
    (*env)->CallStaticVoidMethod(env, cls, mid);
    if ((*env)->ExceptionCheck(env)) {
        LogE("exception in %s", method);
        (*env)->ExceptionDescribe(env);
        (*env)->ExceptionClear(env);
        (*env)->DeleteLocalRef(env, cls);
        return -1;
    }
    (*env)->DeleteLocalRef(env, cls);
    return 0;
}

static jint callVoidString(JNIEnv* env, jobject context, const char* method, const char* strArg) {
    jclass cls = (*env)->GetObjectClass(env, context);
    if (cls == NULL) { LogE("GetObjectClass failed for callVoidString(%s)", method); return -1; }
    jmethodID mid = (*env)->GetStaticMethodID(env, cls, method, "(Ljava/lang/String;)V");
    if (mid == NULL) {
        LogE("static method not found: %s", method);
        (*env)->DeleteLocalRef(env, cls);
        return -1;
    }
    jstring jstr = (*env)->NewStringUTF(env, strArg);
    (*env)->CallStaticVoidMethod(env, cls, mid, jstr);
    if ((*env)->ExceptionCheck(env)) {
        LogE("exception in %s", method);
        (*env)->ExceptionDescribe(env);
        (*env)->ExceptionClear(env);
        (*env)->DeleteLocalRef(env, jstr);
        (*env)->DeleteLocalRef(env, cls);
        return -1;
    }
    (*env)->DeleteLocalRef(env, jstr);
    (*env)->DeleteLocalRef(env, cls);
    return 0;
}

static jint callInt(JNIEnv* env, jobject context, const char* method) {
    jclass cls = (*env)->GetObjectClass(env, context);
    if (cls == NULL) { LogE("GetObjectClass failed for callInt(%s)", method); return -1; }
    jmethodID mid = (*env)->GetStaticMethodID(env, cls, method, "()I");
    if (mid == NULL) {
        LogE("static method not found: %s", method);
        (*env)->DeleteLocalRef(env, cls);
        return -1;
    }
    jint result = (*env)->CallStaticIntMethod(env, cls, mid);
    if ((*env)->ExceptionCheck(env)) {
        LogE("exception in %s", method);
        (*env)->ExceptionDescribe(env);
        (*env)->ExceptionClear(env);
        (*env)->DeleteLocalRef(env, cls);
        return -1;
    }
    (*env)->DeleteLocalRef(env, cls);
    return result;
}

static char* callStringString(JNIEnv* env, jobject context, const char* method, const char* strArg) {
    jclass cls = (*env)->GetObjectClass(env, context);
    if (cls == NULL) { LogE("GetObjectClass failed for callStringString(%s)", method); return NULL; }
    jmethodID mid = (*env)->GetStaticMethodID(env, cls, method, "(Ljava/lang/String;)Ljava/lang/String;");
    if (mid == NULL) {
        LogE("static method not found: %s", method);
        (*env)->DeleteLocalRef(env, cls);
        return NULL;
    }
    jstring jarg = (*env)->NewStringUTF(env, strArg);
    jstring jresult = (jstring)(*env)->CallStaticObjectMethod(env, cls, mid, jarg);
    if ((*env)->ExceptionCheck(env)) {
        LogE("exception in %s", method);
        (*env)->ExceptionDescribe(env);
        (*env)->ExceptionClear(env);
        (*env)->DeleteLocalRef(env, jarg);
        (*env)->DeleteLocalRef(env, cls);
        return NULL;
    }
    char* result = NULL;
    if (jresult != NULL) {
        const char* utf = (*env)->GetStringUTFChars(env, jresult, NULL);
        result = strdup(utf);
        (*env)->ReleaseStringUTFChars(env, jresult, utf);
        (*env)->DeleteLocalRef(env, jresult);
    }
    (*env)->DeleteLocalRef(env, jarg);
    (*env)->DeleteLocalRef(env, cls);
    return result;
}

static jint callBooleanString(JNIEnv* env, jobject context, const char* method, const char* strArg) {
    jclass cls = (*env)->GetObjectClass(env, context);
    if (cls == NULL) { LogE("GetObjectClass failed for callBooleanString(%s)", method); return -1; }
    jmethodID mid = (*env)->GetStaticMethodID(env, cls, method, "(Ljava/lang/String;)Z");
    if (mid == NULL) {
        LogE("static method not found: %s", method);
        (*env)->DeleteLocalRef(env, cls);
        return -1;
    }
    jstring jarg = (*env)->NewStringUTF(env, strArg);
    jboolean result = (*env)->CallStaticBooleanMethod(env, cls, mid, jarg);
    if ((*env)->ExceptionCheck(env)) {
        LogE("exception in %s", method);
        (*env)->ExceptionDescribe(env);
        (*env)->ExceptionClear(env);
        (*env)->DeleteLocalRef(env, jarg);
        (*env)->DeleteLocalRef(env, cls);
        return -1;
    }
    (*env)->DeleteLocalRef(env, jarg);
    (*env)->DeleteLocalRef(env, cls);
    return result ? 1 : 0;
}

static jint callVoidInt(JNIEnv* env, jobject context, const char* method, jint intArg) {
    jclass cls = (*env)->GetObjectClass(env, context);
    if (cls == NULL) { LogE("GetObjectClass failed for callVoidInt(%s)", method); return -1; }
    jmethodID mid = (*env)->GetStaticMethodID(env, cls, method, "(I)V");
    if (mid == NULL) {
        LogE("static method not found: %s", method);
        (*env)->DeleteLocalRef(env, cls);
        return -1;
    }
    (*env)->CallStaticVoidMethod(env, cls, mid, intArg);
    if ((*env)->ExceptionCheck(env)) {
        LogE("exception in %s", method);
        (*env)->ExceptionDescribe(env);
        (*env)->ExceptionClear(env);
        (*env)->DeleteLocalRef(env, cls);
        return -1;
    }
    (*env)->DeleteLocalRef(env, cls);
    return 0;
}
*/
import "C"

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
	"fyne.io/fyne/v2/storage"

	log "github.com/schollz/logger"
)

var errJNI = errors.New("JNI call failed")

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

// --- callers ---

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
		startForegroundService()
	} else if old > 0 && newVal <= 0 {
		stopForegroundService()
	}

	return newVal
}

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

func MimeType(uri fyne.URI) string {
	if uri == nil {
		return ""
	}
	s, _ := callStringString("getMimeType", uri.String())
	return s
}

func OpenURL(intentStr string) error {
	if err := callVoidString("openIntent", intentStr); err != nil {
		return fmt.Errorf("intent failed: %s: %w", intentStr, err)
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
		return name
	}
	return base(uri.Path())
}

func base(path string) string {
	decoded, err := url.PathUnescape(path)
	if err != nil {
		decoded = strings.ReplaceAll(path, "%2F", "/")
	}

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

func (t *Toast) SetIcon(_ fyne.Resource) *Toast       { return t }
func (t *Toast) SetText(message string) *Toast         { t.message = message; return t }
func (t *Toast) SetTimeout(_ time.Duration) *Toast     { return t }
func (t *Toast) SetPadding(_ float32) *Toast           { return t }
func (t *Toast) SetAnimation(_ bool) *Toast            { return t }
func (t *Toast) Short() *Toast                         { return t }
func (t *Toast) Long() *Toast                          { return t }
func (t *Toast) Hide()                                 {}

func (t *Toast) Show() {
	callVoidString("showToast", t.message)
}
