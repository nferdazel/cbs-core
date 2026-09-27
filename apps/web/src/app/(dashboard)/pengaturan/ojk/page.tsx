"use client";

import { useTranslation } from "@/i18n/context";
import { PageHeader } from "@/components/ui/PageHeader";
import { OJKBMPKCard } from "@/components/settings/OJKBMPKCard";
import { OJKKelembagaanCard } from "@/components/settings/OJKKelembagaanCard";
import { OJKOffBalanceCard } from "@/components/settings/OJKOffBalanceCard";
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
      <OJKKelembagaanCard />
      <OJKOffBalanceCard />
      <OJKBMPKCard />
    </>
  );
}
