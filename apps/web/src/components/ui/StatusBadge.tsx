import React from "react";
import { Badge, BadgeProps } from "./Badge";
import { useTranslation } from "@/i18n/context";

export type StatusTone = NonNullable<BadgeProps["variant"]>;

/**
 * Domain makna status. Nama status yang sama bisa bermakna berbeda antar domain
 * (mis. CLOSED pada rekening = berisiko, pada deposito = selesai normal), jadi
 * setiap pemanggil wajib menyatakan domainnya sendiri alih-alih menebak dari teks.
 */
export type StatusDomain =
  | "approval"
  | "journal"
  | "account"
  | "customer"
  | "deposit"
  | "loan"
  | "installment"
  | "businessDate";

/**
 * Pemetaan status ke makna sempit per domain (DESIGN.md bagian "StatusBadge").
 * Warna hanya penanda; teks selalu ditampilkan. Status yang tidak dikenal pada
 * domain yang dinyatakan sengaja ditampilkan netral, bukan dipinjam dari domain lain.
 */
const DOMAIN_TONE: Record<StatusDomain, Record<string, StatusTone>> = {
  // MakerCheckerStatus / PendingApprovalResult
  approval: {
    PENDING: "accent",
    APPROVED: "credit",
    REJECTED: "debit",
  },
  // JournalStatus
  journal: {
    POSTED: "credit",
    REVERSED: "debit",
    FAILED: "debit",
    PENDING_APPROVAL: "accent",
  },
  // AccountStatus: rekening ditutup = terminal, perlu perhatian.
  account: {
    ACTIVE: "credit",
    DORMANT: "accent",
    FROZEN: "debit",
    CLOSED: "debit",
  },
  // CustomerStatus
  customer: {
    PENDING_KYC: "accent",
    ACTIVE: "credit",
    BLOCKED: "debit",
    CLOSED: "debit",
  },
  // DepositStatus: deposito ditutup = pencairan normal.
  deposit: {
    PLACED: "credit",
    MATURED: "accent",
    CLOSED: "neutral",
    BROKEN: "debit",
  },
  // LoanStatus
  loan: {
    PENDING_APPROVAL: "accent",
    APPROVED: "credit",
    DISBURSED: "credit",
    REJECTED: "debit",
    PAID_OFF: "credit",
    DEFAULTED: "debit",
    WRITTEN_OFF: "debit",
  },
  // InstallmentStatus
  installment: {
    PENDING: "accent",
    PAID: "credit",
    OVERDUE: "debit",
    PARTIAL: "accent",
  },
  // Status tanggal buku (SystemBusinessDate)
  businessDate: {
    OPEN: "credit",
    IN_EOD_PROCESSING: "accent",
    CLOSED: "neutral",
  },
};

/**
 * Cadangan untuk pemanggil lama yang belum menyatakan domain. Pertahankan agar
 * perilaku lama tidak berubah; pemanggil baru sebaiknya memakai `domain`.
 */
const GENERIC_TONE: Record<string, StatusTone> = {
  POSTED: "credit",
  ACTIVE: "credit",
  APPROVED: "credit",
  DISBURSED: "credit",
  PAID_OFF: "credit",
  PAID: "credit",
  PENDING: "accent",
  PENDING_APPROVAL: "accent",
  PENDING_KYC: "accent",
  DORMANT: "accent",
  IN_EOD_PROCESSING: "accent",
  REJECTED: "debit",
  REVERSED: "debit",
  FAILED: "debit",
  WRITTEN_OFF: "debit",
  BLOCKED: "debit",
  FROZEN: "debit",
  CLOSED: "debit",
};

export interface StatusBadgeProps {
  status: string | null | undefined;
  /** Domain makna status; pemanggil menyatakan sendiri, bukan menebak dari teks. */
  domain?: StatusDomain;
  /** Penimpa eksplisit bila status tidak termasuk domain mana pun. */
  tone?: StatusTone;
}

export const StatusBadge: React.FC<StatusBadgeProps> = ({
  status,
  domain,
  tone,
}) => {
  const { t } = useTranslation();
  if (!status) {
    return <Badge variant="outline">{t.common.unknown}</Badge>;
  }
  const resolved =
    tone ??
    (domain
      ? (DOMAIN_TONE[domain][status] ?? "outline")
      : GENERIC_TONE[status]) ??
    "outline";
  return <Badge variant={resolved}>{status}</Badge>;
};
