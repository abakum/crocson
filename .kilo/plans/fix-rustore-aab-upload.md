# Plan: Fix RuStore AAB Upload

## Problem

`Suro4ek/rustore-action@v0.0.1` creates a draft and reports "AAB uploaded successfully", but the AAB file is not actually attached to the draft in RuStore Console.

The action uses Node.js native `FormData` + `Blob` for file upload at [`dist/index.js`](https://github.com/Suro4ek/rustore-action/blob/v0.0.1/dist/index.js):
```js
const fileBuffer = fs.readFileSync(filePath);
const formData = new FormData();
formData.append('file', new Blob([fileBuffer]), fileName);
```
This doesn't correctly encode the multipart form data in Node.js 20 — the file part is empty/malformed, but the RuStore API still returns `code: 'OK'`.

Run log evidence (run #27383521791):
```
Draft created successfully, version ID: 2064653060
Uploading AAB: /home/runner/work/.../crocson.aab
AAB uploaded successfully          ← API returned OK, but file is empty/missing
Skipping submit (submit=false). Draft ID: 2064653060
```

## Solution

Replace `Suro4ek/rustore-action@v0.0.1` with direct `curl` calls to the RuStore API per [official docs](https://www.rustore.ru/help/work-with-rustore-api/api-upload-publication-app/apk-file-upload/file-upload-aab). The official curl example is:

```bash
curl --location --request POST \
  'https://public-api.rustore.ru/public/v1/application/com.package.example/version/123/aab' \
  --header 'Public-Token: {YOURtoken}' \
  --form 'file=@"/path/to/package.aab"'
```

## RuStore API Flow (from official docs)

### Step 1: Auth — `POST /public/auth/`

- Sign `keyId + timestamp` with SHA512withRSA using the private key (base64 PKCS#8)
- Request body: `{ "keyId": "...", "timestamp": "...", "signature": "..." }`
- Response: `{ "code": "OK", "body": { "jwe": "...", "ttl": 900 } }`
- The `jwe` token is used as `Public-Token` header in subsequent requests

### Step 2: Create draft — `POST /public/v1/application/{packageName}/version`

- Headers: `Public-Token: {jwe}`, `Content-Type: application/json`
- Body: `{ "whatsNew": "...", "publishType": "MANUAL" }`
- Response: `{ "code": "OK", "body": 243242 }` — `body` is the version ID (number)
- If draft already exists, error message contains `ID = {number}`

### Step 3: Upload AAB — `POST /public/v1/application/{packageName}/version/{versionId}/aab`

- Headers: `Public-Token: {jwe}`
- Form data: `file` (multipart) — the `.aab` file
- Response: `{ "code": "OK", "message": null, "body": null }`

## Changes to `.github/workflows/aab.yml`

Replace the 3 rustore steps (lines 170–210: "Read version", "RuStore upload AAB", "RuStore print result") with a single shell step. The `id: rustore` is kept for output references.

### New step

```yaml
      - name: RuStore — upload AAB (draft)
        if: ${{ inputs.upload-rustore }}
        id: rustore
        env:
          RUSTORE_KEY_ID: ${{ secrets.RUSTORE_KEY_ID }}
          RUSTORE_PRIVATE_KEY: ${{ secrets.RUSTORE_KEY_SECRET }}
        run: |
          set -euo pipefail

          AAB="$GITHUB_WORKSPACE/workspace/crocson/crocson.aab"
          FYNEAPP="$GITHUB_WORKSPACE/workspace/crocson/FyneApp.toml"

          APP_ID=$(grep -E '^\s*ID\s*=' "$FYNEAPP" | sed -E 's/^\s*ID\s*=\s*"([^"]+)".*/\1/')
          VERSION_NAME=$(grep -E '^\s*Version\s*=' "$FYNEAPP" | sed -E 's/^\s*Version\s*=\s*"([^"]+)".*/\1/')
          CHANGELOG="$GITHUB_WORKSPACE/workspace/crocson/metadata/ru-RU/changelogs/${VERSION_NAME}.txt"
          if [ -f "$CHANGELOG" ]; then
            WHATS_NEW=$(cat "$CHANGELOG")
          else
            WHATS_NEW="crocson v${VERSION_NAME}"
          fi

          echo "App: $APP_ID  Version: $VERSION_NAME"
          echo "AAB: $(ls -la "$AAB" | awk '{print $5}') bytes"

          # 1. Auth — SHA512withRSA(keyId + timestamp) per RuStore docs
          TIMESTAMP=$(date -u +'%Y-%m-%dT%H:%M:%S.000Z')
          SIGNATURE=$(printf '%s%s' "$RUSTORE_KEY_ID" "$TIMESTAMP" \
            | openssl dgst -sha512 -sign <(echo "$RUSTORE_PRIVATE_KEY" | base64 -d) \
            | base64 -w0)

          TOKEN=$(curl -sS -X POST https://public-api.rustore.ru/public/auth/ \
            -H 'Content-Type: application/json' \
            -d "$(jq -n --arg kid "$RUSTORE_KEY_ID" \
                     --arg ts "$TIMESTAMP" \
                     --arg sig "$SIGNATURE" \
                     '{keyId:$kid, timestamp:$ts, signature:$sig}')" \
            | jq -r '.body.jwe')

          if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
            echo "ERROR: RuStore auth failed"
            exit 1
          fi
          echo "Auth OK"

          # 2. Create draft
          DRAFT_RESP=$(curl -sS -X POST \
            "https://public-api.rustore.ru/public/v1/application/${APP_ID}/version" \
            -H 'Content-Type: application/json' \
            -H "Public-Token: $TOKEN" \
            -d "$(jq -n --arg wn "$WHATS_NEW" '{whatsNew:$wn, publishType:"MANUAL"}')")

          echo "Draft response:"
          echo "$DRAFT_RESP" | jq .

          VERSION_ID=$(echo "$DRAFT_RESP" | jq -r '.body // empty')
          if [ -z "$VERSION_ID" ]; then
            # Draft already exists — extract ID from error message
            VERSION_ID=$(echo "$DRAFT_RESP" | jq -r '.message' | grep -oP 'ID\s*=\s*\K\d+')
          fi
          if [ -z "$VERSION_ID" ]; then
            echo "ERROR: Could not create/find draft"
            exit 1
          fi
          echo "Draft ID: $VERSION_ID"

          # 3. Upload AAB — curl -F per official RuStore docs
          UPLOAD_RESP=$(curl -sS --location --request POST \
            "https://public-api.rustore.ru/public/v1/application/${APP_ID}/version/${VERSION_ID}/aab" \
            -H "Public-Token: $TOKEN" \
            --form "file=@${AAB}")

          echo "Upload response:"
          echo "$UPLOAD_RESP" | jq .

          CODE=$(echo "$UPLOAD_RESP" | jq -r '.code')
          if [ "$CODE" != "OK" ]; then
            echo "ERROR: AAB upload failed"
            exit 1
          fi
          echo "AAB uploaded to draft $VERSION_ID"

          echo "version_id=$VERSION_ID" >> $GITHUB_OUTPUT
          echo "status=draft" >> $GITHUB_OUTPUT
```

### Remove old steps

Remove these 3 steps from `aab.yml` (they are replaced by the single step above):

1. **"Read version and app ID from FyneApp.toml"** (lines ~170–187) — reading logic moved into the shell script
2. **"RuStore — upload AAB (draft)"** using `Suro4ek/rustore-action@v0.0.1` (lines ~189–200)
3. **"RuStore — print result"** (lines ~202–206) — output is already printed by the new script

## Why This Fixes It

| Aspect | `Suro4ek/rustore-action@v0.0.1` | curl approach |
|--------|----------------------------------|---------------|
| File upload | Node.js `FormData` + `Blob` (broken — empty file part) | `curl --form` (battle-tested multipart encoding) |
| Auth signing | Node.js `crypto.createSign` | `openssl dgst -sha512 -sign` |
| Error visibility | Only action-level messages | Full JSON API responses in logs |
| Dependencies | Third-party action (v0.0.1) | `curl`, `openssl`, `jq` (pre-installed on ubuntu-latest) |

## Files Modified

- `.github/workflows/aab.yml` — replace 3 rustore steps (lines ~170–206) with single curl-based step

## Risks

- `openssl dgst -sha512 -sign` with process substitution reading DER-formatted key — standard and well-supported on ubuntu-latest
- The RuStore private key secret (`RUSTORE_KEY_SECRET`) must be base64-encoded PKCS#8 DER (same format as before — no change to secrets)
