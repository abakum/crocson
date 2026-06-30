# План: подписать arm64 APK upload-ключом Huawei (rustore-upload.keystore)

## Корневая причина (подтверждено)

Huawei App Signing: при включённом «Подписании приложения» Huawei проверяет
**транспортную подпись** загруженного пакета зарегистрированным upload-ключом и
переподписывает финальную сборку своим ключом.

В `aab.yml` сейчас асимметрия:
- **AAB** (`fyne release`, строка ~173) подписан **`rustore-upload.keystore`**
  → проходит проверку Huawei → работает.
- **arm64 APK** (`Sign ARM64 APK`, строка ~153) подписан **`keystore.jks`
  (`ANDROID_SIGNING_KEY`)** → НЕ совпадает с upload-ключом Huawei →
  «Сбой компиляции пакета crocson.apk. Выгрузите пакет повторно».

Значит, upload-ключ Huawei = `rustore-upload.keystore` (тот же, что и для AAB).

## Решение

Подписать **Huawei-копию** arm64 APK ключом `rustore-upload.keystore`. RuStore и
артефакт остаются на `ANDROID_SIGNING_KEY` (их не трогаем — там всё работает).

### Изменение 1 — декодировать rustore keystore в пути arm64

Шаг `Decode RuStore upload keystore` сейчас выполняется только при
`!inputs.arm64`. Добавить условие для Huawei:
```yaml
        if: ${{ !inputs.arm64 || (inputs.arm64 && inputs.upload-appgallery) }}
```

### Изменение 2 — пере-подписать копию APK для Huawei

В шаге `AppGallery — upload (draft)`, в начале, после выбора файла, при
`arm64=true` создать и пере-подписать Huawei-копию:

```bash
          if [ "${{ inputs.arm64 }}" = "true" ]; then
            FILE="crocson.apk"
            # Пере-подписываем upload-ключом Huawei (rustore-upload.keystore),
            # иначе Huawei App Signing отклоняет пакет (сбой компиляции).
            BUILD_TOOLS="$ANDROID_HOME/build-tools/36.0.0"
            cp "$FILE" "crocson-huawei.apk"
            "$BUILD_TOOLS/apksigner" sign \
              --ks /tmp/rustore-upload.keystore \
              --ks-key-alias "${{ secrets.RUSTORE_KEY_ALIAS }}" \
              --ks-pass "pass:${{ secrets.RUSTORE_UPLOAD_KEYSTORE_PASSWORD }}" \
              --key-pass "pass:${{ secrets.RUSTORE_KEY_PASSWORD }}" \
              "crocson-huawei.apk"
            echo "Re-signed crocson-huawei.apk with upload key"
            FILE="crocson-huawei.apk"
          else
            FILE="crocson.aab"
          fi
          FILE_TYPE=5
```

> Примечание: `RUSTORE_KEY_PASSWORD` — если такого секрета нет, использовать тот
> же пароль, что и в `fyne release` AAB (там передаётся только `--keystore-pass`,
> без `--key-pass`). В keystore ключ/хранилище могут иметь общий пароль — тогда
> `--key-pass` = `RUSTORE_UPLOAD_KEYSTORE_PASSWORD`. Уточнить по факту (см.
> «Открытый вопрос»).

Дальше весь upload-флоу (upload-url → OBS PUT → app-file-info → newFeatures →
compile-status → submit) использует `$FILE` = `crocson-huawei.apk`.

### Изменение 3 — поллинг статуса компиляции вместо слепого sleep 300

Из дока: `GET /publish/v2/package/compile/status?appId=&pkgIds=` (pkgIds — это
`pkgVersion` из ответа `app-file-info`, у нас было
`pkgVersion:["1983941021091778240"]`). Ответ `successStatus`: 0=ok, 1=parsing,
2=failed.

После `app-file-info`:
1. Достать `PKG=$(echo "$INFO" | jq -r '.pkgVersion[0]')`.
2. Цикл (≈15 попыток × 30с): запрос статуса → `successStatus`. Если 0 → готово
   (break). Если 2 → `echo ERROR compile failed` + выход (не submit). Если 1 →
   sleep.
3. Только при `successStatus=0` → newFeatures уже проставлены ранее → `app-submit`.

Это заменяет `sleep 300` + однократный submit: теперь submit идёт только когда
пакет реально скомпилирован, а при провале — явная ошибка с кодом.

## Открытый вопрос

Пароль ключа (`--key-pass`) для `rustore-upload.keystore`: шаг AAB передаёт
только `--keystore-pass` (`RUSTORE_UPLOAD_KEYSTORE_PASSWORD`). Если у ключа тот
же пароль — добавить `--key-pass "pass:$RUSTORE_UPLOAD_KEYSTORE_PASSWORD"`. Если
отдельный — нужен секрет `RUSTORE_KEY_PASSWORD`. Уточнить у владельца keystore.

## Что НЕ меняется

- RuStore-шаг (APK подписан ANDROID_SIGNING_KEY — работает).
- Сборка/подпись arm64 APK для RuStore/артефакта (keystore.jks).
- Логика versionCode (AAB=Build−1, arm64 APK=Build+1).

## Проверка

1. Запуск `arm64=true, upload-appgallery=true`. В логе:
   `Re-signed crocson-huawei.apk with upload key` → upload → `app-file-info
   ret.code=0` → поллинг `successStatus=0` → `newFeatures [...] ret=0` →
   `app-submit ret.code=0`.
2. В консоли Huawei: пакет 1.11.71 (vc 1981) скомпилирован без «Сбоя».
3. Если `successStatus=2` — в CI будет явная ошибка вместо немого зависания.

## Риски

- Если upload-ключ Huawei ≠ rustore-upload.keystore (маловероятно: AAB с ним
  работает) — пере-подписка не поможет; тогда нужен отдельный Huawei upload-key.
- 16КБ-выравнивание .so: AAB с ним работал → и APK (тот же .so) выровнен; не
  причина.
- Пере-подписка уже подписанного APK: apksigner корректно заменяет v1/v2/v3.
