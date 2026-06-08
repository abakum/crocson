# Plan: Sign AAB with RuStore upload key, keep APK signing as-is

## Goal
Modify `.github/workflows/aab.yml` so that:
- AAB is signed with the RuStore upload key (`RUSTORE_UPLOAD_KEYSTORE` / `RUSTORE_UPLOAD_KEYSTORE_PASSWORD`)
- APKs (universal + arm64) continue to be signed with the original `ANDROID_SIGNING_KEY` as before

## Changes

### 1. Add step after "Decode keystore": "Decode RuStore upload keystore"
Insert a new step to decode `RUSTORE_UPLOAD_KEYSTORE` to `/tmp/rustore-upload.keystore`. Keep the existing "Decode keystore" step untouched — APKs still need it.

### 2. Update "fyne release — build AAB" (lines 82-84)
- `--keystore /tmp/rustore-upload.keystore`
- `--key-name upload`
- `--keystore-pass "${{ secrets.RUSTORE_UPLOAD_KEYSTORE_PASSWORD }}"`

### 3. "bundletool — universal APK" and "bundletool — arm64 APK" — NO CHANGES
These stay on `/tmp/keystore.jks` with `ANDROID_*` secrets.

### 4. Update cleanup in arm64 step (line 132)
Add `rm -f /tmp/rustore-upload.keystore` alongside existing `rm -f /tmp/keystore.jks`.

## Secrets required (new)
- `RUSTORE_UPLOAD_KEYSTORE` — base64-encoded `upload.keystore`
- `RUSTORE_UPLOAD_KEYSTORE_PASSWORD` — password for keystore and key
- `RUSTORE_KEY_ALIAS` — alias ключа загрузки (значение: `upload`)

### Change in `aab.yml`
Replace hardcoded `upload` in `--key-name` with `${{ secrets.RUSTORE_KEY_ALIAS }}`

---

## New workflow: `.github/workflows/pepk.yml`

### Goal
Create a `workflow_dispatch` workflow that runs `pepk.jar` to generate `pepk_out.zip` (app signing export for RuStore). The keystore/alias/password come from the same secrets used to sign APKs in `aab.yml` (`ANDROID_SIGNING_KEY`, `ANDROID_KEY_ALIAS`, `ANDROID_KEYSTORE_PASSWORD`). The `encryptionkey` is entered manually at run time via `workflow_dispatch` input.

### Workflow structure

```yaml
name: PEPK Export

on:
  workflow_dispatch:
    inputs:
      encryptionkey:
        description: 'RuStore encryption key'
        required: true

jobs:
  pepk:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up JDK 21
        uses: actions/setup-java@v4
        with:
          java-version: '21'
          distribution: 'temurin'

      - name: Decode keystore
        run: echo "${{ secrets.ANDROID_SIGNING_KEY }}" | base64 -d > /tmp/keystore.jks

      - name: Run pepk.jar
        run: |
          java -jar rustore/pepk.jar \
            --keystore=/tmp/keystore.jks \
            --alias="${{ secrets.ANDROID_KEY_ALIAS }}" \
            --output=pepk_out.zip \
            --encryptionkey="${{ github.event.inputs.encryptionkey }}" \
            --include-cert
        env:
          JAVA_TOOL_OPTIONS: "-Djava.awt.headless=true"

      - name: Upload rustore signing artifacts
        uses: actions/upload-artifact@v4
        with:
          name: rustore-signing
          path: |
            pepk_out.zip
            rustore/upload.pem
          retention-days: 7

      - name: Cleanup
        if: always()
        run: rm -f /tmp/keystore.jks
```

### Key details
- **Keystore**: decoded from `ANDROID_SIGNING_KEY` (same as APK signing in `aab.yml`)
- **Alias**: `ANDROID_KEY_ALIAS` secret
- **Password**: pepk will prompt for keystore password via stdin — need to pipe it (`echo "$PASSWORD" | java -jar ...`) or use `--storepass` if pepk supports it. Since pepk.jar reads password interactively, we pipe it: `echo "${{ secrets.ANDROID_KEYSTORE_PASSWORD }}" | java -jar rustore/pepk.jar ...`
- **encryptionkey**: entered manually when triggering the workflow (copied from RuStore Console)
- **pepk.jar**: stored in repo at `rustore/pepk.jar`
- **Artifact**: single artifact `rustore-signing` containing both `pepk_out.zip` and `rustore/upload.pem`. `upload-artifact@v4` does not re-zip individual files — they are stored as-is in the artifact, so no double-packing
