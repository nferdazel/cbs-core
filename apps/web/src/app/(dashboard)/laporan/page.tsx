"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { ApiError, request } from "@/lib/api";
import {
  BalanceSheet,
  CashFlow,
  IncomeStatement,
  ReportKind,
  ReportRow,
  TrialBalanceRow,
} from "@/lib/types";
import { useTranslation } from "@/i18n/context";
import { PageHeader } from "@/components/ui/PageHeader";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { DataTable, Column } from "@/components/ui/DataTable";
import { MoneyText } from "@/components/ui/MoneyText";
import { Select } from "@/components/ui/Select";
import { DateInput } from "@/components/ui/DateInput";
import { DefinitionList } from "@/components/ui/DefinitionList";
import { ErrorState, LoadingState } from "@/components/ui/States";

function isoDate(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

export default function LaporanPage() {
  const { t } = useTranslation();
  const today = new Date();
  const firstOfMonth = new Date(today.getFullYear(), today.getMonth(), 1);

  const [kind, setKind] = useState<ReportKind>("trial-balance");
  const [from, setFrom] = useState(isoDate(firstOfMonth));
  const [to, setTo] = useState(isoDate(today));
  const [asOf, setAsOf] = useState(isoDate(today));

  const [trial, setTrial] = useState<TrialBalanceRow[] | null>(null);
  const [income, setIncome] = useState<IncomeStatement | null>(null);
  const [balance, setBalance] = useState<BalanceSheet | null>(null);
  const [cash, setCash] = useState<CashFlow | null>(null);

  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [reloadKey, setReloadKey] = useState(0);

  // Jenis laporan disimpan sebagai kunci teknis (trial-balance dll.); hanya
  // label tampilannya yang diambil dari kamus.
  const reportOptions = useMemo(
    () => [
      { value: "trial-balance", label: t.reports.kindTrialBalance },
      { value: "balance-sheet", label: t.reports.kindBalanceSheet },
      { value: "income-statement", label: t.reports.kindIncomeStatement },
      { value: "cash-flow", label: t.reports.kindCashFlow },
    ],
    [t],
  );

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    setTrial(null);
    setIncome(null);
    setBalance(null);
    setCash(null);
    try {
      if (kind === "trial-balance") {
        const res = await request<TrialBalanceRow[]>(
          `/reports/trial-balance?from=${from}&to=${to}`,
        );
        setTrial(res.data ?? []);
      } else if (kind === "balance-sheet") {
        const res = await request<BalanceSheet>(
          `/reports/balance-sheet?as_of=${asOf}`,
        );
        setBalance(res.data ?? null);
      } else if (kind === "income-statement") {
        const res = await request<IncomeStatement>(
          `/reports/income-statement?from=${from}&to=${to}`,
        );
        setIncome(res.data ?? null);
      } else {
        const res = await request<CashFlow>(
          `/reports/cash-flow?from=${from}&to=${to}`,
        );
        setCash(res.data ?? null);
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t.reports.loadError);
    } finally {
      setLoading(false);
    }
  }, [kind, from, to, asOf, reloadKey, t]);

  useEffect(() => {
    load();
  }, [load]);

  const trialColumns: Column<TrialBalanceRow>[] = [
    { header: t.reports.colCode, accessorKey: "account_code", isMono: true },
    { header: t.reports.colAccountName, accessorKey: "account_name" },
    { header: t.reports.colNormalBalance, accessorKey: "normal_balance" },
    {
      header: t.reports.colDebit,
      type: "money",
      cell: (row) => <MoneyText value={row.total_debit} />,
    },
    {
      header: t.reports.colCredit,
      type: "money",
      cell: (row) => <MoneyText value={row.total_credit} />,
    },
    {
      header: t.reports.colClosingBalance,
      type: "money",
      cell: (row) => <MoneyText value={row.closing_balance} />,
    },
  ];

  const reportColumns: Column<ReportRow>[] = [
    { header: t.reports.colCode, accessorKey: "account_code", isMono: true },
    { header: t.reports.colAccountName, accessorKey: "account_name" },
    { header: t.reports.colBook, accessorKey: "book" },
    {
      header: t.reports.colAmount,
      type: "money",
      cell: (row) => <MoneyText value={row.amount} />,
    },
  ];

  const usesRange = kind !== "balance-sheet";

  return (
    <>
      <PageHeader title={t.reports.title} description={t.reports.description} />

      <Card className="mb-4">
        <CardContent className="flex flex-wrap items-end gap-4">
          <div className="w-56">
            <Select
              label={t.reports.reportTypeLabel}
              value={kind}
              onChange={(e) => setKind(e.target.value as ReportKind)}
              options={reportOptions}
            />
          </div>
          {usesRange ? (
            <>
              <div className="w-44">
                <DateInput
                  label={t.reports.from}
                  value={from}
                  onChange={(e) => setFrom(e.target.value)}
                />
              </div>
              <div className="w-44">
                <DateInput
                  label={t.reports.to}
                  value={to}
                  onChange={(e) => setTo(e.target.value)}
                />
              </div>
            </>
          ) : (
            <div className="w-44">
              <DateInput
                label={t.reports.asOf}
                value={asOf}
                onChange={(e) => setAsOf(e.target.value)}
              />
            </div>
          )}
          <Button
            onClick={() => setReloadKey((key) => key + 1)}
            loading={loading}
          >
            {t.reports.show}
          </Button>
        </CardContent>
      </Card>

      {error && !loading ? (
        <ErrorState title={t.reports.errorTitle} description={error} />
      ) : (
        <Card>
          <CardHeader>
            <CardTitle>
              {reportOptions.find((o) => o.value === kind)?.label}
            </CardTitle>
          </CardHeader>
          <CardContent className="p-0">
            {loading && kind !== "trial-balance" && <LoadingState />}

            {kind === "trial-balance" && (
              <DataTable
                columns={trialColumns}
                data={trial ?? []}
                keyExtractor={(row) => row.account_code}
                loading={loading}
                emptyMessage={t.reports.emptyTrial}
                zebra
              />
            )}

            {kind === "income-statement" && income && (
              <>
                <DataTable
                  columns={reportColumns}
                  data={income.rows}
                  keyExtractor={(row) => row.account_code}
                  loading={loading}
                  emptyMessage={t.reports.emptyIncome}
                  zebra
                />
                <div className="border-t border-border p-4">
                  <DefinitionList
                    items={[
                      {
                        label: t.reports.totalRevenue,
                        value: <MoneyText value={income.total_revenue} />,
                      },
                      {
                        label: t.reports.totalExpense,
                        value: <MoneyText value={income.total_expense} />,
                      },
                      {
                        label: t.reports.netIncome,
                        value: (
                          <MoneyText
                            value={income.net_income}
                            tone={
                              Number(income.net_income) >= 0
                                ? "credit"
                                : "debit"
                            }
                          />
                        ),
                      },
                    ]}
                  />
                </div>
              </>
            )}

            {kind === "balance-sheet" && balance && (
              <>
                <DataTable
                  columns={reportColumns}
                  data={balance.rows}
                  keyExtractor={(row) => row.account_code}
                  loading={loading}
                  emptyMessage={t.reports.emptyBalance}
                  zebra
                />
                <div className="border-t border-border p-4">
                  <DefinitionList
                    items={[
                      {
                        label: t.reports.totalAssets,
                        value: <MoneyText value={balance.total_assets} />,
                      },
                      {
                        label: t.reports.totalLiabilities,
                        value: <MoneyText value={balance.total_liabilities} />,
                      },
                      {
                        label: t.reports.totalEquity,
                        value: <MoneyText value={balance.total_equity} />,
                      },
                      {
                        label: t.reports.netIncomeCurrent,
                        value: <MoneyText value={balance.net_income} />,
                      },
                    ]}
                  />
                </div>
              </>
            )}

            {kind === "cash-flow" && cash && (
              <>
                <DataTable
                  columns={reportColumns}
                  data={cash.rows}
                  keyExtractor={(row) => row.account_code}
                  loading={loading}
                  emptyMessage={t.reports.emptyCash}
                  zebra
                />
                <div className="border-t border-border p-4">
                  <DefinitionList
                    items={[
                      {
                        label: t.reports.operating,
                        value: <MoneyText value={cash.operating} />,
                      },
                      {
                        label: t.reports.investing,
                        value: <MoneyText value={cash.investing} />,
                      },
                      {
                        label: t.reports.financing,
                        value: <MoneyText value={cash.financing} />,
                      },
                      {
                        label: t.reports.netChange,
                        value: <MoneyText value={cash.net_change} />,
                      },
                    ]}
                  />
                </div>
              </>
            )}

            {kind !== "trial-balance" &&
              !loading &&
              !income &&
              !balance &&
              !cash && (
                <div className="px-4 py-8 text-center text-body text-ink-600">
                  {t.reports.emptyReport}
                </div>
              )}
          </CardContent>
        </Card>
      )}
    </>
  );
}
