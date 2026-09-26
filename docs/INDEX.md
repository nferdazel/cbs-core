# Peta Dokumen — sumber kanonik per topik

Satu topik, satu tempat. Kalau dua dokumen berbeda, yang di tabel ini yang berlaku;
dokumen lain hanya meringkas atau merujuk balik ke sini.

| Topik | Dokumen kanonik | Catatan |
|---|---|---|
| Ikhtisar, tata letak repo, cara jalan, uji | [`README.md`](../README.md) | Titik masuk untuk pengembang baru |
| Deploy, role DB, bootstrap, pemulihan, batas laju | [`DEPLOY.md`](DEPLOY.md) | §7 = keputusan batas laju lintas instance |
| Keputusan produk & perilaku (lintas topik) | [`KEPUTUSAN.md`](KEPUTUSAN.md) | Indeks keputusan; sub-dokumen di bagian atasnya |
| Angka kanonik CKPN/PD/LGD, ta'zir, agunan, identitas bank | [`KEPUTUSAN-PANEL-RISIKO-CKPN.md`](KEPUTUSAN-PANEL-RISIKO-CKPN.md) | §1.2 = nilai PD/LGD SEMENTARA; §2 = ta'zir |
| Kesiapan rilis CKPN & runbook menyalakan | [`CKPN-SIAP-RILIS.md`](CKPN-SIAP-RILIS.md) | §(e)/(i) = langkah aktivasi; §(f) = KPMM |
| Rancangan teknis CKPN individual | [`CKPN-INDIVIDUAL-RANCANGAN.md`](CKPN-INDIVIDUAL-RANCANGAN.md) | Tabel 7.2 = status tahap T0–T5 |
| Naskah SK DPS tentang ta'zir (siap tanda tangan) | [`draft-surat-keputusan-dps-tazir.md`](draft-surat-keputusan-dps-tazir.md) | Artefak aksi; aturannya di panel §2 |
| Galat dinamis & kamus i18n | [`galat-dinamis-i18n.md`](galat-dinamis-i18n.md) | Template pesan & uji keselarasan |
| Konvensi penamaan tabel & retensi | [`PENAMAAN-TABEL.md`](PENAMAAN-TABEL.md) | Termasuk riwayat rename `deposits` → `time_deposits` |
| Laporan satu kali (jangan dianggap kebijakan) | [`LAPORAN_PEMULIHAN_CADANGAN_2026-09-21.md`](LAPORAN_PEMULIHAN_CADANGAN_2026-09-21.md) | Snapshot tanggal; bukan pedoman |
| Celah kelengkapan laporan OJK (triase) | [`CELAH-LAPORAN-OJK.md`](CELAH-LAPORAN-OJK.md) | 46 kolom + 14 laporan; memisah 'akan dimodelkan' vs 'di luar cakupan' |
| Kontrak API | [`openapi/openapi.yaml`](openapi/openapi.yaml) | Dijaga `openapi_guard_test.go` |
| Arah desain UI | `../DESIGN.md` | **Lokal (gitignored)** |
| Catatan kerja & backlog harian | `BACKLOG.md` | **Lokal (gitignored)**; berisi snapshot per putaran |

## Aturan singkat

- **Angka PD/LGD/CKPN** → panel risiko (§1.2). Dokumen lain tidak mengulang nilainya.
- **"Boleh menyalakan CKPN?"** → `CKPN-SIAP-RILIS.md` §(h), bukan `KEPUTUSAN.md`.
- **"Apa status fitur X?"** → `BACKLOG.md` (lokal) untuk kronologi, lalu dokumen kanonik
  topik itu untuk kebenaran akhirnya.
- Kalau dokumen menyebut file/kode, verifikasi ke kode: **kode dan migrasi menang** bila
  berbeda.
