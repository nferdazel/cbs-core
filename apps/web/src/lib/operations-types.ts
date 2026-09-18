/**
 * Tipe respons operasional yang belum ada di @cbs/shared-types.
 * Field disalin dari struct Go di apps/api/internal/domain (jangan menebak).
 * Dipisah dari lib/types.ts agar tidak bertabrakan dengan pekerjaan auth paralel.
 */

/** domain.ProductFamily (product.go) */
export type ProductFamily =
  | "SAVINGS"
  | "TIME_DEPOSIT"
  | "LOAN"
  | "CURRENT_ACCOUNT";

/** domain.COABook (product.go) */
export type COABook = "CONVENTIONAL" | "SYARIAH";

/** domain.ProfitScheme (product.go) */
export type ProfitScheme =
  | "INTEREST"
  | "MURABAHAH"
  | "MUDHARABAH"
  | "MUSYARAKAH"
  | "IJARAH"
  | "WADIAH";

/** domain.ScheduleMethod (product.go) */
export type ScheduleMethod =
  | "FLAT"
  | "ANNUITY"
  | "SLIDING"
  | "BAGI_HASIL"
  | "NONE";

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
  | "1_LANCAR"
  | "2_DPK"
  | "3_KURANG_LANCAR"
  | "4_DIRAGUKAN"
  | "5_MACET";

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
 * domain.Collectibility (ppap.go): kualitas aset versi numerik, 1 Lancar s.d.
 * 5 Macet. JSON mengirimnya sebagai angka karena tipe dasarnya int.
 */
export type PPAPCollectibility = 1 | 2 | 3 | 4 | 5;

/**
 * domain.PPAPRunItem (ppap.go). Struct Go ini tidak punya json tag, sehingga
 * encoding/json memakai nama field apa adanya (PascalCase). Jangan pakai
 * snake_case di sini.
 */
export interface PPAPRunItem {
  LoanID: string;
  LoanNumber: string;
  DPD: number;
  Collectibility: PPAPCollectibility;
  Outstanding: string;
  Target: string;
  Existing: string;
  Adjustment: string;
  CollectibilityChanged: boolean;
  StopAccrual: boolean;
  Posted: boolean;
}

/** domain.PPAPRunFailure (ppap.go). Tanpa json tag: key PascalCase. */
export interface PPAPRunFailure {
  LoanID: string;
  LoanNumber: string;
  Error: string;
}

/**
 * domain.PPAPRunSummary (ppap.go). Tanpa json tag: key PascalCase. Saat preview,
 * ReserveAfter dibiarkan nol dan Posted selalu false karena tidak ada jurnal.
 */
export interface PPAPRunSummary {
  AsOf: string;
  Total: number;
  Processed: number;
  Failed: number;
  Skipped: number;
  TotalAdjustment: string;
  Items: PPAPRunItem[] | null;
  Failures: PPAPRunFailure[] | null;
  ReserveBefore: string;
  ReserveAfter: string;
  Preview: boolean;
}

/** domain.Branch (branch.go). address & phone omitempty: bisa tidak dikirim. */
export interface Branch {
  id: string;
  code: string;
  name: string;
  address?: string;
  phone?: string;
  is_head_office: boolean;
  is_active: boolean;
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
