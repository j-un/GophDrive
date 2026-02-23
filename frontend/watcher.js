/* eslint-disable @typescript-eslint/no-require-imports */
const chokidar = require("chokidar");
const fs = require("fs");
const path = require("path");

console.log("Starting custom Docker file watcher for Next.js...");

// Watch all source files, ignoring node_modules and .next cache
const watcher = chokidar.watch(["./src", "./public"], {
  ignored: [/(^|[\/\\])\../, "**/node_modules/**"], // ignore hidden files
  persistent: true,
  usePolling: true,
  interval: 1000,
  binaryInterval: 3000,
  cwd: process.cwd(),
});

const lastUpdate = new Map();

watcher
  .on("change", (filePath) => {
    const fullPath = path.resolve(process.cwd(), filePath);

    // Prevent infinite reload loops by debouncing self-triggered utimesSync
    const nowMs = Date.now();
    const last = lastUpdate.get(fullPath) || 0;
    if (nowMs - last < 2000) {
      return; // Ignore if we just updated this file
    }

    console.log(`[Watcher] File changed: ${filePath}`);
    try {
      // Artificially update the access and modification times to trigger Turbopack
      const now = new Date();
      lastUpdate.set(fullPath, now.getTime());
      fs.utimesSync(fullPath, now, now);
      console.log(
        `[Watcher] Updated timestamp for ${filePath} to force Next.js rebuild.`,
      );
    } catch (err) {
      console.error(`[Watcher] Error updating ${filePath}:`, err);
    }
  })
  .on("error", (error) => console.error(`[Watcher] Error: ${error}`));

console.log("Watcher initialized and polling...");
