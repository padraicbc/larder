# Larder as an Android app (everything on the phone)

This builds a real APK. The Go server runs **inside the app** (via
[`gomobile bind`](https://go.dev/wiki/Mobile)) and the web app is shown in a
full-screen WebView. There's no server to host: your log, plans, pantry and
ratings are stored privately on the phone.

Only the model runs elsewhere: a cloud API (OpenAI, Anthropic, OpenRouter)
or Ollama on a computer on your network.

> Want the same app on several devices with shared data? Use the hosted
> version instead: see [DEPLOY.md](DEPLOY.md).

## How it fits together

```
android/                     Kotlin shell: WebView + photo picker/camera (~150 lines)
  app/libs/larder.aar        built by gomobile from backend/mobile
backend/mobile/              Go package `larder`: Start(dataDir) -> sign-in URL, Stop()
backend/internal/webui/dist  the web app, embedded into the Go code
```

When the app opens it calls `Larder.start(filesDir)`. That starts the normal
Larder server on `127.0.0.1` on a random port (nothing outside the phone can
connect) and returns a one-time link. The WebView opens the link and gets the
usual session cookie. The API still requires that session, because other apps
on the same phone can reach `127.0.0.1` too; they never see the link.

On the phone there's a single account, it's the admin, and sign-up is off.

## Build it

You need:

- Go 1.24+ and Node 22+
- [Android Studio](https://developer.android.com/studio), then in
  **Settings > Languages & Frameworks > Android SDK > SDK Tools** tick
  **NDK (Side by side)** and **Android SDK Command-line Tools**
- `ANDROID_HOME` set to your SDK, e.g. `export ANDROID_HOME=~/Android/Sdk`
  (macOS: `~/Library/Android/sdk`)
- A JDK 17+. Android Studio's bundled one works:
  `export JAVA_HOME="/path/to/Android Studio/jbr"`

Then:

```sh
make android
```

That builds the web app, embeds it, runs `gomobile bind` to produce
`android/app/libs/larder.aar`, and builds
`android/app/build/outputs/apk/debug/app-debug.apk`.

Install it on a phone with USB debugging on:

```sh
adb install -r android/app/build/outputs/apk/debug/app-debug.apk
```

Or copy the APK to the phone and open it (allow "install unknown apps" when asked).

You can also open the `android/` folder in Android Studio and press Run, after
running `make android` once to create the `.aar`.

If you'd rather not install the Android toolchain, the Go wiki links a
[Docker image for gomobile builds](https://github.com/AgregoreWeb/gomobile-android-docker).

## Try it without a phone

```sh
make preview
```

This runs the exact phone build on your computer and prints the one-time link.

## Set up the model

Open **Profile > Settings** in the app:

- **Cloud:** choose OpenAI, OpenRouter or Anthropic and paste a key.
- **Ollama on your computer:** start it with `OLLAMA_HOST=0.0.0.0 ollama serve`,
  choose **Ollama** and set the base URL to your computer's address, e.g.
  `http://192.168.1.20:11434/v1`. With Tailscale on both devices, its Tailscale
  name works from anywhere.
- For fridge photos, set **Model for photos** to a vision model (e.g. `qwen2.5vl`).

## Good to know

- **Your data** lives in the app's private storage. Uninstalling deletes it.
  Android's cloud backup is turned off (`allowBackup="false"`) because the file
  also holds your model API key.
- **Debugging:** debug builds can be inspected from desktop Chrome at
  `chrome://inspect`. Go's logs appear in `adb logcat`.
- **Release builds** for the Play Store need your own signing key
  (Android Studio: **Build > Generate Signed App Bundle**) and a privacy policy.
- **Version numbers** in `android/build.gradle.kts` are from early 2025.
  Android Studio offers upgrades; accepting them is fine.
