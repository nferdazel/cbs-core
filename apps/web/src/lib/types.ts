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

/** domain.SystemBusinessDate (system_date.go) */
export interface SystemBusinessDate {
  current_date: string;
  status: "OPEN" | "IN_EOD_PROCESSING" | "CLOSED";
  updated_by?: string;
  updated_at: string;
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
  | "trial-balance"
  | "balance-sheet"
  | "income-statement"
  | "cash-flow";

/** domain.EODSummaryResult (system_date.go) */
export interface EODSummaryResult {
  executed_date: string;
  next_business_date: string;
  total_posted_journals_today: number;
  total_deposit_amount_today: string;
  total_withdrawal_amount_today: string;
  deposits_rolled_over: number;
  ppap_processed: number;
  loan_penalties_accrued: number;
  loan_penalty_amount: string;
  accounts_marked_dormant: number;
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
