"use client";

import { PageHeader } from "@/components/ui/PageHeader";
import { LoanWorkspace } from "@/components/loan/LoanWorkspace";
import { BookScopeGuard } from "@/components/layout/BookScopeGuard";
import { useTranslation } from "@/i18n/context";

export default function KreditPage() {
  const { t } = useTranslation();

  return (
    <BookScopeGuard
      book="CONVENTIONAL"
      title={t.loans.titleConventional}
      description={t.loans.descConventional}
    >
      <PageHeader
        title={t.loans.titleConventional}
        description={t.loans.descConventional}
      />
      <LoanWorkspace book="CONVENTIONAL" />
    </BookScopeGuard>
  );
}
