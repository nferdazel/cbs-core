import type { AppInfo } from "./types";

/**
 * Identitas aplikasi bawaan. Dipakai hanya bila endpoint /api/v1/app-info belum
 * terjangkau (mis. saat build tanpa API). Nilainya sengaja sama dengan nilai seed
 * migrasi 000075 supaya tidak ada perubahan visual tak sengaja.
 */
export const DEFAULT_APP_INFO: AppInfo = {
  company_name: "",
  display_name: "CBS Core Backoffice",
  short_name: "CBS Core",
  description: "Core Banking & Akuntansi Double-Entry",
  logo_url: "",
};

/**
 * Menggabungkan identitas dari server dengan bawaan, agar field yang belum ada di
 * respons tidak menjadi `undefined` di UI.
 */
export function mergeAppInfo(
  data: Partial<AppInfo> | null | undefined,
): AppInfo {
  if (!data) return DEFAULT_APP_INFO;
  return {
    company_name: data.company_name ?? DEFAULT_APP_INFO.company_name,
    display_name: data.display_name ?? DEFAULT_APP_INFO.display_name,
    short_name: data.short_name ?? DEFAULT_APP_INFO.short_name,
    description: data.description ?? DEFAULT_APP_INFO.description,
    logo_url: data.logo_url ?? DEFAULT_APP_INFO.logo_url,
  };
}
