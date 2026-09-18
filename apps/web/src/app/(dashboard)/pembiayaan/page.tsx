"use client";

import { PageHeader } from "@/components/ui/PageHeader";
import { LoanWorkspace } from "@/components/loan/LoanWorkspace";

export default function PembiayaanPage() {
  return (
    <>
      <PageHeader
        title="Pembiayaan"
        description="Pembiayaan syariah: murabahah, mudharabah, dan musyarakah dengan margin atau bagi hasil."
      />
      <LoanWorkspace book="SYARIAH" />
    </>
  );
}
