/**
 * Tipe respons API yang belum ada di @cbs/shared-types.
 * Field disalin dari struct Go di apps/api/internal/domain (jangan menebak).
 */

import type { Account } from "@cbs/shared-types";

export interface StaffUser {
  id: string;
  employee_id: string;
  username: string;
  full_name: string;
  email: string;
  role: string;
  branch_code: string;
  /**
   * Cakupan buku tingkat instalasi dari GET /auth/me (KONVENSIONAL/SYARIAH/DUAL) dan
   * daftar buku yang aktif. Dipakai menyaring menu, halaman, dan pemilih buku lini
   * usaha yang tidak dilayani instalasi. Server tetap penentu akses; ini hanya UI.
   */
  book?: string;
  book_scope?: string;
  active_books?: string[];
  /**
   * Izin efektif dan kunci menu yang terbuka, dari GET /auth/me. Sumbernya database
   * (grup pengguna), bukan salinan peran di web; sidebar merender menu dari `menus`.
   */
  permissions?: string[];
  menus?: string[];
  is_active: boolean;
  last_login_at?: string;
  password_changed_at: string;
  created_at: string;
  updated_at: string;
}

/**
 * Respons login/refresh. Token tidak lagi dikirim di body; backend menulisnya
 * sebagai httpOnly cookie. Body hanya membawa masa berlaku dan profil user.
 */
export interface LoginResponse {
  expires_in: number;
  refresh_expires_in?: number;
  user: StaffUser;
}

/**
 * domain.Account (account.go) dengan field yang belum ada di @cbs/shared-types.
 * branch_code, last_activity_at, dan dormant_at dipakai halaman rekening serta
 * transaksi untuk menampilkan status dormant dan aksi reaktivasi.
 */
export interface AccountRecord extends Account {
  branch_code?: string;
  last_activity_at?: string;
  dormant_at?: string;
}

/**
 * domain.AppInfo (apps/api/internal/domain/app_info.go). Identitas aplikasi yang
 * boleh dibaca TANPA login dari GET /api/v1/app-info. company_name berasal dari
 * bank_profile.bank_name; sisanya dari kunci konfigurasi branding.*.
 */
export interface AppInfo {
  company_name: string;
  display_name: string;
  short_name: string;
  description: string;
  logo_url: string;
}

/**
 * domain.BankProfile (apps/api/internal/domain/bank_profile.go). Identitas bank
 * tingkat instalasi yang dipakai dokumen cetak dan endpoint /app-info. Nama kosong
 * berarti profil belum dikonfigurasi, bukan nama yang boleh dikarang UI.
 */
export interface BankProfile {
  name: string;
  address: string;
  city: string;
  phone: string;
  npwp: string;
  updated_at: string;
}

/** domain.SystemBusinessDate (system_date.go) */
export interface SystemBusinessDate {
  current_date: string;
  status: "OPEN" | "IN_EOD_PROCESSING" | "CLOSED";
  updated_by?: string;
  updated_at: string;
}

/**
 * Batas transaksi per peran untuk satu jenis transaksi.
 * Nilai uang dikirim sebagai string berisi angka agar presisi desimal tidak
 * hilang saat melewati JSON. `configured: false` berarti nilai masih bawaan
 * aplikasi dan belum pernah ditetapkan bank.
 */
export interface TransactionLimitRow {
  role: string;
  transaction_type: string;
  per_transaction: string;
  daily_limit: string;
  approval_above: string;
  configured: boolean;
}

/** Respons GET /system/limits. `source` menyebut asal nilai (mis. "config"). */
export interface TransactionLimitsResponse {
  source: string;
  limits: TransactionLimitRow[];
}

/** domain.TrialBalanceRow (report.go) */
export interface TrialBalanceRow {
  account_code: string;
  account_name: string;
  book: string;
  normal_balance: "DEBIT" | "CREDIT";
  total_debit: string;
  total_credit: string;
  opening_balance: string;
  closing_balance: string;
}

/** domain.ReportRow (report.go) */
export interface ReportRow {
  account_code: string;
  account_name: string;
  book: string;
  amount: string;
}

/** domain.IncomeStatement (report.go) */
export interface IncomeStatement {
  total_revenue: string;
  total_expense: string;
  net_income: string;
  rows: ReportRow[];
}

/** domain.BalanceSheet (report.go) */
export interface BalanceSheet {
  total_assets: string;
  total_liabilities: string;
  total_equity: string;
  net_income: string;
  rows: ReportRow[];
}

/** domain.CashFlow (report.go) */
export interface CashFlow {
  operating: string;
  investing: string;
  financing: string;
  net_change: string;
  rows: ReportRow[];
}

export type ReportKind =
  "trial-balance" | "balance-sheet" | "income-statement" | "cash-flow";

/**
 * Peninjauan pemetaan COA ke pos laporan OJK
 * (apps/api/internal/ojkreport/mapping_review.go). Field disalin dari struct Go.
 */
export type OJKMappingDecision = "DISETUJUI" | "DICATAT";

export interface OJKMappingReviewRow {
  form: string;
  coa_code: string;
  coa_name: string;
  coa_found: boolean;
  sandi: string;
  pos_name: string;
  sign: number;
  draft_verified: boolean;
  draft_note: string;
  decision?: OJKMappingDecision;
  review_note?: string;
  decided_by?: string;
  decided_at?: string;
}

export interface OJKUnmappedCOA {
  form: string;
  coa_code: string;
  coa_name: string;
}

export interface OJKUnmappedPosition {
  form: string;
  sandi: string;
  pos_name: string;
}

export interface OJKMappingReview {
  mapping_status: string;
  period: string;
  book: string;
  rows: OJKMappingReviewRow[];
  unmapped_coa: OJKUnmappedCOA[];
  unmapped_positions: OJKUnmappedPosition[];
  duplicate_coa: string[];
  source_unbalanced: boolean;
}

/** domain.EODSummaryResult (system_date.go) */
export interface EODSummaryResult {
  executed_date: string;
  next_business_date: string;
  total_posted_journals_today: number;
  total_deposit_amount_today: string;
  /** Penempatan deposito berjangka (DEPOSIT_PLACEMENT), terpisah dari setoran tunai teller. */
  total_deposit_placements_today: number;
  total_deposit_placement_amount_today: string;
  total_withdrawal_amount_today: string;
  deposits_rolled_over: number;
  ppap_processed: number;
  /** Jumlah kredit yang PPAP-nya benar-benar berubah dan menulis jurnal penyesuaian. */
  ppap_adjusted: number;
  loan_penalties_accrued: number;
  loan_penalty_amount: string;
  /** Angsuran jatuh tempo yang bunganya diakru pada tutup hari (kredit konvensional). */
  loan_interest_accrued: number;
  loan_interest_accrued_amount: string;
  accounts_marked_dormant: number;
  /**
   * Bidang ADITIF mode bayangan CKPN (domain.EODSummaryResult, system_date.go).
   * Terisi hanya saat `ckpn.shadow_mode.enabled` menyala dan `ckpn.enabled` masih mati.
   * Angka ini BUKAN kewajiban akuntansi: tidak ada jurnal yang ditulis dan pengurangan
   * modal inti BELUM dilakukan. `ckpn_shadow_note` juga memuat daftar kekurangan
   * parameter (parameter_gaps pada domain.CKPNComparisonSummary) bila ada.
   */
  ckpn_shadow_mode: boolean;
  ckpn_shadow_processed: number;
  ckpn_shadow_failed: number;
  ckpn_shadow_total_ppka: string;
  ckpn_shadow_total_ckpn: string;
  ckpn_shadow_difference: string;
  /** "PPKA", "CKPN", atau "SAMA": pihak yang nilainya lebih tinggi. */
  ckpn_shadow_higher: string;
  ckpn_shadow_assumptions?: string[];
  ckpn_shadow_note?: string;
  /** Pekerjaan harian best-effort yang gagal/tidak lengkap; teks dari server. */
  warnings?: string[];
  executed_by: string;
  completed_at: string;
}

/** domain.EOMSummaryResult (system_date.go) */
export interface EOMSummaryResult {
  executed_month: string;
  total_admin_fees_deducted: string;
  total_interest_paid: string;
  processed_accounts: number;
  failed_accounts: number;
  completed_at: string;
}

/** domain.EOYBookResult (year_end.go) */
export interface EOYBookResult {
  book: "CONVENTIONAL" | "SYARIAH";
  total_revenue_closed: string;
  total_expense_closed: string;
  net_retained_earnings: string;
  retained_earnings_coa_code: string;
  closing_journal_ref: string;
  already_closed: boolean;
}

/** domain.EOYSummaryResult (system_date.go) */
export interface EOYSummaryResult {
  fiscal_year: number;
  total_revenue_closed: string;
  total_expense_closed: string;
  net_retained_earnings: string;
  closing_journal_ref: string;
  books?: EOYBookResult[];
  completed_at: string;
}

/**
 * Shim kompatibilitas: sebagian modul operasional mengimpor tipe ini dari
 * "@/lib/types", padahal definisinya ada di "@/lib/operations-types".
 * Re-export agar build tetap jalan tanpa mengubah modul tersebut.
 */
export type {
  BankingProduct,
  COABook,
  Loan,
  LoanSchedule,
  OJKCollectibility,
  ProfitType,
} from "./operations-types";
