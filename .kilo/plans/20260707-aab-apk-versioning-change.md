# План: Изменение схемы версионирования AAB и APK

## Цель
Изменить схему versionCode для AAB и APK:
- **AAB**: `Build` (вместо `Build - 1`)
- **APK**: `Build + 1, Build + 2, Build + 3, Build + 4` (вместо `Build, Build + 1, Build + 2, Build + 3`)

## Текущая схема
- AAB versionCode: `BUILD - 1` (строка 118, 139 в `.github/workflows/aab.yml`)
- APK versionCodes: `BUILD, BUILD+1, BUILD+2, BUILD+3` (строки 60 в `metadata/generate_recipe.sh`)
- F-Droid VercodeOperation: `'%c', '%c + 1', '%c + 2', '%c + 3'`
- F-Droid CurrentVersionCode: `BUILD + 3`

Пример для Build=1985:
- AAB: 1984
- APK: 1985, 1986, 1987, 1988
- CurrentVersionCode: 1988

## Желаемая схема
- AAB versionCode: `BUILD`
- APK versionCodes: `BUILD + 1, BUILD + 2, BUILD + 3, BUILD + 4`
- F-Droid VercodeOperation: `'%c + 1', '%c + 2', '%c + 3', '%c + 4'`
- F-Droid CurrentVersionCode: `BUILD + 4`

Пример для Build=1985:
- AAB: 1985
- APK: 1986, 1987, 1988, 1989
- CurrentVersionCode: 1989

## Изменения

### 1. `.github/workflows/aab.yml`
**Строка 118**: Изменить `AAB_VC=$((BUILD_NUMBER - 1))` → `AAB_VC=$BUILD_NUMBER`
**Строка 139**: Изменить `VC=$((BUILD_NUMBER - 1))` → `VC=$BUILD_NUMBER`

Это установит AAB versionCode = Build (вместо Build - 1).

### 2. `metadata/generate_recipe.sh`
**Строки 49-53** (в `TAIL`): Изменить VercodeOperation с:
```yaml
VercodeOperation:
  - '%c'
  - '%c + 1'
  - '%c + 2'
  - '%c + 3'
```
на:
```yaml
VercodeOperation:
  - '%c + 1'
  - '%c + 2'
  - '%c + 3'
  - '%c + 4'
```

**Строка 60**: В функции `generate_builds()` изменить логику versionCode:
- Текущая: `VC=$((BUILD + OFF))` с `OFF=0` для первого билда
- Новая: `VC=$((BUILD + OFF + 1))` (начинаем с Build+1)

**Строка 109**: Изменить `CurrentVersionCode: $((BUILD + 3))` → `CurrentVersionCode: $((BUILD + 4))`

### 3. `metadata/com.github.abakum.crocson.yml`
Перегенерировать скриптом после изменений в `generate_recipe.sh` или обновить вручную:

**Строки 128-132**: Изменить VercodeOperation:
```yaml
VercodeOperation:
  - '%c + 1'
  - '%c + 2'
  - '%c + 3'
  - '%c + 4'
```

**Строка 135**: Обновить CurrentVersionCode (например, с 1988 на 1989 для Build=1985).

## Порядок выполнения
1. Изменить `.github/workflows/aab.yml`
2. Изменить `metadata/generate_recipe.sh`
3. Перегенерировать `metadata/com.github.abakum.crocson.yml` запуском `metadata/generate_recipe.sh`
4. Проверить изменения во всех трех файлах

## Валидация
- Убедиться, что AAB versionCode равен Build
- Убедиться, что APK versionCodes начинаются с Build+1
- Убедиться, что VercodeOperation соответствует новой схеме
- Убедиться, что CurrentVersionCode равен Build+4

## Последствия
- F-Droid будет использовать новую схему VercodeOperation
- Выпуски AAB и APK будут иметь непересекающиеся versionCode
- История versionCode прервется (AAB будет использовать значения, которые ранее использовали APK)