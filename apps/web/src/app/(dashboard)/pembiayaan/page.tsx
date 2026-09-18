"use client";

import { FeatureUnavailable } from "@/components/layout/FeatureUnavailable";

export default function PembiayaanPage() {
  return (
    <FeatureUnavailable
      title="Pembiayaan"
      description="Pembiayaan syariah: Murabahah, Mudharabah/Musyarakah, dan Ijarah."
      reason="Skema bagi hasil dan margin belum tersedia lengkap di sisi API; menampilkan data tiruan akan menyesatkan."
    />
  );
}
