# TrueNAS Agent: Deploy Homebox (QR-login fork)

Copy everything below this line into the TrueNAS agent chat.

---

## Goal

Deploy **Homebox** from the fork `https://github.com/dylanl321/homebox` (includes custom QR login) on this TrueNAS SCALE host as a Docker custom app. Persist data on a ZFS dataset. Expose the UI on port **3100** (or the next free port).

Do **not** use the official `ghcr.io/sysadminsmedia/homebox` image — it does not include QR login.

## Recommended approach (best)

1. Use the already-built image (see **Image availability** below).
2. Install via TrueNAS **Apps → Discover Apps → ⋮ → Install via YAML**.
3. Bind-mount a dedicated dataset to `/data` inside the container.

### Image availability (as of last build)

- Git commit on `main`: includes QR login (`feat: add QR code device login…`).
- Local build tags: `ghcr.io/dylanl321/homebox:main` and `ghcr.io/dylanl321/homebox:fork-qr-fd38fb5f` (≈199MB).
- If GHCR pull fails (package not published yet), use **Option C: load from tar** below.

Do **not** try to `docker build` inside the TrueNAS Apps YAML path unless Dockge/Portainer with a host build context is already available.

## Prerequisites to verify

- TrueNAS SCALE **24.10+** (Electric Eel or newer) with Apps pool configured
- Dataset for app data, e.g. `/mnt/<pool>/apps/homebox` (create if missing; ACL/owner can stay root for the default image)
- Port **3100/tcp** free on the host (pick another host port if not)
- Network path from phones/LAN to `http://<truenas-ip>:3100` (required for QR login scans)

## Secrets / config to set (generate if missing)

- `HBOX_AUTH_API_KEY_PEPPER`: long random string (≥32 chars). Generate with `openssl rand -base64 48`. **Keep stable** across upgrades or API keys break.
- `HBOX_OPTIONS_HOSTNAME`: the exact public/LAN base URL users open in a browser, e.g. `http://192.168.1.50:3100` or `https://homebox.example.com`. QR codes encode the browser origin; this hostname should match how users reach the app.
- Optional later: reverse proxy + TLS; then set hostname to the HTTPS URL and enable `HBOX_OPTIONS_TRUST_PROXY=true` only if the proxy forwards `X-Forwarded-*` correctly.

## Exact compose to install

App name: `homebox`

```yaml
services:
  homebox:
    image: ghcr.io/dylanl321/homebox:main
    container_name: homebox
    restart: unless-stopped
    ports:
      - "3100:7745"
    environment:
      HBOX_LOG_LEVEL: info
      HBOX_LOG_FORMAT: text
      HBOX_MODE: production
      HBOX_WEB_MAX_UPLOAD_SIZE: "50"
      HBOX_OPTIONS_ALLOW_ANALYTICS: "false"
      HBOX_OPTIONS_ALLOW_REGISTRATION: "true"
      HBOX_AUTH_API_KEY_PEPPER: "<PASTE_GENERATED_PEPPER>"
      HBOX_OPTIONS_HOSTNAME: "http://<TRUENAS_LAN_IP>:3100"
    volumes:
      - /mnt/<POOL>/apps/homebox:/data
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "-O", "-", "http://localhost:7745/api/v1/status"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 20s
```

Replace `<PASTE_GENERATED_PEPPER>`, `<TRUENAS_LAN_IP>`, and `<POOL>` with real values before saving.

## Deployment steps (do these)

1. Ensure dataset `/mnt/<POOL>/apps/homebox` exists.
2. If the image is on GHCR and the repo is public, pull should work anonymously. If pull fails with 401/403, either make the package public or configure a GHCR pull credential for Apps.
3. Apps → Discover Apps → ⋮ → **Install via YAML**.
4. Name: `homebox`. Paste the compose above (with replacements). Save / Deploy.
5. Wait until the app is **Running** and healthcheck is healthy.
6. Verify:
   - `http://<TRUENAS_LAN_IP>:3100` loads the Homebox UI
   - `http://<TRUENAS_LAN_IP>:3100/api/v1/status` returns JSON health
7. Create the first admin account (registration enabled).
8. QR login check: Profile → QR Login → generate code → scan from a phone on the same network → phone should land signed in.

## If GHCR image is not available (Option B)

Build and push from a machine with Docker (not required on TrueNAS itself):

```bash
git clone https://github.com/dylanl321/homebox.git
cd homebox
docker build -t ghcr.io/dylanl321/homebox:main \
  --build-arg VERSION=fork-qr \
  --build-arg COMMIT=$(git rev-parse --short HEAD) \
  --build-arg BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
  -f Dockerfile .
echo $GHCR_TOKEN | docker login ghcr.io -u dylanl321 --password-stdin
docker push ghcr.io/dylanl321/homebox:main
```

Then retry the TrueNAS YAML install using that image tag.

Alternatively, open https://github.com/dylanl321/homebox/actions once and enable Actions on the fork, then push to `main` so the `Docker publish` workflow can publish `ghcr.io/dylanl321/homebox`.

## Option C: Load a prebuilt tar on TrueNAS (no registry)

If a `homebox-main.tar` was copied to the NAS (e.g. `/mnt/<POOL>/apps/homebox/homebox-main.tar`):

```bash
docker load -i /mnt/<POOL>/apps/homebox/homebox-main.tar
docker images | grep homebox
```

Then use this compose image line (match whatever tag `docker load` printed; usually `ghcr.io/dylanl321/homebox:main`):

```yaml
image: ghcr.io/dylanl321/homebox:main
```

Proceed with **Install via YAML** as above.

## Upgrades

1. Pull newer image tag / `:main`.
2. Update the custom app image tag (or recreate YAML).
3. Keep the **same** `/data` mount and the **same** `HBOX_AUTH_API_KEY_PEPPER`.
4. Migrations (including `qr_login_tokens`) run automatically on startup.

## Do not

- Do not use official `ghcr.io/sysadminsmedia/homebox` for this deploy
- Do not wipe `/data` unless intentionally resetting the instance
- Do not bind host ports 80/443 if the TrueNAS UI already owns them
- Do not enable `HBOX_OPTIONS_TRUST_PROXY` without a correctly configured reverse proxy

## Success criteria

- App Running on TrueNAS Apps list
- UI reachable on LAN port 3100
- `/api/v1/status` healthy
- QR login from a phone completes without manual password entry

## Report back

When done, report: dataset path used, published host port, image digest/tag, UI URL, healthcheck result, and whether QR login was verified.
