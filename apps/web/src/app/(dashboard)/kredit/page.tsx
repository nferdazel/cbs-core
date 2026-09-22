"use client";

import { PageHeader } from "@/components/ui/PageHeader";
import { LoanWorkspace } from "@/components/loan/LoanWorkspace";
import { BookScopeGuard } from "@/components/layout/BookScopeGuard";

export default function KreditPage() {
  return (
    <BookScopeGuard
      book="CONVENTIONAL"
      title="Kredit"
      description="Pengajuan, persetujuan, pencairan, dan angsuran kredit konvensional."
    >
      <PageHeader
        title="Kredit"
        description="Pengajuan, persetujuan, pencairan, dan angsuran kredit konvensional."
      />
      <LoanWorkspace book="CONVENTIONAL" />
    </BookScopeGuard>
  );
}
