#include "_cgo_export.h"
#include "crocson_jni.h"

static jboolean intentCaseException(JNIEnv* env, const char* context) {
	if ((*env)->ExceptionCheck(env)) {
		LogE("Exception in intent: %s", context);
		(*env)->ExceptionDescribe(env);
		(*env)->ExceptionClear(env);
		return JNI_TRUE;
	}
	return JNI_FALSE;
}

JNIEXPORT void Java_org_golang_app_GoNativeActivity_intentURI(JNIEnv *env, jobject thiz, jstring uri) {
	if (uri == NULL) return;
	const char *curi = (*env)->GetStringUTFChars(env, uri, NULL);
	if (intentCaseException(env, "GetStringUTFChars") || curi == NULL) return;
	intentURINotify((char*)curi);
	(*env)->ReleaseStringUTFChars(env, uri, curi);
}

JNIEXPORT void Java_org_golang_app_GoNativeActivity_intentText(JNIEnv *env, jobject thiz, jstring text) {
	if (text == NULL) return;
	const char *ctext = (*env)->GetStringUTFChars(env, text, NULL);
	if (intentCaseException(env, "GetStringUTFChars") || ctext == NULL) return;
	intentTextNotify((char*)ctext);
	(*env)->ReleaseStringUTFChars(env, text, ctext);
}
