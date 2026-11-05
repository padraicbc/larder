#!/usr/bin/env bash
# Builds the Go server into android/app/libs/larder.aar with gomobile, then the debug APK.
# Run via `make android` (which builds and embeds the web app first).
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)

: "${ANDROID_HOME:?Set ANDROID_HOME to your Android SDK, e.g. ~/Android/Sdk (Android Studio > Settings > Android SDK)}"
if ! ls "$ANDROID_HOME"/ndk/* >/dev/null 2>&1 && [ -z "${ANDROID_NDK_HOME:-}" ]; then
  echo "No NDK found. In Android Studio: Settings > Android SDK > SDK Tools > NDK (Side by side)." >&2
  exit 1
fi

if ! command -v gomobile >/dev/null; then
  go install golang.org/x/mobile/cmd/gomobile@latest
  export PATH="$(go env GOPATH)/bin:$PATH"
fi
gomobile init

cd "$root/backend/mobile"
# gomobile needs golang.org/x/mobile/bind in the module it binds (go.dev/wiki/Mobile).
# It only goes into backend/mobile/go.mod, so the main server stays dependency-light.
go get golang.org/x/mobile/bind

# arm64 = phones, amd64 = the Android emulator on a PC.
gomobile bind \
  -target=android/arm64,android/amd64 \
  -androidapi 24 \
  -ldflags="-s -w" \
  -o "$root/android/app/libs/larder.aar" \
  .

cd "$root/android"
./gradlew assembleDebug

apk=android/app/build/outputs/apk/debug/app-debug.apk
echo
echo "Built $apk"
echo "Install on a phone with USB debugging on:  adb install -r $apk"
