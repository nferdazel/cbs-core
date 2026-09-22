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
export function isPendingApproval(value: unknown): value is PendingApprovalResult {
  return (
    typeof value === "object" &&
    value !== null &&
    "request_id" in value &&
    "status" in value
  );
}

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
