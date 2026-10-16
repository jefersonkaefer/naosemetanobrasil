import { spawn, spawnSync } from "node:child_process";
import { randomBytes } from "node:crypto";
import { mkdtemp, rm } from "node:fs/promises";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const web = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const root = resolve(web, "..");
const preview = process.argv.includes("--preview");
const raw = process.env.TEST_DATABASE_URL;
if (!raw)
  throw new Error(
    "Defina TEST_DATABASE_URL para um PostgreSQL local descartável com nome terminado em _test.",
  );
const database = new URL(raw);
if (
  !["postgres:", "postgresql:"].includes(database.protocol) ||
  !["127.0.0.1", "localhost", "[::1]"].includes(database.hostname) ||
  !database.pathname.endsWith("_test")
)
  throw new Error("E2E recusa bancos externos ou sem o sufixo _test.");

const go = process.env.E2E_GO_BIN || "go";
const temp = await mkdtemp(join(tmpdir(), "naosemeta-e2e-"));
const owned = new Set();
const server = createServer();
await new Promise((resolve, reject) =>
  server.once("error", reject).listen(0, "127.0.0.1", resolve),
);
const port = server.address().port;
await new Promise((resolve) => server.close(resolve));
const baseURL = `http://127.0.0.1:${port}`;
const env = {
  ...process.env,
  APP_ENV: "development",
  LISTEN_HOST: "127.0.0.1",
  DATABASE_URL: raw,
  PORT: String(port),
  PUBLIC_BASE_URL: baseURL,
  FRONTEND_DIR: join(web, "dist"),
  INSTAGRAM_OFFICIAL_HANDLE: preview
    ? process.env.INSTAGRAM_OFFICIAL_HANDLE || "naosemetanobrasil"
    : "naosemeta_e2e",
  INSTAGRAM_VERIFICATION_MODE: preview
    ? process.env.INSTAGRAM_VERIFICATION_MODE || "manual"
    : "webhook",
  ...(!preview
    ? {
        META_APP_SECRET: randomBytes(32).toString("hex"),
        META_WEBHOOK_VERIFY_TOKEN: randomBytes(32).toString("hex"),
        INSTAGRAM_ACCESS_TOKEN: "synthetic-e2e-token-never-use-in-production",
        INSTAGRAM_ACCOUNT_ID: "123456",
        META_GRAPH_API_VERSION: "v25.0",
      }
    : {}),
  VERIFICATION_CODE_PEPPER: randomBytes(32).toString("hex"),
  TOTP_ENCRYPTION_KEY: randomBytes(32).toString("base64"),
  E2E_BASE_URL: baseURL,
  E2E_GO_BIN: go,
  E2E_ADMIN_USERNAME: `admin_e2e_${randomBytes(5).toString("hex")}`,
  E2E_ADMIN_PASSWORD: `E2E-only-${randomBytes(16).toString("hex")}`,
  E2E_TOTP_SECRET: "JBSWY3DPEHPK3PXP",
};
function command(executable, args, cwd = root) {
  const result = spawnSync(executable, args, { cwd, env, stdio: "inherit" });
  if (result.status !== 0) throw new Error(`Falhou: ${executable} ${args[0]}`);
}
function own(child) {
  owned.add(child);
  child.once("exit", () => owned.delete(child));
  return child;
}
for (const signal of ["SIGINT", "SIGTERM"])
  process.on(signal, () => {
    for (const child of owned) child.kill("SIGTERM");
  });

let api;
try {
  if (process.env.npm_execpath)
    command(process.execPath, [process.env.npm_execpath, "run", "build"], web);
  else command("npm", ["run", "build"], web);
  command(go, [
    "build",
    ...(!preview ? ["-tags", "e2e"] : []),
    "-o",
    join(temp, "api"),
    "./cmd/api",
  ]);
  command(go, ["run", "./cmd/migrate"]);
  command(go, ["run", "-tags", "e2e", "./web/e2e/seed.go"]);
  api = own(
    spawn(join(temp, "api"), [], {
      cwd: root,
      env,
      stdio: ["ignore", "pipe", "pipe"],
    }),
  );
  let output = "";
  api.stdout.on("data", (data) => {
    output = (output + data).slice(-8000);
  });
  api.stderr.on("data", (data) => {
    output = (output + data).slice(-8000);
  });
  let ready = false;
  for (let attempt = 0; attempt < 100; attempt++) {
    if (api.exitCode !== null)
      throw new Error(`A API encerrou antes do teste. ${output}`);
    try {
      if (
        (
          await fetch(`${baseURL}/health/ready`, {
            signal: AbortSignal.timeout(1000),
          })
        ).ok
      ) {
        ready = true;
        break;
      }
    } catch {}
    await new Promise((resolve) => setTimeout(resolve, 200));
  }
  if (!ready) throw new Error(`A API não ficou pronta. ${output}`);
  if (preview) {
    console.log(`Prévia local com dados descartáveis: ${baseURL}`);
    await new Promise((resolve) => api.once("exit", resolve));
  } else {
    const browserTests = own(
      spawn(
        process.execPath,
        [
          join(web, "node_modules/@playwright/test/cli.js"),
          "test",
          ...process.argv.slice(2),
        ],
        { cwd: web, env, stdio: "inherit" },
      ),
    );
    const code = await new Promise((resolve, reject) => {
      browserTests.once("error", reject);
      browserTests.once("exit", (code) => resolve(code ?? 1));
    });
    process.exitCode = code;
  }
} finally {
  for (const child of owned) child.kill("SIGTERM");
  if (api && api.exitCode === null)
    await new Promise((resolve) => {
      const timer = setTimeout(() => {
        if (api.exitCode === null) api.kill("SIGKILL");
        resolve();
      }, 10_000);
      api.once("exit", () => {
        clearTimeout(timer);
        resolve();
      });
    });
  await rm(temp, { recursive: true, force: true });
}
