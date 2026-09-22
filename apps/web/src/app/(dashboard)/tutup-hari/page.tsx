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
import { hasPermission } from "@/lib/permissions";
import { useTranslation } from "@/i18n/context";
import type { Dictionary } from "@/i18n/dictionaries/id";
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

// Buku adalah kunci teknis (CONVENTIONAL/SYARIAH); hanya label tampilannya dari kamus.
function bookLabel(t: Dictionary["dayClose"], book: EOYBookResult["book"]): string {
  if (book === "SYARIAH") return t.bookSyariah;
  if (book === "CONVENTIONAL") return t.bookConventional;
  return book;
}

/** Nama pihak yang lebih tinggi pada perbandingan bayangan (domain.CKPNLarger). */
function ckpnShadowHigherLabel(
  t: Dictionary["dayClose"],
  higher: string
): string {
  if (higher === "PPKA") return t.higherPpka;
  if (higher === "CKPN") return t.higherCkpn;
  if (higher === "SAMA") return t.higherSame;
  return higher || "-";
}

/**
 * Bagian mode bayangan CKPN. Sengaja diberi label tegas "MODE BAYANGAN": angka ini
 * belum dijurnal dan belum mengurangi modal inti, jadi tidak boleh terbaca sebagai
 * laporan final.
 */
function CKPNShadowSection({ eod }: { eod: EODSummaryResult }) {
  const { t } = useTranslation();
  return (
    <Card className="mb-4 border-accent-600/40">
      <CardHeader>
        <div className="flex flex-wrap items-center gap-2">
          <CardTitle>{t.dayClose.ckpnTitle}</CardTitle>
          <Badge variant="accent">{t.dayClose.ckpnBadge}</Badge>
        </div>
      </CardHeader>
      <CardContent>
        {eod.ckpn_shadow_note && (
          <p className="mb-4 rounded-md border border-accent-600/40 bg-accent-50 px-4 py-3 text-body text-ink-900">
            {eod.ckpn_shadow_note}
          </p>
        )}
        <DefinitionList
          items={[
            {
              label: t.dayClose.ckpnProcessed,
              value: eod.ckpn_shadow_processed,
              isMono: true,
            },
            {
              label: t.dayClose.ckpnFailed,
              value: (
                <span
                  className={
                    eod.ckpn_shadow_failed > 0
                      ? "text-debit-700"
                      : "text-ink-900"
                  }
                >
                  {eod.ckpn_shadow_failed}
                </span>
              ),
              isMono: true,
            },
            {
              label: t.dayClose.ckpnTotalPpka,
              value: <MoneyText value={eod.ckpn_shadow_total_ppka} />,
            },
            {
              label: t.dayClose.ckpnTotalCkpn,
              value: <MoneyText value={eod.ckpn_shadow_total_ckpn} />,
            },
            {
              label: t.dayClose.ckpnDifference,
              value: <MoneyText value={eod.ckpn_shadow_difference} />,
            },
            {
              label: t.dayClose.ckpnHigher,
              value: ckpnShadowHigherLabel(t.dayClose, eod.ckpn_shadow_higher),
            },
          ]}
        />
        {eod.ckpn_shadow_assumptions &&
          eod.ckpn_shadow_assumptions.length > 0 && (
            <div className="mt-4">
              <p className="mb-2 text-meta font-medium uppercase tracking-wide text-ink-600">
                {t.dayClose.assumptions}
              </p>
              <ul className="list-disc space-y-1 pl-5 text-body text-ink-900">
                {eod.ckpn_shadow_assumptions.map((assumption, index) => (
                  <li key={index}>{assumption}</li>
                ))}
              </ul>
            </div>
          )}
      </CardContent>
    </Card>
  );
}

export default function TutupHariPage() {
  const { user } = useAuth();
  const { t } = useTranslation();
  // Aksi EOD/EOM/EOY dijaga izin system:config di API; izin efektif dari /auth/me.
  const canRun = hasPermission(user, "system:config");

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
          : t.dayClose.dateError
      );
    } finally {
      setDateLoading(false);
    }
  }, [t]);

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
          : t.dayClose.actionError
      );
      setConfirmKind(null);
    } finally {
      setRunning(null);
    }
  };

  const confirmTitle =
    confirmKind === "eod"
      ? t.dayClose.confirmEodTitle
      : confirmKind === "eom"
        ? t.dayClose.confirmEomTitle
        : t.dayClose.confirmEoyTitle;

  const confirmLabel =
    confirmKind === "eod"
      ? t.dayClose.confirmEodLabel
      : confirmKind === "eom"
        ? t.dayClose.confirmEomLabel
        : t.dayClose.confirmEoyLabel;

  const bookColumns: Column<EOYBookResult>[] = [
    { header: t.dayClose.book, cell: (row) => bookLabel(t.dayClose, row.book) },
    {
      header: t.common.status,
      cell: (row) =>
        row.already_closed ? (
          <Badge variant="accent">{t.dayClose.alreadyClosed}</Badge>
        ) : (
          <Badge variant="credit">{t.dayClose.closed}</Badge>
        ),
    },
    {
      header: t.dayClose.revenueClosed,
      type: "money",
      cell: (row) => <MoneyText value={row.total_revenue_closed} />,
    },
    {
      header: t.dayClose.expenseClosed,
      type: "money",
      cell: (row) => <MoneyText value={row.total_expense_closed} />,
    },
    {
      header: t.dayClose.retainedEarnings,
      type: "money",
      cell: (row) => (
        <MoneyText
          value={row.net_retained_earnings}
          tone={Number(row.net_retained_earnings) < 0 ? "debit" : "default"}
        />
      ),
    },
    {
      header: t.dayClose.retainedEarningsCoa,
      cell: (row) => (
        <span className="font-mono">{row.retained_earnings_coa_code || "-"}</span>
      ),
    },
    {
      header: t.dayClose.journalRef,
      cell: (row) => (
        <span className="font-mono">{row.closing_journal_ref || "-"}</span>
      ),
    },
  ];

  return (
    <>
      <PageHeader
        title={t.dayClose.title}
        description={t.dayClose.description}
        actions={
          canRun ? (
            <>
              <Button
                variant="secondary"
                onClick={() => setConfirmKind("eom")}
                disabled={running !== null}
              >
                {t.dayClose.eomButton}
              </Button>
              <Button
                variant="secondary"
                onClick={() => setConfirmKind("eoy")}
                disabled={running !== null}
              >
                {t.dayClose.eoyButton}
              </Button>
              <Button
                onClick={() => setConfirmKind("eod")}
                disabled={running !== null}
                loading={running === "eod"}
              >
                {t.dayClose.eodButton}
              </Button>
            </>
          ) : undefined
        }
      />

      {!canRun && (
        <div className="mb-4 rounded-md border border-border bg-canvas px-4 py-3">
          <p className="text-body font-medium text-ink-900">
            {t.dayClose.restrictedPrefix}
            {t.dayClose.restrictedPermission}
            {t.dayClose.restrictedSuffix}
          </p>
          <p className="mt-1 text-body text-ink-600">
            {t.dayClose.restrictedHint}
          </p>
        </div>
      )}

      {actionError && (
        <div
          className="mb-4 rounded-md border border-debit-700/30 bg-debit-50 px-4 py-3"
          role="alert"
        >
          <p className="text-title font-medium text-debit-700">
            {t.dayClose.actionErrorTitle}
          </p>
          <p className="mt-1 text-body text-debit-700">{actionError}</p>
        </div>
      )}

      <Card className="mb-4">
        <CardHeader>
          <CardTitle>{t.dayClose.currentDateTitle}</CardTitle>
        </CardHeader>
        <CardContent>
          {dateLoading ? (
            <LoadingState label={t.dayClose.loadingDate} />
          ) : dateError ? (
            <ErrorState
              title={t.dayClose.dateErrorTitle}
              description={dateError}
              action={
                <Button variant="secondary" onClick={loadBusinessDate}>
                  {t.common.retry}
                </Button>
              }
            />
          ) : businessDate ? (
            <DefinitionList
              items={[
                {
                  label: t.dayClose.currentDate,
                  value: formatDate(businessDate.current_date),
                  isMono: true,
                },
                {
                  label: t.common.status,
                  value: <StatusBadge status={businessDate.status} />,
                },
                {
                  label: t.dayClose.lastUpdated,
                  value: formatDateTime(businessDate.updated_at),
                  isMono: true,
                },
              ]}
            />
          ) : (
            <p className="text-body text-ink-600">
              {t.dayClose.unavailable}
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
                {t.dayClose.warningsTitle}
              </p>
              <p className="mt-1 text-body text-ink-900">
                {eod.warnings.length} {t.dayClose.warningsDesc}
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
                {t.dayClose.eodTitle}
              </CardTitle>
              <span className="text-meta text-ink-600">
                {t.dayClose.completed} {formatDateTime(eod.completed_at)}
              </span>
            </CardHeader>
            <CardContent>
              {(!eod.warnings || eod.warnings.length === 0) && (
                <p className="text-body text-credit-700">
                  {t.dayClose.eodNoWarnings}
                </p>
              )}
              <DefinitionList
                items={[
                  {
                    label: t.dayClose.executedDate,
                    value: formatDate(eod.executed_date),
                    isMono: true,
                  },
                  {
                    label: t.dayClose.nextBusinessDate,
                    value: formatDate(eod.next_business_date),
                    isMono: true,
                  },
                  {
                    label: t.dayClose.postedJournals,
                    value: eod.total_posted_journals_today,
                    isMono: true,
                  },
                  {
                    label: t.dayClose.totalDeposits,
                    value: (
                      <MoneyText value={eod.total_deposit_amount_today} />
                    ),
                  },
                  {
                    label: t.dayClose.depositPlacements,
                    value: eod.total_deposit_placements_today,
                    isMono: true,
                  },
                  {
                    label: t.dayClose.depositPlacementAmount,
                    value: (
                      <MoneyText
                        value={eod.total_deposit_placement_amount_today}
                      />
                    ),
                  },
                  {
                    label: t.dayClose.totalWithdrawals,
                    value: (
                      <MoneyText value={eod.total_withdrawal_amount_today} />
                    ),
                  },
                ]}
              />
              <div>
                <p className="mb-2 text-meta font-medium uppercase tracking-wide text-ink-600">
                  {t.dayClose.dailyJobs}
                </p>
                <DefinitionList
                  items={[
                    {
                      label: t.dayClose.rolledOver,
                      value: eod.deposits_rolled_over,
                      isMono: true,
                    },
                    {
                      label: t.dayClose.ppapProcessed,
                      value: eod.ppap_processed,
                      isMono: true,
                    },
                    {
                      label: t.dayClose.ppapAdjusted,
                      value: eod.ppap_adjusted,
                      isMono: true,
                    },
                    {
                      label: t.dayClose.penaltiesAccrued,
                      value: eod.loan_penalties_accrued,
                      isMono: true,
                    },
                    {
                      label: t.dayClose.penaltyAmount,
                      value: <MoneyText value={eod.loan_penalty_amount} />,
                    },
                    {
                      label: t.dayClose.interestAccrued,
                      value: eod.loan_interest_accrued,
                      isMono: true,
                    },
                    {
                      label: t.dayClose.interestAccruedAmount,
                      value: <MoneyText value={eod.loan_interest_accrued_amount} />,
                    },
                    {
                      label: t.dayClose.markedDormant,
                      value: eod.accounts_marked_dormant,
                      isMono: true,
                    },
                  ]}
                />
              </div>
            </CardContent>
          </Card>

          {eod.ckpn_shadow_mode && <CKPNShadowSection eod={eod} />}
        </>
      )}

      {eom && (
        <Card className="mb-4">
          <CardHeader>
            <CardTitle>{t.dayClose.eomTitle}</CardTitle>
            <span className="text-meta text-ink-600">
              {t.dayClose.completed} {formatDateTime(eom.completed_at)}
            </span>
          </CardHeader>
          <CardContent>
            <p className="text-body text-credit-700">{t.dayClose.eomDone}</p>
            <DefinitionList
              items={[
                {
                  label: t.dayClose.executedMonth,
                  value: eom.executed_month,
                  isMono: true,
                },
                {
                  label: t.dayClose.adminFees,
                  value: <MoneyText value={eom.total_admin_fees_deducted} />,
                },
                {
                  label: t.dayClose.interestPaid,
                  value: <MoneyText value={eom.total_interest_paid} />,
                },
                {
                  label: t.dayClose.processedAccounts,
                  value: eom.processed_accounts,
                  isMono: true,
                },
                {
                  label: t.dayClose.failedAccounts,
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
            <CardTitle>{t.dayClose.eoyTitle}</CardTitle>
            <span className="text-meta text-ink-600">
              {t.dayClose.completed} {formatDateTime(eoy.completed_at)}
            </span>
          </CardHeader>
          <CardContent>
            <p className="text-body text-credit-700">
              {t.dayClose.eoyDonePrefix} {eoy.fiscal_year}{" "}
              {t.dayClose.eoyDoneSuffix}
            </p>
            <DefinitionList
              items={[
                {
                  label: t.dayClose.fiscalYear,
                  value: eoy.fiscal_year,
                  isMono: true,
                },
                {
                  label: t.dayClose.totalRevenueClosed,
                  value: <MoneyText value={eoy.total_revenue_closed} />,
                },
                {
                  label: t.dayClose.totalExpenseClosed,
                  value: <MoneyText value={eoy.total_expense_closed} />,
                },
                {
                  label: t.dayClose.netRetained,
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
                  label: t.dayClose.closingJournalRef,
                  value: eoy.closing_journal_ref || "-",
                  isMono: true,
                },
              ]}
            />
            {eoy.books && eoy.books.length > 0 && (
              <div>
                <p className="mb-2 text-meta font-medium uppercase tracking-wide text-ink-600">
                  {t.dayClose.perBook}
                </p>
                <DataTable
                  columns={bookColumns}
                  data={eoy.books}
                  keyExtractor={(row) => row.book}
                  emptyMessage={t.dayClose.noBooks}
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
        requireKeyword={confirmKind === "eoy" ? t.dayClose.confirmKeyword : undefined}
        onCancel={() => setConfirmKind(null)}
        onConfirm={() => {
          if (confirmKind) runBatch(confirmKind);
        }}
        description={
          confirmKind === "eod" ? (
            <>
              <p>
                {t.dayClose.descEod1}
              </p>
              <p>
                {t.dayClose.descEod2}
              </p>
            </>
          ) : confirmKind === "eom" ? (
            <>
              <p>
                {t.dayClose.descEom}
              </p>
            </>
          ) : (
            <>
              <p>
                {t.dayClose.descEoy}
              </p>
              <p className="font-medium text-debit-700">
                {t.dayClose.descEoyWarning}
              </p>
            </>
          )
        }
      />
    </>
  );
}
