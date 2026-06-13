import React from "react";
import { Outlet, ScrollRestoration } from "react-router";
import { ThemeProvider } from "./components/ThemeProvider";
import { AuthProvider } from "./context/AuthContext";

export default function App() {
  return (
    <ThemeProvider>
      <AuthProvider>
        <ScrollRestoration />
        <Outlet />
      </AuthProvider>
    </ThemeProvider>
  );
}
