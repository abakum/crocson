# PEPK: два отдельных артефакта вместо одного zip

## Проблема

`upload-artifact@v4` упаковывает оба файла в один zip (`rustore-signing.zip`).
Нужно два отдельных скачиваемых артефакта.

## Решение

Разделить один шаг Upload на два — каждый загружает один файл:

```yaml
- name: Upload pepk output
  uses: actions/upload-artifact@v4
  with:
    name: pepk-output
    path: pepk_out.zip
    retention-days: 7

- name: Upload upload.pem
  uses: actions/upload-artifact@v4
  with:
    name: upload-pem
    path: rustore/upload.pem
    retention-days: 7
```

## Результат

При скачивании — два отдельных артефакта:
1. `pepk-output` → внутри `pepk_out.zip` (зашифрованный ключ + сертификат)
2. `upload-pem` → внутри `upload.pem`
