"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, request } from "@/lib/api";
import { formatDate, formatDateTime } from "@/lib/format";
import type { SystemBusinessDate } from "@/lib/types";
import { useAuth } from "@/lib/useAuth";
import { useTranslation } from "@/i18n/context";
import { PageHeader } from "@/components/ui/PageHeader";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { DefinitionList } from "@/components/ui/DefinitionList";
import { ErrorState, LoadingState } from "@/components/ui/States";
import { TransactionLimits } from "@/components/settings/TransactionLimits";
import { BankProfileCard } from "@/components/settings/BankProfileCard";

export default function PengaturanPage() {
  const { user } = useAuth();
  const { t } = useTranslation();
  const [businessDate, setBusinessDate] = useState<SystemBusinessDate | null>(
    null
  );
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await request<SystemBusinessDate>(
        "/system/business-date"
      );
      setBusinessDate(response.data ?? null);
    } catch (err) {
      setError(
        err instanceof ApiError
          ? err.message
          : t.settingsPage.loadError
      );
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    load();
  }, [load]);

  return (
    <>
      <PageHeader
        title={t.settingsPage.title}
        description={t.settingsPage.description}
      />

      <Card className="mb-4">
        <CardHeader>
          <CardTitle>{t.settingsPage.profileTitle}</CardTitle>
        </CardHeader>
        <CardContent>
          <DefinitionList
            items={[
              { label: t.common.name, value: user?.full_name || user?.username || "-" },
              { label: t.common.username, value: user?.username || "-", isMono: true },
              { label: t.common.role, value: user?.role || "-" },
              { label: t.common.branch, value: user?.branch_code || "-", isMono: true },
            ]}
          />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t.settingsPage.businessDateTitle}</CardTitle>
        </CardHeader>
        <CardContent>
          {loading ? (
            <LoadingState label={t.settingsPage.loadingBusinessDate} />
          ) : error ? (
            <ErrorState
              title={t.settingsPage.businessDateErrorTitle}
              description={error}
            />
          ) : businessDate ? (
            <DefinitionList
              items={[
                {
                  label: t.settingsPage.currentDate,
                  value: formatDate(businessDate.current_date),
                  isMono: true,
                },
                { label: t.common.status, value: businessDate.status },
                {
                  label: t.settingsPage.lastUpdated,
                  value: formatDateTime(businessDate.updated_at),
                  isMono: true,
                },
                ...(businessDate.updated_by
                  ? [
                      {
                        label: t.settingsPage.updatedBy,
                        value: businessDate.updated_by,
                        isMono: true,
                      },
                    ]
                  : []),
              ]}
            />
          ) : (
            <p className="text-body text-ink-600">
              {t.settingsPage.unavailable}
            </p>
          )}
        </CardContent>
      </Card>

      <TransactionLimits />
      <BankProfileCard />
    </>
  );
}
