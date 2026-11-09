# Rancangan ERD Multi-Arena Kicau Mania

> **Status — DRAFT UNTUK REVIEW**
>
> Tanggal revisi 9 November 2026. Diagram ini menggambarkan usulan skema berdasarkan diskusi terbaru dan jawaban lanjutan. Penerapan ke database menunggu persetujuan atas rancangan ini.

Acuan bisnis berasal dari catatan diskusi. Nama entitas seperti `asset_arenas` dalam diagram mewakili tabel `asset.arenas`. Atribut yang ditampilkan dipilih untuk menjelaskan hubungan dan aturan bisnis. [Catatan diskusi](../../discussion_11062026_erd.txt)

## 1. Gambaran utama

Setiap item registrasi menyatakan satu burung mengikuti satu ronde. Status dan gantangan pada item tersebut menunjukkan apakah keikutsertaan sudah dikonfirmasi. Satu header registrasi menampung beberapa item dan pembayaran gabungannya.

```mermaid
flowchart TD
    A["Arena"] --> E["Event"]
    E --> R["Ronde"]
    A --> K["Kategori arena"]
    K --> R
    A --> G["Gantangan"]
    REG["Registrasi · kontak pendaftar"] --> I["Item registrasi / keikutsertaan ronde"]
    REG --> PAY["Pembayaran gabungan"]
    B["Burung · satu pemilik"] --> I
    R --> I
    G --> I
    I --> S["Nilai juri dan revisi"]
    I --> M["Rekaman · status kirim ke pendaftar"]
```

**Penyederhanaan untuk review**

- Satu item registrasi menyimpan burung, ronde, gantangan, dan status keikutsertaan.
- Penugasan juri ronde langsung menunjuk akun juri.
- Satu daftar kriteria arena dipakai bersama untuk seluruh ronde. Ini asumsi sementara untuk kamu tinjau. Daftar dan arti kriteria dikunci setelah mulai digunakan untuk penilaian.
- Status serta tujuan pengiriman video berada pada rekaman.
- Ronde menyimpan fase saat ini. History revisi nilai dan transfer pemilik tetap ditampilkan karena keduanya sudah menjadi kebutuhan yang disepakati.

| Aturan lintas arena | Bentuk hubungan yang diusulkan |
| --- | --- |
| Banyak arena berjalan bersamaan | Event, registrasi, gantangan, dan engine memiliki `arena_id`. |
| Nomor gantangan dapat sama di arena berbeda | Unik pada pasangan `(arena_id, number)`. |
| Kategori dikelola per arena | Admin arena membuat kategorinya sendiri. Kode kategori unik pada pasangan `(arena_id, code)`. |
| Satu registrasi untuk satu arena | Item registrasi, ronde, dan gantangan harus berada pada arena header registrasi. |
| Satu ronde aktif per arena | Ronde berjalan atau terinterupsi membatasi arena terkait. Arena lain dapat menjalankan rondenya. |
| Akun dapat bertugas di beberapa arena | Penugasan role menghubungkan akun, role, dan arena. |
| Burung/pemilik dipakai lintas arena | Master burung dan pemilik bersifat global. Riwayat keikutsertaan berada pada item registrasi. |

## 2. Akun, role, dan perangkat

Akun mempunyai role global melalui `iam.user_roles` dan penugasan operasional melalui `iam.arena_user_roles`. Superadmin mengelola lintas arena. Operator, admin arena, dan juri memperoleh akses melalui penugasan arena.

```mermaid
erDiagram
    iam_users {
        uuid id PK
        varchar name
        varchar phone UK
        boolean is_active
    }
    iam_roles {
        uuid id PK
        varchar name UK
    }
    iam_user_roles {
        uuid id PK
        uuid user_id FK
        uuid role_id FK
    }
    asset_arenas {
        uuid id PK
        varchar name
        varchar timezone
        boolean is_active
    }
    iam_arena_user_roles {
        uuid id PK
        uuid arena_id FK
        uuid user_id FK
        uuid role_id FK
    }
    iam_user_sessions {
        uuid id PK
        uuid user_id FK
        uuid device_id
        timestamptz expires_at
        timestamptz revoked_at
    }

    iam_users ||..o{ iam_user_roles : memiliki
    iam_roles ||..o{ iam_user_roles : role_global
    iam_users ||..o{ iam_arena_user_roles : ditugaskan
    iam_roles ||..o{ iam_arena_user_roles : role_arena
    asset_arenas ||..o{ iam_arena_user_roles : memberi_lingkup
    iam_users ||..o{ iam_user_sessions : memiliki_perangkat
```

- Pasangan akun–role global dan kombinasi akun–arena–role operasional harus unik.
- Maksimal tiga perangkat/browser aktif dihitung per akun secara keseluruhan. Login ulang pada perangkat yang sama memakai slot yang sama. Perangkat keempat ditolak.
- Penugasan sebagai juri arena perlu dilanjutkan dengan penugasan pada ronde, yang terlihat pada diagram penjurian.

## 3. Arena, perangkat, burung, dan pemilik

Setiap arena mengelola gantangan dan engine media. Fase awal menggunakan satu engine media aktif per arena. Setiap burung wajib menunjuk tepat satu pemilik. Seorang pemilik dapat memiliki beberapa burung dan dapat ditautkan ke akun cloud.

```mermaid
erDiagram
    asset_arenas {
        uuid id PK
        varchar name
        text address
        varchar timezone
    }
    asset_gantangan {
        uuid id PK
        uuid arena_id FK
        integer number
        text description
    }
    asset_engines {
        uuid id PK
        uuid arena_id FK
        varchar name
        boolean is_active
        timestamptz last_heartbeat_at
    }
    asset_devices {
        uuid id PK
        uuid arena_id FK
        uuid engine_id FK
        uuid gantangan_id FK "opsional"
        varchar type
        varchar ip_address
    }
    iam_users {
        uuid id PK
        varchar name
    }
    asset_owners {
        uuid id PK
        uuid user_id FK "opsional"
        varchar name
        varchar phone
    }
    asset_birds {
        uuid id PK
        uuid owner_id FK
        varchar ring_number UK
        varchar name
        varchar bird_type
    }
    asset_bird_owner_transfers {
        uuid id PK
        uuid bird_id FK
        uuid from_owner_id FK
        uuid to_owner_id FK
        uuid performed_by_user_id FK
        text reason
        timestamptz transferred_at
    }

    asset_arenas ||..o{ asset_gantangan : menyediakan
    asset_arenas ||..o{ asset_engines : memiliki
    asset_arenas ||..o{ asset_devices : memiliki
    asset_engines ||..o{ asset_devices : menangani
    asset_gantangan |o..o{ asset_devices : ditempati
    iam_users |o..o| asset_owners : tertaut
    asset_owners ||..o{ asset_birds : memiliki
    asset_birds ||..o{ asset_bird_owner_transfers : riwayat_transfer
    asset_owners ||..o{ asset_bird_owner_transfers : pemilik_asal
    asset_owners ||..o{ asset_bird_owner_transfers : pemilik_tujuan
    iam_users ||..o{ asset_bird_owner_transfers : admin_pelaksana
```

- Tautan akun pada master pemilik opsional dan unik jika terisi. Pemilik tanpa akun tetap dapat tercatat.
- Transfer dilakukan admin dengan alasan dan history. Pemilik aktif disimpan pada `asset.birds.owner_id`.
- Burung yang pernah mengikuti lomba tetap mempunyai riwayat peserta meskipun pemiliknya kemudian berubah.
- Perangkat boleh belum ditempatkan pada gantangan. Arena engine, perangkat, dan gantangan yang ditautkan harus sesuai.

## 4. Registrasi, pembayaran, dan keikutsertaan

Pendaftaran manual menyimpan nama dan kontak pendaftar dengan akun opsional. Pendaftaran online memakai akun login. Satu header registrasi membayar beberapa item burung/ronde dalam satu arena. Tarif per item disimpan sebagai snapshot saat pendaftaran.

```mermaid
erDiagram
    asset_arenas {
        uuid id PK
        varchar name
    }
    iam_users {
        uuid id PK
        varchar name
    }
    competition_events {
        uuid id PK
        uuid arena_id FK
        varchar title
        date date
    }
    competition_categories {
        uuid id PK
        uuid arena_id FK
        varchar code
        varchar name
    }
    competition_rounds {
        uuid id PK
        uuid arena_id FK
        uuid event_id FK
        uuid category_id FK
        integer round_number
        varchar bird_type
        numeric entry_fee
        varchar phase
    }
    asset_birds {
        uuid id PK
        varchar name
    }
    asset_gantangan {
        uuid id PK
        uuid arena_id FK
        integer number
    }
    competition_registrations {
        uuid id PK
        uuid arena_id FK
        uuid registered_by_user_id FK "opsional"
        uuid created_by_user_id FK
        varchar registration_source
        varchar registrant_name
        varchar registrant_phone
        numeric total_amount
        varchar status
    }
    competition_registration_items {
        uuid id PK
        uuid arena_id FK
        uuid registration_id FK
        uuid round_id FK
        uuid bird_id FK
        uuid gantangan_id FK "opsional sebelum konfirmasi"
        numeric entry_fee_snapshot
        varchar status
    }
    competition_registration_payments {
        uuid id PK
        uuid arena_id FK
        uuid registration_id FK
        numeric amount
        varchar method
        text proof_reference
        varchar status
        uuid verified_by_user_id FK "opsional sebelum verifikasi"
        timestamptz verified_at
    }

    asset_arenas ||..o{ competition_events : menyelenggarakan
    competition_events ||..o{ competition_rounds : berisi
    asset_arenas ||..o{ competition_categories : memiliki_kategori
    competition_categories ||..o{ competition_rounds : mengelompokkan
    asset_arenas ||..o{ competition_registrations : menerima
    iam_users |o..o{ competition_registrations : akun_pendaftar
    iam_users ||..o{ competition_registrations : membuat_catatan
    competition_registrations ||..o{ competition_registration_items : berisi
    competition_registrations ||..o{ competition_registration_payments : pembayaran_gabungan
    iam_users |o..o{ competition_registration_payments : memverifikasi
    competition_rounds ||..o{ competition_registration_items : dituju
    asset_birds ||..o{ competition_registration_items : didaftarkan
    asset_gantangan |o..o{ competition_registration_items : alokasi_tempat
```

- Header dapat berstatus draft ketika belum mempunyai item atau pembayaran. Item berstatus menunggu pembayaran sampai pembayaran gabungan diverifikasi dan seluruh alokasi berhasil. Item terkonfirmasi menyimpan gantangan yang wajib terisi.
- Metode pembayaran awal adalah tunai atau transfer dengan verifikasi operator/admin. Satu pembayaran berlaku untuk header beserta itemnya. Tabel pembayaran dapat menyimpan beberapa percobaan dengan satu pembayaran gabungan yang diverifikasi untuk konfirmasi.
- Pendaftar tanpa akun disimpan pada snapshot nama dan kontak. `created_by_user_id` menunjuk operator untuk input manual atau akun pendaftar untuk input online.
- Kontak pendaftar menjadi penerima video. Transfer pemilik burung tidak mengubah snapshot registrasi.
- Untuk item aktif, satu burung hanya memiliki satu keikutsertaan pada ronde yang sama. Gantangan yang sudah dialokasikan hanya dapat dipakai satu item aktif per ronde. Item yang dibatalkan melepas alokasi. Arena header, item, ronde, serta gantangan harus sama.
- Kategori dimiliki arena melalui `competition.categories.arena_id`. Admin hanya mengelola kategori pada arena tempat ia ditugaskan. Arena berbeda boleh memiliki kode kategori yang sama, misalnya `VIP`, dengan keunikan `(arena_id, code)`.
- Kategori ronde harus berasal dari arena ronde tersebut. Rancangan SQL memakai FK gabungan `competition.rounds(category_id, arena_id)` ke `competition.categories(id, arena_id)`, dengan constraint unik pendukung `(id, arena_id)` pada kategori.

## 5. Penjurian, revisi nilai, dan media

Setiap ronde memiliki daftar juri yang ditugaskan langsung melalui `judge_user_id`. Kriteria dibaca dari daftar arena. Item registrasi berstatus terkonfirmasi menjadi peserta yang dinilai dan direkam.

```mermaid
erDiagram
    asset_arenas {
        uuid id PK
        varchar name
    }
    competition_rounds {
        uuid id PK
        uuid arena_id FK
        varchar phase
        varchar interrupted_from_phase
        integer session_duration_minutes
        integer judging_duration_minutes
    }
    iam_users {
        uuid id PK
        varchar name
    }
    judging_score_criteria {
        uuid id PK
        uuid arena_id FK
        varchar criteria_name
    }
    judging_round_judges {
        uuid id PK
        uuid round_id FK
        uuid judge_user_id FK
    }
    competition_registration_items {
        uuid id PK
        uuid registration_id FK
        uuid round_id FK
        varchar status
    }
    judging_round_scores {
        uuid id PK
        uuid registration_item_id FK
        uuid round_judge_id FK
        uuid criteria_id FK
        numeric score_value
        integer revision_number
    }
    judging_score_revisions {
        uuid id PK
        uuid score_id FK
        uuid operation_id
        integer revision_number
        numeric score_value
        timestamptz submitted_at
    }
    media_round_recordings {
        uuid id PK
        uuid registration_item_id FK, UK
        text video_url
        text audio_url
        varchar recording_status
        varchar delivery_status
        varchar recipient_phone_snapshot
        timestamptz sent_at
    }
    competition_registrations {
        uuid id PK
        uuid arena_id FK
        varchar registrant_phone
    }

    asset_arenas ||..o{ competition_rounds : menyelenggarakan
    asset_arenas ||..o{ judging_score_criteria : menentukan_kriteria
    competition_rounds ||..o{ judging_round_judges : menugaskan
    iam_users ||..o{ judging_round_judges : menjadi_juri
    competition_rounds ||..o{ competition_registration_items : diikuti
    competition_registrations ||..o{ competition_registration_items : berisi
    competition_registration_items ||..o{ judging_round_scores : dinilai
    judging_round_judges ||..o{ judging_round_scores : memberi_nilai
    judging_score_criteria ||..o{ judging_round_scores : diukur
    judging_round_scores ||..o{ judging_score_revisions : riwayat_revisi
    competition_registration_items ||..o| media_round_recordings : direkam_dan_dikirim
```

| Aturan | Makna pada rancangan |
| --- | --- |
| Juri | Pasangan `(round_id, judge_user_id)` unik. Akun harus memiliki penugasan juri pada arena ronde. Daftar juri dikunci saat ronde dimulai. |
| Kriteria | Satu daftar kriteria per arena untuk seluruh ronde. Keunikan nama berlaku per arena. Asumsi review ini membatasi perubahan daftar dan arti kriteria setelah mulai dipakai. |
| Fase ronde | `PREPARED → RUNNING → JUDGING → CLOSED`, dengan `INTERRUPTED` dan `CANCELLED`. Operator mengendalikan perpindahan fase. |
| Waktu penilaian | Submit dan revisi diterima pada fase `JUDGING` untuk item terkonfirmasi. |
| Kelengkapan | Seluruh item terkonfirmasi, juri yang ditugaskan, dan kriteria arena harus sudah dinilai sebelum penutupan. |
| Nilai | Satu skor berlaku per kombinasi item–juri ronde–kriteria. Nilai bulat 1–10 dan berbobot sama. Skor terkini dan history revisi diperbarui bersama. |
| Konsistensi | Item dan juri pada skor harus berasal dari ronde yang sama. Kriteria harus berasal dari arena ronde tersebut. |
| Pengiriman ulang | ID operasi mengidentifikasi kiriman yang diulang. Nomor revisi membantu memeriksa konflik perubahan. |
| Konektivitas | Cloud menjadi sumber nilai resmi. Gangguan koneksi engine menginterupsi arena terkait dan resume dilakukan operator. |
| Media | Engine/worker sistem menulis metadata rekaman dan status pengiriman. Rekaman terhubung ke item registrasi. |
| Penerima | Worker menyalin kontak pendaftar dari header registrasi ke `recipient_phone_snapshot` pada rekaman saat menyiapkan pengiriman. |
| Jadwal | Durasi 10 menit lomba, 15 menit penjurian, jeda siang, dan batas pukul 17 menjadi panduan operator. |

Contoh satu ronde dengan 24 item terkonfirmasi, 2 juri, dan 3 kriteria membutuhkan **144 nilai kriteria**. Item registrasi sudah menyediakan daftar peserta, sedangkan `round_judges` menyediakan daftar juri yang bertugas.

## Cara membaca relasi

Pada diagram ER, `||` berarti tepat satu, `|o` atau `o|` berarti nol atau satu, dan `o{` berarti nol atau banyak. FK opsional dapat kosong, seperti akun pemilik, akun pendaftar manual, dan gantangan sebelum item dikonfirmasi.

Lingkup arena diperoleh melalui event/ronde, registrasi, dan master aset. `arena_id` ditampilkan pada titik lingkup utama. Pada tahap SQL, kombinasi relasi perlu menjamin bahwa kategori, item registrasi, gantangan, juri, dan kriteria saling sesuai. Batas tiga perangkat berlaku global per akun.

## Ringkasan penyederhanaan

| Bagian | Rancangan untuk review |
| --- | --- |
| Peserta ronde | Item registrasi menyimpan status serta gantangan dan langsung dipakai skor/rekaman. |
| Penugasan juri | `round_judges` menunjuk `users` melalui `judge_user_id`. |
| Kriteria | Daftar per arena dipakai seluruh ronde dan dikunci setelah mulai digunakan. |
| Pengiriman video | Status kirim, tujuan, dan waktu kirim disimpan pada rekaman. |
| Fase ronde | Fase sekarang dan fase asal interupsi disimpan pada ronde. |
| Revisi nilai | Skor berlaku mempunyai history revisi. |
| Transfer pemilik | Pemilik aktif disimpan pada burung dengan history transfer admin. |

Rancangan tetap memuat multi-arena, kategori buatan admin arena, registrasi multi-item, pembayaran gabungan, serta pendaftar manual dengan akun opsional. Tabel pendukung akun seperti auth provider dan paket/langganan membership dapat dilihat pada skema sumber. [Skema sumber saat ini](../../db-dump/full_db.dump)
