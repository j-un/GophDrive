import eslint from "@eslint/js";
import tseslint from "typescript-eslint";

export default tseslint.config(
  eslint.configs.recommended,
  ...tseslint.configs.recommended,
  {
    ignores: [
      "**/*.d.ts",
      "node_modules/**",
      "cdk.out/**",
      "coverage/**",
      "eslint.config.mjs",
    ],
  },
  {
    rules: {
      "@typescript-eslint/no-var-requires": "off",
      // Allow any type where necessary, but we will fix any easily fixable ones
    },
  },
);
