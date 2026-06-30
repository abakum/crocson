# План: app-submit — минимальный подход («до утра»)

## Контекст

Run `28409708928`: пакет + локализованные `newFeatures` (5 языков, ret=0)
загружены в Huawei как черновик; компиляция на стороне Huawei заняла дольше
ожидания, поэтому `app-submit` вернул `204144727` «package is being compiled».

Документ «Obtaining the Callback Result» относится к **download-mode** submit
(`app-submit-with-file` / `app-package-file/by-url` — асинхронные, с
`callbackAddr`). Наш путь — обычный `app-submit` (синхронный ответ `ret.code`),
поэтому callback нам не нужен. Полезное из доки: Huawei рекомендует проверять
статус (`releaseState=4` = отправлено на проверку), а не спамить submit.

## Решение (вместо 15-мин retry-цикла)

Не держать runner ~15 мин в поллинге. Пакет уже черновик и скомпилируется сам
за минуты; submit делаем **одной попыткой** (короткое ожидание + 1 запрос),
ошибку «компилируется» оставляем не-фатальной (WARNING). Если с первой попытки
не успели — отправить на ревью утром вручную (одна кнопка в консоли Huawei) либо
перезапустить шаг позже.

## Изменение

Файл: `.github/workflows/aab.yml`, блок submit в шаге `AppGallery — upload (draft)`.

Оставить текущий однократный submit (уже реализовано): короткое ожидание + 1
запрос + WARNING при не-нуле. НЕ добавлять retry-цикл.

Т.е. блок остаётся таким (без цикла `for ... seq 1 15`):
```bash
          if [ "${{ inputs.appgallery-submit }}" = "true" ]; then
            echo "Waiting 300s — Huawei compiles the package asynchronously..."
            sleep 300
            echo "Submitting app for review..."
            SUB=$(curl -fsS -X POST "$BASE/publish/v2/app-submit?appId=$HUAWEI_APP_ID" \
              "${AUTH[@]}" \
              -H 'Content-Type: application/json' \
              -d '{}')
            echo "app-submit response:"
            echo "$SUB" | jq .
            SUB_RET=$(echo "$SUB" | jq -r '.ret.code')
            if [ "$SUB_RET" != "0" ]; then
              echo "WARNING: app-submit returned ret.code=$SUB_RET — if 'being compiled', submit manually in AppGallery Connect later."
            fi
          fi
```

(Это фактически текущее состояние в `aab.yml` — план сводится к «не менять на
retry-цикл».)

## Проверка / действия утром

1. Если с авто-попытки submit не прошёл (ret=204144727 «being compiled») —
   черновик уже готов в консоли Huawei: **AppGallery Connect → crocson →
   Управление версиями → 1.11.71 → Submit/Отправить на ревью** (одна кнопка).
2. Альтернатива утром — перезапустить workflow с тем же `appgallery-submit=true`:
   пакет перезальётся и после 5 мин submit пройдёт (т.к. повторной долгой
   компиляции не будет).

## Риски

- Однократная попытка может поймать «being compiled» → нужен ручной submit.
  Это приемлемо для подхода «до утра».
- При желании полностью автоматизировать позже — реализовать поллинг эндпоинта
  «Querying the Compilation Status of an App Package» (видели в навигации) до
  `ret.code=0`, а не слепой retry `app-submit`. Это отдельная задача.
