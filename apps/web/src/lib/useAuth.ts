"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { clearSession, getAccessToken, getStoredUser, setStoredUser } from "./auth";
import { request } from "./api";
import type { StaffUser } from "./types";

interface MeResponse {
  user_id: string;
  username: string;
  role: string;
  branch_code: string;
  permissions: string[];
}

interface UseAuthResult {
  user: StaffUser | null;
  ready: boolean;
  logout: () => void;
}

/**
 * Guard sesi untuk halaman dashboard. Redirect ke /login bila tidak ada token.
 * Bila token ada tetapi data user hilang, ambil ulang dari GET /auth/me.
 */
export function useAuth(): UseAuthResult {
  const router = useRouter();
  const [user, setUser] = useState<StaffUser | null>(null);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    let cancelled = false;

    if (!getAccessToken()) {
      router.replace("/login");
      setReady(true);
      return () => {
        cancelled = true;
      };
    }

    const stored = getStoredUser();
    if (stored) {
      setUser(stored);
      setReady(true);
      return () => {
        cancelled = true;
      };
    }

    request<MeResponse>("/auth/me")
      .then((res) => {
        if (cancelled || !res.data) return;
        const me = res.data;
        const fallback: StaffUser = {
          id: me.user_id,
          employee_id: "",
          username: me.username,
          full_name: me.username,
          email: "",
          role: me.role,
          branch_code: me.branch_code,
          is_active: true,
          password_changed_at: "",
          created_at: "",
          updated_at: "",
        };
        setStoredUser(fallback);
        setUser(fallback);
      })
      .catch(() => {
        // request() sudah mengarahkan ke /login bila sesi tidak valid.
      })
      .finally(() => {
        if (!cancelled) setReady(true);
      });

    return () => {
      cancelled = true;
    };
  }, [router]);

  const logout = () => {
    // Best-effort revoke di server; kegagalan tidak menghalangi keluar lokal.
    request("/auth/logout", { method: "POST" }).catch(() => undefined);
    clearSession();
    router.replace("/login");
  };

  return { user, ready, logout };
}
