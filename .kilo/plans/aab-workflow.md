# Plan: Создать `.github/workflows/aab.yml`

## Цель

Создать GitHub Actions workflow: сборка единого AAB (Android App Bundle) со всеми архитектурами через `fyne release` из форка `abakum/tools`, затем верификация — `bundletool build-apks` извлекает split APK для каждой платформы.

## Workflow: `AAB Build and Verify`

**Триггер:** `workflow_dispatch` без опций. Всегда: `build_target = all`, `build_type = release`.

### Job `build-android-aab` на `ubuntu-latest`

1. **Checkout crocson** → `workspace/crocson`
2. **Checkout tools** fork `abakum/tools` main → `workspace/tools`
3. **Setup Go** (из go.mod)
4. **Setup JDK 21** (temurin)
5. **Setup NDK r27d** (nttld/setup-ndk@v1)
6. **Install Android SDK** — platforms;android-36 + build-tools;36.0.0
7. **Install Fyne CLI** из форка:
   ```bash
   cd $GITHUB_WORKSPACE/workspace/tools/cmd/fyne && go install
   ```
8. **Download bundletool-all-1.18.1.jar** + создать wrapper `/usr/local/bin/bundletool`:
    ```bash
    wget -q https://github.com/google/bundletool/releases/download/1.18.1/bundletool-all-1.18.1.jar -O /tmp/bundletool-all-1.18.1.jar
    printf '#!/bin/bash\nexec java -jar /tmp/bundletool-all-1.18.1.jar "$@"\n' > /usr/local/bin/bundletool
    chmod +x /usr/local/bin/bundletool
    ```
    > Версия 1.18.1 — требование RuStore для загрузки AAB. `fyne release` вызывает `bundletool build-bundle` внутри себя — wrapper обеспечивает доступность команды.

9. **Decode keystore** из `secrets.ANDROID_SIGNING_KEY` → `/tmp/keystore.jks`
10. **Read version/build** из FyneApp.toml
11. **fyne release — единый AAB:**
    ```bash
    cd $GITHUB_WORKSPACE/workspace/crocson
    fyne release --os android \
      --keystore /tmp/keystore.jks \
      --key-name "${{ secrets.ANDROID_KEY_ALIAS }}" \
      --keystore-pass "${{ secrets.ANDROID_KEYSTORE_PASSWORD }}" \
      --app-version "$VERSION_NAME" \
      --app-build "$BUILD_NUMBER"
    ```
    Результат: `crocson.aab` (подписан jarsigner, содержит arm/arm64/386/amd64)

12. **bundletool build-apks — извлечь APK:**
    ```bash
    java -jar /tmp/bundletool-all-1.18.1.jar build-apks \
      --bundle=crocson.aab \
      --output-format=DIRECTORY \
      --output=crocson-apks \
      --ks=/tmp/keystore.jks \
      --ks-key-alias "${{ secrets.ANDROID_KEY_ALIAS }}" \
      --ks-pass="pass:${{ secrets.ANDROID_KEYSTORE_PASSWORD }}" \
      --key-pass="pass:${{ secrets.ANDROID_KEY_PASSWORD }}"
    ```
    Результат — каталог `crocson-apks/splits/`:
    - `base-master.apk` (базовый)
    - `base-armeabi_v7a.apk` (arm)
    - `base-arm64_v8a.apk` (arm64)
    - `base-x86.apk` (386)
    - `base-x86_64.apk` (amd64)

13. **Архивировать каталог APK** в tar для upload-artifact:
    ```bash
    tar czf crocson-apks.tar.gz crocson-apks/
    ```
14. **Cleanup:** `rm -f /tmp/keystore.jks`
15. **Upload artifacts:**
    - `crocson.aab`
    - `crocson-apks.tar.gz`

### Secrets (те же что в fyne.yml)
- `ANDROID_SIGNING_KEY`, `ANDROID_KEY_ALIAS`, `ANDROID_KEYSTORE_PASSWORD`, `ANDROID_KEY_PASSWORD`

## Файл

Создать `.github/workflows/aab.yml` на основе `build-android` job из `.github/workflows/fyne.yml`, упрощённый до одного job без create-release.
