import { cache } from "react";
import { API_BASE_URL } from "./api";
import { DEFAULT_APP_INFO, mergeAppInfo } from "./app-info";
import type { AppInfo } from "./types";

/**
 * Membaca identitas aplikasi dari server (endpoint publik, tanpa cookie).
 *
 * Dipakai `generateMetadata` dan root layout agar judul tab mengikuti parameter
 * server pada setiap render, bukan nilai build. `cache` mendeduplikasi panggilan
 * dalam satu render; `no-store` membuat metadata dihitung saat permintaan sehingga
 * perubahan `branding.display_name` terlihat tanpa build ulang. Kegagalan API tidak
 * menggagalkan render: identitas bawaan dipakai.
 */
export const fetchAppInfo = cache(async (): Promise<AppInfo> => {
  try {
    const res = await fetch(`${API_BASE_URL}/app-info`, {
      cache: "no-store",
      headers: { Accept: "application/json" },
    });
    if (!res.ok) return DEFAULT_APP_INFO;
    const payload = (await res.json()) as {
      success?: boolean;
      data?: Partial<AppInfo> | null;
    };
    if (!payload?.success || !payload.data) return DEFAULT_APP_INFO;
    return mergeAppInfo(payload.data);
  } catch {
    return DEFAULT_APP_INFO;
  }
});
