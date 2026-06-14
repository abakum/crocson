#ifndef FOR_ANDROID_H
#define FOR_ANDROID_H

#include <jni.h>

void LogD(const char* message);
jint callVoid(JNIEnv* env, jobject context, const char* method);
jint callVoidString(JNIEnv* env, jobject context, const char* method, const char* strArg);
jint callInt(JNIEnv* env, jobject context, const char* method);
jint callBoolean(JNIEnv* env, jobject context, const char* method);
char* callStringString(JNIEnv* env, jobject context, const char* method, const char* strArg);
jint callBooleanString(JNIEnv* env, jobject context, const char* method, const char* strArg);
char* callStringString2(JNIEnv* env, jobject context, const char* method, const char* strArg1, const char* strArg2);
jlong callLongString(JNIEnv* env, jobject context, const char* method, const char* strArg);
jint callIntString(JNIEnv* env, jobject context, const char* method, const char* strArg);
char* callStringStringString(JNIEnv* env, jobject context, const char* method, const char* strArg1, const char* strArg2, const char* strArg3);
char* callStringString4(JNIEnv* env, jobject context, const char* method, const char* strArg1, const char* strArg2, const char* strArg3, const char* strArg4);
jint callBooleanString2(JNIEnv* env, jobject context, const char* method, const char* strArg1, const char* strArg2);
jint callVoidInt(JNIEnv* env, jobject context, const char* method, jint intArg);
jint callBooleanStringLong(JNIEnv* env, jobject context, const char* method, const char* strArg, jlong longArg);
jint setModTimeUsingFD(JNIEnv* env, jobject context, const char* uriStr, jlong modTimeMillis);

#endif
