import type { APIResponse } from "@cbs/shared-types";
import { clearSession, getCsrfToken, setStoredUser } from "./auth";
import type { LoginResponse } from "./types";

/**
 * Base URL API. Satu tempat menentukan alamat backend.
 *
 * `NEXT_PUBLIC_API_URL` boleh ditulis dengan atau tanpa suffix `/api/v1`:
 *   - "https://api.qouver.com/cbs"          -> ditambah "/api/v1"
 *   - "https://api.qouver.com/cbs/api/v1"   -> dipakai apa adanya
 *
 * Ini mencegah path menjadi "/api/v1/api/v1/..." ketika operator sudah menulis
 * versi lengkap di environment variable.
 */
const FALLBACK_BASE_URL = "https://api.qouver.com/cbs";

export function normalizeBaseUrl(raw?: string): string {
  const base = (raw && raw.trim()) || FALLBACK_BASE_URL;
  const withoutTrailingSlash = base.replace(/\/+$/, "");
  if (/\/api\/v1$/i.test(withoutTrailingSlash)) {
    return withoutTrailingSlash;
  }
  return `${withoutTrailingSlash}/api/v1`;
}

export const API_BASE_URL = normalizeBaseUrl(process.env.NEXT_PUBLIC_API_URL);

export interface ApiErrorPayload {
  status: number;
  message: string;
  data?: unknown;
}

export class ApiError extends Error {
  readonly status: number;
  readonly payload?: unknown;

  constructor({ status, message, data }: ApiErrorPayload) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.payload = data;
  }
}

export interface RequestOptions extends Omit<RequestInit, "body"> {
  /** Body JSON; otomatis di-stringify. */
  body?: unknown;
  /** Izinkan refresh otomatis saat 401 (default true). */
  auth?: boolean;
  /** Nilai header Idempotency-Key untuk transaksi finansial. */
  idempotencyKey?: string;
}

const STATE_CHANGING_METHODS = new Set(["POST", "PUT", "PATCH", "DELETE"]);

function redirectToLogin(): void {
  if (typeof window === "undefined") return;
  if (window.location.pathname.startsWith("/login")) return;
  window.location.assign("/login");
}

function defaultErrorMessage(status: number): string {
  if (status === 401) return "Sesi berakhir, silakan masuk kembali.";
  if (status === 403) return "Anda tidak memiliki izin untuk tindakan ini.";
  if (status === 404) return "Data tidak ditemukan.";
  if (status >= 500) return "Terjadi kesalahan pada server.";
  return "Permintaan gagal diproses.";
}

// Satu refresh berjalan sekaligus: beberapa permintaan yang kena 401 bersamaan
// tidak memicu beberapa kali POST /auth/refresh.
let refreshInFlight: Promise<boolean> | null = null;

async function refreshSession(): Promise<boolean> {
  if (refreshInFlight) return refreshInFlight;

  refreshInFlight = (async () => {
    try {
      // Refresh token ada di httpOnly cookie; tidak ada yang dikirim di body.
      const res = await fetch(`${API_BASE_URL}/auth/refresh`, {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
      });
      const payload = (await res.json()) as APIResponse<LoginResponse>;
      if (!res.ok || !payload?.success || !payload.data) return false;
      if (payload.data.user) setStoredUser(payload.data.user);
      return true;
    } catch {
      return false;
    } finally {
      refreshInFlight = null;
    }
  })();

  return refreshInFlight;
}

async function performFetch(
  path: string,
  options: RequestOptions
): Promise<Response> {
  const { body, idempotencyKey, headers, ...init } = options;
  const method = (init.method ?? "GET").toString().toUpperCase();
  const mergedHeaders = new Headers(headers);
  mergedHeaders.set("Accept", "application/json");
  if (body !== undefined) {
    mergedHeaders.set("Content-Type", "application/json");
  }

  // CSRF double-submit: kirim kembali nilai cookie non-httpOnly hanya untuk
  // request yang mengubah state. Login/refresh tidak memerlukannya (backend
  // juga tidak memeriksanya).
  if (STATE_CHANGING_METHODS.has(method)) {
    const csrf = getCsrfToken();
    if (csrf) mergedHeaders.set("X-CSRF-Token", csrf);
  }

  if (idempotencyKey) {
    mergedHeaders.set("Idempotency-Key", idempotencyKey);
  }

  return fetch(`${API_BASE_URL}${path}`, {
    ...init,
    credentials: "include",
    headers: mergedHeaders,
    body: body === undefined ? undefined : JSON.stringify(body),
  });
}

/**
 * Permintaan generik ke API.
 * - Mengirim cookie sesi (`credentials: "include"`) dan header `X-CSRF-Token`
 *   untuk metode yang mengubah state. Tidak ada token di JS/localStorage.
 * - Mengembalikan `APIResponse<T>`.
 * - Melempar `ApiError` (status + pesan) untuk status non-2xx.
 * - Menangani 401 sekali dengan refresh lalu mengulang; bila gagal, sesi
 *   dibersihkan dan pengguna diarahkan ke /login.
 */
export async function request<T = unknown>(
  path: string,
  options: RequestOptions = {}
): Promise<APIResponse<T>> {
  let response = await performFetch(path, options);

  if (response.status === 401 && options.auth !== false) {
    const refreshed = await refreshSession();
    if (refreshed) {
      response = await performFetch(path, options);
    } else {
      clearSession();
      redirectToLogin();
      throw new ApiError({
        status: 401,
        message: "Sesi berakhir, silakan masuk kembali.",
      });
    }
  }

  let payload: APIResponse<T> | null = null;
  try {
    payload = (await response.json()) as APIResponse<T>;
  } catch {
    payload = null;
  }

  if (!response.ok || !payload?.success) {
    throw new ApiError({
      status: response.status,
      message: payload?.error || payload?.message || defaultErrorMessage(response.status),
      data: payload,
    });
  }

  return payload;
}

/**
 * Kunci idempotensi untuk transaksi finansial. Dikirim sebagai header
 * Idempotency-Key agar retry jaringan tidak membuat transaksi ganda.
 */
export function newIdempotencyKey(): string {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) {
    return crypto.randomUUID();
  }
  return `idem-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

/** Mengambil `data` dari response dan melempar bila kosong. */
export function unwrap<T>(payload: APIResponse<T>): T {
  if (payload.data === undefined || payload.data === null) {
    throw new ApiError({
      status: 200,
      message: payload.message || "Respons API tidak berisi data.",
    });
  }
  return payload.data;
}
