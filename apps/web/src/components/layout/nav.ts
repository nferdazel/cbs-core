import type { LucideIcon } from "lucide-react";
import {
  BarChart3,
  BookOpenCheck,
  Briefcase,
  Building2,
  CalendarCheck,
  CheckSquare,
  CreditCard,
  HandCoins,
  Home,
  Landmark,
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
      { href: "/", labelKey: "beranda", icon: Home },
      { href: "/teller", labelKey: "teller", icon: Wallet },
      { href: "/transaksi", labelKey: "transaksi", icon: CreditCard },
      { href: "/deposito", labelKey: "deposito", icon: PiggyBank },
      { href: "/ppap", labelKey: "ppap", icon: ShieldCheck },
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
      { href: "/kredit", labelKey: "kredit", icon: Landmark },
      { href: "/pembiayaan", labelKey: "pembiayaan", icon: HandCoins },
      { href: "/persetujuan", labelKey: "persetujuan", icon: CheckSquare },
    ],
  },
  {
    titleKey: "groupMaster",
    items: [
      { href: "/nasabah", labelKey: "nasabah", icon: Users },
      { href: "/rekening", labelKey: "rekening", icon: Briefcase },
      { href: "/produk", labelKey: "produk", icon: Package },
      { href: "/cabang", labelKey: "cabang", icon: Building2 },
    ],
  },
  {
    titleKey: "groupAccounting",
    items: [
      { href: "/buku-besar", labelKey: "bukuBesar", icon: BookOpenCheck },
      { href: "/laporan", labelKey: "laporan", icon: BarChart3 },
    ],
  },
  {
    titleKey: "groupSettings",
    items: [{ href: "/pengaturan", labelKey: "pengaturan", icon: Settings }],
  },
];
