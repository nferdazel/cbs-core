import React from "react";

export interface Column<T> {
  header: string;
  accessorKey?: keyof T;
  cell?: (row: T, index: number) => React.ReactNode;
  align?: "left" | "center" | "right";
  isMono?: boolean;
}

export interface DataTableProps<T> {
  columns: Column<T>[];
  data: T[];
  keyExtractor: (row: T, index: number) => string | number;
  emptyMessage?: string;
}

export function DataTable<T>({
  columns,
  data,
  keyExtractor,
  emptyMessage = "Tidak ada data.",
}: DataTableProps<T>) {
  return (
    <div className="w-full overflow-x-auto rounded-lg border border-slate-200 bg-white">
      <table className="w-full text-left text-xs">
        <thead className="bg-slate-100 text-slate-700 uppercase font-sans font-semibold border-b border-slate-200 tracking-wider">
          <tr>
            {columns.map((col, idx) => (
              <th
                key={idx}
                className={`py-2.5 px-4 ${
                  col.align === "right"
                    ? "text-right"
                    : col.align === "center"
                    ? "text-center"
                    : "text-left"
                }`}
              >
                {col.header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-200">
          {data.length === 0 ? (
            <tr>
              <td
                colSpan={columns.length}
                className="py-8 text-center text-slate-500 font-sans text-xs"
              >
                {emptyMessage}
              </td>
            </tr>
          ) : (
            data.map((row, rowIdx) => (
              <tr
                key={keyExtractor(row, rowIdx)}
                className="hover:bg-slate-50/80 transition-colors even:bg-slate-50/40"
              >
                {columns.map((col, colIdx) => {
                  const val = col.accessorKey ? (row[col.accessorKey] as any) : null;
                  const content = col.cell ? col.cell(row, rowIdx) : val;

                  return (
                    <td
                      key={colIdx}
                      className={`py-2.5 px-4 text-slate-800 ${
                        col.isMono ? "font-mono" : "font-sans"
                      } ${
                        col.align === "right"
                          ? "text-right"
                          : col.align === "center"
                          ? "text-center"
                          : "text-left"
                      }`}
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
