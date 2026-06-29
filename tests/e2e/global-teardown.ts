import * as fs from "fs";
import { RUNTIME_FILE, readServer } from "./server";

export default async function globalTeardown(): Promise<void> {
  try {
    const info = readServer();
    if (info.pid) {
      process.kill(info.pid);
    }
  } catch {
    // server already gone / file missing — nothing to clean up
  }
  try {
    fs.unlinkSync(RUNTIME_FILE);
  } catch {
    /* ignore */
  }
}
