/* eslint-disable react-refresh/only-export-components */
import React from "react";
import { vi } from "vitest";

export const mockNavigate = vi.fn();
export const useNavigate = () => mockNavigate;
export const useSearchParams = (): [
  URLSearchParams,
  (p: URLSearchParams) => void,
] => [new URLSearchParams(), vi.fn()];
export const useLocation = () => ({
  pathname: "/",
  search: "",
  hash: "",
  state: null,
});
export const Link = ({
  to,
  children,
  ...props
}: {
  to: string;
  children: React.ReactNode;
  [key: string]: unknown;
}) => (
  <a href={typeof to === "string" ? to : ""} {...props}>
    {children}
  </a>
);
export const Navigate = ({ to }: { to: string }) => (
  <a href={to}>Navigate to {to}</a>
);
export const ScrollRestoration = () => null;
export const Outlet = () => null;
export const RouterProvider = ({ children }: { children?: React.ReactNode }) =>
  children ?? null;
