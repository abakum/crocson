# PEPK: заменить input encryptionkey на secret RUSTORE_ENCRYPTION_KEY

## Изменения в `.github/workflows/pepk.yml`

1. Удалить `inputs.encryptionkey` из `workflow_dispatch` — блок `inputs` убрать полностью
2. Заменить `${{ github.event.inputs.encryptionkey }}` на `${{ secrets.RUSTORE_ENCRYPTION_KEY }}` в шаге "Run pepk.jar"

## Требуется вручную

- Добавить secret `RUSTORE_ENCRYPTION_KEY` в Settings → Secrets and variables → Actions репозитория GitHub со значением публичного ключа RuStore
