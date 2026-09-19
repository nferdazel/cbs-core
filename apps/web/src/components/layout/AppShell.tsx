"use client";

import React from "react";
import { AppHeader } from "./AppHeader";
import { AppSidebar } from "./AppSidebar";
import { useAuth } from "@/lib/useAuth";
import { LoadingState } from "@/components/ui/States";

/**
 * Kerangka aplikasi: header 56px, sidebar 240px, area konten maks 1440px.
 * Desktop-only: lebar minimum 1024px.
 */
export const AppShell: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { user, ready, logout } = useAuth();

  return (
    <div className="min-h-screen min-w-[1024px] bg-canvas">
      <AppHeader user={user} onLogout={logout} />
      <div className="flex">
        <AppSidebar role={user?.role} />
        <main className="min-w-0 flex-1">
          <div className="mx-auto max-w-[1440px] p-6">
            {!ready ? (
              <LoadingState label="Memeriksa sesi..." />
            ) : user ? (
              children
            ) : null}
          </div>
        </main>
      </div>
    </div>
  );
};
