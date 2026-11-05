#!/usr/bin/env bash
# Builds the Go server into android/app/libs/larder.aar with gomobile, then the debug APK.
# Run via `make android` (which builds and embeds the web app first).
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)

: "${ANDROID_HOME:?Set ANDROID_HOME to your Android SDK, e.g. ~/Android/Sdk (Android Studio > Settings > Android SDK)}"
# gomobile reads ANDROID_NDK_HOME separately from Gradle. Pin it here so an
# older shell setting cannot silently produce 4 KB-aligned native libraries.
ndk_version=${LARDER_NDK_VERSION:-28.2.13676358}
export ANDROID_NDK_HOME="$ANDROID_HOME/ndk/$ndk_version"
if [ ! -d "$ANDROID_NDK_HOME/toolchains/llvm" ]; then
  echo "Install Android NDK $ndk_version in Android Studio's SDK Manager (Show Package Details)." >&2
  exit 1
fi
if [ "${ndk_version%%.*}" -lt 28 ]; then
  echo "Larder requires NDK r28 or newer for 16 KB page support." >&2
  exit 1
fi
echo "Using Android NDK $ndk_version (16 KB native page alignment)"

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
  -ldflags="-s -w -extldflags=-Wl,-z,max-page-size=16384,-z,common-page-size=16384" \
  -o "$root/android/app/libs/larder.aar" \
  .

cd "$root/android"
./gradlew assembleDebug

apk=android/app/build/outputs/apk/debug/app-debug.apk
python3 "$root/scripts/check-android-pages.py" "$root/$apk"
echo
echo "Built $apk"
echo "Install on a phone with USB debugging on:  adb install -r $apk"
