# План: публикация в Huawei AppGallery через curl (apk/aab по флагу arm64)

## Контекст и цель

Сейчас шаг `AppGallery — upload (draft)` использует сторонний action
`eatorb/huawei-appgallery-action@v1.0.0`. Его ограничения:

1. **Жёстко шлёт `fileType:"5"`** (код APK) — не годится для AAB (там другой код).
2. **Не отдаёт статус обработки** — поэтому пакет «висит На обработке» без
   понятной причины.
3. Непрозрачные ошибки (`Failed to get upload URL` и т.п.).

Цель — заменить action на curl-шаг в стиле RuStore, который:
- выбирает файл и `fileType` по флагу `inputs.arm64`:
  - `arm64=true` → `crocson.apk` (arm64, ~44 МБ), `fileType=5` (APK). Это
    обход проблемы, что AAB на стороне Huawei разворачивается в универсальный
    APK ~170 МБ из-за 16КБ-выравнивания.
  - `arm64=false` → `crocson.aab`, `fileType=13` (AAB, см. риск ниже).
- честно читает `ret.code` и тело ответа каждого шага;
- оставляет пакет черновиком (`submit=false`), как сейчас.

Ссылки на эндпоинты взяты из исходника action (`src/api/*.ts`),
`DOMAIN = https://connect-api.cloud.huawei.com/api`.

## Изменение

Файл: `.github/workflows/aab.yml`.

Полностью заменить шаг `AppGallery — upload (draft)` (сейчас строки ~373–381)
на curl-реализацию. Условие выполнения оставить `inputs.upload-appgallery`
(работает и для arm64, и для AAB — формат выбирается внутри).

```yaml
      - name: AppGallery — upload (draft)
        if: ${{ inputs.upload-appgallery }}
        env:
          HUAWEI_CLIENT_ID: ${{ secrets.APPGALLERY_CLIENT_ID }}
          HUAWEI_CLIENT_SECRET: ${{ secrets.APPGALLERY_CLIENT_SECRET }}
          HUAWEI_APP_ID: ${{ secrets.APPGALLERY_APP_ID }}
        run: |
          set -euo pipefail
          cd "$GITHUB_WORKSPACE/workspace/crocson"

          # Какой файл и какой fileType грузить
          if [ "${{ inputs.arm64 }}" = "true" ]; then
            FILE="crocson.apk"
            FILE_TYPE=5      # APK
          else
            FILE="crocson.aab"
            FILE_TYPE=13     # AAB (см. «Риск AAB fileType»)
          fi

          if [ ! -f "$FILE" ]; then
            echo "ERROR: $FILE не найден"
            ls -la .
            exit 1
          fi

          SIZE=$(wc -c < "$FILE" | tr -d ' ')
          SHA256=$(sha256sum "$FILE" | awk '{print $1}')
          echo "Загрузка $FILE ($SIZE байт, sha256=$SHA256) fileType=$FILE_TYPE"

          BASE="https://connect-api.cloud.huawei.com/api"

          # 1. OAuth-токен
          TOKEN=$(curl -fsS -X POST "$BASE/oauth2/v1/token" \
            -H 'Content-Type: application/json' \
            -d "$(jq -cn --arg cid "$HUAWEI_CLIENT_ID" \
                       --arg sec "$HUAWEI_CLIENT_SECRET" \
                  '{grant_type:"client_credentials", client_id:$cid, client_secret:$sec}')" \
            | jq -r '.access_token')
          if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
            echo "ERROR: Huawei auth failed"; exit 1
          fi
          echo "Auth OK"

          AUTH=(-H "Authorization: Bearer $TOKEN" -H "client_id: $HUAWEI_CLIENT_ID")

          # 2. Upload URL для OBS
          UP=$(curl -fsS -G "$BASE/publish/v2/upload-url/for-obs" \
            "${AUTH[@]}" \
            --data-urlencode "appId=$HUAWEI_APP_ID" \
            --data-urlencode "fileName=$FILE" \
            --data-urlencode "sha256=$SHA256" \
            --data-urlencode "contentLength=$SIZE" \
            --data-urlencode "releaseType=1")
          UPLOAD_URL=$(echo "$UP" | jq -r '.urlInfo.url')
          OBJECT_ID=$(echo "$UP" | jq -r '.urlInfo.objectId')
          if [ -z "$UPLOAD_URL" ] || [ "$UPLOAD_URL" = "null" ]; then
            echo "ERROR: upload-url failed: $UP"; exit 1
          fi
          echo "OBS upload URL obtained (objectId=$OBJECT_ID)"

          # 3. Загрузка файла в OBS (PUT бинарником + динамические заголовки)
          mapfile -t OBS_HEADERS < <(echo "$UP" | jq -r '.urlInfo.headers | to_entries[] | "\(.key): \(.value)"')
          curl -fsS -X PUT "$UPLOAD_URL" \
            "${OBS_HEADERS[@]/#/-H }" \
            --upload-file "$FILE"
          echo "File uploaded to OBS"

          # 4. Регистрация пакета (fileType: 5=apk, 13=aab)
          INFO=$(curl -fsS -X PUT "$BASE/publish/v2/app-file-info?appId=$HUAWEI_APP_ID" \
            "${AUTH[@]}" \
            -H 'Content-Type: application/json' \
            -d "$(jq -cn --arg ft "$FILE_TYPE" --arg fn "$FILE" --arg oid "$OBJECT_ID" \
                  '{fileType:$ft, files:[{fileName:$fn, fileDestUrl:$oid}]}')")
          echo "app-file-info response:"
          echo "$INFO" | jq .
          RET=$(echo "$INFO" | jq -r '.ret.code')
          if [ "$RET" != "0" ]; then
            echo "ERROR: app-file-info failed (ret.code=$RET)"
            exit 1
          fi

          # submit намеренно НЕ вызываем — пакет остаётся черновиком (как раньше)
          echo "Huawei: $FILE загружен как черновик (fileType=$FILE_TYPE)"
```

### Что меняется по сравнению с action

- Никакого `uses:` — только `curl`/`jq`/`sha256sum` (есть на ubuntu-latest).
- `fileType` корректный под формат (5/13), а не захардкоженный `5`.
- Передаётся честный `sha256` и `contentLength` (action тоже передаёт —
  поведение сохранено).
- Заголовки OBS берутся динамически из ответа `upload-url` (как в action), а не
  угадываются.
- Каждый шаг проверяет `ret.code`/наличие полей и падает с понятной причиной.
- Секреты те же: `APPGALLERY_CLIENT_ID`, `APPGALLERY_CLIENT_SECRET`,
  `APPGALLERY_APP_ID`.

## Риск: код fileType для AAB

- `5` (APK) — подтверждено исходником action и успешной ручной публикацией.
- `13` (AAB) — документированное значение Huawei Publishing API v2; лично
  проверить не удалось (документация Huawei рендерится JS и недоступна для
  чтения).
- **Примечание:** ранее «успешный AAB через action» технически регистрировался с
  `fileType=5` (action хардкодит 5). Значит Huawei могла принимать `.aab` и с
  кодом 5. Поэтому если с `13` пакет опять зависнет/упадёт на `app-file-info` —
  первым делом переключить AAB-ветку на `FILE_TYPE=5` (как было у action).
  План оставляет это как однострочное изменение.

## Что НЕ трогаем

- RuStore-шаг — без изменений (работает).
- Логику versionCode (AAB=Build−1, arm64 APK=Build+1) — без изменений.
- `fyne.yml` — без изменений.

## Про уже зависший черновик (vc 1976, APK)

curl-шаг не починит уже загруженный пакет — он на сервере Huawei. Варианты:
- дождаться перехода в «Отклонено» → появится кнопка удаления в
  «Управление версиями»;
- либо снять через поддержку Huawei.
После переключения на curl новые загрузки пойдут корректным форматом и не будут
зависать.

## Проверка

1. Запуск `arm64=true, upload-appgallery=true` → грузится `crocson.apk`,
   `fileType=5`; в логе `ret.code=0`, «загружен как черновик». В консоли Huawei
   пакет должен обработаться (как при ручной публикации APK).
2. Запуск `arm64=false, build-aab=true, upload-appgallery=true` → грузится
   `crocson.aab`, `fileType=13`. Если `ret.code != 0` или зависает «На обработке»
   — переключить на `FILE_TYPE=5` (см. риск).
3. В обоих случаях в логе видны тело `app-file-info` и `ret.code` — диагностика
   вместо чёрного ящика action.
