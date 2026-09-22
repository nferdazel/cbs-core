import { dirname } from "path";
import { fileURLToPath } from "url";
import { FlatCompat } from "@eslint/eslintrc";

/**
 * ESLint flat config (ESLint 9).
 *
 * `next lint` sudah tidak disarankan pada Next 15.5 dan dihapus di Next 16, jadi
 * memakai CLI `eslint` langsung. `eslint-config-next@15` belum mengekspor flat
 * config, sehingga dibungkus FlatCompat (pola resmi dokumentasi Next 15).
 *
 * Urutan penting: prettier diletakkan paling akhir agar aturan gaya yang
 * bertabrakan dengan formatter dimatikan (bukan dimatikan satu per satu).
 */
const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

const compat = new FlatCompat({
  baseDirectory: __dirname,
});

export default [
  {
    ignores: [
      ".next/**",
      "next-env.d.ts",
      "scripts/**",
      "**/*.config.mjs",
      "**/*.config.ts",
    ],
  },
  ...compat.extends("next/core-web-vitals", "next/typescript"),
  {
    rules: {
      // Variabel/impor tak terpakai adalah temuan nyata (sisa refactor). Awalan
      // `_` tetap diizinkan untuk argumen yang sengaja tidak dipakai.
      "@typescript-eslint/no-unused-vars": [
        "error",
        { argsIgnorePattern: "^_", varsIgnorePattern: "^_" },
      ],
      "react-hooks/rules-of-hooks": "error",
      "react-hooks/exhaustive-deps": "warn",
    },
  },
  // Harus paling akhir: mematikan aturan format yang diurus prettier.
  ...compat.extends("prettier"),
];
