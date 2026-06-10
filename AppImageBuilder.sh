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

echo "=== Building AppImage ==="

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
echo "Desktop: from ${TAR_XZ}"

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
"$APPIMAGETOOL" --no-appstream "$APPDIR"

rm -rf "$APPDIR"

OUTPUT="${APP_NAME}-${ARCH}.AppImage"
if [ -f "$OUTPUT" ]; then
    echo ""
    echo "=== AppImage created ==="
    echo "File: $OUTPUT"
    echo "Size: $(du -h "$OUTPUT" | cut -f1)"
else
    echo "ERROR: AppImage was not created"
    exit 1
fi
