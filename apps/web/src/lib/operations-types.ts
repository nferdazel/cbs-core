/**
 * Tipe respons operasional yang belum ada di @cbs/shared-types.
 * Field disalin dari struct Go di apps/api/internal/domain (jangan menebak).
 * Dipisah dari lib/types.ts agar tidak bertabrakan dengan pekerjaan auth paralel.
 */

import type { JournalEntry } from "@cbs/shared-types";

/**
 * domain.JournalEntry + branch_code. Atribusi cabang jurnal ditambahkan backend
 * belakangan; shared-types belum memuatnya, jadi tipe turunannya ada di sini.
 * branch_code kosong berarti jurnal sistem/batch yang bank-wide (branch_id NULL).
 */
export interface JournalEntryWithBranch extends JournalEntry {
  branch_code?: string;
}

/**
 * Respons 202 dari transaksi yang melewati ambang persetujuan: transaksi tidak
 * diposting, melainkan masuk antrean maker-checker. Bentuknya BUKAN JournalEntry,
 * jadi harus dibedakan sebelum ditampilkan sebagai bukti posting.
 */
export interface PendingApprovalResult {
  request_id: string;
  action_type: string;
  status: "PENDING_APPROVAL";
}

/** Membedakan respons 202 maker-checker dari jurnal yang benar-benar diposting. */
export function isPendingApproval(
  value: unknown,
): value is PendingApprovalResult {
  return (
    typeof value === "object" &&
    value !== null &&
    "request_id" in value &&
    "status" in value
  );
}

/** domain.ProductFamily (product.go) */
export type ProductFamily =
  "SAVINGS" | "TIME_DEPOSIT" | "LOAN" | "CURRENT_ACCOUNT";

/** domain.COABook (product.go) */
export type COABook = "CONVENTIONAL" | "SYARIAH";

/** domain.ProfitScheme (product.go) */
export type ProfitScheme =
  "INTEREST" | "MURABAHAH" | "MUDHARABAH" | "MUSYARAKAH" | "IJARAH" | "WADIAH";

/** domain.ScheduleMethod (product.go) */
export type ScheduleMethod =
  "FLAT" | "ANNUITY" | "SLIDING" | "BAGI_HASIL" | "NONE";

/** domain.BankingProduct (product.go) */
export interface BankingProduct {
  id: string;
  code: string;
  name: string;
  family: ProductFamily;
  book: COABook;
  profit_scheme: ProfitScheme;
  schedule_method: ScheduleMethod;
  rate_annual: string;
  profit_sharing_ratio: string;
  min_amount: string;
  max_amount: string;
  min_term_months: number;
  max_term_months: number;
  allow_partial_payment: boolean;
  early_withdrawal_penalty_rate: string;
  admin_fee: string;
  tax_rate: string;
  is_active: boolean;
}

/** domain.ProfitType (loan.go) */
export type ProfitType = "INTEREST" | "MARGIN" | "BAGI_HASIL";

/** domain.LoanStatus (loan.go) */
export type LoanStatus =
  | "PENDING_APPROVAL"
  | "APPROVED"
  | "DISBURSED"
  | "REJECTED"
  | "PAID_OFF"
  | "DEFAULTED"
  | "WRITTEN_OFF";

/** domain.InstallmentStatus (loan.go) */
export type InstallmentStatus = "PENDING" | "PAID" | "OVERDUE" | "PARTIAL";

/** domain.OJKCollectibility (loan.go) */
export type OJKCollectibility =
  "1_LANCAR" | "2_DPK" | "3_KURANG_LANCAR" | "4_DIRAGUKAN" | "5_MACET";

/** domain.AccrualStatus (loan.go) */
export type AccrualStatus = "ACCRUAL_PERFORMING" | "CASH_BASIS_NPL";

/** domain.LoanSchedule (loan.go) */
export interface LoanSchedule {
  id: string;
  loan_id: string;
  installment_no: number;
  due_date: string;
  principal_amount: string;
  profit_amount: string;
  total_installment: string;
  paid_principal: string;
  paid_profit: string;
  profit_type: ProfitType;
  outstanding_principal: string;
  status: InstallmentStatus;
  paid_at?: string;
  created_at: string;
}

/** domain.Loan (loan.go) */
export interface Loan {
  id: string;
  loan_number: string;
  customer_id: string;
  product_id?: string;
  branch_id?: string;
  disbursement_account_id: string;
  status: LoanStatus;
  collectibility: OJKCollectibility;
  dpd: number;
  accrual_status: AccrualStatus;
  required_ppap: string;
  is_restructured: boolean;
  restructured_count: number;
  restructured_at?: string;
  restructuring_reason?: string;
  principal_amount: string;
  acquisition_cost: string;
  deferred_margin: string;
  interest_rate_annual: string;
  margin_amount: string;
  profit_sharing_ratio: string;
  total_payable: string;
  term_months: number;
  monthly_installment: string;
  outstanding_principal: string;
  penalty_accrued: string;
  akad_number?: string;
  akad_date?: string;
  purpose?: string;
  ao_id?: string;
  approved_by?: string;
  approved_at?: string;
  disbursed_at?: string;
  /**
   * Sandi referensi/inline OJK per kredit (Form 06.00) yang disimpan nullable:
   * kosong/nil berarti belum diisi dan laporan menulis "-", bukan nol. Nominal dan
   * persentase datang sebagai string desimal (shopspring), tanggal sebagai RFC3339.
   */
  ojk_jenis_penggunaan_code?: string;
  ojk_periode_pembayaran_code?: string;
  ojk_kabupaten_code?: string;
  ojk_kelompok_kredit_code?: string;
  ojk_sumber_dana_code?: string;
  ojk_kategori_usaha_code?: string;
  ojk_sifat_kredit_code?: string;
  ojk_penjamin_code?: string;
  ojk_penjamin_bagian_pct?: string | number | null;
  ojk_tanggal_mulai_macet?: string | null;
  ojk_agunan_ppka_amount?: string | number | null;
  ojk_kelonggaran_tarik_amount?: string | number | null;
  ojk_provisi_belum_diamortisasi_amount?: string | number | null;
  ojk_biaya_transaksi_belum_diamortisasi_amount?: string | number | null;
  ojk_pendapatan_bunga_ditangguhkan_amount?: string | number | null;
  ojk_cadangan_kerugian_restrukturisasi_amount?: string | number | null;
  ojk_klasifikasi_aset_code?: string;
  created_at: string;
  updated_at: string;
  schedules?: LoanSchedule[];
}

/** domain.DepositStatus (deposit.go) */
export type DepositStatus = "PLACED" | "MATURED" | "CLOSED" | "BROKEN";

/** domain.AROInstruction (deposit.go) */
export type AROInstruction = "NONE" | "PRINCIPAL" | "PRINCIPAL_AND_PROFIT";

/** domain.Deposit (deposit.go) */
export interface Deposit {
  id: string;
  account_number: string;
  customer_id: string;
  product_id: string;
  branch_id?: string;
  placement_amount: string;
  currency: string;
  term_months: number;
  start_date: string;
  maturity_date: string;
  profit_rate: string;
  yield_rate: string;
  profit_type: ProfitType;
  tax_rate: string;
  aro: boolean;
  aro_instruction: AROInstruction;
  status: DepositStatus;
  accrued_profit: string;
  accrued_tax: string;
  paid_profit: string;
  paid_tax: string;
  early_withdrawal_penalty: string;
  maturity_proceeds: string;
  last_accrual_date?: string;
  closed_at?: string;
  created_at: string;
  updated_at: string;
}

/**
 * domain.DepositPreview (deposit.go). Hasil perhitungan sebelum penempatan disimpan;
 * dipakai teller untuk memeriksa angka sebelum menekan simpan.
 */
export interface DepositPreview {
  placement_amount: string;
  term_months: number;
  start_date: string;
  maturity_date: string;
  profit_type: ProfitType;
  profit_rate: string;
  yield_rate: string;
  tax_rate: string;
  estimated_profit: string;
  estimated_tax: string;
  maturity_proceeds: string;
}

/** domain.DueObligationKind (report.go) */
export type DueObligationKind = "LOAN_INSTALLMENT" | "DEPOSIT_MATURITY";

/** domain.DueObligation (report.go). days_remaining negatif bila sudah lewat. */
export interface DueObligation {
  kind: DueObligationKind;
  reference: string;
  customer_id: string;
  due_date: string;
  amount: string;
  overdue: boolean;
  days_remaining: number;
  installment_no?: number;
  status: string;
}

/**
 * domain.Collectibility (ppap.go): kualitas aset versi numerik, 1 Lancar s.d.
 * 5 Macet. JSON mengirimnya sebagai angka karena tipe dasarnya int.
 */
export type PPAPCollectibility = 1 | 2 | 3 | 4 | 5;

/** domain.PPAPRunItem (ppap.go). */
export interface PPAPRunItem {
  loan_id: string;
  loan_number: string;
  dpd: number;
  collectibility: PPAPCollectibility;
  outstanding: string;
  target: string;
  existing: string;
  adjustment: string;
  collectibility_changed: boolean;
  stop_accrual: boolean;
  posted: boolean;
}

/** domain.PPAPRunFailure (ppap.go). */
export interface PPAPRunFailure {
  loan_id: string;
  loan_number: string;
  error: string;
}

/**
 * domain.PPAPRunSummary (ppap.go). Saat preview, reserve_after dibiarkan nol dan
 * posted selalu false karena tidak ada jurnal.
 */
export interface PPAPRunSummary {
  as_of: string;
  total: number;
  processed: number;
  failed: number;
  skipped: number;
  total_adjustment: string;
  items: PPAPRunItem[] | null;
  failures: PPAPRunFailure[] | null;
  reserve_before: string;
  reserve_after: string;
  preview: boolean;
}

/** domain.OrgUnitLevel (branch.go). CABANG operasional; AREA/WILAYAH opsional. */
export type OrgUnitLevel = "CABANG" | "AREA" | "WILAYAH";

/**
 * domain.Branch (branch.go). address & phone omitempty: bisa tidak dikirim.
 * parent_id & unit_level menandai hierarki organisasi (W14); parent_id kosong
 * berarti unit puncak, dan semua cabang lama bernilai unit_level "CABANG".
 */
export interface Branch {
  id: string;
  code: string;
  name: string;
  address?: string;
  phone?: string;
  is_head_office: boolean;
  is_active: boolean;
  parent_id?: string;
  unit_level: OrgUnitLevel;
}

/** domain.MakerCheckerStatus (maker_checker.go) */
export type MakerCheckerStatus = "PENDING" | "APPROVED" | "REJECTED";

/**
 * makerCheckerResponse (maker_checker_handler.go). Payload memuat `amount`
 * (string) yang ditulis maker_checker_service.go.
 */
export interface MakerCheckerRequest {
  id: string;
  action_type: string;
  payload?: Record<string, unknown>;
  status: MakerCheckerStatus;
  maker_id: string;
  checker_id?: string;
  maker_notes?: string;
  checker_notes?: string;
  reviewed_at?: string;
  created_at: string;
}

/**
 * permission_handler.go: satu anggota grup pengguna (dari staff_users). Hanya
 * field yang aman ditampilkan; kata sandi tidak pernah dikirim.
 */
export interface PermissionGroupMember {
  user_id: string;
  username: string;
  full_name: string;
  role: string;
  is_active: boolean;
}

/**
 * domain.UserGroup lewat permission_handler.go: grup pengguna untuk akses menu
 * dan jenjang kewenangan persetujuan (approval_limit_role). Izin efektif = izin
 * grup ini digabung grup lain (aditif).
 */
export interface PermissionGroup {
  code: string;
  name: string;
  description: string;
  is_system: boolean;
  /** Peran pada matriks limit 000051 yang menjadi jenjang grup; kosong = peran pengguna. */
  approval_limit_role?: string;
  permissions: string[];
  members: PermissionGroupMember[];
}

/** permission_handler.go: pemetaan menu ke izin yang membukanya. */
export interface PermissionMenu {
  menu_key: string;
  permissions: string[];
}

/**
 * Respons permission_handler.go untuk GET /permissions/catalog.
 * available_permissions = seluruh izin yang ditegakkan kode, dikirim server agar
 * web tidak menyimpan salinan daftar izin.
 */
export interface PermissionCatalog {
  groups: PermissionGroup[];
  menus: PermissionMenu[];
  available_permissions: string[];
}

/** domain.BankOffice (kelembagaan.go) — Form 00.04 jaringan kantor. */
export interface BankOffice {
  id: string;
  office_type: string;
  code?: string;
  name: string;
  address?: string;
  city?: string;
  ojk_kabupaten_code?: string;
  opened_at?: string;
  closed_at?: string;
  status: string;
  note?: string;
  created_at?: string;
  updated_at?: string;
}

/** domain.BankManagement (kelembagaan.go) — Form 00.02/00.03 pengurus. */
export interface BankManagement {
  id: string;
  category: string;
  name: string;
  position?: string;
  ojk_position_code?: string;
  license_number?: string;
  license_date?: string;
  started_at?: string;
  ended_at?: string;
  status: string;
  note?: string;
  created_at?: string;
  updated_at?: string;
}

/**
 * domain.KelembagaanReport (kelembagaan.go) lewat handler ExportKelembagaan:
 * GET /reports/ojk/kelembagaan mengirim { report, tables }, jadi daftar kantor
 * dan pengurus berada di data.report. Slice boleh null saat kosong.
 */
export interface KelembagaanSummary {
  as_of: string;
  offices: BankOffice[] | null;
  management: BankManagement[] | null;
  warnings?: string[];
}

/** Respons GET /reports/ojk/kelembagaan (handler membungkus report + tables). */
export interface KelembagaanData {
  report: KelembagaanSummary;
}

/** domain.OffBalanceItem (off_balance.go) — Form 01.01 rekening administratif. */
export interface OffBalanceItem {
  id: string;
  position_code?: string;
  category: string;
  description: string;
  amount: string | number;
  counterparty_customer_id?: string;
  reference?: string;
  as_of: string;
  status: string;
  note?: string;
  created_at?: string;
  updated_at?: string;
}

/** Respons GET /reports/ojk/off-balance/items (kontrak pengisian). */
export interface OffBalanceItemsData {
  items: OffBalanceItem[] | null;
}

/** domain.AYDAItem (ayda.go) — Form 07.00 daftar agunan yang diambil alih. */
export interface AYDAItem {
  id: string;
  collateral_type_code: string;
  collateral_address: string;
  acquisition_date: string;
  initial_recognition_value: string | number;
  accumulated_impairment: string | number;
  net_realizable_value: string | number;
  as_of: string;
  status: string;
  note?: string;
  created_at?: string;
  updated_at?: string;
}

/** Respons GET /reports/ojk/ayda/items (kontrak pengisian). */
export interface AYDAItemsData {
  items: AYDAItem[] | null;
}

/**
 * domain.KepemilikanItem (kepemilikan.go) — Form 00.01 data kepemilikan BPR.
 * Tidak ada bidang identitas: kolom IV No. Identitas sengaja tidak disimpan
 * (keputusan privasi), jadi jangan menambahkannya di sisi klien.
 */
export interface KepemilikanItem {
  id: string;
  shareholder_name: string;
  shareholder_address: string;
  shareholder_type_code: string;
  shareholder_status_code: string;
  nominal_amount: string | number;
  ownership_percentage: string | number;
  change_status_code: string;
  as_of: string;
  status: string;
  note?: string;
  created_at?: string;
  updated_at?: string;
}

/** Respons GET /reports/ojk/kepemilikan/items (kontrak pengisian). */
export interface KepemilikanItemsData {
  items: KepemilikanItem[] | null;
}

/**
 * domain.PinjamanItem (pinjaman.go) — Form 00.07 daftar pinjaman yang diterima.
 * Lima belas kolom, kecuali kolom XV Baki Debet Neto yang turunan (XII - XIII -
 * XIV) dan tidak disimpan. Nominal datang sebagai string/number desimal
 * shopspring, tanggal sebagai RFC3339.
 */
export interface PinjamanItem {
  id: string;
  counterparty_id: string;
  creditor_group_code: string;
  bank_code: string;
  location_code: string;
  jenis_code: string;
  relationship_code: string;
  start_date: string;
  maturity_date: string;
  interest_rate: string | number;
  interest_calc_code: string;
  plafon: string | number;
  collateral_type_code: string;
  collateral_amount: string | number;
  baki_debet: string | number;
  unamortized_transaction_cost: string | number;
  unamortized_discount: string | number;
  as_of: string;
  status: string;
  note?: string;
  created_at?: string;
  updated_at?: string;
}

/** Respons GET /reports/ojk/pinjaman/items (kontrak pengisian). */
export interface PinjamanItemsData {
  items: PinjamanItem[] | null;
}

/**
 * domain.PropertiItem (properti.go), Form 17.00 daftar properti terbengkalai.
 * Kolom I Sandi Kantor tidak diserialisasi (diambil dari kantor pelapor tunggal);
 * kolom IX Jumlah turunan (VII - VIII) dan tidak disimpan. Nominal datang sebagai
 * string/number desimal shopspring, tanggal sebagai RFC3339.
 */
export interface PropertiItem {
  id: string;
  no_register: string;
  jenis_properti_code: string;
  alamat_properti: string;
  koordinat: string;
  tanggal_penetapan: string;
  biaya_perolehan_atau_nilai_wajar: string | number;
  akumulasi_penyusutan_atau_amortisasi: string | number;
  metode_pengukuran_code: string;
  as_of: string;
  status: string;
  note?: string;
  created_at?: string;
  updated_at?: string;
}

/** Respons GET /reports/ojk/properti/items (kontrak pengisian). */
export interface PropertiItemsData {
  items: PropertiItem[] | null;
}

/**
 * domain.AsetTetapItem (aset_tetap.go), Form 08.00 register aset tetap,
 * inventaris, dan aset tidak berwujud. Satu baris adalah satu aset; penggabungan
 * per kombinasi dilakukan builder laporan, bukan di sini. Kolom I Sandi Kantor
 * tidak diserialisasi (diambil dari kantor pelapor tunggal); kolom VIII Nilai
 * Tercatat turunan (V - VI - VII) dan tidak diserialisasi karena hanya berupa
 * metode di domain. Nominal datang sebagai string/number desimal shopspring,
 * tanggal sebagai RFC3339. status_aset_code boleh kosong untuk aset tidak
 * berwujud (sandi jenis 2xx).
 */
export interface AsetTetapItem {
  id: string;
  jenis_aset_code: string;
  sumber_perolehan_code: string;
  status_aset_code: string;
  biaya_perolehan: string | number;
  akumulasi_penyusutan_amortisasi: string | number;
  akumulasi_kerugian_penurunan_nilai: string | number;
  metode_pengukuran_code: string;
  as_of: string;
  status: string;
  note?: string;
  created_at?: string;
  updated_at?: string;
}

/** Respons GET /reports/ojk/aset-tetap/items (kontrak pengisian). */
export interface AsetTetapItemsData {
  items: AsetTetapItem[] | null;
}

/**
 * domain.PenyertaanItem (penyertaan.go), Form 16.00 daftar penyertaan modal.
 * Kolom I Sandi Kantor tidak diserialisasi (diambil dari kantor pelapor tunggal).
 * Tidak ada kolom turunan: kolom X "Jumlah Bulan Laporan" walaupun namanya begitu
 * adalah nilai tercatat penyertaan pada bulan laporan, rupiah penuh yang diisi bank;
 * blok CKPN juga isian bank. Nominal datang sebagai string/number desimal shopspring,
 * tanggal sebagai RFC3339.
 */
export interface PenyertaanItem {
  id: string;
  no_register: string;
  counterparty_id: string;
  metode_penyertaan_code: string;
  kualitas_code: string;
  tujuan_penyertaan_code: string;
  tanggal_mulai: string;
  persentase_penyertaan: string | number;
  nominal: string | number;
  jumlah_bulan_laporan: string | number;
  cadangan_kerugian_penurunan_nilai: string | number;
  ckpn_aset_baik: string | number;
  ckpn_aset_kurang_baik: string | number;
  ckpn_aset_tidak_baik: string | number;
  jenis_ckpn_code: string;
  as_of: string;
  status: string;
  note?: string;
  created_at?: string;
  updated_at?: string;
}

/** Respons GET /reports/ojk/penyertaan/items (kontrak pengisian). */
export interface PenyertaanItemsData {
  items: PenyertaanItem[] | null;
}

/**
 * domain.AsetKeuanganItem (aset_keuangan_lainnya.go), Form 18.00 daftar aset
 * keuangan lainnya. Satu baris adalah satu rekening unik. Kolom I Sandi Kantor
 * tidak diserialisasi (diambil dari kantor pelapor tunggal). Tidak ada kolom
 * turunan: seluruh nilai V-XIII adalah isian bank. Form memakai format tanggal
 * TT-MM-TTTT pada laporan. Nominal datang sebagai string/number desimal
 * shopspring, tanggal sebagai RFC3339.
 */
export interface AsetKeuanganItem {
  id: string;
  no_rekening: string;
  counterparty_id: string;
  jenis_code: string;
  tanggal_mulai: string;
  tanggal_jatuh_tempo: string;
  suku_bunga: string | number;
  nominal: string | number;
  nilai_agunan_diperhitungkan: string | number;
  cadangan_kerugian_penurunan_nilai: string | number;
  ckpn_aset_baik: string | number;
  ckpn_aset_kurang_baik: string | number;
  ckpn_aset_tidak_baik: string | number;
  klasifikasi_aset_keuangan_code: string;
  jenis_ckpn_code: string;
  as_of: string;
  status: string;
  note?: string;
  created_at?: string;
  updated_at?: string;
}

/** Respons GET /reports/ojk/aset-keuangan/items (kontrak pengisian). */
export interface AsetKeuanganItemsData {
  items: AsetKeuanganItem[] | null;
}

/**
 * domain.SindikasiItem (kredit_sindikasi.go), Form 06.02 daftar kredit sindikasi.
 * Satu baris adalah satu rekening fasilitas kredit sindikasi; form tidak punya baris
 * JUMLAH. Kolom I Sandi Kantor tidak diserialisasi (diambil dari kantor pelapor
 * tunggal). Kolom III No. Identitas sengaja tidak ada: tidak disimpan (keputusan
 * privasi) dan laporan menulis "-" beralasan. Tidak ada kolom turunan: seluruh angka
 * adalah isian bank. No. Rekening kosong sah bila pendanaan bukan di bank pelapor
 * (kolom X = sandi "2"). Nominal datang sebagai string/number desimal shopspring,
 * tanggal sebagai RFC3339.
 */
export interface SindikasiItem {
  id: string;
  counterparty_id: string;
  no_rekening?: string;
  jumlah_pendanaan_sindikasi: string | number;
  bagian_pendanaan: string | number;
  sandi_bank_peserta: string;
  plafon: string | number;
  baki_debet: string | number;
  status_kepesertaan_code: string;
  nomor_perjanjian_induk: string;
  pendanaan_di_bank_pelapor_code: string;
  kualitas_code: string;
  tunggakan_pokok: string | number;
  tunggakan_bunga: string | number;
  hari_tunggakan_pokok: number;
  hari_tunggakan_bunga: number;
  as_of: string;
  status: string;
  note?: string;
  created_at?: string;
  updated_at?: string;
}

/** Respons GET /reports/ojk/kredit-sindikasi/items (kontrak pengisian). */
export interface SindikasiItemsData {
  items: SindikasiItem[] | null;
}

/**
 * domain.SuratBerhargaItem (surat_berharga.go), Form 04.00 daftar surat berharga.
 * Kolom I Sandi Kantor tidak diserialisasi (diambil dari kantor pelapor tunggal).
 * Tidak ada kolom turunan: XI "Biaya Perolehan Diamortisasi/Nilai Wajar" adalah
 * isian bank karena form memberi dua kemungkinan (amortized cost atau nilai wajar).
 * XVII/XVIII adalah sandi Lampiran 08/09 (sentinel 9/99 bila tanpa peringkat);
 * persisnya diketik bank. XXIV boleh kosong (hanya bila penawaran umum efek).
 * tanggal_pemeringkatan boleh null. Nominal datang sebagai string/number desimal
 * shopspring, tanggal sebagai RFC3339.
 */
export interface SuratBerhargaItem {
  id: string;
  klasifikasi_code: string;
  suku_bunga: string | number;
  tanggal_mulai: string;
  tanggal_jatuh_tempo: string;
  nominal: string | number;
  nominal_dijaminkan: string | number;
  biaya_perolehan: string | number;
  diskonto_premium_belum_diamortisasi: string | number;
  biaya_transaksi_belum_diamortisasi: string | number;
  laba_rugi_belum_direalisasi: string | number;
  biaya_perolehan_diamortisasi: string | number;
  nomor_surat_berharga: string;
  counterparty_id: string;
  jenis_code: string;
  kualitas_code: string;
  cadangan_kerugian_penurunan_nilai: string | number;
  lembaga_pemeringkat_code: string;
  peringkat_surat_berharga_code: string;
  tanggal_pemeringkatan: string | null;
  tanggal_penerbitan: string;
  ckpn_aset_baik: string | number;
  ckpn_aset_kurang_baik: string | number;
  ckpn_aset_tidak_baik: string | number;
  klasifikasi_aset_keuangan_code: string;
  jenis_ckpn_code: string;
  as_of: string;
  status: string;
  note?: string;
  created_at?: string;
  updated_at?: string;
}

/** Respons GET /reports/ojk/surat-berharga/items (kontrak pengisian). */
export interface SuratBerhargaItemsData {
  items: SuratBerhargaItem[] | null;
}

/** domain.BMPKRelatedParty (bmpk.go) — penandaan pihak terkait per nasabah. */
export interface BMPKRelatedParty {
  customer_id: string;
  relationship_type: string;
  note?: string;
}

/** domain.BMPKLimit (bmpk.go). max_amount rupiah penuh; nol sah. */
export interface BMPKLimit {
  customer_id: string;
  max_amount: string | number;
  effective_date?: string;
  note?: string;
}

/** Respons GET /reports/ojk/bmpk/master (kontrak pengisian). */
export interface BMPKMasterData {
  related_parties: BMPKRelatedParty[] | null;
  limits: BMPKLimit[] | null;
}

/**
 * Satu pilihan penempatan pada bank lain dari GET /reports/ojk/placements. Daftar
 * hanya memuat id dan label; nilai tersimpan diambil terpisah lewat
 * GET /ojk/placement-codes/{placementId}.
 */
export interface OJKPlacementOption {
  id: string;
  label: string;
}

/** Respons GET /reports/ojk/placements (kontrak pengisian). */
export interface OJKPlacementsData {
  placements: OJKPlacementOption[] | null;
}

/**
 * Respons GET/PUT /ojk/placement-codes/{placementId} (domain.OJKPlacementCodesView).
 * Sandi/teks nullable: null berarti belum diisi. Nominal juga null selama belum
 * diisi, dibedakan dari nol; desimal shopspring tiba sebagai number.
 */
export interface OJKPlacementCodesData {
  placement_id: string;
  label: string;
  ojk_kabupaten_code: string | null;
  ojk_hubungan_bank_code: string | null;
  ojk_alasan_diblokir_code: string | null;
  counterparty_cif: string | null;
  ojk_klasifikasi_aset_code: string | null;
  blocked_amount: string | number | null;
  accrued_interest_receivable: string | number | null;
  accrued_interest_pending: string | number | null;
}
