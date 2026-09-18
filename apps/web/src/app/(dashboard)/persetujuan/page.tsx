"use client";

import { FeatureUnavailable } from "@/components/layout/FeatureUnavailable";

export default function PersetujuanPage() {
  return (
    <FeatureUnavailable
      title="Persetujuan"
      description="Antrean maker-checker untuk transaksi bernominal besar dan inisiasi kredit."
      reason="Handler maker-checker masih memakai kolom yang tidak ada di schema (temuan A2), sehingga data tidak dapat ditampilkan dengan andal."
    />
  );
}
