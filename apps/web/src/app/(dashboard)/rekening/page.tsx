"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import type { AccountStatus } from "@cbs/shared-types";
import { ApiError, request } from "@/lib/api";
import { formatDateTime } from "@/lib/format";
import type { AccountRecord } from "@/lib/types";
import { useAuth } from "@/lib/useAuth";
import { hasPermission } from "@/lib/permissions";
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
import { AccountReactivation } from "@/components/account/AccountReactivation";
import { AccountOpening } from "@/components/account/AccountOpening";

const PAGE_SIZE = 20;
const SEARCH_DEBOUNCE_MS = 300;

type StatusFilter = AccountStatus | "ALL";

interface Meta {
  page: number;
  page_size: number;
  total_items: number;
  total_pages: number;
}

export default function RekeningPage() {
  const { t } = useTranslation();
  const { user } = useAuth();

  // Status disimpan sebagai kunci teknis; hanya label tampilannya dari kamus.
  const statusOptions = useMemo(
    () => [
      { value: "ALL", label: t.accountPage.statusAll },
      { value: "ACTIVE", label: t.accountPage.statusActive },
      { value: "DORMANT", label: t.accountPage.statusDormant },
      { value: "FROZEN", label: t.accountPage.statusFrozen },
      { value: "CLOSED", label: t.accountPage.statusClosed },
    ],
    [t]
  );

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
      setError(err instanceof ApiError ? err.message : t.accountPage.loadError);
    } finally {
      setLoading(false);
    }
  }, [t]);

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

  // Izin efektif dari GET /auth/me, bukan peran. API tetap penjaga sebenarnya
  // (accounts:freeze untuk reaktivasi, accounts:open untuk pembukaan rekening).
  const authorized = hasPermission(user, "accounts:freeze");
  const canOpen = hasPermission(user, "accounts:open");

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
    { header: t.accountPage.colAccountNumber, accessorKey: "account_number", isMono: true },
    { header: t.accountPage.colOwner, cell: (row) => row.customer_name || "-" },
    { header: t.accountPage.colType, accessorKey: "account_type" },
    { header: t.accountPage.colCurrency, accessorKey: "currency", isMono: true },
    {
      header: t.accountPage.colBalance,
      type: "money",
      cell: (row) => <MoneyText value={row.balance} />,
    },
    { header: t.common.status, accessorKey: "status", type: "status" },
    {
      header: t.accountPage.colLastActivity,
      cell: (row) =>
        row.last_activity_at ? formatDateTime(row.last_activity_at) : "-",
      isMono: true,
    },
  ];

  if (authorized) {
    columns.push({
      header: t.common.actions,
      align: "right",
      cell: (row) => (
        <AccountReactivation account={row} onReactivated={handleReactivated} />
      ),
    });
  }

  return (
    <>
      <PageHeader
        title={t.accountPage.title}
        description={t.accountPage.description}
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
              label={t.accountPage.filterStatus}
              value={statusFilter}
              onChange={(event) =>
                setStatusFilter(event.target.value as StatusFilter)
              }
              options={statusOptions}
            />
          </div>
          <p className="pb-2 text-meta text-ink-600">
            {t.accountPage.showingPrefix} {filteredAccounts.length}{" "}
            {t.common.of} {accounts.length} {t.accountPage.showingSuffix}
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
        <ErrorState title={t.accountPage.errorTitle} description={error} />
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
                    : t.accountPage.emptyFilteredStatus
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
