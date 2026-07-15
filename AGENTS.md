# AGENTS.md

## Project

crocson — a Go + [Fyne](https://fyne.io) app. Primary targets: Android (arm64) and Windows.
The Java side (`GoNativeActivity.java`) runs inside a Fyne `NativeActivity` and bridges to Go via cgo.

## Verifying code

Before considering work done, build both targets:

```
make arm64 wsl
```

- `make arm64` — `fyne package -os android/arm64 --release --sign`: compiles the Go
  code **and** the project `.java` files (`GoNativeActivity.java`, `CrocsonService.java`,
  etc.) into `crocson.apk`. A Java compile error surfaces here.
- `make wsl` — `GOOS=windows CC=x86_64-w64-mingw32-gcc ... go build`: the Windows binary
  via mingw under WSL.

Together `make arm64 wsl` covers compilation for both supported platforms.

## Plans

When creating plan files, use a **date-based** name, not a numeric timestamp:

```
.kilo/plans/YYYYMMDD-<concise-kebab-topic>.md
```

`YYYYMMDD` is the current date (e.g. `.kilo/plans/20260715-camera-dedup-statusbar.md`).

## Java ↔ Go bridge files

- `GoNativeActivity.java` — the `NativeActivity`: lifecycle, keyboard, file/intent
  handling, foreground service control, and the built-in QR camera block (Camera1 +
  a native preview Dialog that feeds NV21 frames to Go).
- `for_android.go` / `for_android.c` / `for_android.h` — cgo bridge (JNI exports like
  `cameraFrameNotify`, plus `callVoid`/`callInt`/`callBoolean` helpers used by Go).
- `qr_camera.go` — Go side of the QR scanner: gozxing decode loop, frame rotation,
  permission/camera lifecycle.
