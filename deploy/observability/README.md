# Observabilitas (VictoriaMetrics) — pusat metrik Qouver

Berisi manifest untuk **VictoriaMetrics**, penyimpan metrik yang melayani
**semua project** di VPS (cbs, fond, majadu, sds), bukan cbs saja.

## Kenapa VictoriaMetrics

VPS (3,5 GB RAM) dipakai bersama banyak container. VictoriaMetrics:
- **Hemat**: ~20 MB saat idle, ~50 MB saat aktif (Prometheus 200-400 MB, SigNoz 2-4 GB).
- **100% kompatibel** dengan `/metrics` format Prometheus yang sudah dipakai project.
- **Satu binary**, tanpa dependensi (bukan ClickHouse seperti SigNoz).

Ini dipilih setelah SigNoz dinilai tidak muat di VPS 4 GB.

## Isi

| Berkas | Fungsi |
|---|---|
| `victoriametrics.container` | Quadlet unit (podman rootless + systemd user), pola sama `cbs-api.container`. |
| `promscrape.yml` | Konfigurasi scrape: target semua project di network podman `qouver`. |

## Pemasangan (di VPS)

```bash
# 1. Direktori data & config
mkdir -p /srv/qouver/apps/observability/{data,config}

# 2. Kirim berkas
scp victoriametrics.container <vps>:~/.config/containers/systemd/
scp promscrape.yml <vps>:/srv/qouver/apps/observability/config/

# 3. Jalankan
ssh <vps> 'podman pull docker.io/victoriametrics/victoria-metrics:v1.153.0'
ssh <vps> 'systemctl --user daemon-reload && systemctl --user start victoriametrics'
```

Auto-start saat boot dijamin `[Install] WantedBy=default.target` + `Linger=yes`
(sama seperti container lain).

## Akses

- Port **bind ke `127.0.0.1:8428`** — tidak publik. Scrape & UI hanya dari host.
- UI (vmui) via SSH tunnel:
  ```bash
  ssh -L 8428:127.0.0.1:8428 <vps>
  # buka http://127.0.0.1:8428/vmui
  ```
- Data scrape tersimpan 90 hari di `/srv/qouver/apps/observability/data`.

## Menambah project baru

Tambahkan `job_name` di `promscrape.yml`, pakai **nama container** (bukan IP):

```yaml
  - job_name: project-baru
    metrics_path: /metrics
    static_configs:
      - targets: ["project-baru-api:8080"]
        labels: { project: project-baru, env: prod }
```

Lalu restart: `systemctl --user restart victoriametrics`.

## Catatan quadlet (penting)

Gunakan **satu `Exec=`** berisi seluruh flag. `Exec=` berulang pada satu container
hanya menerapkan yang **terakhir** — flag lain terabaikan (sempat terjadi: VM jalan
tanpa `-promscrape.config`, sehingga tidak ada target).

## Status target

- `majadu-api`: UP.
- `cbs-api`: menunggu W18 (`GET /metrics`) ter-deploy.
- `sds-api`: menunggu endpoint `/metrics` tersedia (sekarang 503).
