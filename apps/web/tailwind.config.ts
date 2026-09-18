import type { Config } from "tailwindcss";

/**
 * Pemetaan token DESIGN.md ke Tailwind. Nilai hanya boleh berasal dari CSS
 * custom property di globals.css; jangan tulis hex di sini.
 */
const config: Config = {
  content: [
    "./src/pages/**/*.{js,ts,jsx,tsx,mdx}",
    "./src/components/**/*.{js,ts,jsx,tsx,mdx}",
    "./src/app/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      fontFamily: {
        sans: ["var(--font-sans)"],
        mono: ["var(--font-mono)"],
      },
      fontSize: {
        meta: ["var(--text-meta)", { lineHeight: "var(--leading-meta)" }],
        body: ["var(--text-body)", { lineHeight: "var(--leading-body)" }],
        title: ["var(--text-title)", { lineHeight: "var(--leading-title)" }],
        page: ["var(--text-page)", { lineHeight: "var(--leading-page)" }],
      },
      colors: {
        navy: {
          600: "var(--color-navy-600)",
          700: "var(--color-navy-700)",
          800: "var(--color-navy-800)",
        },
        accent: {
          50: "var(--color-accent-50)",
          600: "var(--color-accent-600)",
        },
        credit: {
          50: "var(--color-credit-50)",
          700: "var(--color-credit-700)",
        },
        debit: {
          50: "var(--color-debit-50)",
          700: "var(--color-debit-700)",
        },
        info: {
          700: "var(--color-info-700)",
        },
        surface: "var(--color-surface)",
        canvas: "var(--color-canvas)",
        border: "var(--color-border)",
        "border-strong": "var(--color-border-strong)",
        ink: {
          400: "var(--color-ink-400)",
          600: "var(--color-ink-600)",
          900: "var(--color-ink-900)",
        },
      },
      borderColor: {
        DEFAULT: "var(--color-border)",
      },
      spacing: {
        "1": "var(--space-1)",
        "2": "var(--space-2)",
        "3": "var(--space-3)",
        "4": "var(--space-4)",
        "5": "var(--space-5)",
        "6": "var(--space-6)",
        "8": "var(--space-8)",
      },
      borderRadius: {
        sm: "var(--radius-sm)",
        md: "var(--radius-md)",
      },
      boxShadow: {
        overlay: "var(--shadow-overlay)",
      },
      transitionDuration: {
        fast: "120ms",
      },
    },
  },
  plugins: [],
};
export default config;
