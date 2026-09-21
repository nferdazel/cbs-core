# Rencana Kerja

Cara kerja: **kerjakan → audit → perbaiki temuan → ulangi → item berikutnya → ulangi.**
Setiap item dianggap selesai hanya setelah auditnya lewat. Item yang menyentuh uang
(posting, PPKA, bunga, pajak) diaudit oleh peninjau independen, bukan hanya oleh penyusunnya.

Urutan disusun menurut risiko, bukan kemudahan: yang paling mungkin menyembunyikan bug
duluan, karena bug yang tertangkap lebih awal jauh lebih murah.

## Item 1 — Uji integrasi jalur uang baru pada database nyata

Cakupan: pembatalan pencairan kredit, koreksi nominal pencairan, run PPAP dengan agunan di
balik saklar (Pasal 20, Pasal 21, agunan tunai), dan pencarian nama lewat indeks token.

Alasan jadi yang pertama: dua bug nyata sebelumnya — `Source` yang tidak diteruskan sehingga
seluruh pembatalan ditolak, dan tanggal jurnal kontra yang memakai tanggal kalender — hanya
tertangkap lewat database sungguhan. Unit test memeriksa permintaan, bukan baris yang tersimpan.

Kriteria selesai: seluruh alur lolos pada database yang dibangun dari nol (seluruh rantai
migrasi), container uji dibersihkan setelahnya, dan temuan dicatat apa adanya.

## Item 2 — CKPN menurut standar akuntansi

Kewajiban sejak 1 Januari 2025, terpisah dari PPKA. Bangun kerangkanya: CKPN sebagai konsep
penyisihan akuntansi dengan metodologi sebagai **parameter kebijakan bank**, bukan angka yang
dikarang pengembang. Sediakan perbandingan PPKA versus CKPN dan jurnalnya.

Kriteria selesai: tidak ada parameter metodologi yang dikarang; selisih PPKA vs CKPN terlihat
di laporan; saklar mematikan berarti tidak ada perubahan perilaku.

## Item 3 — Kerugian restrukturisasi (Pasal 32), tiga langkah berurutan

3a simpan suku bunga efektif orisinal; 3b hitung PPKA atas saldo setelah kerugian; 3c pemetaan
jurnal beban penurunan nilai. Ketiganya prasyarat; mengerjakan jurnalnya lebih dulu akan
menghasilkan angka yang salah.

## Item 4 — Pasal 23: pengurang PPKA untuk penempatan yang dijamin LPS

Aturan yang belum diimplementasikan sama sekali, dan baru ketahuan saat koreksi pasal.

## Item 5 — Ekspor OJK lanjutan

Form lain (00.00, 01.01, 05.00, 06.00, 09.00, 13.00, 00.13-00.15) dan komponen rasio yang belum
tersedia (ATMR, kualitas kredit per debitur, rata-rata aset). Menunggu bahan dari bank: panduan
konversi COA yang diwajibkan SEOJK.

## Item 6 — Sisa kepatuhan agunan

Kolom NJOP tersendiri untuk huruf d/e, agunan tunai per porsi sesuai Pasal 17 ayat (1), dan
penanda kriteria penjamin BUMN/BUMD.

## Item 7 — Pemisahan kunci blind index dan AAD (bertahap)

Kolom versi kunci dan dukungan baca lebih dulu, rotasi menyusul.

## Item 8 — Sisa UX

Pencarian nama di antarmuka, dan perbaikan menu yang menampilkan item yang akan ditolak 403
untuk sebagian peran.

## Item 9 — Housekeeping (perlu tindakan pemilik sistem)

Rotasi password superadmin, keputusan kepemilikan database (`cbs` dimiliki role aplikasi,
seharusnya role admin), penghapusan berkas kredensial sementara, dan push 56 commit yang
masih ditahan.

## Saklar yang sengaja tetap mati

`ppap.collateral.enabled` tetap `false` sampai audit Item 1 dan kepatuhan agunan selesai.
