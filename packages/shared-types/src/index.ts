export type CustomerStatus = 'PENDING_KYC' | 'ACTIVE' | 'BLOCKED' | 'CLOSED';

export interface Customer {
  id: string;
  cif_number: string;
  full_name: string;
  id_card_number: string;
  email: string;
  phone_number: string;
  address: string;
  status: CustomerStatus;
  metadata?: Record<string, any>;
  created_at: string;
  updated_at: string;
}

export type AccountType = 'SAVINGS' | 'CHECKING' | 'LOAN' | 'INTERNAL_GL';
export type AccountStatus = 'ACTIVE' | 'DORMANT' | 'FROZEN' | 'CLOSED';

export interface Account {
  id: string;
  account_number: string;
  customer_id?: string;
  customer_name?: string;
  coa_id: string;
  coa_code?: string;
  account_type: AccountType;
  currency: string;
  balance: string;
  available_balance: string;
  hold_balance: string;
  status: AccountStatus;
  version: number;
  created_at: string;
  updated_at: string;
}

export type TransactionType =
  | 'DEPOSIT'
  | 'WITHDRAWAL'
  | 'TRANSFER_INTERNAL'
  | 'FEE_CHARGE'
  | 'INTEREST_ACCRUAL'
  | 'REVERSAL'
  | 'ADJUSTMENT';

export type JournalStatus = 'POSTED' | 'REVERSED' | 'FAILED' | 'PENDING_APPROVAL';
export type EntryDirection = 'DEBIT' | 'CREDIT';

export interface JournalLine {
  id: string;
  journal_entry_id: string;
  account_id: string;
  account_number?: string;
  direction: EntryDirection;
  amount: string;
  currency: string;
  balance_after: string;
  sequence: number;
  description: string;
  created_at: string;
}

export interface JournalEntry {
  id: string;
  reference_number: string;
  idempotency_key?: string;
  transaction_type: TransactionType;
  description: string;
  status: JournalStatus;
  posted_at: string;
  created_by: string;
  lines?: JournalLine[];
  created_at: string;
}

export interface APIResponse<T = any> {
  success: boolean;
  message?: string;
  data?: T;
  error?: string;
  meta?: {
    page: number;
    page_size: number;
    total_items: number;
    total_pages: number;
  };
}
