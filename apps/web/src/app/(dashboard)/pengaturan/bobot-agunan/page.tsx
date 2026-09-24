"use client";

import { useTranslation } from "@/i18n/context";
import { PageHeader } from "@/components/ui/PageHeader";
import { CollateralWeightGate } from "@/components/settings/CollateralWeightGate";

/**
 * Halaman gerbang bobot agunan. Penjagaan ada di API
 * (system:config:read untuk menilai, system:config untuk mengajukan aktivasi);
 * halaman ini hanya menampilkan penilaian dan mengajukan lewat maker-checker.
 */
export default function CollateralWeightPage() {
  const { t } = useTranslation();

  return (
    <>
      <PageHeader
        title={t.collateralWeight.title}
        description={t.collateralWeight.description}
      />
      <CollateralWeightGate />
    </>
  );
}
