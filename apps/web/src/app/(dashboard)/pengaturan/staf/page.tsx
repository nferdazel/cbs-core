"use client";

import { useTranslation } from "@/i18n/context";
import { PageHeader } from "@/components/ui/PageHeader";
import { StaffManagement } from "@/components/settings/StaffManagement";

/**
 * Halaman pengelolaan staf. Penjagaan sebenarnya ada di API
 * (users:read untuk daftar, users:create untuk tambah, users:update untuk
 * ubah/reset sandi); halaman ini menyusun formulir dan menampilkan keadaan
 * apa adanya.
 */
export default function StaffPage() {
  const { t } = useTranslation();

  return (
    <>
      <PageHeader
        title={t.staffPage.title}
        description={t.staffPage.description}
      />
      <StaffManagement />
    </>
  );
}
