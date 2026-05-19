# План публикации crocson на F-Droid

### 1. Создать git-тег для текущей версии
```bash
git tag v1.11.57
git push origin v1.11.57
```
YAML рецепт ссылается на `commit: v1.11.57` — тег должен существовать.

### 2. Форкнуть fdroiddata на GitLab
1. Зайти на `https://gitlab.com/fdroid/fdroiddata`
2. Нажать **Fork**
3. Клонировать свой форк локально

### 3. Добавить YAML рецепт
```bash
# В форке fdroiddata:
cp /path/to/crocson/metadata/com.github.abakum.crocson.yml metadata/
```

### 4. Создать Merge Request
Заголовок: **New app: com.github.abakum.crocson**

Описание:
```
### crocson

A GUI for [croc](https://github.com/schollz/croc) — a tool for secure file transfer between any two computers.
Fork of howeyc/crocgui with additional features.

**Why fork dependencies:**
- `github.com/abakum/tools` — patched Fyne CLI with Android fixes not yet in upstream
- `github.com/abakCroc/croc` — patched croc with features needed by the GUI
- `github.com/abakum/peerdiscovery` — patched for Android compatibility

All dependencies are FOSS. License: ISC.
```

### 5. Пройти ревью F-Droid maintainers
Ревьюеры могут попросить:
- Уточнить параметры сборки — NDK версия, build-скрипт
- Объяснить каждый форк зависимости
- Проверить отсутствие бинарников — `I_trust_the_signer_of_this.exe` и `croc.cer` уже перенесены в отдельный репозиторий

### 6. Публикация
После одобрения MR — приложение автоматически появится в F-Droid в течение следующего цикла сборки.

---

## При обновлении версии

1. Обновить `Version` и `Build` в `FyneApp.toml`
2. Создать `changelogs/<Version>.txt` с текстом ченжлога
3. Обновить `metadata/com.github.abakum.crocson.yml`:
   - Добавить 4 новых записи в `Builds:` с новыми `versionCode` — инкремент от предыдущего
   - Обновить `CurrentVersion` и `CurrentVersionCode`
4. Создать `metadata/en-US/changelogs/<versionCode>.txt` для каждого ABI — скопировать из `changelogs/<Version>.txt`
5. Создать git-тег `v<Version>`
6. Обновить рецепт в форке `fdroiddata`

---

## Полезные ссылки

- [F-Droid Inclusion Policy](https://f-droid.org/en/docs/Inclusion_Policy/)
- [F-Droid Build Metadata Reference](https://f-droid.org/en/docs/Build_Metadata_Reference/)
- [Fastlane Structure](https://f-droid.org/en/docs/All_About_Descriptions_Graphics_and_Screenshots/)
- [fdroiddata repository](https://gitlab.com/fdroid/fdroiddata)
- [Inclusion How-To](https://f-droid.org/en/docs/Submitting_to_F-Droid_Quick_Start_Guide/)
