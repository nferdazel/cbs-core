import type { StaffUser } from "./types";

/**
 * Apakah pengguna memegang izin efektif dari GET /auth/me. Izin berasal dari
 * database (grup pengguna), bukan salinan peran di web. Dipakai hanya untuk
 * menyembunyikan kontrol; penjagaan sebenarnya tetap di API lewat RequirePermission.
 *
 * Izin yang belum dimuat (undefined) diperlakukan sebagai belum berhak supaya
 * kontrol tidak tampil sebelum server memastikan. Server tetap penentu akhir.
 */
export function hasPermission(
  user: Pick<StaffUser, "permissions"> | null | undefined,
  permission: string
): boolean {
  return user?.permissions?.includes(permission) ?? false;
}
