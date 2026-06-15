#ifndef FOR_IOS_H
#define FOR_IOS_H

#import <Foundation/Foundation.h>

void setIdleTimerDisabled(BOOL disabled);

char* CreateBookmarkFromURLDownload(void);
char* CreateFileInDownloads(char* bookmarkDataStr, char* fileName, char* mimeType);

char* CreateBookmarkFromURL(const char* urlString);
char* ResolveBookmarkToURL(const char* bookmarkDataString, bool* isStaleOut);
void  StopAccessingSecurityScopedResource(const char* urlString);
char* CreateFileInTreeIOS(const char* bookmarkData, const char* fileName, const char* mimeType);

#endif
