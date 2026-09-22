import React from "react";
import type { ProfitType } from "@/lib/operations-types";

/**
 * Label tampilan bersama lintas halaman; teksnya tetap dari kamus i18n.
 * Disatukan agar satu makna tidak punya dua salinan yang bisa menyimpang.
 */

/** Nama nasabah dari peta id->nama; bila tak ada, id ditampilkan sebagai kode. */
export function customerLabel(
  customerId: string,
  names: Record<string, string>,
): React.ReactNode {
  const name = names[customerId];
  if (name) return name;
  return <span className="font-mono">{customerId}</span>;
}

/**
 * Skema imbal hasil disimpan sebagai kunci teknis (MARGIN dll.); hanya label
 * tampilannya yang diambil dari kamus, jadi pemanggil yang menentukan bagian
 * kamus mana (simpanan atau kredit).
 */
export function profitTypeLabel(
  labels: {
    profitMargin: string;
    profitBagiHasil: string;
    profitInterest: string;
  },
  type: ProfitType,
): string {
  if (type === "MARGIN") return labels.profitMargin;
  if (type === "BAGI_HASIL") return labels.profitBagiHasil;
  return labels.profitInterest;
}
