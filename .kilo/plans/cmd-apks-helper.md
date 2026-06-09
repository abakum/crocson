# Plan: cmd/apks/main.go — очистка .apks от сжатых variants

## Задача
Написать `cmd/apks/main.go` — хелпер, который принимает `.apks` файл, удаляет сжатые варианты, переименовывает несжатые, обновляет `toc.pb`, записывает результат как `crocson2.apks` (или имя из `-o`).

## Входной .apks (плохой)
Содержит несколько variant:
- `splits/base-master_1.apk` — сжатый master (variant 1)
- `splits/base-master_2.apk` — сжатый master (variant 2)
- `splits/base-master_3.apk` — **несжатый master** ✓ (variant 3)
- `splits/base-arm64_v8a_0.apk` — standalone (variant 0)
- `splits/base-arm64_v8a_1.apk` — сжатый (variant 1)
- `splits/base-arm64_v8a_2.apk` — **несжатый** ✓ (variant 2)
- Аналогично для armeabi_v7a, x86, x86_64

## Выходной .apks (хороший)
Только несжатые, переименованные:
- `splits/base-master.apk`
- `splits/base-arm64_v8a.apk`
- `splits/base-armeabi_v7a.apk`
- `splits/base-x86.apk`
- `splits/base-x86_64.apk`
- `toc.pb` — обновлённый

## Алгоритм

1. Распаковать входной `.apks` (zip) во временную директорию
2. Прочитать и распарсить `toc.pb` (protobuf binary wire format)
3. Найти variants с `variant_number >= 2` (это несжатые)
4. В найденных variants обновить `apk_description[].path`:
   - `_2.apk` → `.apk`
   - `_3.apk` → `.apk`
5. Удалить variants с `variant_number < 2`
6. Пересобрать `toc.pb`
7. В `splits/`:
   - Удалить `*_0.apk`, `*_1.apk`, `base-master_1.apk`, `base-master_2.apk`
   - Переименовать `base-<abi>_2.apk` → `base-<abi>.apk`
   - Переименовать `base-master_3.apk` → `base-master.apk`
8. Запаковать в выходной `.apks`

## Proto wire format парсинг (без внешних зависимостей)

Только `encoding/binary` и ручной парсинг, как в `cmd/fix-cd-extra/main.go`.

Wire types:
- 0 (VARINT): varint
- 2 (LEN): length-delimited (string, bytes, embedded message)

Структура `BuildApksResult`:
```
field 1 (LEN, repeated): Variant
field 2 (LEN): Bundletool
field 4 (LEN): string package_name
```

Структура `Variant`:
```
field 1 (LEN): VariantTargeting
field 2 (LEN, repeated): ApkSet
field 3 (VARINT): uint32 variant_number
field 4 (LEN): VariantProperties
```

Структура `ApkSet`:
```
field 1 (LEN): ModuleMetadata
field 2 (LEN, repeated): ApkDescription
```

Структура `ApkDescription`:
```
field 1 (LEN): ApkTargeting
field 2 (LEN): string path
field 3-9: oneof metadata
```

### Подход к парсингу/модификации

Полный парс/сериализация protobuf wire format:
1. `parseMessage(data) → []fieldEntry` — парсит raw bytes в список `{fieldNum, wireType, rawValue}`
2. `serializeMessage(entries []fieldEntry) → []byte` — обратно в binary
3. Для каждого Variant: извлечь `variant_number` (field 3, VARINT)
4. Для Variant с `variant_number >= 2`: пройтись по ApkSet → ApkDescription → path (field 2) и заменить суффикс
5. Собрать сообщение заново только с нужными variants

## Usage
```bash
go run ./cmd/apks crocson.apks                  # → crocson2.apks
go run ./cmd/apks -o output.apks crocson.apks   # → output.apks
```

## Тестирование
На первом этапе — на образце `crocson.apks` из `cmd/apks/`:
```bash
cd cmd/apks
go run main.go crocson.apks
unzip -l crocson2.apks  # проверить содержимое
```

## Файлы
- Новый: `cmd/apks/main.go` — без внешних зависимостей
