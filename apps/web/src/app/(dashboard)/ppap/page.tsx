"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, newIdempotencyKey, request } from "@/lib/api";
import { formatDate } from "@/lib/format";
import type { PPAPRunItem, PPAPRunSummary } from "@/lib/operations-types";
import { PageHeader } from "@/components/ui/PageHeader";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { DataTable, Column } from "@/components/ui/DataTable";
import { MoneyText } from "@/components/ui/MoneyText";
import { DefinitionList } from "@/components/ui/DefinitionList";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { ErrorState, LoadingState } from "@/components/ui/States";

const COLLECTIBILITY_LABEL: Record<number, string> = {
  1: "Lancar",
  2: "Dalam Perhatian Khusus",
  3: "Kurang Lancar",
  4: "Diragukan",
  5: "Macet",
};

function collectibilityLabel(value: number): string {
  return COLLECTIBILITY_LABEL[value] ?? `Golongan ${value}`;
}

function summaryItems(summary: PPAPRunSummary): PPAPRunItem[] {
  return summary.items ?? [];
}

export default function PPAPPage() {
  const [preview, setPreview] = useState<PPAPRunSummary | null>(null);
  const [previewLoading, setPreviewLoading] = useState(true);
  const [previewError, setPreviewError] = useState<string | null>(null);

  const [runSummary, setRunSummary] = useState<PPAPRunSummary | null>(null);
  const [runError, setRunError] = useState<string | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [running, setRunning] = useState(false);

  const loadPreview = useCallback(async () => {
    setPreviewLoading(true);
    setPreviewError(null);
    try {
      const response = await request<PPAPRunSummary>("/ppap/preview");
      setPreview(response.data ?? null);
    } catch (err) {
      setPreviewError(
        err instanceof ApiError ? err.message : "Gagal memuat pratinjau PPAP."
      );
    } finally {
      setPreviewLoading(false);
    }
  }, []);

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
          : "Perhitungan PPAP harian gagal dijalankan."
      );
      setConfirmOpen(false);
    } finally {
      setRunning(false);
    }
  };

  const itemColumns: Column<PPAPRunItem>[] = [
    {
      header: "Nomor Kredit",
      cell: (row) => <span className="font-mono">{row.loan_number}</span>,
    },
    {
      header: "Kolektibilitas",
      cell: (row) => collectibilityLabel(row.collectibility),
    },
    {
      header: "DPD",
      align: "right",
      isMono: true,
      cell: (row) => `${row.dpd} hari`,
    },
    {
      header: "Pokok",
      type: "money",
      cell: (row) => <MoneyText value={row.outstanding} />,
    },
    {
      header: "Cadangan Dibutuhkan",
      type: "money",
      cell: (row) => <MoneyText value={row.target} />,
    },
    {
      header: "Penyesuaian",
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
        title="PPAP"
        description="Penyisihan Penghapusan Aktiva Produktif: kolektibilitas dan cadangan kerugian kredit."
        actions={
          <>
            <Button
              variant="secondary"
              onClick={loadPreview}
              loading={previewLoading}
            >
              Pratinjau
            </Button>
            <Button onClick={() => setConfirmOpen(true)}>
              Jalankan PPAP Harian
            </Button>
          </>
        }
      />

      <div className="mb-4 rounded-md border border-border bg-canvas px-4 py-3">
        <p className="text-body text-ink-600">
          PPAP normalnya dijalankan otomatis oleh proses EOD. Tombol “Jalankan
          PPAP Harian” hanya untuk menjalankan ulang perhitungan secara manual
          di luar jadwal EOD; aksi ini memposting jurnal penyesuaian cadangan.
        </p>
      </div>

      {runError && (
        <div
          className="mb-4 rounded-md border border-debit-700/30 bg-debit-50 px-4 py-3"
          role="alert"
        >
          <p className="text-body text-debit-700">{runError}</p>
        </div>
      )}

      {runSummary && (
        <Card className="mb-4">
          <CardHeader>
            <CardTitle>
              Hasil Perhitungan{" "}
              <span className="font-mono">{formatDate(runSummary.as_of)}</span>
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-body text-credit-700">
              Perhitungan PPAP harian selesai.
            </p>
            <DefinitionList
              items={[
                { label: "Total Kredit", value: runSummary.total, isMono: true },
                { label: "Berhasil", value: runSummary.processed, isMono: true },
                { label: "Gagal", value: runSummary.failed, isMono: true },
                { label: "Tanpa Perubahan", value: runSummary.skipped, isMono: true },
                {
                  label: "Total Penyesuaian",
                  value: <MoneyText value={runSummary.total_adjustment} />,
                },
                {
                  label: "Cadangan Sebelum",
                  value: <MoneyText value={runSummary.reserve_before} />,
                },
                {
                  label: "Cadangan Sesudah",
                  value: <MoneyText value={runSummary.reserve_after} />,
                },
              ]}
            />
            {runSummary.failures && runSummary.failures.length > 0 && (
              <div className="rounded-md border border-debit-700/30 bg-debit-50 px-4 py-3">
                <p className="text-title font-medium text-debit-700">
                  Kredit gagal diproses
                </p>
                <ul className="mt-2 space-y-1">
                  {runSummary.failures.map((failure) => (
                    <li key={failure.loan_id} className="text-body text-ink-900">
                      <span className="font-mono">{failure.loan_number}</span>:{" "}
                      {failure.error}
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle>
            Pratinjau Perhitungan
            {preview && (
              <span className="font-normal text-ink-600">
                {" "}
                — {formatDate(preview.as_of)}
              </span>
            )}
          </CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          {previewLoading ? (
            <div className="p-4">
              <LoadingState label="Menghitung pratinjau PPAP..." />
            </div>
          ) : previewError ? (
            <div className="p-4">
              <ErrorState
                title="Gagal memuat pratinjau PPAP"
                description={previewError}
                action={
                  <Button variant="secondary" onClick={loadPreview}>
                    Coba lagi
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
                      label: "Kredit Diproses",
                      value: `${preview.processed} dari ${preview.total}`,
                      isMono: true,
                    },
                    { label: "Gagal", value: preview.failed, isMono: true },
                    { label: "Tanpa Perubahan", value: preview.skipped, isMono: true },
                    {
                      label: "Total Penyesuaian",
                      value: <MoneyText value={preview.total_adjustment} />,
                    },
                    {
                      label: "Cadangan Saat Ini (GL)",
                      value: <MoneyText value={preview.reserve_before} />,
                    },
                  ]}
                />
              </div>
              <DataTable
                columns={itemColumns}
                data={summaryItems(preview)}
                keyExtractor={(row) => row.loan_id}
                emptyMessage="Tidak ada kredit aktif yang dihitung pada tanggal ini."
                zebra
              />
            </>
          ) : (
            <div className="p-4">
              <ErrorState
                title="Pratinjau kosong"
                description="API tidak mengembalikan data pratinjau."
                action={
                  <Button variant="secondary" onClick={loadPreview}>
                    Coba lagi
                  </Button>
                }
              />
            </div>
          )}
        </CardContent>
      </Card>

      <ConfirmDialog
        open={confirmOpen}
        title="Jalankan PPAP Harian"
        loading={running}
        confirmLabel="Jalankan dan Posting"
        onCancel={() => setConfirmOpen(false)}
        onConfirm={runDaily}
        description={
          <>
            <p>
              Perhitungan akan memperbarui kolektibilitas kredit aktif dan
              memposting jurnal penyesuaian cadangan PPAP. Aksi ini tidak dapat
              dibatalkan.
            </p>
            {preview ? (
              <dl className="space-y-1">
                <div className="flex justify-between">
                  <dt className="text-ink-600">Tanggal acuan</dt>
                  <dd className="font-mono">{formatDate(preview.as_of)}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-ink-600">Kredit diproses</dt>
                  <dd className="font-mono">
                    {preview.processed} dari {preview.total}
                  </dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-ink-600">Total penyesuaian</dt>
                  <dd>
                    <MoneyText value={preview.total_adjustment} />
                  </dd>
                </div>
              </dl>
            ) : (
              <p className="text-accent-600">
                Nominal belum tersedia. Jalankan pratinjau terlebih dahulu untuk
                melihat estimasi penyesuaian sebelum memposting.
              </p>
            )}
          </>
        }
      />
    </>
  );
}
