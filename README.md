# Larder

A nutrition tracker and meal planner: log meals from text, photos or saved product labels; repeat your usual meals; adjust a day’s calorie target; and generate meal plans with shopping lists. Replace individual meals, cook from what’s in your fridge, and rate recipes so future suggestions get better. Multi-user with JWT auth. Go backend, SvelteKit + daisyUI frontend, any LLM you like.

## Quick start

```sh
cd web && npm install && cd ..
make api    # terminal 1: Go API on :8080 (data in backend/data/nutrition.json)
make web    # terminal 2: open http://localhost:5173
```

The first account you create is the **admin** (the only one who can change model settings). It starts in **demo mode** (canned recipes and example photo results, no model needed), so you can click around straight away. Then open **Settings** to plug in a model:

- **Ollama**: `ollama pull llama3.1`, choose the Ollama preset, Test connection, Save.
- **OpenAI / OpenRouter / Anthropic**: choose the preset, set the model and key.
- Or set `NUTRI_LLM_API_KEY` in the environment instead of saving the key.
- **Photos** need a model that accepts images. If your main model is text-only (e.g. `llama3.1`), set **Model for photos** to a vision model (`ollama pull qwen2.5vl` or `llava`).

## Configuration

Environment variables (secrets, sign-up, HTTPS, proxies) are listed in [docs/DEPLOY.md](docs/DEPLOY.md#environment-variables).

Upgrading from the single-user version? Your old data file is migrated automatically and handed to the first account you create.

`make run` builds one self-contained binary (the web app is embedded) that serves everything on :8080.

## On your phone

Two ways, both from the same code:

- **Installable web app:** host the server, open it in Chrome on Android, choose **Install app**. Opens even when offline and shares data across your devices. See [docs/DEPLOY.md](docs/DEPLOY.md) (Tailscale or Caddy, Docker Compose included).
- **Android app (APK):** the Go server runs inside the app via `gomobile bind`; no hosting, data stays on the phone. See [docs/ANDROID.md](docs/ANDROID.md). Try it on your computer first with `make preview`.

### Android 16 KB page support

Install Android NDK **28.2.13676358** and set `ANDROID_HOME` before running `make android`. The build selects this NDK even if your shell has an older `ANDROID_NDK_HOME`. Set `LARDER_NDK_VERSION` to use another installed NDK r28 or newer. Python 3 is required for the automatic APK verification.

The gomobile backend is linked with explicit 16 KB alignment for both ARM64 and x86-64. Gradle packages the native libraries uncompressed, and every build checks ELF LOAD alignment, RELRO boundaries and APK library offsets. Misalignment fails the build.

To verify an APK manually:

```sh
python3 scripts/check-android-pages.py android/app/build/outputs/apk/debug/app-debug.apk
"$ANDROID_HOME/build-tools/35.0.0/zipalign" -c -P 16 4 android/app/build/outputs/apk/debug/app-debug.apk
```

Passing these checks verifies binary and packaging alignment. Runtime testing needs a device or emulator where `adb shell getconf PAGE_SIZE` returns `16384`; a result of `4096` only tests 4 KB operation. See the [Android build guide](docs/ANDROID.md#build-it) for setup and installation.

## Everyday use

- **Log food:** describe a meal, choose a recipe, enter macros manually, or open **Photo → Meal photo**. Review detected foods and quantities, remove incorrect items, and add anything missed. After changing the food list, use **Update estimate** before logging.
- **Repeat a meal:** use a **Log again** button on the daily log, or open **Log food → Recent** to search your previous meals. Frequently logged meals appear first. Use **Adjust** to change servings or nutrition before saving. Repeating a log copies its saved nutrition and quantities without calling the model.
- **Read a package’s nutrition panel:** open **Log food → Photo → Product label**. Photograph the panel, check its values and whether they are per 100 g, per 100 ml or per serving, and give the product a useful name. Enter the amount eaten; the app calculates calories, protein, carbs, fat and fibre from the panel values. An ingredients list alone is not enough for this flow.
- **Save and reuse products:** choose **Save product for next time**, or log the product to save it automatically. Next time, open **Log food → Products**, search its name, and enter the new amount eaten. Saved products retain the original nutrition basis and precision independently of food logs. For example, a product with 380 kcal per 100 g contributes 228 kcal when you eat 60 g. No model request is needed to reuse a saved product.
- **Adjust one day’s calories:** below the daily totals, select **Adjust this day’s calories**. Enter a target or use the ±100/250 kcal buttons, optionally add a note such as “Gym session”, and save. The daily rings and weekly chart use the adjusted target. **Reset to usual** removes the adjustment. Your Profile and protein, carb, fat and fibre targets stay unchanged. These dated log adjustments do not change the targets used for undated meal plans.
- **Replace one plan meal:** open a plan and choose **Replace meal** on the breakfast, lunch, dinner or snack you want to change. Add an optional preference. Other meals stay unchanged, day totals refresh, and the shopping list updates while keeping checked items and manual additions. Only the selected occurrence changes when a recipe is reused elsewhere in the plan.
- **Cook from your pantry:** type ingredients in **Pantry**, or use **Add from photos**, review the list, then choose **Suggest recipes**. Enable **Only use what I have** for ideas based on your pantry and staples. Plan shopping lists mark ingredients you already have.

Start by setting your stats, targets, diets and allergies in **Profile**, then generate a plan in **Plans**. Rate recipes to guide future suggestions; **Recipes** shows what the app has learned from those ratings.

Demo recipes and plans, their shopping lists, and food logs linked to demo recipes are hidden when a real model provider is saved in Settings. Switching back to Demo mode makes them visible again; the data is retained.

## Backup and restore

Open **Settings → Backup and restore** and choose **Download backup** to save a portable JSON file. On Android, choose where to save it with the system document picker; in a browser, it downloads normally.

Use **Restore from file** to select a backup. The app validates it and shows the export date and item counts before you choose **Replace data and restore**. Restore replaces your own profile and saved data; it does not merge records or affect another account. Download your current data first if you want to keep both versions.

Backups include logs and food quantities, saved product nutrition, daily calorie adjustments, recipes, plans, shopping lists, pantry items and ratings. They exclude account credentials, model settings and API keys, and appearance preferences. Your current sign-in and model settings are retained after restore; configure the model separately when moving to a new installation. Only Larder backup files are accepted, up to 32 MB; the raw server data file is a different format.

## Data storage

All persistent data uses a JSON file, not SQLite. It includes profiles, logs and their food quantities, saved products and nutrition values, day calorie adjustments, recipes, plans, shopping lists, pantry items and ratings. Hosted data is partitioned by user; Android keeps its file in the app’s private storage. Existing files gain the new collections automatically when loaded.

## How it fits together

```
backend/
  cmd/server/            main: flags, env, graceful shutdown
  internal/models/       domain types (mirrored in web/src/lib/types.ts)
  internal/auth/         PBKDF2 passwords, JWT issue/verify, login rate limiter
  internal/llm/          Client + VisionClient; OpenAI-compatible, Anthropic, Mock
  internal/nutrition/    the interesting logic, all pure functions:
    targets.go           Mifflin-St Jeor -> macro targets
    safety.go            checks every LLM recipe against diet/exclusions/allergies
    feedback.go          ratings -> loved/disliked recipes + ingredients/tags
    prompts.go           prompt builders (feedback goes in here)
    planner.go           generate -> validate -> safety check -> one corrective retry
    replace_meal.go      replace one meal, checking the kept meals and daily macros
    meal_photo.go        identify meal foods, quantities and estimated macros
    product_label.go     read printed nutrition and its measurement basis
    shopping.go          aggregate, convert units, scale to household
    pantry.go            pantry matching, photo scan, cook-from-pantry
    categories.go        guesses the aisle for typed items
  internal/store/        JSON file store, data partitioned per user
  internal/api/          net/http handlers; auth.go has the middleware, local.go the on-device sign-in
  internal/webui/        the built web app, embedded with go:embed
  mobile/                gomobile entry point (its own module) + cmd/preview
web/src/
  lib/                   api client, types, format helpers, label portion maths, components
  routes/                Today, Plans, Pantry, Shopping, Recipes, Profile, Settings, Login
  service-worker/        caches the app shell for fast, offline-tolerant starts
android/                 Kotlin WebView shell for the APK
```

**The feedback loop:** `nutrition.Summarize` averages ratings per recipe, then scores each ingredient and tag by `(avg - 3)` across the recipes it appears in. Loved and disliked recipes (with your comments) plus the top ingredients go into the next plan's prompt.

**Auth:** register/login return a JWT (HS256, 30 days) in an `HttpOnly`, `SameSite=Strict` cookie scoped to `/api`, so page scripts can't read it and other sites can't use it. The same token is in the JSON response for scripts and mobile clients (`Authorization: Bearer …`). Passwords use PBKDF2-SHA256 with 600k iterations from Go's standard library, so the only new dependency is `golang-jwt/jwt`. Logins are rate-limited per IP + email (5 failures per 15 minutes) and unknown emails take the same time as wrong passwords. Every store method takes a user ID, and tests check that one user gets 404s for another's plans, recipes and pantry.

Known limits: logging out clears the cookie, but a copied token stays valid until it expires. If you need "sign out everywhere", add a `TokenVersion` to `User`, put it in the claims, and bump it on logout. Behind a trusted reverse proxy, configure `NUTRI_TRUST_PROXY=1` as described in [docs/DEPLOY.md](docs/DEPLOY.md).

**Photos:** the browser shrinks each photo to 1600px JPEG before upload (which also strips EXIF/GPS). The server checks the real file type from its bytes, caps size and count, and sends the images to the vision model. Scanning does not add food to the log automatically. Review the detected meal or nutrition values first; named products can also be saved without logging a portion. Nutrition panels require readable calorie, protein, carb and fat values; the app does not infer them from an ingredients list. Check any notes about unreported fibre.

**Safety:** LLMs ignore constraints sometimes. `CheckRecipe` runs on every generated recipe. If anything fails, the model gets one retry with the problems listed; if it still fails, the recipe is saved with a visible warning rather than silently. Individual meal replacements are stricter: they are rejected after the corrective retry if diet, allergy, exclusion, preparation-time or daily macro checks still fail, keeping the original plan. The calorie and each macro check allow a 10% daily tolerance, or the original day’s deviation if it was already further from target; a replacement must not worsen that deviation. The keyword lists in `safety.go` are deliberately simple; extend them as you find gaps. This is a helper, not a guarantee, for serious allergies.

## Tests

```sh
make test     # Go (server + mobile module) and web
make check    # go vet + svelte-check
```

- Go: table-driven tests for targets, safety rules, feedback, shopping maths, pantry matching and categories, the planner's and pantry cook's retry logic (with a scripted fake model), text and image requests for both providers (against `httptest` servers), the store (including per-user isolation and v1 migration), JWTs (expiry, tampering, `alg: none`), passwords, rate limiting, and end-to-end API journeys including multipart meal and nutrition-panel uploads, repeat logs, saved products, day calorie targets, and individual meal replacement with shopping-list updates.
- Mobile: the on-device server (loopback only, one-time sign-in links, no second accounts, data surviving restarts).
- Web: Vitest for format and pantry-matching helpers, plus nutrition-label scaling for grams, millilitres and servings, corrected values and invalid inputs.

To add a test, copy the nearest table and add a row. `safety_test.go` is the easiest place to start: one line per case.

## Notes

- SvelteKit 3: config lives in `vite.config.ts` (no `svelte.config.js`), imports use `#lib/...` via `package.json` `imports` instead of `$lib`, and the service worker uses `$app/manifest` + `$app/env` instead of `$service-worker`.
- Use HTTPS (and `NUTRI_SECURE_COOKIES=1`) if you expose it beyond your own network.
- Metric units only for now.

## Ideas for next

- SQLite store (`modernc.org/sqlite` keeps it cgo-free)
- Barcode scanning / Open Food Facts lookup for logging
- "Used it up" button on pantry recipes that removes ingredients from the pantry
- Password reset and changing email/password
- Share a pantry and plans within a household
- Streaming plan generation so long local-model runs show progress
