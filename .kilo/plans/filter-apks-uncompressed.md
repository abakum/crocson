# Plan: Фильтрация .apks — оставить только вариант с несжатыми .so

## Контекст
bundletool `build-apks` генерирует несколько вариантов (variants) split APK:
- **Variant 0** (`*_0.apk`): standalone APK для pre-L (API < 21) — сжатые .so
- **Variant 1** (`*_1.apk`): split APK для API 21–22 — сжатые .so  
- **Variant 2** (`*_2.apk`): split APK для API 23+ — **несжатые .so** ✓
- **Variant 3** (`base-master_3.apk`): master split для API 23+ — **несжатые .so** ✓

Нам нужны только variant 2 (ABI splits) и variant 3 (master). Названия файлов после переименования:
- `base-master_3.apk` → `base-master.apk`
- `base-arm64_v8a_2.apk` → `base-arm64_v8a.apk`
- `base-armeabi_v7a_2.apk` → `base-armeabi_v7a.apk`
- `base-x86_2.apk` → `base-x86.apk`
- `base-x86_64_2.apk` → `base-x86_64.apk`

## toc.pb
Это `BuildApksResult` protobuf (schema: `commands.proto`). Содержит:
- `variant[]` — список вариантов с `variant_number`, `targeting`, `apk_set[]`
- Каждый `apk_set[].apk_description[].path` — путь к файлу APK

Нужно:
1. Удалить variants 0 и 1 (сжатые .so)
2. Обновить `path` в оставшихся variant 2 и 3 (убрать суффиксы `_2`, `_3`)
3. Перенумеровать `variant_number` чтобы были последовательными (0, 1 вместо 2, 3) — **или оставить как есть**, bundletool при `extract-apks` ищет по targeting, а не по номеру

Лучше оставить номера как есть — меньше правок в protobuf, и bundletool `install-apks` / `extract-apks` работает по targeting, не по порядковому номеру.

## Шаг для workflow (заменяет шаг `bundletool — all ABIs APKs`)

```yaml
      - name: bundletool — all ABIs APKs (uncompressed .so only)
        if: ${{ inputs.build-apks }}
        run: |
          cd $GITHUB_WORKSPACE/workspace/crocson

          java -jar /tmp/bundletool-all-1.18.1.jar build-apks \
            --bundle=crocson.aab \
            --output=crocson.apks \
            --ks=/tmp/keystore.jks \
            --ks-key-alias "${{ secrets.ANDROID_KEY_ALIAS }}" \
            --ks-pass="pass:${{ secrets.ANDROID_KEYSTORE_PASSWORD }}" \
            --key-pass="pass:${{ secrets.ANDROID_KEY_PASSWORD }}"

          mkdir apks-work
          cd apks-work
          unzip ../crocson.apks

          echo "Before cleanup:"
          find splits -type f -name '*.apk' | sort

          # Удаляем сжатые варианты (variant 0 и 1)
          rm -f splits/base-master_1.apk splits/base-master_2.apk
          rm -f splits/*_0.apk splits/*_1.apk

          # Переименовываем несжатые
          mv splits/base-master_3.apk splits/base-master.apk
          for abi in arm64_v8a armeabi_v7a x86 x86_64; do
            mv "splits/base-${abi}_2.apk" "splits/base-${abi}.apk"
          done

          echo "After cleanup:"
          find splits -type f -name '*.apk' | sort

          # Обновляем toc.pb через Python (protobuf text format → binary)
          python3 "$GITHUB_WORKSPACE/workspace/crocson/fix_toc_pb.py" toc.pb

          cd ..
          rm crocson.apks
          cd apks-work
          zip -r ../crocson.apks toc.pb splits/
          cd ..
          rm -rf apks-work

          ls -la crocson.apks
```

## Скрипт fix_toc_pb.py

Python-скрипт для обновления `toc.pb`:
1. Декодирует `BuildApksResult` из binary protobuf
2. Удаляет variants 0 и 1
3. Обновляет `path` в оставшихся variant 2 и 3 (убирает суффиксы `_2`, `_3`)
4. Кодирует обратно в binary protobuf

Используем `protoc --decode_raw` / `protoc --encode` или чистый Python с `google.protobuf` (установлен в GitHub Actions runner).

Альтернатива — использовать `sed` для binary-безопасной замены строк в protobuf. Но это ненадёжно.

### Вариант:protoc с .proto файлами

Лучший подход — скачать `commands.proto` и его зависимости из репозитория bundletool, затем:
```bash
protoc --decode android.bundle.BuildApksResult commands.proto < toc.pb > toc.json
# Правим JSON
protoc --encode android.bundle.BuildApksResult commands.proto < toc_fixed.json > toc_new.pb
```

Но это сложно из-за зависимостей proto-файлов.

### Вариант: Python с protobuf

На GitHub Actions runner `pip install protobuf` доступен. Но нужен скомпилированный `_pb2.py`.

### Самый простой вариант: Python raw protobuf manipulation

Написать скрипт на Python, который парсит binary protobuf напрямую (wire format) и модифицирует нужные поля. Это можно сделать без `.proto` файлов.

```python
#!/usr/bin/env python3
"""Fix toc.pb: remove compressed variants, rename uncompressed ones."""
import sys
import struct

def read_varint(data, pos):
    result = 0
    shift = 0
    while True:
        b = data[pos]
        result |= (b & 0x7F) << shift
        pos += 1
        if not (b & 0x80):
            break
        shift += 7
    return result, pos

def write_varint(value):
    out = b''
    while value > 0x7F:
        out += bytes([(value & 0x7F) | 0x80])
        value >>= 7
    out += bytes([value & 0x7F])
    return out

# ... (полная реализация protobuf wire format парсера)
```

Это overengineering. Есть более простой путь.

### Самый простой вариант: pip install protobuf + runtime descriptor

На GitHub Actions можно:
```bash
pip install protobuf
wget https://raw.githubusercontent.com/google/bundletool/master/src/main/proto/commands.proto
wget https://raw.githubusercontent.com/google/bundletool/master/src/main/proto/targeting.proto
wget https://raw.githubusercontent.com/google/bundletool/master/src/main/proto/config.proto
wget https://raw.githubusercontent.com/google/bundletool/master/src/main/proto/device_targeting_config.proto
protoc --python_out=. commands.proto targeting.proto config.proto device_targeting_config.proto
python3 fix_toc_pb.py
```

### Или ещё проще: вообще не трогать toc.pb

Если мы не планируем использовать `bundletool extract-apks` или `bundletool install-apks` с этим `.apks` файлом, а просто распространяем его как архив для ручной установки через `adb install-multiple`, то `toc.pb` не нужен.

`adb install-multiple` принимает список APK напрямую, без `.apks` обёртки.

## Рекомендация

Самый чистый подход:
1. bundletool генерирует `.apks`
2. Распаковываем, удаляем сжатые варианты, переименовываем несжатые
3. **Не трогаем toc.pb** — просто пересобираем `.apks` zip с обновлёнными файлами
4. Пользователь при необходимости достаёт APK из `.apks` вручную или через `bundletool extract-apks` (в этом случае `toc.pb` будет содержать устаревшие пути, но можно документировать)

**Или** распространять не `.apks`, а просто zip с переименованными APK — без `toc.pb` вообще.

## Решение пользователя
Нужно уточнить у пользователя — будет ли использоваться `bundletool install-apks` / `extract-apks`, или `.apks` — это просто архив для распространения.
