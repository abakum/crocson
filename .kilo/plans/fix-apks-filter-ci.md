# Plan: Fix apks-filter for CI output

## Problem
On CI, `bundletool build-apks --device-spec` produces a single compressed variant (variant_number=0, uncompressed_native_libraries=false) with only 3 files. The filter expects an uncompressed variant to exist.

Root cause: `--device-spec` mode does not produce uncompressed `.so` variants. That only happens in default mode (without `--mode` or `--device-spec`).

## Solution: Remove `--device-spec` from bundletool build-apks step

Remove `--device-spec=/tmp/device-all.json` from the `bundletool build-apks` command so it generates ALL variants (compressed + uncompressed), like the local build does. Then the filter step will correctly find and keep the uncompressed variant.

### Changes to `.github/workflows/aab.yml`

In the "bundletool — all ABIs APKs" step, change:
```yaml
          java -jar /tmp/bundletool-all-1.18.1.jar build-apks \
            --bundle=crocson.aab \
            --output=crocson.apks \
            --device-spec=/tmp/device-all.json \
            --ks=/tmp/keystore.jks \
            --ks-key-alias "${{ secrets.ANDROID_KEY_ALIAS }}" \
            --ks-pass="pass:${{ secrets.ANDROID_KEYSTORE_PASSWORD }}" \
            --key-pass="pass:${{ secrets.ANDROID_KEY_PASSWORD }}"

          rm -f /tmp/device-all.json
```

To:
```yaml
          java -jar /tmp/bundletool-all-1.18.1.jar build-apks \
            --bundle=crocson.aab \
            --output=crocson.apks \
            --ks=/tmp/keystore.jks \
            --ks-key-alias "${{ secrets.ANDROID_KEY_ALIAS }}" \
            --ks-pass="pass:${{ secrets.ANDROID_KEYSTORE_PASSWORD }}" \
            --key-pass="pass:${{ secrets.ANDROID_KEY_PASSWORD }}"
```

Remove the `cat > /tmp/device-all.json` block and `rm -f /tmp/device-all.json` since they're no longer needed.

Also remove the `unzip -l crocson.apks` diagnostic from the bundletool step since the filter step already prints file listing.

This will produce the full set of variants (12 files, 3 variants) matching local output, and the filter step will correctly select variant 2.
