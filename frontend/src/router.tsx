import React from "react";
import { createBrowserRouter } from "react-router";
import App from "./App";
import HomePage from "./pages/HomePage";
import DrivePage from "./pages/DrivePage";
import NotePage from "./pages/NotePage";
import SettingsPage from "./pages/SettingsPage";
import PrivacyPage from "./pages/PrivacyPage";
import TermsPage from "./pages/TermsPage";
import NotFoundPage from "./pages/NotFoundPage";

export const router = createBrowserRouter([
  {
    path: "/",
    element: <App />,
    children: [
      { index: true, element: <HomePage /> },
      { path: "drive/", element: <DrivePage /> },
      { path: "note/", element: <NotePage /> },
      { path: "settings/", element: <SettingsPage /> },
      { path: "privacy/", element: <PrivacyPage /> },
      { path: "terms/", element: <TermsPage /> },
      { path: "*", element: <NotFoundPage /> },
    ],
  },
]);
