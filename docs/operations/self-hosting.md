# Deploy Piik Server

English · [简体中文](./self-hosting.zh-CN.md) · [Documentation](../README.md)

Piik Server packages the web interface, room management and optional media
forwarding in **one server binary**. Extract and run it; room data is stored in SQLite.

## Try it locally

These server instructions use a Linux x64 machine.

This is a private adaptation of [TNTcraftHIM/Piik](https://github.com/TNTcraftHIM/Piik),
retaining the MIT license. Upstream binaries and images do not contain these fixes.
Check out this repository's adaptation branch and build from a clean checkout
with Node (see `.node-version`) and Go 1.26.8. The output directory must be new
and outside the repository:

```sh
npm ci
node scripts/package-server-release.mjs ../piik-runtime
```

Extract the resulting **`piik-<revision>-runtime.tar.gz`** into the service directory and run:

```sh
./piik-server
```

Open `http://localhost:8787`. This is a local trial; use the configuration below
to let friends connect over the internet. To start a temporary room on your computer,
use the [Piik App guide](../guide/getting-started.md).

## Docker Compose

On a Linux x64 server with Docker Compose v2, build this fork's image from a clean
repository root, then copy the deployment files. The output directory must be new:

```sh
npm ci
node scripts/package-server-release.mjs ../piik-container --container-image piik:self-hosted
mkdir -p ../piik-deploy
cp deploy/container/compose.yaml ../piik-deploy/
cp deploy/container/.env.example ../piik-deploy/.env
cd ../piik-deploy
```

Replace `share.example.com` in `.env` with your domain and set a non-blank private
`SITE_ACCESS_PASSWORD`, then start Piik:

```sh
docker compose run --rm piik --check-config
docker compose up -d
```

The local `piik:self-hosted` image includes the Web UI, signaling, STUN
and optional SFU. Complete [HTTPS](#2-enable-https) and
[firewall configuration](#3-open-the-ports-and-verify) below. The default is P2P;
the sample `.env` also shows how to enable SFU fallback. Keep the `piik-data`
volume, which stores room data and optional diagnostics. See
[container maintenance](./service-management.md#container) for updates and backups.

## Put it online

You need a Linux x64 server and a domain pointing to its public IP.
The example uses `share.example.com`; replace it with your domain.

### 1. Configure and start Piik

Create a `.env` file next to the binary:

```dotenv
PIIK_ENV=production
LISTEN_HOST=127.0.0.1
PUBLIC_BASE_URL=https://share.example.com
STUN_URLS=stun:share.example.com:3478
MAX_VIEWERS_PER_ROOM=20
SITE_ACCESS_PASSWORD=
TRUSTED_PROXY_CIDRS=127.0.0.1/32,::1/128
```

Fill in `SITE_ACCESS_PASSWORD` without committing the real password, then run
these commands from that directory as a regular user:

```sh
./piik-server --check-config
./piik-server
```

The server reads `.env` automatically and stores rooms in `rooms.sqlite` in the
working directory. Production rejects a blank site password. New Browser profiles
default to invitation-only rooms; existing rooms and explicitly saved policies
are preserved. Invited Viewers can still join that room directly.
Login and room creation have per-client IP limits and return HTTP 429 when exceeded;
see [bounds](../standards/configuration.md#bounds). The proxy trust setting above
is for a same-host proxy only. Containers need the actual proxy source IP/CIDR.
Proxies must overwrite client-supplied `X-Forwarded-For`; never trust arbitrary peers.

`MAX_VIEWERS_PER_ROOM` sets the room's Viewer limit, excluding the Host. Choose
`1..20` and restart to apply it. More Viewers can require more network and relay
resources. See [room capacity](../standards/configuration.md#room-capacity)
for defaults and the difference from App rooms.

### 2. Enable HTTPS

Use your existing HTTPS reverse proxy to forward to `127.0.0.1:8787` with
WebSocket support. If you do not have one, [install Caddy](https://caddyserver.com/docs/install)
and add this to its Caddyfile:

```text
share.example.com {
    reverse_proxy 127.0.0.1:8787
}
```

Reload Caddy. It obtains and renews the certificate automatically when DNS points
to the server and TCP 80/443 are reachable. See [Caddy's HTTPS proxy guide](https://caddyserver.com/docs/quick-starts/reverse-proxy#https).

### 3. Open the ports and verify

Allow **TCP 80/443** for HTTPS and **UDP 3478** for STUN in the server firewall
and cloud security group. Keep TCP 8787 private. The STUN hostname must resolve
directly to the server; a CDN HTTP proxy does not forward its UDP traffic.

Open `https://share.example.com/healthz`; it should return `{"status":"ok"}`.
Then open the site, share a screen and join from another device. This initial
configuration uses P2P media, so participants need a usable UDP path.

## Optional: media fallback

Add `SFU_UDP_PORT=7882` to `.env`, allow UDP 7882, and restart Piik to enable
automatic SFU fallback. When the server is behind NAT, also set `SFU_PUBLIC_IP`
to its reachable public IPv4 address. The same binary provides the fallback.
The Host must turn off **Privacy mode** before sharing to allow this route.
Both direct and SFU media need a usable UDP connection.
TURN/TCP/TLS relaying is not implemented, so UDP-blocked networks are not guaranteed
to connect. Room HTTP APIs, `/signal` WebSocket, self-hosted STUN and optional SFU
remain available. P2P media uses participants' bandwidth; the server provides the
page, authorization and signaling, and carries media only when SFU is used.
This deployment does not require the upstream demo or cloudflared; the latter
belongs to the App's public-invite mode.

## Keep it running and update

For automatic startup, use the [systemd or container guide](./service-management.md).
Keep `.env` and `rooms.sqlite` across updates. Back up room data while the server is
stopped, replace the executable with the new release, then restart and check
health and room access. Read release notes before an upgrade that changes data formats.

Automatic release and website publication are off by default for this adaptation.
Set the GitHub Actions repository variables `PIIK_RELEASES_ENABLED=true` or
`PIIK_WEBSITE_ENABLED=true` and the required permissions only when publication is intended.

[All settings and ports](../standards/configuration.md) ·
[Maintainer release tooling](../deployment.md)
