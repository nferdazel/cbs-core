/**
 * Setelan Prettier untuk apps/web.
 *
 * `printWidth` sengaja tetap 80 (bawaan Prettier), bukan dinaikkan: pada 100,
 * Prettier menggabungkan ekspresi JSX bertipe `x ?? <span>...</span>` menjadi
 * satu baris sehingga penjaga `pnpm check:i18n` salah menandai potongan kode
 * sebagai teks pengguna. Lebar 80 mempertahankan bentuk ber-tanda-kurung dan
 * penjaga tetap lulus.
 *
 * Berkas lama diformat sekali di perubahan ini (mekanis); setelah itu CI
 * `prettier --check` menahan kemunduran format berikutnya.
 */
/** @type {import("prettier").Config} */
export default {
  printWidth: 80,
  semi: true,
  singleQuote: false,
  trailingComma: "all",
};
