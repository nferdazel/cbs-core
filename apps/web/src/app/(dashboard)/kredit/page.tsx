"use client";

import { PageHeader } from "@/components/ui/PageHeader";
import { LoanWorkspace } from "@/components/loan/LoanWorkspace";

export default function KreditPage() {
  return (
    <>
      <PageHeader
        title="Kredit"
        description="Pengajuan, persetujuan, pencairan, dan angsuran kredit konvensional."
      />
      <LoanWorkspace book="CONVENTIONAL" />
    </>
  );
}
