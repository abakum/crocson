//go:build android

package main

/*
#include <jni.h>
#include <stdlib.h>
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
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"

	"fyne.io/fyne/v2/driver"
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
