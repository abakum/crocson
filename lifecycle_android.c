#include "_cgo_export.h"
#include "crocson_jni.h"

static jboolean lifecycleCaseException(JNIEnv* env, const char* context) {
	if ((*env)->ExceptionCheck(env)) {
		LogE("Exception in lifecycleEvent: %s", context);
		(*env)->ExceptionDescribe(env);
		(*env)->ExceptionClear(env);
		return JNI_TRUE;
	}
	return JNI_FALSE;
}

JNIEXPORT void Java_org_golang_app_GoNativeActivity_lifecycleEvent(JNIEnv *env, jobject thiz, jstring event) {
	if (event == NULL) return;
	const char *cevent = (*env)->GetStringUTFChars(env, event, NULL);
	if (lifecycleCaseException(env, "GetStringUTFChars") || cevent == NULL) return;
	lifecycleEventNotify((char*)cevent);
	(*env)->ReleaseStringUTFChars(env, event, cevent);
}
