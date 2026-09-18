import React from "react";
import { Badge, BadgeProps } from "./Badge";

type StatusTone = NonNullable<BadgeProps["variant"]>;

/**
 * Pemetaan status ke makna sempit (DESIGN.md bagian "StatusBadge").
 * Warna hanya penanda; teks selalu ditampilkan.
 */
const STATUS_TONE: Record<string, StatusTone> = {
  // Berhasil / final
  POSTED: "credit",
  ACTIVE: "credit",
  APPROVED: "credit",
  DISBURSED: "credit",
  PAID_OFF: "credit",
  PAID: "credit",
  // Butuh perhatian manusia
  PENDING: "accent",
  PENDING_APPROVAL: "accent",
  PENDING_KYC: "accent",
  DORMANT: "accent",
  IN_EOD_PROCESSING: "accent",
  // Ditolak / berisiko
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
}

export const StatusBadge: React.FC<StatusBadgeProps> = ({ status }) => {
  if (!status) {
    return <Badge variant="outline">TIDAK DIKETAHUI</Badge>;
  }
  const tone = STATUS_TONE[status] ?? "outline";
  return <Badge variant={tone}>{status}</Badge>;
};
