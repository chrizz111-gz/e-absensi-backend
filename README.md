# E-Absensi Backend

Backend REST API untuk sistem **E-Absensi** sekolah. Dibangun dengan **Go + Gin + GORM + PostgreSQL**.

**Epic 1 — Authentication & Account** tersedia (registrasi, login, sesi/JWT, OTP, ganti/reset password, Google Sign-In, otorisasi role, rate limit). **Epic 2 — User & Role Management** selesai, meliputi fondasi RBAC (permission middleware, daftar role, matriks hak akses) dan CRUD user/role Fase 3 (daftar, detail, buat akun staf, status aktif/nonaktif, ganti role, ubah profil, hapus akun).

> Dokumen ini ditujukan terutama untuk **tim frontend (Mobile, Web, Desktop)**. Berisi cara menjalankan server, konfigurasi, skenario uji, dan catatan integrasi. Untuk **kontrak API lengkap** (endpoint, kode error, role), lihat **[ENDPOINTS.md](ENDPOINTS.md)**.

---

## Daftar Isi

- [Tech Stack](#tech-stack)
- [Struktur Proyek](#struktur-proyek)
- [Menjalankan Server (Lokal)](#menjalankan-server-lokal)
- [Quick Start (Coba Cepat)](#quick-start-coba-cepat)
- [Environment Variables](#environment-variables)
- [Dokumentasi API per Epic](#dokumentasi-api-per-epic)
  - [Epic 1 — Authentication & Account](#epic-1--authentication--account)
  - [Epic 2 — User & Role Management](#epic-2--user--role-management)
- [Skenario Uji Manual](#skenario-uji-manual)
- [Catatan untuk Tim Frontend](#catatan-untuk-tim-frontend)
- [Konfigurasi CORS](#konfigurasi-cors)
- [Roadmap](#roadmap)

---

## Tech Stack

| Komponen | Teknologi |
|---|---|
| Bahasa | Go 1.26 |
| Web framework | Gin |
| ORM | GORM |
| Database | PostgreSQL 18 |
| Auth | JWT (HS256), refresh token dengan rotasi |
| Password hashing | bcrypt |
| Google Sign-In | `google.golang.org/api/idtoken` |

Arsitektur: **Controller → Service → Repository** (routing tipis, logika di service, query di repository).

---

## Struktur Proyek

```
e-absensi-backend/
├── main.go                 # Entrypoint: load .env, connect DB, jalankan server
├── config/                 # Konfigurasi app + koneksi database
├── models/                 # Model GORM (users, roles, permissions, user_roles, role_permissions, otp_codes, sessions)
├── repositories/           # Query database (termasuk permission matrix)
├── services/               # Logika bisnis (auth, otp, password, user CRUD)
├── controllers/            # Handler HTTP (auth, OTP, RBAC, user admin)
├── middlewares/            # AuthRequired, RequireRoles, RequirePermission, RateLimit, ErrorHandler
├── routes/                 # Registrasi seluruh route
├── dto/                    # Request/Response DTO
├── utils/                  # JWT, password, OTP, mailer, response envelope, error
├── database/               # (repo terpisah) Skrip SQL migrasi - clone dari Akyrr/absensi
└── .env                    # Kredensial (JANGAN di-commit)
```

---

## Menjalankan Server (Lokal)

### Prasyarat

- Go 1.26+
- PostgreSQL 18 (service `postgresql-x64-18`)

### 1. Siapkan database

Skema database berada di **repo terpisah** (dikelola tim DB) dan **tidak ikut** ter-clone bersama repo ini. Clone dulu ke folder `database/`:

```bash
git clone https://github.com/Akyrr/absensi.git database
```

Kalau folder `database/` sudah ada, ambil update terbaru:

```powershell
git -C database pull
```

Semua file migrasi sekarang berada di `database\sql\`. Jalankan sesuai epic secara berurutan lewat `psql` (Windows: `C:\Program Files\PostgreSQL\18\bin\psql.exe`):

```powershell
$env:PGPASSWORD="<password>"
# Epic 1
psql -U <user> -h 127.0.0.1 -d absensi -f database\sql\001_epic1_auth.sql
psql -U <user> -h 127.0.0.1 -d absensi -f database\sql\002_access_and_encoding.sql
psql -U <user> -h 127.0.0.1 -d absensi -f database\sql\003_add_failed_login_at.sql
psql -U <user> -h 127.0.0.1 -d absensi -f database\sql\005_otp_length_comment.sql

# Epic 2 (wajib untuk endpoint role/permission)
psql -U <user> -h 127.0.0.1 -d absensi -f database\sql\004_epic2_user_role_management.sql
```

> `005` hanya menyinkronkan komentar tabel OTP, tidak mengubah struktur.
> `004_epic2_user_role_management.sql` menambahkan metadata role, tabel `permissions`/`role_permissions`, dan seed 7 permission Epic 2.
> Untuk DB yang sudah berjalan, jalankan **hanya file baru** yang belum pernah diterapkan.


### 2. Konfigurasi `.env`

Salin template lalu isi secret JWT secara otomatis (secret **tidak** disimpan di repo):

```powershell
Copy-Item .env.example .env
./scripts/gen-secrets.ps1
```

> **macOS/Linux:** `cp .env.example .env && ./scripts/gen-secrets.sh`
>
> File `.env` sudah masuk `.gitignore` — **jangan pernah** di-commit. Alternatif cepat: minta file `.env` dev ke maintainer lewat chat privat.
> Nilai lain (Google, SMTP, dll.) opsional — lihat [Environment Variables](#environment-variables).

### 3. Jalankan

```bash
go mod download
go run .
```

Server default berjalan di **`http://localhost:8080`**.

Cek kesehatan:

```bash
curl http://localhost:8080/api/status
```

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "Sistem Backend E-Absensi Siap!",
  "data": { "database": "ok" }
}
```

### Menjalankan test

```bash
go test ./...
```

---

## Quick Start (Coba Cepat)

Setelah server berjalan (`go run .`), buka **terminal kedua** untuk mengirim request.

> **Windows PowerShell:** jangan pakai `curl.exe -d '{...}'` untuk mengirim JSON — PowerShell 5.1 **membuang tanda kutip ganda** sehingga server menerima JSON rusak (`unexpected EOF` / `invalid character`). Pakai **`Invoke-RestMethod`** (paling praktis) atau `curl.exe -d "@file.json"`. Lihat catatan lengkap di [Catatan untuk Tim Frontend](#catatan-untuk-tim-frontend).

**1. Helper (opsional, tempel sekali per sesi PowerShell):**

```powershell
function Call($method, $path, $body, $token) {
  $headers = @{}
  if ($token) { $headers["Authorization"] = "Bearer $token" }
  $p = @{ Method=$method; Uri="http://localhost:8080$path"; Headers=$headers }
  if ($body) { $p["Body"] = ($body | ConvertTo-Json); $p["ContentType"] = "application/json" }
  try { Invoke-RestMethod @p | ConvertTo-Json -Depth 6 }
  catch { "STATUS $([int]$_.Exception.Response.StatusCode): $($_.ErrorDetails.Message)" }
}
```

**2. Cek server hidup:**

```powershell
Call Get "/api/status"
```

**3. Registrasi siswa:**

```powershell
$body = @{
  name     = "Budi Santoso"
  email    = "budi@example.com"
  nisn             = "1234567890"
  password         = "Rahasia123!"
  confirm_password = "Rahasia123!"
  username         = "budi"
} | ConvertTo-Json

Invoke-RestMethod -Method Post -Uri http://localhost:8080/api/v1/auth/register `
  -ContentType "application/json" -Body $body | ConvertTo-Json -Depth 5
```

**4. Login → ambil token:**

```powershell
$login = Invoke-RestMethod -Method Post -Uri http://localhost:8080/api/v1/auth/login `
  -ContentType "application/json" `
  -Body (@{ identifier="budi@example.com"; password="Rahasia123!" } | ConvertTo-Json)

$tok = $login.data.access_token    # access token (15 menit)
$ref = $login.data.refresh_token   # refresh token (7 hari)
$login.data.user.roles             # -> siswa
```

**5. Pakai token:**

```powershell
Call Post "/api/v1/auth/logout-all" @{} $tok
```

> **OTP di mode dev:** karena `SMTP_HOST` kosong, kode OTP (6 karakter alfanumerik uppercase, `A-Z` + `2-9`) dicetak ke **konsol tempat server berjalan**:
> `[DEV-EMAIL] OTP purpose=change_password untuk Budi <budi@example.com> => A7K2Q9`

---

## Environment Variables

| Variabel | Default | Keterangan |
|---|---|---|
| `APP_ENV` | `development` | `development` atau `production` |
| `APP_PORT` | `8080` | Port HTTP server |
| `DB_USER` | — | User PostgreSQL |
| `DB_PASSWORD` | — | Password PostgreSQL |
| `DB_HOST` | — | Host DB (mis. `127.0.0.1`) |
| `DB_PORT` | — | Port DB (mis. `5432`) |
| `DB_NAME` | — | Nama database (`absensi`) |
| `DB_SSLMODE` | — | Umumnya `disable` untuk lokal |
| `JWT_SECRET` | — | Secret access token |
| `JWT_REFRESH_SECRET` | — | Secret refresh token |
| `JWT_ACCESS_TTL` | `15m` | Masa berlaku access token |
| `JWT_REFRESH_TTL` | `168h` | Masa berlaku refresh token (7 hari) |
| `GOOGLE_CLIENT_ID` | _(kosong)_ | Client ID OAuth Google. **Kosong = fitur Google Sign-In nonaktif** |
| `SMTP_HOST` | _(kosong)_ | Host SMTP. **Kosong = OTP di-log ke konsol (mode dev)** |
| `SMTP_PORT` | `2525` | Port SMTP |
| `SMTP_USER` | _(kosong)_ | User SMTP |
| `SMTP_PASS` | _(kosong)_ | Password SMTP |
| `SMTP_FROM` | `E-Absensi <no-reply@...>` | Alamat pengirim |
| `OTP_LENGTH` | `6` | Panjang OTP dalam karakter (4–10), charset `A-Z` + `2-9` |
| `OTP_TTL` | `5m` | Masa berlaku kode OTP |
| `OTP_RESEND_COOLDOWN` | `60s` | Jeda minimum sebelum kirim ulang OTP |
| `OTP_MAX_RESEND_PER_HOUR` | `5` | Batas jumlah pengiriman OTP per jam |
| `OTP_MAX_ATTEMPTS` | `5` | Maks percobaan salah sebelum kode hangus |
| `PASSWORD_RESET_TOKEN_TTL` | `10m` | Umur reset token setelah OTP diverifikasi |
| `RATE_LIMIT_RPS` | `5` | Rate limit per-IP (request/detik). `0` = nonaktif |
| `RATE_LIMIT_BURST` | `10` | Burst rate limit per-IP |
| `CORS_ALLOWED_ORIGINS` | _(kosong)_ | Daftar origin web yang diizinkan (dipisah koma). Kosong = default dev (React `:3000` & `:5173`). Lihat [Konfigurasi CORS](#konfigurasi-cors) |

---

## Dokumentasi API per Epic

Kontrak API lengkap dipindah ke file terpisah agar dokumen ini tetap ringkas:

➡️ **[ENDPOINTS.md](ENDPOINTS.md)** — konvensi response, alur autentikasi (JWT & sesi), daftar endpoint, kode error, role, dan rate limit.

### Epic 1 — Authentication & Account

Tersedia di prefix **`/api/v1/auth`**:

- Registrasi siswa (`POST /register`) dengan `confirm_password` wajib.
- Login email/username, Google Sign-In, refresh token dengan rotasi, logout satu device/semua device.
- OTP ganti & reset password: **6 karakter alfanumerik uppercase**, charset `A-Z` + `2-9`, maks **5 percobaan**.
- Password policy 8–72 byte, wajib huruf kecil/besar/angka/simbol, tanpa spasi; password baru tidak boleh sama dengan password lama.
- Access token **15 menit** (header `Authorization: Bearer`), refresh token **7 hari**.
- Otorisasi role (`RequireRoles`) dan rate limit per-IP.

### Epic 2 — User & Role Management

Tersedia di prefix **`/api/v1`**:

- `GET /roles` (`role.assign`) dan `GET /permissions/matrix` (`permission.view`) — read-only RBAC.
- CRUD user admin:
  - `GET /users` — pagination (`page`, `limit`), pencarian `q` pada nama/email/NISN (escape `%`/`_`), filter `role[]`, `status`, dan `include_archived`, urut default `name A-Z`. Response memakai `data.items` + `data.meta`.
  - `GET /users/{id}` — detail user + role.
  - `POST /users` — buat akun staf tanpa password; hanya menerima `name`, `email`, dan `roles[]`; `roles[]` hanya role `assignable=true`.
  - `PATCH /users/{id}/status` — aktif/nonaktif; nonaktif wajib `inactive_reason`, mencabut semua sesi, tidak boleh menonaktifkan diri sendiri/Admin aktif terakhir, dan menolak status yang sama dengan `409 ACCOUNT_ALREADY_ACTIVE`/`ACCOUNT_ALREADY_INACTIVE`.
  - `PUT /users/{id}/roles` — ganti role assignable, role terkunci (`wali_kelas`, `siswa_inti`, `siswa`) dipertahankan; kombinasi tidak valid memakai `422 INVALID_ROLE_COMBINATION`.
  - `PATCH /users/{id}` — ubah nama/email; `updated_at` opsional untuk optimistic locking; email berubah mengosongkan `google_id`; simpan tanpa perubahan tetap sukses tanpa event baru.
  - `DELETE /users/{id}` — hanya akun staf, bukan diri sendiri/Admin terakhir, dan menolak akun yang masih punya riwayat data.

Semua endpoint `/users` wajib **Bearer token + role `admin` + permission granular** (`account.view`, `account.create`, `account.update`, `account.set_status`, `account.delete`, `role.assign`). Response user **tidak pernah** memuat `password_hash` atau `google_id`.

**Kontrak umum:**

- **Base URL (lokal):** `http://localhost:8080`
- Semua response memakai envelope `{ success, code, message, data }` (+ `errors[]` saat error validasi/konflik).
- `errors[]` berbentuk objek `{ field, code, message }`, bukan array string.
- Branching di frontend **wajib** memakai field `code`, bukan `message`.
- Detail endpoint, status HTTP, dan kode error: lihat **[ENDPOINTS.md](ENDPOINTS.md)**.

---

## Skenario Uji Manual

Daftar skenario untuk memverifikasi Epic 1 & 2 secara manual (PowerShell). Kolom **Hasil**: ✅ = sudah diverifikasi pada server lokal; ⬜ = belum diuji ulang setelah perubahan terakhir.

| # | Skenario | Aksi | Diharapkan | Hasil |
|---|---|---|---|---|
| 1 | Health check | `GET /api/status` | 200 `SUCCESS` | ✅ `SUCCESS` |
| 2 | Registrasi baru | `POST /register` (email/NISN baru) | 201 `AUTH_REGISTER_SUCCESS`, role `siswa` | ✅ 201 `AUTH_REGISTER_SUCCESS` |
| 3 | Registrasi duplikat | `POST /register` (email sama) | 409 `CONFLICT` | ✅ 409 `CONFLICT` |
| 4 | Validasi field | `POST /register` (email/NISN/password tidak valid) | 422 `VALIDATION_ERROR` + `errors[]` | ✅ 422 `VALIDATION_ERROR` |
| 5 | Login benar | `POST /login` | 200 `AUTH_LOGIN_SUCCESS` | ✅ 200 `AUTH_LOGIN_SUCCESS` |
| 6 | Refresh (rotasi) | `POST /refresh` token valid | 200 `AUTH_REFRESH_SUCCESS`, token baru | ✅ 200 `AUTH_REFRESH_SUCCESS` |
| 7 | Refresh token lama | `POST /refresh` token **lama** (sudah dipakai) | 401 `AUTH_SESSION_REVOKED` | ✅ 401 `AUTH_SESSION_REVOKED` |
| 8 | Request OTP ganti pw | `POST /change-password/request` | 200 `OTP_REQUESTED`, `attempts_remaining=5`, `expires_at` | ⬜ belum diuji ulang |
| 9 | Cooldown OTP | request lagi < 60 detik | 429 `OTP_RESEND_TOO_SOON` + `retry_after_seconds` + header `Retry-After` | ⬜ belum diuji ulang |
| 10 | OTP salah | `POST /change-password/verify-otp` kode ngawur | 400 `OTP_INVALID` + `attempts_remaining` menurun | ⬜ belum diuji ulang |
| 11 | OTP benar | `POST /change-password/verify-otp` kode dari konsol | 200 `OTP_VERIFIED` + `reset_token` | ⬜ belum diuji ulang |
| 12 | Simpan password baru | `POST /change-password/confirm` dengan `reset_token` | 200 `AUTH_PASSWORD_CHANGED` | ⬜ belum diuji ulang |
| 13 | Sesi lama dicabut | `POST /refresh` token sebelum ganti pw | 401 `AUTH_SESSION_REVOKED` | ⬜ belum diuji ulang |
| 14 | Password lama mati | `POST /login` password lama | 401 `AUTH_INVALID_CREDENTIALS` | ⬜ belum diuji ulang |
| 15 | Password baru jalan | `POST /login` password baru | 200 `AUTH_LOGIN_SUCCESS` | ⬜ belum diuji ulang |
| 16 | Reset (forgot) | `POST /forgot-password` → `verify-otp` → `reset` | 200 `OTP_REQUESTED` → `OTP_VERIFIED` → `AUTH_PASSWORD_CHANGED` | ⬜ belum diuji ulang |
| 17 | Anti-enumerasi | `POST /forgot-password` email **tak terdaftar** | 200 `OTP_REQUESTED` (tidak membocorkan) | ✅ 200 `OTP_REQUESTED` |
| 18 | Google belum diatur | `POST /google` (`GOOGLE_CLIENT_ID` kosong) | 503 `AUTH_GOOGLE_DISABLED` | ✅ 503 `AUTH_GOOGLE_DISABLED` |
| 19 | Logout satu device | `POST /logout` | 200 `AUTH_LOGOUT_SUCCESS` | ✅ 200 `AUTH_LOGOUT_SUCCESS` |
| 20 | Logout semua | `POST /logout-all` | 200 `AUTH_LOGOUT_SUCCESS` | ✅ 200 `AUTH_LOGOUT_SUCCESS` |
| 21 | Lockout akun | 5× login gagal | gagal #1–4 = 401, **#5 = 429 `AUTH_ACCOUNT_LOCKED`** | ✅ #1–4 401, #5 429 |
| 22 | CORS origin diizinkan | `OPTIONS` dengan `Origin: http://localhost:3000` | 204 + `Access-Control-Allow-Origin` | ✅ 204 + header |
| 23 | CORS origin terlarang | `OPTIONS` dengan `Origin: http://evil.example.com` | 403 | ✅ 403 |
| 24 | Rate limit | 40 request cepat ke `/login` | sebagian `429 RATE_LIMITED` + `Retry-After: 60` | ✅ 11 lolos / 29 = 429, `Retry-After: 60` |
| 25 | OTP 5× salah | salah 5× lalu coba kode benar | salah #1–4 = 400, **#5 = 429 `OTP_TOO_MANY_ATTEMPTS`**; kode benar tetap ditolak | ⬜ belum diuji ulang |
| 26 | Password lemah | register/ganti dengan `aaaaaaaa` | 422 `VALIDATION_ERROR` + `errors[].code = PASSWORD_*` | ⬜ belum diuji ulang |
| 27 | Konfirmasi beda | `confirm_password` ≠ `new_password` | 422 `VALIDATION_ERROR` + `PASSWORD_MISMATCH` | ⬜ belum diuji ulang |
| 28 | Password sama | password baru = password lama | 400 `AUTH_PASSWORD_SAME` | ⬜ belum diuji ulang |
| 29 | Reset token dipakai 2× | `POST /change-password/confirm` dengan token yang sama | 401 `AUTH_RESET_TOKEN_INVALID` | ⬜ belum diuji ulang |
| 30 | Daftar role | `GET /roles` dengan token Admin | 200 `SUCCESS`, data `roles[]` + `assignable` | ✅ 200, 7 role |
| 31 | Matriks permission | `GET /permissions/matrix` dengan token Admin | 200 `SUCCESS`, data `roles`, `permissions`, `grants` | ✅ 200, 7 permission/7 grant |
| 32 | Default deny | `GET /roles` dengan token siswa | 403 `FORBIDDEN` | ✅ 403 `FORBIDDEN` |
| 33 | Tanpa token | `GET /roles` tanpa header Bearer | 401 `UNAUTHORIZED` | ✅ 401 `UNAUTHORIZED` |
| 34 | Daftar user (Admin) | `GET /users` dengan token Admin | 200 `USER_LIST`, `data.items` + `data.meta` default `page=1`, `limit=20` | ✅ 200 `USER_LIST` |
| 35 | Urutan default | `GET /users` | Nama terurut A–Z | ✅ `Budi Santoso` → `Phase Three Admin` |
| 36 | Escape wildcard `q` | `GET /users?q=%25` dan `?q=_` | Hanya mencocokkan karakter literal `%`/`_` | ✅ masing-masing 1 user |
| 37 | Filter role & status | `GET /users?role[]=guru_mapel&status=active` | 200, hanya user dengan role/status tersebut | ✅ 200 |
| 38 | Validasi query | `?page=0`, `?limit=101`, `?status=deleted`, role tidak dikenal | 422 `VALIDATION_ERROR` | ✅ 422 |
| 39 | Detail user | `GET /users/{id}` | 200, `data` `UserAdminResponse` + `roles[]` | ✅ 200 |
| 40 | Buat akun staf | `POST /users` (`roles: ["guru_mapel"]`) | 201 `CREATED`, `password_hash=NULL`, `is_active=true` | ✅ 201 |
| 41 | Email duplikat (create) | `POST /users` dengan email terdaftar | 409 `EMAIL_ALREADY_EXISTS` | ✅ 409 |
| 42 | Role terkunci (create) | `POST /users` (`roles: ["wali_kelas"]`) | 422 `ROLE_NOT_ASSIGNABLE` | ✅ 422 |
| 43 | Ubah profil | `PATCH /users/{id}` dengan/tanpa `updated_at` | 200 `SUCCESS` | ✅ 200 |
| 44 | Optimistic locking | `PATCH /users/{id}` dengan `updated_at` lama | 409 `STALE_DATA` | ✅ 409 |
| 45 | Email berubah → Google dicabut | `PATCH /users/{id}` email baru | `google_id` di DB menjadi `NULL` | ✅ `NULL` |
| 46 | Email duplikat (update) | `PATCH /users/{id}` email milik user lain | 409 `EMAIL_ALREADY_EXISTS` | ✅ 409 |
| 47 | Email tidak valid | `PATCH /users/{id}` email rusak | 422 `INVALID_EMAIL` | ✅ 422 |
| 48 | Ganti role + role terkunci | `PUT /users/{id}/roles` pada user ber-role `wali_kelas` | 200, `wali_kelas` tetap ada | ✅ `waka_kurikulum,wali_kelas` |
| 49 | Nonaktif + cabut sesi | `PATCH /users/{id}/status` `is_active=false` | 200; semua sesi user `revoked_at` terisi; refresh lama 401 | ✅ 0 sesi aktif |
| 50 | Aktifkan kembali | `PATCH /users/{id}/status` `is_active=true` | 200; `inactive_reason`/`deactivated_at` NULL, `failed_login_attempts=0` | ✅ |
| 51 | Reason wajib/valid | `is_active=false` tanpa/`inactive_reason` salah | 422 `FIELD_REQUIRED` / `INVALID_INACTIVE_REASON` | ✅ 422 |
| 52 | Akun siswa dikunci | `PATCH`/`DELETE /users/{id}` target siswa | 422 `ACCOUNT_TYPE_NOT_EDITABLE` | ✅ 422 |
| 53 | Aksi pada diri sendiri | `DELETE`/`PATCH status`/cabut admin sendiri | 403 `SELF_ACTION_FORBIDDEN` | ✅ 403 |
| 54 | Non-admin | `GET /users` dengan token siswa | 403 `FORBIDDEN` | ✅ 403 |
| 55 | Hapus akun staf | `DELETE /users/{id}` | 200 `SUCCESS` | ✅ 200 |
| 56 | Hapus akun berriwayat | `DELETE` user yang direferensikan tabel riwayat | 409 `ACCOUNT_HAS_HISTORY` | ✅ 409 |
| 57 | Tanpa token (users) | `GET /users` tanpa header Bearer | 401 `UNAUTHORIZED` | ✅ 401 |
| 58 | Status sudah sama | `PATCH status` nonaktif lagi / aktif lagi | 409 `ACCOUNT_ALREADY_INACTIVE` / `ACCOUNT_ALREADY_ACTIVE` | ✅ 409 |
| 59 | Role kosong/tidak valid | `PUT /users/{id}/roles` `[]` atau campur staf+siswa | 422 `INVALID_ROLE_COMBINATION` | ✅ 422 |

> **Belum diuji (butuh kredensial eksternal, bukan bug):** jalur **sukses** Google Sign-In (perlu `GOOGLE_CLIENT_ID` + `id_token` asli) dan pengiriman email via **SMTP asli** (selama `SMTP_HOST` kosong, OTP dicetak ke konsol).

---

## Catatan untuk Tim Frontend

**Umum**
- Gunakan field `code` untuk logika percabangan.
- Simpan `access_token` & `refresh_token` di secure storage (Keychain/Keystore, untuk web hindari `localStorage` bila memungkinkan).
- Setiap `401 AUTH_TOKEN_EXPIRED` → panggil `/refresh` **sekali**, lalu ulangi request. Bila refresh gagal → paksa login ulang.
- Setiap `AUTH_SESSION_REVOKED` → bersihkan token & login ulang.
- Setelah **ganti/reset password** atau **logout-all** → paksa login ulang (sesi lama hangus).
- Perhatikan `device_info` saat login untuk memudahkan audit sesi.
- **OTP sekarang 6 karakter alfanumerik uppercase** (contoh `A7K2Q9`), memakai `A-Z` + `2-9`; karakter `0/O/1/I/L` dihindari agar tidak ambigu.
- Alur ganti/reset password: `request` → `verify-otp` (dapat `reset_token`) → `confirm`/`reset` (kirim `reset_token` + `new_password` + `confirm_password`).
- `reset_token` berumur pendek dan **sekali pakai**; minta OTP baru otomatis membatalkan token lama.
- Response OTP menyertakan `attempts_remaining`, `expires_at`, dan `retry_after_seconds` (saat cooldown).
- Field error menyebut nama kolom: contoh `{ "field": "email", "code": "EMAIL_ALREADY_EXISTS", "message": "..." }` untuk manajemen akun; registrasi memakai `EMAIL_TAKEN`.
- `confirm_password` wajib pada register, change password, dan reset password; nilainya harus sama dengan password baru.
- `GET /roles` dan `GET /permissions/matrix` wajib Bearer token; masing-masing membutuhkan permission `role.assign` dan `permission.view`. Jangan hanya menyembunyikan menu di UI — server tetap menolak dengan `403 FORBIDDEN` bila permission tidak ada.
- Semua endpoint `/api/v1/users` wajib **Bearer token + role `admin`**. UI Admin sebaiknya tetap menyembunyikan aksi yang tidak diizinkan, tetapi server adalah sumber kebenaran.
- Response user **tidak pernah** memuat `password_hash` atau `google_id`. `POST /users` membuat akun staf tanpa password; jangan mengirim field password ke endpoint tersebut.
- Payload `roles` memakai **nama role** (contoh `guru_mapel`), bukan `id`. Hanya role dengan `assignable: true` yang boleh dikirim. Role terkunci (`wali_kelas`, `siswa_inti`, `siswa`) dipertahankan otomatis saat `PUT /users/{id}/roles`.
- `PATCH /users/{id}` menerima `updated_at` **opsional** dari response detail. Bila dikirim dan sudah tidak cocok, server membalas `409 STALE_DATA`; GET ulang detail lalu ulangi perubahan — jangan retry dengan `updated_at` lama.
- Menonaktifkan akun mencabut seluruh sesinya; user tersebut akan menerima `401 UNAUTHORIZED` pada request berikutnya. Aktivasi mereset alasan nonaktif dan counter login gagal.
- Aksi pada akun sendiri (`DELETE`, ubah status, cabut role Admin sendiri) ditolak `403 SELF_ACTION_FORBIDDEN`. Penghapusan akun yang masih punya riwayat data ditolak `409 ACCOUNT_HAS_HISTORY`.
- `GET /users` mendukung `q` (nama/email/NISN), `role[]` (nama role, boleh lebih dari satu), `status=active|inactive`, `include_archived`, `page`, dan `limit`. Karakter `%`/`_` pada `q` dicari sebagai teks literal. Akun terarsip disembunyikan secara default.

**Mobile & Desktop**
- Simpan token di secure storage platform.
- Google Sign-In: gunakan SDK native (Google Sign-In Android/iOS, atau `google_sign_in`) untuk memperoleh `id_token`, lalu kirim ke `/auth/google`.

**Web**
- Google Sign-In: Google Identity Services (GIS) → `id_token` → `/auth/google`.
- CORS sudah aktif. Origin dev default (React): `http://localhost:3000` (CRA) dan `http://localhost:5173` (Vite). Detail & cara ganti: lihat [Konfigurasi CORS](#konfigurasi-cors).

**Windows PowerShell (pengujian manual)**
- **Jangan** kirim JSON inline dengan `curl.exe -d '{...}'` di PowerShell 5.1 — tanda kutip ganda dibuang sehingga server menerima JSON rusak (`unexpected EOF` / `invalid character`). Pilih salah satu:
  - **`Invoke-RestMethod`** (paling praktis):
    ```powershell
    Invoke-RestMethod -Method Post -Uri http://localhost:8080/api/v1/auth/login `
      -ContentType "application/json" `
      -Body (@{ identifier="budi@example.com"; password="Rahasia123!" } | ConvertTo-Json)
    ```
  - **File body** (bila tetap ingin `curl.exe`): simpan `{"identifier":"...","password":"..."}` ke `body.json`, lalu
    ```powershell
    curl.exe -s -X POST http://localhost:8080/api/v1/auth/login -H "Content-Type: application/json" -d "@body.json"
    ```
- Kirim token lewat header: `Authorization: Bearer <access_token>`.
- Untuk melihat **status code** + isi body saat gagal, bungkus dengan `try/catch`:
  ```powershell
  try {
    Invoke-RestMethod -Method Post -Uri $url -ContentType "application/json" -Body ($body | ConvertTo-Json)
  } catch {
    "STATUS $([int]$_.Exception.Response.StatusCode): $($_.ErrorDetails.Message)"
  }
  ```
- Kode **OTP** (mode dev) tercetak di **konsol tempat server berjalan**, bukan di response.

---

## Konfigurasi CORS

CORS mengatur **website (origin) mana yang browser-nya boleh memanggil API**. Ini **hanya berlaku untuk frontend web** — mobile & desktop tidak terpengaruh.

- **Default (dev, bila `CORS_ALLOWED_ORIGINS` kosong):**
  `http://localhost:3000`, `http://localhost:5173`, `http://127.0.0.1:3000`, `http://127.0.0.1:5173`
  (React CRA = `:3000`, Vite = `:5173`.)
- **Mengganti origin** — isi `CORS_ALLOWED_ORIGINS` (dipisah koma), contoh:
  ```
  CORS_ALLOWED_ORIGINS=http://localhost:3000,https://absensi.sekolah.sch.id
  ```
- **Production** — isi domain asli, contoh:
  ```
  CORS_ALLOWED_ORIGINS=https://absensi.sekolah.sch.id,https://admin.absensi.sekolah.sch.id
  ```

Detail teknis: method `GET, POST, PUT, PATCH, DELETE, OPTIONS`; header `Origin, Content-Type, Accept, Authorization`; `Allow-Credentials: true`; preflight `OPTIONS` ditangani otomatis; hasil preflight di-cache 12 jam.

> **Kenapa daftar eksplisit, bukan `*`?** Karena kita memakai `Allow-Credentials: true` (agar browser mengirim header `Authorization`). Aturan CORS melarang kombinasi `*` + credentials, dan daftar eksplisit juga mencegah website lain memanggil API kita.

> **Catatan:** CORS **bukan** pengganti autentikasi. Endpoint yang butuh login tetap wajib menyertakan token yang valid. CORS hanya gerbang "website mana" — bukan "user mana".

---

## Roadmap

- [x] **Epic 1 — Authentication & Account**
  - [x] Registrasi, login manual, sesi + refresh (rotasi)
  - [x] OTP ganti & reset password
  - [x] Google Sign-In
  - [x] Otorisasi role (`RequireRoles`)
  - [x] Rate limit per-IP
  - [x] CORS untuk frontend web
- [x] **Epic 2 — User & Role Management**
  - [x] Fondasi RBAC: `RequirePermission`, `GET /roles`, `GET /permissions/matrix`
  - [x] CRUD user & role (Fase 3)
- [ ] Epic 3 — Absensi
- [ ] Epic 4 — Laporan & rekap
- [ ] … (total 14 Epic sesuai PRD)
