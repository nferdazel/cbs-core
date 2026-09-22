import type { LucideIcon } from "lucide-react";
import {
  BarChart3,
  BookOpenCheck,
  Briefcase,
  Building2,
  CalendarCheck,
  CalendarClock,
  CheckSquare,
  CreditCard,
  HandCoins,
  Home,
  Landmark,
  ListChecks,
  Package,
  PiggyBank,
  Settings,
  ShieldCheck,
  Users,
  Wallet,
} from "lucide-react";
import type { Dictionary } from "@/i18n/dictionaries/id";

export interface NavItem {
  href: string;
  labelKey: keyof Dictionary["nav"];
  icon: LucideIcon;
  /**
   * Kunci menu stabil yang dipetakan ke izin di database (user_group / menu_catalog,
   * migrasi 000079). Web TIDAK menyimpan salinan peran maupun izin: server mengirim
   * daftar menu yang terbuka lewat GET /auth/me -> `menus`, dan sidebar hanya
   * merender item yang kuncinya ada di daftar itu. Pemetaan menu<->izin dapat diubah
   * tanpa rilis kode web.
   */
  menuKey: string;
  /**
   * Bila diisi, item hanya tampil pada instalasi yang mengaktifkan buku ini
   * (GET /auth/me -> active_books). Cakupan buku dibaca dari server, bukan
   * di-hardcode; ini semata penyaring tampilan, bukan batas keamanan.
   */
  book?: "CONVENTIONAL" | "SYARIAH";
}

export interface NavGroup {
  titleKey: keyof Dictionary["nav"];
  items: NavItem[];
}

/** Navigasi per domain (DESIGN.md bagian "Navigasi"). */
export const NAV_GROUPS: NavGroup[] = [
  {
    titleKey: "groupOperations",
    items: [
      { href: "/", labelKey: "beranda", icon: Home, menuKey: "beranda" },
      { href: "/teller", labelKey: "teller", icon: Wallet, menuKey: "teller" },
      {
        href: "/transaksi",
        labelKey: "transaksi",
        icon: CreditCard,
        menuKey: "transaksi",
      },
      {
        href: "/deposito",
        labelKey: "deposito",
        icon: PiggyBank,
        menuKey: "deposito",
      },
      {
        href: "/jatuh-tempo",
        labelKey: "jatuhTempo",
        icon: CalendarClock,
        menuKey: "jatuh_tempo",
      },
      { href: "/ppap", labelKey: "ppap", icon: ShieldCheck, menuKey: "ppap" },
      {
        href: "/tutup-hari",
        labelKey: "tutupHari",
        icon: CalendarCheck,
        menuKey: "tutup_hari",
      },
    ],
  },
  {
    titleKey: "groupCredit",
    items: [
      {
        href: "/kredit",
        labelKey: "kredit",
        icon: Landmark,
        menuKey: "kredit",
        book: "CONVENTIONAL",
      },
      {
        href: "/pembiayaan",
        labelKey: "pembiayaan",
        icon: HandCoins,
        menuKey: "pembiayaan",
        book: "SYARIAH",
      },
      {
        href: "/persetujuan",
        labelKey: "persetujuan",
        icon: CheckSquare,
        menuKey: "persetujuan",
      },
    ],
  },
  {
    titleKey: "groupMaster",
    items: [
      {
        href: "/nasabah",
        labelKey: "nasabah",
        icon: Users,
        menuKey: "nasabah",
      },
      {
        href: "/rekening",
        labelKey: "rekening",
        icon: Briefcase,
        menuKey: "rekening",
      },
      { href: "/produk", labelKey: "produk", icon: Package, menuKey: "produk" },
      {
        href: "/cabang",
        labelKey: "cabang",
        icon: Building2,
        menuKey: "cabang",
      },
    ],
  },
  {
    titleKey: "groupAccounting",
    items: [
      {
        href: "/buku-besar",
        labelKey: "bukuBesar",
        icon: BookOpenCheck,
        menuKey: "buku_besar",
      },
      {
        href: "/laporan",
        labelKey: "laporan",
        icon: BarChart3,
        menuKey: "laporan",
      },
      {
        href: "/pemetaan-ojk",
        labelKey: "pemetaanOjk",
        icon: ListChecks,
        menuKey: "pemetaan_ojk",
      },
    ],
  },
  {
    titleKey: "groupSettings",
    items: [
      {
        href: "/pengaturan",
        labelKey: "pengaturan",
        icon: Settings,
        menuKey: "pengaturan",
      },
    ],
  },
];
