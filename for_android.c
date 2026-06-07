#include <android/log.h>
#define LogE(...) __android_log_print(ANDROID_LOG_ERROR, "croc", __VA_ARGS__)
#include "_cgo_export.h"
#include "for_android.h"

#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <errno.h>

void LogD(const char* message) {
	__android_log_write(ANDROID_LOG_DEBUG, "croc", message);
}

jint callVoid(JNIEnv* env, jobject context, const char* method) {
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

jint callVoidString(JNIEnv* env, jobject context, const char* method, const char* strArg) {
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

jint callInt(JNIEnv* env, jobject context, const char* method) {
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

char* callStringString(JNIEnv* env, jobject context, const char* method, const char* strArg) {
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

jint callBooleanString(JNIEnv* env, jobject context, const char* method, const char* strArg) {
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

char* callStringString2(JNIEnv* env, jobject context, const char* method, const char* strArg1, const char* strArg2) {
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

jlong callLongString(JNIEnv* env, jobject context, const char* method, const char* strArg) {
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

jint callIntString(JNIEnv* env, jobject context, const char* method, const char* strArg) {
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

char* callStringStringString(JNIEnv* env, jobject context, const char* method, const char* strArg1, const char* strArg2, const char* strArg3) {
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

char* callStringString4(JNIEnv* env, jobject context, const char* method, const char* strArg1, const char* strArg2, const char* strArg3, const char* strArg4) {
    jclass cls = (*env)->GetObjectClass(env, context);
    if (cls == NULL) { LogE("GetObjectClass failed for callStringString4(%s)", method); return NULL; }
    jmethodID mid = (*env)->GetStaticMethodID(env, cls, method, "(Ljava/lang/String;Ljava/lang/String;Ljava/lang/String;Ljava/lang/String;)Ljava/lang/String;");
    if (mid == NULL) {
        LogE("static method not found: %s", method);
        (*env)->DeleteLocalRef(env, cls);
        return NULL;
    }
    jstring jarg1 = (*env)->NewStringUTF(env, strArg1);
    jstring jarg2 = (*env)->NewStringUTF(env, strArg2);
    jstring jarg3 = (*env)->NewStringUTF(env, strArg3);
    jstring jarg4 = (*env)->NewStringUTF(env, strArg4);
    jstring jresult = (jstring)(*env)->CallStaticObjectMethod(env, cls, mid, jarg1, jarg2, jarg3, jarg4);
    if ((*env)->ExceptionCheck(env)) {
        LogE("exception in %s", method);
        (*env)->ExceptionDescribe(env);
        (*env)->ExceptionClear(env);
        (*env)->DeleteLocalRef(env, jarg1);
        (*env)->DeleteLocalRef(env, jarg2);
        (*env)->DeleteLocalRef(env, jarg3);
        (*env)->DeleteLocalRef(env, jarg4);
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
    (*env)->DeleteLocalRef(env, jarg4);
    (*env)->DeleteLocalRef(env, cls);
    return result;
}

jint callBooleanString2(JNIEnv* env, jobject context, const char* method, const char* strArg1, const char* strArg2) {
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

jint callVoidInt(JNIEnv* env, jobject context, const char* method, jint intArg) {
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

jint callBooleanStringLong(JNIEnv* env, jobject context, const char* method, const char* strArg, jlong longArg) {
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

jint setModTimeUsingFD(JNIEnv* env, jobject context, const char* uriStr, jlong modTimeMillis) {
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

static jboolean caseException(JNIEnv* env, const char* context) {
	if ((*env)->ExceptionCheck(env)) {
		LogE("Exception: %s", context);
		(*env)->ExceptionDescribe(env);
		(*env)->ExceptionClear(env);
		return JNI_TRUE;
	}
	return JNI_FALSE;
}

JNIEXPORT void Java_org_golang_app_GoNativeActivity_lifecycleEvent(JNIEnv *env, jobject thiz, jstring event) {
	if (event == NULL) return;
	const char *cevent = (*env)->GetStringUTFChars(env, event, NULL);
	if (caseException(env, "lifecycle GetStringUTFChars") || cevent == NULL) return;
	lifecycleEventNotify((char*)cevent);
	(*env)->ReleaseStringUTFChars(env, event, cevent);
}

JNIEXPORT void Java_org_golang_app_GoNativeActivity_intentURI(JNIEnv *env, jobject thiz, jstring uri) {
	if (uri == NULL) return;
	const char *curi = (*env)->GetStringUTFChars(env, uri, NULL);
	if (caseException(env, "intentURI GetStringUTFChars") || curi == NULL) return;
	intentURINotify((char*)curi);
	(*env)->ReleaseStringUTFChars(env, uri, curi);
}

JNIEXPORT void Java_org_golang_app_GoNativeActivity_intentText(JNIEnv *env, jobject thiz, jstring text) {
	if (text == NULL) return;
	const char *ctext = (*env)->GetStringUTFChars(env, text, NULL);
	if (caseException(env, "intentText GetStringUTFChars") || ctext == NULL) return;
	intentTextNotify((char*)ctext);
	(*env)->ReleaseStringUTFChars(env, text, ctext);
}
