# План: починить 403 на загрузке пакета в Huawei OBS

## Симптом

Run `28409104947`, шаг `AppGallery — upload (draft)` упал на PUT в Huawei OBS:
```
OBS upload URL obtained (objectId=RU/2026062923/...apk)
curl: (22) The requested URL returned error: 403
```
Все предыдущие шаги прошли (token OK, upload-url получен, fileType=5).

## Корневая причина

Способ передачи заголовков OBS в curl. Текущая конструкция:
```bash
mapfile -t OBS_HEADERS < <(echo "$UP" | jq -r '.urlInfo.headers | to_entries[] | "\(.key): \(.value)"')
curl -fsS -X PUT "$UPLOAD_URL" "${OBS_HEADERS[@]/#/-H }" --upload-file "$FILE"
```

`"${OBS_HEADERS[@]/#/-H }"` разворачивается в **склеенный** argv на каждый
заголовок, напр. один argv `"-H Authorization: AWS SIG=="`. curl трактует это как
short-option `-H` со склеенным значением ` Authorization: AWS SIG==` (с
пробелом/искажением) → заголовок `Authorization` уходит битым → подпись OBS не
сходится → `403`.

Оригинальный action (`28404573191`) прошёл этот PUT, потому что передавал
заголовки как JS-объект в `axios.put(url, body, {headers})` — без склейки.

## Исправление

Файл: `.github/workflows/aab.yml`, шаг `AppGallery — upload (draft)`.

Заменить блок (строки ~438–442):
```bash
          mapfile -t OBS_HEADERS < <(echo "$UP" | jq -r '.urlInfo.headers | to_entries[] | "\(.key): \(.value)"')
          curl -fsS -X PUT "$UPLOAD_URL" \
            "${OBS_HEADERS[@]/#/-H }" \
            --upload-file "$FILE"
          echo "File uploaded to OBS"
```
на:
```bash
          HARGS=()
          while IFS= read -r h; do HARGS+=("-H" "$h"); done < \
            <(echo "$UP" | jq -r '.urlInfo.headers | to_entries[] | "\(.key): \(.value)"')
          echo "OBS headers: $(echo "$UP" | jq -r '.urlInfo.headers | keys | join(", ")')"
          curl -fsS -X PUT "$UPLOAD_URL" \
            "${HARGS[@]}" \
            --upload-file "$FILE"
          echo "File uploaded to OBS"
```

`HARGS` = `("-H" "Authorization: AWS SIG==" "-H" "Content-Type: ..." ...)` —
чистые раздельные argv, как ждет curl. `-X PUT` оставлен для ясности
(`--upload-file` и так делает PUT).

### Диагностика (опционально)

Строка `echo "OBS request headers: ..."` печатает **только имена** заголовков
(без значений — подпись не утекает), чтобы при повторном сбое видеть, какие
заголовки Huawei прислал. Если опция избыточна — убрать.

## Проверка

1. Перезапустить workflow (`arm64=true`, `upload-appgallery=true`).
2. `File uploaded to OBS` без `403`, затем `app-file-info ret.code=0`, далее
   `newFeatures [...]` по 5 языкам и (если `appgallery-submit=true`) `app-submit`.

## Риски

- Не подтверждено, что причина именно в склейке (но это единственное отличие от
  работающего action). Если после фикса `403` повторится — добавить `-v` (stderr)
  к curl временно для деталей OBS; возможно потребуется `--data-binary @-` вместо
  `--upload-file`, но это запасной вариант.
