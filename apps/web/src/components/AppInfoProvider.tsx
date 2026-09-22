"use client";

import React, { createContext, useContext, useEffect, useState } from "react";
import { request } from "@/lib/api";
import { DEFAULT_APP_INFO, mergeAppInfo } from "@/lib/app-info";
import type { AppInfo } from "@/lib/types";

/**
 * Identitas aplikasi untuk komponen klien (header, halaman login, judul tab).
 *
 * Nilai awal datang dari server (agar HTML pertama sudah benar), lalu disegarkan
 * dari endpoint publik setelah halaman tampil. Judul tab ditetapkan di klien agar
 * perubahan parameter branding terlihat tanpa build ulang.
 */
const AppInfoContext = createContext<AppInfo>(DEFAULT_APP_INFO);

export const useAppInfo = (): AppInfo => useContext(AppInfoContext);

interface AppInfoProviderProps {
  initial?: AppInfo;
  children: React.ReactNode;
}

export const AppInfoProvider: React.FC<AppInfoProviderProps> = ({
  initial,
  children,
}) => {
  const [info, setInfo] = useState<AppInfo>(initial ?? DEFAULT_APP_INFO);

  useEffect(() => {
    const title = info.display_name.trim();
    if (title) document.title = title;
  }, [info.display_name]);

  useEffect(() => {
    let cancelled = false;
    // Endpoint publik: tanpa auth, jadi 401 tidak memicu alur refresh/redirect.
    request<AppInfo>("/app-info", { auth: false })
      .then((res) => {
        if (!cancelled && res.data) setInfo(mergeAppInfo(res.data));
      })
      .catch(() => {
        // Pertahankan identitas awal; kegagalan identitas tidak boleh merusak halaman.
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <AppInfoContext.Provider value={info}>{children}</AppInfoContext.Provider>
  );
};
