# Kicau Mania - AI Agent & Developer Guidelines

Dokumen ini adalah aturan utama (*Project Rules*) yang **wajib dipatuhi secara otomatis** oleh AI Agent dan developer saat bekerja di repositori ini.

---

## 1. Aturan Arsitektur & Desain Kode (Clean Architecture)
- **Pola Modul:** Gunakan pola *Domain-Driven Modular Monolith* berbasis Clean Architecture pada `cloud-app` dan `local-app` (`internal/<domain>/{model, repository, usecase, delivery}`).
- **Routing API:**
  - `cloud-app`: Kelompokkan endpoint ke dalam 3-tier security (`public`, `RequireUser`, `engine`).
  - `local-app`: Kelompokkan endpoint berbasis topologi jaringan (`LANViewer`, `LocalControl`, `Operator`).
- **Penanganan Error:**
  - Gunakan paket sentinel `internal/apperror` (`ErrNotFound`, `ErrConflict`, `ErrInvalid`, `ErrForbidden`, `ErrUnauthorized`, `ErrUnavailable`).
  - Manfaatkan `apperror.PublicError` untuk pesan aman yang diizinkan tampil ke publik.
  - Tangani pemetaan HTTP secara terpusat melalui `response.WriteError(c, err)`. Jangan pernah mengekspos error SQL/driver ke klien (Prinsip *Zero Technical Information Leakage* & OWASP A10:2025).
- **Pagination:** Semua endpoint pembacaan koleksi data dinamis wajib menggunakan **cursor-based pagination** (`request.Page(c)` dan `response.Paginate[T]`). Dilarang keras menggunakan pagination `page` dan `offset` pada tabel transaksi.
- **Integritas Transaksi:** Operasi multi-tabel wajib dibungkus dengan `store.WithTransaction(ctx, fn)`.
- **Ketahanan Vendor Eksternal:** Seluruh pemanggilan HTTP pihak ketiga (misal: WhatsApp Gateway) wajib dilindungi oleh `resilience.CircuitBreaker`.

---

## 2. Standar Zero-Comment & Executable Constraints (`no-code-comments`)
Dilarang mengandalkan komentar agar developer atau AI mengingat suatu aturan. Jadikan aturan itu dapat dieksekusi oleh mesin:
1. **Jika ada batasan (constraint):** Masukkan ke dalam API atau Type System (misal: `decimal.Decimal` untuk uang, newtypes/UUID untuk ID, validasi skema deklaratif via `request.Bind`).
2. **Jika ada konvensi tim:** Tegakkan melalui linter atau AST architectural test (seperti `architecture_test.go`).
3. **Jika ada aturan bisnis:** Buktikan melalui table-driven unit tests. Test adalah dokumentasi bisnis yang tidak bisa berbohong.
4. **Pengecualian satu-satunya:** Hanya izinkan compiler directives (`//go:embed`, `//go:generate`) dan anotasi deklaratif OpenAPI/Swagger (`// @Summary`, `// @Router`).

---

## 3. Protokol Eliminasi Workaround (`eliminate-workarounds`)
- **Dilarang keras:**
  - Menulis komentar `/* ... */` untuk mengarsipkan kode lama. Hapus kodenya langsung; Git history adalah tempat arsip.
  - Membiarkan fungsi usang (*deprecated*) hidup berdampingan dengan fungsi baru.
  - Membuat *wrapper* baru di atas workaround lama.
  - Mempertahankan kode hack dengan komentar pembelaan.
- **Boy Scout Rule:** Setiap kali membaca kode dan menemukan workaround atau duplikasi, bersihkan dan selesaikan langsung pada akarnya.

---

## 4. Protokol Operasional & Verifikasi Agen (`agent-ctl`)
Binary mandiri pengontrol aplikasi terletak di: `bin/agent-ctl.exe`.
AI Agent wajib mematuhi alur kerja berikut:
1. **Cek Status:** Jalankan `bin/agent-ctl.exe status` sebelum dan sesudah perubahan.
2. **Eksekusi Pengujian:** Jalankan `bin/agent-ctl.exe verify <target> --no-smoke` setiap kali selesai mengubah kode.
3. **Bukti Kerja (Evidence):** Dilarang melaporkan pekerjaan selesai tanpa melampirkan ringkasan bukti kerja dari `bin/agent-ctl.exe evidence <target>`.
4. **Pembersihan Proses:** Selalu panggil `bin/agent-ctl.exe stop <target>` setelah sesi pengujian live selesai agar tidak ada proses *zombie* yang mengunci port.

---

## 5. Rujukan Peta Fitur (Feature Map)
Sebelum mengubah atau menambahkan interaksi UI/API, selalu konsultasikan dokumen spesifikasi pada folder `feature-map/`:
- **Desktop HUD & Operator:** `../wails/kicau-mania-desktop/feature-map/`
- **Cloud Backend API:** `cloud-app/feature-map/`
- **Edge Media Engine:** `local-app/feature-map/`
