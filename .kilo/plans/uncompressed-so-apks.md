# Plan: Сборка .apks с несжатыми .so (пост-обработка bundletool)

## Проблема
`bundletool build-apks` сжимает нативные `.so` в split APK, несмотря на `android:extractNativeLibs="false"` в манифесте. Флаг `--system-apk-options=UNCOMPRESSED_NATIVE_LIBRARIES` работает только в режиме `SYSTEM`.

## Решение
Пост-обработка `.apks`: распаковать каждый split APK, пересобрать с несжатыми `.so`, переподписать.

## toc.pb
Не трогаем. Он содержит метаданные о вариантах (ABI, density и т.д.) и не зависит от сжатия внутри APK. Перепаковываем `.apks` как zip с исходным `toc.pb` и обновлёнными split APK.

## Шаг для workflow (заменяет шаг `bundletool — all ABIs APKs`)

```yaml
      - name: bundletool — all ABIs APKs (uncompressed .so)
        if: ${{ inputs.build-apks }}
        run: |
          cd $GITHUB_WORKSPACE/workspace/crocson

          # 1. Генерируем .apks через bundletool (как раньше)
          java -jar /tmp/bundletool-all-1.18.1.jar build-apks \
            --bundle=crocson.aab \
            --output=crocson.apks \
            --ks=/tmp/keystore.jks \
            --ks-key-alias "${{ secrets.ANDROID_KEY_ALIAS }}" \
            --ks-pass="pass:${{ secrets.ANDROID_KEYSTORE_PASSWORD }}" \
            --key-pass="pass:${{ secrets.ANDROID_KEY_PASSWORD }}"

          # 2. Распаковываем .apks
          mkdir apks-work
          cd apks-work
          unzip ../crocson.apks

          # 3. Для каждого split APK: пересобираем с несжатыми .so
          for apk in splits/*.apk; do
            mkdir apk-tmp
            cd apk-tmp
            unzip ../"$apk"
            # Пересобираем: .so без сжатия, всё остальное как есть
            zip -r ../"$apk" . -x 'lib/*' -x 'lib/**'
            # Добавляем lib/ с несжатыми .so
            if [ -d "lib" ]; then
              find lib -type f -exec zip -0 ../"$apk" {} \;
            fi
            cd ..
            rm -rf apk-tmp
          done

          # 4. Переподписываем каждый APK
          for apk in splits/*.apk; do
            zipalign -f 4 "$apk" "$apk.aligned"
            apksigner sign \
              --ks /tmp/keystore.jks \
              --ks-key-alias "${{ secrets.ANDROID_KEY_ALIAS }}" \
              --ks-pass "pass:${{ secrets.ANDROID_KEYSTORE_PASSWORD }}" \
              --key-pass "pass:${{ secrets.ANDROID_KEY_PASSWORD }}" \
              "$apk.aligned"
            mv "$apk.aligned" "$apk"
          done

          # 5. Собираем .apks обратно (с тем же toc.pb)
          cd ..
          rm crocson.apks
          cd apks-work
          zip -r ../crocson.apks toc.pb splits/
          cd ..
          rm -rf apks-work

          ls -la crocson.apks
```

## Зависимости
- `zipalign` — входит в Android SDK Build Tools (уже установлен `build-tools;36.0.0`)
- `apksigner` — тоже в build-tools (`$ANDROID_HOME/build-tools/36.0.0/apksigner`)

## Что не меняется
- `bundletool — universal APK` — оставляем как есть (universal APK тоже содержит сжатые .so, но это полная версия для одного ABI, вопрос к пользователю — обрабатывать ли её тоже)
- `toc.pb` — берём из оригинального .apks как есть
- Все остальные шаги workflow — без изменений

## Вопрос пользователю
Обрабатывать ли universal APK тем же способом (несжатые .so)?
