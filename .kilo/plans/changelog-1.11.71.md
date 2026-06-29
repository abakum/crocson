# План: changelog 1.11.71.txt для всех локалей

## Цель

Добавить файл `1.11.71.txt` в `changelogs/` для всех 5 локалей метадаты.
Текст (от пользователя): «Тест рецепта публикации на AppGallery».

## Переводы

- **ru-RU** (исходник): `Тест рецепта публикации на AppGallery`
- **en-US**: `AppGallery publishing recipe test`
- **ja-JP**: `AppGallery公開レシピのテスト`
- **tr-TR**: `AppGallery yayınlama reçetesi testi`
- **zh-CN**: `AppGallery 发布流程测试`

## Изменения

Создать по одному файлу (однострочное содержание, без завершающего пробела):

| Файл | Содержание |
|------|------------|
| `metadata/ru-RU/changelogs/1.11.71.txt` | `Тест рецепта публикации на AppGallery` |
| `metadata/en-US/changelogs/1.11.71.txt` | `AppGallery publishing recipe test` |
| `metadata/ja-JP/changelogs/1.11.71.txt` | `AppGallery公開レシピのテスト` |
| `metadata/tr-TR/changelogs/1.11.71.txt` | `AppGallery yayınlama reçetesi testi` |
| `metadata/zh-CN/changelogs/1.11.71.txt` | `AppGallery 发布流程测试` |

Формат соответствует существующим `1.11.70.txt` (короткий текст, без заголовка
версии;wfстрока/несколько строк).

## Связь с релизом

- В `FyneApp.toml` теперь `Version = "1.11.71"`, `Build = 1980`.
- RuStore-шаг в `aab.yml` (строка ~268) берёт ченджлог именно по пути
  `metadata/ru-RU/changelogs/${VERSION_NAME}.txt` → будет использован новый
  `ru-RU/1.11.71.txt`. Файлы других локалей — для консолей/релиз-ноутов.
- Huawei: ченджлог в curl-шаг сейчас не передаётся (submit без release notes);
  при необходимости добавить отдельно — следующей задачей.
