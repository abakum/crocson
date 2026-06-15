#import <Foundation/Foundation.h>
#import <UIKit/UIKit.h>
#import <stdlib.h>
#import "for_ios.h"

// Устанавливает флаг предотвращения сна
void setIdleTimerDisabled(BOOL disabled) {
    @autoreleasepool {
        [[UIApplication sharedApplication] setIdleTimerDisabled:disabled];
    }
}

// Получаем security-scoped bookmark для папки Downloads
char* CreateBookmarkFromURLDownload() {
    @autoreleasepool {
        // 1. Получаем URL папки Downloads
        NSFileManager *fileManager = [NSFileManager defaultManager];
        NSURL *downloadsURL = [fileManager URLForDirectory:NSDownloadsDirectory
                                                  inDomain:NSUserDomainMask
                                         appropriateForURL:nil
                                                    create:YES  // Создаем если не существует
                                                     error:nil];

        if (!downloadsURL) {
            return strdup("error: cannot get Downloads directory");
        }

        // 2. Создаем security-scoped bookmark
        NSError *error = nil;
        NSData *bookmarkData = [downloadsURL bookmarkDataWithOptions:NSURLBookmarkCreationWithSecurityScope
                                      includingResourceValuesForKeys:nil
                                                       relativeToURL:nil
                                                               error:&error];

        if (error) {
            NSString *errorMsg = [NSString stringWithFormat:@"error: failed to create bookmark: %@", error.localizedDescription];
            return strdup([errorMsg UTF8String]);
        }

        if (!bookmarkData) {
            return strdup("error: bookmark data is nil");
        }

        // 3. Конвертируем в base64 строку
        NSString *bookmarkString = [bookmarkData base64EncodedStringWithOptions:0];
        return strdup([bookmarkString UTF8String]);
    }
}

// Создает файл в указанной папке
char* CreateFileInDownloads(char* bookmarkDataStr, char* fileName, char* mimeType) {
    @autoreleasepool {
        // 1. Восстанавливаем bookmark
        NSString *bookmarkString = [NSString stringWithUTF8String:bookmarkDataStr];
        NSData *bookmarkData = [[NSData alloc] initWithBase64EncodedString:bookmarkString options:0];

        if (!bookmarkData) {
            return strdup("error: invalid bookmark data");
        }

        BOOL isStale = NO;
        NSError *error = nil;
        NSURL *downloadsURL = [NSURL URLByResolvingBookmarkData:bookmarkData
                                                        options:NSURLBookmarkResolutionWithSecurityScope
                                                  relativeToURL:nil
                                            bookmarkDataIsStale:&isStale
                                                          error:&error];

        if (error) {
            NSString *errorMsg = [NSString stringWithFormat:@"error: failed to resolve bookmark: %@", error.localizedDescription];
            return strdup([errorMsg UTF8String]);
        }

        if (isStale) {
            return strdup("error: bookmark is stale");
        }

        if (!downloadsURL) {
            return strdup("error: resolved URL is nil");
        }

        // 2. Начинаем security-scoped доступ
        if (![downloadsURL startAccessingSecurityScopedResource]) {
            return strdup("error: cannot start security-scoped access");
        }

        // 3. Создаем полный путь к файлу
        NSString *fileNameStr = [NSString stringWithUTF8String:fileName];
        NSURL *fileURL = [downloadsURL URLByAppendingPathComponent:fileNameStr];

        // 4. Создаем директории если нужно
        NSURL *directoryURL = [fileURL URLByDeletingLastPathComponent];
        if (![directoryURL isEqual:downloadsURL]) {
            // Создаем вложенные директории
            NSFileManager *fileManager = [NSFileManager defaultManager];
            [fileManager createDirectoryAtURL:directoryURL
                  withIntermediateDirectories:YES
                                   attributes:nil
                                        error:nil];
        }

        // 5. Создаем файл
        NSFileManager *fileManager = [NSFileManager defaultManager];
        if (![fileManager createFileAtPath:[fileURL path] contents:nil attributes:nil]) {
            [downloadsURL stopAccessingSecurityScopedResource];
            return strdup("error: failed to create file");
        }

        // 6. Создаем security-scoped bookmark для нового файла
        NSData *fileBookmarkData = [fileURL bookmarkDataWithOptions:NSURLBookmarkCreationWithSecurityScope
                                     includingResourceValuesForKeys:nil
                                                      relativeToURL:nil
                                                              error:&error];

        [downloadsURL stopAccessingSecurityScopedResource];

        if (error) {
            NSString *errorMsg = [NSString stringWithFormat:@"error: failed to create file bookmark: %@", error.localizedDescription];
            return strdup([errorMsg UTF8String]);
        }

        if (!fileBookmarkData) {
            return strdup("error: file bookmark data is nil");
        }

        // 7. Конвертируем в base64 строку
        NSString *fileBookmarkString = [fileBookmarkData base64EncodedStringWithOptions:0];
        return strdup([fileBookmarkString UTF8String]);
    }
}

// Создание security-scoped bookmark из URL
char* CreateBookmarkFromURL(const char* urlString) {
    @autoreleasepool {
        NSString *nsUrlString = [NSString stringWithUTF8String:urlString];
        NSURL *url = [NSURL URLWithString:nsUrlString];

        if (!url) {
            return strdup("error: invalid URL");
        }

        NSError *error = nil;
        NSData *bookmarkData = [url bookmarkDataWithOptions:NSURLBookmarkCreationWithSecurityScope
                             includingResourceValuesForKeys:nil
                                              relativeToURL:nil
                                                      error:&error];

        if (error) {
            NSString *errorMsg = [NSString stringWithFormat:@"error: failed to create bookmark: %@", error.localizedDescription];
            return strdup([errorMsg UTF8String]);
        }

        if (!bookmarkData) {
            return strdup("error: bookmark data is nil");
        }

        NSString *bookmarkString = [bookmarkData base64EncodedStringWithOptions:0];
        return strdup([bookmarkString UTF8String]);
    }
}

// Разрешение bookmark'а в URL с security scope
char* ResolveBookmarkToURL(const char* bookmarkDataString, bool* isStaleOut) {
    @autoreleasepool {
        NSString *nsBookmarkData = [NSString stringWithUTF8String:bookmarkDataString];
        NSData *bookmarkData = [[NSData alloc] initWithBase64EncodedString:nsBookmarkData options:0];

        if (!bookmarkData) {
            return strdup("error: invalid bookmark data");
        }

        NSError *error = nil;
        BOOL isStale = NO;
        NSURL *url = [NSURL URLByResolvingBookmarkData:bookmarkData
                                               options:NSURLBookmarkResolutionWithSecurityScope
                                         relativeToURL:nil
                                   bookmarkDataIsStale:&isStale
                                                 error:&error];

        if (error) {
            NSString *errorMsg = [NSString stringWithFormat:@"error: failed to resolve bookmark: %@", error.localizedDescription];
            return strdup([errorMsg UTF8String]);
        }

        if (!url) {
            return strdup("error: resolved URL is nil");
        }

        if (isStaleOut) {
            *isStaleOut = isStale;
        }

        // Начинаем security-scoped доступ
        if (![url startAccessingSecurityScopedResource]) {
            return strdup("error: failed to start security scoped access");
        }

        return strdup([url.absoluteString UTF8String]);
    }
}

// Остановка security-scoped доступа
void StopAccessingSecurityScopedResource(const char* urlString) {
    @autoreleasepool {
        NSString *nsUrlString = [NSString stringWithUTF8String:urlString];
        NSURL *url = [NSURL URLWithString:nsUrlString];

        if (url) {
            [url stopAccessingSecurityScopedResource];
        }
    }
}

// Функция для создания файла через security-scoped bookmark
char* CreateFileInTreeIOS(const char* bookmarkData, const char* fileName, const char* mimeType) {
    @autoreleasepool {
        NSString *nsBookmarkData = [NSString stringWithUTF8String:bookmarkData];
        NSString *nsFileName = [NSString stringWithUTF8String:fileName];
        NSString *nsMimeType = [NSString stringWithUTF8String:mimeType];

        // Разрешаем bookmark в URL
        NSData *bookmarkDataObj = [[NSData alloc] initWithBase64EncodedString:nsBookmarkData options:0];
        if (!bookmarkDataObj) {
            return strdup("error: invalid bookmark data");
        }

        NSError *error = nil;
        BOOL isStale = NO;
        NSURL *targetURL = [NSURL URLByResolvingBookmarkData:bookmarkDataObj
                                                    options:NSURLBookmarkResolutionWithSecurityScope
                                              relativeToURL:nil
                                        bookmarkDataIsStale:&isStale
                                                      error:&error];

        if (error || !targetURL) {
            NSString *errorMsg = [NSString stringWithFormat:@"error: failed to resolve bookmark: %@", error ? error.localizedDescription : @"unknown error"];
            return strdup([errorMsg UTF8String]);
        }

        // Начинаем security-scoped доступ
        if (![targetURL startAccessingSecurityScopedResource]) {
            return strdup("error: failed to access security scoped resource");
        }

        // Проверяем, что это директория
        NSNumber *isDirectory = nil;
        if (![targetURL getResourceValue:&isDirectory forKey:NSURLIsDirectoryKey error:&error] || !isDirectory.boolValue) {
            [targetURL stopAccessingSecurityScopedResource];
            return strdup("error: target is not a directory");
        }

        // Создаем файл внутри директории
        NSURL *newFileURL = [targetURL URLByAppendingPathComponent:nsFileName];
        NSData *emptyData = [NSData data];
        BOOL success = [emptyData writeToURL:newFileURL options:NSDataWritingAtomic error:&error];

        // Останавливаем доступ
        [targetURL stopAccessingSecurityScopedResource];

        if (!success) {
            NSString *errorMsg = [NSString stringWithFormat:@"error: failed to create file: %@", error.localizedDescription];
            return strdup([errorMsg UTF8String]);
        }

        // Создаем bookmark для нового файла
        NSData *newBookmarkData = [newFileURL bookmarkDataWithOptions:NSURLBookmarkCreationWithSecurityScope
                                       includingResourceValuesForKeys:nil
                                                        relativeToURL:nil
                                                                error:&error];

        if (error) {
            // Все равно возвращаем URL, даже если не удалось создать bookmark
            return strdup([newFileURL.absoluteString UTF8String]);
        }

        // Возвращаем URL нового файла
        return strdup([newFileURL.absoluteString UTF8String]);
    }
}
