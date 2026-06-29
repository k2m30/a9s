import { spawn, spawnSync, ChildProcess } from "child_process";
import * as fs from "fs";
import * as path from "path";
import { RUNTIME_FILE, ServerInfo } from "./server";

const REPO_ROOT = path.resolve(__dirname, "..", "..");
const BIN = path.join(REPO_ROOT, "a9s");
const ADDR = process.env.A9S_E2E_ADDR || "127.0.0.1:7799";

// global-setup builds the current code and boots the demo web server, so the
// browser tests always run against HEAD. The server prints
// "a9s web server: http://127.0.0.1:PORT/?token=..." on stdout; we parse that
// (the token is crypto/rand per run, so it cannot be hard-coded) and hand the
// URL to the specs via RUNTIME_FILE.
export default async function globalSetup(): Promise<void> {
  if (!process.env.A9S_E2E_SKIP_BUILD) {
    const build = spawnSync("go", ["build", "-o", "a9s", "./cmd/a9s"], {
      cwd: REPO_ROOT,
      stdio: "inherit",
    });
    if (build.status !== 0) {
      throw new Error("global-setup: `go build` failed");
    }
  }
  if (!fs.existsSync(BIN)) {
    throw new Error(`global-setup: binary not found at ${BIN}`);
  }

  const child = spawn(BIN, ["--demo", "--web-addr", ADDR], {
    cwd: REPO_ROOT,
    env: { ...process.env, A9S_MODE: "web" },
    stdio: ["ignore", "pipe", "pipe"],
  });

  let url: string;
  try {
    url = await waitForServerURL(child);
  } catch (e) {
    // Don't orphan the server if the URL never appeared (timeout / bind failure).
    try {
      child.kill();
    } catch {
      /* ignore */
    }
    throw e;
  }
  const u = new URL(url);
  const info: ServerInfo = {
    baseURL: `${u.protocol}//${u.host}`,
    url,
    token: u.searchParams.get("token") || "",
    pid: child.pid || 0,
  };
  fs.writeFileSync(RUNTIME_FILE, JSON.stringify(info, null, 2));

  // Detach so the server outlives global-setup; global-teardown kills it by pid.
  child.unref();
}

function waitForServerURL(child: ChildProcess): Promise<string> {
  return new Promise((resolve, reject) => {
    let buf = "";
    const timer = setTimeout(() => {
      reject(new Error("global-setup: timed out waiting for the web server URL"));
    }, 20_000);

    // The URL banner is printed to stderr (cmd/a9s/main.go), so watch both
    // streams with the same matcher.
    const onData = (chunk: Buffer) => {
      buf += chunk.toString();
      const m = buf.match(/a9s web server:\s*(\S+)/);
      if (m) {
        clearTimeout(timer);
        child.stdout?.off("data", onData);
        child.stderr?.off("data", onData);
        resolve(m[1]);
      }
    };
    child.stdout?.on("data", onData);
    child.stderr?.on("data", onData);
    child.on("exit", (code) => {
      clearTimeout(timer);
      reject(new Error(`global-setup: server exited early (code ${code}). Output:\n${buf}`));
    });
  });
}
