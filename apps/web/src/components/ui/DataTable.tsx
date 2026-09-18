import React from "react";
import { MoneyText } from "./MoneyText";
import { StatusBadge } from "./StatusBadge";

export type DataTableColumnType =
  | "text"
  | "money"
  | "account"
  | "date"
  | "status";

export interface Column<T> {
  header: string;
  accessorKey?: keyof T;
  cell?: (row: T, index: number) => React.ReactNode;
  /** Tipe bawaan menentukan render & alignment. */
  type?: DataTableColumnType;
  align?: "left" | "center" | "right";
  width?: string;
  isMono?: boolean;
}

export interface DataTableProps<T> {
  columns: Column<T>[];
  data: T[];
  keyExtractor: (row: T, index: number) => string | number;
  loading?: boolean;
  error?: string | null;
  emptyMessage?: string;
  /** Baris zebra untuk tabel panjang. */
  zebra?: boolean;
}

const alignClass = (align?: Column<unknown>["align"]) =>
  align === "right" ? "text-right" : align === "center" ? "text-center" : "text-left";

/**
 * Tabel data dengan header sticky dan state kosong/memuat/galat di dalam tabel.
 * Kolom `money` otomatis rata kanan dan memakai MoneyText.
 */
export function DataTable<T>({
  columns,
  data,
  keyExtractor,
  loading = false,
  error = null,
  emptyMessage = "Tidak ada data.",
  zebra = false,
}: DataTableProps<T>) {
  return (
    <div className="w-full overflow-x-auto rounded-md border border-border bg-surface">
      <table className="w-full border-collapse text-left text-body">
        <thead className="sticky top-0 bg-canvas text-meta uppercase tracking-wide text-ink-600">
          <tr>
            {columns.map((col, idx) => (
              <th
                key={idx}
                scope="col"
                style={col.width ? { width: col.width } : undefined}
                className={`border-b border-border px-3 py-2 font-medium ${
                  col.type === "money" ? "text-right" : alignClass(col.align)
                }`}
              >
                {col.header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {loading ? (
            <tr>
              <td colSpan={columns.length} className="px-3 py-8 text-center text-ink-600">
                Memuat data...
              </td>
            </tr>
          ) : error ? (
            <tr>
              <td
                colSpan={columns.length}
                className="px-3 py-8 text-center text-debit-700"
                role="alert"
              >
                {error}
              </td>
            </tr>
          ) : data.length === 0 ? (
            <tr>
              <td colSpan={columns.length} className="px-3 py-8 text-center text-ink-600">
                {emptyMessage}
              </td>
            </tr>
          ) : (
            data.map((row, rowIdx) => (
              <tr
                key={keyExtractor(row, rowIdx)}
                className={`border-b border-border last:border-b-0 ${
                  zebra && rowIdx % 2 === 1 ? "bg-canvas" : "bg-surface"
                }`}
              >
                {columns.map((col, colIdx) => {
                  const raw = col.accessorKey ? (row[col.accessorKey] as unknown) : null;
                  let content: React.ReactNode = raw as React.ReactNode;

                  if (col.cell) {
                    content = col.cell(row, rowIdx);
                  } else if (col.type === "money") {
                    content = <MoneyText value={raw as string | number | null} />;
                  } else if (col.type === "status") {
                    content = <StatusBadge status={raw as string | null} />;
                  }

                  return (
                    <td
                      key={colIdx}
                      style={col.width ? { width: col.width } : undefined}
                      className={`px-3 py-2 align-top text-ink-900 ${
                        col.type === "money" ? "text-right" : alignClass(col.align)
                      } ${col.isMono ? "font-mono" : ""}`}
                    >
                      {content}
                    </td>
                  );
                })}
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  );
}
