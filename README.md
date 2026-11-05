# Larder

A nutrition tracker and meal planner: log macros, generate meal plans with shopping lists, cook from what's in your fridge (type it or snap a photo), and rate meals so future suggestions get better. Multi-user with JWT auth. Go backend, SvelteKit + daisyUI frontend, any LLM you like.

## Quick start

```sh
cd web && npm install && cd ..
make api    # terminal 1: Go API on :8080 (data in backend/data/nutrition.json)
make web    # terminal 2: open http://localhost:5173
```

The first account you create is the **admin** (the only one who can change model settings). It starts in **demo mode** (canned recipes and a pretend fridge photo, no model needed), so you can click around straight away. Then open **Settings** to plug in a model:

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

## Try this first

1. **Profile**: add your stats, press **Calculate targets**, tick `vegetarian`, add an allergy. Save.
2. **Plans**: generate a 3-day plan. Open the shopping list and tick a few things.
3. Rate a couple of recipes (one high, one low, with a comment).
4. **Recipes**: the "What your ratings are teaching it" panel shows what was learned.
5. **Today**: log food by describing it ("two eggs on toast") or from a recipe.
6. **Pantry**: type `eggs, spinach, feta`, or tap **Add from photos**. Tick what's right, then **Suggest recipes**. Turn on **Only use what I have** for zero-shopping ideas.
7. Open a plan's shopping list: items already in your pantry are marked.

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
    shopping.go          aggregate, convert units, scale to household
    pantry.go            pantry matching, photo scan, cook-from-pantry
    categories.go        guesses the aisle for typed items
  internal/store/        JSON file store, data partitioned per user
  internal/api/          net/http handlers; auth.go has the middleware, local.go the on-device sign-in
  internal/webui/        the built web app, embedded with go:embed
  mobile/                gomobile entry point (its own module) + cmd/preview
web/src/
  lib/                   api client, types, format helpers, components
  routes/                Today, Plans, Pantry, Shopping, Recipes, Profile, Settings, Login
  service-worker/        caches the app shell for fast, offline-tolerant starts
android/                 Kotlin WebView shell for the APK
```

**The feedback loop:** `nutrition.Summarize` averages ratings per recipe, then scores each ingredient and tag by `(avg - 3)` across the recipes it appears in. Loved and disliked recipes (with your comments) plus the top ingredients go into the next plan's prompt.

**Auth:** register/login return a JWT (HS256, 30 days) in an `HttpOnly`, `SameSite=Strict` cookie scoped to `/api`, so page scripts can't read it and other sites can't use it. The same token is in the JSON response for scripts and mobile clients (`Authorization: Bearer …`). Passwords use PBKDF2-SHA256 with 600k iterations from Go's standard library, so the only new dependency is `golang-jwt/jwt`. Logins are rate-limited per IP + email (5 failures per 15 minutes) and unknown emails take the same time as wrong passwords. Every store method takes a user ID, and tests check that one user gets 404s for another's plans, recipes and pantry.

Known limits: logging out clears the cookie, but a copied token stays valid until it expires. If you need "sign out everywhere", add a `TokenVersion` to `User`, put it in the claims, and bump it on logout. Behind a reverse proxy the rate limiter sees the proxy's IP; read `X-Forwarded-For` in `clientIP` if you trust your proxy.

**Photos:** the browser shrinks each photo to 1600px JPEG before upload (which also strips EXIF/GPS). The server checks the real file type from its bytes, caps size and count, and sends the images to the vision model. Nothing is saved until the user reviews the list.

**Safety:** LLMs ignore constraints sometimes. `CheckRecipe` runs on every generated recipe. If anything fails, the model gets one retry with the problems listed; if it still fails, the recipe is saved with a visible warning rather than silently. The keyword lists in `safety.go` are deliberately simple; extend them as you find gaps. This is a helper, not a guarantee, for serious allergies.

## Tests

```sh
make test     # Go (server + mobile module) and web
make check    # go vet + svelte-check
```

- Go: table-driven tests for targets, safety rules, feedback, shopping maths, pantry matching and categories, the planner's and pantry cook's retry logic (with a scripted fake model), text and image requests for both providers (against `httptest` servers), the store (including per-user isolation and v1 migration), JWTs (expiry, tampering, `alg: none`), passwords, rate limiting, and end-to-end API journeys including multipart photo uploads.
- Mobile: the on-device server (loopback only, one-time sign-in links, no second accounts, data surviving restarts).
- Web: Vitest for the format and pantry-matching helpers.

To add a test, copy the nearest table and add a row. `safety_test.go` is the easiest place to start: one line per case.

## Notes

- SvelteKit 3: config lives in `vite.config.ts` (no `svelte.config.js`), imports use `#lib/...` via `package.json` `imports` instead of `$lib`, and the service worker uses `$app/manifest` + `$app/env` instead of `$service-worker`.
- Use HTTPS (and `NUTRI_SECURE_COOKIES=1`) if you expose it beyond your own network.
- Metric units only for now.

## Ideas for next

- SQLite store (`modernc.org/sqlite` keeps it cgo-free)
- Barcode scanning / Open Food Facts lookup for logging
- Swap a single meal in a plan ("regenerate this one")
- "Used it up" button on pantry recipes that removes ingredients from the pantry
- Password reset and changing email/password
- Share a pantry and plans within a household
- Streaming plan generation so long local-model runs show progress
