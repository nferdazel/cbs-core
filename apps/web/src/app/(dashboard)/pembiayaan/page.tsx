"use client";

import { PageHeader } from "@/components/ui/PageHeader";
import { LoanWorkspace } from "@/components/loan/LoanWorkspace";
import { BookScopeGuard } from "@/components/layout/BookScopeGuard";

export default function PembiayaanPage() {
  return (
    <BookScopeGuard
      book="SYARIAH"
      title="Pembiayaan"
      description="Pembiayaan syariah: murabahah, mudharabah, dan musyarakah dengan margin atau bagi hasil."
    >
      <PageHeader
        title="Pembiayaan"
        description="Pembiayaan syariah: murabahah, mudharabah, dan musyarakah dengan margin atau bagi hasil."
      />
      <LoanWorkspace book="SYARIAH" />
    </BookScopeGuard>
  );
}
