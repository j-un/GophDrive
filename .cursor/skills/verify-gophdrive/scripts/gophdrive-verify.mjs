#!/usr/bin/env node
// Drive the local GophDrive web app for verification.
//
//   mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs <command>
//
// Commands:
//   doctor
//   launch
//   cleanup
//   browser start
//   browser goto <url>
//   browser click --role <role> --name <accessible-name>
//   browser fill --placeholder <placeholder> --value <text>
//   browser press --key <Enter|Escape|Meta+n|Meta+k|Ctrl+n|Ctrl+k>
//   browser insert --text <text>          # into the CodeMirror editor
//   browser wait-text --text <text> [--scope list] [--timeout <ms>]
//   browser wait-name --role <role> --name <accessible-name> [--timeout <ms>]
//   browser wait-absent --text <text> [--scope list] [--timeout <ms>]
//   browser wait-url --includes <substring> [--timeout <ms>]
//   browser screenshot --path <file>
//   browser snapshot --path <file>
//   browser downloads [--copy-to <dir>]
//   browser cookies                        # names and hosts only, never token values
//   browser text
//
// Names match the accessible name (aria-label, otherwise visible text) as a
// case-sensitive substring. Hidden elements are skipped. Paths are relative
// to the repo root.

import { spawn, execFileSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(scriptDir, "../../../..");
const skillDir = path.resolve(scriptDir, "..");
const runDir = path.join(skillDir, "run");
const evidenceDir = path.join(skillDir, "evidence");
const stackPath = path.join(runDir, "stack.json");
const browserPath = path.join(runDir, "browser.json");
const downloadDir = path.join(runDir, "downloads");
const overmindLog = path.join(runDir, "overmind.log");

const FRONTEND = "http://localhost:5173";
const BACKEND_USER = "http://localhost:8080/auth/user";
const DYNAMO = "http://127.0.0.1:8000/";
const WASM = "http://localhost:5173/core.wasm";

function die(message) {
  const error = new Error(message);
  error.fatal = true;
  throw error;
}

function ensureRun() {
  fs.mkdirSync(runDir, { recursive: true });
}

function readJson(file) {
  try {
    return JSON.parse(fs.readFileSync(file, "utf8"));
  } catch {
    return null;
  }
}

function writeJson(file, value) {
  ensureRun();
  fs.writeFileSync(file, JSON.stringify(value, null, 2) + "\n");
}

function which(bin) {
  const extra = [
    "/opt/homebrew/bin",
    "/usr/local/bin",
    path.join(process.env.HOME || "", ".local/bin"),
  ];
  const dirs = [...extra, ...(process.env.PATH || "").split(":")];
  for (const dir of dirs) {
    if (!dir) continue;
    const candidate = path.join(dir, bin);
    if (fs.existsSync(candidate)) return candidate;
  }
  return null;
}

function repoPath(p) {
  return path.isAbsolute(p) ? p : path.join(repoRoot, p);
}

function opts(args) {
  const o = {};
  const pos = [];
  for (let i = 0; i < args.length; i++) {
    const arg = args[i];
    if (arg.startsWith("--")) {
      const key = arg.slice(2);
      const next = args[i + 1];
      if (next === undefined || next.startsWith("--")) {
        o[key] = true;
      } else {
        o[key] = next;
        i++;
      }
    } else {
      pos.push(arg);
    }
  }
  return { o, pos };
}

async function probe(url, accept, readBody) {
  try {
    const res = await fetch(url, {
      redirect: "manual",
      signal: AbortSignal.timeout(4000),
    });
    let text = "";
    if (readBody) text = await res.text();
    else await res.body?.cancel();
    const ok = accept(res, text);
    return { state: ok ? "up" : "foreign", status: res.status, text };
  } catch (error) {
    const refused =
      error?.cause?.code === "ECONNREFUSED" ||
      error?.code === "ECONNREFUSED" ||
      String(error).includes("ECONNREFUSED") ||
      error?.name === "TimeoutError" ||
      error?.name === "AbortError";
    return {
      state: refused ? "down" : "foreign",
      status: 0,
      text: String(error?.cause?.code || error?.name || error),
    };
  }
}

function probeFrontend() {
  return probe(
    FRONTEND + "/",
    (res, text) => res.status === 200 && text.includes("<title>GophDrive</title>"),
    true,
  );
}

function probeBackend() {
  return probe(
    BACKEND_USER,
    (res, text) => res.status === 401 && text.includes("Unauthorized"),
    true,
  );
}

function probeDynamo() {
  return probe(DYNAMO, (res) => res.status > 0, false);
}

function probeWasm() {
  return probe(WASM, (res) => res.status === 200, false);
}

async function stackHealth() {
  const [frontend, backend, dynamo, wasm] = await Promise.all([
    probeFrontend(),
    probeBackend(),
    probeDynamo(),
    probeWasm(),
  ]);
  return { frontend, backend, dynamo, wasm };
}

function listener(port) {
  try {
    const out = execFileSync(
      "lsof",
      ["-nP", `-iTCP:${port}`, "-sTCP:LISTEN", "-Fp"],
      { encoding: "utf8" },
    );
    const pid = Number(out.match(/^p(\d+)/m)?.[1] || 0);
    if (!pid) return null;
    const command = execFileSync("ps", ["-p", String(pid), "-o", "command="], {
      encoding: "utf8",
    }).trim();
    return { pid, command };
  } catch {
    return null;
  }
}

function printProbe(label, probeResult, port) {
  const who = listener(port);
  const owner = who ? ` pid=${who.pid} cmd=${who.command}` : "";
  console.log(
    `${label}: ${probeResult.state} status=${probeResult.status}${owner}`,
  );
}

async function doctor() {
  const health = await stackHealth();
  const stack = readJson(stackPath);
  console.log("GophDrive doctor");
  console.log(`repo: ${repoRoot}`);
  console.log(`frontend url: ${FRONTEND}`);
  printProbe("frontend", health.frontend, 5173);
  printProbe("backend", health.backend, 8080);
  printProbe("dynamodb", health.dynamo, 8000);
  printProbe("wasm", health.wasm, 5173);
  if (stack?.adopted) {
    console.log("ownership: adopted existing stack; cleanup will not stop it");
  } else if (stack?.startedOvermind) {
    console.log(
      `ownership: started by this skill (overmind pid ${stack.overmindPid})`,
    );
  } else {
    console.log("ownership: no stack.json; cleanup will not stop a stack");
  }
  const worth =
    health.frontend.state === "up" &&
    health.backend.state === "up" &&
    health.dynamo.state === "up" &&
    health.wasm.state === "up";
  if (!worth) {
    console.log("worth driving: no");
    if (health.frontend.state === "foreign" || health.backend.state === "foreign") {
      console.log(
        "a port is taken by something that is not this GophDrive dev stack; do not kill it from this skill",
      );
    }
    process.exit(1);
  }
  console.log("worth driving: yes");
}

function overmindAlive() {
  const bin = which("overmind");
  if (!bin) return false;
  try {
    execFileSync(bin, ["status"], {
      cwd: repoRoot,
      stdio: "ignore",
      timeout: 5000,
    });
    return true;
  } catch {
    return false;
  }
}

async function waitHealthy(timeoutMs) {
  const started = Date.now();
  let last = "";
  while (Date.now() - started < timeoutMs) {
    const health = await stackHealth();
    const summary = `frontend=${health.frontend.state} backend=${health.backend.state} dynamodb=${health.dynamo.state} wasm=${health.wasm.state}`;
    if (summary !== last) {
      fs.writeSync(1, `waiting: ${summary}\n`);
      last = summary;
    }
    const stack = readJson(stackPath);
    if (stack?.startedOvermind && stack.overmindPid) {
      try {
        process.kill(stack.overmindPid, 0);
      } catch {
        if (fs.existsSync(overmindLog)) {
          const log = fs.readFileSync(overmindLog, "utf8");
          fs.writeSync(2, log.slice(-4000) + "\n");
        }
        throw new Error(
          "overmind exited before the stack was usable; see run/overmind.log",
        );
      }
    }
    if (
      health.frontend.state === "up" &&
      health.backend.state === "up" &&
      health.dynamo.state === "up" &&
      health.wasm.state === "up"
    ) {
      return;
    }
    if (health.frontend.state === "foreign" || health.backend.state === "foreign") {
      throw new Error(
        "a port started answering as something other than GophDrive; refusing to continue",
      );
    }
    await new Promise((r) => setTimeout(r, 1000));
  }
  if (fs.existsSync(overmindLog)) {
    const log = fs.readFileSync(overmindLog, "utf8");
    console.error(log.slice(-4000));
  }
  throw new Error(`stack was not usable within ${timeoutMs}ms`);
}

async function launch() {
  if (!fs.existsSync(path.join(repoRoot, ".env"))) {
    die("missing .env — run ./scripts/setup.sh before launching");
  }
  const health = await stackHealth();
  if (health.frontend.state === "foreign" || health.backend.state === "foreign") {
    die(
      "port 5173 or 8080 is occupied by a process that is not GophDrive; refusing to start a second stack",
    );
  }
  const partial =
    (health.frontend.state === "up") !== (health.backend.state === "up");
  if (partial) {
    die(
      "only one of frontend (:5173) and backend (:8080) is up; refusing to start overmind on top of a partial stack",
    );
  }
  if (health.frontend.state === "up" && health.backend.state === "up") {
    if (health.dynamo.state !== "up") {
      die("frontend and backend are up but DynamoDB Local on :8000 is not");
    }
    const prev = readJson(stackPath);
    if (!prev?.startedOvermind) {
      writeJson(stackPath, {
        adopted: true,
        startedOvermind: false,
        startedDynamo: false,
      });
      console.log("adopted existing stack; cleanup will not stop it");
    } else {
      console.log("stack already started by this skill");
    }
    if (health.wasm.state !== "up") {
      console.log("waiting for core.wasm");
      await waitHealthy(180000);
    }
    console.log(`ready ${FRONTEND}`);
    return;
  }

  if (overmindAlive()) {
    console.log("overmind is already running; waiting for it to become usable");
    writeJson(stackPath, {
      adopted: true,
      startedOvermind: false,
      startedDynamo: false,
    });
    await waitHealthy(180000);
    console.log(`ready ${FRONTEND}`);
    return;
  }

  const docker = which("docker");
  const overmind = which("overmind");
  const mise = which("mise");
  if (!docker) die("docker is not on PATH");
  if (!overmind) die("overmind is not on PATH");
  if (!mise) die("mise is not on PATH");
  try {
    execFileSync(docker, ["info"], { stdio: "ignore" });
  } catch {
    die(
      "Docker daemon is not running. Start it (Rancher Desktop: rdctl start) and run launch again. This skill does not start the daemon.",
    );
  }

  let startedDynamo = false;
  if (health.dynamo.state !== "up") {
    console.log("starting DynamoDB Local");
    execFileSync(docker, ["compose", "up", "-d", "dynamodb-local"], {
      cwd: repoRoot,
      stdio: "inherit",
    });
    startedDynamo = true;
  }

  const sock = path.join(repoRoot, ".overmind.sock");
  if (fs.existsSync(sock)) {
    console.log("removing stale .overmind.sock");
    fs.rmSync(sock, { force: true });
  }

  ensureRun();
  const logFd = fs.openSync(overmindLog, "a");
  const env = {
    ...process.env,
    PATH: [
      path.dirname(mise),
      path.dirname(overmind),
      path.dirname(docker),
      process.env.PATH || "",
    ].join(":"),
  };
  const child = spawn(overmind, ["start"], {
    cwd: repoRoot,
    detached: true,
    env,
    stdio: ["ignore", logFd, logFd],
  });
  child.unref();
  fs.closeSync(logFd);
  writeJson(stackPath, {
    adopted: false,
    startedOvermind: true,
    overmindPid: child.pid,
    startedDynamo,
  });
  console.log(`started overmind pid=${child.pid}`);
  try {
    await waitHealthy(180000);
  } catch (error) {
    console.error(error.message || error);
    stopStack();
    throw error;
  }
  console.log(`ready ${FRONTEND}`);
}

function killPid(pid) {
  if (!pid) return;
  try {
    process.kill(-pid, "SIGTERM");
  } catch {
    try {
      process.kill(pid, "SIGTERM");
    } catch {
      // already gone
    }
  }
}

function killByCommand(token) {
  let out = "";
  try {
    out = execFileSync("ps", ["-ax", "-o", "pid=", "-o", "command="], {
      encoding: "utf8",
    });
  } catch {
    return;
  }
  for (const line of out.split("\n")) {
    if (!line.includes(token)) continue;
    const pid = Number(line.trim().split(/\s+/)[0]);
    if (!pid || pid === process.pid) continue;
    try {
      process.kill(pid, "SIGTERM");
    } catch {
      // already gone
    }
  }
}

function stopStack() {
  const stack = readJson(stackPath);
  if (!stack?.startedOvermind) {
    console.log("cleanup: leaving the existing stack running");
  } else {
    const overmind = which("overmind");
    if (overmind) {
      try {
        execFileSync(overmind, ["quit"], {
          cwd: repoRoot,
          stdio: "inherit",
          timeout: 20000,
        });
      } catch (error) {
        console.error(`overmind quit: ${error.message}`);
        killPid(stack.overmindPid);
      }
    } else {
      killPid(stack.overmindPid);
    }
    console.log("cleanup: stopped overmind started by this skill");
  }
  if (stack?.startedDynamo) {
    const docker = which("docker");
    if (docker) {
      try {
        execFileSync(docker, ["compose", "stop", "dynamodb-local"], {
          cwd: repoRoot,
          stdio: "inherit",
        });
        console.log("cleanup: stopped DynamoDB Local started by this skill");
      } catch (error) {
        console.error(`docker compose stop: ${error.message}`);
      }
    }
  }
}

function stopBrowser() {
  const browser = readJson(browserPath);
  if (!browser) return;
  killPid(browser.pid);
  if (browser.profile) killByCommand(browser.profile);
  console.log("cleanup: stopped the verification Chrome");
}

function cleanup() {
  stopBrowser();
  stopStack();
  fs.rmSync(runDir, { recursive: true, force: true });
  console.log(`cleanup: removed ${runDir}`);
  console.log(`evidence kept at ${evidenceDir}`);
}

function chromeBinary() {
  const fromEnv = process.env.GOPHDRIVE_CHROME;
  const candidates = [
    fromEnv,
    "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
    "/Applications/Chromium.app/Contents/MacOS/Chromium",
    "/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
  ].filter(Boolean);
  for (const candidate of candidates) {
    if (fs.existsSync(candidate)) return candidate;
  }
  die(
    "Google Chrome was not found. Set GOPHDRIVE_CHROME to the browser binary.",
  );
}

async function browserStart() {
  const existing = readJson(browserPath);
  if (existing?.pid) {
    try {
      process.kill(existing.pid, 0);
      die(
        "a verification Chrome is already running; run cleanup before browser start",
      );
    } catch {
      // stale pid
    }
  }
  const health = await stackHealth();
  if (
    health.frontend.state !== "up" ||
    health.backend.state !== "up" ||
    health.wasm.state !== "up"
  ) {
    die("stack is not worth driving; run doctor");
  }
  ensureRun();
  const profile = path.join(runDir, "chrome-profile");
  fs.rmSync(profile, { recursive: true, force: true });
  fs.mkdirSync(profile, { recursive: true });
  fs.mkdirSync(downloadDir, { recursive: true });
  // Browser.setDownloadBehavior lasts only while that DevTools socket is open.
  // This script starts a new process per command, so Chrome reads the directory from the profile.
  const prefsDir = path.join(profile, "Default");
  fs.mkdirSync(prefsDir, { recursive: true });
  fs.writeFileSync(
    path.join(prefsDir, "Preferences"),
    JSON.stringify({
      download: {
        default_directory: downloadDir,
        directory_upgrade: true,
        prompt_for_download: false,
      },
      savefile: { default_directory: downloadDir },
    }),
  );
  const chrome = chromeBinary();
  const child = spawn(
    chrome,
    [
      "--headless=new",
      "--disable-gpu",
      "--no-first-run",
      "--no-default-browser-check",
      "--disable-background-networking",
      "--disable-sync",
      "--disable-extensions",
      "--allow-insecure-localhost",
      "--remote-allow-origins=*",
      `--user-data-dir=${profile}`,
      "--remote-debugging-port=0",
      "--window-size=1280,900",
    ],
    { detached: true, stdio: "ignore" },
  );
  child.unref();
  const port = await readDevtoolsPort(profile);
  writeJson(browserPath, { pid: child.pid, port, profile });
  await openPage(port, "about:blank");
  console.log(`browser ready pid=${child.pid} port=${port}`);
  console.log(`profile ${profile}`);
  console.log("drive http://localhost:5173 — not 127.0.0.1 (session cookie is host-only)");
}

function readDevtoolsPort(profile) {
  const file = path.join(profile, "DevToolsActivePort");
  const started = Date.now();
  return new Promise((resolve, reject) => {
    const timer = setInterval(() => {
      if (fs.existsSync(file)) {
        const port = Number(fs.readFileSync(file, "utf8").split("\n")[0]);
        if (port) {
          clearInterval(timer);
          resolve(port);
          return;
        }
      }
      if (Date.now() - started > 20000) {
        clearInterval(timer);
        reject(new Error("Chrome did not open a DevTools port"));
      }
    }, 200);
  });
}

async function openPage(port, url) {
  const endpoint = `http://127.0.0.1:${port}/json/new?${encodeURIComponent(url)}`;
  let res;
  for (let attempt = 0; attempt < 20; attempt++) {
    try {
      res = await fetch(endpoint, { method: "PUT" });
      if (!res.ok) res = await fetch(endpoint, { method: "GET" });
      if (res.ok) break;
    } catch {
      // DevTools port is up a moment before it accepts HTTP.
    }
    await new Promise((r) => setTimeout(r, 200));
  }
  if (!res?.ok) die(`could not open a Chrome tab (${res?.status || "no response"})`);
  const body = await res.json();
  const session = readJson(browserPath) || {};
  writeJson(browserPath, { ...session, targetId: body.id });
  return body;
}

function browserSession() {
  const session = readJson(browserPath);
  if (!session?.port) die("no verification browser; run browser start");
  return session;
}

async function pageTarget() {
  const session = browserSession();
  const list = await fetch(`http://127.0.0.1:${session.port}/json/list`).then(
    (r) => r.json(),
  );
  const pages = list.filter((target) => target.type === "page");
  const page =
    pages.find((target) => target.id === session.targetId) || pages.at(-1);
  if (!page?.webSocketDebuggerUrl) die("Chrome has no page target");
  return page;
}

class Cdp {
  constructor(ws) {
    this.ws = ws;
    this.next = 1;
    this.pending = new Map();
    ws.addEventListener("message", (event) => {
      const message = JSON.parse(event.data);
      if (!message.id || !this.pending.has(message.id)) return;
      const { resolve, reject } = this.pending.get(message.id);
      this.pending.delete(message.id);
      if (message.error) reject(new Error(JSON.stringify(message.error)));
      else resolve(message.result || {});
    });
  }
  send(method, params = {}) {
    const id = this.next++;
    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
      this.ws.send(JSON.stringify({ id, method, params }));
    });
  }
}

function connect(wsUrl) {
  const ws = new WebSocket(wsUrl);
  return new Promise((resolve, reject) => {
    ws.addEventListener("open", () => resolve(new Cdp(ws)));
    ws.addEventListener("error", () => reject(new Error(`websocket failed: ${wsUrl}`)));
  });
}

async function withPage(fn) {
  const page = await pageTarget();
  const cdp = await connect(page.webSocketDebuggerUrl);
  try {
    await cdp.send("Page.enable");
    await cdp.send("Runtime.enable");
    await cdp.send("Emulation.setDeviceMetricsOverride", {
      width: 1280,
      height: 900,
      deviceScaleFactor: 1,
      mobile: false,
    });
    return await fn(cdp);
  } finally {
    cdp.ws.close();
  }
}

async function evaluate(cdp, expression) {
  const result = await cdp.send("Runtime.evaluate", {
    expression,
    returnByValue: true,
    awaitPromise: true,
  });
  if (result.exceptionDetails) {
    die(
      `page error: ${result.exceptionDetails.text} ${result.exceptionDetails.exception?.description || ""}`,
    );
  }
  return result.result?.value;
}

function matchVisibleControl(role, name, { click = false } = {}) {
  const activate = click
    ? `hits[0].el.scrollIntoView({ block: "center", inline: "center" });
        hits[0].el.click();`
    : "";
  return `(() => {
        const norm = (s) => (s || "").replace(/\\s+/g, " ").trim();
        const accessibleName = (el) => norm(el.getAttribute("aria-label") || el.innerText || "");
        const want = ${JSON.stringify(name)};
        const wantRole = ${JSON.stringify(role)}.toLowerCase();
        const nodes = [...document.querySelectorAll("a,button,input,textarea,[role]")];
        const hits = [];
        for (const el of nodes) {
          if (el.closest("[aria-hidden='true']")) continue;
          const style = getComputedStyle(el);
          if (style.display === "none" || style.visibility === "hidden") continue;
          const rect = el.getBoundingClientRect();
          if (rect.width < 1 || rect.height < 1) continue;
          const tag = el.tagName.toLowerCase();
          let implicit = el.getAttribute("role");
          if (!implicit) {
            if (tag === "a") implicit = "link";
            else if (tag === "button") implicit = "button";
            else if (tag === "textarea") implicit = "textbox";
            else if (tag === "input") {
              const type = (el.getAttribute("type") || "text").toLowerCase();
              implicit = type === "text" || type === "search" || type === "" ? "textbox" : type;
            }
          }
          if (implicit !== wantRole) continue;
          const accessible = accessibleName(el);
          if (want && !accessible.includes(want)) continue;
          hits.push({ el, accessible, exact: accessible === want });
        }
        hits.sort((a, b) => (b.exact - a.exact) || (a.accessible.length - b.accessible.length));
        if (!hits.length) {
          const visible = [];
          for (const el of nodes) {
            const style = getComputedStyle(el);
            if (style.display === "none" || style.visibility === "hidden") continue;
            const rect = el.getBoundingClientRect();
            if (rect.width < 1 || rect.height < 1) continue;
            const accessible = accessibleName(el);
            if (accessible) visible.push(accessible);
          }
          return { ok: false, visible: visible.slice(0, 40), url: location.href };
        }
        ${activate}
        return { ok: true, name: hits[0].accessible, url: location.href };
      })()`;
}

async function click(role, name) {
  if (!role || !name) die("click requires --role and --name");
  const found = await withPage(async (cdp) =>
    evaluate(cdp, matchVisibleControl(role, name, { click: true })),
  );
  if (!found?.ok) {
    die(
      `no visible ${role} matching ${JSON.stringify(name)}\nvisible:\n${(found?.visible || []).join("\n")}`,
    );
  }
  console.log(`clicked ${role} ${JSON.stringify(found.name)}`);
}

async function fill(placeholder, value) {
  if (!placeholder || value === undefined) {
    die("fill requires --placeholder and --value");
  }
  const found = await withPage(async (cdp) =>
    evaluate(
      cdp,
      `(() => {
        const inputs = [...document.querySelectorAll("input,textarea")];
        const el = inputs.find((node) => node.getAttribute("placeholder") === ${JSON.stringify(placeholder)});
        if (!el) return { ok: false };
        const proto = el instanceof HTMLTextAreaElement ? HTMLTextAreaElement : HTMLInputElement;
        Object.getOwnPropertyDescriptor(proto.prototype, "value").set.call(el, ${JSON.stringify(value)});
        el.dispatchEvent(new Event("input", { bubbles: true }));
        el.focus();
        return { ok: true, value: el.value };
      })()`,
    ),
  );
  if (!found?.ok) die(`no field with placeholder ${JSON.stringify(placeholder)}`);
  console.log(`filled ${JSON.stringify(placeholder)}`);
}

const KEYS = {
  Enter: { key: "Enter", code: "Enter", windowsVirtualKeyCode: 13, modifiers: 0 },
  Escape: { key: "Escape", code: "Escape", windowsVirtualKeyCode: 27, modifiers: 0 },
  "Meta+n": { key: "n", code: "KeyN", windowsVirtualKeyCode: 78, modifiers: 4 },
  "Meta+k": { key: "k", code: "KeyK", windowsVirtualKeyCode: 75, modifiers: 4 },
  "Ctrl+n": { key: "n", code: "KeyN", windowsVirtualKeyCode: 78, modifiers: 2 },
  "Ctrl+k": { key: "k", code: "KeyK", windowsVirtualKeyCode: 75, modifiers: 2 },
};

async function press(key) {
  const spec = KEYS[key];
  if (!spec) die(`unsupported key ${JSON.stringify(key)}; use ${Object.keys(KEYS).join(", ")}`);
  await withPage(async (cdp) => {
    await cdp.send("Input.dispatchKeyEvent", { type: "keyDown", ...spec });
    await cdp.send("Input.dispatchKeyEvent", {
      type: "keyUp",
      key: spec.key,
      code: spec.code,
      windowsVirtualKeyCode: spec.windowsVirtualKeyCode,
      modifiers: spec.modifiers,
    });
  });
  console.log(`pressed ${key}`);
}

async function insertText(text) {
  if (!text) die("insert requires --text");
  const found = await withPage(async (cdp) => {
    const ok = await evaluate(
      cdp,
      `(() => {
        const el = document.querySelector(".cm-content");
        if (!el) return false;
        el.focus();
        return true;
      })()`,
    );
    if (!ok) return false;
    await cdp.send("Input.insertText", { text });
    return true;
  });
  if (!found) die("CodeMirror editor (.cm-content) is not on the page");
  console.log("inserted text into the editor");
}

async function pageText(cdp) {
  return evaluate(
    cdp,
    `(() => ({
      url: location.href,
      title: document.title,
      text: document.body ? document.body.innerText : ""
    }))()`,
  );
}

async function headingSiblingText(cdp) {
  return evaluate(
    cdp,
    `(() => {
      const h1 = document.querySelector("h1");
      const sibling = h1 && h1.nextElementSibling;
      const page = document.body ? document.body.innerText : "";
      if (!sibling) return { url: location.href, title: document.title, text: page, ready: false };
      return { url: location.href, title: document.title, text: sibling.innerText || "", ready: true };
    })()`,
  );
}

async function waitFor(kind, expected, timeoutMs, scope) {
  if (scope !== undefined && scope !== "list") {
    die(`unknown scope ${JSON.stringify(scope)}; the only scope is list`);
  }
  const timeout = Number(timeoutMs || 20000);
  const started = Date.now();
  let last = null;
  while (Date.now() - started < timeout) {
    last = await withPage((cdp) =>
      scope === "list" ? headingSiblingText(cdp) : pageText(cdp),
    );
    const pending = scope === "list" && last?.ready !== true;
    const haystack = kind === "url" ? last?.url || "" : last?.text || "";
    const ok =
      !pending &&
      (kind === "absent" ? !haystack.includes(expected) : haystack.includes(expected));
    if (ok) {
      console.log(`${kind} ok ${JSON.stringify(expected)}`);
      return;
    }
    await new Promise((r) => setTimeout(r, 250));
  }
  console.error(`url: ${last?.url}`);
  console.error((last?.text || "").slice(0, 1500));
  die(`timed out after ${timeout}ms waiting for ${kind} ${JSON.stringify(expected)}`);
}

async function waitName(role, name, timeoutMs) {
  if (!role || !name) die("wait-name requires --role and --name");
  const timeout = Number(timeoutMs || 20000);
  const started = Date.now();
  let last = null;
  while (Date.now() - started < timeout) {
    last = await withPage((cdp) => evaluate(cdp, matchVisibleControl(role, name)));
    if (last?.ok) {
      console.log(`name ok ${JSON.stringify(role)} ${JSON.stringify(last.name)}`);
      return;
    }
    await new Promise((r) => setTimeout(r, 250));
  }
  console.error(`url: ${last?.url}`);
  console.error((last?.visible || []).join("\n"));
  die(
    `timed out after ${timeout}ms waiting for name ${JSON.stringify(role)} ${JSON.stringify(name)}`,
  );
}

async function snapshot(file) {
  const body = await withPage(async (cdp) =>
    evaluate(
      cdp,
      `(() => {
        const norm = (s) => (s || "").replace(/\\s+/g, " ").trim();
        const lines = ["url: " + location.href, "title: " + document.title];
        const h1 = document.querySelector("h1");
        if (h1) lines.push("h1: " + norm(h1.innerText));
        for (const el of document.querySelectorAll("a,button,input,textarea,[role='button']")) {
          const style = getComputedStyle(el);
          if (style.display === "none" || style.visibility === "hidden") continue;
          const rect = el.getBoundingClientRect();
          if (rect.width < 1 || rect.height < 1) continue;
          const tag = el.tagName.toLowerCase();
          let role = el.getAttribute("role");
          if (!role) {
            if (tag === "a") role = "link";
            else if (tag === "button") role = "button";
            else if (tag === "input" || tag === "textarea") role = "textbox";
          }
          const name = norm(el.getAttribute("aria-label") || el.innerText || "");
          const placeholder = el.getAttribute("placeholder");
          if (placeholder) lines.push(role + " placeholder " + JSON.stringify(placeholder));
          else if (name) lines.push(role + " " + JSON.stringify(name));
        }
        return lines.join("\\n") + "\\n";
      })()`,
    ),
  );
  const out = repoPath(file);
  fs.mkdirSync(path.dirname(out), { recursive: true });
  fs.writeFileSync(out, body || "");
  console.log(`snapshot ${out}`);
}

async function screenshot(file) {
  const image = await withPage(async (cdp) => {
    const shot = await cdp.send("Page.captureScreenshot", { format: "png" });
    return shot.data;
  });
  const out = repoPath(file);
  fs.mkdirSync(path.dirname(out), { recursive: true });
  fs.writeFileSync(out, Buffer.from(image, "base64"));
  console.log(`screenshot ${out}`);
}

function listDownloads() {
  if (!fs.existsSync(downloadDir)) return [];
  return fs
    .readdirSync(downloadDir)
    .filter((name) => !name.endsWith(".crdownload"))
    .map((name) => path.join(downloadDir, name));
}

async function downloads(copyTo) {
  const started = Date.now();
  let files = listDownloads();
  while (!files.length && Date.now() - started < 15000) {
    await new Promise((r) => setTimeout(r, 250));
    files = listDownloads();
  }
  if (!files.length) die(`no completed download in ${downloadDir}`);
  for (const file of files) console.log(file);
  if (copyTo) {
    const destDir = repoPath(copyTo);
    fs.mkdirSync(destDir, { recursive: true });
    for (const file of files) {
      const dest = path.join(destDir, path.basename(file));
      fs.copyFileSync(file, dest);
      console.log(`copied ${dest}`);
    }
  }
}

async function cookies() {
  const names = await withPage(async (cdp) => {
    await cdp.send("Network.enable");
    const result = await cdp.send("Network.getCookies", {});
    return (result.cookies || []).map((cookie) => ({
      name: cookie.name,
      domain: cookie.domain,
      secure: cookie.secure,
      httpOnly: cookie.httpOnly,
    }));
  });
  if (!names.length) console.log("no cookies");
  for (const cookie of names) {
    console.log(
      `${cookie.name} domain=${cookie.domain} secure=${cookie.secure} httpOnly=${cookie.httpOnly}`,
    );
  }
}

async function showText() {
  const body = await withPage((cdp) => pageText(cdp));
  console.log(body?.url || "");
  console.log(body?.text || "");
}

async function goto(url) {
  if (!url) die("goto requires a url");
  if (url.includes("127.0.0.1:5173")) {
    die("use http://localhost:5173 — the session cookie is host-only and FRONTEND_URL is localhost");
  }
  await withPage(async (cdp) => {
    await cdp.send("Page.navigate", { url });
  });
  const started = Date.now();
  while (Date.now() - started < 20000) {
    const ready = await withPage((cdp) =>
      evaluate(cdp, "document.readyState"),
    );
    const current = await withPage((cdp) => evaluate(cdp, "location.href"));
    if (ready === "complete" && current && current !== "about:blank") {
      console.log(`goto ${current}`);
      return;
    }
    await new Promise((r) => setTimeout(r, 200));
  }
  die(`navigation to ${url} did not finish`);
}

function help() {
  console.log(`mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs <command>

  doctor
  launch
  cleanup
  browser start
  browser goto <url>
  browser click --role <role> --name <name>
  browser fill --placeholder <placeholder> --value <text>
  browser press --key <Enter|Escape|Meta+n|Meta+k|Ctrl+n|Ctrl+k>
  browser insert --text <text>
  browser wait-text --text <text> [--scope list] [--timeout <ms>]
  browser wait-name --role <role> --name <name> [--timeout <ms>]
  browser wait-absent --text <text> [--scope list] [--timeout <ms>]
  browser wait-url --includes <substring> [--timeout <ms>]
  browser screenshot --path <file>
  browser snapshot --path <file>
  browser downloads [--copy-to <dir>]
  browser cookies
  browser text`);
}

const [command, sub, ...rest] = process.argv.slice(2);

try {
  if (!command || command === "--help" || command === "help") {
    help();
  } else if (command === "doctor") {
    await doctor();
  } else if (command === "launch") {
    await launch();
  } else if (command === "cleanup") {
    cleanup();
  } else if (command === "browser") {
    const { o, pos } = opts(rest);
    if (sub === "start") await browserStart();
    else if (sub === "goto") await goto(pos[0]);
    else if (sub === "click") await click(o.role, o.name);
    else if (sub === "fill") await fill(o.placeholder, o.value);
    else if (sub === "press") await press(o.key);
    else if (sub === "insert") await insertText(o.text);
    else if (sub === "wait-text") await waitFor("text", o.text, o.timeout, o.scope);
    else if (sub === "wait-name") await waitName(o.role, o.name, o.timeout);
    else if (sub === "wait-absent") await waitFor("absent", o.text, o.timeout, o.scope);
    else if (sub === "wait-url") await waitFor("url", o.includes, o.timeout);
    else if (sub === "screenshot") await screenshot(o.path);
    else if (sub === "snapshot") await snapshot(o.path);
    else if (sub === "downloads") await downloads(o["copy-to"]);
    else if (sub === "cookies") await cookies();
    else if (sub === "text") await showText();
    else die(`unknown browser command ${sub || ""}`);
  } else {
    die(`unknown command ${command}`);
  }
} catch (error) {
  console.error(error?.message || error);
  process.exit(1);
}
