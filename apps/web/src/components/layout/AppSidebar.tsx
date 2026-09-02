"use client";

import React from "react";
import {
  Wallet,
  Landmark,
  ShieldCheck,
  BookOpenCheck,
  Users,
} from "lucide-react";
import { useTranslation } from "@/i18n/context";

export type TabType = "operations" | "loans" | "approvals" | "ledger" | "accounts";

interface AppSidebarProps {
  activeTab: TabType;
  setActiveTab: (tab: TabType) => void;
  pendingApprovalsCount: number;
}

export const AppSidebar: React.FC<AppSidebarProps> = ({
  activeTab,
  setActiveTab,
  pendingApprovalsCount,
}) => {
  const { t } = useTranslation();

  const navItems = [
    { id: "operations" as TabType, label: t.nav.operations, icon: Wallet },
    { id: "loans" as TabType, label: t.nav.loans, icon: Landmark },
    {
      id: "approvals" as TabType,
      label: t.nav.approvals,
      icon: ShieldCheck,
      badge: pendingApprovalsCount > 0 ? pendingApprovalsCount : undefined,
    },
    { id: "ledger" as TabType, label: t.nav.ledger, icon: BookOpenCheck },
    { id: "accounts" as TabType, label: t.nav.accounts, icon: Users },
  ];

  return (
    <nav className="bg-white border-b border-slate-200 px-6 py-1.5 shadow-xs">
      <div className="max-w-7xl mx-auto flex items-center space-x-1 overflow-x-auto">
        {navItems.map((item) => {
          const Icon = item.icon;
          const isActive = activeTab === item.id;

          return (
            <button
              key={item.id}
              onClick={() => setActiveTab(item.id)}
              className={`flex items-center space-x-2 px-4 py-2.5 rounded-lg text-xs font-semibold transition-all border ${
                isActive
                  ? "bg-slate-900 text-white border-slate-900 shadow-xs"
                  : "bg-transparent text-slate-600 hover:text-slate-900 hover:bg-slate-100 border-transparent"
              }`}
            >
              <Icon className="w-4 h-4 shrink-0" />
              <span className="whitespace-nowrap">{item.label}</span>
              {item.badge !== undefined && (
                <span
                  className={`text-[10px] font-mono font-bold px-1.5 py-0.2 rounded-full border ${
                    isActive
                      ? "bg-amber-500 text-slate-950 border-amber-400"
                      : "bg-amber-100 text-amber-900 border-amber-200"
                  }`}
                >
                  {item.badge}
                </span>
              )}
            </button>
          );
        })}
      </div>
    </nav>
  );
};
