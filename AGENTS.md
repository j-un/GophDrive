# AI Agent Operating Procedures for GophDrive

## Project Context
- **Before starting any task**, you MUST read `PROJECT_GUIDE.md` to understand the overall architecture, storage abstraction, and deployment requirements of GophDrive.

## PWA Cache Maintenance (CRITICAL)
GophDrive is a PWA (Progressive Web App) that uses a Service Worker for client-side caching.
**When modifying `frontend/src/`, `core/`, or `frontend/public/`, you MUST follow these steps:**

1. **Update `CACHE_NAME` in `frontend/public/sw.js`**
   - Format: `gophdrive-YYYYMMDD-NN` (e.g., `gophdrive-20260303-01`)
   - **Reason**: The browser only detects an update if `sw.js` changes by at least one byte. Changing the cache name ensures users receive the latest code and WASM binaries.
2. **Include Cache Update in Commit Message**
   - Example: `feat: update feature and PWA cache v20260303-01`

## Wasm Logic Changes
- When modifying Go logic in `core/`, you MUST run `./scripts/internal/build-wasm.sh` to rebuild `frontend/public/core.wasm`.
- **CRITICAL**: Any change to Wasm logic or its interface MUST be accompanied by a PWA cache update (`CACHE_NAME` change).

## Engineering Standards
- Adhere to the patterns defined in `PROJECT_GUIDE.md`.
- Ensure type safety and idiomatic Go/TypeScript code.
