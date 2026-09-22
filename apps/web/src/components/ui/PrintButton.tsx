"use client";

import { Printer } from "lucide-react";
import { API_BASE_URL } from "@/lib/api";
import { useTranslation } from "@/i18n/context";
import { Button, type ButtonProps } from "@/components/ui/Button";

export interface PrintButtonProps {
  /**
   * Jalur dokumen cetak relatif terhadap API, mis. `/documents/deposit-slip/{ref}`.
   * Komponen menambahkan prefiks API sehingga dokumen dibuka lewat rewrite Next
   * secara same-origin dan cookie sesi tetap terkirim.
   */
  url: string;
  label?: string;
  variant?: ButtonProps["variant"];
  size?: ButtonProps["size"];
}

/**
 * Membuka dokumen cetak yang dihasilkan API di tab baru, lalu memicu dialog cetak
 * browser. Tombol ini hanya membungkus alur cetak; format dokumen tetap milik API,
 * sehingga tidak ada HTML cetak yang diduplikasi di frontend.
 */
export function PrintButton({
  url,
  label,
  variant = "secondary",
  size = "sm",
}: PrintButtonProps) {
  const { t } = useTranslation();
  const handlePrint = () => {
    const target = window.open(`${API_BASE_URL}${url}`, "_blank");
    // Popup bisa diblokir; dalam hal itu tidak ada yang bisa dicetak otomatis dan
    // pengguna dapat mengizinkan popup lalu mencoba lagi.
    if (!target) return;

    const trigger = () => {
      try {
        target.focus();
        target.print();
      } catch {
        // Akses ke tab ditolak browser; dokumen tetap terbuka untuk dicetak manual.
      }
    };
    // once: dokumen cetak hanya dimuat sekali; hindari print() berulang bila
    // halaman memicu event load lagi.
    target.addEventListener("load", trigger, { once: true });
  };

  return (
    <Button
      type="button"
      size={size}
      variant={variant}
      icon={<Printer className="h-4 w-4" />}
      onClick={handlePrint}
    >
      {label ?? t.common.print}
    </Button>
  );
}
