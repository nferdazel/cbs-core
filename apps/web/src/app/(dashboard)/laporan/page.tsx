"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, request } from "@/lib/api";
import {
  BalanceSheet,
  CashFlow,
  IncomeStatement,
  ReportKind,
  ReportRow,
  TrialBalanceRow,
} from "@/lib/types";
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

const REPORT_OPTIONS = [
  { value: "trial-balance", label: "Neraca Saldo" },
  { value: "balance-sheet", label: "Neraca" },
  { value: "income-statement", label: "Laba Rugi" },
  { value: "cash-flow", label: "Arus Kas" },
];

export default function LaporanPage() {
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
          `/reports/trial-balance?from=${from}&to=${to}`
        );
        setTrial(res.data ?? []);
      } else if (kind === "balance-sheet") {
        const res = await request<BalanceSheet>(
          `/reports/balance-sheet?as_of=${asOf}`
        );
        setBalance(res.data ?? null);
      } else if (kind === "income-statement") {
        const res = await request<IncomeStatement>(
          `/reports/income-statement?from=${from}&to=${to}`
        );
        setIncome(res.data ?? null);
      } else {
        const res = await request<CashFlow>(
          `/reports/cash-flow?from=${from}&to=${to}`
        );
        setCash(res.data ?? null);
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Gagal memuat laporan.");
    } finally {
      setLoading(false);
    }
  }, [kind, from, to, asOf, reloadKey]);

  useEffect(() => {
    load();
  }, [load]);

  const trialColumns: Column<TrialBalanceRow>[] = [
    { header: "Kode", accessorKey: "account_code", isMono: true },
    { header: "Nama Akun", accessorKey: "account_name" },
    { header: "Saldo Normal", accessorKey: "normal_balance" },
    {
      header: "Debit",
      type: "money",
      cell: (row) => <MoneyText value={row.total_debit} />,
    },
    {
      header: "Kredit",
      type: "money",
      cell: (row) => <MoneyText value={row.total_credit} />,
    },
    {
      header: "Saldo Akhir",
      type: "money",
      cell: (row) => <MoneyText value={row.closing_balance} />,
    },
  ];

  const reportColumns: Column<ReportRow>[] = [
    { header: "Kode", accessorKey: "account_code", isMono: true },
    { header: "Nama Akun", accessorKey: "account_name" },
    { header: "Buku", accessorKey: "book" },
    {
      header: "Jumlah",
      type: "money",
      cell: (row) => <MoneyText value={row.amount} />,
    },
  ];

  const usesRange = kind !== "balance-sheet";

  return (
    <>
      <PageHeader
        title="Laporan"
        description="Laporan keuangan dihitung dari jurnal. Pilih jenis dan rentang tanggal."
      />

      <Card className="mb-4">
        <CardContent className="flex flex-wrap items-end gap-4">
          <div className="w-56">
            <Select
              label="Jenis Laporan"
              value={kind}
              onChange={(e) => setKind(e.target.value as ReportKind)}
              options={REPORT_OPTIONS}
            />
          </div>
          {usesRange ? (
            <>
              <div className="w-44">
                <DateInput label="Dari" value={from} onChange={(e) => setFrom(e.target.value)} />
              </div>
              <div className="w-44">
                <DateInput label="Sampai" value={to} onChange={(e) => setTo(e.target.value)} />
              </div>
            </>
          ) : (
            <div className="w-44">
              <DateInput
                label="Per Tanggal"
                value={asOf}
                onChange={(e) => setAsOf(e.target.value)}
              />
            </div>
          )}
          <Button onClick={() => setReloadKey((key) => key + 1)} loading={loading}>
            Tampilkan
          </Button>
        </CardContent>
      </Card>

      {error && !loading ? (
        <ErrorState title="Gagal memuat laporan" description={error} />
      ) : (
        <Card>
          <CardHeader>
            <CardTitle>{REPORT_OPTIONS.find((o) => o.value === kind)?.label}</CardTitle>
          </CardHeader>
          <CardContent className="p-0">
            {loading && kind !== "trial-balance" && <LoadingState />}

            {kind === "trial-balance" && (
              <DataTable
                columns={trialColumns}
                data={trial ?? []}
                keyExtractor={(row) => row.account_code}
                loading={loading}
                emptyMessage="Tidak ada saldo pada rentang ini."
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
                  emptyMessage="Tidak ada mutu pendapatan/beban pada rentang ini."
                  zebra
                />
                <div className="border-t border-border p-4">
                  <DefinitionList
                    items={[
                      { label: "Total Pendapatan", value: <MoneyText value={income.total_revenue} /> },
                      { label: "Total Beban", value: <MoneyText value={income.total_expense} /> },
                      { label: "Laba/Rugi Bersih", value: <MoneyText value={income.net_income} tone={Number(income.net_income) >= 0 ? "credit" : "debit"} /> },
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
                  emptyMessage="Tidak ada saldo akun."
                  zebra
                />
                <div className="border-t border-border p-4">
                  <DefinitionList
                    items={[
                      { label: "Total Aset", value: <MoneyText value={balance.total_assets} /> },
                      { label: "Total Liabilitas", value: <MoneyText value={balance.total_liabilities} /> },
                      { label: "Total Ekuitas", value: <MoneyText value={balance.total_equity} /> },
                      { label: "Laba/Rugi Berjalan", value: <MoneyText value={balance.net_income} /> },
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
                  emptyMessage="Tidak ada arus kas pada rentang ini."
                  zebra
                />
                <div className="border-t border-border p-4">
                  <DefinitionList
                    items={[
                      { label: "Operasi", value: <MoneyText value={cash.operating} /> },
                      { label: "Investasi", value: <MoneyText value={cash.investing} /> },
                      { label: "Pendanaan", value: <MoneyText value={cash.financing} /> },
                      { label: "Perubahan Bersih", value: <MoneyText value={cash.net_change} /> },
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
                  Tidak ada data laporan.
                </div>
              )}
          </CardContent>
        </Card>
      )}
    </>
  );
}
