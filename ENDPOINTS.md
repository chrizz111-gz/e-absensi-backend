
# E-Absensi — Dokumentasi API (Epic 1, 2, 3, & 4)

Referensi kontrak API untuk **tim frontend (Mobile, Web, Desktop)**. Dokumen ini berisi **konvensi response, alur autentikasi, daftar endpoint, kode error, role, permission, dan rate limit**.
Referensi kontrak API untuk **tim frontend (Mobile, Web, Desktop)**. Dokumen ini berisi **konvensi response, alur autentikasi, daftar endpoint, kode error, role, permission, dan rate limit** untuk modul **Epic 1 (Auth)**, **Epic 2 (User & RBAC)**, **Epic 3 (Master Data)**, dan **Epic 4 (Jadwal)**.

> Cara menjalankan server, konfigurasi `.env`, skenario uji manual, dan CORS ada di **[README.md](README.md)**.

---

## Konvensi API

- **Base URL (lokal):** `http://localhost:8080`
- **Prefix endpoint auth:** `/api/v1/auth`
- **Prefix endpoint admin (Epic 2):** `/api/v1` (roles, permissions, users)
- **Format data:** JSON (`Content-Type: application/json`)
- **Semua response** memakai envelope yang sama:

```json
{
  "success": true,
  "code": "AUTH_LOGIN_SUCCESS",
  "message": "Login berhasil",
  "data": {}
}
```

Saat gagal:

```json
{
  "success": false,
  "code": "VALIDATION_ERROR",
  "message": "Validasi data gagal",
  "errors": [
    {
      "field": "email",
      "code": "INVALID_EMAIL",
      "message": "Email harus berupa email yang valid"
    },
    {
      "field": "password",
      "code": "PASSWORD_TOO_SHORT",
      "message": "Password minimal 8 karakter"
    }
  ]
}
```

| Field     | Tipe           | Keterangan                                                                              |
| --------- | -------------- | --------------------------------------------------------------------------------------- |
| `success` | bool           | `true` bila sukses                                                                      |
| `code`    | string         | Kode mesin (untuk branching di frontend). Lihat [Daftar Kode Error](#daftar-kode-error) |
| `message` | string         | Pesan human-readable (Bahasa Indonesia)                                                 |
| `data`    | object \| null | Payload sukses, atau data tambahan pada error OTP (mis. `attempts_remaining`)           |
| `errors`  | object[]       | Hanya ada saat `VALIDATION_ERROR`/`CONFLICT`. Bentuk: `{ field, code, message }`        |

**HTTP status yang dipakai:**

| Status                      | Arti                                                                                              |
| --------------------------- | ------------------------------------------------------------------------------------------------- |
| `200 OK`                    | Sukses                                                                                            |
| `201 Created`               | Resource berhasil dibuat (registrasi/POST user)                                                   |
| `400 Bad Request`           | Permintaan tidak valid (mis. OTP salah/kedaluwarsa)                                               |
| `401 Unauthorized`          | Kredensial/token salah, kedaluwarsa, atau akun nonaktif pada request terautentikasi               |
| `403 Forbidden`             | Tidak punya hak akses (role/permission)                                                           |
| `404 Not Found`             | Data/akun tak ditemukan                                                                           |
| `409 Conflict`              | Duplikat data, data sudah berubah (stale), proteksi Admin terakhir, atau akun masih punya riwayat |
| `422 Unprocessable Entity`  | Validasi field gagal                                                                              |
| `429 Too Many Requests`     | Terkena lockout / rate limit / batas OTP                                                          |
| `500 Internal Server Error` | Kesalahan server                                                                                  |
| `503 Service Unavailable`   | Fitur belum dikonfigurasi (mis. Google)                                                           |

---

## Epic 1 — Authentication & Account

### Autentikasi (JWT & Sesi)

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
6. **OTP 6 karakter alfanumerik uppercase** (contoh `A7K2Q9`, charset `A-Z` + `2-9`), maks **5 percobaan salah**. Setelah percobaan ke-5, kode hangus walau kode berikutnya benar.
7. **Verifikasi OTP terpisah dari simpan password.** `verify-otp` mengembalikan `reset_token` berumur pendek dan sekali pakai; password baru dikirim ke endpoint `confirm`/`reset`.

---

### Referensi Endpoint Epic 1

Ringkasan (🔒 = butuh header `Authorization: Bearer`):

| Method | Path                                         | Auth   | Deskripsi                                 |
| ------ | -------------------------------------------- | ------ | ----------------------------------------- |
| GET    | `/api/status`                                | —      | Health check                              |
| POST   | `/api/v1/auth/register`                      | —      | Registrasi siswa                          |
| POST   | `/api/v1/auth/login`                         | —      | Login email/username                      |
| POST   | `/api/v1/auth/google`                        | —      | Login via Google                          |
| POST   | `/api/v1/auth/refresh`                       | —      | Tukar refresh token                       |
| POST   | `/api/v1/auth/forgot-password`               | —      | Minta OTP reset password                  |
| POST   | `/api/v1/auth/forgot-password/resend`        | —      | Kirim ulang OTP reset                     |
| POST   | `/api/v1/auth/forgot-password/verify-otp`    | —      | Cek OTP reset → `reset_token`             |
| POST   | `/api/v1/auth/forgot-password/reset`         | —      | Simpan password baru dengan `reset_token` |
| POST   | `/api/v1/auth/logout` 🔒                     | Bearer | Logout device ini                         |
| POST   | `/api/v1/auth/logout-all` 🔒                 | Bearer | Logout semua device                       |
| POST   | `/api/v1/auth/change-password/request` 🔒    | Bearer | Minta OTP ganti password                  |
| POST   | `/api/v1/auth/change-password/resend` 🔒     | Bearer | Kirim ulang OTP ganti password            |
| POST   | `/api/v1/auth/change-password/verify-otp` 🔒 | Bearer | Cek OTP ganti password → `reset_token`    |
| POST   | `/api/v1/auth/change-password/confirm` 🔒    | Bearer | Simpan password baru dengan `reset_token` |

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

| Field              | Tipe   | Wajib | Aturan                                                                                          |
| ------------------ | ------ | ----- | ----------------------------------------------------------------------------------------------- |
| `name`             | string | ✅    | 2–150 karakter                                                                                  |
| `email`            | string | ✅    | Format email, unik, maks 255                                                                    |
| `nisn`             | string | ✅    | **Tepat 10 digit angka**, unik                                                                  |
| `password`         | string | ✅    | 8–72 karakter, huruf besar/kecil, angka, simbol, tanpa spasi                                    |
| `confirm_password` | string | ✅    | Harus sama dengan `password`                                                                    |
| `username`         | string | ❌    | 3–30 karakter (huruf kecil, angka, titik, underscore), unik; huruf besar otomatis dinormalisasi |

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Budi Santoso",
    "email": "budi@example.com",
    "nisn": "1234567890",
    "password": "Rahasia123!",
    "confirm_password": "Rahasia123!",
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

Error umum: `409 CONFLICT` dengan `errors[]` berisi `EMAIL_TAKEN`/`NISN_TAKEN`/`USERNAME_TAKEN`,
atau `422 VALIDATION_ERROR` dengan detail field.

---

### 2. Login

`POST /api/v1/auth/login`

**Request body**

| Field         | Tipe   | Wajib | Keterangan              |
| ------------- | ------ | ----- | ----------------------- |
| `identifier`  | string | ✅    | Email **atau** username |
| `password`    | string | ✅    |                         |
| `device_info` | string | ❌    | Info device, maks 255   |

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{ "identifier": "budi@example.com", "password": "Rahasia123!", "device_info": "Android 14" }'
```

**Response `200 OK`** → `AuthResponse` (`code: "AUTH_LOGIN_SUCCESS"`).

Error: `401 AUTH_INVALID_CREDENTIALS`, `429 AUTH_ACCOUNT_LOCKED`, `403 AUTH_ACCOUNT_INACTIVE`.

---

### 3. Login Google

### 3. Login & Registrasi Siswa via Google

`POST /api/v1/auth/google`

**Request body**

| Field         | Tipe   | Wajib | Keterangan                                                                          |
| ------------- | ------ | ----- | ----------------------------------------------------------------------------------- |
| `id_token`    | string | ✅    | ID token dari Google Sign-In SDK                                                    |
| `nisn`        | string | ❌    | **Wajib 10 digit angka** saat registrasi siswa baru via Google                      |
| `name`        | string | ❌    | Opsional override nama saat registrasi siswa via Google (default: nama dari Google) |
| `device_info` | string | ❌    | Maks 255                                                                            |

Perilaku:

- Akun dengan `google_id` cocok → langsung login.
- Belum tertaut, tapi **email sama** dengan akun terdaftar → akun Google dikaitkan, lalu login.
- Email belum terdaftar → `404 AUTH_ACCOUNT_NOT_FOUND` (diminta registrasi dulu; **tidak** auto-create).
- Belum tertaut, tapi **email sama** dengan akun terdaftar (mis. akun staf yang baru dibuat Admin) → akun Google dikaitkan (`google_id`), lalu login.
- Email belum terdaftar & **`nisn` belum diisi** → `404 AUTH_ACCOUNT_NOT_FOUND` beserta `data: { "email": "...", "name": "...", "avatar_url": "..." }` agar frontend dapat meng-auto-fill form registrasi siswa dan meminta input `nisn`.
- Email belum terdaftar & **`nisn` diisi (10 digit)** → otomatis membuat akun siswa baru (`users` + role `siswa` + profil `students`), lalu langsung login.
- Akun yang sedang terkunci (`locked_until`) tetap ditolak `429 AUTH_ACCOUNT_LOCKED`.
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

Alur 3 langkah: **minta OTP** → **cek OTP** → **simpan password baru**.

#### 7a. Minta / kirim ulang OTP

`POST /api/v1/auth/change-password/request` (atau `/resend`)

Tanpa body (identitas dari access token).

```json
{
  "success": true,
  "code": "OTP_REQUESTED",
  "message": "Kode OTP telah dikirim ke email Anda",
  "data": {
    "attempts_remaining": 5,
    "expires_at": "2026-11-04T10:05:00+07:00",
    "resend_after_seconds": 60
  }
}
```

- Cooldown kirim ulang **60 detik** → `429 OTP_RESEND_TOO_SOON` + `data.retry_after_seconds` + header `Retry-After`.
- Maks **5 kali per jam** → `429 OTP_RESEND_LIMIT`.

#### 7b. Cek OTP

`POST /api/v1/auth/change-password/verify-otp`

| Field  | Tipe   | Wajib | Aturan                                                       |
| ------ | ------ | ----- | ------------------------------------------------------------ |
| `code` | string | ✅    | Tepat 6 karakter `A-Z`/`2-9` (input lowercase dinormalisasi) |

```json
{
  "success": true,
  "code": "OTP_VERIFIED",
  "message": "Kode OTP valid. Silakan buat kata sandi baru.",
  "data": {
    "reset_token": "eyJhbGciOi...",
    "expires_at": "2026-11-04T10:10:00+07:00",
    "attempts_remaining": 5
  }
}
```

Jika salah, response error menyertakan sisa percobaan:

```json
{
  "success": false,
  "code": "OTP_INVALID",
  "message": "Kode OTP salah",
  "data": {
    "attempts_remaining": 4,
    "expires_at": "2026-11-04T10:05:00+07:00"
  }
}
```

- Salah 5× → `429 OTP_TOO_MANY_ATTEMPTS`, `attempts_remaining: 0`.
- Kode hangus setelah batas tercapai; kode benar berikutnya tetap ditolak.

#### 7c. Simpan password baru

`POST /api/v1/auth/change-password/confirm`

| Field              | Tipe   | Wajib | Aturan                                          |
| ------------------ | ------ | ----- | ----------------------------------------------- |
| `reset_token`      | string | ✅    | Dari `verify-otp`                               |
| `new_password`     | string | ✅    | 8–72 karakter, huruf besar/kecil, angka, simbol |
| `confirm_password` | string | ✅    | Harus sama dengan `new_password`                |

```json
{
  "success": true,
  "code": "AUTH_PASSWORD_CHANGED",
  "message": "Password berhasil diubah..."
}
```

> Semua sesi dicabut → frontend harus mengarahkan user login ulang.

Error: `401 AUTH_RESET_TOKEN_INVALID`, `401 AUTH_RESET_TOKEN_EXPIRED`, `400 AUTH_PASSWORD_SAME`,
`422 VALIDATION_ERROR` (`PASSWORD_*`, `PASSWORD_MISMATCH`).

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
{
  "success": true,
  "code": "OTP_REQUESTED",
  "message": "Jika email terdaftar, kode OTP telah dikirim",
  "data": {
    "attempts_remaining": 5,
    "expires_at": "2026-11-04T10:05:00+07:00",
    "resend_after_seconds": 60
  }
}
```

#### 8b. Cek OTP

`POST /api/v1/auth/forgot-password/verify-otp`

| Field   | Tipe   | Wajib | Aturan                                                       |
| ------- | ------ | ----- | ------------------------------------------------------------ |
| `email` | string | ✅    | Format email                                                 |
| `code`  | string | ✅    | Tepat 6 karakter `A-Z`/`2-9` (input lowercase dinormalisasi) |

```json
{
  "success": true,
  "code": "OTP_VERIFIED",
  "message": "Kode OTP valid. Silakan buat kata sandi baru.",
  "data": {
    "reset_token": "eyJhbGciOi...",
    "expires_at": "2026-11-04T10:10:00+07:00",
    "attempts_remaining": 5
  }
}
```

#### 8c. Simpan password baru

`POST /api/v1/auth/forgot-password/reset`

| Field              | Tipe   | Wajib | Aturan                                          |
| ------------------ | ------ | ----- | ----------------------------------------------- |
| `reset_token`      | string | ✅    | Dari `verify-otp`                               |
| `new_password`     | string | ✅    | 8–72 karakter, huruf besar/kecil, angka, simbol |
| `confirm_password` | string | ✅    | Harus sama dengan `new_password`                |

```json
{
  "success": true,
  "code": "AUTH_PASSWORD_CHANGED",
  "message": "Password berhasil direset..."
}
```

> Semua sesi dicabut → arahkan user login ulang.

---

### Aturan OTP

| Aspek                | Nilai                                                             |
| -------------------- | ----------------------------------------------------------------- |
| Panjang              | 6 karakter alfanumerik uppercase (contoh `A7K2Q9`)                |
| Charset              | `A-Z` + `2-9`; tanpa `0/O/1/I/L` agar tidak ambigu                |
| Masa berlaku         | 5 menit                                                           |
| Cooldown kirim ulang | 60 detik                                                          |
| Maks kirim ulang     | 5 kali/jam                                                        |
| Maks percobaan salah | 5 kali (lalu hangus)                                              |
| Sekali pakai         | Ya (dikonsumsi setelah password benar-benar disimpan)             |
| Reset token          | Sekali pakai, umur maks 10 menit dan tidak melebihi sisa umur OTP |
| Isolasi purpose      | OTP `change_password` ≠ `reset_password`                          |

---

## Epic 2 — User & Role Management

Seluruh endpoint Epic 2 wajib memakai **Bearer access token**. Endpoint user
(`/api/v1/users`) hanya dapat diakses role `admin` dan tetap dilindungi
permission granular dari database. Otorisasi tidak memakai klaim role di token.
Bagian ini mencakup Fase 2 (RBAC read-only) dan Fase 3 (CRUD user & role).

Permission yang dipakai:

| Permission           | Dipakai oleh                          |
| -------------------- | ------------------------------------- |
| `account.view`       | `GET /users`, `GET /users/{id}`       |
| `account.create`     | `POST /users`                         |
| `account.set_status` | `PATCH /users/{id}/status`            |
| `role.assign`        | `PUT /users/{id}/roles`, `GET /roles` |
| `account.update`     | `PATCH /users/{id}`                   |
| `account.delete`     | `DELETE /users/{id}`                  |
| `permission.view`    | `GET /permissions/matrix`             |

### Referensi Endpoint Epic 2

| Method | Path                         | Auth           | Permission           | Deskripsi                       |
| ------ | ---------------------------- | -------------- | -------------------- | ------------------------------- |
| GET    | `/api/v1/roles`              | Bearer         | `role.assign`        | Daftar role master              |
| GET    | `/api/v1/permissions/matrix` | Bearer         | `permission.view`    | Matriks hak akses read-only     |
| GET    | `/api/v1/users`              | Bearer + admin | `account.view`       | Daftar user (pagination/filter) |
| GET    | `/api/v1/users/{id}`         | Bearer + admin | `account.view`       | Detail user + role              |
| POST   | `/api/v1/users`              | Bearer + admin | `account.create`     | Buat akun staf                  |
| PATCH  | `/api/v1/users/{id}/status`  | Bearer + admin | `account.set_status` | Aktifkan/nonaktifkan akun       |
| PUT    | `/api/v1/users/{id}/roles`   | Bearer + admin | `role.assign`        | Ganti role assignable           |
| PATCH  | `/api/v1/users/{id}`         | Bearer + admin | `account.update`     | Ubah profil                     |
| DELETE | `/api/v1/users/{id}`         | Bearer + admin | `account.delete`     | Hapus akun staf                 |

### Objek User Admin

**UserAdminResponse** — dipakai oleh seluruh endpoint `/api/v1/users`. Response
ini **tidak pernah** memuat `password_hash` atau `google_id`.

```json
{
  "id": 21,
  "name": "Guru Mapel",
  "email": "guru@example.com",
  "is_active": true,
  "created_at": "2026-11-06T10:00:00+07:00",
  "updated_at": "2026-11-06T10:05:00+07:00",
  "roles": [
    {
      "id": 4,
      "name": "guru_mapel",
      "label": "Guru Mata Pelajaran",
      "category": "staff",
      "assignable": true
    }
  ]
}
```

Saat akun nonaktif, response juga memuat `inactive_reason` dan
`deactivated_at`. `nisn` dan `avatar_url` hanya muncul bila terisi. Role pada
response dapat berisi role terkunci (`wali_kelas`, `siswa_inti`, `siswa`) dengan
`assignable: false`.

### 9. Daftar Role (read-only)

`GET /api/v1/roles`

Permission: `role.assign`

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "Daftar role berhasil diambil",
  "data": {
    "roles": [
      {
        "id": 1,
        "name": "admin",
        "label": "Admin / Staf Kurikulum",
        "category": "staff",
        "assignable": true
      },
      {
        "id": 5,
        "name": "wali_kelas",
        "label": "Wali Kelas",
        "category": "staff",
        "assignable": false
      }
    ]
  }
}
```

### 10. Matriks Hak Akses

`GET /api/v1/permissions/matrix`

Permission: `permission.view`

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "Matriks hak akses berhasil diambil",
  "data": {
    "roles": [
      {
        "id": 1,
        "name": "admin",
        "label": "Admin / Staf Kurikulum",
        "category": "staff",
        "assignable": true
      }
    ],
    "permissions": [
      {
        "id": 1,
        "code": "account.view",
        "module": "account",
        "description": "Melihat daftar dan detail akun"
      }
    ],
    "grants": [
      {
        "role_id": 1,
        "permission_id": 1
      }
    ]
  }
}
```

`grants` adalah sel matriks: frontend membentuk tabel dari `roles` ×
`permissions`, lalu menandai sel tercentang bila pasangan `role_id` dan
`permission_id` ada di `grants`.

Error yang mungkin: `401 UNAUTHORIZED`, `401 AUTH_TOKEN_EXPIRED`,
`403 FORBIDDEN`.

---

### 11. Daftar User

`GET /api/v1/users`

Permission: `account.view` — hanya role `admin`.

**Query parameter**

| Parameter          | Wajib | Default | Aturan                                                                     |
| ------------------ | ----- | ------- | -------------------------------------------------------------------------- |
| `page`             | ❌    | `1`     | Bilangan bulat ≥ 1                                                         |
| `limit`            | ❌    | `20`    | Bilangan bulat 1–100                                                       |
| `q`                | ❌    | —       | Pencarian partial pada `name`, `email`, dan `nisn`                         |
| `role[]`           | ❌    | —       | Nama role; boleh diulang. User cocok bila memiliki **salah satu** role     |
| `status`           | ❌    | —       | `active` atau `inactive`                                                   |
| `include_archived` | ❌    | `false` | `true` untuk menampilkan akun yang sudah diarsipkan (`archived_at` terisi) |

- Urutan default: `name ASC`, lalu `id ASC`.
- Karakter `%` dan `_` pada `q` diperlakukan sebagai **teks literal**, bukan wildcard.
- Akun dengan `archived_at` terisi **disembunyikan secara default** dan hanya muncul bila `include_archived=true`.
- `total_pages` bernilai `0` bila tidak ada data.

```json
{
  "success": true,
  "code": "USER_LIST",
  "message": "Daftar akun berhasil diambil",
  "data": {
    "items": [
      {
        "id": 21,
        "name": "Guru Mapel",
        "email": "guru@example.com",
        "is_active": true,
        "created_at": "2026-11-06T10:00:00+07:00",
        "updated_at": "2026-11-06T10:05:00+07:00",
        "roles": [
          {
            "id": 4,
            "name": "guru_mapel",
            "label": "Guru Mata Pelajaran",
            "category": "staff",
            "assignable": true
          }
        ]
      }
    ],
    "meta": {
      "page": 1,
      "limit": 20,
      "total": 1,
      "total_pages": 1
    }
  }
}
```

Error: `401 UNAUTHORIZED`, `403 FORBIDDEN`, `422 VALIDATION_ERROR`
(`INVALID_PAGE`, `INVALID_LIMIT`, `INVALID_STATUS`, `ROLE_NOT_FOUND`,
`INVALID_BOOLEAN`).

---

### 12. Detail User

`GET /api/v1/users/{id}`

Permission: `account.view` — hanya role `admin`.

**Response `200 OK`** → `data` berisi satu `UserAdminResponse`.

Error: `404 ACCOUNT_NOT_FOUND` bila user tidak ada, `422 VALIDATION_ERROR`
(`INVALID_ID`) bila `id` bukan bilangan positif.

---

### 13. Buat User

`POST /api/v1/users`

Permission: `account.create` — hanya role `admin`.

Akun staf dibuat **tanpa password dan tanpa Google**. Hanya `name`, `email`,
dan `roles` yang diterima. Nilai default: `password_hash = NULL`,
`is_active = true`. Insert `users`, `user_roles`, dan event `ACCOUNT_CREATED`
ke `activity_logs` dilakukan dalam **satu transaksi**.

**Request body**

| Field   | Tipe     | Wajib | Aturan                                              |
| ------- | -------- | ----- | --------------------------------------------------- |
| `name`  | string   | ✅    | 2–150 karakter; di-trim                             |
| `email` | string   | ✅    | Format email, unik, maks 255; di-trim dan lowercase |
| `roles` | string[] | ✅    | Minimal 1 **nama role**, hanya `assignable: true`   |

```json
{
  "name": "Guru Mapel",
  "email": "guru@example.com",
  "roles": ["guru_mapel"]
}
```

**Response `201 Created`**

```json
{
  "success": true,
  "code": "CREATED",
  "message": "Akun berhasil dibuat",
  "data": { "...UserAdminResponse..." }
}
```

Error:

- `409 EMAIL_ALREADY_EXISTS` + `errors[].field = email`
- `422 VALIDATION_ERROR` + `INVALID_EMAIL` / `FIELD_REQUIRED` /
  `ROLE_NOT_FOUND`
- `422 ROLE_NOT_ASSIGNABLE` untuk role terkunci
- `422 INVALID_ROLE_COMBINATION` bila `roles` kosong

> `roles` berisi **nama role** (mis. `guru_mapel`), bukan `id`.

---

### 14. Ubah Status User

`PATCH /api/v1/users/{id}/status`

Permission: `account.set_status` — hanya role `admin`.

**Request body**

| Field             | Tipe   | Wajib                        | Aturan                                   |
| ----------------- | ------ | ---------------------------- | ---------------------------------------- |
| `is_active`       | bool   | ✅                           | `false` = nonaktif, `true` = aktif       |
| `inactive_reason` | string | Wajib saat `is_active=false` | `graduated`, `transferred`, atau `other` |

```json
{ "is_active": false, "inactive_reason": "transferred" }
```

Perilaku:

- User **tidak boleh menonaktifkan akunnya sendiri** → `403 SELF_ACTION_FORBIDDEN`.
- Deaktivasi Admin terakhir yang masih aktif → `409 LAST_ADMIN_PROTECTED`.
- Deaktivasi **mencabut seluruh sesi** user tersebut.
- Aktivasi menghapus `inactive_reason`, `deactivated_at`, dan `archived_at`,
  serta mereset `failed_login_attempts` (dan state lockout).
- `inactive_reason` diabaikan saat `is_active=true`.
- Status yang sama dengan kondisi akun saat ini ditolak:
  `409 ACCOUNT_ALREADY_ACTIVE` atau `409 ACCOUNT_ALREADY_INACTIVE`.
- `graduated` hanya valid untuk akun siswa. `no_class` adalah alasan sistem
  Epic 10 dan **tidak diterima** lewat endpoint manual ini.
- Setiap perubahan status menulis event `ACCOUNT_ACTIVATED` atau
  `ACCOUNT_DEACTIVATED` ke `activity_logs` dalam transaksi yang sama.

**Response `200 OK`** → `data` berisi `UserAdminResponse` terbaru.

Error: `403 SELF_ACTION_FORBIDDEN`, `404 ACCOUNT_NOT_FOUND`,
`409 LAST_ADMIN_PROTECTED`, `409 ACCOUNT_ALREADY_ACTIVE`,
`409 ACCOUNT_ALREADY_INACTIVE`, `422 VALIDATION_ERROR`
(`FIELD_REQUIRED`, `INVALID_INACTIVE_REASON`).

---

### 15. Ganti Role User

`PUT /api/v1/users/{id}/roles`

Permission: `role.assign` — hanya role `admin`.

**Request body**

```json
{ "roles": ["waka_kurikulum", "guru_mapel"] }
```

| Field   | Tipe     | Wajib | Aturan                                                                                    |
| ------- | -------- | ----- | ----------------------------------------------------------------------------------------- |
| `roles` | string[] | ✅    | Boleh array kosong bila role terkunci masih tersisa. Hanya role dengan `assignable: true` |

Perilaku:

- Mengganti **seluruh role assignable** user. Role terkunci
  (`wali_kelas`, `siswa_inti`, `siswa`) **selalu dipertahankan**.
- Admin yang mencoba mencabut role `admin` dari dirinya sendiri →
  `403 SELF_ACTION_FORBIDDEN`.
- Mencabut Admin aktif terakhir → `409 LAST_ADMIN_PROTECTED`.
- `roles: []` mencabut semua role assignable, tetapi akun wajib tetap memiliki
  minimal satu role (termasuk role terkunci) → jika tidak,
  `422 INVALID_ROLE_COMBINATION`.
- Role staf dan siswa tidak boleh dicampur pada satu akun, dan `siswa_inti`
  wajib bersama `siswa` → `422 INVALID_ROLE_COMBINATION`.
- Setiap perubahan role menulis event `ROLE_ASSIGNED`/`ROLE_REVOKED` ke
  `activity_logs` dalam transaksi yang sama.

**Response `200 OK`** → `data` berisi `UserAdminResponse` terbaru.

Error: `403 SELF_ACTION_FORBIDDEN`, `404 ACCOUNT_NOT_FOUND`,
`409 LAST_ADMIN_PROTECTED`, `422 ROLE_NOT_ASSIGNABLE`,
`422 INVALID_ROLE_COMBINATION`, `422 VALIDATION_ERROR` (`ROLE_NOT_FOUND`).

---

### 16. Ubah Profil User

`PATCH /api/v1/users/{id}`

Permission: `account.update` — hanya role `admin`.

**Request body**

| Field        | Tipe             | Wajib | Aturan                                                                      |
| ------------ | ---------------- | ----- | --------------------------------------------------------------------------- |
| `name`       | string           | ❌    | 2–150 karakter; di-trim                                                     |
| `email`      | string           | ❌    | Format email, unik, maks 255; di-trim dan lowercase                         |
| `updated_at` | string (RFC3339) | ❌    | Nilai `updated_at` terakhir dari GET detail; untuk optimistic locking       |
| `updated_at` | string (RFC3339) | ✅    | Nilai `updated_at` terakhir dari GET detail; wajib untuk optimistic locking |

```json
{
  "name": "Guru Mapel Updated",
  "email": "guru.baru@example.com",
  "updated_at": "2026-11-06T10:05:00.123456+07:00"
}
```

Perilaku:

- **Optimistic locking opsional:** bila `updated_at` dikirim dan sudah tidak
  sama dengan database → `409 STALE_DATA`. Frontend harus GET ulang lalu
  mengirim ulang.
- Bila `updated_at` tidak dikirim, perubahan tetap diproses tanpa pengecekan
  versi.
- Simpan tanpa perubahan (nilai sama) tetap `200` dan **tidak** menulis event
  baru.
- **Optimistic locking wajib:** `updated_at` harus dikirim sesuai nilai terakhir dari database. Jika sudah berubah → `409 STALE_DATA`. Frontend harus GET ulang lalu mengirim ulang.
- Simpan tanpa perubahan (nilai sama) tetap `200` dan **tidak** menulis event baru.
- Bila email berubah, `google_id` lama otomatis dikosongkan.
- Akun dengan role siswa (`category: student`) tidak dapat diubah lewat endpoint
  ini → `422 ACCOUNT_TYPE_NOT_EDITABLE`.
- Perubahan yang benar-benar mengubah data menulis event `ACCOUNT_UPDATED` ke
  `activity_logs` dalam transaksi yang sama.

**Response `200 OK`** → `data` berisi `UserAdminResponse` terbaru.

Error: `404 ACCOUNT_NOT_FOUND`, `409 EMAIL_ALREADY_EXISTS`,
`409 STALE_DATA`, `422 ACCOUNT_TYPE_NOT_EDITABLE`,
`422 VALIDATION_ERROR` (`INVALID_EMAIL`, `FIELD_REQUIRED`, `TOO_SHORT`,
`TOO_LONG`).

---

### 17. Hapus User

`DELETE /api/v1/users/{id}`

Permission: `account.delete` — hanya role `admin`.

Perilaku:

- Hanya akun **staf** yang boleh dihapus. Akun siswa →
  `422 ACCOUNT_TYPE_NOT_EDITABLE`.
- Tidak boleh menghapus akun sendiri → `403 SELF_ACTION_FORBIDDEN`.
- Admin terakhir tidak boleh dihapus → `409 LAST_ADMIN_PROTECTED`.
- Bila user masih direferensikan riwayat data (`ON DELETE RESTRICT`) →
  `409 ACCOUNT_HAS_HISTORY`.
- User yang tidak ada / sudah terhapus → `404 ACCOUNT_NOT_FOUND`.
- Snapshot akun ditulis sebagai event `ACCOUNT_DELETED` ke `activity_logs`
  **sebelum** user dihapus, dalam transaksi yang sama.
- Relasi `user_roles`, `sessions`, dan OTP ikut terhapus lewat cascade.

**Response `200 OK`**

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "Akun berhasil dihapus",
  "data": null
}
```

---

## Epic 3 — Master Data Management

Seluruh endpoint Epic 3 wajib memakai **Bearer access token** dan dilindungi middleware `RequirePermission` berbasis database (`user_roles ⋈ role_permissions ⋈ permissions`). Semua endpoint **`PATCH`** di Epic 3 wajib menyertakan field **`updated_at`** terakhir dari `GET` untuk _Optimistic Locking_ (`409 STALE_DATA` bila data sudah berubah).

### Matriks Permission & Endpoint Epic 3

| Method | Path                          | Permission             | Role yang Diizinkan                                       | Deskripsi                                               |
| ------ | ----------------------------- | ---------------------- | --------------------------------------------------------- | ------------------------------------------------------- |
| GET    | `/api/v1/academic-years`      | `academic_year.view`   | `admin`, `waka_kurikulum`                                 | Daftar tahun ajaran                                     |
| GET    | `/api/v1/academic-years/{id}` | `academic_year.view`   | `admin`, `waka_kurikulum`                                 | Detail tahun ajaran                                     |
| POST   | `/api/v1/academic-years`      | `academic_year.create` | **`admin`** (Waka `403`)                                  | Tambah tahun ajaran                                     |
| PATCH  | `/api/v1/academic-years/{id}` | `academic_year.update` | `admin`, `waka_kurikulum`                                 | Ubah / aktifkan tahun ajaran                            |
| DELETE | `/api/v1/academic-years/{id}` | `academic_year.delete` | **`admin`** (Waka `403`)                                  | Hapus tahun ajaran tanpa kelas                          |
| GET    | `/api/v1/subjects`            | `subject.view`         | `admin`, `waka_kurikulum`, `guru_mapel`                   | Daftar mata pelajaran                                   |
| GET    | `/api/v1/subjects/{id}`       | `subject.view`         | `admin`, `waka_kurikulum`, `guru_mapel`                   | Detail mata pelajaran                                   |
| POST   | `/api/v1/subjects`            | `subject.create`       | `admin`, `waka_kurikulum`                                 | Tambah mata pelajaran                                   |
| PATCH  | `/api/v1/subjects/{id}`       | `subject.update`       | `admin`, `waka_kurikulum`                                 | Ubah / nonaktifkan mapel                                |
| DELETE | `/api/v1/subjects/{id}`       | `subject.delete`       | `admin`, `waka_kurikulum`                                 | Hapus mapel yang belum dipakai                          |
| GET    | `/api/v1/classes`             | `class.view`           | Seluruh 7 role                                            | Daftar kelas                                            |
| GET    | `/api/v1/classes/{id}`        | `class.view`           | Seluruh 7 role                                            | Detail kelas (+ anggota siswa terproteksi)              |
| POST   | `/api/v1/classes`             | `class.create`         | `admin`, `waka_kurikulum`                                 | Tambah kelas                                            |
| PATCH  | `/api/v1/classes/{id}`        | `class.update`         | `admin`, `waka_kurikulum`                                 | Ubah / nonaktifkan kelas                                |
| DELETE | `/api/v1/classes/{id}`        | `class.delete`         | `admin`, `waka_kurikulum`                                 | Hapus kelas yang belum dipakai                          |
| GET    | `/api/v1/teachers/candidates` | `teacher.view`         | `admin`, `waka_kurikulum`                                 | Dropdown akun `guru_mapel` tanpa profil guru            |
| GET    | `/api/v1/teachers`            | `teacher.view`         | `admin`, `waka_kurikulum`                                 | Daftar profil guru                                      |
| GET    | `/api/v1/teachers/{id}`       | `teacher.view`         | `admin`, `waka_kurikulum`                                 | Detail profil guru                                      |
| POST   | `/api/v1/teachers`            | `teacher.create`       | `admin`, `waka_kurikulum`                                 | Tambah profil guru                                      |
| PATCH  | `/api/v1/teachers/{id}`       | `teacher.update`       | `admin`, `waka_kurikulum`                                 | Ubah NIP & mapel guru                                   |
| GET    | `/api/v1/students`            | `student.view`         | `admin`, `waka_kurikulum`, `waka_kesiswaan`, `wali_kelas` | Daftar siswa (Wali Kelas otomatis di-scope ke kelasnya) |
| GET    | `/api/v1/students/{id}`       | `student.view`         | `admin`, `waka_kurikulum`, `waka_kesiswaan`, `wali_kelas` | Detail siswa (Anti-IDOR untuk Wali Kelas)               |
| PATCH  | `/api/v1/students/{id}`       | `student.update`       | `admin`, `waka_kurikulum`                                 | Ubah nama, NISN, atau kelas siswa                       |

---

### 18. Tahun Ajaran (`F3.5`)

#### `GET /api/v1/academic-years` & `GET /api/v1/academic-years/{id}`

Query parameter `GET /academic-years`: `page` (default `1`), `limit` (default `20`, maks `100`), `q` (cari nama), `is_active` (`true`/`false`), `active_semester` (`semester_1`/`semester_2`).

#### `POST /api/v1/academic-years` (Khusus Admin)

```json
{
  "name": "2026/2027",
  "active_semester": "semester_1",
  "start_date": "2026-07-13",
  "end_date": "2027-06-25",
  "is_active": true
}
```

- Jika `is_active: true`, tahun ajaran lain yang sedang aktif otomatis dinonaktifkan dalam 1 transaksi DB (maksimal 1 tahun ajaran aktif).
- Error: `409 ACADEMIC_YEAR_ALREADY_EXISTS`, `422 VALIDATION_ERROR` (`INVALID_SEMESTER`, `INVALID_DATE`, `INVALID_DATE_RANGE`).

#### `PATCH /api/v1/academic-years/{id}` (Admin & Waka Kurikulum)

```json
{
  "active_semester": "semester_2",
  "is_active": true,
  "updated_at": "2026-11-17T10:00:00.123456+07:00"
}
```

- Error: `404 ACADEMIC_YEAR_NOT_FOUND`, `409 STALE_DATA`, `409 ACADEMIC_YEAR_ALREADY_EXISTS`.

#### `DELETE /api/v1/academic-years/{id}` (Khusus Admin)

- Jika sudah memiliki data kelas (`ON DELETE RESTRICT`) → `409 ACADEMIC_YEAR_IN_USE`.

---

### 19. Mata Pelajaran (`F3.4`)

#### `GET /api/v1/subjects` & `GET /api/v1/subjects/{id}`

Query parameter: `page`, `limit`, `q` (cari `code` atau `name`), `is_active` (`true`/`false`) atau `status` (`active`/`inactive`).

#### `POST /api/v1/subjects` & `PATCH /api/v1/subjects/{id}`

```json
{
  "code": "MTK-10",
  "name": "Matematika Kelas X",
  "is_active": true
}
```

Pada `PATCH`, sertakan `"updated_at": "..."`.

- Error: `404 SUBJECT_NOT_FOUND`, `409 SUBJECT_ALREADY_EXISTS`, `409 STALE_DATA`.

#### `DELETE /api/v1/subjects/{id}`

- Jika mapel sudah diampu guru atau dipakai di jadwal → `409 SUBJECT_IN_USE` (_"Mata pelajaran tidak dapat dihapus karena sedang digunakan. Silakan nonaktifkan (is_active = false) mata pelajaran ini"_).

---

### 20. Kelas (`F3.3`)

#### `GET /api/v1/classes` & `GET /api/v1/classes/{id}`

Query parameter `GET /classes`: `page`, `limit`, `q` (nama kelas), `academic_year_id`, `grade_level` (`10`, `11`, `12`), `is_active` (`true`/`false`).

- Setiap item memuat ringkasan `academic_year`, `wali_kelas` (bila ada), dan `student_count`.
- **Proteksi Privasi pada `GET /classes/{id}`:** Daftar anggota siswa (`students[]`) ditampilkan untuk role staf (`admin`, `waka_kurikulum`, `waka_kesiswaan`, `guru_mapel`, `wali_kelas`). Jika diakses oleh `siswa` atau `siswa_inti`, `students[]` **hanya** ditampilkan apabila kelas tersebut adalah kelas milik siswa yang bersangkutan; untuk kelas lain, field `students` disembunyikan.

#### `POST /api/v1/classes` & `PATCH /api/v1/classes/{id}`

```json
{
  "name": "X RPL 1",
  "grade_level": 10,
  "academic_year_id": 1,
  "is_active": true
}
```

Pada `PATCH`, sertakan `"updated_at": "..."`.

- Nama kelas unik per `academic_year_id` (`409 CLASS_ALREADY_EXISTS`).

#### `DELETE /api/v1/classes/{id}`

- Jika kelas masih memiliki siswa atau jadwal → `409 CLASS_IN_USE` (_"Kelas tidak dapat dihapus karena masih memiliki siswa atau jadwal terkait. Silakan nonaktifkan (is_active = false) kelas ini"_).

---

### 21. Data Guru (`F3.2`)

> **Catatan PRD:** Tidak ada endpoint `DELETE /teachers/{id}`.

#### `GET /api/v1/teachers/candidates` (atau `/api/v1/teachers/available-users`)

Mengembalikan daftar akun aktif ber-role `guru_mapel` yang **belum memiliki** profil guru (untuk dropdown saat tambah data guru). Mendukung query pencarian `?q=`.

#### `GET /api/v1/teachers` & `GET /api/v1/teachers/{id}`

Query parameter: `page`, `limit`, `q` (cari nama, email, atau NIP), `subject_id`, `is_active` (`true`/`false`).

#### `POST /api/v1/teachers`

```json
{
  "user_id": 21,
  "nip": "198705122015041001",
  "subject_ids": [1, 2]
}
```

Validasi:

- `user_id` harus memiliki role `guru_mapel` → jika tidak, `422 ACCOUNT_NOT_TEACHER_ROLE`.
- `user_id` belum memiliki profil guru → jika sudah ada, `409 TEACHER_PROFILE_ALREADY_EXISTS`.
- `nip` wajib diisi (tidak disyaratkan unik).
- `subject_ids` wajib minimal 1 mata pelajaran berstatus aktif (`422 SUBJECT_INACTIVE` bila mapel nonaktif).

#### `PATCH /api/v1/teachers/{id}`

```json
{
  "nip": "198705122015041002",
  "subject_ids": [1, 3],
  "updated_at": "2026-11-17T10:00:00.123456+07:00"
}
```

---

### 22. Data Siswa (`F3.1`)

> **Catatan PRD:** Tidak ada endpoint `POST` maupun `DELETE` siswa. Profil siswa otomatis dibuat saat registrasi siswa (`POST /auth/register` atau `POST /auth/google`).

#### `GET /api/v1/students` & `GET /api/v1/students/{id}`

Query parameter `GET /students`: `page`, `limit`, `q` (nama, email, NISN), `class_id`, `unassigned=true` (filter siswa Tanpa Kelas / `class_id = null`), `is_active` (`true`/`false`), `include_archived` (default `false`).

- **Scoping & Anti-IDOR Wali Kelas:** Jika pemanggil ber-role `wali_kelas` (tanpa role `admin`/`waka_kurikulum`/`waka_kesiswaan`), hasil `GET /students` otomatis dipaksa _server-side_ hanya untuk siswa di kelas perwaliannya. Meminta `class_id` kelas lain atau mengakses `GET /students/{id}` milik siswa kelas lain langsung ditolak dengan `403 FORBIDDEN`.

#### `PATCH /api/v1/students/{id}` (Admin & Waka Kurikulum)

Hanya menerima whitelist (`name`, `nisn`, `class_id`, `unassign_class`, dan `updated_at`):

```json
{
  "name": "Budi Santoso",
  "nisn": "1234567890",
  "class_id": 1,
  "updated_at": "2026-11-17T10:00:00.123456+07:00"
}
```

- Perubahan `name` dan `nisn` otomatis disinkronkan ke tabel `users` dalam 1 transaksi DB.
- `nisn` wajib 10 digit angka & unik (`409 CONFLICT` + `NISN_TAKEN`).
- `class_id` tujuan harus berstatus aktif (`422 CLASS_INACTIVE`). Untuk mengosongkan kelas siswa ("Tanpa Kelas"), kirim `"unassign_class": true`.

---

## Epic 4 — Schedule Management

Jadwal berbasis **tanggal spesifik (`YYYY-MM-DD`)**, bukan pola mingguan berulang. Tidak ada endpoint tambah jadwal manual satuan; jadwal baru dimasukkan melalui alur **Import Excel 2 Langkah**.

### Matriks Permission & Endpoint Epic 4

| Method | Path                                         | Permission                   | Role yang Diizinkan       | Deskripsi                            |
| ------ | -------------------------------------------- | ---------------------------- | ------------------------- | ------------------------------------ |
| GET    | `/api/v1/schedules/template`                 | `schedule.download_template` | **`admin`** (Waka `403`)  | Unduh file `.xlsx` template jadwal   |
| POST   | `/api/v1/schedules/import/preview`           | `schedule.import`            | **`admin`** (Waka `403`)  | Upload `.xlsx` & validasi preview    |
| POST   | `/api/v1/schedules/import/confirm`           | `schedule.import`            | **`admin`** (Waka `403`)  | Konfirmasi simpan hasil preview      |
| GET    | `/api/v1/schedules`                          | `schedule.view`              | Seluruh 7 role            | Daftar jadwal (di-scope sesuai role) |
| GET    | `/api/v1/schedules/{id}`                     | `schedule.view`              | Seluruh 7 role            | Detail jadwal (di-scope sesuai role) |
| PATCH  | `/api/v1/schedules/{id}`                     | `schedule.update`            | `admin`, `waka_kurikulum` | Ubah tanggal/jam/kelas/mapel/guru    |
| PATCH  | `/api/v1/schedules/{id}/convert-to-activity` | `schedule.convert_activity`  | `admin`, `waka_kurikulum` | Ubah jadwal pelajaran → kegiatan     |
| PATCH  | `/api/v1/schedules/{id}/revert-to-lesson`    | `schedule.convert_activity`  | `admin`, `waka_kurikulum` | Kembalikan kegiatan → pelajaran      |
| DELETE | `/api/v1/schedules/{id}`                     | `schedule.delete`            | **`admin`** (Waka `403`)  | Hapus baris jadwal permanen          |

---

### 23. Download Template Excel (`F4.1`)

`GET /api/v1/schedules/template` (Khusus Admin)

Mengembalikan binary file `.xlsx` (`Content-Type: application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`) dengan header baku:

1. `Tanggal (YYYY-MM-DD)`
2. `Jam Mulai (HH:MM)`
3. `Jam Selesai (HH:MM)`
4. `Nama Kelas`
5. `Kode Mata Pelajaran`
6. `Email Guru`

beserta 1 baris contoh.

---

### 24. Upload, Validasi Preview & Confirm Import (`F4.2`)

#### 24a. Preview Import (Khusus Admin)

`POST /api/v1/schedules/import/preview`

- **Content-Type:** `multipart/form-data` dengan field `file` berisi file `.xlsx`.
- Jika file rusak atau header kolom tidak sesuai template → `422 IMPORT_FILE_INVALID`.
- Memvalidasi setiap baris tanpa menyimpan ke database, lalu memisahkan ke 3 kelompok:
  - `valid_rows`: siap diimport.
  - `skipped_rows`: kombinasi `class_id + date + start_time` sudah ada di database (dilewati agar tidak menimpa jadwal yang sudah dikoreksi manual).
  - `error_rows`: baris tidak valid beserta `row_number` dan daftar `errors[]` (format tanggal/jam salah, `start_time >= end_time`, kelas/mapel tidak ditemukan atau nonaktif, guru tidak aktif atau bukan `guru_mapel`, atau jam bertumpuk).
- Mengembalikan `import_token` (berlaku 30 menit).

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "Validasi preview import jadwal selesai",
  "data": {
    "import_token": "8d1c2e3a-...",
    "expires_at": "2026-11-18T10:30:00+07:00",
    "total_rows": 3,
    "valid_count": 1,
    "skipped_count": 1,
    "error_count": 1,
    "valid_rows": [
      {
        "row_number": 2,
        "date": "2026-11-18",
        "start_time": "07:00",
        "end_time": "08:30",
        "class_name": "X RPL 1",
        "subject_code": "MTK-10",
        "teacher_email": "guru@example.com",
        "class_id": 1,
        "subject_id": 1,
        "teacher_id": 1
      }
    ],
    "skipped_rows": [],
    "error_rows": []
  }
}
```

#### 24b. Confirm Import (Khusus Admin)

`POST /api/v1/schedules/import/confirm`

```json
{
  "import_token": "8d1c2e3a-..."
}
```

- Jika `import_token` tidak ditemukan, sudah dipakai, atau kedaluwarsa → `422 IMPORT_NOT_VALIDATED`.
- Menyimpan seluruh `valid_rows` sebagai `type = "pelajaran"` dalam 1 transaksi DB dengan `ON CONFLICT (class_id, schedule_date, start_time) DO NOTHING` dan mencatat event `SCHEDULE_IMPORTED`.

---

### 25. Lihat Jadwal & Scoping Role (`F4.3`)

`GET /api/v1/schedules` & `GET /api/v1/schedules/{id}`

**Query parameter `GET /schedules`:**

- `date_from` & `date_to` (`YYYY-MM-DD`): Jika kosong, otomatis memakai **1 minggu berjalan (Senin s.d. Minggu)**. Jika `date_to < date_from` → `422 VALIDATION_ERROR` (`INVALID_DATE_RANGE`).
- `class_id`, `teacher_id`, `subject_id`, `type` (`pelajaran` atau `kegiatan`), `page`, `limit`.

**Aturan Server-Side Scoping berdasarkan Role:**

- **Admin, Waka Kurikulum, Waka Kesiswaan:** Melihat jadwal seluruh sekolah.
- **Guru Mapel:** Hasil query otomatis dipaksa _server-side_ hanya untuk `teacher_id` miliknya (mengabaikan manipulasi query param `teacher_id`). Mengakses `GET /schedules/{id}` milik guru lain ditolak `403 FORBIDDEN`.
- **Wali Kelas, Siswa Inti, Siswa:** Hasil query otomatis dipaksa _server-side_ hanya untuk `class_id` kelasnya. Meminta `class_id` kelas lain atau mengakses `GET /schedules/{id}` kelas lain ditolak `403 FORBIDDEN`.

---

### 26. Ubah Jadwal (`F4.4`)

`PATCH /api/v1/schedules/{id}` (Admin & Waka Kurikulum)

```json
{
  "date": "2026-11-18",
  "start_time": "08:30",
  "end_time": "10:00",
  "class_id": 1,
  "subject_id": 2,
  "teacher_id": 1,
  "updated_at": "2026-11-17T10:00:00.123456+07:00"
}
```

- Menolak dengan `409 SCHEDULE_SLOT_CONFLICT` bila slot waktu kelas atau guru bertumpuk dengan jadwal lain pada tanggal tersebut.
- Menolak dengan `409 STALE_DATA` bila `updated_at` sudah usang.

---

### 27. Konversi Kegiatan & Revert ke Pelajaran (`F4.6`)

#### 27a. Convert ke Kegiatan

`PATCH /api/v1/schedules/{id}/convert-to-activity` (Admin & Waka Kurikulum)

```json
{
  "activity_name": "Upacara Hari Pahlawan",
  "updated_at": "2026-11-17T10:00:00.123456+07:00"
}
```

- Mengubah `type` menjadi `"kegiatan"`, mengisi `activity_name`, mengembalikan `subject_id: null` pada response, dan **tetap mempertahankan `teacher_id`** (agar guru tetap bertanggung jawab mengisi jurnal).
- Jika jadwal sudah berstatus `"kegiatan"` → `409 SCHEDULE_ALREADY_ACTIVITY`.

#### 27b. Revert ke Pelajaran

`PATCH /api/v1/schedules/{id}/revert-to-lesson` (Admin & Waka Kurikulum)

```json
{
  "updated_at": "2026-11-17T10:05:00.123456+07:00"
}
```

- Mengembalikan `type` menjadi `"pelajaran"` dan mengosongkan `activity_name`.
- Jika kegiatan sudah memiliki data peserta/absensi kegiatan (Epic 8) → `409 SCHEDULE_IN_USE`.

---

### 28. Hapus Jadwal (`F4.5`)

`DELETE /api/v1/schedules/{id}` (Khusus Admin; Waka Kurikulum `403`)

- Jika jadwal sudah memiliki data terkait (absensi siswa, kehadiran guru, jurnal mengajar, atau absensi kegiatan) → `409 SCHEDULE_IN_USE`.
- Menyimpan snapshot ke `activity_logs` (`SCHEDULE_DELETED`) sebelum melakukan _hard delete_.

---

## Daftar Kode Error

Branching di frontend sebaiknya memakai field **`code`**, bukan `message`.

### Sukses

| Code                    | HTTP | Keterangan                           |
| ----------------------- | ---- | ------------------------------------ |
| `SUCCESS`               | 200  | Sukses umum                          |
| `CREATED`               | 201  | Resource dibuat                      |
| `USER_LIST`             | 200  | Daftar akun berhasil diambil         |
| `AUTH_REGISTER_SUCCESS` | 201  | Registrasi berhasil                  |
| `AUTH_LOGIN_SUCCESS`    | 200  | Login berhasil                       |
| `AUTH_REFRESH_SUCCESS`  | 200  | Token diperbarui                     |
| `AUTH_LOGOUT_SUCCESS`   | 200  | Logout berhasil                      |
| `AUTH_GOOGLE_SUCCESS`   | 200  | Login Google berhasil                |
| `OTP_REQUESTED`         | 200  | OTP terkirim                         |
| `OTP_VERIFIED`          | 200  | OTP valid, `reset_token` diterbitkan |
| `AUTH_PASSWORD_CHANGED` | 200  | Password berubah                     |

### Gagal

| Code                             | HTTP | Keterangan                                                                  |
| -------------------------------- | ---- | --------------------------------------------------------------------------- |
| `VALIDATION_ERROR`               | 422  | Validasi field gagal (lihat `errors[]`)                                     |
| `UNAUTHORIZED`                   | 401  | Token tidak ada/tidak valid, atau akun nonaktif pada request terautentikasi |
| `AUTH_TOKEN_EXPIRED`             | 401  | Access/refresh token kedaluwarsa                                            |
| `AUTH_SESSION_REVOKED`           | 401  | Sesi dicabut/tidak valid                                                    |
| `AUTH_INVALID_CREDENTIALS`       | 401  | Email/username atau password salah                                          |
| `AUTH_ACCOUNT_LOCKED`            | 429  | Akun terkunci sementara                                                     |
| `AUTH_ACCOUNT_INACTIVE`          | 403  | Akun nonaktif saat login; request terautentikasi memakai `UNAUTHORIZED`     |
| `FORBIDDEN`                      | 403  | Tidak punya hak akses (role/permission)                                     |
| `NOT_FOUND`                      | 404  | Data tidak ditemukan                                                        |
| `ACCOUNT_NOT_FOUND`              | 404  | Akun target tidak ditemukan                                                 |
| `AUTH_ACCOUNT_NOT_FOUND`         | 404  | Akun (untuk Google) belum terdaftar                                         |
| `CONFLICT`                       | 409  | Duplikat data                                                               |
| `EMAIL_ALREADY_EXISTS`           | 409  | Email sudah terdaftar pada manajemen akun                                   |
| `ACCOUNT_ALREADY_ACTIVE`         | 409  | Akun sudah aktif                                                            |
| `ACCOUNT_ALREADY_INACTIVE`       | 409  | Akun sudah nonaktif                                                         |
| `STALE_DATA`                     | 409  | `updated_at` sudah berubah; muat ulang lalu coba lagi                       |
| `LAST_ADMIN_PROTECTED`           | 409  | Operasi akan menghilangkan Admin terakhir                                   |
| `SELF_ACTION_FORBIDDEN`          | 403  | Operasi pada akun sendiri tidak diizinkan                                   |
| `ACCOUNT_HAS_HISTORY`            | 409  | Akun masih memiliki riwayat data (FK `ON DELETE RESTRICT`)                  |
| `ROLE_NOT_ASSIGNABLE`            | 422  | Role terkunci/tidak boleh diberikan lewat endpoint ini                      |
| `INVALID_ROLE_COMBINATION`       | 422  | Kombinasi role tidak valid atau akun tanpa role                             |
| `ACCOUNT_TYPE_NOT_EDITABLE`      | 422  | Akun siswa tidak boleh diubah/dihapus lewat manajemen akun                  |
| `ACADEMIC_YEAR_NOT_FOUND`        | 404  | Tahun ajaran tidak ditemukan                                                |
| `ACADEMIC_YEAR_ALREADY_EXISTS`   | 409  | Nama tahun ajaran sudah terdaftar                                           |
| `ACADEMIC_YEAR_IN_USE`           | 409  | Tahun ajaran tidak dapat dihapus karena sudah memiliki kelas                |
| `SUBJECT_NOT_FOUND`              | 404  | Mata pelajaran tidak ditemukan                                              |
| `SUBJECT_ALREADY_EXISTS`         | 409  | Kode mata pelajaran sudah terdaftar                                         |
| `SUBJECT_IN_USE`                 | 409  | Mata pelajaran sedang dipakai guru/jadwal (arahkan ke nonaktif)             |
| `CLASS_NOT_FOUND`                | 404  | Kelas tidak ditemukan                                                       |
| `CLASS_ALREADY_EXISTS`           | 409  | Nama kelas sudah terdaftar pada tahun ajaran tersebut                       |
| `CLASS_IN_USE`                   | 409  | Kelas masih memiliki siswa/jadwal terkait (arahkan ke nonaktif)             |
| `TEACHER_NOT_FOUND`              | 404  | Profil guru tidak ditemukan                                                 |
| `TEACHER_PROFILE_ALREADY_EXISTS` | 409  | Akun sudah memiliki profil guru                                             |
| `ACCOUNT_NOT_TEACHER_ROLE`       | 422  | Akun yang dipilih tidak memiliki role `guru_mapel`                          |
| `STUDENT_NOT_FOUND`              | 404  | Profil siswa tidak ditemukan                                                |
| `IMPORT_FILE_INVALID`            | 422  | File `.xlsx` rusak atau format header kolom tidak sesuai template           |
| `IMPORT_NOT_VALIDATED`           | 422  | `import_token` tidak ditemukan, sudah dipakai, atau kedaluwarsa             |
| `SCHEDULE_NOT_FOUND`             | 404  | Baris jadwal tidak ditemukan                                                |
| `SCHEDULE_SLOT_CONFLICT`         | 409  | Jadwal kelas atau guru bentrok pada tanggal dan jam tersebut                |
| `SCHEDULE_ALREADY_ACTIVITY`      | 409  | Jadwal sudah berstatus `kegiatan`                                           |
| `SCHEDULE_ALREADY_LESSON`        | 409  | Jadwal sudah berstatus `pelajaran`                                          |
| `SCHEDULE_IN_USE`                | 409  | Jadwal sudah memiliki data absensi/jurnal/kegiatan terkait                  |
| `INTERNAL_ERROR`                 | 500  | Kesalahan server                                                            |
| `RATE_LIMITED`                   | 429  | Terlalu banyak request per-IP                                               |
| `OTP_INVALID`                    | 400  | Kode OTP salah                                                              |
| `OTP_EXPIRED`                    | 400  | Kode OTP kedaluwarsa                                                        |
| `OTP_TOO_MANY_ATTEMPTS`          | 429  | Percobaan OTP habis                                                         |
| `OTP_RESEND_TOO_SOON`            | 429  | Cooldown kirim ulang belum lewat                                            |
| `OTP_RESEND_LIMIT`               | 429  | Batas kirim per jam tercapai                                                |
| `AUTH_RESET_TOKEN_INVALID`       | 401  | Reset token tidak valid/sudah dipakai                                       |
| `AUTH_RESET_TOKEN_EXPIRED`       | 401  | Reset token kedaluwarsa                                                     |
| `AUTH_PASSWORD_SAME`             | 400  | Password baru sama dengan password lama                                     |
| `AUTH_GOOGLE_DISABLED`           | 503  | Google Sign-In belum dikonfigurasi                                          |
| `AUTH_GOOGLE_TOKEN_INVALID`      | 401  | ID token Google tidak valid                                                 |
| `AUTH_GOOGLE_EMAIL_UNVERIFIED`   | 401  | Email Google belum terverifikasi                                            |

### Kode detail field (`errors[].code`)

| Code                           | Keterangan                                                                                                  |
| ------------------------------ | ----------------------------------------------------------------------------------------------------------- |
| `FIELD_REQUIRED`               | Field wajib diisi                                                                                           |
| `INVALID_EMAIL`                | Format email tidak valid                                                                                    |
| `TOO_SHORT`                    | Nilai lebih pendek dari batas minimum                                                                       |
| `TOO_LONG`                     | Nilai lebih panjang dari batas maksimum                                                                     |
| `INVALID_NISN`                 | NISN bukan 10 digit angka                                                                                   |
| `INVALID_USERNAME`             | Username bukan 3-30 karakter huruf kecil/angka/titik/underscore                                             |
| `INVALID_OTP_FORMAT`           | Kode OTP bukan 6 karakter `A-Z`/`2-9`                                                                       |
| `PASSWORD_TOO_SHORT`           | Password kurang dari 8 karakter                                                                             |
| `PASSWORD_TOO_LONG`            | Password lebih dari 72 karakter                                                                             |
| `PASSWORD_NEEDS_LOWER`         | Password belum memuat huruf kecil                                                                           |
| `PASSWORD_NEEDS_UPPER`         | Password belum memuat huruf besar                                                                           |
| `PASSWORD_NEEDS_DIGIT`         | Password belum memuat angka                                                                                 |
| `PASSWORD_NEEDS_SYMBOL`        | Password belum memuat simbol                                                                                |
| `PASSWORD_CONTAINS_WHITESPACE` | Password mengandung spasi                                                                                   |
| `PASSWORD_MISMATCH`            | `confirm_password` tidak sama dengan password baru                                                          |
| `EMAIL_TAKEN`                  | Email sudah terdaftar saat registrasi                                                                       |
| `EMAIL_ALREADY_EXISTS`         | Email sudah terdaftar saat manajemen akun                                                                   |
| `NISN_TAKEN`                   | NISN sudah terdaftar                                                                                        |
| `USERNAME_TAKEN`               | Username sudah dipakai                                                                                      |
| `INVALID_ID`                   | Path `id` bukan bilangan positif                                                                            |
| `INVALID_PAGE`                 | `page` bukan bilangan bulat ≥ 1                                                                             |
| `INVALID_LIMIT`                | `limit` bukan 1–100                                                                                         |
| `INVALID_STATUS`               | `status` bukan `active`/`inactive`                                                                          |
| `INVALID_INACTIVE_REASON`      | `inactive_reason` tidak valid; `graduated` hanya untuk siswa dan `no_class` tidak diterima lewat API manual |
| `INVALID_BOOLEAN`              | Query boolean bukan `true`/`false`                                                                          |
| `ROLE_NOT_FOUND`               | Ada nama role yang tidak dikenal                                                                            |
| `ROLE_NOT_ASSIGNABLE`          | Role terkunci/tidak boleh diberikan lewat endpoint ini                                                      |
| `INVALID_ROLE_COMBINATION`     | Role staf+siswa tercampur, `siswa_inti` tanpa `siswa`, atau akun tanpa role                                 |
| `ACCOUNT_TYPE_NOT_EDITABLE`    | Akun siswa tidak boleh diubah/dihapus lewat manajemen akun                                                  |
| `INVALID_DATE`                 | Format tanggal bukan `YYYY-MM-DD`                                                                           |
| `INVALID_DATE_RANGE`           | `end_date < start_date` atau `date_to < date_from`                                                          |
| `INVALID_TIME`                 | Format jam bukan `HH:MM`                                                                                    |
| `INVALID_TIME_RANGE`           | `start_time >= end_time`                                                                                    |
| `INVALID_SEMESTER`             | `active_semester` bukan `semester_1` atau `semester_2`                                                      |
| `INVALID_GRADE_LEVEL`          | `grade_level` bukan `10`, `11`, atau `12`                                                                   |
| `INVALID_SCHEDULE_TYPE`        | `type` jadwal bukan `pelajaran` atau `kegiatan`                                                             |
| `SUBJECT_INACTIVE`             | Mata pelajaran yang dipilih sedang nonaktif                                                                 |
| `CLASS_INACTIVE`               | Kelas tujuan yang dipilih sedang nonaktif                                                                   |

---

## Role Pengguna

Endpoint bisa dibatasi per role (middleware `RequireRoles`). Nilai role (dipakai di `data.user.roles`):

| Role             | Keterangan                           |
| ---------------- | ------------------------------------ |
| `admin`          | Administrator                        |
| `waka_kurikulum` | Wakil Kepala Sekolah — Kurikulum     |
| `waka_kesiswaan` | Wakil Kepala Sekolah — Kesiswaan     |
| `guru_mapel`     | Guru Mata Pelajaran                  |
| `wali_kelas`     | Wali Kelas                           |
| `siswa_inti`     | Siswa Inti                           |
| `siswa`          | Siswa (role default saat registrasi) |

Role bersifat **union** — satu akun bisa punya beberapa role. Role dengan
`assignable: false` (`wali_kelas`, `siswa_inti`, `siswa`) tidak dapat
diberikan lewat `POST /users` maupun `PUT /users/{id}/roles`; role tersebut
dipertahankan saat role assignable diganti.

---

## Rate Limit

- Berlaku untuk seluruh `/api/v1/auth/*`, **per alamat IP**.
- Default: **5 request/detik**, burst **10**.
- Melebihi batas → `429 RATE_LIMITED` + header `Retry-After: 60`.
- Frontend disarankan menangani `429` dengan backoff, dan **tidak** retry agresif.
