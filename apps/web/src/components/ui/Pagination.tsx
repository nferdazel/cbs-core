import React from "react";
import { Button } from "./Button";

export interface PaginationProps {
  page: number;
  pageSize: number;
  totalItems: number;
  totalPages: number;
  onPageChange: (page: number) => void;
}

export const Pagination: React.FC<PaginationProps> = ({
  page,
  pageSize,
  totalItems,
  totalPages,
  onPageChange,
}) => {
  const safeTotalPages = Math.max(totalPages, 1);
  const firstItem = totalItems === 0 ? 0 : (page - 1) * pageSize + 1;
  const lastItem = Math.min(page * pageSize, totalItems);

  return (
    <div className="flex items-center justify-between border-t border-border px-4 py-1 text-meta text-ink-600">
      <span>
        {firstItem}-{lastItem} dari {totalItems} item
      </span>
      <div className="flex items-center gap-2">
        <Button
          variant="secondary"
          size="sm"
          onClick={() => onPageChange(page - 1)}
          disabled={page <= 1}
        >
          Sebelumnya
        </Button>
        <span className="font-mono text-ink-900">
          {page} / {safeTotalPages}
        </span>
        <Button
          variant="secondary"
          size="sm"
          onClick={() => onPageChange(page + 1)}
          disabled={page >= safeTotalPages}
        >
          Berikutnya
        </Button>
      </div>
    </div>
  );
};
