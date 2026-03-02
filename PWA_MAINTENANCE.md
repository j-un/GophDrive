# PWA Maintenance Guide

GophDrive operates as a Progressive Web App (PWA) and utilizes a Service Worker for robust cache management.

## Mandatory Cache Update Rules

Whenever you make changes to the following components, you **MUST** update the `CACHE_NAME` in `frontend/public/sw.js`.

- **frontend/src/**: Changes to UI, logic, styles, or frontend code.
- **core/**: Changes to Go core logic (this updates `core.wasm`).
- **frontend/public/**: Changes to static assets like icons, WASM-related files, etc.

### Update Procedure

Update the `CACHE_NAME` at the beginning of `frontend/public/sw.js` using the format `gophdrive-YYYYMMDD-NN` (Date + Sequence Number).

```javascript
// frontend/public/sw.js
const CACHE_NAME = 'gophdrive-20260303-01';
```

### Why is this necessary?

- **Detection**: The browser detects an update only when the `sw.js` file changes by at least one byte.
- **Cache Purging**: Changing the `CACHE_NAME` forces the browser to discard the old cache and fetch the latest files from the network.
- **Integrity**: Failure to update the cache name may result in users running outdated code or mismatched WASM binaries, leading to crashes or data inconsistency.
