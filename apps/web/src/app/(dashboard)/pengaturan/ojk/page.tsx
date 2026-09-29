"use client";

import { useTranslation } from "@/i18n/context";
import { PageHeader } from "@/components/ui/PageHeader";
import { OJKAsetKeuanganLainnyaCard } from "@/components/settings/OJKAsetKeuanganLainnyaCard";
import { OJKAsetTetapCard } from "@/components/settings/OJKAsetTetapCard";
import { OJKAYDACard } from "@/components/settings/OJKAYDACard";
import { OJKBMPKCard } from "@/components/settings/OJKBMPKCard";
import { OJKKepemilikanCard } from "@/components/settings/OJKKepemilikanCard";
import { OJKKreditSindikasiCard } from "@/components/settings/OJKKreditSindikasiCard";
import { OJKKelembagaanCard } from "@/components/settings/OJKKelembagaanCard";
import { OJKOffBalanceCard } from "@/components/settings/OJKOffBalanceCard";
import { OJKPenyertaanModalCard } from "@/components/settings/OJKPenyertaanModalCard";
import { OJKPinjamanDiterimaCard } from "@/components/settings/OJKPinjamanDiterimaCard";
import { OJKPlacementCodesCard } from "@/components/settings/OJKPlacementCodesCard";
import { OJKPropertiTerbengkalaiCard } from "@/components/settings/OJKPropertiTerbengkalaiCard";
import { OJKProfileCard } from "@/components/settings/OJKProfileCard";
import { OJKReferenceCodesCard } from "@/components/settings/OJKReferenceCodesCard";
import { OJKStrukturOrganisasiCard } from "@/components/settings/OJKStrukturOrganisasiCard";
import { OJKSuratBerhargaCard } from "@/components/settings/OJKSuratBerhargaCard";

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
      <OJKSuratBerhargaCard />
      <OJKAYDACard />
      <OJKKepemilikanCard />
      <OJKPinjamanDiterimaCard />
      <OJKPropertiTerbengkalaiCard />
      <OJKAsetTetapCard />
      <OJKPenyertaanModalCard />
      <OJKAsetKeuanganLainnyaCard />
      <OJKKreditSindikasiCard />
      <OJKStrukturOrganisasiCard />
      <OJKBMPKCard />
    </>
  );
}
