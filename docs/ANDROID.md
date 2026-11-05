# Larder as an Android app (everything on the phone)

This builds a real APK. The Go server runs **inside the app** (via
[`gomobile bind`](https://go.dev/wiki/Mobile)) and the web app is shown in a
full-screen WebView. There’s no server to host: your logs, saved product nutrition,
daily calorie adjustments, plans, pantry and ratings are stored privately on the phone.

Only the model runs elsewhere: a cloud API (OpenAI, Anthropic, OpenRouter)
or Ollama on a computer on your network.

> Want the same app on several devices with shared data? Use the hosted
> version instead: see [DEPLOY.md](DEPLOY.md).

## How it fits together

```
android/                     Kotlin shell: WebView + photo picker/camera
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
- Python 3 for the automatic native-library and APK alignment check
- [Android Studio](https://developer.android.com/studio), then in
  **Settings > Languages & Frameworks > Android SDK > SDK Tools** tick
  **NDK (Side by side)** and **Android SDK Command-line Tools**. Enable **Show Package Details** and install NDK **28.2.13676358**
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

The script pins `ANDROID_NDK_HOME` to `$ANDROID_HOME/ndk/28.2.13676358`, overriding an older shell setting. To use another installed NDK r28 or newer, set `LARDER_NDK_VERSION`. It passes explicit 16 KB linker flags to gomobile and checks the resulting APK’s ELF LOAD segments, RELRO boundaries and uncompressed library offsets. The build fails if those checks fail. Gradle uses uncompressed native-library packaging with AGP 8.7.3.

To recheck an existing APK:

```sh
python3 scripts/check-android-pages.py android/app/build/outputs/apk/debug/app-debug.apk
"$ANDROID_HOME/build-tools/35.0.0/zipalign" -c -P 16 4 android/app/build/outputs/apk/debug/app-debug.apk
```

These checks verify binary and packaging alignment. Runtime testing still requires a 16 KB device or emulator; `adb shell getconf PAGE_SIZE` should report `16384`. See [Android’s 16 KB page-size guide](https://developer.android.com/guide/practices/page-sizes).

Install it on a phone with USB debugging on:

```sh
adb install -r android/app/build/outputs/apk/debug/app-debug.apk
```

If `adb devices` shows `unauthorized`, unlock the phone and accept **Allow USB debugging?**
Enable **Always allow from this computer** if appropriate, then run `adb devices` again.
If the prompt does not appear, use **Developer options → Revoke USB debugging authorizations**
and reconnect the USB cable.

Use `adb install -r` for updates to preserve the app’s existing data. After source changes,
run `make android` again before reinstalling so both the web app and Go backend are rebuilt.

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
- For meal, package nutrition-panel and pantry photos, set **Model for photos** to a vision model (e.g. `qwen2.5vl`). The selected model must accept images.

## Log meals and reuse products

- **Meal photos:** open **Log food → Photo → Meal photo** and take or choose a picture. Correct the foods and quantities, add anything missed, and update the estimate before logging.
- **Package nutrition:** open **Log food → Photo → Product label** and photograph the nutrition panel. Check its scanned values and basis (100 g, 100 ml or a serving), name the product, then enter how much you ate. The app scales the printed nutrition to that amount; an ingredients list alone is insufficient.
- **Saved products:** save the named product separately, or log it to save automatically. Open **Log food → Products** next time, search by name, and enter the amount eaten. Saved nutrition is independent of food logs and can be reused without a model request.
- **Usual meals:** use **Log again** on the daily log or search **Log food → Recent**. **Adjust** lets you change servings before adding the meal.
- **Daily calories:** use **Adjust this day’s calories** under the daily totals. Increase or lower that date’s calorie target, add a note, or reset to your usual target. Macro targets and Profile settings stay unchanged.
- **Plan changes:** use **Replace meal** on an individual plan meal. The replacement is checked against dietary constraints and the day’s macros, then the shopping list and totals update. An unsuitable replacement leaves the original in place.

## Backup and restore

In **Settings → Backup and restore**, choose **Download backup**. Android opens its document picker so you can save the JSON file to a location you choose, including a connected cloud-storage provider.

To move data to a new phone, install Larder there, open **Restore from file**, and choose your saved Larder backup. Review the date and item counts, then confirm **Replace data and restore**. Restore replaces the current installation’s profile and saved data. Back up the current data first if you want to keep it.

The backup includes saved products and nutrition, food logs and quantities, daily calorie adjustments, recipes, plans, shopping lists, pantry and ratings. Sign-in credentials, model settings/API keys and appearance preferences are excluded. Configure the model separately on a new phone. Files are limited to 32 MB and validated before restore.

## Good to know

- **Storage:** data is a JSON file at `filesDir/larder.json`, not SQLite. It includes named products with their nutrition basis, meal quantities and daily calorie adjustments. It survives restarts and updates installed with `adb install -r`.
- **Removing data:** uninstalling the app or clearing its storage deletes the local file. Android’s cloud backup is turned off (`allowBackup="false"`) because it also holds your model API key.
- **Model access:** new meal estimates, photo scans, plans and replacements need access to your configured model. Saved-product portion calculations and repeating logged meals use saved data.
- **Debugging:** debug builds can be inspected from desktop Chrome at `chrome://inspect`. Go’s logs appear in `adb logcat`.
- **Release builds** for the Play Store need your own signing key (Android Studio: **Build > Generate Signed App Bundle**) and a privacy policy.
- **Build versions:** application ID, SDK targets and app version are in `android/app/build.gradle.kts`; plugin versions are in `android/build.gradle.kts`.
