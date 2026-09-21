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
   * Bila diisi, item hanya tampil bagi peran ini. Tutup hari memerlukan permission
   * `system:config` yang menurut domain.RolePermissions hanya dimiliki SUPERADMIN.
   */
  requiredRole?: string;
  /**
   * Daftar peran yang boleh melihat item. Dipakai bila izin endpoint halaman dimiliki
   * lebih dari satu peran. Bila diisi, `requiredRole` tidak dipakai. Peran yang tidak
   * disebut disembunyikan (termasuk saat peran belum diketahui), bukan ditampilkan
   * lalu ditolak 403 oleh backend.
   */
  requiredRoles?: string[];
}

export interface NavGroup {
  titleKey: keyof Dictionary["nav"];
  items: NavItem[];
}

// Salinan izin dari domain.RolePermissions (apps/api/internal/domain/staff.go).
// Peran Go tidak dibagikan ke web, jadi daftar ini dijaga manual: samakan dengan
// RolePermissions bila permission berubah, agar menu tidak menampilkan halaman yang
// akan ditolak backend dengan 403.

// ledger:read — dipakai daftar transaksi, jatuh tempo, buku besar, dan laporan.
const LEDGER_READ_ROLES = ["SUPERADMIN", "ADMIN", "SUPERVISOR", "TELLER", "CS", "AUDITOR"];
// loans:read — dipakai daftar kredit/pembiayaan dan pratinjau PPAP.
const LOANS_READ_ROLES = ["SUPERADMIN", "ADMIN", "SUPERVISOR", "AO", "AUDITOR"];
// maker_checker:approve/reject — antrean persetujuan.
const MAKER_CHECKER_ROLES = ["SUPERADMIN", "ADMIN", "SUPERVISOR"];
// users:read — daftar cabang.
const USERS_READ_ROLES = ["SUPERADMIN", "ADMIN", "SUPERVISOR", "AUDITOR"];
// reports:export — definisi/ekspor laporan OJK dan peninjauan pemetaan (baca).
const REPORTS_ROLES = ["SUPERADMIN", "ADMIN", "SUPERVISOR", "AUDITOR"];
// transactions:deposit/withdraw/transfer — layar teller yang hanya memposting transaksi.
const TRANSACTION_ROLES = ["SUPERADMIN", "ADMIN", "TELLER"];

/** Navigasi per domain (DESIGN.md bagian "Navigasi"). */
export const NAV_GROUPS: NavGroup[] = [
  {
    titleKey: "groupOperations",
    items: [
      { href: "/", labelKey: "beranda", icon: Home },
      {
        href: "/teller",
        labelKey: "teller",
        icon: Wallet,
        requiredRoles: TRANSACTION_ROLES,
      },
      {
        href: "/transaksi",
        labelKey: "transaksi",
        icon: CreditCard,
        requiredRoles: LEDGER_READ_ROLES,
      },
      { href: "/deposito", labelKey: "deposito", icon: PiggyBank },
      {
        href: "/jatuh-tempo",
        labelKey: "jatuhTempo",
        icon: CalendarClock,
        requiredRoles: LEDGER_READ_ROLES,
      },
      {
        href: "/ppap",
        labelKey: "ppap",
        icon: ShieldCheck,
        requiredRoles: LOANS_READ_ROLES,
      },
      {
        href: "/tutup-hari",
        labelKey: "tutupHari",
        icon: CalendarCheck,
        requiredRole: "SUPERADMIN",
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
        requiredRoles: LOANS_READ_ROLES,
      },
      {
        href: "/pembiayaan",
        labelKey: "pembiayaan",
        icon: HandCoins,
        requiredRoles: LOANS_READ_ROLES,
      },
      {
        href: "/persetujuan",
        labelKey: "persetujuan",
        icon: CheckSquare,
        requiredRoles: MAKER_CHECKER_ROLES,
      },
    ],
  },
  {
    titleKey: "groupMaster",
    items: [
      { href: "/nasabah", labelKey: "nasabah", icon: Users },
      { href: "/rekening", labelKey: "rekening", icon: Briefcase },
      { href: "/produk", labelKey: "produk", icon: Package },
      {
        href: "/cabang",
        labelKey: "cabang",
        icon: Building2,
        requiredRoles: USERS_READ_ROLES,
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
        requiredRoles: LEDGER_READ_ROLES,
      },
      {
        href: "/laporan",
        labelKey: "laporan",
        icon: BarChart3,
        requiredRoles: LEDGER_READ_ROLES,
      },
      {
        href: "/pemetaan-ojk",
        labelKey: "pemetaanOjk",
        icon: ListChecks,
        requiredRoles: REPORTS_ROLES,
      },
    ],
  },
  {
    titleKey: "groupSettings",
    items: [{ href: "/pengaturan", labelKey: "pengaturan", icon: Settings }],
  },
];
