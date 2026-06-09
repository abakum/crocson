# Plan: Сборка .apks через --device-spec со всеми ABI и sdkVersion=23

## Идея
Использовать `--device-spec` с `sdkVersion: 23` и перечислить все ABI в `supportedAbis`. При `sdkVersion >= 23` bundletool генерирует split APK с несжатыми `.so`. Имена файлов будут чистыми (`base-arm64_v8a.apk`), `toc.pb` корректным.

## Что НЕ меняется
- Шаг `bundletool — universal APK` (`--mode=universal`) — остаётся как есть
- Все остальные шаги workflow

## Что меняется
Только шаг `bundletool — all ABIs APKs` — добавляем `--device-spec`:

```yaml
      - name: bundletool — all ABIs APKs
        if: ${{ inputs.build-apks }}
        run: |
          cd $GITHUB_WORKSPACE/workspace/crocson

          cat > /tmp/device-all.json << 'EOF'
          {"supportedAbis":["arm64-v8a","armeabi-v7a","x86","x86_64"],"screenDensity":640,"supportedLocales":["en"],"sdkVersion":23}
          EOF

          java -jar /tmp/bundletool-all-1.18.1.jar build-apks \
            --bundle=crocson.aab \
            --output=crocson.apks \
            --device-spec=/tmp/device-all.json \
            --ks=/tmp/keystore.jks \
            --ks-key-alias "${{ secrets.ANDROID_KEY_ALIAS }}" \
            --ks-pass="pass:${{ secrets.ANDROID_KEYSTORE_PASSWORD }}" \
            --key-pass="pass:${{ secrets.ANDROID_KEY_PASSWORD }}"

          rm -f /tmp/device-all.json
          ls -la crocson.apks
          echo "Contents of crocson.apks:"
          unzip -l crocson.apks
```

## Риск
Если bundletool с `--device-spec` и несколькими ABI сгенерирует split только для первого совместимого ABI (а не для всех), то понадобится подход 2: отдельный вызов на каждый ABI с объединением результатов. Но это выяснится только при запуске workflow.

## Файлы
- `.github/workflows/aab.yml` — заменить шаг `bundletool — all ABIs APKs` (строки 133–144)
