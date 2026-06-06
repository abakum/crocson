#include "_cgo_export.h"
#include "crocson_jni.h"

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
