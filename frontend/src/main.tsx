import React from "react";
import ReactDOM from "react-dom/client";
import { RouterProvider } from "react-router";
import { registerSW } from "virtual:pwa-register";
import { router } from "./router";
import "./styles/globals.css";

// Register the Workbox-generated service worker. immediate:true installs the
// new SW as soon as it's downloaded rather than waiting for all tabs to close.
registerSW({ immediate: true });

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <RouterProvider router={router} />
  </React.StrictMode>,
);
