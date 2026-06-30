# План: публиковать RuStore через AAB (в т.ч. arm64-only)

## Контекст
В `arm64`-режиме RuStore сейчас получает `crocson.apk` (vc=Build+1), а Huawei/AppGallery —
arm64-only `crocson.aab` (vc=Build-1, transport-подписан `rustore-upload.keystore`).
Решено: arm64-only AAB (одна `.so`) публикуется во **все** сторы. APK-путь становится ненужным.

Во **всех** режимах сборки (arm64 и нет) уже строится `crocson.aab`, transport-подписанный
`rustore-upload.keystore` — он подходит и для RuStore, и для AppGallery.

## Изменения в `.github/workflows/aab.yml`

### 1. Шаг «RuStore — upload (draft)» (≈ строки 279-286)
Убрать ветку `if [ arm64 ] -> APK / else AAB`. Всегда брать AAB:
```sh
FILE="$GITHUB_WORKSPACE/workspace/crocson/crocson.aab"
FILE_TYPE="aab"
```
(остальная логика RuStore — auth/draft/upload/commit/verify — без изменений,
 т.к. она уже параметризована `$FILE_TYPE` и поддерживает `aab`)

### 2. Шаг «Build arm64 AAB + APK» (≈ 104-147)
Оставить только сборку arm64-only AAB (vc=Build-1). Удалить блок сборки
arm64-APK (vc=Build+1), включая `fyne package -os android/arm64` и проверку `crocson.apk`.
Назвать шаг «Build arm64-only AAB».

### 3. Удалить шаг «Sign ARM64 APK» (≈ 149-169)
Он подписывал только arm64-APK, который больше не нужен.

### 4. Шаг «Decode keystore» (≈ 94-97)
Расшифровка `/tmp/keystore.jks` теперь нужна только для bundletool (build-apk/build-apks).
Изменить условие с
`inputs.arm64 || inputs.build-apk || inputs.build-apks`
на
`inputs.build-apk || inputs.build-apks`.

### 5. Артефакты (≈ 234-268)
- «Upload crocson.aab»: условие `!inputs.arm64 && inputs.build-aab` → `inputs.build-aab`
  (AAB артефакт теперь выгружается и в arm64-режиме).
- Удалить шаг «Upload crocson-arm64.apk» (≈ 252-259).
- Остальные (`crocson-all.apk`, `crocson.apks`) без изменений — они и так gated на `!inputs.arm64`.

### 6. Описание входа `arm64` (≈ строки 22-25)
«ARM64 only (APK instead of AAB)» → «ARM64 only AAB (single .so)».

## Замечания
- **versionCode**: RuStore получит vc=Build-1 (как у AAB), ранее APK шёл с vc=Build+1.
  Huawei и RuStore — разные магазины, общего пространства versionCode нет, конфликта не будет.
- AppGallery-шаг уже использует `crocson.aab` во всех режимах — правок не требует.
- Шаги bundletool (build-apk/build-apks) и `Setup bundletool` не трогаются.
