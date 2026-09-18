"use client";

import { useCallback, useEffect, useState } from "react";
import type { JournalLine } from "@cbs/shared-types";
import { ApiError, isCrossBranchError, request } from "@/lib/api";
import { formatDateTime } from "@/lib/format";
import { PageHeader } from "@/components/ui/PageHeader";
import { Card, CardContent } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { DataTable, Column } from "@/components/ui/DataTable";
import { MoneyText } from "@/components/ui/MoneyText";
import { Badge } from "@/components/ui/Badge";
import { Pagination } from "@/components/ui/Pagination";
import { ErrorState, EmptyState } from "@/components/ui/States";

const PAGE_SIZE = 25;

interface Meta {
  page: number;
  page_size: number;
  total_items: number;
  total_pages: number;
}

export default function TransaksiPage() {
  const [accountInput, setAccountInput] = useState("");
  const [account, setAccount] = useState<string | null>(null);
  const [lines, setLines] = useState<JournalLine[]>([]);
  const [meta, setMeta] = useState<Meta | null>(null);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [forbidden, setForbidden] = useState(false);

  const load = useCallback(async (accNumber: string, targetPage: number) => {
    setLoading(true);
    setError(null);
    setForbidden(false);
    try {
      const response = await request<JournalLine[]>(
        `/accounts/${encodeURIComponent(accNumber)}/statements?page=${targetPage}&page_size=${PAGE_SIZE}`
      );
      setLines(response.data ?? []);
      if (response.meta) setMeta(response.meta as Meta);
    } catch (err) {
      // Rekening cabang lain dibalas 403, bukan daftar mutasi kosong. Tampilkan
      // sebabnya agar tidak disalahartikan sebagai "tidak ada transaksi".
      setForbidden(isCrossBranchError(err));
      setError(err instanceof ApiError ? err.message : "Gagal memuat mutasi rekening.");
      setLines([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (account) load(account, page);
  }, [account, page, load]);

  const handleSearch = (event: React.FormEvent) => {
    event.preventDefault();
    const trimmed = accountInput.trim();
    if (!trimmed) return;
    setPage(1);
    setAccount(trimmed);
  };

  const columns: Column<JournalLine>[] = [
    {
      header: "Waktu",
      cell: (row) => formatDateTime(row.created_at),
      isMono: true,
    },
    {
      header: "Arah",
      cell: (row) => (
        <Badge variant={row.direction === "DEBIT" ? "debit" : "credit"}>
          {row.direction}
        </Badge>
      ),
    },
    {
      header: "Nominal",
      type: "money",
      cell: (row) => (
        <MoneyText
          value={row.amount}
          tone={row.direction === "DEBIT" ? "debit" : "credit"}
        />
      ),
    },
    {
      header: "Saldo Setelah",
      type: "money",
      cell: (row) => <MoneyText value={row.balance_after} />,
    },
    { header: "Keterangan", accessorKey: "description" },
    { header: "Referensi", accessorKey: "journal_entry_id", isMono: true },
  ];

  return (
    <>
      <PageHeader
        title="Transaksi"
        description="Telusuri mutasi satu rekening dari buku besar. Masukkan nomor rekening untuk melihat."
      />

      <Card className="mb-4">
        <CardContent>
          <form onSubmit={handleSearch} className="flex items-end gap-3">
            <div className="w-72">
              <Input
                label="Nomor Rekening"
                value={accountInput}
                onChange={(e) => setAccountInput(e.target.value)}
                isMono
                placeholder="mis. 102601020001"
              />
            </div>
            <Button type="submit" loading={loading && account !== null}>
              Tampilkan Mutasi
            </Button>
          </form>
        </CardContent>
      </Card>

      {error && !loading ? (
        <ErrorState
          title={
            forbidden
              ? "Akses lintas cabang ditolak"
              : "Gagal memuat mutasi"
          }
          description={error}
        />
      ) : account === null ? (
        <EmptyState
          title="Belum ada rekening dipilih"
          description="Masukkan nomor rekening lalu tekan Tampilkan Mutasi."
        />
      ) : (
        <Card>
          <CardContent className="p-0">
            <DataTable
              columns={columns}
              data={lines}
              keyExtractor={(row) => row.id}
              loading={loading}
              emptyMessage="Tidak ada mutasi untuk rekening ini."
              zebra
            />
            {meta && (
              <Pagination
                page={meta.page}
                pageSize={meta.page_size}
                totalItems={meta.total_items}
                totalPages={meta.total_pages}
                onPageChange={(next) => setPage(next)}
              />
            )}
          </CardContent>
        </Card>
      )}
    </>
  );
}
