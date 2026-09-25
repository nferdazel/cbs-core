"use client";

import { useTranslation } from "@/i18n/context";
import { PageHeader } from "@/components/ui/PageHeader";
import { CKPNActivationCard } from "@/components/settings/CKPNActivationCard";

/**
 * Halaman pengaturan aktivasi CKPN. Penjagaan sebenarnya ada di API
 * (system:config:read untuk baca, system:config untuk ubah); halaman ini hanya
 * menyusun formulir dan menampilkan status apa adanya.
 */
export default function CkpnActivationPage() {
  const { t } = useTranslation();

  return (
    <>
      <PageHeader
        title={t.ckpnActivation.title}
        description={t.ckpnActivation.description}
      />
      <CKPNActivationCard />
    </>
  );
}
