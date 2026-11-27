import { Container, getRandom, type StopParams } from "@cloudflare/containers";
import type { DurableObject } from "cloudflare:workers";

/**
 * App variables forwarded from the Worker env into the container process.
 * Plain vars live in wrangler.jsonc; secrets are set with `wrangler secret put`.
 */
const APP_VARS = [
  "APP_ENV",
  "PUBLIC_BASE_URL",
  "RUN_WITH_DATABASE",
  "DB_USER",
  "DB_PASS",
  "DB_HOST",
  "DB_PORT",
  "DB_NAME",
  "DB_TLS",
  "DB_TLS_CA_PEM",
  "DB_TLS_SERVER_NAME",
  "DB_TIMEOUT",
  "DB_MAX_OPEN_CONNS",
  "DB_MAX_IDLE_CONNS",
  "DB_CONN_MAX_LIFETIME",
  "DB_CONN_MAX_IDLE_TIME",
  "SERVER_READ_TIMEOUT",
  "SERVER_WRITE_TIMEOUT",
  "SERVER_IDLE_TIMEOUT",
  "SESSION_SECURE",
  "SESSION_SAMESITE",
  "SESSION_IDLE_TIMEOUT",
  "SESSION_ABSOLUTE_TIMEOUT",
  "ALTCHA_HMAC_KEY",
  "REGISTRATION_IP_SALT",
  "TRUST_PROXY",
  "PROXY_HEADER",
  "TRUSTED_PROXIES",
  "GAMESERVER_API_KEY",
  "REDIS_URL",
  "REDIS_ADDR",
  "REDIS_PASSWORD",
  "REDIS_DB",
  "REDIS_TLS",
  "REDIS_REQUIRED",
  "GSM_ENABLED",
  "GSM_AGENT_URL",
  "GSM_AGENT_SECRET",
  "GSM_AGENT_TIMEOUT",
  "GSM_PUBLIC_HOST",
  "GSM_PUBLIC_IP",
  "GSM_MAX_PLAYERS_DEFAULT",
  "GSM_MIN_PLAYERS_DEFAULT",
  "GSM_IDLE_TIMEOUT",
  "GSM_REAP_INTERVAL",
  "GSM_HEARTBEAT_TIMEOUT",
  "GSM_MAX_SERVERS",
  "GSM_READY_TIMEOUT",
  "NETCODE_PRIVATE_KEY",
  "NETCODE_PROTOCOL_ID",
  "NETCODE_TOKEN_TTL",
  "CLIENT_TOKEN_TTL",
  "STUDIO_TOKEN_TTL",
  "R2_ACCOUNT_ID",
  "R2_BUCKET",
  "R2_ACCESS_KEY_ID",
  "R2_SECRET_ACCESS_KEY",
  "R2_PUBLIC_BASE_URL",
  "R2_ENDPOINT",
  "UPLOAD_MAX_BYTES",
  "STORAGE_DIR",
  "LIMITER_MAX",
  "LIMITER_EXPIRATION",
  "FILTER_ENABLED",
  "TYPESAFE_API_KEY",
  "TYPESAFE_BASE_URL",
  "TYPESAFE_MODEL",
  "TYPESAFE_ENDPOINT",
  "FILTER_TIMEOUT",
  "SITELOCK_ENABLED",
  "SITELOCK_PASSWORD",
  "LIVETIMER_ENABLED",
  "LIVETIMER_DURATION",
] as const;

type AppVar = (typeof APP_VARS)[number];

export interface Env extends Partial<Record<AppVar, string>> {
  VERTEXIA_PLATFORM: DurableObjectNamespace<VertexiaPlatform>;
  /** Number of container instances to spread requests over (Worker-only, not forwarded). */
  INSTANCE_COUNT?: string;
}

const CONTAINER_PORT = 8080;
const DEFAULT_INSTANCE_COUNT = 2;
/** Keep in sync with containers[0].max_instances in wrangler.jsonc. */
const MAX_INSTANCES = 4;

function buildEnvVars(env: Env): Record<string, string> {
  const vars: Record<string, string> = {};
  for (const key of APP_VARS) {
    const value = env[key];
    if (typeof value === "string") vars[key] = value;
  }
  vars.PORT = String(CONTAINER_PORT);
  vars.TRUST_PROXY = "true";
  vars.PROXY_HEADER = "X-Forwarded-For";
  return vars;
}

function unavailable(): Response {
  return new Response(JSON.stringify({ error: "unavailable" }), {
    status: 503,
    headers: {
      "Content-Type": "application/json",
      "Retry-After": "5",
      "Cache-Control": "no-store",
    },
  });
}

export class VertexiaPlatform extends Container<Env> {
  override defaultPort = CONTAINER_PORT;
  // Idle time before the instance is stopped; raise to reduce cold starts, lower to save cost.
  override sleepAfter = "15m";
  // Needs outbound access to MySQL, Redis, R2 and the gameserver agent.
  override enableInternet = true;

  constructor(ctx: DurableObject["ctx"], env: Env) {
    super(ctx, env, { envVars: buildEnvVars(env) });
  }

  override onStart(): void {
    console.log(`[vertexia-platform] container started (${this.ctx.id.toString()})`);
  }

  override onStop(params: StopParams): void {
    console.log(
      `[vertexia-platform] container stopped (${this.ctx.id.toString()}): exit=${params.exitCode} reason=${params.reason}`,
    );
  }

  override onError(error: unknown): unknown {
    console.error(`[vertexia-platform] container error (${this.ctx.id.toString()}):`, error);
    throw error;
  }

  override async fetch(request: Request): Promise<Response> {
    // Start (or confirm) the container up front so a failed start maps to a clean 503.
    const state = await this.getState();
    if (!this.ctx.container?.running || state.status !== "healthy") {
      try {
        await this.startAndWaitForPorts(CONTAINER_PORT, { abort: request.signal });
      } catch (err) {
        console.error("[vertexia-platform] container failed to start:", err);
        return unavailable();
      }
    }
    // Container.fetch proxies HTTP and WebSocket upgrades to defaultPort.
    return super.fetch(request);
  }
}

function instanceCount(env: Env): number {
  const n = Number.parseInt(env.INSTANCE_COUNT ?? "", 10);
  if (!Number.isFinite(n) || n < 1) return DEFAULT_INSTANCE_COUNT;
  return Math.min(n, MAX_INSTANCES);
}

function forwardedRequest(request: Request): Request {
  const url = new URL(request.url);
  const headers = new Headers(request.headers);

  // Never trust client-supplied forwarding headers.
  for (const name of [...headers.keys()]) {
    if (name.startsWith("x-forwarded-")) headers.delete(name);
  }
  headers.delete("forwarded");
  headers.delete("x-real-ip");

  const clientIp = request.headers.get("CF-Connecting-IP");
  if (clientIp) headers.set("X-Forwarded-For", clientIp);
  headers.set("X-Forwarded-Proto", "https");
  headers.set("X-Forwarded-Host", url.host);

  // Same method, URL and (streamed) body; only headers change.
  return new Request(request, { headers });
}

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    try {
      const container = await getRandom(env.VERTEXIA_PLATFORM, instanceCount(env));
      return await container.fetch(forwardedRequest(request));
    } catch (err) {
      console.error("[vertexia-platform] forward failed:", err);
      return unavailable();
    }
  },
} satisfies ExportedHandler<Env>;
