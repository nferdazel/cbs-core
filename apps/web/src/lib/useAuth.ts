"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { clearSession, getStoredUser, setStoredUser } from "./auth";
import { request } from "./api";
import type { StaffUser } from "./types";

interface MeResponse {
  user_id: string;
  username: string;
  role: string;
  branch_code: string;
  book?: string;
  book_scope?: string;
  active_books?: string[];
  permissions: string[];
  menus?: string[];
}

interface UseAuthResult {
  user: StaffUser | null;
  ready: boolean;
  logout: () => void;
}

/**
 * Guard sesi untuk halaman dashboard.
 *
 * Karena token ada di httpOnly cookie, status login tidak bisa dibaca dari JS.
 * Sumber kebenaran adalah GET /auth/me: bila cookie tidak valid, request()
 * membersihkan cache dan mengarahkan ke /login. Profil tersimpan hanya dipakai
 * untuk menampilkan UI lebih cepat sebelum verifikasi server selesai.
 */
export function useAuth(): UseAuthResult {
  const router = useRouter();
  const [user, setUser] = useState<StaffUser | null>(null);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    let cancelled = false;

    const stored = getStoredUser();
    if (stored) setUser(stored);

    request<MeResponse>("/auth/me")
      .then((res) => {
        if (cancelled || !res.data) return;
        const me = res.data;
        const profile: StaffUser = {
          id: me.user_id,
          employee_id: "",
          username: me.username,
          full_name: me.username,
          email: "",
          role: me.role,
          branch_code: me.branch_code,
          book: me.book,
          book_scope: me.book_scope,
          active_books: me.active_books,
          permissions: me.permissions,
          menus: me.menus,
          is_active: true,
          password_changed_at: "",
          created_at: "",
          updated_at: "",
        };
        setStoredUser(profile);
        setUser(profile);
      })
      .catch(() => {
        // request() sudah membersihkan cache dan mengarahkan ke /login bila sesi
        // tidak valid.
      })
      .finally(() => {
        if (!cancelled) setReady(true);
      });

    return () => {
      cancelled = true;
    };
  }, [router]);

  const logout = () => {
    // Server menghapus cookie sesi & CSRF; kegagalan tidak menghalangi keluar lokal.
    request("/auth/logout", { method: "POST" }).catch(() => undefined);
    clearSession();
    router.replace("/login");
  };

  return { user, ready, logout };
}
