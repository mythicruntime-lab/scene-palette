// Owns only the server/test processes it starts. No fixed port or user browser profile.
// Run this longer check through jobgate on the shared development workstation.
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import path from "node:path";
import { spawn } from "node:child_process";
import { once } from "node:events";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
assert.equal(process.argv.length, 2, "Usage: node scripts/verify-browser.mjs");
const runID = `ci-${new Date().toISOString().replace(/[:.]/g, "-")}-${process.pid}`;
const output = path.join(root, "bin", "ci", runID);
await fs.mkdir(output, { recursive: true });
const executable = path.join(output, process.platform === "win32" ? "scene-palette.exe" : "scene-palette");
const logFile = path.join(output, "server.log");
const resultFile = path.join(output, "result.json");
const result = { startedAt: new Date().toISOString(), runID, steps: [], status: "running" };
const children = new Set();
let server;
let serverOutput = "";
let cleanupPromise;

function start(command, args, options = {}) {
  const child = spawn(command, args, { cwd: root, env: process.env, windowsHide: true, detached: process.platform !== "win32", ...options });
  children.add(child);
  child.once("close", () => children.delete(child));
  return child;
}

async function stop(child) {
  if (!child || !child.pid || child.exitCode !== null || child.signalCode !== null) return;
  const closed = once(child, "close").catch(() => {});
  if (process.platform === "win32") {
    // Only our captured child PID and its descendants; never terminate by process name.
    const killer = spawn("taskkill.exe", ["/PID", String(child.pid), "/T", "/F"], { windowsHide: true, stdio: "ignore" });
    await once(killer, "close").catch(() => child.kill());
    await closed;
    return;
  }
  const killGroup = signal => {
    try { process.kill(-child.pid, signal); }
    catch (error) { if (error.code !== "ESRCH") throw error; }
  };
  killGroup("SIGTERM");
  const timeout = setTimeout(() => killGroup("SIGKILL"), 3000);
  timeout.unref();
  try { await closed; } finally { clearTimeout(timeout); }
}

function stopAll() {
  cleanupPromise ||= Promise.all([...children].map(stop));
  return cleanupPromise;
}

for (const signal of ["SIGINT", "SIGTERM"]) {
  process.once(signal, () => {
    result.status = "interrupted";
    void stopAll().finally(() => process.exit(signal === "SIGINT" ? 130 : 143));
  });
}

async function run(label, command, args, timeoutMs) {
  console.log(`\n${label}`);
  const child = start(command, args, { stdio: "inherit" });
  let timedOut = false;
  const timer = setTimeout(() => { timedOut = true; void stop(child); }, timeoutMs);
  const began = Date.now();
  try {
    const [code, signal] = await once(child, "close");
    result.steps.push({ label, status: code === 0 && !timedOut ? "passed" : "failed", code, signal, elapsedMs: Date.now() - began });
    assert.ok(!timedOut, `${label} exceeded ${timeoutMs / 1000}s`);
    assert.equal(code, 0, `${label} failed (${signal || code})`);
  } finally {
    clearTimeout(timer);
  }
}

try {
  await run("Build the local test server", "go", ["build", "-o", executable, "."], 180000);
  server = start(executable, ["serve", "-addr", "127.0.0.1:0"], { stdio: ["ignore", "pipe", "pipe"] });
  result.serverPID = server.pid;
  let serverError;
  server.on("error", error => { serverError = error; });
  const capture = data => { serverOutput = (serverOutput + data.toString()).slice(-32768); };
  server.stdout.on("data", capture);
  server.stderr.on("data", capture);
  const deadline = Date.now() + 20000;
  let baseURL;
  while (Date.now() < deadline) {
    if (serverError) throw serverError;
    assert.equal(server.exitCode ?? server.signalCode, null, `Test server exited before readiness: ${serverOutput}`);
    const match = /Scene Palette frontend:\s+(http:\/\/127\.0\.0\.1:\d+\/)/.exec(serverOutput);
    if (match) {
      try {
        const response = await fetch(new URL("api/info", match[1]), { signal: AbortSignal.timeout(1000) });
        if (response.ok) { baseURL = match[1]; break; }
      } catch { /* Retry within the bounded readiness window. */ }
    }
    await new Promise(resolve => setTimeout(resolve, 100));
  }
  assert.ok(baseURL, `Test server was not ready after 20s: ${serverOutput}`);
  result.url = baseURL;
  await fs.writeFile(logFile, serverOutput);
  const node = process.execPath;
  await run("Browser workflows (Go and standalone)", node,
    ["scripts/browser-e2e.mjs", "--url", baseURL, "--output", `bin/e2e/${runID}`], 300000);
  await run("Readable UI corpus: exact parity, readability, and accent distinction", node,
    ["scripts/parity-corpus.mjs", "--url", baseURL, "--output", `bin/corpus/${runID}-readable-ui`,
      "--color-policy", "readable-ui", "--require-readable", "--require-distinct"], 300000);
  await run("Source hues corpus: exact parity and readability", node,
    ["scripts/parity-corpus.mjs", "--url", baseURL, "--output", `bin/corpus/${runID}-source-hues`,
      "--color-policy", "source-hues", "--require-readable"], 300000);
  result.status = "passed";
} catch (error) {
  result.status = "failed";
  result.error = error.stack || String(error);
  console.error(result.error);
  process.exitCode = 1;
} finally {
  await stopAll();
  result.finishedAt = new Date().toISOString();
  await fs.writeFile(logFile, serverOutput);
  await fs.writeFile(resultFile, JSON.stringify(result, null, 2) + "\n");
}
