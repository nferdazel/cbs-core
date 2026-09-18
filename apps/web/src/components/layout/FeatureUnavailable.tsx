"use client";

import React from "react";
import { useTranslation } from "@/i18n/context";
import { PageHeader } from "@/components/ui/PageHeader";
import { EmptyState } from "@/components/ui/States";

export interface FeatureUnavailableProps {
  title: string;
  description: string;
  /** Keterangan tambahan tentang API yang belum siap. */
  reason?: string;
}

/** Halaman jujur untuk fitur yang API-nya belum tersedia. Tidak ada kontrol aktif. */
export const FeatureUnavailable: React.FC<FeatureUnavailableProps> = ({
  title,
  description,
  reason,
}) => {
  const { t } = useTranslation();
  return (
    <>
      <PageHeader title={title} description={description} />
      <EmptyState
        title={t.common.notAvailable}
        description={reason ?? t.common.notAvailableDesc}
      />
    </>
  );
};
