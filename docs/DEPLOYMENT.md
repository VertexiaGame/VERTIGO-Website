# Deploying VERTEXIA

How to run the platform at `https://vertexia.wempos-forum.com` on Cloudflare
Workers + Containers, with gameservers on a dedicated Linux host. Read
[ARCHITECTURE.md](ARCHITECTURE.md) first for the moving parts.

Nothing here has been applied to a real account. Every step below is manual.

## 1. What you need

| Piece | Requirement |
| --- | --- |
| Cloudflare | Workers Paid plan (Containers), the `wempos-forum.com` zone, an R2 bucket |
| MySQL | 8.x reachable from the internet over TLS with a CA you can pin |
| Redis | 7.x over TLS (`rediss://`), AOF persistence (`appendonly yes`, `appendfsync everysec`), password auth, reachable from Cloudflare and the gameserver host |
| Gameserver host | Linux x86_64 with a public IPv4 (IPv6 optional), systemd, the `RaveEngineServer` binary and maps |
| Build machine | Docker (linux/amd64), Node 24 with pnpm 12.21.0 (`corepack enable`), Go 1.25 for the agent |

## 2. DNS

| Name | Type | Proxy | Points to |
| --- | --- | --- | --- |
| `vertexia.wempos-forum.com` | Worker custom domain | proxied (orange) | the `vertexia-platform` Worker |
| `games.vertexia.wempos-forum.com` | A (and AAAA if used) | **DNS only (grey)** | gameserver host public IP |

The game hostname must be grey-cloud: Cloudflare's proxy does not carry raw
UDP to a host on normal plans, and the platform puts the resolved IPs into
connect tokens. If the IP is static you can also set `GSM_PUBLIC_IP` and skip
the DNS lookup. The platform refuses to start in production when
`GSM_PUBLIC_HOST` equals the web hostname.

Add the custom domain under the Worker's Settings, Domains & Routes, or add a
`routes` entry with `custom_domain: true` to `cloudflare/wrangler.jsonc`.

## 3. Data services

**MySQL.** Create the database and a user limited to it. Set `DB_TLS=true`
and pass the server CA as `DB_TLS_CA_PEM` (PEM text; literal `\n` sequences
are accepted so it fits one secret line). Set `DB_TLS_SERVER_NAME` when the
certificate name differs from `DB_HOST`. `preferred` and `skip-verify` exist
for development only. Pool defaults: 25 open, 10 idle, 30 minute lifetime,
5 minute idle time.

**Redis.** Use one logical database for the platform and the agent. Keys used:
`session:`, `ratelimit:`, `auth:token:`, `studio:code:`, `studio:state:`,
`altcha:used:`, cooldowns, `gsm:` (records and locks), `gsmagent:` (agent
records and nonces) and the pub/sub channel `feed:events`. With
`APP_ENV=production` the app refuses to start without Redis.

**R2.** Create a bucket (for example `vertexia-uploads`) and an R2 API token
with Object Read & Write on that bucket only. Leave `R2_PUBLIC_BASE_URL`
unset unless you also add a rule that only serves the public prefixes
(`static/uploads/`, `static/renders/outfits/`, `static/renders/shop/`,
`assets/uploads/shop/`). Private keys such as `uploads/games/vrtx/<id>.vrtx`
and `uploads/assets/` have guessable names and must not be publicly
reachable. Without a public URL the app streams objects itself.

To migrate existing files, copy the repository's runtime directories
(`static/uploads`, `static/renders`, `assets/uploads`, `uploads`) into the
bucket with the same relative keys, for example with `rclone copy`.

## 4. Gameserver host

1. Create a `gsm` system user and the directories `/etc/vertexia`,
   `/var/lib/gsm-agent`, `/var/log/gsm-agent`, `/opt/vertexia`.
2. Install `RaveEngineServer` and its maps under `/opt/vertexia`.
3. Build and copy the agent:

   ```sh
   GOTOOLCHAIN=auto CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o gsm-agent ./cmd/gsm-agent
   install -m 0755 gsm-agent /usr/local/bin/gsm-agent
   ```

4. Copy `deploy/gsm-agent/gsm-agent.env.example` to
   `/etc/vertexia/gsm-agent.env` (mode 0640, `root:gsm`) and fill in real
   values. Copy `deploy/gsm-agent/gsm-agent.service` to
   `/etc/systemd/system/` and run `systemctl enable --now gsm-agent`.
5. Firewall (nftables, ufw or the provider's firewall):
   * allow UDP on `GSM_PORT_RANGE` from anywhere;
   * allow the agent port (default TCP 8443) **only** from the platform. Either
     run `cloudflared` on the host and listen on `127.0.0.1:8443`
     (`GSM_AGENT_ALLOW_HTTP=true` is accepted only on loopback), restrict the
     port to Cloudflare's egress ranges, or terminate mTLS in front of it. The
     HMAC signature already protects every route, so this is defense in depth;
   * deny everything else inbound except SSH.
6. The agent's TLS certificate must be valid for the hostname in
   `GSM_AGENT_URL` (production requires `https`). A Cloudflare Tunnel hostname
   gives you this for free.

Stopping or restarting the agent leaves running gameservers alone
(`KillMode=process`); on start it adopts them from Redis.

## 5. Platform configuration

Non-secret values live in `cloudflare/wrangler.jsonc` `vars`. Secrets are set
with `pnpm exec wrangler secret put NAME` from `cloudflare/`. The Worker only
forwards variables listed in `APP_VARS` in `cloudflare/src/index.ts`.

| Variable | Secret | Default | Notes |
| --- | --- | --- | --- |
| `APP_ENV` | | | `production` enables strict validation |
| `PUBLIC_BASE_URL` | | | `https://vertexia.wempos-forum.com` |
| `RUN_WITH_DATABASE` | | | `true` |
| `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER` | yes | | |
| `DB_PASS` | yes | | |
| `DB_TLS` | | off | `true` in production |
| `DB_TLS_CA_PEM` | yes | | CA certificate PEM |
| `DB_TLS_SERVER_NAME` | yes | | when it differs from `DB_HOST` |
| `DB_TIMEOUT` | | `10s` | dial, read and write timeout |
| `DB_MAX_OPEN_CONNS`, `DB_MAX_IDLE_CONNS`, `DB_CONN_MAX_LIFETIME`, `DB_CONN_MAX_IDLE_TIME` | | 25, 10, 30m, 5m | |
| `REDIS_URL` | yes | | `rediss://user:pass@host:port/db` |
| `REDIS_ADDR`, `REDIS_PASSWORD` (secret), `REDIS_DB`, `REDIS_TLS` | | | alternative to `REDIS_URL` |
| `REDIS_REQUIRED` | | `true` in production | |
| `SESSION_SECURE` | | | `true` |
| `SESSION_SAMESITE` | | `Lax` | |
| `SESSION_IDLE_TIMEOUT`, `SESSION_ABSOLUTE_TIMEOUT` | | | |
| `TRUST_PROXY`, `PROXY_HEADER`, `TRUSTED_PROXIES` | | | set to `true`, `X-Forwarded-For`, `0.0.0.0/0,::/0` in `wrangler.jsonc`. Safe only because the container has no public ingress and the Worker rewrites `X-Forwarded-For` |
| `ALTCHA_HMAC_KEY` | yes | | captcha key |
| `REGISTRATION_IP_SALT` | yes | `ALTCHA_HMAC_KEY` | |
| `GAMESERVER_API_KEY` | yes | | shared with the agent and gameservers |
| `GSM_ENABLED` | | `true` | |
| `GSM_AGENT_URL` | yes | | for example `https://gsm.vertexia.wempos-forum.com`; required in production |
| `GSM_AGENT_SECRET` | yes | | at least 32 characters; same as the agent |
| `GSM_AGENT_TIMEOUT` | | `10s` | |
| `GSM_PUBLIC_HOST` | | | `games.vertexia.wempos-forum.com` |
| `GSM_PUBLIC_IP` | | | optional comma list that skips DNS |
| `GSM_MAX_PLAYERS_DEFAULT`, `GSM_MIN_PLAYERS_DEFAULT` | | 100, 2 | |
| `GSM_IDLE_TIMEOUT`, `GSM_REAP_INTERVAL` | | 5m, 15s | |
| `GSM_HEARTBEAT_TIMEOUT` | | `90s` | |
| `GSM_MAX_SERVERS` | | 0 (no cap) | global, enforced under a Redis lock |
| `GSM_READY_TIMEOUT` | | `30s` | |
| `NETCODE_PRIVATE_KEY` | yes | | 64 hex characters; same as the agent. Generate with `openssl rand -hex 32` |
| `NETCODE_PROTOCOL_ID` | yes | | unsigned integer; same as the agent and client |
| `NETCODE_TOKEN_TTL` | | `60s` | connect token lifetime |
| `CLIENT_TOKEN_TTL`, `STUDIO_TOKEN_TTL` | | 24h, 168h | access token lifetimes |
| `R2_ACCOUNT_ID`, `R2_BUCKET` | yes | | R2 is required in production |
| `R2_ACCESS_KEY_ID`, `R2_SECRET_ACCESS_KEY` | yes | | |
| `R2_PUBLIC_BASE_URL` | | | see the R2 warning above |
| `R2_ENDPOINT` | | derived from account id | |
| `STORAGE_DIR` | | `.` | local storage root (development) |
| `UPLOAD_MAX_BYTES` | | 40 MiB | global upload ceiling |
| `LIMITER_MAX`, `LIMITER_EXPIRATION` | | | global rate limit |
| `FILTER_ENABLED`, `TYPESAFE_API_KEY` (secret), `TYPESAFE_BASE_URL`, `TYPESAFE_MODEL`, `FILTER_TIMEOUT` | | | text filter |
| `SITELOCK_ENABLED`, `SITELOCK_PASSWORD` (secret) | | | |
| `LIVETIMER_ENABLED`, `LIVETIMER_DURATION` | | | |
| `INSTANCE_COUNT` | | 2 | Worker only: instances to spread requests over (max 4) |

The agent's variables are documented in
`deploy/gsm-agent/gsm-agent.env.example`. Secrets there: `GSM_AGENT_SECRET`,
the TLS key file, `REDIS_URL` / `REDIS_PASSWORD`, `NETCODE_PRIVATE_KEY`,
`GAMESERVER_API_KEY`.

Clients (RaveEngine client and Studio) read the platform origin from
`VERTEXIA_API_DOMAIN` (legacy `VERTIGO_API_DOMAIN`) and default to
`https://vertexia.wempos-forum.com`. Their `NETCODE_PROTOCOL_ID` must match.

## 6. Container

`/Dockerfile` builds a static Go binary (pinned `golang:1.25.14-alpine3.24`
by digest) into `alpine:3.24.4` (pinned by digest), runs as uid 10001,
listens on `PORT=8080`, and only the runtime directories are writable. The
Worker class is `VertexiaPlatform` with binding `VERTEXIA_PLATFORM`, instance
type `standard-1`, `max_instances: 4`. Health endpoints:

* `GET /healthz`: process is up.
* `GET /readyz`: MySQL and Redis respond.

Sessions, rate limits, tokens and gameserver state live in Redis, so requests
can land on any instance.

## 7. Build and deploy

```sh
cd cloudflare
pnpm install --frozen-lockfile
pnpm exec tsc --noEmit
pnpm exec wrangler login
# once per secret listed above:
pnpm exec wrangler secret put DB_PASS
pnpm exec wrangler deploy --dry-run   # builds the image locally, uploads nothing
pnpm exec wrangler deploy             # builds, pushes the image, deploys the Worker
```

Then check:

```sh
curl -fsS https://vertexia.wempos-forum.com/healthz
curl -fsS https://vertexia.wempos-forum.com/readyz
```

and, as an admin, the gameserver health page (`/api/v1/admin/gs/health`),
which reports the agent's `/v1/health` in remote mode.

## 8. Rollback

* Worker and container: `pnpm exec wrangler deployments list`, then
  `pnpm exec wrangler rollback <version-id>`. Container images are versioned
  with the Worker.
* Agent: keep the previous binary as `/usr/local/bin/gsm-agent.prev`; swap it
  back and `systemctl restart gsm-agent`. Running gameservers keep running.
* Data: schema is unchanged by this work. Redis holds only rebuildable state
  (users log in again; gameservers are re-registered by their heartbeats or
  reaped).

## 9. Security notes

* The gameserver API key is accepted only from headers. The Unikey is still
  accepted from `?ukey=` for the shipped client and gameserver; move them to
  bearer tokens or headers, then remove the query form.
* The Studio poll endpoint returns the authorization code to whoever knows
  the client-chosen `state`. Binding the flow to a PKCE verifier needs a
  matching Studio change.
* Pending banners, clan icons and shop items still live under public
  prefixes, as before.
