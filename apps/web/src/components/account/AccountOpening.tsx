"use client";

import { useEffect, useState } from "react";
import type { Customer } from "@cbs/shared-types";
import { ApiError, newIdempotencyKey, request, unwrap } from "@/lib/api";
import type { AccountRecord } from "@/lib/types";
import type { BankingProduct } from "@/lib/operations-types";
import { useAuth } from "@/lib/useAuth";
import { useTranslation } from "@/i18n/context";
import { Alert } from "@/components/ui/Alert";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { StatusBadge } from "@/components/ui/StatusBadge";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";

/** Keluarga produk yang dibuka lewat POST /accounts/open, bukan lewat deposito/kredit. */
const OPENABLE_FAMILIES = new Set(["SAVINGS", "CURRENT_ACCOUNT"]);

export interface SelectedCustomer {
  id: string;
  cifNumber: string;
  fullName: string;
}

export interface AccountOpeningProps {
  /** Nasabah yang sudah terpilih sebelum formulir dibuka (mis. dari pendaftaran). */
  initialCustomer?: SelectedCustomer | null;
  /** Menutup formulir tanpa membuka rekening. */
  onClose: () => void;
  /** Dipanggil setelah API membalas sukses, untuk menyegarkan daftar rekening. */
  onOpened?: (account: AccountRecord) => void;
}

/**
 * Form pembukaan rekening. Field mengikuti domain.OpenAccountInput; branch_code
 * tidak dikirim karena service mengambil cabang dari JWT, bukan body. Pemilihan
 * nasabah memakai pencarian yang sama dengan daftar nasabah (q = CIF awalan atau
 * NIK persis), bukan memuat 100 nasabah lalu menyuruh pengguna menggulir.
 */
export function AccountOpening({
  initialCustomer,
  onClose,
  onOpened,
}: AccountOpeningProps) {
  const { t } = useTranslation();
  const { user } = useAuth();

  const [selected, setSelected] = useState<SelectedCustomer | null>(
    initialCustomer ?? null
  );
  const [searchTerm, setSearchTerm] = useState("");
  const [results, setResults] = useState<Customer[]>([]);
  const [searching, setSearching] = useState(false);
  const [searchError, setSearchError] = useState<string | null>(null);

  const [products, setProducts] = useState<BankingProduct[]>([]);
  const [productId, setProductId] = useState("");
  const [productsLoading, setProductsLoading] = useState(true);
  const [productError, setProductError] = useState<string | null>(null);
  const [productForbidden, setProductForbidden] = useState(false);

  const [currency, setCurrency] = useState("IDR");
  const [formError, setFormError] = useState<string | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [opened, setOpened] = useState<AccountRecord | null>(null);

  // Pencarian nasabah: debounce 300ms agar setiap ketikan tidak memicu permintaan.
  useEffect(() => {
    const term = searchTerm.trim();
    if (term.length < 2) {
      setResults([]);
      setSearching(false);
      setSearchError(null);
      return;
    }

    let cancelled = false;
    setSearching(true);
    const timer = setTimeout(async () => {
      try {
        const response = await request<Customer[]>(
          `/customers?page=1&page_size=8&q=${encodeURIComponent(term)}`
        );
        if (cancelled) return;
        setResults(response.data ?? []);
        setSearchError(null);
      } catch (err) {
        if (cancelled) return;
        setResults([]);
        setSearchError(
          err instanceof ApiError ? err.message : t.accountOpening.searchError
        );
      } finally {
        if (!cancelled) setSearching(false);
      }
    }, 300);

    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [searchTerm]);

  // Produk diambil sekali. Hanya keluarga simpanan/giro yang bisa dibuka di sini;
  // deposito berjangka punya alur sendiri dan kredit lewat pengajuan kredit.
  useEffect(() => {
    let cancelled = false;
    request<BankingProduct[]>("/products")
      .then((response) => {
        if (cancelled) return;
        setProducts(
          (response.data ?? []).filter((product) =>
            OPENABLE_FAMILIES.has(product.family)
          )
        );
        setProductError(null);
        setProductForbidden(false);
      })
      .catch((err) => {
        if (cancelled) return;
        setProductError(
          err instanceof ApiError ? err.message : t.accountOpening.productLoadError
        );
        setProductForbidden(err instanceof ApiError && err.status === 403);
      })
      .finally(() => {
        if (!cancelled) setProductsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const selectedProduct = products.find((product) => product.id === productId);
  const branchCode = user?.branch_code || "-";

  const validate = (): string | null => {
    if (!selected) return t.accountOpening.requiredCustomer;
    if (!productId) return t.accountOpening.requiredProduct;
    return null;
  };

  const openConfirm = (event: React.FormEvent) => {
    event.preventDefault();
    const error = validate();
    if (error) {
      setFormError(error);
      return;
    }
    setFormError(null);
    setConfirmOpen(true);
  };

  const submit = async () => {
    if (!selected) return;
    setSubmitting(true);
    setFormError(null);
    try {
      const response = await request<AccountRecord>("/accounts/open", {
        method: "POST",
        idempotencyKey: newIdempotencyKey(),
        body: {
          customer_id: selected.id,
          product_id: productId,
          currency,
        },
      });
      const account = unwrap(response);
      setConfirmOpen(false);
      setOpened(account);
      onOpened?.(account);
    } catch (err) {
      setFormError(
        err instanceof ApiError ? err.message : t.accountOpening.openError
      );
      setConfirmOpen(false);
    } finally {
      setSubmitting(false);
    }
  };

  const resetForNew = () => {
    setOpened(null);
    setSelected(null);
    setSearchTerm("");
    setResults([]);
    setSearchError(null);
    setProductId("");
    setFormError(null);
  };

  const customerLabel = selected
    ? `${selected.cifNumber} — ${selected.fullName}`
    : "-";

  return (
    <Card className="mb-4">
      <CardHeader>
        <CardTitle>{t.accountOpening.title}</CardTitle>
        <Button variant="ghost" size="sm" onClick={onClose}>
          {t.accountOpening.closeButton}
        </Button>
      </CardHeader>
      <CardContent>
        {opened ? (
          <div className="space-y-4">
            <Alert variant="success" title={t.accountOpening.successTitle}>
              <p className="mt-2 text-meta text-ink-600">
                {t.accountOpening.successNumberLabel}
              </p>
              <p className="font-mono text-page text-ink-900">
                {opened.account_number}
              </p>
              <p className="mt-1 text-body text-ink-900">
                {customerLabel}
              </p>
              <p className="mt-1 text-meta text-ink-600">
                {t.accountOpening.successHint}
              </p>
            </Alert>
            <div className="flex gap-2">
              <Button onClick={resetForNew}>{t.accountOpening.againButton}</Button>
              <Button variant="secondary" onClick={onClose}>
                {t.accountOpening.closeButton}
              </Button>
            </div>
          </div>
        ) : (
          <form onSubmit={openConfirm} className="space-y-4">
            <p className="text-meta text-ink-600">{t.accountOpening.description}</p>

            {selected ? (
              <div className="space-y-1">
                <span className="block text-meta font-medium text-ink-600">
                  {t.accountOpening.selectedCustomer}
                </span>
                <div className="flex items-center justify-between gap-4 rounded-md border border-border bg-canvas px-3 py-2">
                  <div className="min-w-0">
                    <span className="font-mono text-body text-ink-900">
                      {selected.cifNumber}
                    </span>
                    <span className="ml-2 text-body text-ink-900">
                      {selected.fullName}
                    </span>
                  </div>
                  <Button
                    type="button"
                    variant="secondary"
                    size="sm"
                    onClick={() => setSelected(null)}
                  >
                    {t.accountOpening.changeCustomer}
                  </Button>
                </div>
              </div>
            ) : (
              <div className="space-y-1">
                <div className="flex items-end gap-2">
                  <div className="w-80">
                    <Input
                      label={t.accountOpening.searchCustomerLabel}
                      value={searchTerm}
                      onChange={(event) => setSearchTerm(event.target.value)}
                      placeholder={t.accountOpening.searchCustomerPlaceholder}
                      autoFocus
                      aria-describedby="account-customer-search-hint"
                    />
                  </div>
                </div>
                <p id="account-customer-search-hint" className="text-meta text-ink-600">
                  {t.accountOpening.searchCustomerHint}
                </p>

                {searching && (
                  <p className="text-meta text-ink-600" role="status">
                    {t.accountOpening.searching}
                  </p>
                )}
                {searchError && (
                  <p className="text-meta text-debit-700" role="alert" aria-live="assertive">
                    {searchError}
                  </p>
                )}
                {!searching &&
                  !searchError &&
                  searchTerm.trim().length >= 2 &&
                  results.length === 0 && (
                    <p className="text-meta text-ink-600" role="status">
                      {t.accountOpening.searchEmpty}
                    </p>
                  )}
                {searchTerm.trim().length > 0 && searchTerm.trim().length < 2 && (
                  <p className="text-meta text-ink-600" role="status">
                    {t.accountOpening.searchMin}
                  </p>
                )}

                {results.length > 0 && (
                  <ul className="divide-y divide-border rounded-md border border-border">
                    {results.map((customer) => (
                      <li key={customer.id}>
                        <button
                          type="button"
                          onClick={() =>
                            setSelected({
                              id: customer.id,
                              cifNumber: customer.cif_number,
                              fullName: customer.full_name,
                            })
                          }
                          className="flex w-full items-center justify-between gap-4 px-3 py-2 text-left hover:bg-canvas focus:bg-canvas focus:outline-none focus:ring-1 focus:ring-navy-600"
                        >
                          <span className="min-w-0">
                            <span className="font-mono text-body text-ink-900">
                              {customer.cif_number}
                            </span>
                            <span className="ml-2 text-body text-ink-900">
                              {customer.full_name}
                            </span>
                          </span>
                          <StatusBadge status={customer.status} domain="customer" />
                        </button>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            )}

            <div className="grid grid-cols-2 gap-4">
              <Select
                label={t.accountOpening.product}
                value={productId}
                onChange={(event) => setProductId(event.target.value)}
                options={products.map((product) => ({
                  value: product.id,
                  label: `${product.code} — ${product.name}`,
                }))}
                placeholder={
                  productsLoading
                    ? t.accountOpening.productLoading
                    : products.length === 0
                      ? t.accountOpening.noProducts
                      : t.accountOpening.productPlaceholder
                }
                disabled={productsLoading || products.length === 0}
              />
              <Select
                label={t.accountOpening.currency}
                value={currency}
                onChange={(event) => setCurrency(event.target.value)}
                options={[{ value: "IDR", label: t.accountOpening.currencyIdr }]}
                helperText={t.accountOpening.currencyHint}
              />
            </div>

            {productError && (
              <Alert
                variant="error"
                title={`${t.accountOpening.productLoadError} ${productError}`}
              >
                {productForbidden && (
                  <p className="mt-1 text-meta">
                    {t.accountOpening.productForbidden}
                  </p>
                )}
              </Alert>
            )}

            <div className="w-80 space-y-1">
              <span className="block text-meta font-medium text-ink-600">
                {t.accountOpening.branch}
              </span>
              <p className="flex h-9 items-center rounded-md border border-border bg-canvas px-3 font-mono text-body text-ink-900">
                {branchCode}
              </p>
              <p className="text-meta text-ink-600">{t.accountOpening.branchHint}</p>
            </div>

            {formError && (
              <p className="text-meta text-debit-700" role="alert" aria-live="assertive">
                {formError}
              </p>
            )}

            <Button type="submit">{t.accountOpening.submit}</Button>
          </form>
        )}
      </CardContent>

      <ConfirmDialog
        open={confirmOpen}
        title={t.accountOpening.confirmTitle}
        loading={submitting}
        confirmLabel={t.accountOpening.openButton}
        onCancel={() => setConfirmOpen(false)}
        onConfirm={submit}
        description={
          <>
            <p>{t.accountOpening.confirmDescription}</p>
            <dl className="mt-2 space-y-1">
              <div className="flex justify-between gap-4">
                <dt className="text-ink-600">{t.accountOpening.customer}</dt>
                <dd className="text-right text-ink-900">{customerLabel}</dd>
              </div>
              <div className="flex justify-between gap-4">
                <dt className="text-ink-600">{t.accountOpening.product}</dt>
                <dd className="text-right text-ink-900">
                  {selectedProduct
                    ? `${selectedProduct.code} — ${selectedProduct.name}`
                    : "-"}
                </dd>
              </div>
              <div className="flex justify-between gap-4">
                <dt className="text-ink-600">{t.accountOpening.currency}</dt>
                <dd className="font-mono text-ink-900">{currency}</dd>
              </div>
              <div className="flex justify-between gap-4">
                <dt className="text-ink-600">{t.accountOpening.branch}</dt>
                <dd className="font-mono text-ink-900">{branchCode}</dd>
              </div>
            </dl>
          </>
        }
      />
    </Card>
  );
}
