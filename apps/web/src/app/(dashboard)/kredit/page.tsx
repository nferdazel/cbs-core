"use client";

import { FeatureUnavailable } from "@/components/layout/FeatureUnavailable";

export default function KreditPage() {
  return (
    <FeatureUnavailable
      title="Kredit"
      description="Pengajuan, analisa, akad, pencairan, angsuran, dan pelunasan kredit konvensional."
      reason="Endpoint kredit sudah ada, namun form origination belum aman dipasang tanpa validasi produk dan jadwal angsuran yang lengkap."
    />
  );
}
