"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { ApiError, request } from "@/lib/api";
import type { OJKDefinitions, OJKMonthlyForm } from "@/lib/types";
import { useTranslation } from "@/i18n/context";
import { Alert } from "@/components/ui/Alert";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { DataTable, Column } from "@/components/ui/DataTable";
import { ErrorState, LoadingState } from "@/components/ui/States";

/**
 * Daftar kelengkapan form Laporan Bulanan BPR (kanal APOLO) dari
 * GET /reports/ojk/definitions. Form yang belum dapat dibangun tetap ditampilkan
 * beserta alasannya, supaya cakupan terlihat dan bukan disembunyikan. Baris
 * diurutkan: yang terbit lebih dulu, lalu menurut kode form.
 */
export function OJKMonthlyFormsCard() {
  const { t } = useTranslation();

  const [data, setData] = useState<OJKDefinitions | null>(null);
  const [loading, setLoading] = useState(true);
  const [forbidden, setForbidden] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setForbidden(false);
    setLoadError(null);
    try {
      const response = await request<OJKDefinitions>(
        "/reports/ojk/definitions",
      );
      setData(response.data ?? null);
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setForbidden(true);
      } else {
        setLoadError(
          err instanceof ApiError ? err.message : t.reports.ojkFormsLoadError,
        );
      }
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    load();
  }, [load]);

  const forms = useMemo(() => {
    const list = data?.monthly_forms ?? [];
    return [...list].sort((a, b) => {
      if (a.buildable !== b.buildable) return a.buildable ? -1 : 1;
      return a.form.localeCompare(b.form);
    });
  }, [data]);

  const published = forms.filter((form) => form.buildable).length;
  const unpublished = forms.length - published;

  const columns: Column<OJKMonthlyForm>[] = [
    {
      header: t.reports.ojkFormsColForm,
      accessorKey: "form",
      isMono: true,
      width: "120px",
    },
    { header: t.reports.ojkFormsColName, accessorKey: "name" },
    {
      header: t.reports.ojkFormsColStatus,
      width: "140px",
      cell: (row) => (
        <Badge variant={row.buildable ? "credit" : "outline"}>
          {row.buildable
            ? t.reports.ojkFormsStatusPublished
            : t.reports.ojkFormsStatusUnpublished}
        </Badge>
      ),
    },
    {
      header: t.reports.ojkFormsColReason,
      cell: (row) => row.unavailable_reason || "-",
    },
  ];

  return (
    <Card className="mt-4">
      <CardHeader>
        <CardTitle>{t.reports.ojkFormsTitle}</CardTitle>
      </CardHeader>

      {loading ? (
        <CardContent>
          <LoadingState label={t.reports.ojkFormsLoading} />
        </CardContent>
      ) : forbidden ? (
        <CardContent>
          <ErrorState
            title={t.reports.ojkFormsTitle}
            description={t.reports.ojkFormsForbidden}
          />
        </CardContent>
      ) : loadError ? (
        <CardContent>
          <ErrorState
            title={t.reports.ojkFormsTitle}
            description={loadError}
            action={
              <Button variant="secondary" onClick={load}>
                {t.common.retry}
              </Button>
            }
          />
        </CardContent>
      ) : (
        <>
          <CardContent>
            <p className="text-body text-ink-600">
              {t.reports.ojkFormsDescription}
            </p>
            <div className="mt-3 flex flex-wrap items-center gap-3">
              <Badge variant="credit">
                {t.reports.ojkFormsStatusPublished}: {published}
              </Badge>
              <Badge variant="outline">
                {t.reports.ojkFormsStatusUnpublished}: {unpublished}
              </Badge>
            </div>
            <Alert variant="info" className="mt-3">
              {t.reports.ojkFormsNotBankFault}
            </Alert>
          </CardContent>
          <CardContent className="p-0">
            <DataTable
              columns={columns}
              data={forms}
              keyExtractor={(row) => row.form}
              emptyMessage={t.reports.ojkFormsEmpty}
              zebra
            />
          </CardContent>
        </>
      )}
    </Card>
  );
}
