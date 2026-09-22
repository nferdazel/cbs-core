"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, newIdempotencyKey, request } from "@/lib/api";
import { formatDate } from "@/lib/format";
import type { PPAPRunItem, PPAPRunSummary } from "@/lib/operations-types";
import { useTranslation } from "@/i18n/context";
import { Alert } from "@/components/ui/Alert";
import { PageHeader } from "@/components/ui/PageHeader";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { DataTable, Column } from "@/components/ui/DataTable";
import { MoneyText } from "@/components/ui/MoneyText";
import { DefinitionList } from "@/components/ui/DefinitionList";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { ErrorState, LoadingState } from "@/components/ui/States";

function summaryItems(summary: PPAPRunSummary): PPAPRunItem[] {
  return summary.items ?? [];
}

export default function PPAPPage() {
  const { t } = useTranslation();
  const [preview, setPreview] = useState<PPAPRunSummary | null>(null);
  const [previewLoading, setPreviewLoading] = useState(true);
  const [previewError, setPreviewError] = useState<string | null>(null);

  const [runSummary, setRunSummary] = useState<PPAPRunSummary | null>(null);
  const [runError, setRunError] = useState<string | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [running, setRunning] = useState(false);

  // Kolektibilitas disimpan sebagai angka 1-5; hanya labelnya dari kamus.
  const collectibilityLabel = (value: number): string => {
    switch (value) {
      case 1:
        return t.ppap.collectibility1;
      case 2:
        return t.ppap.collectibility2;
      case 3:
        return t.ppap.collectibility3;
      case 4:
        return t.ppap.collectibility4;
      case 5:
        return t.ppap.collectibility5;
      default:
        return `${t.ppap.collectibilityOtherPrefix}${value}`;
    }
  };

  const loadPreview = useCallback(async () => {
    setPreviewLoading(true);
    setPreviewError(null);
    try {
      const response = await request<PPAPRunSummary>("/ppap/preview");
      setPreview(response.data ?? null);
    } catch (err) {
      setPreviewError(
        err instanceof ApiError ? err.message : t.ppap.previewLoadError
      );
    } finally {
      setPreviewLoading(false);
    }
  }, [t]);

  useEffect(() => {
    loadPreview();
  }, [loadPreview]);

  const runDaily = async () => {
    setRunning(true);
    setRunError(null);
    try {
      const response = await request<PPAPRunSummary>("/ppap/run-daily", {
        method: "POST",
        idempotencyKey: newIdempotencyKey(),
      });
      setRunSummary(response.data ?? null);
      setConfirmOpen(false);
      // Perhitungan mengubah kolektibilitas dan cadangan, jadi pratinjau dimuat ulang.
      await loadPreview();
    } catch (err) {
      setRunError(
        err instanceof ApiError
          ? err.message
          : t.ppap.runError
      );
      setConfirmOpen(false);
    } finally {
      setRunning(false);
    }
  };

  const itemColumns: Column<PPAPRunItem>[] = [
    {
      header: t.ppap.colLoanNumber,
      cell: (row) => <span className="font-mono">{row.loan_number}</span>,
    },
    {
      header: t.ppap.colCollectibility,
      cell: (row) => collectibilityLabel(row.collectibility),
    },
    {
      header: t.ppap.colDpd,
      align: "right",
      isMono: true,
      cell: (row) => `${row.dpd} ${t.ppap.dpdSuffix}`,
    },
    {
      header: t.ppap.colPrincipal,
      type: "money",
      cell: (row) => <MoneyText value={row.outstanding} />,
    },
    {
      header: t.ppap.colReserveRequired,
      type: "money",
      cell: (row) => <MoneyText value={row.target} />,
    },
    {
      header: t.ppap.colAdjustment,
      type: "money",
      cell: (row) => (
        <MoneyText
          value={row.adjustment}
          tone={Number(row.adjustment) < 0 ? "credit" : "default"}
        />
      ),
    },
  ];

  return (
    <>
      <PageHeader
        title={t.ppap.title}
        description={t.ppap.description}
        actions={
          <>
            <Button
              variant="secondary"
              onClick={loadPreview}
              loading={previewLoading}
            >
              {t.ppap.previewButton}
            </Button>
            <Button onClick={() => setConfirmOpen(true)}>
              {t.ppap.runButton}
            </Button>
          </>
        }
      />

      <Alert variant="info" className="mb-4">
        {t.ppap.notice}
      </Alert>

      {runError && (
        <Alert variant="error" className="mb-4">
          {runError}
        </Alert>
      )}

      {runSummary && (
        <Card className="mb-4">
          <CardHeader>
            <CardTitle>
              {t.ppap.runResultTitle}{" "}
              <span className="font-mono">{formatDate(runSummary.as_of)}</span>
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-body text-credit-700">
              {t.ppap.runSuccess}
            </p>
            <DefinitionList
              items={[
                { label: t.ppap.totalLoans, value: runSummary.total, isMono: true },
                { label: t.ppap.processedLabel, value: runSummary.processed, isMono: true },
                { label: t.ppap.failedLabel, value: runSummary.failed, isMono: true },
                { label: t.ppap.skippedLabel, value: runSummary.skipped, isMono: true },
                {
                  label: t.ppap.totalAdjustment,
                  value: <MoneyText value={runSummary.total_adjustment} />,
                },
                {
                  label: t.ppap.reserveBefore,
                  value: <MoneyText value={runSummary.reserve_before} />,
                },
                {
                  label: t.ppap.reserveAfter,
                  value: <MoneyText value={runSummary.reserve_after} />,
                },
              ]}
            />
            {runSummary.failures && runSummary.failures.length > 0 && (
              <Alert variant="error" title={t.ppap.failuresTitle}>
                <ul className="space-y-1">
                  {runSummary.failures.map((failure) => (
                    <li key={failure.loan_id} className="text-ink-900">
                      <span className="font-mono">{failure.loan_number}</span>:{" "}
                      {failure.error}
                    </li>
                  ))}
                </ul>
              </Alert>
            )}
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle>
            {t.ppap.previewTitle}
            {preview && (
              <span className="font-normal text-ink-600">
                {" "}
                {formatDate(preview.as_of)}
              </span>
            )}
          </CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          {previewLoading ? (
            <div className="p-4">
              <LoadingState label={t.ppap.previewLoading} />
            </div>
          ) : previewError ? (
            <div className="p-4">
              <ErrorState
                title={t.ppap.previewErrorTitle}
                description={previewError}
                action={
                  <Button variant="secondary" onClick={loadPreview}>
                    {t.common.retry}
                  </Button>
                }
              />
            </div>
          ) : preview ? (
            <>
              <div className="border-b border-border p-4">
                <DefinitionList
                  items={[
                    {
                      label: t.ppap.colProcessedLoans,
                      value: `${preview.processed} ${t.common.of} ${preview.total}`,
                      isMono: true,
                    },
                    { label: t.ppap.failedLabel, value: preview.failed, isMono: true },
                    { label: t.ppap.skippedLabel, value: preview.skipped, isMono: true },
                    {
                      label: t.ppap.totalAdjustment,
                      value: <MoneyText value={preview.total_adjustment} />,
                    },
                    {
                      label: t.ppap.reserveCurrent,
                      value: <MoneyText value={preview.reserve_before} />,
                    },
                  ]}
                />
              </div>
              <DataTable
                columns={itemColumns}
                data={summaryItems(preview)}
                keyExtractor={(row) => row.loan_id}
                emptyMessage={t.ppap.emptyItems}
                zebra
              />
            </>
          ) : (
            <div className="p-4">
              <ErrorState
                title={t.ppap.previewEmptyTitle}
                description={t.ppap.previewEmptyDesc}
                action={
                  <Button variant="secondary" onClick={loadPreview}>
                    {t.common.retry}
                  </Button>
                }
              />
            </div>
          )}
        </CardContent>
      </Card>

      <ConfirmDialog
        open={confirmOpen}
        title={t.ppap.confirmTitle}
        loading={running}
        confirmLabel={t.ppap.confirmButton}
        onCancel={() => setConfirmOpen(false)}
        onConfirm={runDaily}
        description={
          <>
            <p>
              {t.ppap.confirmDesc}
            </p>
            {preview ? (
              <dl className="space-y-1">
                <div className="flex justify-between">
                  <dt className="text-ink-600">{t.ppap.refDate}</dt>
                  <dd className="font-mono">{formatDate(preview.as_of)}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-ink-600">{t.ppap.processedLoansShort}</dt>
                  <dd className="font-mono">
                    {preview.processed} {t.common.of} {preview.total}
                  </dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-ink-600">{t.ppap.totalAdjustmentShort}</dt>
                  <dd>
                    <MoneyText value={preview.total_adjustment} />
                  </dd>
                </div>
              </dl>
            ) : (
              <p className="text-accent-600">
                {t.ppap.amountsUnavailable}
              </p>
            )}
          </>
        }
      />
    </>
  );
}
