# Plan: RuStore AAB Upload via `Suro4ek/rustore-action`

## Goal

Add a checkbox-controlled step to upload `crocson.aab` to RuStore as a **draft** using the ready-made action [`Suro4ek/rustore-action@v0.0.1`](https://github.com/Suro4ek/rustore-action).

## Action Parameters

| Input | Value | Source |
|-------|-------|--------|
| `key_id` | `${{ secrets.RUSTORE_KEY_ID }}` | GitHub Secret |
| `private_key` | `${{ secrets.RUSTORE_KEY_SECRET }}` | GitHub Secret |
| `application_id` | `${{ steps.version.outputs.app_id }}` | `FyneApp.toml` → `ID` |
| `file` | `$GITHUB_WORKSPACE/workspace/crocson/crocson.aab` | Build output |
| `whats_new` | Content of `metadata/ru-RU/changelogs/{version}.txt` | Changelog file |
| `publish_type` | `MANUAL` | Default — manual publish after review |
| `submit` | `false` | Draft only, no auto-submit to moderation |

## Changes to `.github/workflows/aab.yml`

### 1. Add checkbox input

```yaml
      upload-rustore:
        description: 'Upload .aab to RuStore (draft)'
        required: false
        default: false
        type: boolean
```

### 2. Add step after "Upload crocson.aab" (before Cleanup)

```yaml
      - name: Read version and app ID from FyneApp.toml
        if: ${{ inputs.upload-rustore }}
        id: version
        run: |
          FYNEAPP_TOML="$GITHUB_WORKSPACE/workspace/crocson/FyneApp.toml"
          VERSION_NAME=$(grep -E '^\s*Version\s*=' "$FYNEAPP_TOML" | sed -E 's/^\s*Version\s*=\s*"([^"]+)".*/\1/')
          APP_ID=$(grep -E '^\s*ID\s*=' "$FYNEAPP_TOML" | sed -E 's/^\s*ID\s*=\s*"([^"]+)".*/\1/')
          CHANGELOG="$GITHUB_WORKSPACE/workspace/crocson/metadata/ru-RU/changelogs/${VERSION_NAME}.txt"
          if [ -f "$CHANGELOG" ]; then
            WHATS_NEW=$(cat "$CHANGELOG")
          else
            WHATS_NEW="crocson v${VERSION_NAME}"
          fi
          echo "version=$VERSION_NAME" >> $GITHUB_OUTPUT
          echo "app_id=$APP_ID" >> $GITHUB_OUTPUT
          echo "whats_new<<EOF" >> $GITHUB_OUTPUT
          echo "$WHATS_NEW" >> $GITHUB_OUTPUT
          echo "EOF" >> $GITHUB_OUTPUT

      - name: RuStore — upload AAB (draft)
        if: ${{ inputs.upload-rustore }}
        id: rustore
        uses: Suro4ek/rustore-action@v0.0.1
        with:
          key_id: ${{ secrets.RUSTORE_KEY_ID }}
          private_key: ${{ secrets.RUSTORE_KEY_SECRET }}
          application_id: '${{ steps.version.outputs.app_id }}'
          file: '${{ github.workspace }}/workspace/crocson/crocson.aab'
          whats_new: '${{ steps.version.outputs.whats_new }}'
          publish_type: 'MANUAL'
          submit: 'false'

      - name: RuStore — print result
        if: ${{ inputs.upload-rustore }}
        run: |
          echo "version_id=${{ steps.rustore.outputs.version_id }}"
          echo "status=${{ steps.rustore.outputs.status }}"
```

### 3. No changes to Cleanup step

No additional temp files created by the action.

## Secrets Required

| Secret | Description |
|--------|-------------|
| `RUSTORE_KEY_ID` | Key ID from RuStore Console → API RuStore |
| `RUSTORE_KEY_SECRET` | Private key (base64 PKCS#8) downloaded when creating the key |

Note: `RUSTORE_COMPANY_ID` is not needed — `Suro4ek/rustore-action` uses only `key_id` + `private_key` for auth. The `companyId` parameter is deprecated in the RuStore API.

## Notes

- The `Read version and app ID from FyneApp.toml` step reads `Version`, `ID`, and the changelog file. It is gated by `upload-rustore` and will not run when the checkbox is off.
- `whats_new` is read from `metadata/ru-RU/changelogs/{version}.txt`. If the file is missing, falls back to `crocson v{version}`.
- `application_id` is read from `FyneApp.toml` `ID` field, not hardcoded.
- `submit: false` means the action creates a draft + uploads AAB but does **not** send it for moderation. That remains a manual step in RuStore Console.
- The action reuses existing drafts if one already exists.
- After AAB upload, the draft becomes visible in RuStore Console for manual review/submission.
