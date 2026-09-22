"use client";

import React from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useTranslation } from "@/i18n/context";
import { NAV_GROUPS, type NavItem } from "./nav";

export interface AppSidebarProps {
  /**
   * Kunci menu yang terbuka bagi pengguna, dihitung server dari izin efektif
   * (GET /auth/me -> menus). Web tidak menyimpan salinan peran/izin. Bila belum
   * diketahui (null/undefined), semua item ditampilkan — server tetap penentu
   * akses — supaya sesi lama tidak kehilangan menu.
   */
  menus?: string[] | null;
  /**
   * Buku yang aktif di instalasi (GET /auth/me -> active_books). Item yang menandai
   * buku lain tidak dirender.
   */
  activeBooks?: string[] | null;
}

/**
 * Apakah item boleh tampil. Penyaringan buku dari cakupan instalasi, lalu kunci
 * menu harus ada di daftar yang diberikan server. Menyembunyikan menu BUKAN batas
 * keamanan; setiap endpoint tetap dijaga izin di API.
 */
function isItemVisible(item: NavItem, menus?: string[] | null, activeBooks?: string[] | null): boolean {
  if (item.book && activeBooks && !activeBooks.includes(item.book)) {
    return false;
  }
  if (menus == null) {
    return true;
  }
  return menus.includes(item.menuKey);
}

/**
 * Sidebar vertikal tetap 240px. Item aktif ditandai latar navy + batas aksen
 * di sisi kiri (bukan warna saja). Item yang tidak dibuka izin backend
 * (mis. Tutup Hari) disembunyikan, bukan ditampilkan lalu ditolak.
 */
export const AppSidebar: React.FC<AppSidebarProps> = ({ menus, activeBooks }) => {
  const pathname = usePathname();
  const { t } = useTranslation();

  return (
    <nav
      aria-label={t.nav.mainNavigation}
      className="sticky top-14 h-[calc(100vh-56px)] w-[240px] shrink-0 overflow-y-auto bg-navy-800 py-3 text-white"
    >
      {NAV_GROUPS.map((group) => (
        <div key={group.titleKey} className="mb-3">
          <p className="px-4 py-1 text-meta font-medium uppercase tracking-wide text-white/50">
            {t.nav[group.titleKey]}
          </p>
          <ul>
            {group.items
              .filter((item) => isItemVisible(item, menus, activeBooks))
              .map((item) => {
                const isActive =
                  item.href === "/"
                    ? pathname === "/"
                    : pathname === item.href || pathname.startsWith(`${item.href}/`);
                const Icon = item.icon;
                return (
                  <li key={item.href}>
                    <Link
                      href={item.href}
                      aria-current={isActive ? "page" : undefined}
                      className={`flex items-center gap-2 border-l-2 py-2 pl-3 pr-4 text-body transition-colors duration-fast ${
                        isActive
                          ? "border-accent-600 bg-navy-700 font-medium text-white"
                          : "border-transparent text-white/70 hover:bg-navy-700 hover:text-white"
                      }`}
                    >
                      <Icon className="h-4 w-4 shrink-0" aria-hidden />
                      <span>{t.nav[item.labelKey]}</span>
                    </Link>
                  </li>
                );
              })}
          </ul>
        </div>
      ))}
    </nav>
  );
};
