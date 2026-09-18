"use client";

import { useCallback, useEffect, useState } from "react";
import type { Account } from "@cbs/shared-types";
import { ApiError, request } from "@/lib/api";
import { PageHeader } from "@/components/ui/PageHeader";
import { Card, CardContent } from "@/components/ui/Card";
import { DataTable, Column } from "@/components/ui/DataTable";
import { MoneyText } from "@/components/ui/MoneyText";
import { Pagination } from "@/components/ui/Pagination";
import { ErrorState } from "@/components/ui/States";

const PAGE_SIZE = 20;

interface Meta {
  page: number;
  page_size: number;
  total_items: number;
  total_pages: number;
}

export default function RekeningPage() {
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [meta, setMeta] = useState<Meta | null>(null);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async (targetPage: number) => {
    setLoading(true);
    setError(null);
    try {
      const response = await request<Account[]>(
        `/accounts?page=${targetPage}&page_size=${PAGE_SIZE}`
      );
      setAccounts(response.data ?? []);
      if (response.meta) setMeta(response.meta as Meta);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Gagal memuat rekening.");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load(page);
  }, [page, load]);

  const columns: Column<Account>[] = [
    { header: "Nomor Rekening", accessorKey: "account_number", isMono: true },
    { header: "Pemilik", cell: (row) => row.customer_name || "-" },
    { header: "Tipe", accessorKey: "account_type" },
    { header: "Mata Uang", accessorKey: "currency", isMono: true },
    {
      header: "Saldo",
      type: "money",
      cell: (row) => <MoneyText value={row.balance} />,
    },
    { header: "Status", accessorKey: "status", type: "status" },
  ];

  return (
    <>
      <PageHeader
        title="Rekening"
        description="Direktori rekening nasabah: tabungan, giro, dan kredit. Akun buku besar internal tidak ditampilkan di sini."
      />
      {error && !loading ? (
        <ErrorState title="Gagal memuat rekening" description={error} />
      ) : (
        <Card>
          <CardContent className="p-0">
            <DataTable
              columns={columns}
              data={accounts}
              keyExtractor={(row) => row.id}
              loading={loading}
              emptyMessage="Belum ada rekening."
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
