//go:build android

package main

/*
#include <jni.h>
#include <android/log.h>

#define LogD(...) __android_log_print(ANDROID_LOG_DEBUG, "croc", __VA_ARGS__)
#define LogE(...) __android_log_print(ANDROID_LOG_ERROR, "croc", __VA_ARGS__)

static jobject globalWakeLock = NULL;

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

static void acquireWakeLock(JNIEnv* env, jobject activity) {
    if (globalWakeLock != NULL) return;

    jclass activity_class = (*env)->GetObjectClass(env, activity);
    jmethodID getSystemServiceMethod = (*env)->GetMethodID(env, activity_class,
        "getSystemService", "(Ljava/lang/String;)Ljava/lang/Object;");
    if (caseException(env, "GetMethodID getSystemService")) return;

    jclass contextClass = (*env)->FindClass(env, "android/content/Context");
    if (caseException(env, "FindClass Context")) return;

    jfieldID powerServiceField = (*env)->GetStaticFieldID(env, contextClass, "POWER_SERVICE", "Ljava/lang/String;");
    if (caseException(env, "GetStaticFieldID POWER_SERVICE")) return;

    jstring powerServiceString = (*env)->GetStaticObjectField(env, contextClass, powerServiceField);
    if (caseException(env, "GetStaticObjectField POWER_SERVICE")) return;

    jobject powerManager = (*env)->CallObjectMethod(env, activity, getSystemServiceMethod, powerServiceString);
    if (caseException(env, "CallObjectMethod getSystemService") || powerManager == NULL) return;

    jclass powerManager_class = (*env)->GetObjectClass(env, powerManager);
    if (caseException(env, "GetObjectClass PowerManager")) return;

    jmethodID newWakeLockMethod = (*env)->GetMethodID(env, powerManager_class,
        "newWakeLock", "(ILjava/lang/String;)Landroid/os/PowerManager$WakeLock;");
    if (caseException(env, "GetMethodID newWakeLock")) return;

    jstring tag = (*env)->NewStringUTF(env, "crocson:transfer");
    if (caseException(env, "NewStringUTF")) return;

    jobject localWakeLock = (*env)->CallObjectMethod(env, powerManager, newWakeLockMethod, 1, tag);
    (*env)->DeleteLocalRef(env, tag);

    if (caseException(env, "newWakeLock") || localWakeLock == NULL) return;

    globalWakeLock = (*env)->NewGlobalRef(env, localWakeLock);
    if (globalWakeLock == NULL) {
        LogE("Failed to create global reference");
        return;
    }

    jclass wakeLock_class = (*env)->GetObjectClass(env, globalWakeLock);
    if (caseException(env, "GetObjectClass WakeLock")) return;

    jmethodID acquireMethod = (*env)->GetMethodID(env, wakeLock_class, "acquire", "()V");
    if (caseException(env, "GetMethodID acquire")) return;

    if (acquireMethod != NULL) {
        (*env)->CallVoidMethod(env, globalWakeLock, acquireMethod);
        if (caseException(env, "CallVoidMethod acquire")) return;
        LogD("WakeLock acquired");
    }
}

static void releaseWakeLock(JNIEnv* env, jobject activity) {
    if (globalWakeLock == NULL) return;

    jclass wakeLock_class = (*env)->GetObjectClass(env, globalWakeLock);
    if (caseException(env, "GetObjectClass WakeLock")) return;

    jmethodID releaseMethod = (*env)->GetMethodID(env, wakeLock_class, "release", "()V");
    if (caseException(env, "GetMethodID release")) return;

    if (releaseMethod != NULL) {
        (*env)->CallVoidMethod(env, globalWakeLock, releaseMethod);
        if (caseException(env, "CallVoidMethod release")) return;
        LogD("WakeLock released");
    }

    (*env)->DeleteGlobalRef(env, globalWakeLock);
    globalWakeLock = NULL;
}

static jclass loadServiceClass(JNIEnv* env, jobject context) {
    jclass context_class = (*env)->GetObjectClass(env, context);
    if (caseException(env, "GetObjectClass context")) return NULL;

    jmethodID getClassLoaderMethod = (*env)->GetMethodID(env, context_class,
        "getClassLoader", "()Ljava/lang/ClassLoader;");
    if (caseException(env, "GetMethodID getClassLoader")) {
        (*env)->DeleteLocalRef(env, context_class);
        return NULL;
    }

    jobject classLoader = (*env)->CallObjectMethod(env, context, getClassLoaderMethod);
    (*env)->DeleteLocalRef(env, context_class);
    if (caseException(env, "CallObjectMethod getClassLoader") || classLoader == NULL) return NULL;

    jclass classLoaderClass = (*env)->GetObjectClass(env, classLoader);
    if (caseException(env, "GetObjectClass classLoader")) {
        (*env)->DeleteLocalRef(env, classLoader);
        return NULL;
    }

    jmethodID loadClassMethod = (*env)->GetMethodID(env, classLoaderClass,
        "loadClass", "(Ljava/lang/String;)Ljava/lang/Class;");
    if (caseException(env, "GetMethodID loadClass")) {
        (*env)->DeleteLocalRef(env, classLoaderClass);
        (*env)->DeleteLocalRef(env, classLoader);
        return NULL;
    }

    jstring className = (*env)->NewStringUTF(env, "com.github.abakum.crocson.CrocsonService");
    if (caseException(env, "NewStringUTF className")) {
        (*env)->DeleteLocalRef(env, classLoaderClass);
        (*env)->DeleteLocalRef(env, classLoader);
        return NULL;
    }

    jclass service_class = (jclass)(*env)->CallObjectMethod(env, classLoader, loadClassMethod, className);
    (*env)->DeleteLocalRef(env, className);
    (*env)->DeleteLocalRef(env, classLoaderClass);
    (*env)->DeleteLocalRef(env, classLoader);

    if (caseException(env, "loadClass CrocsonService") || service_class == NULL) {
        LogE("Failed to load CrocsonService via ClassLoader");
        return NULL;
    }

    LogD("CrocsonService loaded via ClassLoader");
    return service_class;
}

static void startCrocsonService(JNIEnv* env, jobject context) {
    jclass intent_class = (*env)->FindClass(env, "android/content/Intent");
    if (caseException(env, "FindClass Intent")) return;

    jclass service_class = loadServiceClass(env, context);
    if (service_class == NULL) {
        (*env)->DeleteLocalRef(env, intent_class);
        return;
    }

    jmethodID intent_ctor = (*env)->GetMethodID(env, intent_class,
        "<init>", "(Landroid/content/Context;Ljava/lang/Class;)V");
    if (caseException(env, "GetMethodID Intent ctor")) {
        (*env)->DeleteLocalRef(env, intent_class);
        (*env)->DeleteLocalRef(env, service_class);
        return;
    }

    jobject intent = (*env)->NewObject(env, intent_class, intent_ctor, context, service_class);
    if (caseException(env, "NewObject Intent")) {
        (*env)->DeleteLocalRef(env, intent_class);
        (*env)->DeleteLocalRef(env, service_class);
        return;
    }

    jclass context_class = (*env)->GetObjectClass(env, context);
    jint sdk = getSdkInt(env);

    if (sdk >= 26) {
        jmethodID start_fgs = (*env)->GetMethodID(env, context_class,
            "startForegroundService", "(Landroid/content/Intent;)Landroid/content/ComponentName;");
        if (!caseException(env, "GetMethodID startForegroundService") && start_fgs != NULL) {
            (*env)->CallObjectMethod(env, context, start_fgs, intent);
            caseException(env, "startForegroundService");
            LogD("Foreground service started (API %d)", sdk);
        }
    } else {
        jmethodID start_svc = (*env)->GetMethodID(env, context_class,
            "startService", "(Landroid/content/Intent;)Landroid/content/ComponentName;");
        if (start_svc != NULL) {
            (*env)->CallObjectMethod(env, context, start_svc, intent);
            caseException(env, "startService");
            LogD("Service started (legacy, API %d)", sdk);
        }
    }

    (*env)->DeleteLocalRef(env, intent);
    (*env)->DeleteLocalRef(env, intent_class);
    (*env)->DeleteLocalRef(env, service_class);
    (*env)->DeleteLocalRef(env, context_class);
}
static void stopCrocsonService(JNIEnv* env, jobject context) {
    jclass intent_class = (*env)->FindClass(env, "android/content/Intent");
    if (caseException(env, "FindClass Intent")) return;

    jclass service_class = loadServiceClass(env, context);
    if (service_class == NULL) {
        (*env)->DeleteLocalRef(env, intent_class);
        return;
    }

    jmethodID intent_ctor = (*env)->GetMethodID(env, intent_class,
        "<init>", "(Landroid/content/Context;Ljava/lang/Class;)V");
    if (caseException(env, "GetMethodID Intent ctor")) {
        (*env)->DeleteLocalRef(env, intent_class);
        (*env)->DeleteLocalRef(env, service_class);
        return;
    }

    jobject intent = (*env)->NewObject(env, intent_class, intent_ctor, context, service_class);
    if (caseException(env, "NewObject Intent")) {
        (*env)->DeleteLocalRef(env, intent_class);
        (*env)->DeleteLocalRef(env, service_class);
        return;
    }

    jclass context_class = (*env)->GetObjectClass(env, context);
    jmethodID stop_svc = (*env)->GetMethodID(env, context_class,
        "stopService", "(Landroid/content/Intent;)Z");
    if (stop_svc != NULL) {
        (*env)->CallBooleanMethod(env, context, stop_svc, intent);
        caseException(env, "stopService");
        LogD("Foreground service stopped");
    }

    (*env)->DeleteLocalRef(env, intent);
    (*env)->DeleteLocalRef(env, intent_class);
    (*env)->DeleteLocalRef(env, service_class);
    (*env)->DeleteLocalRef(env, context_class);
}
*/
import "C"
import (
	"sync/atomic"
	"unsafe"

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
		acquireWakeLock()
		startForegroundService()
	} else if old > 0 && newVal <= 0 {
		stopForegroundService()
		releaseWakeLock()
	}

	return newVal
}

func SleepAllowed() bool {
	return atomic.LoadInt32(&sleepCounter) <= 0
}

func acquireWakeLock() {
	driver.RunNative(func(ctx interface{}) error {
		if ac, ok := ctx.(*driver.AndroidContext); ok {
			C.acquireWakeLock((*C.JNIEnv)(unsafe.Pointer(ac.Env)), (C.jobject)(unsafe.Pointer(ac.Ctx)))
		}
		return nil
	})
}

func releaseWakeLock() {
	driver.RunNative(func(ctx interface{}) error {
		if ac, ok := ctx.(*driver.AndroidContext); ok {
			C.releaseWakeLock((*C.JNIEnv)(unsafe.Pointer(ac.Env)), (C.jobject)(unsafe.Pointer(ac.Ctx)))
		}
		return nil
	})
}

func startForegroundService() {
	driver.RunNative(func(ctx interface{}) error {
		if ac, ok := ctx.(*driver.AndroidContext); ok {
			C.startCrocsonService((*C.JNIEnv)(unsafe.Pointer(ac.Env)), (C.jobject)(unsafe.Pointer(ac.Ctx)))
		}
		return nil
	})
}

func stopForegroundService() {
	driver.RunNative(func(ctx interface{}) error {
		if ac, ok := ctx.(*driver.AndroidContext); ok {
			C.stopCrocsonService((*C.JNIEnv)(unsafe.Pointer(ac.Env)), (C.jobject)(unsafe.Pointer(ac.Ctx)))
		}
		return nil
	})
}
