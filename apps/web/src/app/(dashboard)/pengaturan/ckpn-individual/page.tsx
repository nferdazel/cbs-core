"use client";

import { useTranslation } from "@/i18n/context";
import { PageHeader } from "@/components/ui/PageHeader";
import { CKPNIndividualPanel } from "@/components/settings/CKPNIndividualPanel";

/**
 * Halaman pintu masuk CKPN individual. Penjagaan ada di API: pemindaian butuh
 * loans:read dan penandaan butuh loans:approve. Halaman hanya menampilkan usulan
 * pemindaian dan mencatat keputusan pengelola.
 */
export default function CKPNIndividualPage() {
  const { t } = useTranslation();

  return (
    <>
      <PageHeader
        title={t.ckpnIndividual.title}
        description={t.ckpnIndividual.description}
      />
      <CKPNIndividualPanel />
    </>
  );
}
