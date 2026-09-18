/**
 * Tipe respons API yang belum ada di @cbs/shared-types.
 * Field disalin dari struct Go di apps/api/internal/domain (jangan menebak).
 */

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
