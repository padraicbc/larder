# Run Larder on a server and install it on your phone

The server is one binary with the web app inside. Put it somewhere your phone
can reach over **HTTPS** (Android only installs web apps from HTTPS sites),
open it in Chrome, and choose **Install app**.

Pick one of the two setups below.

## Build

```sh
make build          # -> backend/bin/larder (about 9 MB, no other files needed)
# or
make docker         # -> image "larder"
```

## Option A: Tailscale (private, no domain needed)

Best if only you and your household use it. Nothing is exposed to the internet.

1. Install [Tailscale](https://tailscale.com/download) on the server and on your phone, signed into the same account.
2. Run Larder on the server:

   ```sh
   export NUTRI_JWT_SECRET=$(openssl rand -hex 32)   # keep this somewhere safe
   NUTRI_SECURE_COOKIES=1 NUTRI_TRUST_PROXY=1 ./backend/bin/larder
   ```

3. Give it an HTTPS address on your tailnet:

   ```sh
   tailscale serve --bg 8080
   ```

   This prints something like `https://myserver.tail1234.ts.net`. (Turn on HTTPS
   certificates in the Tailscale admin console if it asks.)

4. On your phone, open that address in Chrome, create your account, then open the
   ⋮ menu and choose **Install app** (or use the **Install app** button in Larder's Settings).

## Option B: Your own domain with Caddy (public)

Use this if you want to reach it without Tailscale. You need a domain name pointing
at the server, with ports 80 and 443 open.

### With Docker Compose

```sh
cat > .env <<'ENV'
LARDER_DOMAIN=larder.example.com
NUTRI_JWT_SECRET=paste-output-of-openssl-rand-hex-32
ENV
docker compose up -d
```

Caddy gets the HTTPS certificate automatically. Data lives in the `larder-data` volume.

### Without Docker (systemd + Caddy)

`/etc/larder.env`:

```sh
NUTRI_JWT_SECRET=paste-output-of-openssl-rand-hex-32
NUTRI_SECURE_COOKIES=1
NUTRI_TRUST_PROXY=1
NUTRI_DATA=/var/lib/larder/larder.json
NUTRI_ADDR=127.0.0.1:8080
```

`/etc/systemd/system/larder.service`:

```ini
[Unit]
Description=Larder
After=network-online.target

[Service]
ExecStart=/usr/local/bin/larder
EnvironmentFile=/etc/larder.env
DynamicUser=yes
StateDirectory=larder
Restart=on-failure
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes

[Install]
WantedBy=multi-user.target
```

`/etc/caddy/Caddyfile`:

```
larder.example.com {
	reverse_proxy 127.0.0.1:8080
}
```

```sh
sudo cp backend/bin/larder /usr/local/bin/
sudo systemctl enable --now larder
sudo systemctl reload caddy
```

## After the first account

The first account is the admin. Once everyone in your household has signed up,
set `NUTRI_SIGNUP=closed` and restart so strangers can't create accounts.

## Using Ollama on another computer

Run Ollama so it accepts connections from your network (`OLLAMA_HOST=0.0.0.0 ollama serve`),
then in Larder's Settings choose **Ollama** and change the base URL to that computer,
e.g. `http://192.168.1.20:11434/v1` or its Tailscale name.

## Backups and updates

Users can export and restore their own data from **Settings → Backup and restore**. The portable backup includes the profile and all saved app data, including product nutrition and daily calorie adjustments. Restore validates the file, shows a summary and requires confirmation before replacing that user’s data. It does not affect other accounts, server settings or credentials. Portable backups exclude model settings and API keys and are limited to 32 MB.

For a complete server backup, including accounts and server configuration, copy the raw data file as described below. The raw server file and portable user backup have different formats; the Settings importer accepts only portable Larder backups.

- All data is one JSON file (`NUTRI_DATA`), including saved product nutrition, meal quantities,
  day calorie adjustments, plans, pantry items and ratings. It uses JSON rather than SQLite.
  It’s written atomically, so copying it while
  Larder runs is safe: `cp /var/lib/larder/larder.json backup-$(date +%F).json`.
- Existing JSON files load the new product and day-adjustment collections automatically.
- To update, rebuild and replace the binary (or `docker compose up -d --build`).
  Installed web apps pick up the new version on next launch. Native Android APKs must
  be rebuilt and reinstalled separately; see [ANDROID.md](ANDROID.md).
- Keep `NUTRI_JWT_SECRET` the same across updates, or everyone gets signed out.

## Environment variables

| Variable | Default | |
|---|---|---|
| `NUTRI_JWT_SECRET` | generated, stored in the data file | At least 32 bytes. Set it in production. |
| `NUTRI_SECURE_COOKIES` | off | `1` behind HTTPS |
| `NUTRI_TRUST_PROXY` | off | `1` behind Caddy/Tailscale, so login rate limits see real client IPs |
| `NUTRI_SIGNUP` | `open` | `closed` = only the first account can register |
| `NUTRI_LLM_API_KEY` | | fallback model key |
| `NUTRI_DATA` | `data/nutrition.json` | |
| `NUTRI_ADDR` | `:8080` | |
| `NUTRI_STATIC` | | serve the web app from a folder instead of the embedded copy |
