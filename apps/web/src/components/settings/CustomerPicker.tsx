"use client";

import { useCallback, useState } from "react";
import type { Customer } from "@cbs/shared-types";
import { ApiError, request } from "@/lib/api";
import { useTranslation } from "@/i18n/context";
import { Alert } from "@/components/ui/Alert";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";

export interface CustomerPickerProps {
  /** Nasabah terpilih; null berarti belum ada. */
  selected: Customer | null;
  onSelect: (customer: Customer | null) => void;
  /** Nonaktifkan seluruh kontrol (mis. pengguna tidak berhak mengubah). */
  disabled?: boolean;
}

/**
 * Pencarian nasabah lewat nomor CIF untuk mengisi customer_id pada data OJK yang
 * merujuk nasabah (lawan transaksi off-balance, pihak terkait, dan batas BMPK).
 * Memakai endpoint GET /customers yang sudah ada; pemilihan mengembalikan objek
 * Customer agar pemanggil dapat menampilkan CIF dan namanya.
 */
export function CustomerPicker({
  selected,
  onSelect,
  disabled = false,
}: CustomerPickerProps) {
  const { t } = useTranslation();
  const [term, setTerm] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const search = useCallback(async () => {
    const query = term.trim();
    if (!query) {
      setError(t.ojkData.customer.required);
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const params = new URLSearchParams({
        q: query,
        page: "1",
        page_size: "5",
      });
      const response = await request<Customer[]>(
        `/customers?${params.toString()}`,
      );
      const found = (response.data ?? [])[0];
      if (!found) {
        onSelect(null);
        setError(t.ojkData.customer.notFound);
        return;
      }
      onSelect(found);
    } catch (err) {
      onSelect(null);
      setError(
        err instanceof ApiError ? err.message : t.ojkData.customer.searchError,
      );
    } finally {
      setLoading(false);
    }
  }, [term, onSelect, t]);

  return (
    <div className="space-y-2">
      <div className="flex items-end gap-2">
        <div className="w-72">
          <Input
            label={t.ojkData.customer.searchLabel}
            helperText={t.ojkData.customer.searchHint}
            placeholder={t.ojkData.customer.searchPlaceholder}
            value={term}
            disabled={disabled}
            onChange={(event) => setTerm(event.target.value)}
          />
        </div>
        <Button
          variant="secondary"
          onClick={search}
          loading={loading}
          disabled={disabled}
        >
          {loading ? t.ojkData.customer.searching : t.ojkData.customer.search}
        </Button>
        {selected && (
          <Button
            variant="ghost"
            disabled={disabled}
            onClick={() => {
              onSelect(null);
              setTerm("");
              setError(null);
            }}
          >
            {t.ojkData.customer.clear}
          </Button>
        )}
      </div>
      {error && <Alert variant="error">{error}</Alert>}
      {selected && (
        <p className="text-body text-ink-900">
          {t.ojkData.customer.selected}:{" "}
          <span className="font-mono">{selected.cif_number}</span>{" "}
          {selected.full_name}
        </p>
      )}
    </div>
  );
}
