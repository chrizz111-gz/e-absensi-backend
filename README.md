# E-Absensi Backend

Backend REST API untuk sistem **E-Absensi** sekolah. Dibangun dengan **Go + Gin + GORM + PostgreSQL**.

Saat ini baru **Epic 1 — Authentication & Account** yang tersedia (registrasi, login, sesi/JWT, OTP, ganti/reset password, Google Sign-In, otorisasi role, rate limit). Endpoint Epic lain akan menyusul.

> Dokumen ini ditujukan terutama untuk **tim frontend (Mobile, Web, Desktop)**. Berisi cara menjalankan server, konvensi response, alur autentikasi, dan referensi endpoint lengkap.

---

## Daftar Isi

- [Tech Stack](#tech-stack)
- [Struktur Proyek](#struktur-proyek)
- [Menjalankan Server (Lokal)](#menjalankan-server-lokal)
- [Quick Start (Coba Cepat)](#quick-start-coba-cepat)
- [Environment Variables](#environment-variables)
- [Konvensi API](#konvensi-api)
- [Autentikasi (JWT & Sesi)](#autentikasi-jwt--sesi)
- [Referensi Endpoint](#referensi-endpoint)
- [Daftar Kode Error](#daftar-kode-error)
- [Role Pengguna](#role-pengguna)
- [Rate Limit](#rate-limit)
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
├── models/                 # Model GORM (users, roles, user_roles, otp_codes, sessions)
├── repositories/           # Query database
├── services/               # Logika bisnis (auth, otp, password)
├── controllers/            # Handler HTTP
├── middlewares/            # AuthRequired, RequireRoles, RateLimit, ErrorHandler
├── routes/                 # Registrasi seluruh route
├── dto/                    # Request/Response DTO
├── utils/                  # JWT, password, OTP, mailer, response envelope, error
├── database/               # Skrip SQL migrasi (001_, 002_, 003_)
└── .env                    # Kredensial (JANGAN di-commit)
```

---

## Menjalankan Server (Lokal)

### Prasyarat

- Go 1.26+
- PostgreSQL 18 (service `postgresql-x64-18`)

### 1. Siapkan database

Jalankan skrip migrasi secara berurutan lewat `psql` (Windows: `C:\Program Files\PostgreSQL\18\bin\psql.exe`):

```powershell
$env:PGPASSWORD="<password>"
psql -U <user> -h 127.0.0.1 -d absensi -f database\001_epic1_auth.sql
psql -U <user> -h 127.0.0.1 -d absensi -f database\002_access_and_encoding.sql
psql -U <user> -h 127.0.0.1 -d absensi -f database\003_add_failed_login_at.sql
```

### 2. Konfigurasi `.env`

Salin `.env.example` menjadi `.env` lalu isi nilainya (lihat [Environment Variables](#environment-variables)).

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
  nisn     = "1234567890"
  password = "rahasia123"
  username = "budi"
} | ConvertTo-Json

Invoke-RestMethod -Method Post -Uri http://localhost:8080/api/v1/auth/register `
  -ContentType "application/json" -Body $body | ConvertTo-Json -Depth 5
```

**4. Login → ambil token:**

```powershell
$login = Invoke-RestMethod -Method Post -Uri http://localhost:8080/api/v1/auth/login `
  -ContentType "application/json" `
  -Body (@{ identifier="budi@example.com"; password="rahasia123" } | ConvertTo-Json)

$tok = $login.data.access_token    # access token (15 menit)
$ref = $login.data.refresh_token   # refresh token (7 hari)
$login.data.user.roles             # -> siswa
```

**5. Pakai token:**

```powershell
Call Post "/api/v1/auth/logout-all" @{} $tok
```

> **OTP di mode dev:** karena `SMTP_HOST` kosong, kode OTP dicetak ke **konsol tempat server berjalan**:
> `[DEV-EMAIL] OTP purpose=change_password untuk Budi <budi@example.com> => KJWGJDQH`

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
| `RATE_LIMIT_RPS` | `5` | Rate limit per-IP (request/detik). `0` = nonaktif |
| `RATE_LIMIT_BURST` | `10` | Burst rate limit per-IP |
| `CORS_ALLOWED_ORIGINS` | _(kosong)_ | Daftar origin web yang diizinkan (dipisah koma). Kosong = default dev (React `:3000` & `:5173`). Lihat [Konfigurasi CORS](#konfigurasi-cors) |

---

## Konvensi API

- **Base URL (lokal):** `http://localhost:8080`
- **Prefix endpoint auth:** `/api/v1/auth`
- **Format data:** JSON (`Content-Type: application/json`)
- **Semua response** memakai envelope yang sama:

```json
{
  "success": true,
  "code": "AUTH_LOGIN_SUCCESS",
  "message": "Login berhasil",
  "data": { }
}
```

Saat gagal:

```json
{
  "success": false,
  "code": "VALIDATION_ERROR",
  "message": "Validasi data gagal",
  "errors": ["Email harus berupa email yang valid", "Password minimal 8 karakter"]
}
```

| Field | Tipe | Keterangan |
|---|---|---|
| `success` | bool | `true` bila sukses |
| `code` | string | Kode mesin (untuk branching di frontend). Lihat [Daftar Kode Error](#daftar-kode-error) |
| `message` | string | Pesan human-readable (Bahasa Indonesia) |
| `data` | object \| null | Payload sukses (kadang `null`) |
| `errors` | string[] | Hanya ada saat `VALIDATION_ERROR` |

**HTTP status yang dipakai:**

| Status | Arti |
|---|---|
| `200 OK` | Sukses |
| `201 Created` | Registrasi berhasil |
| `400 Bad Request` | Permintaan tidak valid (mis. OTP salah/kedaluwarsa) |
| `401 Unauthorized` | Kredensial/token salah atau kedaluwarsa |
| `403 Forbidden` | Tidak punya hak akses (role/akun nonaktif) |
| `404 Not Found` | Data/akun tak ditemukan |
| `409 Conflict` | Duplikat (email/NISN/username) |
| `422 Unprocessable Entity` | Validasi field gagal |
| `429 Too Many Requests` | Terkena lockout / rate limit / batas OTP |
| `500 Internal Server Error` | Kesalahan server |
| `503 Service Unavailable` | Fitur belum dikonfigurasi (mis. Google) |

---

## Autentikasi (JWT & Sesi)

Alur berbasis **access token** (pendek) + **refresh token** (panjang):

- **Access token** — masa berlaku default **15 menit**. Dikirim di header:
  ```
  Authorization: Bearer <access_token>
  ```
- **Refresh token** — masa berlaku default **7 hari**. Dipakai hanya di `POST /refresh`. **Hanya hash** yang disimpan di server.

### Aturan penting

1. **Rotasi refresh token** — setiap kali `POST /refresh` dipanggil, server menerbitkan **refresh token baru** dan **mencabut yang lama**. Frontend **wajib menyimpan refresh token terbaru**; memakai token lama akan gagal (`401 AUTH_SESSION_REVOKED`).
2. **Multi-device** — setiap login membuat sesi baru. `logout` mencabut 1 device, `logout-all` mencabut semua device.
3. **Ganti/Reset password** otomatis **mencabut semua sesi** → frontend harus memaksa login ulang.
4. **Otorisasi tidak memakai role di token.** Server selalu memuat ulang role user dari database. Perubahan role langsung berlaku.
5. **Lockout brute-force:** 5 kali gagal login dalam 15 menit → akun terkunci 15 menit (`429 AUTH_ACCOUNT_LOCKED`).

---

## Referensi Endpoint

Ringkasan (🔒 = butuh header `Authorization: Bearer`):

| Method | Path | Auth | Deskripsi |
|---|---|---|---|
| GET | `/api/status` | — | Health check |
| POST | `/api/v1/auth/register` | — | Registrasi siswa |
| POST | `/api/v1/auth/login` | — | Login email/username |
| POST | `/api/v1/auth/google` | — | Login via Google |
| POST | `/api/v1/auth/refresh` | — | Tukar refresh token |
| POST | `/api/v1/auth/forgot-password` | — | Minta OTP reset password |
| POST | `/api/v1/auth/forgot-password/resend` | — | Kirim ulang OTP reset |
| POST | `/api/v1/auth/forgot-password/verify` | — | Verifikasi OTP + password baru |
| POST | `/api/v1/auth/logout` 🔒 | Bearer | Logout device ini |
| POST | `/api/v1/auth/logout-all` 🔒 | Bearer | Logout semua device |
| POST | `/api/v1/auth/change-password/request` 🔒 | Bearer | Minta OTP ganti password |
| POST | `/api/v1/auth/change-password/resend` 🔒 | Bearer | Kirim ulang OTP ganti password |
| POST | `/api/v1/auth/change-password/verify` 🔒 | Bearer | Verifikasi OTP + ganti password |

### Objek umum

**UserResponse**

```json
{
  "id": 12,
  "name": "Budi Santoso",
  "email": "budi@example.com",
  "username": "budi",
  "nisn": "1234567890",
  "avatar_url": "https://...",
  "is_active": true,
  "roles": ["siswa"]
}
```

**AuthResponse** (dikembalikan oleh login/refresh/google)

```json
{
  "user": { "...UserResponse..." },
  "access_token": "eyJhbGciOi...",
  "refresh_token": "eyJhbGciOi...",
  "token_type": "Bearer",
  "expires_in": 900
}
```

`expires_in` dalam **detik**.

---

### 1. Registrasi Siswa

`POST /api/v1/auth/register`

Registrasi mandiri siswa. Otomatis mendapat role `siswa`.

**Request body**

| Field | Tipe | Wajib | Aturan |
|---|---|---|---|
| `name` | string | ✅ | 2–150 karakter |
| `email` | string | ✅ | Format email, unik, maks 255 |
| `nisn` | string | ✅ | **Tepat 10 digit angka**, unik |
| `password` | string | ✅ | 8–72 karakter |
| `username` | string | ❌ | 3–50 karakter, unik bila diisi |

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Budi Santoso",
    "email": "budi@example.com",
    "nisn": "1234567890",
    "password": "rahasia123",
    "username": "budi"
  }'
```

**Response `201 Created`**

```json
{
  "success": true,
  "code": "AUTH_REGISTER_SUCCESS",
  "message": "Registrasi berhasil",
  "data": { "...UserResponse..." }
}
```

Error umum: `409 CONFLICT` (email/NISN/username sudah dipakai), `422 VALIDATION_ERROR`.

---

### 2. Login

`POST /api/v1/auth/login`

**Request body**

| Field | Tipe | Wajib | Keterangan |
|---|---|---|---|
| `identifier` | string | ✅ | Email **atau** username |
| `password` | string | ✅ | |
| `device_info` | string | ❌ | Info device, maks 255 |

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{ "identifier": "budi@example.com", "password": "rahasia123", "device_info": "Android 14" }'
```

**Response `200 OK`** → `AuthResponse` (`code: "AUTH_LOGIN_SUCCESS"`).

Error: `401 AUTH_INVALID_CREDENTIALS`, `429 AUTH_ACCOUNT_LOCKED`, `403 AUTH_ACCOUNT_INACTIVE`.

---

### 3. Login Google

`POST /api/v1/auth/google`

**Request body**

| Field | Tipe | Wajib | Keterangan |
|---|---|---|---|
| `id_token` | string | ✅ | ID token dari Google Sign-In SDK |
| `device_info` | string | ❌ | Maks 255 |

Perilaku:

- Akun dengan `google_id` cocok → langsung login.
- Belum tertaut, tapi **email sama** dengan akun terdaftar → akun Google dikaitkan, lalu login.
- Email belum terdaftar → `404 AUTH_ACCOUNT_NOT_FOUND` (diminta registrasi dulu; **tidak** auto-create).
- Email Google harus `email_verified = true`, jika tidak → `401 AUTH_GOOGLE_EMAIL_UNVERIFIED`.
- Bila `GOOGLE_CLIENT_ID` belum diisi di server → `503 AUTH_GOOGLE_DISABLED`.

**Response `200 OK`** → `AuthResponse` (`code: "AUTH_GOOGLE_SUCCESS"`).

> Alur di frontend: pakai SDK Google Sign-In untuk memperoleh `id_token`, lalu kirim ke endpoint ini.

---

### 4. Refresh Token

`POST /api/v1/auth/refresh`

**Request body**

```json
{ "refresh_token": "eyJhbGciOi..." }
```

**Response `200 OK`** → `AuthResponse` (`code: "AUTH_REFRESH_SUCCESS"`).

> Simpan `refresh_token` **baru** dari response dan buang yang lama.

Error: `401 AUTH_SESSION_REVOKED` (token lama/ dicabut), `401 AUTH_TOKEN_EXPIRED` (refresh token kedaluwarsa).

---

### 5. Logout (device ini) 🔒

`POST /api/v1/auth/logout`

**Request body**

```json
{ "refresh_token": "eyJhbGciOi..." }
```

**Response `200 OK`**

```json
{ "success": true, "code": "AUTH_LOGOUT_SUCCESS", "message": "Logout berhasil" }
```

Bersifat idempotent (token tak dikenal tetap dianggap sukses).

---

### 6. Logout Semua Device 🔒

`POST /api/v1/auth/logout-all`

Tanpa body.

**Response `200 OK`**

```json
{
  "success": true,
  "code": "AUTH_LOGOUT_SUCCESS",
  "message": "Semua sesi berhasil dihentikan",
  "data": { "revoked_sessions": 3 }
}
```

---

### 7. Ganti Password (butuh login) 🔒

Alur 2 langkah: **minta OTP** → **verifikasi OTP + password baru**.

#### 7a. Minta / kirim ulang OTP

`POST /api/v1/auth/change-password/request` (atau `/resend`)

Tanpa body (identitas dari access token).

```json
{ "success": true, "code": "OTP_REQUESTED", "message": "Kode OTP telah dikirim ke email Anda" }
```

- Cooldown kirim ulang **60 detik** → `429 OTP_RESEND_TOO_SOON`.
- Maks **5 kali per jam** → `429 OTP_RESEND_LIMIT`.

#### 7b. Verifikasi + ganti password

`POST /api/v1/auth/change-password/verify`

| Field | Tipe | Wajib | Aturan |
|---|---|---|---|
| `code` | string | ✅ | Tepat 8 karakter |
| `new_password` | string | ✅ | 8–72 karakter |

```json
{ "success": true, "code": "AUTH_PASSWORD_CHANGED", "message": "Password berhasil diubah..." }
```

> Semua sesi dicabut → frontend harus mengarahkan user login ulang.

Error: `400 OTP_INVALID`, `400 OTP_EXPIRED`, `429 OTP_TOO_MANY_ATTEMPTS`.

---

### 8. Reset Password (lupa password, publik)

Alur 3 langkah.

#### 8a. Minta / kirim ulang OTP

`POST /api/v1/auth/forgot-password` (atau `/resend`)

**Request body**

```json
{ "email": "budi@example.com" }
```

**Response `200 OK`** (selalu sama, **tidak membocorkan** apakah email terdaftar):

```json
{ "success": true, "code": "OTP_REQUESTED", "message": "Jika email terdaftar, kode OTP telah dikirim" }
```

#### 8b. Verifikasi + password baru

`POST /api/v1/auth/forgot-password/verify`

| Field | Tipe | Wajib | Aturan |
|---|---|---|---|
| `email` | string | ✅ | Format email |
| `code` | string | ✅ | Tepat 8 karakter |
| `new_password` | string | ✅ | 8–72 karakter |

```json
{ "success": true, "code": "AUTH_PASSWORD_CHANGED", "message": "Password berhasil direset..." }
```

> Semua sesi dicabut → arahkan user login ulang.

---

### Aturan OTP

| Aspek | Nilai |
|---|---|
| Panjang | 8 karakter |
| Masa berlaku | 5 menit |
| Cooldown kirim ulang | 60 detik |
| Maks kirim ulang | 5 kali/jam |
| Maks percobaan salah | 5 kali (lalu hangus) |
| Sekali pakai | Ya (setelah verifikasi sukses, hangus) |
| Isolasi purpose | OTP `change_password` ≠ `reset_password` |

---

## Daftar Kode Error

Branching di frontend sebaiknya memakai field **`code`**, bukan `message`.

### Sukses

| Code | HTTP | Keterangan |
|---|---|---|
| `SUCCESS` | 200 | Sukses umum |
| `CREATED` | 201 | Resource dibuat |
| `AUTH_REGISTER_SUCCESS` | 201 | Registrasi berhasil |
| `AUTH_LOGIN_SUCCESS` | 200 | Login berhasil |
| `AUTH_REFRESH_SUCCESS` | 200 | Token diperbarui |
| `AUTH_LOGOUT_SUCCESS` | 200 | Logout berhasil |
| `AUTH_GOOGLE_SUCCESS` | 200 | Login Google berhasil |
| `OTP_REQUESTED` | 200 | OTP terkirim |
| `AUTH_PASSWORD_CHANGED` | 200 | Password berubah |

### Gagal

| Code | HTTP | Keterangan |
|---|---|---|
| `VALIDATION_ERROR` | 422 | Validasi field gagal (lihat `errors[]`) |
| `AUTH_UNAUTHORIZED` | 401 | Token tidak ada/tidak valid |
| `AUTH_TOKEN_EXPIRED` | 401 | Access/refresh token kedaluwarsa |
| `AUTH_SESSION_REVOKED` | 401 | Sesi dicabut/tidak valid |
| `AUTH_INVALID_CREDENTIALS` | 401 | Email/username atau password salah |
| `AUTH_ACCOUNT_LOCKED` | 429 | Akun terkunci sementara |
| `AUTH_ACCOUNT_INACTIVE` | 403 | Akun nonaktif |
| `FORBIDDEN` | 403 | Tidak punya hak akses (role) |
| `NOT_FOUND` | 404 | Data tidak ditemukan |
| `AUTH_ACCOUNT_NOT_FOUND` | 404 | Akun (untuk Google) belum terdaftar |
| `CONFLICT` | 409 | Duplikat data |
| `INTERNAL_ERROR` | 500 | Kesalahan server |
| `RATE_LIMITED` | 429 | Terlalu banyak request per-IP |
| `OTP_INVALID` | 400 | Kode OTP salah |
| `OTP_EXPIRED` | 400 | Kode OTP kedaluwarsa |
| `OTP_TOO_MANY_ATTEMPTS` | 429 | Percobaan OTP habis |
| `OTP_RESEND_TOO_SOON` | 429 | Cooldown kirim ulang belum lewat |
| `OTP_RESEND_LIMIT` | 429 | Batas kirim per jam tercapai |
| `AUTH_GOOGLE_DISABLED` | 503 | Google Sign-In belum dikonfigurasi |
| `AUTH_GOOGLE_TOKEN_INVALID` | 401 | ID token Google tidak valid |
| `AUTH_GOOGLE_EMAIL_UNVERIFIED` | 401 | Email Google belum terverifikasi |

---

## Role Pengguna

Endpoint bisa dibatasi per role (middleware `RequireRoles`). Nilai role (dipakai di `data.user.roles`):

| Role | Keterangan |
|---|---|
| `admin` | Administrator |
| `waka_kurikulum` | Wakil Kepala Sekolah — Kurikulum |
| `waka_kesiswaan` | Wakil Kepala Sekolah — Kesiswaan |
| `guru_mapel` | Guru Mata Pelajaran |
| `wali_kelas` | Wali Kelas |
| `siswa_inti` | Siswa Inti |
| `siswa` | Siswa (role default saat registrasi) |

Role bersifat **union** — satu akun bisa punya beberapa role.

---

## Rate Limit

- Berlaku untuk seluruh `/api/v1/auth/*`, **per alamat IP**.
- Default: **5 request/detik**, burst **10**.
- Melebihi batas → `429 RATE_LIMITED` + header `Retry-After: 60`.
- Frontend disarankan menangani `429` dengan backoff, dan **tidak** retry agresif.

---

## Skenario Uji Manual

Daftar skenario untuk memverifikasi Epic 1 secara manual (PowerShell). Kolom **Hasil** adalah keluaran yang **sudah diverifikasi** pada server lokal (rate limit dinonaktifkan untuk skenario fungsional).

| # | Skenario | Aksi | Diharapkan | Hasil |
|---|---|---|---|---|
| 1 | Health check | `GET /api/status` | 200 `SUCCESS` | ✅ `SUCCESS` |
| 2 | Registrasi baru | `POST /register` (email/NISN baru) | 201 `AUTH_REGISTER_SUCCESS`, role `siswa` | ✅ 200 `AUTH_REGISTER_SUCCESS` |
| 3 | Registrasi duplikat | `POST /register` (email sama) | 409 `CONFLICT` | ✅ 409 `CONFLICT` |
| 4 | Validasi field | `POST /register` (email/NISN/password tidak valid) | 422 `VALIDATION_ERROR` + `errors[]` | ✅ 422 `VALIDATION_ERROR` |
| 5 | Login benar | `POST /login` | 200 `AUTH_LOGIN_SUCCESS` | ✅ 200 `AUTH_LOGIN_SUCCESS` |
| 6 | Refresh (rotasi) | `POST /refresh` token valid | 200 `AUTH_REFRESH_SUCCESS`, token baru | ✅ 200 `AUTH_REFRESH_SUCCESS` |
| 7 | Refresh token lama | `POST /refresh` token **lama** (sudah dipakai) | 401 `AUTH_SESSION_REVOKED` | ✅ 401 `AUTH_SESSION_REVOKED` |
| 8 | Request OTP ganti pw | `POST /change-password/request` | 200 `OTP_REQUESTED` | ✅ 200 `OTP_REQUESTED` |
| 9 | Cooldown OTP | request lagi < 60 detik | 429 `OTP_RESEND_TOO_SOON` | ✅ 429 `OTP_RESEND_TOO_SOON` |
| 10 | OTP salah | `POST /change-password/verify` kode ngawur | 400 `OTP_INVALID` | ✅ 400 `OTP_INVALID` |
| 11 | Ganti password benar | verify dengan kode dari konsol | 200 `AUTH_PASSWORD_CHANGED` | ✅ 200 `AUTH_PASSWORD_CHANGED` |
| 12 | Sesi lama dicabut | `POST /refresh` token sebelum ganti pw | 401 `AUTH_SESSION_REVOKED` | ✅ 401 `AUTH_SESSION_REVOKED` |
| 13 | Password lama mati | `POST /login` password lama | 401 `AUTH_INVALID_CREDENTIALS` | ✅ 401 `AUTH_INVALID_CREDENTIALS` |
| 14 | Password baru jalan | `POST /login` password baru | 200 `AUTH_LOGIN_SUCCESS` | ✅ 200 `AUTH_LOGIN_SUCCESS` |
| 15 | Reset (forgot) | `POST /forgot-password` → verify | 200 `OTP_REQUESTED` → `AUTH_PASSWORD_CHANGED` | ✅ 200 → 200 |
| 16 | Anti-enumerasi | `POST /forgot-password` email **tak terdaftar** | 200 `OTP_REQUESTED` (tidak membocorkan) | ✅ 200 `OTP_REQUESTED` |
| 17 | Google belum diatur | `POST /google` (`GOOGLE_CLIENT_ID` kosong) | 503 `AUTH_GOOGLE_DISABLED` | ✅ 503 `AUTH_GOOGLE_DISABLED` |
| 18 | Logout satu device | `POST /logout` | 200 `AUTH_LOGOUT_SUCCESS` | ✅ 200 `AUTH_LOGOUT_SUCCESS` |
| 19 | Logout semua | `POST /logout-all` | 200 `AUTH_LOGOUT_SUCCESS` | ✅ 200 `AUTH_LOGOUT_SUCCESS` |
| 20 | Lockout akun | 5× login gagal | gagal #1–4 = 401, **#5 = 429 `AUTH_ACCOUNT_LOCKED`** | ✅ #1–4 401, #5 429 |
| 21 | CORS origin diizinkan | `OPTIONS` dengan `Origin: http://localhost:3000` | 204 + `Access-Control-Allow-Origin` | ✅ 204 + header |
| 22 | CORS origin terlarang | `OPTIONS` dengan `Origin: http://evil.example.com` | 403 | ✅ 403 |
| 23 | Rate limit | 40 request cepat ke `/login` | sebagian `429 RATE_LIMITED` + `Retry-After: 60` | ✅ 11 lolos / 29 = 429, `Retry-After: 60` |

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
      -Body (@{ identifier="budi@example.com"; password="rahasia123" } | ConvertTo-Json)
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
- [ ] Epic 2 — Manajemen data master (kelas, jurusan, mapel, dsb.)
- [ ] Epic 3 — Absensi
- [ ] Epic 4 — Laporan & rekap
- [ ] … (total 14 Epic sesuai PRD)
