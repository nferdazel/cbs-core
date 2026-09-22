"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { AlertTriangle } from "lucide-react";
import { ApiError, request } from "@/lib/api";
import type {
  TransactionLimitRow,
  TransactionLimitsResponse,
} from "@/lib/types";
import type { Dictionary } from "@/i18n/dictionaries/id";
import { useTranslation } from "@/i18n/context";
import { Alert } from "@/components/ui/Alert";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { DataTable, Column } from "@/components/ui/DataTable";
import { ErrorState, LoadingState } from "@/components/ui/States";

type ConfiguredFilter = "ALL" | "CONFIGURED" | "UNCONFIGURED";

/** Label jenis transaksi; nilai yang belum dikenal ditampilkan apa adanya. */
function transactionTypeLabel(value: string, t: Dictionary): string {
  const known = t.limits.txType as Record<string, string>;
  return known[value] ?? value;
}

/** Asal nilai; nilai yang belum dikenal ditampilkan apa adanya. */
function sourceLabel(value: string, t: Dictionary): string {
  const known = t.limits.source as Record<string, string>;
  return known[value] ?? value;
}

/**
 * Endpoint batas transaksi bisa saja belum ada atau ditolak 403. Keduanya
 * dijelaskan apa adanya agar halaman tidak tampak kosong tanpa sebab.
 */
function loadErrorMessage(err: unknown, t: Dictionary): string {
  if (err instanceof ApiError) {
    if (err.status === 403) return t.limits.forbidden;
    if (err.status === 404) return t.limits.unavailable;
    return err.message || t.limits.loadError;
  }
  return t.limits.loadError;
}

/**
 * Batas transaksi per peran dan ambang persetujuan. `configured: false` berarti
 * nilai masih bawaan aplikasi dan belum pernah ditetapkan bank, jadi baris itu
 * ditandai teks, bukan hanya warna.
 */
export function TransactionLimits() {
  const { t } = useTranslation();
  const [data, setData] = useState<TransactionLimitsResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState<ConfiguredFilter>("ALL");

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await request<TransactionLimitsResponse>("/system/limits");
      setData(response.data ?? null);
    } catch (err) {
      setError(loadErrorMessage(err, t));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    load();
  }, [load]);

  const rows = useMemo(() => data?.limits ?? [], [data]);

  const filtered = useMemo(() => {
    const term = search.trim().toLowerCase();
    return rows.filter((row) => {
      if (statusFilter === "CONFIGURED" && !row.configured) return false;
      if (statusFilter === "UNCONFIGURED" && row.configured) return false;
      if (term) {
        const haystack =
          `${row.role} ${row.transaction_type} ${transactionTypeLabel(
            row.transaction_type,
            t
          )}`.toLowerCase();
        if (!haystack.includes(term)) return false;
      }
      return true;
    });
  }, [rows, search, statusFilter, t]);

  // Dikelompokkan per peran agar bank membaca kebijakan tiap peran sekaligus.
  const groups = useMemo(() => {
    const byRole = new Map<string, TransactionLimitRow[]>();
    for (const row of filtered) {
      const list = byRole.get(row.role);
      if (list) list.push(row);
      else byRole.set(row.role, [row]);
    }
    return Array.from(byRole.entries())
      .sort(([a], [b]) => a.localeCompare(b, "id"))
      .map(([role, groupRows]) => ({
        role,
        rows: [...groupRows].sort((a, b) =>
          a.transaction_type.localeCompare(b.transaction_type, "id")
        ),
      }));
  }, [filtered]);

  const unconfiguredCount = useMemo(
    () => rows.filter((row) => !row.configured).length,
    [rows]
  );

  const columns: Column<TransactionLimitRow>[] = [
    {
      header: t.limits.colTransactionType,
      cell: (row) => transactionTypeLabel(row.transaction_type, t),
    },
    {
      header: t.limits.colPerTransaction,
      type: "money",
      accessorKey: "per_transaction",
    },
    {
      header: t.limits.colDailyLimit,
      type: "money",
      accessorKey: "daily_limit",
    },
    {
      header: t.limits.colApprovalAbove,
      type: "money",
      accessorKey: "approval_above",
    },
    {
      header: t.limits.colStatus,
      cell: (row) =>
        row.configured ? (
          <Badge variant="credit">{t.limits.configured}</Badge>
        ) : (
          // "Masih bawaan" = perlu perhatian, bukan kegagalan; kuning (accent),
          // konsisten dengan notis peringatan di atas tabel.
          <Badge variant="accent">
            <AlertTriangle className="mr-1 h-3 w-3" aria-hidden />
            {t.limits.unconfigured}
          </Badge>
        ),
    },
  ];

  return (
    <Card className="mb-4">
      <CardHeader>
        <CardTitle>{t.limits.title}</CardTitle>
      </CardHeader>
      <CardContent>
        {loading ? (
          <LoadingState label={t.states.loading} />
        ) : error ? (
          <ErrorState
            title={t.limits.title}
            description={error}
            action={
              <Button variant="secondary" onClick={load}>
                {t.common.retry}
              </Button>
            }
          />
        ) : (
          <>
            <p className="text-body text-ink-600">{t.limits.description}</p>
            <p className="text-meta text-ink-600">
              {t.limits.sourceLabel}:{" "}
              <span className="font-mono">
                {data?.source ? sourceLabel(data.source, t) : "-"}
              </span>
              {data?.source ? ` (${data.source})` : ""}
            </p>

            {unconfiguredCount > 0 ? (
              <Alert variant="warning">
                {t.limits.unconfiguredNotice}
              </Alert>
            ) : (
              <p className="text-body text-credit-700" role="status">
                {t.limits.allConfigured}
              </p>
            )}

            <div className="flex flex-wrap items-end gap-4">
              <div className="w-72">
                <Input
                  label={t.limits.searchLabel}
                  value={search}
                  onChange={(event) => setSearch(event.target.value)}
                  placeholder={t.limits.searchPlaceholder}
                />
              </div>
              <div className="w-56">
                <Select
                  label={t.limits.statusFilterLabel}
                  value={statusFilter}
                  onChange={(event) =>
                    setStatusFilter(event.target.value as ConfiguredFilter)
                  }
                  options={[
                    { value: "ALL", label: t.limits.filterAll },
                    { value: "CONFIGURED", label: t.limits.filterConfigured },
                    { value: "UNCONFIGURED", label: t.limits.filterUnconfigured },
                  ]}
                />
              </div>
            </div>

            <p className="sr-only" role="status" aria-live="polite">
              {filtered.length} {t.limits.resultCount}
            </p>

            {groups.length === 0 ? (
              <p className="text-body text-ink-600">
                {rows.length === 0 ? t.limits.empty : t.limits.emptyFiltered}
              </p>
            ) : (
              groups.map((group) => (
                <div key={group.role} className="space-y-2">
                  <h4 className="text-title font-semibold text-ink-900">
                    <span className="font-normal text-ink-600">
                      {t.limits.colRole}:{" "}
                    </span>
                    <span className="font-mono">{group.role}</span>
                  </h4>
                  <DataTable
                    columns={columns}
                    data={group.rows}
                    keyExtractor={(row) => `${row.role}/${row.transaction_type}`}
                    emptyMessage={t.limits.empty}
                    zebra
                  />
                </div>
              ))
            )}
          </>
        )}
      </CardContent>
    </Card>
  );
}
