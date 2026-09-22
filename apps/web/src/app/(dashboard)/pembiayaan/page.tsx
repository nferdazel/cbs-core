"use client";

import { PageHeader } from "@/components/ui/PageHeader";
import { LoanWorkspace } from "@/components/loan/LoanWorkspace";
import { BookScopeGuard } from "@/components/layout/BookScopeGuard";
import { useTranslation } from "@/i18n/context";

export default function PembiayaanPage() {
  const { t } = useTranslation();

  return (
    <BookScopeGuard
      book="SYARIAH"
      title={t.loans.titleSyariah}
      description={t.loans.descSyariah}
    >
      <PageHeader
        title={t.loans.titleSyariah}
        description={t.loans.descSyariah}
      />
      <LoanWorkspace book="SYARIAH" />
    </BookScopeGuard>
  );
}
