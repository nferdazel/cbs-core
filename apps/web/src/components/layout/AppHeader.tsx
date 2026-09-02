"use client";

import React from "react";
import { Building2, LogOut, Globe, ShieldCheck, Lock, Landmark } from "lucide-react";
import { useTranslation } from "@/i18n/context";
import { Badge } from "@/components/ui/Badge";

interface AppHeaderProps {
  user: any;
  onLogout: () => void;
}

export const AppHeader: React.FC<AppHeaderProps> = ({ user, onLogout }) => {
  const { language, setLanguage, t } = useTranslation();

  return (
    <header className="sticky top-0 z-40 bg-white border-b border-slate-200 px-6 py-2.5 shadow-xs">
      <div className="max-w-7xl mx-auto flex items-center justify-between">
        {/* Brand & System Info */}
        <div className="flex items-center space-x-3">
          <div className="bg-slate-900 text-white p-2 rounded-lg shadow-sm">
            <Building2 className="w-5 h-5 text-white" />
          </div>
          <div>
            <div className="flex items-center space-x-2">
              <span className="font-bold text-base text-slate-900 tracking-tight">
                {t.common.systemTitle}
              </span>
              <Badge variant="info" size="sm">
                {t.common.bprBmtReady}
              </Badge>
            </div>
            <p className="text-[11px] text-slate-500 hidden sm:block">
              {t.common.systemSubtitle}
            </p>
          </div>
        </div>

        {/* Operational Status Badges */}
        <div className="hidden lg:flex items-center space-x-4 text-xs font-mono">
          <div className="flex items-center space-x-1.5 bg-slate-100 border border-slate-200 px-3 py-1 rounded-md text-slate-700">
            <Landmark className="w-3.5 h-3.5 text-slate-500" />
            <span className="font-sans font-medium text-slate-500">{t.header.businessDate}:</span>
            <span className="font-bold text-slate-900">02 Sep 2026</span>
            <span className="bg-emerald-100 text-emerald-800 text-[10px] px-1.5 py-0.2 rounded font-bold">OPEN</span>
          </div>

          <div className="flex items-center space-x-1.5 bg-slate-100 border border-slate-200 px-3 py-1 rounded-md text-slate-700">
            <Lock className="w-3.5 h-3.5 text-emerald-600" />
            <span className="font-sans font-medium text-slate-500">{t.header.vaultStatus}:</span>
            <span className="font-bold text-emerald-700">OPEN</span>
          </div>
        </div>

        {/* Right Section: Language Toggle & User Profile */}
        <div className="flex items-center space-x-3">
          {/* Language Switcher */}
          <div className="flex items-center bg-slate-100 p-0.5 rounded-lg border border-slate-200 text-xs font-medium">
            <button
              onClick={() => setLanguage("id")}
              className={`px-2 py-1 rounded-md transition-colors ${
                language === "id"
                  ? "bg-white text-slate-900 font-bold shadow-xs"
                  : "text-slate-600 hover:text-slate-900"
              }`}
            >
              🇮🇩 ID
            </button>
            <button
              onClick={() => setLanguage("en")}
              className={`px-2 py-1 rounded-md transition-colors ${
                language === "en"
                  ? "bg-white text-slate-900 font-bold shadow-xs"
                  : "text-slate-600 hover:text-slate-900"
              }`}
            >
              🇬🇧 EN
            </button>
          </div>

          {/* User Badge */}
          <div className="flex items-center space-x-2.5 bg-slate-50 border border-slate-200 rounded-lg px-3 py-1">
            <div className="w-7 h-7 rounded-md bg-slate-900 text-white flex items-center justify-center font-bold text-xs font-mono">
              {user?.role?.slice(0, 2) || "SA"}
            </div>
            <div className="text-left hidden md:block">
              <div className="text-xs font-bold text-slate-900 flex items-center space-x-1">
                <span>{user?.full_name || "Staff User"}</span>
                <ShieldCheck className="w-3.5 h-3.5 text-blue-700" />
              </div>
              <div className="text-[10px] text-slate-500 flex items-center space-x-1">
                <span className="font-semibold text-emerald-700">{user?.role || "SUPERADMIN"}</span>
                <span>• {user?.branch_code || "HO"}</span>
              </div>
            </div>
          </div>

          {/* Logout Button */}
          <button
            onClick={onLogout}
            className="p-2 rounded-lg bg-slate-100 hover:bg-red-50 text-slate-600 hover:text-red-600 border border-slate-200 hover:border-red-200 transition-colors text-xs flex items-center gap-1 font-medium"
            title={t.common.logout}
          >
            <LogOut className="w-4 h-4" />
            <span className="hidden xl:inline">{t.common.logout}</span>
          </button>
        </div>
      </div>
    </header>
  );
};
