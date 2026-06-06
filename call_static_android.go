//go:build android

package main

/*
#include <jni.h>
#include <stdlib.h>
#include <string.h>
#include <android/log.h>

#define LogE(...) __android_log_print(ANDROID_LOG_ERROR, "croc", __VA_ARGS__)
static void LogD(const char* message) {
	__android_log_write(ANDROID_LOG_DEBUG, "croc", message);
}

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

static char* callStringString2(JNIEnv* env, jobject context, const char* method, const char* strArg1, const char* strArg2) {
    jclass cls = (*env)->GetObjectClass(env, context);
    if (cls == NULL) { LogE("GetObjectClass failed for callStringString2(%s)", method); return NULL; }
    jmethodID mid = (*env)->GetStaticMethodID(env, cls, method, "(Ljava/lang/String;Ljava/lang/String;)Ljava/lang/String;");
    if (mid == NULL) {
        LogE("static method not found: %s", method);
        (*env)->DeleteLocalRef(env, cls);
        return NULL;
    }
    jstring jarg1 = (*env)->NewStringUTF(env, strArg1);
    jstring jarg2 = (*env)->NewStringUTF(env, strArg2);
    jstring jresult = (jstring)(*env)->CallStaticObjectMethod(env, cls, mid, jarg1, jarg2);
    if ((*env)->ExceptionCheck(env)) {
        LogE("exception in %s", method);
        (*env)->ExceptionDescribe(env);
        (*env)->ExceptionClear(env);
        (*env)->DeleteLocalRef(env, jarg1);
        (*env)->DeleteLocalRef(env, jarg2);
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
    (*env)->DeleteLocalRef(env, jarg1);
    (*env)->DeleteLocalRef(env, jarg2);
    (*env)->DeleteLocalRef(env, cls);
    return result;
}

static jlong callLongString(JNIEnv* env, jobject context, const char* method, const char* strArg) {
    jclass cls = (*env)->GetObjectClass(env, context);
    if (cls == NULL) { LogE("GetObjectClass failed for callLongString(%s)", method); return -1; }
    jmethodID mid = (*env)->GetStaticMethodID(env, cls, method, "(Ljava/lang/String;)J");
    if (mid == NULL) {
        LogE("static method not found: %s", method);
        (*env)->DeleteLocalRef(env, cls);
        return -1;
    }
    jstring jarg = (*env)->NewStringUTF(env, strArg);
    jlong result = (*env)->CallStaticLongMethod(env, cls, mid, jarg);
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
    return result;
}

static jint callIntString(JNIEnv* env, jobject context, const char* method, const char* strArg) {
    jclass cls = (*env)->GetObjectClass(env, context);
    if (cls == NULL) { LogE("GetObjectClass failed for callIntString(%s)", method); return -1; }
    jmethodID mid = (*env)->GetStaticMethodID(env, cls, method, "(Ljava/lang/String;)I");
    if (mid == NULL) {
        LogE("static method not found: %s", method);
        (*env)->DeleteLocalRef(env, cls);
        return -1;
    }
    jstring jarg = (*env)->NewStringUTF(env, strArg);
    jint result = (*env)->CallStaticIntMethod(env, cls, mid, jarg);
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
    return result;
}

static char* callStringStringString(JNIEnv* env, jobject context, const char* method, const char* strArg1, const char* strArg2, const char* strArg3) {
    jclass cls = (*env)->GetObjectClass(env, context);
    if (cls == NULL) { LogE("GetObjectClass failed for callStringStringString(%s)", method); return NULL; }
    jmethodID mid = (*env)->GetStaticMethodID(env, cls, method, "(Ljava/lang/String;Ljava/lang/String;Ljava/lang/String;)Ljava/lang/String;");
    if (mid == NULL) {
        LogE("static method not found: %s", method);
        (*env)->DeleteLocalRef(env, cls);
        return NULL;
    }
    jstring jarg1 = (*env)->NewStringUTF(env, strArg1);
    jstring jarg2 = (*env)->NewStringUTF(env, strArg2);
    jstring jarg3 = (*env)->NewStringUTF(env, strArg3);
    jstring jresult = (jstring)(*env)->CallStaticObjectMethod(env, cls, mid, jarg1, jarg2, jarg3);
    if ((*env)->ExceptionCheck(env)) {
        LogE("exception in %s", method);
        (*env)->ExceptionDescribe(env);
        (*env)->ExceptionClear(env);
        (*env)->DeleteLocalRef(env, jarg1);
        (*env)->DeleteLocalRef(env, jarg2);
        (*env)->DeleteLocalRef(env, jarg3);
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
    (*env)->DeleteLocalRef(env, jarg1);
    (*env)->DeleteLocalRef(env, jarg2);
    (*env)->DeleteLocalRef(env, jarg3);
    (*env)->DeleteLocalRef(env, cls);
    return result;
}

static jint callBooleanString2(JNIEnv* env, jobject context, const char* method, const char* strArg1, const char* strArg2) {
    jclass cls = (*env)->GetObjectClass(env, context);
    if (cls == NULL) { LogE("GetObjectClass failed for callBooleanString2(%s)", method); return -1; }
    jmethodID mid = (*env)->GetStaticMethodID(env, cls, method, "(Ljava/lang/String;Ljava/lang/String;)Z");
    if (mid == NULL) {
        LogE("static method not found: %s", method);
        (*env)->DeleteLocalRef(env, cls);
        return -1;
    }
    jstring jarg1 = (*env)->NewStringUTF(env, strArg1);
    jstring jarg2 = (*env)->NewStringUTF(env, strArg2);
    jboolean result = (*env)->CallStaticBooleanMethod(env, cls, mid, jarg1, jarg2);
    if ((*env)->ExceptionCheck(env)) {
        LogE("exception in %s", method);
        (*env)->ExceptionDescribe(env);
        (*env)->ExceptionClear(env);
        (*env)->DeleteLocalRef(env, jarg1);
        (*env)->DeleteLocalRef(env, jarg2);
        (*env)->DeleteLocalRef(env, cls);
        return -1;
    }
    (*env)->DeleteLocalRef(env, jarg1);
    (*env)->DeleteLocalRef(env, jarg2);
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

static jint callBooleanStringLong(JNIEnv* env, jobject context, const char* method, const char* strArg, jlong longArg) {
    jclass cls = (*env)->GetObjectClass(env, context);
    if (cls == NULL) { LogE("GetObjectClass failed for callBooleanStringLong(%s)", method); return -1; }
    jmethodID mid = (*env)->GetStaticMethodID(env, cls, method, "(Ljava/lang/String;J)Z");
    if (mid == NULL) {
        LogE("static method not found: %s", method);
        (*env)->DeleteLocalRef(env, cls);
        return -1;
    }
    jstring jarg = (*env)->NewStringUTF(env, strArg);
    jboolean result = (*env)->CallStaticBooleanMethod(env, cls, mid, jarg, longArg);
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

#include <sys/stat.h>
#include <errno.h>

static jint setModTimeUsingFD(JNIEnv* env, jobject context, const char* uriStr, jlong modTimeMillis) {
    jint ret = 0;
    jclass activityClass = (*env)->GetObjectClass(env, context);
    if (activityClass == NULL) { LogE("setModTimeUsingFD: activityClass NULL"); return 0; }
    jmethodID getContentResolver = (*env)->GetMethodID(env, activityClass, "getContentResolver", "()Landroid/content/ContentResolver;");
    if (getContentResolver == NULL) { LogE("setModTimeUsingFD: getContentResolver NULL"); (*env)->DeleteLocalRef(env, activityClass); return 0; }
    jobject resolver = (*env)->CallObjectMethod(env, context, getContentResolver);
    if ((*env)->ExceptionCheck(env) || resolver == NULL) { LogE("setModTimeUsingFD: resolver NULL"); (*env)->ExceptionClear(env); (*env)->DeleteLocalRef(env, activityClass); return 0; }

    jclass uriClass = (*env)->FindClass(env, "android/net/Uri");
    if (uriClass == NULL) { (*env)->DeleteLocalRef(env, resolver); (*env)->DeleteLocalRef(env, activityClass); return 0; }
    jmethodID parseMethod = (*env)->GetStaticMethodID(env, uriClass, "parse", "(Ljava/lang/String;)Landroid/net/Uri;");
    if (parseMethod == NULL) { (*env)->DeleteLocalRef(env, uriClass); (*env)->DeleteLocalRef(env, resolver); (*env)->DeleteLocalRef(env, activityClass); return 0; }
    jstring juriStr = (*env)->NewStringUTF(env, uriStr);
    jobject uri = (*env)->CallStaticObjectMethod(env, uriClass, parseMethod, juriStr);
    (*env)->DeleteLocalRef(env, juriStr);
    if ((*env)->ExceptionCheck(env) || uri == NULL) { LogE("setModTimeUsingFD: uri NULL"); (*env)->ExceptionClear(env); (*env)->DeleteLocalRef(env, uriClass); (*env)->DeleteLocalRef(env, resolver); (*env)->DeleteLocalRef(env, activityClass); return 0; }

    jclass resolverClass = (*env)->GetObjectClass(env, resolver);
    jmethodID openFD = (*env)->GetMethodID(env, resolverClass, "openFileDescriptor", "(Landroid/net/Uri;Ljava/lang/String;)Landroid/os/ParcelFileDescriptor;");
    if (openFD == NULL) { LogE("setModTimeUsingFD: openFD NULL"); (*env)->DeleteLocalRef(env, resolverClass); (*env)->DeleteLocalRef(env, uri); (*env)->DeleteLocalRef(env, uriClass); (*env)->DeleteLocalRef(env, resolver); (*env)->DeleteLocalRef(env, activityClass); return 0; }
    jstring mode = (*env)->NewStringUTF(env, "rw");
    jobject pfd = (*env)->CallObjectMethod(env, resolver, openFD, uri, mode);
    (*env)->DeleteLocalRef(env, mode);
    if ((*env)->ExceptionCheck(env) || pfd == NULL) { LogE("setModTimeUsingFD: pfd NULL (openFileDescriptor failed)"); (*env)->ExceptionClear(env); (*env)->DeleteLocalRef(env, resolverClass); (*env)->DeleteLocalRef(env, uri); (*env)->DeleteLocalRef(env, uriClass); (*env)->DeleteLocalRef(env, resolver); (*env)->DeleteLocalRef(env, activityClass); return 0; }

    jclass pfdClass = (*env)->GetObjectClass(env, pfd);
    jmethodID getFd = (*env)->GetMethodID(env, pfdClass, "getFd", "()I");
    jint fd = (*env)->CallIntMethod(env, pfd, getFd);
    LogE("setModTimeUsingFD: fd=%d", fd);
    if (fd >= 0) {
        long sec = modTimeMillis / 1000;
        long nsec = (modTimeMillis % 1000) * 1000000;
        struct timespec ts[2];
        ts[0].tv_sec = sec; ts[0].tv_nsec = nsec;
        ts[1].tv_sec = sec; ts[1].tv_nsec = nsec;
        if (futimens(fd, ts) == 0) {
            ret = 1;
            LogE("setModTimeUsingFD: futimens OK");
        } else {
            LogE("setModTimeUsingFD: futimens failed errno=%d", errno);
        }
    }
    jmethodID closePfd = (*env)->GetMethodID(env, pfdClass, "close", "()V");
    if (closePfd != NULL) (*env)->CallVoidMethod(env, pfd, closePfd);

    (*env)->DeleteLocalRef(env, pfdClass);
    (*env)->DeleteLocalRef(env, pfd);
    (*env)->DeleteLocalRef(env, resolverClass);
    (*env)->DeleteLocalRef(env, uri);
    (*env)->DeleteLocalRef(env, uriClass);
    (*env)->DeleteLocalRef(env, resolver);
    (*env)->DeleteLocalRef(env, activityClass);
    return ret;
}
*/
import "C"

import (
	"errors"
	"fmt"
	"net/url"
	"os"
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
		return "", fmt.Errorf("empty result from createFileInDownloads")
	}

	return result, nil
}

func ChildDownload(component string) (child fyne.URI, cleanup func(), err error) {
	cleanup = func() {}

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
