"use client";

import { useTranslation } from "@/i18n/context";
import { PageHeader } from "@/components/ui/PageHeader";
import { OJKAsetTetapCard } from "@/components/settings/OJKAsetTetapCard";
import { OJKAYDACard } from "@/components/settings/OJKAYDACard";
import { OJKBMPKCard } from "@/components/settings/OJKBMPKCard";
import { OJKKepemilikanCard } from "@/components/settings/OJKKepemilikanCard";
import { OJKKelembagaanCard } from "@/components/settings/OJKKelembagaanCard";
import { OJKOffBalanceCard } from "@/components/settings/OJKOffBalanceCard";
import { OJKPenyertaanModalCard } from "@/components/settings/OJKPenyertaanModalCard";
import { OJKPinjamanDiterimaCard } from "@/components/settings/OJKPinjamanDiterimaCard";
import { OJKPlacementCodesCard } from "@/components/settings/OJKPlacementCodesCard";
import { OJKPropertiTerbengkalaiCard } from "@/components/settings/OJKPropertiTerbengkalaiCard";
import { OJKProfileCard } from "@/components/settings/OJKProfileCard";
import { OJKReferenceCodesCard } from "@/components/settings/OJKReferenceCodesCard";

/**
 * Halaman identitas Form 00.00. Penjagaan sebenarnya ada di API
 * (system:config:read untuk baca, system:config untuk ubah); halaman ini hanya
 * menyusun formulir dan menampilkan nilai apa adanya.
 */
export default function OjkIdentityPage() {
  const { t } = useTranslation();

  return (
    <>
      <PageHeader
        title={t.ojkProfile.title}
        description={t.ojkProfile.description}
      />
      <OJKProfileCard />
      <OJKReferenceCodesCard />
      <OJKPlacementCodesCard />
      <OJKKelembagaanCard />
      <OJKOffBalanceCard />
      <OJKAYDACard />
      <OJKKepemilikanCard />
      <OJKPinjamanDiterimaCard />
      <OJKPropertiTerbengkalaiCard />
      <OJKAsetTetapCard />
      <OJKPenyertaanModalCard />
      <OJKBMPKCard />
    </>
  );
}
