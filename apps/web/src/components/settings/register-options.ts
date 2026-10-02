/**
 * Helper bersama untuk kartu register OJK. Sebelumnya `Option`, `withCurrent`,
 * dan `labelOf` disalin apa adanya di 17 kartu; satu definisi di sini mencegah
 * salinan menyimpang antar formulir.
 */

export type Option = { value: string; label: string };

/**
 * Tambahkan opsi saat ini bila belum ada di daftar. Dipakai agar nilai
 * tersimpan yang tidak (lagi) ada di katalog kode tetap tampil di Select,
 * bukan hilang dan diam-diam terkirim ulang.
 */
export function withCurrent(options: Option[], current: string): Option[] {
  if (!current || options.some((option) => option.value === current)) {
    return options;
  }
  return [...options, { value: current, label: current }];
}

/** Label untuk sebuah value; jatuh kembali ke value mentah bila tak ditemukan. */
export function labelOf(options: Option[], value: string): string {
  return options.find((option) => option.value === value)?.label ?? value;
}
