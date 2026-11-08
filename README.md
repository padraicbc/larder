# Larder

A nutrition tracker and meal planner: log meals from text, photos or saved product labels; repeat your usual meals; adjust a day’s calorie target; and generate meal plans with shopping lists. Save and reuse named meals, fix them in plans, track weight and workouts, check food quality by barcode or ingredients photo in the shop, rate the products you scan, replace individual meals, cook from what’s in your fridge, and rate recipes so future suggestions get better. Multi-user with JWT auth. Go backend, SvelteKit + daisyUI frontend, any LLM you like.

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
- **Save and reuse products:** choose **Save product for next time**, or log the product to save it automatically. A product found by barcode (Open Food Facts) while logging is saved to Products straight away. Next time, open **Log food → Products**, search its name, and enter the new amount eaten. Saved products retain the original nutrition basis and precision independently of food logs. For example, a product with 380 kcal per 100 g contributes 228 kcal when you eat 60 g. No model request is needed to reuse a saved product.
- **Browse and rate products:** open **Products** in the sidebar to see every saved product with its nutrition per 100 g/ml (or per serving) and a 0–100% ingredient rating. Search, sort by best rated, protein or calories, and use the **Need ingredients or nutrition** filter to find products that are named but not yet rated (a generic whole food only ever needs its nutrition). Tap a product to **edit everything**: name, barcode, nutrition (per g, ml or serving) and ingredients, then **Save changes**; **Photograph ingredients** reads them from a picture. Delete removes it. **Tap the rating badge (or “Why 72%?”) for a popup that explains it:** the starting score, each thing that lowered or raised it with the points and a plain-language reason (for example why an E-number colour or hydrogenated fat is a concern), the sum, and what each rating band means. The rating starts at 90 and gains or loses points for E-number and named additives (colours, nitrites, BHA/BHT, sweeteners), high-fructose syrup, hydrogenated fat, sugar high in the list, very long ingredient lists, energy density, fibre and protein; each point is listed. **It also counts how many different additives and processing markers there are** (yeast extract, dairy powders, dextrose, added fibre and the like as well as E-numbers), because gums, acids, emulsifiers and modified starches are each mild on their own but a long list is the clearest sign of an ultra-processed recipe: 2 additives cost 4 points, 3 cost 8, 4–5 cost 13 and 6 or more cost 18 (the same additive written as an E-number and by name counts once, and flour fortification such as calcium carbonate, iron and niacin isn't counted). A typical frozen pizza with stabilisers, an emulsifier, modified starch, citric and ascorbic acid and flavouring now rates around 57% (fair), not the mid-70s. Common harmless additives such as citric acid and pectin cost nothing. It is a rough guide from the ingredients and macros only (salt and sugar totals aren’t stored), not medical advice. Ratings are computed when viewed, so rule changes apply to existing products.
- **Log everyday food in a few taps:** open **Log food → Products**. Under **Everyday foods**, tap a common one (boiled or scrambled eggs, cooked chicken breast, bacon, white or wholemeal bread, semi-skimmed milk, cooked rice or pasta, porridge oats, bananas…), **browse by kind** (Eggs, Meat & poultry, Fish & seafood, Dairy, Bread/rice & pasta, Vegetables, Fruit, Beans & pulses, Nuts & seeds, Fats & oils, Sweet & spreads, Drinks), or type to search a built-in list of **243 everyday foods**: loose fruit and vegetables (raw and boiled or baked), eggs, meat and poultry, **fish and seafood** (salmon, cod, haddock, mackerel, tuna, prawns…), **dairy** (milk, yoghurt, cheeses, butter, cream, plant milks), **bread, rice, pasta, oats and grains**, **beans and pulses** (baked beans, lentils, chickpeas, hummus, tofu), **nuts and seeds**, oils, sugar, honey, jam, chocolate, and drinks (juice, cola, coffee, tea). **Raw and cooked are separate entries** where it matters, because people weigh them either way. Picking one saves it to your Products on the spot (no set-up) and selects it, so the amount box and quick buttons are right there. **Quick buttons add up real portions**: one large egg is 50 g, a slice of white bread 29 g, a medium banana 118 g, a cup of milk 244 g, a tablespoon of peanut butter 16 g (two slices is two taps), plus +50/+100/+150 g where there's no natural piece. Everything is **USDA FoodData Central values per 100 g** (public domain; each entry names its USDA record), so nothing is guessed. Foods you've used are found again in your own products, with no duplicates, and **Change food** puts the picker back. Not included: alcohol, branded products and ready meals (scan or photograph those instead).
- **Add your own loose foods:** on **Products → Add a generic food**, pick from the same list or type your own nutrition (a name and values per 100 g); there's also an optional **Not on the list? Fill it in with the model** button, which only estimates, so check it. A generic food needs no barcode, counts as its own single ingredient, and is rated like anything else. If you edit a picked food's numbers it's saved as your own values, not as USDA data. Any existing product can be switched to generic in its editor, and generic foods are found by name in the Shop check search. The list is generated by `scripts/build_foods.py` from the USDA dataset (add a food by adding its exact USDA name and re-running), and tests check every value is sane (energy must roughly match protein, carbs and fat) and spot-check values anyone can verify.
- **Export and import products:** under **Products → Export or import products**. **Export** saves a `larder-products` JSON file (no ids, dates, ratings or account details) containing only complete products: a barcode, ingredients and nutrition, or for generic foods just nutrition. The panel shows how many were left out and why. **Import** reads such a file and shows a **preview first**: how many would be added, which are skipped because you already have that barcode or name, and which are rejected (no name, missing or invalid barcode, no ingredients, no valid nutrition basis or values) with the reason for each. **Nothing you already have is ever changed or replaced**; this is enforced when saving, not just on screen, so a second import of the same file adds nothing and edits you made stay. Repeats inside the file are skipped too. Imported products get their own ids and are rated with your rules (a file may mark a product's source as Open Food Facts or USDA, and keeps its portions). In the Android app the exported file is saved with the system file picker, like backups.
- **Tweak the rating rules:** **Settings → Rating rules** lists every rule with the default points the rating uses (the starting score of 90, how many ingredients is “long”, sugar near the top, each nutrition rule, the additive-count tiers, and every individual additive). Change any number to make something matter more or less to you (0 switches a rule off), reset a single rule with ↺, add **your own rules** (a word or phrase and the points it adds or takes off, e.g. “palm oil” −10), or reset everything to the defaults. The rules are per account, only changes are stored, and every rating follows them immediately.
- **Ratings are never stored:** a rating is worked out from the saved ingredients and nutrition every time it's shown, so existing products and scans always follow the current rules and nothing needs recalculating (the barcode is the key, the ingredients are stored as text beside it; a rating sent by a client or found in an old file or backup is dropped). The rating popup says which rules version produced the number (for example “rating rules v5”) and the time it was calculated, so you can tell at a glance whether an app or server is on the latest rules. On Android the server is inside the app, so a rules change reaches your phone only after `make android` and a reinstall. To fix a misread list or add missing ingredients, edit the product on **Products**, or the **Ingredients** box in a **Shop check** result, then save (or **Photograph ingredients** to read them from a picture); the rating updates straight away.
- **Identify products by barcode:** products can store a barcode (EAN-8, UPC-A or EAN-13, checksum-verified). Photographing a label also reads the printed digits when visible; a misread code is dropped rather than guessed. In **Products** (add by barcode) or **Log food → Products**, you get two option cards with small pictures showing the difference, plus a box to type the digits. **Scan barcode** opens the live camera, which reads the barcode by itself with no picture taken (needs a secure page and a browser with `BarcodeDetector`, such as Chrome on Android; the card is greyed out with a note where that isn't available). **Photo** takes a picture, or lets you pick one from your gallery, and reads the barcode from it: on-device where the browser can, otherwise with your photo model, and the check digit must validate so a misread is rejected. Photo works on every device, including the Android app. In the Android app, Android asks for camera permission the first time you use live scan; if it was blocked, the message tells you to allow Camera for Larder in Android Settings, and offers **Use Photo instead**. A saved product is found instantly and offline, and saving a product with a barcode you already have updates it instead of duplicating it. Search matches barcodes too.
- **Online barcode lookup (optional):** off by default. The server's admin can turn on **Settings → Barcode lookup**. Then a barcode that isn't in your saved products is looked up on [Open Food Facts](https://world.openfoodfacts.org): name, ingredients and nutrition per 100 g/ml are filled in for you to check before saving. Only the barcode digits are sent. The data is community-edited, so verify it against the pack; products saved this way are marked as from Open Food Facts.
- **Save a meal made of several foods:** if you logged a meal as separate products (say 2 Weetabix, 75 g porridge oats and a scoop of protein powder), tap **Group as meal** on that meal in **Today**, tick the foods, and name it, e.g. “Usual breakfast”. Optionally combine them into a single log entry. The meal keeps the total nutrition and each food with its amount. Log it again from **Log food → Recent → Your saved meals** (your chosen servings and meal slot apply), or manage, rename and delete meals under **Meals** in the sidebar.
- **Fix meals in a plan:** on **Plans**, under “Meals you have the same every day”, choose a saved meal for breakfast, lunch, dinner or snack (or leave “Planned for me”). Every day of the plan includes it as-is, its foods go on the shopping list, it counts towards Meals a day, and the model plans only the other meals around what’s left of your targets. If every slot is fixed, no model request is made. Fixed meals get the same allergy and exclusion warnings as other recipes. The slot choices are remembered on this device.
- **Check food quality in the shop:** open **Shop check** (sidebar, the More grid on Profile, or the button on Today). Scan a barcode (live camera, a photo of it, or type the digits) and the result opens as a popup with the 0–100% rating, what marks it down or counts in its favour, the ingredients and nutrition per 100 g/ml. Tap **Why …%?** for the full explanation. **A rating needs both the ingredients and the nutrition: if either is missing there is no rating at all**, and the popup says what's still needed and lets you add it by photo or by typing the nutrition. Barcodes are looked up in your own products first, then Open Food Facts if an admin enabled it.
  - **Barcode not found anywhere:** you're asked for the ingredients and the nutrition panel (and the barcode): one photo showing everything works, or take separate photos, or type the nutrition. The barcode you already scanned is kept.
  - **Barcode known but ingredients or nutrition missing** (some Open Food Facts entries have none): the popup asks for the missing half. Ingredients Open Food Facts only has in another language are used but flagged **Provisional**, because some additives may be missed.
  - **Ingredients known but no barcode:** you're offered **Add the barcode too** (scan, photo or type it).
  - **Once both are known it becomes a real product** (listed under **Products**, rated, loggable, editable) and nothing is left over. Until then it's kept as an unfinished draft so you don't lose what you've added.
  - **Look up something you've saved:** the box on Shop check searches your products and unfinished drafts by name or barcode as you type (it waits for a short pause, and needs two characters). Tap a match to open its rating.
  Open Food Facts ingredients are requested in English first because the ratings read English. Ingredient photos are read by a model, so check the list against the pack.
- **Track your weight:** open **Weight**, enter a reading (one per day; saving again replaces it), and see the trend as a line chart. Filter it to 2 months, 6 months, 1 year, 2 years or all time (your choice is remembered on the device). Hover, tap or use the arrow keys to read exact values, and expand the table to review or delete readings. The summary shows your latest weight, the change over the chosen period and the low–high range. Weight is stored in kg and doesn’t change the targets on your Profile; use **Calculate targets** there when you want to update them.
- **Log workouts on a calendar:** open **Workouts**. Pick a day on the month grid, choose a kind from the drop-down (Running, Weights, Walking, Cycling, Swimming, HIIT, Yoga and Football to start), set a duration (quick buttons for 15–90 minutes, or type it), and optionally add calories burned as estimated by your watch or tracker, and a note. Choose **＋ Add a new kind…** to type your own, such as Padel; it’s saved to the list for next time and can be removed again under “Your own kinds”. You can log several sessions on one day. The month summary shows sessions, active days, total time, time per kind and total calories burned when you’ve entered them. The calories you burn on a day are shown on **Today** under that day’s totals. They are for your records and don’t change the daily calorie target; adjust the target yourself if you want to eat back some of them.
- **Adjust one day’s calories:** below the daily totals, select **Adjust this day’s calories**. Enter a target or use the ±100/250 kcal buttons, optionally add a note such as “Gym session”, and save. The daily rings and weekly chart use the adjusted target. **Reset to usual** removes the adjustment. Your Profile and protein, carb, fat and fibre targets stay unchanged. These dated log adjustments do not change the targets used for undated meal plans.
- **Replace one plan meal:** open a plan and choose **Replace meal** on the breakfast, lunch, dinner or snack you want to change. Add an optional preference. Other meals stay unchanged, day totals refresh, and the shopping list updates while keeping checked items and manual additions. Only the selected occurrence changes when a recipe is reused elsewhere in the plan.
- **Cook from your pantry:** type ingredients in **Pantry**, or use **Add from photos**, review the list, then choose **Suggest recipes**. Enable **Only use what I have** for ideas based on your pantry and staples. Plan shopping lists mark ingredients you already have.

Start by setting your stats, targets, diets and allergies in **Profile**, then generate a plan in **Plans**. Rate recipes to guide future suggestions; **Recipes** shows what the app has learned from those ratings.

Demo recipes and plans, their shopping lists, and food logs linked to demo recipes are hidden when a real model provider is saved in Settings. Switching back to Demo mode makes them visible again; the data is retained.

## Backup and restore

Open **Settings → Backup and restore** and choose **Download backup** to save a portable JSON file. On Android, choose where to save it with the system document picker; in a browser, it downloads normally.

Use **Restore from file** to select a backup. The app validates it and shows the export date and item counts before you choose **Replace data and restore**. Restore replaces your own profile and saved data; it does not merge records or affect another account. Download your current data first if you want to keep both versions.

Backups include logs and food quantities, saved meals, weight readings, workouts, unfinished shop scans, saved product nutrition, your rating rules, daily calorie adjustments, recipes, plans, shopping lists, pantry items and ratings. They exclude account credentials, model settings and API keys, and appearance preferences. Your current sign-in and model settings are retained after restore; configure the model separately when moving to a new installation. Only Larder backup files are accepted, up to 32 MB; the raw server data file is a different format.

## Data storage

All persistent data uses a JSON file, not SQLite. It includes profiles, logs and their food quantities, saved meals, weight readings, workouts, unfinished shop scans, your rating rules, saved products and nutrition values, day calorie adjustments, recipes, plans, shopping lists, pantry items and ratings. Hosted data is partitioned by user; Android keeps its file in the app’s private storage. Existing files gain the new collections automatically when loaded.

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
    barcode.go           barcode normalisation and GS1 checksum
    openfoodfacts.go     optional barcode lookup (off by default)
    ingredients_photo.go read an ingredients list from a photo (shop check fallback)
    product_rating.go    0–100 product score from ingredients and macros
    fixed.go             saved meals fixed into plan slots; quantity parsing
    shopping.go          aggregate, convert units, scale to household
    pantry.go            pantry matching, photo scan, cook-from-pantry
    categories.go        guesses the aisle for typed items
  internal/store/        JSON file store, data partitioned per user
  internal/api/          net/http handlers; auth.go has the middleware, local.go the on-device sign-in
  internal/webui/        the built web app, embedded with go:embed
  mobile/                gomobile entry point (its own module) + cmd/preview
web/src/
  lib/                   api client, types, format helpers, label portion maths, components
  routes/                Today, Plans, Pantry, Shopping, Recipes, Products, Shop check, Meals, Weight, Workouts, Profile, Settings, Login
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

- Go: table-driven tests for targets, safety rules, feedback, shopping maths, pantry matching and categories, the planner's and pantry cook's retry logic (with a scripted fake model), text and image requests for both providers (against `httptest` servers), the store (including per-user isolation and v1 migration), JWTs (expiry, tampering, `alg: none`), passwords, rate limiting, and end-to-end API journeys including multipart meal and nutrition-panel uploads, repeat logs, saved products, day calorie targets, and individual meal replacement with shopping-list updates; barcodes and the Open Food Facts mapping (against a fake server), product ratings (additives, sugars, edge cases) and shop scans (barcode order, ingredients photo, drafts becoming products, search, isolation), rating rules (overrides, switching rules off, your own words, validation, per-account), generic whole foods and the everyday foods list (243 foods across 12 categories, USDA values, portions, search, sanity checks per category), and product export/import (complete products only, preview, no overwrites, repeats, validation reasons), the need for both ingredients and nutrition, weight readings and workouts (validation, ordering, custom kinds, isolation), saved meals (grouping, logging, plan slots and shopping list) and fixed meals in prompts.
- Mobile: the on-device server (loopback only, one-time sign-in links, no second accounts, data surviving restarts).
- Web: Vitest for format and pantry-matching helpers, weight-chart ranges and gridlines, plus nutrition-label scaling for grams, millilitres and servings, corrected values and invalid inputs.

To add a test, copy the nearest table and add a row. `safety_test.go` is the easiest place to start: one line per case.

## Notes

- SvelteKit 3: config lives in `vite.config.ts` (no `svelte.config.js`), imports use `#lib/...` via `package.json` `imports` instead of `$lib`, and the service worker uses `$app/manifest` + `$app/env` instead of `$service-worker`.
- Use HTTPS (and `NUTRI_SECURE_COOKIES=1`) if you expose it beyond your own network.
- Metric units only for now.

## Ideas for next

- SQLite store (`modernc.org/sqlite` keeps it cgo-free)
- Add a day's workout calories burned to its calorie target automatically (today you can adjust the target yourself, and burned calories are shown but not applied)
- "Used it up" button on pantry recipes that removes ingredients from the pantry
- Password reset and changing email/password
- Share a pantry and plans within a household
- Streaming plan generation so long local-model runs show progress

**No package or database match:** in Shop check, open **No package or database match? Ask AI** and describe the food (brand, variety and country help). The configured model suggests nutrition and ingredients from its knowledge, without live web search. Review the values, choose **Use estimate**, then **Save AI estimate**. For a saved product, use the same option under **Profile → Products**, then **Save changes**. Existing values are kept; missing information is filled. AI estimates stay marked as unverified and their ratings remain provisional. Percentages are always calculated per request, never saved.

**Food chat:** open **Food chat** on Today or under **Profile**, or choose **Discuss this in Food chat** from a product/scan. Ask follow-up questions, request nutrition/ingredient lookups, then choose **Prepare product** or **Prepare meal** to review an editable draft. Only **Save product**, **Update product** or **Save meal** writes food data; saving a reusable meal does not log it. Changing meal foods or portions requires recalculating its nutrition before saving. Chat and draft state stay in the current browser session.

Food chat uses the configured model. With an OpenAI Responses-compatible connection, **Search the web** enables the real `web_search` tool through `/responses`; replies show whether a search actually ran and retain clickable citations. An unsupported model or API returns an error instead of silently pretending to search. Turn search off for ordinary conversation with other compatible providers. AI-assisted saved data retains its source notes and provisional ratings. Percentages are computed per request and are never stored. No default model is substituted and no additional search account is required for the OpenAI path.

**Coffee types:** Today → Log food → Products includes cappuccino, latte, iced latte, mocha, espresso and macchiato, with separate skimmed-milk cappuccino/latte options. Milk coffees offer USDA small/medium/large portions (12/16/20 fl oz); weights are shown because café sizes vary. Espresso uses shot measures. Values include the milk stated in the name; added sugar/syrups/cream are excluded except the chocolate syrup in mocha. The selected USDA FNDDS/SR Legacy source records are in `scripts/data/coffee-usda.json`; regenerate these entries with `python3 scripts/build_coffees.py backend/internal/nutrition/foods.json`.

**Product picker:** in Today → Log food → Products, search sits above Everyday foods. Saved products appear only in search results or when **Favourites** is opened. Tap ☆ next to a saved product (or after selecting a food) to favourite it. The quick list shows up to eight favourites; search finds the rest. Favourites persist with the account and can also be toggled on the Products page.
