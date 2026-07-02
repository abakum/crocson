# Privacy policy link opens in-app (app-link loop) instead of the browser

## Root cause

The app registers a verified app link for its own GitHub Pages host in
`AndroidManifest.xml:39-46`:

```xml
<intent-filter android:autoVerify="true">
    <action android:name="android.intent.action.VIEW" />
    <category android:name="android.intent.category.DEFAULT" />
    <category android:name="android.intent.category.BROWSABLE" />
    <data android:scheme="https"
        android:host="abakum.github.io"
        android:pathPattern="/croc.*" />
</intent-filter>
```

`pathPattern="/croc.*"` matches BOTH:
- relay deep links `https://abakum.github.io/croc#...` (path `/croc`, params in fragment) — intended to open the app; parsed by `fromURI` (`applinks.go:191-195`); base constant `IO = "https://abakum.github.io/croc#"` (`main.go:83`), and
- the privacy page `https://abakum.github.io/croc/privacy-policy.html` (`PrivacyPolicyURL`, `main.go:90`) — NOT intended to open the app.

Flow of the bug:
1. Tap the "Privacy Policy" hyperlink (`privacy.go:77-85`, Fyne `widget.NewHyperlink` default behavior) → Fyne `app.OpenURL`.
2. Fyne Android `OpenURL` builds a bare `ACTION_VIEW` intent and calls `startActivity`
   (`app_mobile_and.c:70-84`) — it never requests a browser.
3. Because of `autoVerify="true"` + matching path, Android resolves the URL back to
   our own `singleTop` activity via `onNewIntent` (`GoNativeActivity.java:987`).
4. `processIntentData` treats ANY `ACTION_VIEW` URI as a file to send
   (`GoNativeActivity.java:1080-1088`) → URI flows to `uriFromIntent`.
5. `fromURI` fails (not a relay link), so `send.go` falls through to file-copy handling
   → `copy https://.../privacy-policy.html .../send/privacy-policy.html`
   (matches the reported log: `send.go:1749`, the `copy` info line).

Key fact: Android matches `path`/`pathPattern` only against the URL path component
(between host and `?`/`#`); fragment and query are ignored. Relay links have path
exactly `/croc`; the privacy page has path `/croc/privacy-policy.html`.

## Plan

### 1. Narrow the app-link path (core fix)
File: `AndroidManifest.xml`, the `autoVerify` intent filter (~lines 39-46).

Replace:
```xml
android:pathPattern="/croc.*"
```
with exact path match:
```xml
android:path="/croc"
```

Effect:
- Relay deep links `https://abakum.github.io/croc#...` (path `/croc`) → still open the app. ✓
- Privacy page `/croc/privacy-policy.html` → no longer matches → Fyne `ACTION_VIEW`
  resolves to the default browser and opens there. ✓
- `autoVerify` stays valid: verification is host-level via `assetlinks.json`;
  narrowing the path does not invalidate it, only restricts which paths are captured.

### 2. (Recommended) Harden intent ingestion so an http(s) VIEW never copies as a send file
File: `GoNativeActivity.java`, `processIntentData`, the `ACTION_VIEW` branch
(~lines 1080-1088).

Currently every `ACTION_VIEW` URI is passed to `sendIntentURIs`, which makes `send.go`
copy any non-relay http(s) URL into the send folder. Guard the branch: for an
`ACTION_VIEW` whose data scheme is `http`/`https`, only forward to Go when it is a
relay deep link (path `/croc` on `abakum.github.io`); otherwise log and ignore.

This is defensive (the manifest fix already prevents the privacy page from reaching
`onNewIntent`) but closes the latent bug for any future/forgotten https path that the
app might re-ingest.

## Validation
- Build and install the APK on an emulator.
- Tap the "Privacy Policy" hyperlink in the consent dialog (and the About tab).
  Confirm it opens in the browser, NOT in-app (no `send.go:1749` / `copy .../send/`
  log lines; no file added to the send list).
- Share/open a relay deep link `https://abakum.github.io/croc#<base64>` from outside
  (e.g. browser/chat) and confirm the app still opens it and applies the relay config.
- Confirm `logcat` shows the browser Activity launched for the privacy URL and the
  relay link still delivered via `onNewIntent`.

## Risks / notes
- `android:path="/croc"` is an exact path match; equivalent to
  `android:pathPattern="/croc"` (no wildcard). Use exact `path` for clarity.
- If a future app-link page under `/croc/...` is needed, add a separate explicit
  filter for it (acceptable, explicit > broad).
- Verification only requires `https://abakum.github.io/.well-known/assetlinks.json`
  (already published); path narrowing does not require re-publishing that file.
- No Java/Go logic change is strictly required for the reported bug; step 2 is optional
  hardening.
