# План: публикация в Huawei (fileType + пауза + локализованные release notes)

> Все значения подтверждены официальной докой Huawei. Это финальная версия.

## Подтверждённые значения (из доки Huawei)

| Назначение | Эндпоинт / поле | Значение |
|------------|-----------------|----------|
| Пакет APK **и AAB** | `app-file-info`, `fileType` | **5** (одно значение для обоих) |
| Submit | `app-submit` | body `{}`; **пауза 5 мин** после app-file-info (дока: 2–5 мин асинхр. парсинг, берём максимум) |
| «новые функции» (release notes) | `app-language-info` PUT, поле **`newFeatures`** (≤500 симв.) | `{lang, newFeatures}` — отдельный запрос на каждый язык |
| Коды языков | пример доки `zh-CN` | совпадают с папками метадаты (`ru-RU`, `en-US`, `ja-JP`, `tr-TR`, `zh-CN`) |

## Исправление 1 — fileType=5 (блокирует загрузку AAB)

Файл: `.github/workflows/aab.yml`, шаг `AppGallery — upload (draft)`.

```diff
           if [ "${{ inputs.arm64 }}" = "true" ]; then
             FILE="crocson.apk"
-            FILE_TYPE=5
           else
             FILE="crocson.aab"
-            FILE_TYPE=13      # неверно: AAB тоже 5
           fi
+          FILE_TYPE=5         # APK и AAB — один код (подтверждено докой)
```

## Исправление 2 — локализованные newFeatures (5 языков)

После успешного `app-file-info`, ДО блока submit. Читаем ченджлоги версии и
шлём `app-language-info` для каждого языка:

```bash
          FYNEAPP="$GITHUB_WORKSPACE/workspace/crocson/FyneApp.toml"
          VERSION_NAME=$(grep -E '^\s*Version\s*=' "$FYNEAPP" | sed -E 's/^\s*Version\s*=\s*"([^"]+)".*/\1/')
          for LOC in ru-RU en-US ja-JP tr-TR zh-CN; do
            NOTES_FILE="$GITHUB_WORKSPACE/workspace/crocson/metadata/$LOC/changelogs/${VERSION_NAME}.txt"
            if [ ! -f "$NOTES_FILE" ]; then
              echo "newFeatures [$LOC]: skip (no changelog file)"
              continue
            fi
            NOTES=$(cat "$NOTES_FILE")
            RESP=$(curl -fsS -X PUT "$BASE/publish/v2/app-language-info?appId=$HUAWEI_APP_ID&releaseType=1" \
              "${AUTH[@]}" -H 'Content-Type: application/json' \
              -d "$(jq -cn --arg lang "$LOC" --arg nf "$NOTES" '{lang:$lang, newFeatures:$nf}')")
            echo "newFeatures [$LOC] ret=$(echo "$RESP" | jq -r '.ret.code') $(echo "$RESP" | jq -r '.ret.msg // ""')"
          done
```

## Исправление 3 — пауза 2 мин перед app-submit

В существующем блоке `if [ "${{ inputs.appgallery-submit }}" = "true" ]` добавить
`sleep 120` перед `app-submit`:

```bash
          if [ "${{ inputs.appgallery-submit }}" = "true" ]; then
            echo "Waiting 300s — Huawei parses the package asynchronously before submit (doc: 2-5 min, using 5)..."
            sleep 300
            echo "Submitting app for review..."
            SUB=$(curl -fsS -X POST "$BASE/publish/v2/app-submit?appId=$HUAWEI_APP_ID" \
              "${AUTH[@]}" -H 'Content-Type: application/json' -d '{}')
            echo "app-submit response:"
            echo "$SUB" | jq .
            SUB_RET=$(echo "$SUB" | jq -r '.ret.code')
            if [ "$SUB_RET" != "0" ]; then
              echo "WARNING: app-submit returned ret.code=$SUB_RET"
            fi
          fi
```

## Итоговый порядок шага AppGallery (полный скелет)

1. Выбор FILE по `arm64`; `FILE_TYPE=5`.
2. Проверка файла, size, sha256.
3. token → `upload-url/for-obs`.
4. PUT в OBS.
5. PUT `app-file-info` (fileType=5) → проверка `ret.code=0`.
6. Цикл `app-language-info` (newFeatures по 5 языкам) — **исправление 2**.
7. Если `appgallery-submit`: `sleep 120` → POST `app-submit` — **исправления 2+3**.

## Зависимости / порядок внедрения

1. Сначала `changelog-1.11.71.md` (создать 5 файлов `1.11.71.txt`) — без них
   newFeatures некуда читать.
2. Затем правки `aab.yml` (1–3).

## Риски

- `FILE_TYPE=13` для AAB → неверная регистрация → «На обработке» навсегда
  (исправление 1 закрывает).
- Submit без паузы → ret.code != 0 «package not parsed» (исправление 3, пауза 5 мин, закрывает).
- PUT `app-language-info` с одним `newFeatures`: если язык уже существует —
  частичное обновление OK (appName обязателен только при добавлении нового
  языка, по доке). Если Huawei трактует PUT как полную замену записи —
  потребуется read-modify-write (GET текущих appName/appDesc → PUT с ними + newFeatures).
  На первый запуск смотреть `ret.code` по каждому языку.
- `newFeatures` ≤500 символов — наши ченджлоги короткие, в лимит укладываются.
