"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import type { AccountStatus } from "@cbs/shared-types";
import { ApiError, request } from "@/lib/api";
import { formatDateTime } from "@/lib/format";
import type { AccountRecord } from "@/lib/types";
import { useAuth } from "@/lib/useAuth";
import { useTranslation } from "@/i18n/context";
import { PageHeader } from "@/components/ui/PageHeader";
import { Button } from "@/components/ui/Button";
import { Card, CardContent } from "@/components/ui/Card";
import { DataTable, Column } from "@/components/ui/DataTable";
import { Input } from "@/components/ui/Input";
import { MoneyText } from "@/components/ui/MoneyText";
import { Pagination } from "@/components/ui/Pagination";
import { Select } from "@/components/ui/Select";
import { ErrorState } from "@/components/ui/States";
import {
  AccountReactivation,
  canReactivateAccount,
} from "@/components/account/AccountReactivation";
import {
  AccountOpening,
  canOpenAccount,
} from "@/components/account/AccountOpening";

const PAGE_SIZE = 20;
const SEARCH_DEBOUNCE_MS = 300;

type StatusFilter = AccountStatus | "ALL";

const STATUS_OPTIONS = [
  { value: "ALL", label: "Semua status" },
  { value: "ACTIVE", label: "Aktif" },
  { value: "DORMANT", label: "Dormant" },
  { value: "FROZEN", label: "Dibekukan" },
  { value: "CLOSED", label: "Ditutup" },
];

interface Meta {
  page: number;
  page_size: number;
  total_items: number;
  total_pages: number;
}

export default function RekeningPage() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const [accounts, setAccounts] = useState<AccountRecord[]>([]);
  const [meta, setMeta] = useState<Meta | null>(null);
  const [page, setPage] = useState(1);
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("ALL");
  const [search, setSearch] = useState("");
  const [query, setQuery] = useState("");
  const [reloadKey, setReloadKey] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  const [formOpen, setFormOpen] = useState(false);

  // Debounce: kata kunci baru dikirim setelah pengguna berhenti mengetik.
  useEffect(() => {
    const timer = setTimeout(() => {
      setQuery(search.trim());
      setPage(1);
    }, SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(timer);
  }, [search]);

  const load = useCallback(async (targetPage: number, term: string) => {
    setLoading(true);
    setError(null);
    try {
      const params = new URLSearchParams({
        page: String(targetPage),
        page_size: String(PAGE_SIZE),
      });
      if (term) params.set("q", term);
      const response = await request<AccountRecord[]>(`/accounts?${params.toString()}`);
      setAccounts(response.data ?? []);
      if (response.meta) setMeta(response.meta as Meta);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Gagal memuat rekening.");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load(page, query);
  }, [page, query, reloadKey, load]);

  // API /accounts belum menerima parameter status, jadi penyaringan dilakukan di
  // sisi klien dan hanya mencakup halaman yang sedang dimuat.
  const filteredAccounts = useMemo(
    () =>
      statusFilter === "ALL"
        ? accounts
        : accounts.filter((account) => account.status === statusFilter),
    [accounts, statusFilter]
  );

  const authorized = canReactivateAccount(user?.role);
  const canOpen = canOpenAccount(user?.role);

  const handleReactivated = (updated: AccountRecord) => {
    // Perbarui baris di tempat, lalu muat ulang halaman agar sinkron dengan server.
    setAccounts((prev) =>
      prev.map((account) => (account.id === updated.id ? updated : account))
    );
    setSuccessMessage(
      `Rekening ${updated.account_number} berhasil direaktivasi. Status kini ACTIVE.`
    );
    load(page, query);
  };

  const handleOpened = () => {
    setPage(1);
    setReloadKey((key) => key + 1);
  };

  const resetSearch = () => setSearch("");

  const columns: Column<AccountRecord>[] = [
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
    {
      header: "Aktivitas Terakhir",
      cell: (row) =>
        row.last_activity_at ? formatDateTime(row.last_activity_at) : "-",
      isMono: true,
    },
  ];

  if (authorized) {
    columns.push({
      header: "Aksi",
      align: "right",
      cell: (row) => (
        <AccountReactivation account={row} onReactivated={handleReactivated} />
      ),
    });
  }

  return (
    <>
      <PageHeader
        title="Rekening"
        description="Direktori rekening nasabah: tabungan, giro, dan kredit. Akun buku besar internal tidak ditampilkan di sini."
        actions={
          canOpen ? (
            <Button onClick={() => setFormOpen((open) => !open)}>
              {t.accountOpening.openButton}
            </Button>
          ) : undefined
        }
      />

      {successMessage && (
        <div
          role="status"
          className="mb-4 rounded-md border border-credit-700/30 bg-credit-50 px-4 py-3 text-body text-credit-700"
        >
          {successMessage}
        </div>
      )}

      {formOpen && (
        <AccountOpening
          onClose={() => setFormOpen(false)}
          onOpened={handleOpened}
        />
      )}

      <Card className="mb-4">
        <CardContent className="flex flex-wrap items-end gap-4">
          <div className="w-80">
            <Input
              label={t.accountList.searchLabel}
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder={t.accountList.searchPlaceholder}
              aria-describedby="rekening-search-hint"
            />
          </div>
          {search && (
            <Button variant="secondary" onClick={resetSearch}>
              {t.accountList.clearButton}
            </Button>
          )}
          <div className="w-56">
            <Select
              label="Filter Status"
              value={statusFilter}
              onChange={(event) =>
                setStatusFilter(event.target.value as StatusFilter)
              }
              options={STATUS_OPTIONS}
            />
          </div>
          <p className="pb-2 text-meta text-ink-600">
            Menampilkan {filteredAccounts.length} dari {accounts.length} rekening
            di halaman ini. API belum mendukung filter status, jadi penyaringan
            hanya berlaku per halaman.
          </p>
        </CardContent>
        <p
          id="rekening-search-hint"
          className="border-t border-border px-4 py-2 text-meta text-ink-600"
        >
          {t.accountList.searchHint}
        </p>
      </Card>

      {error && !loading ? (
        <ErrorState title="Gagal memuat rekening" description={error} />
      ) : (
        <Card>
          <CardContent className="p-0">
            <DataTable
              columns={columns}
              data={filteredAccounts}
              keyExtractor={(row) => row.id}
              loading={loading}
              emptyMessage={
                query
                  ? t.accountList.emptyFiltered
                  : statusFilter === "ALL"
                    ? t.accountList.empty
                    : "Tidak ada rekening dengan status ini pada halaman ini."
              }
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
