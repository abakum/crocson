#!/usr/bin/env bash
set -euo pipefail

APP_ID="com.github.abakum.crocson"
APP_NAME="crocson"
APPDIR="${APP_NAME}.AppDir"
ARCH="x86_64"
APPIMAGETOOL_DIR="${XDG_BIN_HOME:-$HOME/.local/bin}"
APPIMAGETOOL="${APPIMAGETOOL_DIR}/appimagetool-${ARCH}.AppImage"
APPIMAGETOOL_URL="https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-${ARCH}.AppImage"

TAR_XZ="${APP_NAME}.tar.xz"
TAR_PREFIX="${APP_NAME}"
FYNEAPP="FyneApp.toml"

echo "=== Building AppImage ==="

if [ ! -f "$FYNEAPP" ]; then
    echo "ERROR: ${FYNEAPP} not found."
    exit 1
fi

VERSION=$(grep -E '^\s*Version\s*=' "$FYNEAPP" | sed -E 's/^\s*Version\s*=\s*"([^"]+)".*/\1/')
if [ -z "$VERSION" ]; then
    echo "ERROR: Cannot extract Version from ${FYNEAPP}"
    exit 1
fi
echo "Version: $VERSION"

if [ ! -f "$TAR_XZ" ]; then
    echo "ERROR: ${TAR_XZ} not found. Run 'make linux' first."
    exit 1
fi

echo "Cleaning old AppDir..."
rm -rf "$APPDIR"
mkdir -p "${APPDIR}/usr/bin"

echo "Extracting ${TAR_XZ}..."
TEMP_EXTRACT=$(mktemp -d)
trap "rm -rf $TEMP_EXTRACT" EXIT
tar -xf "$TAR_XZ" -C "$TEMP_EXTRACT"

BINARY=$(find "$TEMP_EXTRACT" -name "$APP_NAME" -type f | head -1)
if [ -z "$BINARY" ]; then
    echo "ERROR: ${APP_NAME} binary not found in ${TAR_XZ}"
    ls -laR "$TEMP_EXTRACT"
    exit 1
fi

cp "$BINARY" "${APPDIR}/usr/bin/${APP_NAME}"
chmod +x "${APPDIR}/usr/bin/${APP_NAME}"
echo "Binary: $(du -h "${APPDIR}/usr/bin/${APP_NAME}" | cut -f1)"

cp "${TEMP_EXTRACT}/${TAR_PREFIX}/usr/local/share/pixmaps/${APP_ID}.png" "${APPDIR}/${APP_ID}.png"
echo "Icon: from ${TAR_XZ}"

cp "${TEMP_EXTRACT}/${TAR_PREFIX}/usr/local/share/applications/${APP_ID}.desktop" "${APPDIR}/${APP_ID}.desktop"
mkdir -p "${APPDIR}/usr/share/applications"
cp "${APPDIR}/${APP_ID}.desktop" "${APPDIR}/usr/share/applications/${APP_ID}.desktop"
echo "Desktop: from ${TAR_XZ}"

echo "Generating AppStream metainfo..."
METAINFO_DIR="${APPDIR}/usr/share/metainfo"
mkdir -p "$METAINFO_DIR"
cat > "${METAINFO_DIR}/${APP_ID}.appdata.xml" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<component type="desktop-application">
  <id>${APP_ID}</id>
  <name>${APP_NAME}</name>
  <summary>Easily and securely send things from one computer to another</summary>
  <metadata_license>CC0-1.0</metadata_license>
  <project_license>ISC</project_license>
  <developer id="com.github.abakum">
    <name>Konstantin Abakumov</name>
  </developer>
  <description>
    <p>
      crocson is a cross-platform GUI and CLI for croc, allowing secure file
      transfer, WebDAV sharing, encrypted chat with video calls, and more on
      Android, Windows, Linux, and macOS.
    </p>
    <p>File transfer from sender to receiver</p>
    <ul>
      <li>Via relay without port forwarding</li>
      <li>Desktop: drag-and-drop, command line, stdin pipe</li>
      <li>Android: "Share" and "Open with" from file managers</li>
    </ul>
    <p>WebDAV — two-way file transfer</p>
    <ul>
      <li>Built-in HTTP/HTTPS server</li>
      <li>File browsing via browser, streaming audio/video</li>
      <li>Tunneling through encrypted tunnel</li>
    </ul>
    <p>Chat</p>
    <ul>
      <li>Web chat with session history</li>
      <li>Video calls, video messages, desktop screen sharing</li>
      <li>Server-side recording of webcam/microphone/screenshots</li>
    </ul>
    <p>Security</p>
    <ul>
      <li>End-to-end encryption (PAKE), one-time passwords (TOTP)</li>
      <li>QR code generation/scanning, Deep Links</li>
    </ul>
    <p>CLI mode: pipes, text sending, transfer resuming, quiet mode</p>
  </description>
  <screenshots>
    <screenshot type="default">
      <image type="source">https://raw.githubusercontent.com/abakum/crocson/master/metadata/en-US/images/phoneScreenshots/1.png</image>
    </screenshot>
    <screenshot>
      <image type="source">https://raw.githubusercontent.com/abakum/crocson/master/metadata/en-US/images/phoneScreenshots/2.png</image>
    </screenshot>
    <screenshot>
      <image type="source">https://raw.githubusercontent.com/abakum/crocson/master/metadata/en-US/images/phoneScreenshots/4.png</image>
    </screenshot>
  </screenshots>
  <launchable type="desktop-id">${APP_ID}.desktop</launchable>
  <url type="homepage">https://github.com/abakum/crocson</url>
  <url type="bugtracker">https://github.com/abakum/crocson/issues</url>
  <url type="help">https://github.com/abakum/crocson/blob/master/README.md</url>
  <provides>
    <binary>${APP_NAME}</binary>
  </provides>
  <categories>
    <category>Network</category>
    <category>FileTransfer</category>
  </categories>
  <releases>
    <release version="${VERSION}" date="$(date -u +%Y-%m-%d)"/>
  </releases>
  <content_rating type="oars-1.1"/>
</component>
EOF
echo "Metainfo: ${METAINFO_DIR}/${APP_ID}.appdata.xml"

cat > "${APPDIR}/AppRun" <<'APPRUN'
#!/bin/sh
SELF=$(readlink -f "$0")
HERE=${SELF%/*}
export PATH="${HERE}/usr/bin:${PATH}"
export XDG_DATA_DIRS="${HERE}/usr/share:${XDG_DATA_DIRS:-/usr/local/share:/usr/share}"
exec "${HERE}/usr/bin/APPNAME" "$@"
APPRUN
sed -i "s/APPNAME/${APP_NAME}/" "${APPDIR}/AppRun"
chmod +x "${APPDIR}/AppRun"

if [ ! -f "$APPIMAGETOOL" ]; then
    echo "Downloading appimagetool to ${APPIMAGETOOL_DIR}..."
    mkdir -p "$APPIMAGETOOL_DIR"
    wget -q "$APPIMAGETOOL_URL" -O "$APPIMAGETOOL"
    chmod +x "$APPIMAGETOOL"
fi

echo "Building AppImage..."
"$APPIMAGETOOL" "$APPDIR"

rm -rf "$APPDIR"

OUTPUT="${APP_NAME}-${VERSION}-${ARCH}.AppImage"
EXPECTED="${APP_NAME}-${ARCH}.AppImage"
if [ -f "$EXPECTED" ] && [ "$EXPECTED" != "$OUTPUT" ]; then
    mv "$EXPECTED" "$OUTPUT"
fi
if [ -f "$OUTPUT" ]; then
    echo ""
    echo "=== AppImage created ==="
    echo "File: $OUTPUT"
    echo "Size: $(du -h "$OUTPUT" | cut -f1)"
else
    echo "ERROR: AppImage was not created"
    exit 1
fi
