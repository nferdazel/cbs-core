"use client";

import type { StaffUser } from "./types";

/**
 * Penyimpanan sesi.
 *
 * Token TIDAK disimpan di JavaScript. Access & refresh token berada di httpOnly
 * cookie yang ditetapkan backend, sehingga skrip XSS tidak dapat membacanya.
 * localStorage hanya menyimpan profil user non-sensitif untuk tampilan.
 *
 * Status login yang otoritatif ditentukan cookie di server; verifikasi
 * dilakukan lewat GET /auth/me (lihat useAuth). Tidak ada flag token lokal yang
 * bisa dipalsukan.
 */

const USER_KEY = "cbs_user";

/** Nama cookie CSRF non-httpOnly, harus sama dengan backend (CBS_CSRF_COOKIE). */
const CSRF_COOKIE = process.env.NEXT_PUBLIC_CSRF_COOKIE || "csrf_token";

function isBrowser(): boolean {
  return typeof window !== "undefined";
}

export function getStoredUser(): StaffUser | null {
  if (!isBrowser()) return null;
  const raw = window.localStorage.getItem(USER_KEY);
  if (!raw) return null;
  try {
    return JSON.parse(raw) as StaffUser;
  } catch {
    return null;
  }
}

export function setStoredUser(user: StaffUser): void {
  if (!isBrowser()) return;
  window.localStorage.setItem(USER_KEY, JSON.stringify(user));
}

/** Membersihkan cache profil non-sensitif. Cookie sesi hanya bisa dihapus server. */
export function clearSession(): void {
  if (!isBrowser()) return;
  window.localStorage.removeItem(USER_KEY);
}

/**
 * Membaca token CSRF double-submit dari cookie non-httpOnly. Backend menetapkan
 * cookie ini saat login/refresh; JS mengirimnya kembali lewat header
 * X-CSRF-Token untuk request yang mengubah state.
 */
export function getCsrfToken(): string | null {
  if (!isBrowser()) return null;
  const escaped = CSRF_COOKIE.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const match = document.cookie.match(
    new RegExp(`(?:^|;\\s*)${escaped}=([^;]*)`),
  );
  return match ? decodeURIComponent(match[1]) : null;
}
