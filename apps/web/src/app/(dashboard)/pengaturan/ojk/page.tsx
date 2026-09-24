"use client";

import { useTranslation } from "@/i18n/context";
import { PageHeader } from "@/components/ui/PageHeader";
import { OJKProfileCard } from "@/components/settings/OJKProfileCard";

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
    </>
  );
}
