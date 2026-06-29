# План: чекбокс «сразу подать Huawei на проверку»

## Цель

Добавить опциональный флаг, чтобы после загрузки пакета в Huawei сразу вызвать
`app-submit` (отправка черновика на ревью). Пользователь хочет попробовать —
возможно, явный submit «протолкнёт» зависший на обработке этап.

## Изменение 1 — новый input

Файл: `.github/workflows/aab.yml`, блок `inputs:` (после `upload-appgallery`,
строки 31–35).

```yaml
      appgallery-submit:
        description: 'AppGallery: submit for review right after upload'
        required: false
        default: false
        type: boolean
```

## Изменение 2 — вызов app-submit в curl-шаге

В шаге `AppGallery — upload (draft)`, после успешного `app-file-info`
(после строки `echo "Huawei: $FILE uploaded as draft ..."`), добавить:

```bash
          if [ "${{ inputs.appgallery-submit }}" = "true" ]; then
            echo "Submitting app for review..."
            SUB=$(curl -fsS -X POST "$BASE/publish/v2/app-submit?appId=$HUAWEI_APP_ID" \
              "${AUTH[@]}" \
              -H 'Content-Type: application/json' \
              -d '{}')
            echo "app-submit response:"
            echo "$SUB" | jq .
            SUB_RET=$(echo "$SUB" | jq -r '.ret.code')
            if [ "$SUB_RET" != "0" ]; then
              echo "WARNING: app-submit returned ret.code=$SUB_RET (version may need release notes / required fields in AppGallery Connect)"
            fi
          fi
```

Обоснование по эндпоинту: `POST /publish/v2/app-submit?appId=` с пустым телом
`{}` — ровно то, что делает action в `src/api/publish.ts:submitApp`.

### Семантика / реверс

- Флаг **не делает шаг ошибочным** при неудаче submit (`WARNING`, `exit 0`) —
  чтобы загрузка пакета засчиталась, а причина отказа submit была видна в логе
  (обычно Huawei требует заполнить релизные заметки / инфо версии в Connect).
- По умолчанию `false` — текущее поведение (только черновик) сохраняется.

## Риск / ожидания

- `app-submit` не «разгребает» серверную обработку уже загруженного файла: если
  пакет реально завис на валидации подписи/манифеста, submit вернёт ошибку вида
  «version is being processed» или «incomplete». Это и будет полезной
  диагностикой вместо немого «На обработке».
- Для новой загрузки (корректный `fileType`) submit должен пройти, если в
  AppGallery Connect заполнены обязательные поля версии.

## Проверка

1. Запуск `arm64=true, upload-appgallery=true, appgallery-submit=true` →
   `app-submit` вызывается, в логе виден `ret.code` и сообщение.
2. `ret.code=0` — пакет ушёл на ревью.
3. `ret.code!=0` — лог покажет, чего не хватает (релизные заметки и т.п.).
