Struktur folder yang ada di file PETA-KODE.md sama sekali tidak berantakan. Sebaliknya, struktur ini sangat matang, spesifik, dan merupakan implementasi nyata dari Hexagonal/Clean Architecture untuk sistem *edge-node* terdistribusi.

Alasan mengapa struktur ini terasa "berantakan" saat dilihat pertama kali adalah karena sistem ini menangani tiga masalah kompleks secara bersamaan (bisnis turnamen, pemrosesan media/hardware, dan sinkronisasi *offline-first*), dan semuanya diletakkan sejajar di bawah direktori `internal/`.

Berikut adalah analisis mengapa arsitektur ini sudah tepat sasaran dan langkah konkret untuk mengelolanya agar tidak terasa membingungkan.

### Mengapa Struktur Ini Sudah Tepat (Validasi Arsitektur)

1. **Arah Dependensi yang Ketat (Inversion of Control):** Struktur ini secara eksplisit memaksakan alur **Handler → UseCase → Repository → penyimpanan/infrastruktur**. Layer `usecase` memuat aturan bisnis murni dan koordinasi, sedangkan implementasi teknis disuntikkan dari luar.


2. **Fleksibilitas Modul (Domain-Driven):** Setiap domain hanya memiliki folder yang benar-benar dibutuhkan. Contohnya, domain `video` tidak dipaksa memiliki `repository` atau `worker` sendiri karena tugas utamanya hanya menyediakan metadata untuk membuka video.


3. **Pemisahan Lifecycle dan Wiring:** File `internal/app/container.go` bertanggung jawab murni untuk merakit dependensi konkret, sedangkan `internal/server/server.go` murni menangani *startup, shutdown,* dan supervisi modul. Ini mencegah aplikasi hancur total jika satu kamera gagal menyala.


4. **Isolasi Alur Offline-First:** Sinkronisasi cloud tidak dicampur ke dalam logika skor arena. Domain `synchronization` berdiri sendiri untuk membaca maksimal 100 perubahan, mengirim *batch*, dan memvalidasi *acknowledgment*.



### Langkah Memperbaiki Tata Letak Visual (Grup Bounded Context)

Jika melihat deretan 16+ folder di dalam `internal/` terasa terlalu padat, kamu tidak perlu menulis ulang kodenya. Kamu hanya perlu mengelompokkan folder-folder tersebut ke dalam 3 *Bounded Context* utama (Core, Media, dan Edge-Sync).

Berikut adalah rekomendasi restrukturisasi ringan untuk merapikan visual folder `internal/` tanpa merusak logika yang sudah berjalan:

**Langkah 1: Buat grup untuk Core Business Lomba**
Pindahkan domain yang murni mengurus aturan turnamen dan antarmuka pengguna lokal.

* Pindahkan `arena/`, `auth/`, dan `identifier/` ke dalam `internal/core/`.

**Langkah 2: Buat grup untuk Media & Hardware Engine**
Kumpulkan semua modul yang berurusan dengan perangkat keras, *stream* kamera, dan sinyal audio. Modul-modul ini biasanya rakus CPU/Memori dan memiliki *worker* sendiri.

* Pindahkan `channel/`, `video/`, `audio/`, `streaming/`, `saving/`, dan `media/` ke dalam `internal/mediaengine/`.

**Langkah 3: Buat grup untuk Edge-to-Cloud & Infastruktur**
Kelompokkan domain yang bertugas menjaga aplikasi tetap hidup secara lokal dan menjembatani data ke *cloud* saat internet tersedia.

* Pindahkan `synchronization/`, `exchange/`, `operation/`, `network/`, `storage/`, dan `runtime/` ke dalam `internal/edge/`.

**Langkah 4: Biarkan Shared Layer tetap di Root `internal/**`
Layer yang dipakai secara luas oleh seluruh aplikasi harus tetap mudah diakses.

* Biarkan `app/`, `server/`, `delivery/http/`, dan `apperror/` tetap berada langsung di bawah `internal/`.

### Verifikasi Keberhasilan

Setelah melakukan restrukturisasi pemindahan folder, jalankan perintah pengujian dari *root* direktori untuk memastikan *import path* sudah diperbarui dengan benar dan tidak ada *circular dependency*:
`go test ./...`
*Verifikasi:* Semua tes lulus (PASS), membuktikan bahwa logika bisnis tidak terpengaruh oleh perpindahan lokasi folder.