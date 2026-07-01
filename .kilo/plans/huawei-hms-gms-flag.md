# План: Проблема 1 — AppGallery «has not integrated HMS / cannot be used on HMS devices»

## Постановка Huawei
> Your app has been approved. Note: Your app has not integrated HMS and cannot be
> used or displayed on HMS devices. Test details: Startup failed.

## Диагноз (почему HMS Core НЕ нужен)
Фраза «не интегрировано HMS» — не требование тащить HMS SDK. Это результат
автотеста Huawei: приложение запустили на устройстве **без GMS**, оно «упало»
(по их интерпретации), и Huawei классифицировал пакет как GMS-зависимый →
скрыл с HMS-устройств. Сам «Startup failed» — не реальный краш, а следствие
этой классификации/тестового стенда Huawei.

Доказательства, что GMS/HMS приложению не нужны:
- `go.mod`: нет firebase / play-services / gms / hms (проверено `grep`).
- В исходниках `.go/.java/.xml` нет обращений к `com.google.android.gms` / HMS Kit.
- Чистый Go/Fyne + NativeActivity + OpenGL ES — рендерится самостоятельно.
- Запускается на **AOSP-эмуляторе без GMS** (по слову пользователя).
- ELF `.so` уже выровнен `0x4000` (16KB) в `crocson-arm64.apk` (readelf): краш
  на старте из-за выравнивания исключён (фикс `huawei-launch-crash.md` уже в билде).

Следствие: принудительная установка HMS Core пользователям не требуется.
Нужно лишь **правдиво заявить** Huawei, что приложение не зависит от GMS —
выставить флаг `needGms=0`. После этого пакет станет отображаться/запускаться
на HMS-устройствах.

## Ключевой API
`PUT /api/publish/v2/properties/gms?appId=<APP_ID>` (AppGalleryConnect.txt:4005-4159)
- Header: `client_id`, `Authorization: Bearer <token>`
- Body: `{"needGms":0}`  (0 = не зависит от GMS → доступно на HMS-устройствах)
- Ответ: `ret.code=0` — успех.

## Важно: атрибут уровня приложения, не версии
`properties/gms` принимает **только `appId`**, без `versionId`
(AppGalleryConnect.txt:4096-4102, 4128). Поэтому:
- Флаг меняет **глобальную** GMS-классификацию приложения в профиле AppGallery
  Connect (действует на профиль и все будущие версии).
- **Уже одобренную/отправленную версию** он переоценивает НЕ мгновенно: Huawei
  применяет классификацию при проверке/релизе версии. Значит:
  - выставить флаг в профиле можно в любой момент (п.2/п.3);
  - но чтобы пометка «cannot be used on HMS devices» ушла с уже опубликованной
    версии — нужен **повторный submit текущей версии** (через `appgallery-submit`
    или кнопку в консоли) ИЛИ **новый релиз** с уже выставленным флагом. Именно
  поэтому шаг в CI (п.1) обязателен — флаг ставится на каждый билд.

Порядок: (а) выставить `needGms=0` → (б) переотправить текущую версию / выпустить
новую → Huawei переоценит → HMS-устройства увидят пакет.

## Изменения

### 1. (обязательно) Шаг в CI — `.github/workflows/aab.yml`
В шаге `AppGallery — upload (draft)` (`.github/workflows/aab.yml:334`), после
успешного `app-file-info` (после строки 411 `echo "Huawei: $FILE uploaded as draft ..."`),
добавить объявление флага, чтобы оно сохранялось при каждой заливке. Флаг уровня
приложения (только `appId`), но выставляется на каждой заливке, чтобы новый релиз
гарантированно нёс `needGms=0`:

```bash
          # Объявляем, что приложение НЕ зависит от GMS (HMS-устройства видят пакет).
          # Чистый Go/Fyne, нет firebase/play-services — needGms=0 правдив.
          GMS=$(curl -fsS -X PUT "$BASE/publish/v2/properties/gms?appId=$HUAWEI_APP_ID" \
            "${AUTH[@]}" \
            -H 'Content-Type: application/json' \
            -d '{"needGms":0}')
          echo "set gms flag response:"
          echo "$GMS" | jq .
          GMS_RET=$(echo "$GMS" | jq -r '.ret.code')
          if [ "$GMS_RET" != "0" ]; then
            echo "WARNING: properties/gms returned ret.code=$GMS_RET (flag can be set manually in AppGallery Connect)"
          fi
```

Семантика: не-фатальный WARNING при ошибке (как у `appgallery-submit`), чтобы
загрузка пакета засчиталась; причина видна в логе.

### 2. (опционально) Через UI AppGallery Connect
AppGallery Connect → crocson → «Информация о приложении» (или раздел
«Настройки GMS» / GMS dependency) → снять/выключить «Зависит от GMS» → Сохранить.
Эквивалентно `needGms=0`. Само по себе, как и флаг, не перевыпустит уже одобренную
версию — нужно переотправить текущую версию или выпустить новый релиз (см. блок
«Важно»). Основной путь — шаг в CI (п.1), UI как страховка при ошибке рет-кода.

## Что НЕ делаем
- НЕ интегрируем HMS Core SDK — приложение его не использует.
- НЕ добавляем HMS-пермишены/метаданные в AndroidManifest.xml.
- НЕ трогаем запуск/рендер — на AOSP всё работает.

## Риск / открытые вопросы
- `needGms=0` — правдивая декларация (зависимостей действительно нет).
- Флаг уровня приложения, но срабатывает на витрине при переоценке версии
  (повторный submit / новый релиз) — см. блок «Важно».
- Если после `needGms=0` и переотправки Huawei снова покажет «Startup failed» в
  реальном Cloud-тесте — запрашивать logcat у поддержки Huawei по заявке (это уже
  серверный/тестовый стенд, не код приложения). На локальном AOSP краша нет.
- `properties/gms` при ошибке рет-кода — выставить через UI (п.2).

## Проверка
1. Запустить CI с п.1 → в логе шага `properties/gms` `ret.code=0`, в профиле
   приложения «GMS dependency» = No / «Зависит от GMS» = выключено.
2. Переотправить версию (`appgallery-submit=true` или кнопкой в консоли) —
   новый релиз нёс уже выставленный флаг, Huawei переоценит его.
3. Дождаться ревью/переоценки: приложение должно стать доступным/отображаемым на
   HMS-устройствах (исчезнет пометка «cannot be used or displayed on HMS devices»).
