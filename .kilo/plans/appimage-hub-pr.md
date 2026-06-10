# Plan: Workflow `appimage.yml` для PR в AppImage Hub

## Что делает workflow

Ручной запуск (`workflow_dispatch`), который создаёт PR в `AppImage/appimage.github.io` с файлом `data/crocson`.

Файл `data/crocson` содержит одну строку:
```
https://github.com/abakum/crocson/
```

## Шаги workflow

1. **Fork** `AppImage/appimage.github.io` (если ещё не форкнут) через `gh repo fork`
2. **Clone** форка, создать ветку `add-crocson`
3. **Создать** файл `data/crocson` с URL репозитория
4. **Commit, push** в форк
5. **Создать PR** из форка в `AppImage/appimage.github.io:master`

## Требования

- **PAT** (Personal Access Token) с правом `public_repo` — нужен для fork и PR в чужой репозиторий
- Секрет в репозитории crocson: `APPIMAGEHUB_TOKEN`
- `GITHUB_TOKEN` (автоматический) НЕ подходит для cross-repo операций

## Примечания

- Workflow запускается **вручную** — это разовая операция
- Если файл `data/crocson` уже есть upstream — `gh pr create` вернёт ошибку, это нормально
- PR проверяется автоматически через test.yml в appimage.github.io (worker.sh скачает последний AppImage с Releases и провалидирует)

## Файл

`.github/workflows/appimage.yml`
