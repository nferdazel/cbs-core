"use client";

import React from "react";
import { useAuth } from "@/lib/useAuth";
import { useTranslation } from "@/i18n/context";
import { LoadingState } from "@/components/ui/States";
import { FeatureUnavailable } from "./FeatureUnavailable";

export interface BookScopeGuardProps {
  /** Buku yang dibutuhkan halaman ini. */
  book: "CONVENTIONAL" | "SYARIAH";
  title: string;
  description: string;
  children: React.ReactNode;
}

/**
 * Penjaga halaman lini usaha. Cakupan dibaca dari server lewat GET /auth/me
 * (active_books); web tidak menebak. Bila buku tidak aktif, ditampilkan halaman
 * "tidak tersedia" alih-alih form yang pasti ditolak API. Ini hanya lapisan UI:
 * backend tetap penentu akses dan menolak endpoint lini usaha yang tidak aktif.
 *
 * Bila `active_books` belum tersedia (sesi lama/backend lama), halaman dibiarkan
 * tampil agar tidak memutus operasi; server tetap menolak bila memang tidak aktif.
 */
export const BookScopeGuard: React.FC<BookScopeGuardProps> = ({
  book,
  title,
  description,
  children,
}) => {
  const { user, ready } = useAuth();
  const { t } = useTranslation();

  if (!ready) {
    return <LoadingState label={t.common.checkingSession} />;
  }

  const activeBooks = user?.active_books;
  if (activeBooks && !activeBooks.includes(book)) {
    const label = book === "SYARIAH" ? t.bookScope.syariah : t.bookScope.conventional;
    return (
      <FeatureUnavailable
        title={title}
        description={description}
        reason={`${t.bookScope.reasonPrefix} ${label} ${t.bookScope.reasonMiddle} ${user?.book_scope ?? "DUAL"}${t.bookScope.reasonSuffix}`}
      />
    );
  }

  return <>{children}</>;
};
