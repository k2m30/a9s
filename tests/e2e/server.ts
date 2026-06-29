import * as fs from "fs";
import * as path from "path";

// RUNTIME_FILE is written by global-setup with the booted server's tokenized
// URL and pid, and read by the specs (and global-teardown).
export const RUNTIME_FILE = path.join(__dirname, ".runtime.json");

export type ServerInfo = {
  // baseURL has no token (e.g. http://127.0.0.1:7799) — for direct asset GETs.
  baseURL: string;
  // url is the full tokenized entry point (http://127.0.0.1:7799/?token=...).
  url: string;
  token: string;
  pid: number;
  // live is true when the server was booted against a real AWS profile.
  live: boolean;
  // profile is the AWS profile name used, or null in demo mode.
  profile: string | null;
};

export function readServer(): ServerInfo {
  return JSON.parse(fs.readFileSync(RUNTIME_FILE, "utf-8"));
}
