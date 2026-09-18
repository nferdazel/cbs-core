"use client";

import type { StaffUser, LoginResponse } from "./types";

/**
 * Penyimpanan sesi.
 *
 * TODO(security): saat ini token disimpan di localStorage karena backend masih
 * mengembalikan token di body. Standar produksi (lihat BACKLOG "Baseline security")
 * adalah httpOnly + Secure + SameSite cookie dengan CSRF double-submit. Perubahan
 * itu memerlukan perubahan backend dan tidak bisa dilakukan murni di frontend.
 */

const ACCESS_TOKEN_KEY = "cbs_access_token";
const REFRESH_TOKEN_KEY = "cbs_refresh_token";
const USER_KEY = "cbs_user";

function isBrowser(): boolean {
  return typeof window !== "undefined";
}

export function getAccessToken(): string | null {
  if (!isBrowser()) return null;
  return window.localStorage.getItem(ACCESS_TOKEN_KEY);
}

export function getRefreshToken(): string | null {
  if (!isBrowser()) return null;
  return window.localStorage.getItem(REFRESH_TOKEN_KEY);
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

export function saveSession(session: LoginResponse): void {
  if (!isBrowser()) return;
  window.localStorage.setItem(ACCESS_TOKEN_KEY, session.access_token);
  window.localStorage.setItem(REFRESH_TOKEN_KEY, session.refresh_token);
  if (session.user) {
    window.localStorage.setItem(USER_KEY, JSON.stringify(session.user));
  }
}

export function clearSession(): void {
  if (!isBrowser()) return;
  window.localStorage.removeItem(ACCESS_TOKEN_KEY);
  window.localStorage.removeItem(REFRESH_TOKEN_KEY);
  window.localStorage.removeItem(USER_KEY);
}

export function hasSession(): boolean {
  return Boolean(getAccessToken());
}
