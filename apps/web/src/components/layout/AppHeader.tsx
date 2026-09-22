"use client";

import React, { useEffect, useState } from "react";
import { Building2, LogOut, ShieldCheck } from "lucide-react";
import { useTranslation } from "@/i18n/context";
import { useAppInfo } from "@/components/AppInfoProvider";
import { request } from "@/lib/api";
import { formatDate } from "@/lib/format";
import { StatusBadge } from "@/components/ui/StatusBadge";
import type { SystemBusinessDate, StaffUser } from "@/lib/types";

interface AppHeaderProps {
  user: StaffUser | null;
  onLogout: () => void;
}

/**
 * Header 56px. Tanggal buku diambil dari GET /system/business-date; bila gagal
 * ditampilkan "tidak diketahui", bukan tanggal hardcode. Nama aplikasi dan
 * keterangannya berasal dari identitas server (GET /api/v1/app-info), bukan literal.
 */
export const AppHeader: React.FC<AppHeaderProps> = ({ user, onLogout }) => {
  const { language, setLanguage, t } = useTranslation();
  const appInfo = useAppInfo();
  const [businessDate, setBusinessDate] = useState<SystemBusinessDate | null>(null);
  const [dateUnavailable, setDateUnavailable] = useState(false);
  // Nama bank dari identitas instalasi. Bila profil bank belum diisi, bidang ini
  // kosong dan header jatuh kembali ke keterangan aplikasi, bukan bidang kosong.
  const bankName = appInfo.company_name.trim();

  useEffect(() => {
    let cancelled = false;
    request<SystemBusinessDate>("/system/business-date")
      .then((res) => {
        if (!cancelled && res.data) setBusinessDate(res.data);
        else if (!cancelled) setDateUnavailable(true);
      })
      .catch(() => {
        if (!cancelled) setDateUnavailable(true);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <header className="sticky top-0 z-40 flex h-14 items-center justify-between border-b border-border bg-navy-800 px-4 text-white">
      <div className="flex items-center gap-3">
        <div className="flex h-8 w-8 items-center justify-center rounded-md bg-navy-700">
          {appInfo.logo_url ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img
              src={appInfo.logo_url}
              alt={appInfo.company_name || appInfo.display_name}
              className="h-4 w-4 object-contain"
            />
          ) : (
            <Building2 className="h-4 w-4 text-white" aria-hidden />
          )}
        </div>
        <div title={bankName || undefined}>
          <div className="text-title font-semibold leading-tight">{appInfo.display_name}</div>
          <div className="text-meta text-white/60">
            {bankName || appInfo.description}
          </div>
        </div>
      </div>

      <div className="flex items-center gap-4">
        <div className="flex items-center gap-2 text-meta">
          <span className="text-white/60">{t.header.businessDate}:</span>
          {businessDate ? (
            <>
              <span className="font-mono text-white">
                {formatDate(businessDate.current_date)}
              </span>
              <StatusBadge status={businessDate.status} domain="businessDate" />
            </>
          ) : (
            <span className="font-mono text-white/70">
              {dateUnavailable ? t.header.businessDateUnknown : t.common.loading}
            </span>
          )}
        </div>

        <div
          role="group"
          aria-label={t.common.language}
          className="flex items-center rounded-md border border-white/20 p-0.5 text-meta"
        >
          <button
            type="button"
            aria-pressed={language === "id"}
            onClick={() => setLanguage("id")}
            className={`rounded-sm px-2 py-0.5 transition-colors duration-fast ${
              language === "id" ? "bg-white text-navy-800 font-medium" : "text-white/70 hover:text-white"
            }`}
          >
            ID
          </button>
          <button
            type="button"
            aria-pressed={language === "en"}
            onClick={() => setLanguage("en")}
            className={`rounded-sm px-2 py-0.5 transition-colors duration-fast ${
              language === "en" ? "bg-white text-navy-800 font-medium" : "text-white/70 hover:text-white"
            }`}
          >
            EN
          </button>
        </div>

        <div className="flex items-center gap-2">
          <div className="flex h-8 w-8 items-center justify-center rounded-md bg-navy-700 text-meta font-medium uppercase">
            {user?.role?.slice(0, 2) || "??"}
          </div>
          <div className="text-left">
            <div className="flex items-center gap-1 text-meta font-medium">
              <span>{user?.full_name || user?.username || "-"}</span>
              <ShieldCheck className="h-3 w-3 text-white/60" aria-hidden />
            </div>
            <div className="text-meta text-white/60">
              {user?.role || "-"} &middot; {user?.branch_code || "-"}
            </div>
          </div>
        </div>

        <button
          type="button"
          onClick={onLogout}
          title={t.common.logout}
          className="flex items-center gap-1 rounded-md border border-white/20 px-2 py-1 text-meta text-white/80 transition-colors duration-fast hover:bg-navy-700 hover:text-white"
        >
          <LogOut className="h-4 w-4" aria-hidden />
          <span>{t.common.logout}</span>
        </button>
      </div>
    </header>
  );
};
