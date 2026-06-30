# План: arm64 → AAB(транспорт) для Huawei + APK(приложение) для RuStore

## Идея (от пользователя)

В режиме `arm64=true` собирать **два одноплатформенных (только arm64) артефакта**:

| Артефакт | Подпись | versionCode | Магазин |
|----------|---------|-------------|---------|
| `crocson.aab` (arm64-only) | транспортная (`rustore-upload.keystore`) | Build − 1 | **Huawei AppGallery** |
| `crocson.apk` (arm64) | приложения (`keystore.jks` / `ANDROID_SIGNING_KEY`) | Build + 1 | **RuStore** |

Почему так:
- RuStore принимает APK, подписанный ключом приложения (работает).
- Huawei (App Signing) ожидает пакет, подписанный транспортным/upload-ключом →
  AAB подписан `rustore-upload.keystore` (как и полный AAB, который работает).
- Оба пакета arm64-only → маленький universal APK у магазинов (без 4-ABI и без
  ~170 МБ после 16КБ-выравнивания).

Подтверждено в исходнике fyne (`abakum/tools/cmd/fyne`):
`build.go:45` принимает `android/arm64`; `build_androidapp.go:126` ставит
`ext=".aab"` при `release`. Значит `fyne release -os android/arm64` даёт
**arm64-only AAB**.

## Изменения в `.github/workflows/aab.yml`

### 1. Включить bundletool и rustore-keystore в режиме arm64

- Шаг `Setup bundletool` (`if: ${{ !inputs.arm64 }}`) → убрать условие (нужно
  для `fyne release` AAB и в arm64-режиме).
- Шаг `Decode RuStore upload keystore` (`if: ${{ !inputs.arm64 }}`) → тоже
  убрать условие (нужно для подписи arm64-only AAB транспортным ключом).

### 2. Шаг `fyne package — build ARM64 APK` → собирать и AAB, и APK

Переименовать в `Build arm64 AAB + APK`, `if: ${{ inputs.arm64 }}`. В одном
`run` захватить исходный `BUILD_NUMBER` один раз и собрать оба пакета с
корректными versionCode:

```bash
          cd "$GITHUB_WORKSPACE/workspace/crocson"
          BUILD_NUMBER=$(grep -E '^\s*Build\s*=' FyneApp.toml | sed -E 's/^\s*Build\s*=\s*([0-9]+).*/\1/')

          # --- arm64-only AAB (транспортная подпись, vc=Build-1) → Huawei ---
          AAB_VC=$((BUILD_NUMBER - 1))
          sed -i "s/^Build = .*/Build = $AAB_VC/" FyneApp.toml
          fyne release --os android/arm64 \
            --keystore /tmp/rustore-upload.keystore \
            --key-name "${{ secrets.RUSTORE_KEY_ALIAS }}" \
            --keystore-pass "${{ secrets.RUSTORE_UPLOAD_KEYSTORE_PASSWORD }}"
          [ -f crocson.aab ] || { echo "ERROR: crocson.aab not created"; ls -la .; exit 1; }
          echo "arm64-only AAB built (vc=$AAB_VC)"

          # --- arm64 APK (vc=Build+1) → RuStore, подпишется app-ключом далее ---
          APK_VC=$((BUILD_NUMBER + 1))
          sed -i "s/^Build = .*/Build = $APK_VC/" FyneApp.toml
          export CGO_ENABLED=1 GOOS=android GOARCH=arm64
          fyne package -os android/arm64 --release
          [ -f crocson.apk ] || { echo "ERROR: crocson.apk not created"; ls -la .; exit 1; }
          echo "arm64 APK built (vc=$APK_VC)"
```

Захват `BUILD_NUMBER` один раз — гарантирует, что AAB=Build−1, APK=Build+1 от
одного исходного значения (иначе второй шаг прочитал бы уже изменённый Build).

Шаг `Sign ARM64 APK` (`if: ${{ inputs.arm64 }}`) — **без изменений**: подписывает
`crocson.apk` ключом приложения (`keystore.jks`) для RuStore.

### 3. Шаг `AppGallery — upload (draft)` — всегда AAB

Сейчас выбирает `crocson.apk` при arm64. Заменить на постоянный AAB (в обоих
режимах теперь есть `crocson.aab`):

```bash
          FILE="crocson.aab"
          FILE_TYPE=5
```

(Убрать ветку `if arm64 → crocson.apk`.) Дальше upload-url → OBS → app-file-info →
newFeatures → submit — без изменений (всё уже отлажено на AAB).

### 4. Шаг `RuStore — upload (draft)` — без изменений

Уже выбирает `crocson.apk` при `arm64=true` и `crocson.aab` при `arm64=false`.
В arm64-режиме RuStore получит app-key APK — корректно.

### 5. Артефакты (опционально)

В arm64-режиме теперь есть и `crocson.aab`, и `crocson.apk`. При желании
загружать оба как artifacts (сейчас `Upload crocson.aab` — `if: !arm64`, а
`Upload crocson-arm64.apk` — `if: arm64`). Можно оставить как есть.

## Не меняется

- `fyne release — build AAB` (`if: !arm64`) — полный AAB для не-arm64 режима.
- versionCode-схема: AAB=Build−1, arm64 APK=Build+1, полный AAB=Build−1.
- `fyne.yml` — без изменений.

## Риски / действия

- **Монотонность versionCode в Huawei:** ранее туда ушёл (и упал) APK vc 1981.
  Новый AAB будет vc 1979 (Build−1) — **меньше** 1981. Huawei может отклонить
  как даунгрейд, даже если неудачный черновик удалить. Решение: перед запуском
  удалить в консоли Huawei зависший/упавший пакет vc 1981 и/или поднять `Build`
  так, чтобы `Build−1` > макс. ранее загруженного в Huawei кода.
- `fyne release -os android/arm64` теоретически может иметь особенности сборки;
  на первом запуске проверить, что `crocson.aab` создался и содержит только
  arm64 (`bundletool`/`unzip -l` — `lib/arm64-v8a`).
- Время сборки в arm64-режиме растёт (AAB + APK), но оба одноплатформенные —
  приемлемо.

## Проверка

1. Запуск `arm64=true, upload-rustore=true, upload-appgallery=true`:
   - `arm64-only AAB built (vc=1979)` + `arm64 APK built (vc=1981)`.
   - RuStore: `crocson.apk` (vc 1981, app-key) — ОК.
   - Huawei: `crocson.aab` (vc 1979, transport-key) → `app-file-info ret=0` →
     компиляция без сбоя → `app-submit ret=0`.
2. В консоли Huawei universal APK для 1.11.71 — заметно меньше ~170 МБ (только
   arm64).
