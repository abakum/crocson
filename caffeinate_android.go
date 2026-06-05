//go:build android

package main

/*
#include <jni.h>
#include <android/log.h>

#define LogD(...) __android_log_print(ANDROID_LOG_DEBUG, "croc", __VA_ARGS__)
#define LogE(...) __android_log_print(ANDROID_LOG_ERROR, "croc", __VA_ARGS__)

static jboolean caseException(JNIEnv* env, const char* context) {
    if ((*env)->ExceptionCheck(env)) {
        LogE("Exception in %s", context);
        (*env)->ExceptionDescribe(env);
        (*env)->ExceptionClear(env);
        return JNI_TRUE;
    }
    return JNI_FALSE;
}

static jint getSdkInt(JNIEnv* env) {
    jclass version_class = (*env)->FindClass(env, "android/os/Build$VERSION");
    if (version_class == NULL) return 0;
    jfieldID sdk_int_field = (*env)->GetStaticFieldID(env, version_class, "SDK_INT", "I");
    if (sdk_int_field == NULL) {
        (*env)->DeleteLocalRef(env, version_class);
        return 0;
    }
    jint sdk = (*env)->GetStaticIntField(env, version_class, sdk_int_field);
    (*env)->DeleteLocalRef(env, version_class);
    return sdk;
}
*/
import "C"
import (
	"sync/atomic"
	"unsafe"

	log "github.com/schollz/logger"

	"fyne.io/fyne/v2/driver"
)

// GetSDKInt возвращает версию Android SDK
func GetSDKInt() int32 {
	var sdk C.jint

	driver.RunNative(func(ctx interface{}) error {
		ac := ctx.(*driver.AndroidContext)
		env := (*C.JNIEnv)(unsafe.Pointer(ac.Env))
		sdk = C.getSdkInt(env)
		return nil
	})

	return int32(sdk)
}

func apiLevel() int {
	return int(GetSDKInt())
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
