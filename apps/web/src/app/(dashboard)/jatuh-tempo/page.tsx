"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import type { Customer } from "@cbs/shared-types";
import { ApiError, request } from "@/lib/api";
import { formatDate } from "@/lib/format";
import { useTranslation } from "@/i18n/context";
import type { DueObligation, DueObligationKind } from "@/lib/operations-types";
import { PageHeader } from "@/components/ui/PageHeader";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Select } from "@/components/ui/Select";
import { Badge } from "@/components/ui/Badge";
import { DataTable, type Column } from "@/components/ui/DataTable";
import { MoneyText } from "@/components/ui/MoneyText";
import { ErrorState } from "@/components/ui/States";

/** Horison hari; label dibentuk dari kamus agar bahasa mengikuti pilihan pengguna. */
const HORIZON_DAYS = ["7", "30", "60", "90"] as const;

/** Penanda status berbasis selisih hari; negatif berarti sudah lewat. */
function DueStatusBadge({ item }: { item: DueObligation }) {
  const { t } = useTranslation();
  if (item.overdue) {
    return (
      <Badge variant="debit">
        {t.dueDates.overduePrefix}
        {Math.abs(item.days_remaining)}
        {t.dueDates.overdueSuffix}
      </Badge>
    );
  }
  if (item.days_remaining === 0) {
    return <Badge variant="accent">{t.dueDates.today}</Badge>;
  }
  return (
    <Badge variant="credit">
      {t.dueDates.remainingPrefix}
      {item.days_remaining}
      {t.dueDates.remainingSuffix}
    </Badge>
  );
}

export default function JatuhTempoPage() {
  const { t } = useTranslation();
  const [items, setItems] = useState<DueObligation[]>([]);
  const [days, setDays] = useState("30");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  // Hanya nilai yang dibaca; tidak ada aksi muat ulang di halaman ini, jadi setter
  // tidak pernah dipakai (temuan @typescript-eslint/no-unused-vars).
  const [reloadKey] = useState(0);
  const [customerNames, setCustomerNames] = useState<Record<string, string>>(
    {},
  );

  const horizonOptions = useMemo(
    () =>
      HORIZON_DAYS.map((value) => ({
        value,
        label: `${value} ${t.dueDates.daysAhead}`,
      })),
    [t],
  );

  const kindLabel = useCallback(
    (kind: DueObligationKind): string =>
      kind === "LOAN_INSTALLMENT"
        ? t.dueDates.kindLoan
        : t.dueDates.kindDeposit,
    [t],
  );

  const load = useCallback(
    async (horizon: string) => {
      setLoading(true);
      setError(null);
      try {
        const response = await request<DueObligation[]>(
          `/reports/due-obligations?days=${encodeURIComponent(horizon)}`,
        );
        setItems(response.data ?? []);
      } catch (err) {
        setError(err instanceof ApiError ? err.message : t.dueDates.loadError);
      } finally {
        setLoading(false);
      }
    },
    [t],
  );

  useEffect(() => {
    load(days);
  }, [days, reloadKey, load]);

  // Nama nasabah tidak dikembalikan endpoint agar backend tidak mendekripsi sekaligus
  // banyak baris; peta nama diambil dari daftar nasabah yang sudah dipakai halaman lain.
  useEffect(() => {
    let cancelled = false;
    request<Customer[]>("/customers?page=1&page_size=100")
      .then((res) => {
        if (cancelled) return;
        setCustomerNames(
          Object.fromEntries((res.data ?? []).map((c) => [c.id, c.full_name])),
        );
      })
      .catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, []);

  const overdueCount = useMemo(
    () => items.filter((item) => item.overdue).length,
    [items],
  );

  const columns: Column<DueObligation>[] = [
    {
      header: t.dueDates.colKind,
      cell: (row) => kindLabel(row.kind),
    },
    {
      header: t.dueDates.colReference,
      cell: (row) => (
        <span className="font-mono">
          {row.reference}
          {row.installment_no
            ? ` / ${t.dueDates.installmentSuffix}${row.installment_no}`
            : ""}
        </span>
      ),
    },
    {
      header: t.dueDates.colCustomer,
      cell: (row) =>
        customerNames[row.customer_id] ?? (
          <span className="font-mono">{row.customer_id}</span>
        ),
    },
    {
      header: t.dueDates.colDueDate,
      cell: (row) => formatDate(row.due_date),
      isMono: true,
    },
    {
      header: t.dueDates.colAmount,
      type: "money",
      cell: (row) => <MoneyText value={row.amount} />,
    },
    {
      header: t.common.status,
      cell: (row) => <DueStatusBadge item={row} />,
    },
  ];

  return (
    <>
      <PageHeader
        title={t.dueDates.title}
        description={t.dueDates.description}
      />

      {error && !loading ? (
        <ErrorState title={t.dueDates.errorTitle} description={error} />
      ) : (
        <Card>
          <CardHeader>
            <CardTitle>
              {t.dueDates.listTitle}
              {overdueCount > 0 && (
                <span className="ml-2 align-middle">
                  <Badge variant="debit">
                    {overdueCount}
                    {t.dueDates.overdueCountSuffix}
                  </Badge>
                </span>
              )}
            </CardTitle>
            <div className="w-56">
              <Select
                label={t.dueDates.horizon}
                value={days}
                onChange={(event) => setDays(event.target.value)}
                options={horizonOptions}
              />
            </div>
          </CardHeader>
          <CardContent className="p-0">
            <DataTable
              columns={columns}
              data={items}
              keyExtractor={(row) =>
                `${row.kind}-${row.reference}-${row.due_date}`
              }
              loading={loading}
              emptyMessage={t.dueDates.empty}
              zebra
            />
          </CardContent>
        </Card>
      )}
    </>
  );
}
