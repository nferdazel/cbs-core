# Inventaris galat dinamis & i18n

Catatan ini adalah hasil audit pesan galat di `apps/api/internal/service/**` yang
menggabungkan teks Indonesia dengan data (mis. `fmt.Errorf("batas minimum produk %s", kode)`).
Saat bahasa instalasi `CBS_LANGUAGE=en`, pesan seperti itu tetap berbahasa Indonesia.

## Pola perbaikan (sudah ada, dipakai putaran ini)

1. Sentinel berkode di `internal/domain`: `NewLocalizedError("kode_katalog", "pesan ID lama")`.
2. Terjemahan `ID` + `EN` di `internal/i18n/catalog.go` (konstanta `Msg*`, `codeList`, dan `catalog`).
   Uji `internal/domain/errors_localized_test.go` menuntut terjemahan `ID` **sama persis**
   dengan pesan sentinel, dan uji `internal/i18n/catalog_test.go` menuntut `EN` ada.
3. Data dinamis **ditambahkan sebagai akhiran** lewat `%w`, bukan di tengah kalimat:

   ```go
   fmt.Errorf("%w %s (%s)", domain.ErrProductAmountAboveMax, product.Code, product.MaxAmount.String())
   ```

   Handler (`translatedDomainError`) menerjemahkan basis lalu menempelkan sisa `err.Error()`
   apa adanya, sehingga makna pesan ID lama utuh.

**Konsekuensi penting:** pola ini hanya dapat menjaga pesan ID identik bila data berada di
**akhir** kalimat. Pesan yang datanya di tengah (mis. `produk %s sedang tidak aktif`,
`rekening %s berada pada buku %s ...`) tidak dapat dipindahkan tanpa mengubah bunyi pesan
Indonesia; menyelesaikannya menuntut dukungan placeholder ber-argumen di katalog
(`Textf` per galat), di luar mekanisme `%w` yang ada.

## Jumlah terukur (non-test, `internal/service/**`)

| Ukuran | Jumlah |
| --- | --- |
| `fmt.Errorf` | 290 |
| `errors.New` | 62 |
| `fmt.Errorf` + `errors.New` | 352 |
| `fmt.Errorf` mengandung `%` (punya data dinamis) | 275 |

Angka tugas menyebut **201 titik**. Perbedaan berasal dari cara menghitung (kemungkinan
menyaring hanya yang berbahasa Indonesia dan bukan galat pembungkus internal seperti
`"membaca ...: %w"`). Inventaris di bawah memakai hitungan saya dan menyebut asumsinya.

## Berkas terpadat (non-test)

| Berkas | fmt.Errorf + errors.New |
| --- | --- |
| `loan_service.go` | 78 |
| `deposit_service.go` | 34 |
| `batch_process_service.go` | 24 |
| `ledger_service.go` | 23 |
| `restructure_loss_service.go` | 18 |
| `account_service.go` | 18 |
| `document_service.go` | 15 |
| `staff_service.go` | 14 |
| `ckpn_service.go` | 14 |
| `savings_interest_service.go` | 12 |
| `customer_service.go` | 12 |
| `posting_service.go` | 11 |

## Putaran ini (kelompok teratas, 25 titik)

Galat validasi pada alur harian staf: pengajuan/pencairan kredit, pembayaran angsuran,
penempatan/penarikan deposito, pembukaan rekening, registrasi nasabah, transfer teller.

| # | Berkas | Pesan (ID lama) | Sentinel |
| --- | --- | --- | --- |
| 1 | `loan_service.go` | nominal di atas maksimum produk (data di akhir) | `ErrProductAmountAboveMax` |
| 2 | `loan_service.go` | jangka waktu di luar rentang produk (data di akhir) | `ErrProductTermOutOfRange` |
| 3-4 | `loan_service.go` | rekening pencairan tidak ditemukan (2 lokasi) | `ErrDisbursementAccountNotFound` |
| 5 | `loan_service.go` | rekening pencairan bukan milik nasabah | `ErrDisbursementAccountNotOwned` |
| 6 | `loan_service.go` | rekening nasabah tidak punya kode COA | `ErrLoanAccountCOAMissing` |
| 7-8 | `loan_service.go` | kredit tidak dalam status aktif (2 lokasi) | `ErrLoanNotActive` |
| 9 | `loan_service.go` | jadwal angsuran tidak ditemukan | `ErrInstallmentNotFound` |
| 10 | `loan_service.go` | angsuran ini sudah dibayar penuh | `ErrInstallmentAlreadyPaid` |
| 11 | `loan_service.go` | hanya kredit aktif yang dapat direstrukturisasi | `ErrRestructureOnlyActiveLoan` |
| 12 | `loan_service.go` | jangka waktu baru harus positif | `ErrRestructureNewTermPositive` |
| 13-14 | `loan_service.go` | nominal recovery harus positif (2 lokasi) | `ErrRecoveryAmountPositive` |
| 15 | `loan_service.go` | alasan pembatalan pencairan wajib diisi | `ErrDisbursementCancelReasonRequired` |
| 16 | `loan_service.go` | alasan koreksi nominal wajib diisi | `ErrLoanCorrectionReasonRequired` |
| 17-18 | `deposit_service.go` | nominal di atas maksimum produk (penempatan + penarikan) | `ErrProductAmountAboveMax` |
| 19-20 | `deposit_service.go` | jangka waktu di luar rentang produk (penempatan + penarikan) | `ErrProductTermOutOfRange` |
| 21-22 | `customer_service.go` | nama lengkap wajib diisi (2 lokasi) | `ErrCustomerNameRequired` |
| 23-24 | `ledger_service.go` | rekening asal dan tujuan tidak boleh sama (2 lokasi) | `ErrSameAccountTransfer` |
| 25 | `account_service.go` | nomor rekening sudah terpakai | `ErrAccountNumberTaken` |

Uji: `internal/handler/http/dynamic_error_i18n_test.go` (2 titik dinamis dibuktikan
berubah bahasa tanpa mengubah pesan ID; satu di antaranya dipakai untuk membuktikan uji
bisa gagal bila sentinel dikembalikan ke `fmt.Errorf`).

## Urutan yang disarankan (putaran berikutnya)

1. **Sisa `loan_service.go` + `deposit_service.go`** yang datanya di akhir, lalu validasi
   statis (`errors.New`) pada alur yang sama: `"kredit tidak terhubung ke produk"`
   (5 lokasi), `"rekening pembayaran angsuran tidak ditemukan"`, dll.
2. **`ledger_service.go` / `posting_service.go`** — penolakan transaksi teller yang
   dilihat pengguna setiap hari.
3. **`account_service.go` / `customer_service.go`** — validasi pembukaan rekening dan
   nasabah; pesan yang datanya di tengah perlu keputusan format (placeholder ber-argumen).
4. **`restructure_loss_service.go`, `savings_interest_service.go`** — operasi berkala.
5. **`batch_process_service.go`, `document_service.go`, `staff_service.go`, `ckpn_service.go`**
   — mayoritas galat operator/dukungan, frekuensi rendah bagi pengguna akhir.
6. Terakhir, galat pembungkus internal (`"membaca ...: %w"`, `"menyimpan ...: %w"`) —
   tidak ditampilkan ke pengguna (ditangkap `isBusinessError`), jadi prioritas rendah.

## Putaran lanjutan 1 (SELESAI)

Ditambahkan mekanisme **placeholder ber-argumen** untuk pesan yang datanya berada di
TENGAH kalimat (mis. "produk ABC bukan produk kredit/pembiayaan"). Sebelumnya hanya
bisa menempelkan data di AKHIR lewat `%w`, yang akan MENGUBAH bunyi pesan Indonesia -
dilarang karena bank membandingkan berkas lama.

- `domain.NewLocalizedErrorf(code, format, args...)` menyimpan argumen; lapisan HTTP
  menerjemahkan lewat `i18n.Textf` sehingga ID dan EN masing-masing menyusun kalimat
  dengan placeholder yang sama, dan data dinamis berada di posisi yang benar.
- Konstruktor berplaceholder: `NotLoanProduct`, `PaymentMethodUnknown`,
  `ProductNotForSavings`, `ProductInactive`, `BranchInactive`, `COAAccountNotFound`.
- Semua sentinel lama yang berbunyi "data di akhir" untuk enam kasus itu DIHAPUS agar
  tidak ada dua bentuk pesan untuk kondisi yang sama.

Titik yang diselesaikan putaran ini:
- `loan_service.go`: produk bukan produk kredit, metode pembayaran tidak dikenal,
  kredit tidak terhubung ke produk (5 lokasi), rekening pembayaran angsuran tidak
  ditemukan, hapus buku (hanya aktif / tanpa sisa pokok / bukan status hapus buku),
  rekening recovery tidak ditemukan, koreksi di bawah pokok dibayar, koreksi di bawah
  pokok jadwal (dipisah karena bunyinya berbeda).
- `deposit_service.go`: cabang tidak aktif, instruksi ARO tidak dikenal.
- `account_service.go`: produk tidak untuk rekening simpanan, produk tidak aktif,
  cabang tidak aktif, akun COA tidak ditemukan (placeholder).
- Uji: `TestPesanValidasiLanjutanIkutBahasaInstalasi` (12 kasus) menegakkan pesan ID
  TIDAK berubah dan pesan EN memakai katalog; dibuktikan bisa gagal dengan mematikan
  jalur `Textf` (raw `%s` bocor ke pesan pengguna -> uji gagal).

### Sisa (urutan berikutnya, masih berlaku)
1. `ledger_service.go` / `posting_service.go`: SEBAGIAN BESAR adalah galat invarian
   posting (jurnal tanpa baris/sisi debit, tanggal bisnis belum terpasang). Ini bukan
   kesalahan masukan pengguna dan tidak masuk `businessErrors`, jadi SENGAJA tidak
   diterjemahkan - menerjemahkannya akan menyiratkan kesalahan operator. Sisanya yang
   benar-benar user-facing: `ErrSelfReversal` sudah berkode.
2. Sisa pesan validasi statis pada alur yang sama bila masih ada.
3. `restructure_loss_service.go`, `savings_interest_service.go`.
4. `batch_process_service.go`, `document_service.go`, `staff_service.go`, `ckpn_service.go`
   (mayoritas galat operator/dukungan, frekuensi rendah).
5. Galat pembungkus internal ("membaca ...: %w") - prioritas rendah, disembunyikan.
