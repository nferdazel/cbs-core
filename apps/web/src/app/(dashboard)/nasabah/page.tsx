"use client";

import { useCallback, useEffect, useState } from "react";
import type { Customer } from "@cbs/shared-types";
import { ApiError, request } from "@/lib/api";
import { useAuth } from "@/lib/useAuth";
import { useTranslation } from "@/i18n/context";
import { PageHeader } from "@/components/ui/PageHeader";
import { Button } from "@/components/ui/Button";
import { Card, CardContent } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { DataTable, Column } from "@/components/ui/DataTable";
import { Pagination } from "@/components/ui/Pagination";
import { ErrorState } from "@/components/ui/States";
import {
  CustomerRegistration,
  canCreateCustomer,
} from "@/components/customer/CustomerRegistration";
import {
  AccountOpening,
  canOpenAccount,
  type SelectedCustomer,
} from "@/components/account/AccountOpening";

const PAGE_SIZE = 20;
const SEARCH_DEBOUNCE_MS = 300;

interface Meta {
  page: number;
  page_size: number;
  total_items: number;
  total_pages: number;
}

export default function NasabahPage() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const [customers, setCustomers] = useState<Customer[]>([]);
  const [meta, setMeta] = useState<Meta | null>(null);
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const [query, setQuery] = useState("");
  const [reloadKey, setReloadKey] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [formOpen, setFormOpen] = useState(false);
  const [accountCustomer, setAccountCustomer] = useState<SelectedCustomer | null>(
    null
  );

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
      const response = await request<Customer[]>(`/customers?${params.toString()}`);
      setCustomers(response.data ?? []);
      if (response.meta) setMeta(response.meta as Meta);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Gagal memuat nasabah.");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load(page, query);
  }, [page, query, reloadKey, load]);

  const canRegister = canCreateCustomer(user?.role);
  const canOpen = canOpenAccount(user?.role);

  const resetSearch = () => setSearch("");

  const columns: Column<Customer>[] = [
    { header: "CIF", accessorKey: "cif_number", isMono: true },
    { header: "Nama", accessorKey: "full_name" },
    { header: "Telepon", accessorKey: "phone_number", isMono: true },
    { header: "Email", accessorKey: "email" },
    { header: "Status", accessorKey: "status", type: "status" },
  ];

  return (
    <>
      <PageHeader
        title="Nasabah"
        description="Data induk nasabah (CIF). Nilai pribadi disimpan terenkripsi di backend."
        actions={
          canRegister ? (
            <Button
              onClick={() => {
                setFormOpen((open) => !open);
                setAccountCustomer(null);
              }}
            >
              {t.customerRegistration.openButton}
            </Button>
          ) : undefined
        }
      />

      {formOpen && (
        <CustomerRegistration
          onClose={() => setFormOpen(false)}
          onRegistered={() => setReloadKey((key) => key + 1)}
          onOpenAccount={
            canOpen
              ? (customer) => {
                  setFormOpen(false);
                  setAccountCustomer({
                    id: customer.id,
                    cifNumber: customer.cif_number,
                    fullName: customer.full_name,
                  });
                }
              : undefined
          }
        />
      )}

      {accountCustomer && (
        <AccountOpening
          initialCustomer={accountCustomer}
          onClose={() => setAccountCustomer(null)}
        />
      )}

      <Card className="mb-4">
        <CardContent>
          <div className="flex items-end gap-2">
            <div className="w-80">
              <Input
                label={t.customerList.searchLabel}
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                placeholder={t.customerList.searchPlaceholder}
                aria-describedby="nasabah-search-hint"
              />
            </div>
            {search && (
              <Button variant="secondary" onClick={resetSearch}>
                {t.customerList.clearButton}
              </Button>
            )}
          </div>
          <p id="nasabah-search-hint" className="text-meta text-ink-600">
            {t.customerList.searchHint}
          </p>
          {/* Keadaan muat/kosong/jumlah hasil diumumkan ke pembaca layar, tidak hanya
              terlihat dari warna atau teks di dalam tabel. */}
          <p className="sr-only" role="status" aria-live="polite">
            {loading
              ? t.customerList.loading
              : error
                ? ""
                : customers.length === 0
                  ? query
                    ? t.customerList.emptyFiltered
                    : t.customerList.empty
                  : `${meta?.total_items ?? customers.length} ${t.customerList.resultCount}`}
          </p>
        </CardContent>
      </Card>

      {error && !loading ? (
        <ErrorState title="Gagal memuat nasabah" description={error} />
      ) : (
        <Card>
          <CardContent className="p-0">
            <DataTable
              columns={columns}
              data={customers}
              keyExtractor={(row) => row.id}
              loading={loading}
              emptyMessage={
                query ? t.customerList.emptyFiltered : t.customerList.empty
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
