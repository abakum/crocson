# Plan: Replace arm64 APK with APKS for all ABIs

## Goal
In `.github/workflows/aab.yml`, replace the arm64-only APK step (`bundletool — arm64 APK`) with a step that generates split APKs for **all** ABIs (armeabi-v7a, arm64-v8a, x86, x86_64).

## Changes

### 1. Replace step "bundletool — arm64 APK" (lines 115–137)

**Before:** Creates a device-spec JSON for arm64 only, then uses `--device-spec=/tmp/device-arm64.json`, and zips only `base-master.apk` + `base-arm64_v8a.apk`.

**After:** Run `bundletool build-apks` **without** `--device-spec` and with `--output-format=DIRECTORY` — this generates split APKs for all supported ABIs. Then zip `base-master.apk` + all ABI split APKs (`base-arm64_v8a.apk`, `base-armeabi_v7a.apk`, `base-x86.apk`, `base-x86_64.apk`).

New step content:
```yaml
      - name: bundletool — all ABIs APKs
        run: |
          cd $GITHUB_WORKSPACE/workspace/crocson
          java -jar /tmp/bundletool-all-1.18.1.jar build-apks \
            --bundle=crocson.aab \
            --output-format=DIRECTORY \
            --output=crocson-allabis-dir \
            --ks=/tmp/keystore.jks \
            --ks-key-alias "${{ secrets.ANDROID_KEY_ALIAS }}" \
            --ks-pass="pass:${{ secrets.ANDROID_KEYSTORE_PASSWORD }}" \
            --key-pass="pass:${{ secrets.ANDROID_KEY_PASSWORD }}"
          echo "All ABI split APKs:"
          find crocson-allabis-dir/splits -type f -name '*.apk' | sort
          cd crocson-allabis-dir
          zip -r $GITHUB_WORKSPACE/workspace/crocson/crocson-allabis.zip splits
          cd $GITHUB_WORKSPACE/workspace/crocson
          rm -rf crocson-allabis-dir
          rm -f /tmp/keystore.jks /tmp/rustore-upload.keystore
          ls -la crocson-allabis.zip
```

Key changes:
- Removed `--device-spec` — now generates splits for all ABIs
- Removed the device-spec JSON file creation
- Changed output dir from `crocson-arm64-dir` → `crocson-allabis-dir`
- **Zip entire `splits/` directory** instead of listing individual APK files — simpler and future-proof
- Output file: `crocson-allabis.zip`

### 2. Update upload step (lines 157–164)

Rename artifact from `crocson-arm64.zip` → `crocson-allabis.zip`:

```yaml
      - name: Upload crocson-allabis.zip
        if: success() || failure()
        uses: actions/upload-artifact@v4
        with:
          name: crocson-allabis.zip
          path: ${{ github.workspace }}/workspace/crocson/crocson-allabis.zip
          retention-days: 7
          if-no-files-found: error
```

## Files modified
- `.github/workflows/aab.yml` — 2 edits (step content + upload artifact name)
