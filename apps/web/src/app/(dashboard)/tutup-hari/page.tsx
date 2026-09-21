"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, newIdempotencyKey, request } from "@/lib/api";
import { formatDate, formatDateTime } from "@/lib/format";
import type {
  EODSummaryResult,
  EOMSummaryResult,
  EOYBookResult,
  EOYSummaryResult,
  SystemBusinessDate,
} from "@/lib/types";
import { useAuth } from "@/lib/useAuth";
import { PageHeader } from "@/components/ui/PageHeader";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Badge } from "@/components/ui/Badge";
import { DefinitionList } from "@/components/ui/DefinitionList";
import { MoneyText } from "@/components/ui/MoneyText";
import { DataTable, type Column } from "@/components/ui/DataTable";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { StatusBadge } from "@/components/ui/StatusBadge";
import { ErrorState, LoadingState } from "@/components/ui/States";

type BatchKind = "eod" | "eom" | "eoy";

/** Permission `system:config` (domain.RolePermissions) hanya dimiliki SUPERADMIN. */
const CLOSING_ROLE = "SUPERADMIN";

function bookLabel(book: EOYBookResult["book"]): string {
  if (book === "SYARIAH") return "Syariah";
  if (book === "CONVENTIONAL") return "Konvensional";
  return book;
}

const BOOK_COLUMNS: Column<EOYBookResult>[] = [
  { header: "Buku", cell: (row) => bookLabel(row.book) },
  {
    header: "Status",
    cell: (row) =>
      row.already_closed ? (
        <Badge variant="accent">Sudah ditutup sebelumnya</Badge>
      ) : (
        <Badge variant="credit">Ditutup</Badge>
      ),
  },
  {
    header: "Pendapatan Ditutup",
    type: "money",
    cell: (row) => <MoneyText value={row.total_revenue_closed} />,
  },
  {
    header: "Beban Ditutup",
    type: "money",
    cell: (row) => <MoneyText value={row.total_expense_closed} />,
  },
  {
    header: "Laba Ditahan",
    type: "money",
    cell: (row) => (
      <MoneyText
        value={row.net_retained_earnings}
        tone={Number(row.net_retained_earnings) < 0 ? "debit" : "default"}
      />
    ),
  },
  {
    header: "Akun Laba Ditahan",
    cell: (row) => (
      <span className="font-mono">{row.retained_earnings_coa_code || "-"}</span>
    ),
  },
  {
    header: "Ref Jurnal",
    cell: (row) => (
      <span className="font-mono">{row.closing_journal_ref || "-"}</span>
    ),
  },
];

export default function TutupHariPage() {
  const { user } = useAuth();
  const canRun = user?.role === CLOSING_ROLE;

  const [businessDate, setBusinessDate] = useState<SystemBusinessDate | null>(
    null
  );
  const [dateLoading, setDateLoading] = useState(true);
  const [dateError, setDateError] = useState<string | null>(null);

  const [eod, setEod] = useState<EODSummaryResult | null>(null);
  const [eom, setEom] = useState<EOMSummaryResult | null>(null);
  const [eoy, setEoy] = useState<EOYSummaryResult | null>(null);

  const [confirmKind, setConfirmKind] = useState<BatchKind | null>(null);
  const [running, setRunning] = useState<BatchKind | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const loadBusinessDate = useCallback(async () => {
    setDateLoading(true);
    setDateError(null);
    try {
      const response = await request<SystemBusinessDate>(
        "/system/business-date"
      );
      setBusinessDate(response.data ?? null);
    } catch (err) {
      setDateError(
        err instanceof ApiError
          ? err.message
          : "Gagal memuat tanggal bisnis sistem."
      );
    } finally {
      setDateLoading(false);
    }
  }, []);

  useEffect(() => {
    loadBusinessDate();
  }, [loadBusinessDate]);

  const runBatch = async (kind: BatchKind) => {
    setRunning(kind);
    setActionError(null);
    try {
      if (kind === "eod") {
        const response = await request<EODSummaryResult>("/batch/eod", {
          method: "POST",
          idempotencyKey: newIdempotencyKey(),
        });
        setEod(response.data ?? null);
      } else if (kind === "eom") {
        const response = await request<EOMSummaryResult>("/batch/eom", {
          method: "POST",
          idempotencyKey: newIdempotencyKey(),
        });
        setEom(response.data ?? null);
      } else {
        const response = await request<EOYSummaryResult>("/batch/eoy", {
          method: "POST",
          idempotencyKey: newIdempotencyKey(),
        });
        setEoy(response.data ?? null);
      }
      setConfirmKind(null);
      // EOD memajukan tanggal bisnis, jadi tanggal berjalan dimuat ulang.
      await loadBusinessDate();
    } catch (err) {
      setActionError(
        err instanceof ApiError
          ? err.message
          : "Proses tutup gagal dijalankan."
      );
      setConfirmKind(null);
    } finally {
      setRunning(null);
    }
  };

  const confirmTitle =
    confirmKind === "eod"
      ? "Jalankan Tutup Hari (EOD)"
      : confirmKind === "eom"
        ? "Jalankan Tutup Bulan (EOM)"
        : "Jalankan Tutup Tahun (EOY)";

  const confirmLabel =
    confirmKind === "eod"
      ? "Jalankan Tutup Hari"
      : confirmKind === "eom"
        ? "Jalankan Tutup Bulan"
        : "Tutup Buku Tahun Ini";

  return (
    <>
      <PageHeader
        title="Tutup Hari & Buku"
        description="Jalankan proses tutup hari (EOD), tutup bulan (EOM), dan tutup tahun (EOY)."
        actions={
          canRun ? (
            <>
              <Button
                variant="secondary"
                onClick={() => setConfirmKind("eom")}
                disabled={running !== null}
              >
                Tutup Bulan (EOM)
              </Button>
              <Button
                variant="secondary"
                onClick={() => setConfirmKind("eoy")}
                disabled={running !== null}
              >
                Tutup Tahun (EOY)
              </Button>
              <Button
                onClick={() => setConfirmKind("eod")}
                disabled={running !== null}
                loading={running === "eod"}
              >
                Jalankan Tutup Hari (EOD)
              </Button>
            </>
          ) : undefined
        }
      />

      {!canRun && (
        <div className="mb-4 rounded-md border border-border bg-canvas px-4 py-3">
          <p className="text-body font-medium text-ink-900">
            Tutup hari hanya dapat dijalankan oleh peran {CLOSING_ROLE}.
          </p>
          <p className="mt-1 text-body text-ink-600">
            Tanggal bisnis berjalan tetap ditampilkan di bawah ini. Minta
            superadmin menjalankan EOD, EOM, atau EOY.
          </p>
        </div>
      )}

      {actionError && (
        <div
          className="mb-4 rounded-md border border-debit-700/30 bg-debit-50 px-4 py-3"
          role="alert"
        >
          <p className="text-title font-medium text-debit-700">
            Proses tutup gagal
          </p>
          <p className="mt-1 text-body text-debit-700">{actionError}</p>
        </div>
      )}

      <Card className="mb-4">
        <CardHeader>
          <CardTitle>Tanggal Bisnis Berjalan</CardTitle>
        </CardHeader>
        <CardContent>
          {dateLoading ? (
            <LoadingState label="Memuat tanggal bisnis..." />
          ) : dateError ? (
            <ErrorState
              title="Gagal memuat tanggal bisnis"
              description={dateError}
              action={
                <Button variant="secondary" onClick={loadBusinessDate}>
                  Coba lagi
                </Button>
              }
            />
          ) : businessDate ? (
            <DefinitionList
              items={[
                {
                  label: "Tanggal Berjalan",
                  value: formatDate(businessDate.current_date),
                  isMono: true,
                },
                {
                  label: "Status",
                  value: <StatusBadge status={businessDate.status} />,
                },
                {
                  label: "Terakhir Diperbarui",
                  value: formatDateTime(businessDate.updated_at),
                  isMono: true,
                },
              ]}
            />
          ) : (
            <p className="text-body text-ink-600">
              Tanggal bisnis belum tersedia.
            </p>
          )}
        </CardContent>
      </Card>

      {eod && (
        <>
          {eod.warnings && eod.warnings.length > 0 && (
            <div
              className="mb-4 rounded-md border border-accent-600/40 bg-accent-50 px-4 py-3"
              role="alert"
            >
              <p className="text-title font-semibold text-accent-600">
                Tutup hari berhasil, tetapi belum lengkap
              </p>
              <p className="mt-1 text-body text-ink-900">
                {eod.warnings.length} pekerjaan harian tidak berjalan dan perlu
                diperiksa. Tutup hari tetap selesai, tetapi hasilnya tidak
                menyeluruh.
              </p>
              <ul className="mt-2 list-disc space-y-1 pl-5 text-body text-ink-900">
                {eod.warnings.map((warning, index) => (
                  <li key={index}>{warning}</li>
                ))}
              </ul>
            </div>
          )}

          <Card className="mb-4">
            <CardHeader>
              <CardTitle>
                Hasil Tutup Hari (EOD)
              </CardTitle>
              <span className="text-meta text-ink-600">
                Selesai {formatDateTime(eod.completed_at)}
              </span>
            </CardHeader>
            <CardContent>
              {(!eod.warnings || eod.warnings.length === 0) && (
                <p className="text-body text-credit-700">
                  Tutup hari selesai tanpa peringatan.
                </p>
              )}
              <DefinitionList
                items={[
                  {
                    label: "Tanggal Dieksekusi",
                    value: formatDate(eod.executed_date),
                    isMono: true,
                  },
                  {
                    label: "Tanggal Bisnis Berikutnya",
                    value: formatDate(eod.next_business_date),
                    isMono: true,
                  },
                  {
                    label: "Jurnal Terposting Hari Ini",
                    value: eod.total_posted_journals_today,
                    isMono: true,
                  },
                  {
                    label: "Total Setoran Hari Ini",
                    value: (
                      <MoneyText value={eod.total_deposit_amount_today} />
                    ),
                  },
                  {
                    label: "Penempatan Deposito Berjangka",
                    value: eod.total_deposit_placements_today,
                    isMono: true,
                  },
                  {
                    label: "Nominal Penempatan Deposito",
                    value: (
                      <MoneyText
                        value={eod.total_deposit_placement_amount_today}
                      />
                    ),
                  },
                  {
                    label: "Total Penarikan Hari Ini",
                    value: (
                      <MoneyText value={eod.total_withdrawal_amount_today} />
                    ),
                  },
                ]}
              />
              <div>
                <p className="mb-2 text-meta font-medium uppercase tracking-wide text-ink-600">
                  Pekerjaan Harian
                </p>
                <DefinitionList
                  items={[
                    {
                      label: "Deposito Diperpanjang Otomatis",
                      value: eod.deposits_rolled_over,
                      isMono: true,
                    },
                    {
                      label: "PPAP Diproses",
                      value: eod.ppap_processed,
                      isMono: true,
                    },
                    {
                      label: "PPAP Disesuaikan",
                      value: eod.ppap_adjusted,
                      isMono: true,
                    },
                    {
                      label: "Denda Kredit Diakru",
                      value: eod.loan_penalties_accrued,
                      isMono: true,
                    },
                    {
                      label: "Nominal Denda",
                      value: <MoneyText value={eod.loan_penalty_amount} />,
                    },
                    {
                      label: "Bunga Kredit Diakru",
                      value: eod.loan_interest_accrued,
                      isMono: true,
                    },
                    {
                      label: "Nominal Bunga Diakru",
                      value: <MoneyText value={eod.loan_interest_accrued_amount} />,
                    },
                    {
                      label: "Rekening Ditandai Dormant",
                      value: eod.accounts_marked_dormant,
                      isMono: true,
                    },
                  ]}
                />
              </div>
            </CardContent>
          </Card>
        </>
      )}

      {eom && (
        <Card className="mb-4">
          <CardHeader>
            <CardTitle>Hasil Tutup Bulan (EOM)</CardTitle>
            <span className="text-meta text-ink-600">
              Selesai {formatDateTime(eom.completed_at)}
            </span>
          </CardHeader>
          <CardContent>
            <p className="text-body text-credit-700">Tutup bulan selesai.</p>
            <DefinitionList
              items={[
                {
                  label: "Bulan Dieksekusi",
                  value: eom.executed_month,
                  isMono: true,
                },
                {
                  label: "Total Biaya Admin Dipotong",
                  value: <MoneyText value={eom.total_admin_fees_deducted} />,
                },
                {
                  label: "Total Bunga Dibayar",
                  value: <MoneyText value={eom.total_interest_paid} />,
                },
                {
                  label: "Rekening Diproses",
                  value: eom.processed_accounts,
                  isMono: true,
                },
                {
                  label: "Rekening Gagal",
                  value: (
                    <span
                      className={
                        eom.failed_accounts > 0
                          ? "text-debit-700"
                          : "text-ink-900"
                      }
                    >
                      {eom.failed_accounts}
                    </span>
                  ),
                  isMono: true,
                },
              ]}
            />
          </CardContent>
        </Card>
      )}

      {eoy && (
        <Card className="mb-4">
          <CardHeader>
            <CardTitle>Hasil Tutup Tahun (EOY)</CardTitle>
            <span className="text-meta text-ink-600">
              Selesai {formatDateTime(eoy.completed_at)}
            </span>
          </CardHeader>
          <CardContent>
            <p className="text-body text-credit-700">
              Tutup buku tahun {eoy.fiscal_year} selesai.
            </p>
            <DefinitionList
              items={[
                {
                  label: "Tahun Fiskal",
                  value: eoy.fiscal_year,
                  isMono: true,
                },
                {
                  label: "Total Pendapatan Ditutup",
                  value: <MoneyText value={eoy.total_revenue_closed} />,
                },
                {
                  label: "Total Beban Ditutup",
                  value: <MoneyText value={eoy.total_expense_closed} />,
                },
                {
                  label: "Laba Ditahan Neto",
                  value: (
                    <MoneyText
                      value={eoy.net_retained_earnings}
                      tone={
                        Number(eoy.net_retained_earnings) < 0
                          ? "debit"
                          : "default"
                      }
                    />
                  ),
                },
                {
                  label: "Referensi Jurnal Penutup",
                  value: eoy.closing_journal_ref || "-",
                  isMono: true,
                },
              ]}
            />
            {eoy.books && eoy.books.length > 0 && (
              <div>
                <p className="mb-2 text-meta font-medium uppercase tracking-wide text-ink-600">
                  Rincian per Buku
                </p>
                <DataTable
                  columns={BOOK_COLUMNS}
                  data={eoy.books}
                  keyExtractor={(row) => row.book}
                  emptyMessage="Tidak ada buku yang ditutup."
                />
              </div>
            )}
          </CardContent>
        </Card>
      )}

      <ConfirmDialog
        open={confirmKind !== null}
        title={confirmTitle}
        confirmLabel={confirmLabel}
        destructive={confirmKind === "eoy"}
        loading={running !== null}
        requireKeyword={confirmKind === "eoy" ? "TUTUP BUKU" : undefined}
        onCancel={() => setConfirmKind(null)}
        onConfirm={() => {
          if (confirmKind) runBatch(confirmKind);
        }}
        description={
          confirmKind === "eod" ? (
            <>
              <p>
                Tutup hari akan menjalankan pekerjaan harian (perpanjangan
                otomatis deposito, PPAP, akrual denda kredit, penandaan rekening
                dormant), lalu memajukan tanggal bisnis ke hari berikutnya.
              </p>
              <p>
                Pastikan seluruh transaksi hari ini sudah diposting sebelum
                melanjutkan.
              </p>
            </>
          ) : confirmKind === "eom" ? (
            <>
              <p>
                Tutup bulan akan mengakru dan membayar bunga tabungan, serta
                memotong biaya administrasi untuk periode bulan berjalan. Jurnal
                terkait ikut diposting.
              </p>
            </>
          ) : (
            <>
              <p>
                Tutup tahun akan memindahkan saldo akun pendapatan dan beban ke
                laba ditahan untuk Buku Konvensional dan Syariah, lalu menerbitkan
                jurnal penutup.
              </p>
              <p className="font-medium text-debit-700">
                Aksi ini tidak dapat dibatalkan. Setelah ditutup, buku tidak dapat
                dibuka kembali.
              </p>
            </>
          )
        }
      />
    </>
  );
}
