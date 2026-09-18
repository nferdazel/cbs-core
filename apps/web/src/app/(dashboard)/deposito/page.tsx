"use client";

import { FeatureUnavailable } from "@/components/layout/FeatureUnavailable";

export default function DepositoPage() {
  return (
    <FeatureUnavailable
      title="Deposito"
      description="Deposito berjangka: buka, rollover, pencairan (break), bunga/margin, dan PPh final."
      reason="Endpoint deposito belum tersedia di API (lihat BACKLOG Fase 3)."
    />
  );
}
