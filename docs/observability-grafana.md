# Panduan Akses & Operasional Observability (Grafana, Loki, Prometheus, Alloy)

Dokumen ini berisi catatan cepat untuk mengakses **Grafana** dan stack observabilitas `cloud-app` di VPS secara aman melalui **SSH Tunnel**.

---

## 1. Cara Mengakses Grafana dari Laptop

Demi keamanan, port Grafana, Prometheus, dan Alloy di VPS hanya di-bind ke `127.0.0.1` (tidak dibuka ke publik).

1. Buka terminal (PowerShell / Command Prompt) di laptop Anda, lalu jalankan:
   ```powershell
   ssh -i "C:\Users\WIN II\.ssh\id_ed25519" -L 3001:127.0.0.1:3001 -N root@187.124.138.204
   ```
   *(Biarkan terminal ini tetap terbuka selama Anda mengakses Grafana. Tekan `Ctrl + C` jika sudah selesai).*

2. Buka browser dan kunjungi:
   - **URL Grafana:** `http://localhost:3001`
   - **Dashboard Utama:** Masuk ke menu **Dashboards** → folder **Kicau Mania** → **Kicau Mania Cloud - Observability** (sudah terpasang otomatis).

---

## 2. Akses Tool Observabilitas Lain (Opsional via SSH Tunnel)

Jika ingin membuka UI Prometheus atau Grafana Alloy secara langsung:

| Layanan | Perintah SSH Tunnel | URL Lokal |
| :--- | :--- | :--- |
| **Grafana** | `ssh -i "C:\Users\WIN II\.ssh\id_ed25519" -L 3001:127.0.0.1:3001 -N root@187.124.138.204` | `http://localhost:3001` |
| **Prometheus** | `ssh -i "C:\Users\WIN II\.ssh\id_ed25519" -L 9090:127.0.0.1:9090 -N root@187.124.138.204` | `http://localhost:9090` |
| **Alloy UI** | `ssh -i "C:\Users\WIN II\.ssh\id_ed25519" -L 12345:127.0.0.1:12345 -N root@187.124.138.204` | `http://localhost:12345` |

*(Atau gabungkan sekaligus dalam satu perintah SSH):*
```powershell
ssh -i "C:\Users\WIN II\.ssh\id_ed25519" -L 3001:127.0.0.1:3001 -L 9090:127.0.0.1:9090 -L 12345:127.0.0.1:12345 -N root@187.124.138.204
```

---

## 3. Query Log Cepat di Grafana (Explore → Loki)

- **Semua log dari `cloud-app`:**
  ```logql
  {service="cloud-app"}
  ```
- **Hanya error (`level="error"`):**
  ```logql
  {service="cloud-app", level="error"}
  ```
- **Lacak 1 request spesifik berdasarkan `X-Request-ID`:**
  ```logql
  {service="cloud-app"} | requestId="<MASUKKAN_REQUEST_ID>"
  ```

---

## 4. Perintah Operasional di VPS

Jika perlu mengecek status, restart, atau melihat log stack observabilitas di VPS:

```bash
cd /opt/apps/kicau-mania
docker compose -f compose.observability.yaml ps
docker compose -f compose.observability.yaml up -d
docker compose -f compose.observability.yaml logs -f alloy
```
