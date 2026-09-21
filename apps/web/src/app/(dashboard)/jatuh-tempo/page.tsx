"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import type { Customer } from "@cbs/shared-types";
import { ApiError, request } from "@/lib/api";
import { formatDate } from "@/lib/format";
import type { DueObligation, DueObligationKind } from "@/lib/operations-types";
import { PageHeader } from "@/components/ui/PageHeader";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Select } from "@/components/ui/Select";
import { Badge } from "@/components/ui/Badge";
import { DataTable, type Column } from "@/components/ui/DataTable";
import { MoneyText } from "@/components/ui/MoneyText";
import { ErrorState } from "@/components/ui/States";

const HORIZON_OPTIONS = [
  { value: "7", label: "7 hari ke depan" },
  { value: "30", label: "30 hari ke depan" },
  { value: "60", label: "60 hari ke depan" },
  { value: "90", label: "90 hari ke depan" },
];

function kindLabel(kind: DueObligationKind): string {
  return kind === "LOAN_INSTALLMENT" ? "Angsuran Kredit" : "Jatuh Tempo Deposito";
}

/** Penanda status berbasis selisih hari; negatif berarti sudah lewat. */
function DueStatusBadge({ item }: { item: DueObligation }) {
  if (item.overdue) {
    return <Badge variant="debit">Lewat {Math.abs(item.days_remaining)} hari</Badge>;
  }
  if (item.days_remaining === 0) {
    return <Badge variant="accent">Hari ini</Badge>;
  }
  return <Badge variant="credit">{item.days_remaining} hari lagi</Badge>;
}

export default function JatuhTempoPage() {
  const [items, setItems] = useState<DueObligation[]>([]);
  const [days, setDays] = useState("30");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [reloadKey, setReloadKey] = useState(0);
  const [customerNames, setCustomerNames] = useState<Record<string, string>>({});

  const load = useCallback(async (horizon: string) => {
    setLoading(true);
    setError(null);
    try {
      const response = await request<DueObligation[]>(
        `/reports/due-obligations?days=${encodeURIComponent(horizon)}`
      );
      setItems(response.data ?? []);
    } catch (err) {
      setError(
        err instanceof ApiError ? err.message : "Gagal memuat daftar jatuh tempo."
      );
    } finally {
      setLoading(false);
    }
  }, []);

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
          Object.fromEntries((res.data ?? []).map((c) => [c.id, c.full_name]))
        );
      })
      .catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, []);

  const overdueCount = useMemo(
    () => items.filter((item) => item.overdue).length,
    [items]
  );

  const columns: Column<DueObligation>[] = [
    {
      header: "Jenis",
      cell: (row) => kindLabel(row.kind),
    },
    {
      header: "Referensi",
      cell: (row) => (
        <span className="font-mono">
          {row.reference}
          {row.installment_no ? ` / ke-${row.installment_no}` : ""}
        </span>
      ),
    },
    {
      header: "Nasabah",
      cell: (row) =>
        customerNames[row.customer_id] ?? (
          <span className="font-mono">{row.customer_id}</span>
        ),
    },
    {
      header: "Jatuh Tempo",
      cell: (row) => formatDate(row.due_date),
      isMono: true,
    },
    {
      header: "Nominal",
      type: "money",
      cell: (row) => <MoneyText value={row.amount} />,
    },
    {
      header: "Status",
      cell: (row) => <DueStatusBadge item={row} />,
    },
  ];

  return (
    <>
      <PageHeader
        title="Jatuh Tempo"
        description="Kewajiban yang akan atau sudah jatuh tempo: angsuran kredit dan deposito berjangka, diurutkan dari tanggal terdekat."
      />

      {error && !loading ? (
        <ErrorState title="Gagal memuat daftar jatuh tempo" description={error} />
      ) : (
        <Card>
          <CardHeader>
            <CardTitle>
              Daftar Jatuh Tempo
              {overdueCount > 0 && (
                <span className="ml-2 align-middle">
                  <Badge variant="debit">{overdueCount} lewat</Badge>
                </span>
              )}
            </CardTitle>
            <div className="w-56">
              <Select
                label="Horison"
                value={days}
                onChange={(event) => setDays(event.target.value)}
                options={HORIZON_OPTIONS}
              />
            </div>
          </CardHeader>
          <CardContent className="p-0">
            <DataTable
              columns={columns}
              data={items}
              keyExtractor={(row) => `${row.kind}-${row.reference}-${row.due_date}`}
              loading={loading}
              emptyMessage="Tidak ada kewajiban yang jatuh tempo pada horison ini."
              zebra
            />
          </CardContent>
        </Card>
      )}
    </>
  );
}
