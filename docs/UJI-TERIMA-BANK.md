# Uji Terima Bank — membuktikan sistem tidak merugikan

> **Untuk calon bank pertama.** Dokumen ini memberi skenario yang dapat Anda jalankan
> sendiri untuk membuktikan sistem bekerja benar, **sebelum** mempercayakannya uang
> nyata. Tidak ada angka karangan di sini: tiap langkah memakai alur nyata aplikasi, dan
> tiap harapan dapat diperiksa di layar atau di laporan.
>
> Cara pakai: sediakan satu instalasi uji (bukan produksi), buat **satu cabang, satu
> produk tabungan, satu produk kredit**, lalu jalankan skenario di bawah berurutan.
> Tandai LULUS/GAGAL. Bila ada yang gagal, itu temuan untuk pengembang — bukan untuk
> Anda akali.

Alasan tiap uji dipilih: ini titik-titik di mana **kesalahan sistem langsung berarti
kerugian bank atau sanksi OJK**, jadi paling wajib dibuktikan.

---

## U-01 — Setoran & penarikan tidak membuat uang hilang

**Kenapa penting:** kesalahan jurnal = saldo salah = kerugian/keluhan nasabah.

1. Teller → setor Rp 1.000.000 ke satu rekening tabungan.
2. Buka Buku Besar untuk transaksi itu.
3. Terima bila: jumlah **debit = kredit** persis (Rp 1.000.000 keduanya), dan saldo
   rekening naik tepat Rp 1.000.000.
4. Teller → tarik Rp 300.000 dari rekening itu. Terima bila saldo turun tepat
   Rp 300.000 dan jurnalnya kembali seimbang.
5. **GAGAL bila:** saldo akhir ≠ Rp 700.000, atau ada jurnal yang tidak seimbang.

## U-02 — Penarikan melebihi saldo ditolak

**Kenapa penting:** sistem harus menolak, bukan memberi saldo minus.

1. Dari rekening bersaldo Rp 700.000, coba tarik Rp 1.000.000.
2. Terima bila: **ditolak** dengan pesan "saldo tidak mencukupi".
3. **GAGAL bila:** transaksi lolos dan saldo jadi negatif.

## U-03 — Transaksi besar masuk antrean persetujuan (maker-checker)

**Kenapa penting:** ini kontrol wajib; tanpa ini tidak ada pemisahan tugas.

1. Sebagai **teller**, jalankan transaksi yang melewati ambang persetujuan.
2. Terima bila: transaksi **tidak langsung** mengubah saldo, melainkan muncul di
   halaman **Persetujuan** sebagai "menunggu".
3. Sebagai **pejabat pemeriksa berbeda**, setujui. Terima bila saldo baru berubah
   **setelah** persetujuan.
4. **GAGAL bila:** teller bisa menyetujui transaksinya sendiri, atau saldo berubah
   sebelum disetujui.

## U-04 — Tutup hari (EOD) mengunci periode & mengakrual

**Kenapa penting:** akrual bunga/penyusutan dan penguncian tanggal adalah inti
akuntansi bank.

1. Jalankan **Tutup Hari** untuk satu tanggal.
2. Terima bila: ringkasan menyebut akun diproses/dilewati, dan tanggal bisnis maju.
3. Coba buat transaksi bertanggal **sebelum** tutup hari.
4. **GAGAL bila:** sistem menerima transaksi mundur tanpa peringatan, atau EOD gagal
   tanpa menyebut sebab.

## U-05 — Laporan OJK menolak angka yang belum diratifikasi

**Kenapa penting:** mengirim angka CKPN sementara ke OJK = risiko sanksi.

1. Ekspor laporan bulanan OJK dari menu Laporan (izin `reports:export`).
2. Terima bila: bila parameter CKPN masih **SEMENTARA** dan CKPN menyala, ekspor
   **ditolak** dengan pesan yang menyebut langkah perbaikan (bukan angka tebakan).
3. **GAGAL bila:** ekspor berhasil padahal parameter belum diratifikasi.

> Ini sudah diuji pada instalasi contoh: percobaan memaksa status parameter menjadi
> `FINAL` tanpa bukti berita acara **ditolak** oleh trigger basis data.

## U-06 — Kolom yang belum ada datanya ditulis "-", bukan 0

**Kenapa penting:** nol palsu menyesatkan analisis OJK; "-" jujur.

1. Lihat form OJK yang memuat kolom yang memang tidak disimpan sistem (mis. Nomor
   Identitas pada beberapa form).
2. Terima bila: kolom itu bertanda **"-"** disertai alasan, **bukan "0"**.
3. **GAGAL bila:** ada kolom berisi 0 yang sebenarnya berarti "tidak ada sumber data".

## U-07 — Cakupan cabang tidak bocor

**Kenapa penting:** petugas cabang A tidak boleh melihat/mengubah data cabang B.

1. Masuk sebagai pengguna yang **terbatas cabang A**.
2. Buka daftar rekening/nasabah dan daftar audit.
3. Terima bila: hanya data cabang A yang tampil (peran lintas cabang seperti
   SUPERADMIN/AUDITOR memang melihat semua — itu benar).
4. **GAGAL bila:** pengguna cabang A melihat data cabang B.

## U-08 — Pemisahan wewenang pada register OJK

**Kenapa penting:** data OJK harus bisa diisi, tapi hanya oleh yang berwenang.

1. Sebagai peran tanpa izin konfigurasi, coba ubah satu register (mis. modal/pihak
   lawan).
2. Terima bila: **ditolak** (401/403), dan rute baca tanpa login juga ditolak.
3. **GAGAL bila:** siapa pun dapat mengubah data kelembagaan/register.

---

## Setelah semua LULUS

Anda siap melanjutkan ke **`ONBOARDING-BANK.md`**: mengisi identitas bank, parameter
CKPN (dengan ratifikasi), pemetaan COA, dan data register. Sampai itu selesai, laporan
OJK akan **menolak** atau menandai kolom "belum tersedia" — sesuai desain, bukan
kekurangan.

## Bila ada yang GAGAL

Catat: langkah ke berapa, apa yang diharapkan, apa yang terjadi, pada tanggal/versi mana.
Serahkan ke pengembang. **Jangan** mematikan pemeriksaan untuk "melancarkan" — setiap
penolakan di atas sengaja ada.
