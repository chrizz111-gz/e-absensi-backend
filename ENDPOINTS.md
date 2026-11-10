# E-Absensi — Dokumentasi API (Epic 1 & 2)

Referensi kontrak API untuk **tim frontend (Mobile, Web, Desktop)**. Dokumen ini berisi **konvensi response, alur autentikasi, daftar endpoint, kode error, role, permission, dan rate limit**.

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
  "data": { }
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

| Field | Tipe | Keterangan |
|---|---|---|
| `success` | bool | `true` bila sukses |
| `code` | string | Kode mesin (untuk branching di frontend). Lihat [Daftar Kode Error](#daftar-kode-error) |
| `message` | string | Pesan human-readable (Bahasa Indonesia) |
| `data` | object \| null | Payload sukses, atau data tambahan pada error OTP (mis. `attempts_remaining`) |
| `errors` | object[] | Hanya ada saat `VALIDATION_ERROR`/`CONFLICT`. Bentuk: `{ field, code, message }` |

**HTTP status yang dipakai:**

| Status | Arti |
|---|---|
| `200 OK` | Sukses |
| `201 Created` | Resource berhasil dibuat (registrasi/POST user) |
| `400 Bad Request` | Permintaan tidak valid (mis. OTP salah/kedaluwarsa) |
| `401 Unauthorized` | Kredensial/token salah, kedaluwarsa, atau akun nonaktif pada request terautentikasi |
| `403 Forbidden` | Tidak punya hak akses (role/permission) |
| `404 Not Found` | Data/akun tak ditemukan |
| `409 Conflict` | Duplikat data, data sudah berubah (stale), proteksi Admin terakhir, atau akun masih punya riwayat |
| `422 Unprocessable Entity` | Validasi field gagal |
| `429 Too Many Requests` | Terkena lockout / rate limit / batas OTP |
| `500 Internal Server Error` | Kesalahan server |
| `503 Service Unavailable` | Fitur belum dikonfigurasi (mis. Google) |

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

| Method | Path | Auth | Deskripsi |
|---|---|---|---|
| GET | `/api/status` | — | Health check |
| POST | `/api/v1/auth/register` | — | Registrasi siswa |
| POST | `/api/v1/auth/login` | — | Login email/username |
| POST | `/api/v1/auth/google` | — | Login via Google |
| POST | `/api/v1/auth/refresh` | — | Tukar refresh token |
| POST | `/api/v1/auth/forgot-password` | — | Minta OTP reset password |
| POST | `/api/v1/auth/forgot-password/resend` | — | Kirim ulang OTP reset |
| POST | `/api/v1/auth/forgot-password/verify-otp` | — | Cek OTP reset → `reset_token` |
| POST | `/api/v1/auth/forgot-password/reset` | — | Simpan password baru dengan `reset_token` |
| POST | `/api/v1/auth/logout` 🔒 | Bearer | Logout device ini |
| POST | `/api/v1/auth/logout-all` 🔒 | Bearer | Logout semua device |
| POST | `/api/v1/auth/change-password/request` 🔒 | Bearer | Minta OTP ganti password |
| POST | `/api/v1/auth/change-password/resend` 🔒 | Bearer | Kirim ulang OTP ganti password |
| POST | `/api/v1/auth/change-password/verify-otp` 🔒 | Bearer | Cek OTP ganti password → `reset_token` |
| POST | `/api/v1/auth/change-password/confirm` 🔒 | Bearer | Simpan password baru dengan `reset_token` |

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
| `password` | string | ✅ | 8–72 karakter, huruf besar/kecil, angka, simbol, tanpa spasi |
| `confirm_password` | string | ✅ | Harus sama dengan `password` |
| `username` | string | ❌ | 3–30 karakter (huruf kecil, angka, titik, underscore), unik; huruf besar otomatis dinormalisasi |

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

| Field | Tipe | Wajib | Keterangan |
|---|---|---|---|
| `identifier` | string | ✅ | Email **atau** username |
| `password` | string | ✅ | |
| `device_info` | string | ❌ | Info device, maks 255 |

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{ "identifier": "budi@example.com", "password": "Rahasia123!", "device_info": "Android 14" }'
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

| Field | Tipe | Wajib | Aturan |
|---|---|---|---|
| `code` | string | ✅ | Tepat 6 karakter `A-Z`/`2-9` (input lowercase dinormalisasi) |

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

| Field | Tipe | Wajib | Aturan |
|---|---|---|---|
| `reset_token` | string | ✅ | Dari `verify-otp` |
| `new_password` | string | ✅ | 8–72 karakter, huruf besar/kecil, angka, simbol |
| `confirm_password` | string | ✅ | Harus sama dengan `new_password` |

```json
{ "success": true, "code": "AUTH_PASSWORD_CHANGED", "message": "Password berhasil diubah..." }
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

| Field | Tipe | Wajib | Aturan |
|---|---|---|---|
| `email` | string | ✅ | Format email |
| `code` | string | ✅ | Tepat 6 karakter `A-Z`/`2-9` (input lowercase dinormalisasi) |

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

| Field | Tipe | Wajib | Aturan |
|---|---|---|---|
| `reset_token` | string | ✅ | Dari `verify-otp` |
| `new_password` | string | ✅ | 8–72 karakter, huruf besar/kecil, angka, simbol |
| `confirm_password` | string | ✅ | Harus sama dengan `new_password` |

```json
{ "success": true, "code": "AUTH_PASSWORD_CHANGED", "message": "Password berhasil direset..." }
```

> Semua sesi dicabut → arahkan user login ulang.

---

### Aturan OTP

| Aspek | Nilai |
|---|---|
| Panjang | 6 karakter alfanumerik uppercase (contoh `A7K2Q9`) |
| Charset | `A-Z` + `2-9`; tanpa `0/O/1/I/L` agar tidak ambigu |
| Masa berlaku | 5 menit |
| Cooldown kirim ulang | 60 detik |
| Maks kirim ulang | 5 kali/jam |
| Maks percobaan salah | 5 kali (lalu hangus) |
| Sekali pakai | Ya (dikonsumsi setelah password benar-benar disimpan) |
| Reset token | Sekali pakai, umur maks 10 menit dan tidak melebihi sisa umur OTP |
| Isolasi purpose | OTP `change_password` ≠ `reset_password` |

---

## Epic 2 — User & Role Management

Seluruh endpoint Epic 2 wajib memakai **Bearer access token**. Endpoint user
(`/api/v1/users`) hanya dapat diakses role `admin` dan tetap dilindungi
permission granular dari database. Otorisasi tidak memakai klaim role di token.
Bagian ini mencakup Fase 2 (RBAC read-only) dan Fase 3 (CRUD user & role).

Permission yang dipakai:

| Permission | Dipakai oleh |
|---|---|
| `account.view` | `GET /users`, `GET /users/{id}` |
| `account.create` | `POST /users` |
| `account.set_status` | `PATCH /users/{id}/status` |
| `role.assign` | `PUT /users/{id}/roles`, `GET /roles` |
| `account.update` | `PATCH /users/{id}` |
| `account.delete` | `DELETE /users/{id}` |
| `permission.view` | `GET /permissions/matrix` |

### Referensi Endpoint Epic 2

| Method | Path | Auth | Permission | Deskripsi |
|---|---|---|---|---|
| GET | `/api/v1/roles` | Bearer | `role.assign` | Daftar role master |
| GET | `/api/v1/permissions/matrix` | Bearer | `permission.view` | Matriks hak akses read-only |
| GET | `/api/v1/users` | Bearer + admin | `account.view` | Daftar user (pagination/filter) |
| GET | `/api/v1/users/{id}` | Bearer + admin | `account.view` | Detail user + role |
| POST | `/api/v1/users` | Bearer + admin | `account.create` | Buat akun staf |
| PATCH | `/api/v1/users/{id}/status` | Bearer + admin | `account.set_status` | Aktifkan/nonaktifkan akun |
| PUT | `/api/v1/users/{id}/roles` | Bearer + admin | `role.assign` | Ganti role assignable |
| PATCH | `/api/v1/users/{id}` | Bearer + admin | `account.update` | Ubah profil |
| DELETE | `/api/v1/users/{id}` | Bearer + admin | `account.delete` | Hapus akun staf |

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

| Parameter | Wajib | Default | Aturan |
|---|---|---|---|
| `page` | ❌ | `1` | Bilangan bulat ≥ 1 |
| `limit` | ❌ | `20` | Bilangan bulat 1–100 |
| `q` | ❌ | — | Pencarian partial pada `name`, `email`, dan `nisn` |
| `role[]` | ❌ | — | Nama role; boleh diulang. User cocok bila memiliki **salah satu** role |
| `status` | ❌ | — | `active` atau `inactive` |
| `include_archived` | ❌ | `false` | `true` untuk menampilkan akun yang sudah diarsipkan (`archived_at` terisi) |

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

| Field | Tipe | Wajib | Aturan |
|---|---|---|---|
| `name` | string | ✅ | 2–150 karakter; di-trim |
| `email` | string | ✅ | Format email, unik, maks 255; di-trim dan lowercase |
| `roles` | string[] | ✅ | Minimal 1 **nama role**, hanya `assignable: true` |

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

| Field | Tipe | Wajib | Aturan |
|---|---|---|---|
| `is_active` | bool | ✅ | `false` = nonaktif, `true` = aktif |
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

| Field | Tipe | Wajib | Aturan |
|---|---|---|---|
| `roles` | string[] | ✅ | Boleh array kosong bila role terkunci masih tersisa. Hanya role dengan `assignable: true` |

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

| Field | Tipe | Wajib | Aturan |
|---|---|---|---|
| `name` | string | ❌ | 2–150 karakter; di-trim |
| `email` | string | ❌ | Format email, unik, maks 255; di-trim dan lowercase |
| `updated_at` | string (RFC3339) | ❌ | Nilai `updated_at` terakhir dari GET detail; untuk optimistic locking |

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

## Daftar Kode Error

Branching di frontend sebaiknya memakai field **`code`**, bukan `message`.

### Sukses

| Code | HTTP | Keterangan |
|---|---|---|
| `SUCCESS` | 200 | Sukses umum |
| `CREATED` | 201 | Resource dibuat |
| `USER_LIST` | 200 | Daftar akun berhasil diambil |
| `AUTH_REGISTER_SUCCESS` | 201 | Registrasi berhasil |
| `AUTH_LOGIN_SUCCESS` | 200 | Login berhasil |
| `AUTH_REFRESH_SUCCESS` | 200 | Token diperbarui |
| `AUTH_LOGOUT_SUCCESS` | 200 | Logout berhasil |
| `AUTH_GOOGLE_SUCCESS` | 200 | Login Google berhasil |
| `OTP_REQUESTED` | 200 | OTP terkirim |
| `OTP_VERIFIED` | 200 | OTP valid, `reset_token` diterbitkan |
| `AUTH_PASSWORD_CHANGED` | 200 | Password berubah |

### Gagal

| Code | HTTP | Keterangan |
|---|---|---|
| `VALIDATION_ERROR` | 422 | Validasi field gagal (lihat `errors[]`) |
| `UNAUTHORIZED` | 401 | Token tidak ada/tidak valid, atau akun nonaktif pada request terautentikasi |
| `AUTH_TOKEN_EXPIRED` | 401 | Access/refresh token kedaluwarsa |
| `AUTH_SESSION_REVOKED` | 401 | Sesi dicabut/tidak valid |
| `AUTH_INVALID_CREDENTIALS` | 401 | Email/username atau password salah |
| `AUTH_ACCOUNT_LOCKED` | 429 | Akun terkunci sementara |
| `AUTH_ACCOUNT_INACTIVE` | 403 | Akun nonaktif saat login; request terautentikasi memakai `UNAUTHORIZED` |
| `FORBIDDEN` | 403 | Tidak punya hak akses (role/permission) |
| `NOT_FOUND` | 404 | Data tidak ditemukan |
| `ACCOUNT_NOT_FOUND` | 404 | Akun target tidak ditemukan |
| `AUTH_ACCOUNT_NOT_FOUND` | 404 | Akun (untuk Google) belum terdaftar |
| `CONFLICT` | 409 | Duplikat data |
| `EMAIL_ALREADY_EXISTS` | 409 | Email sudah terdaftar pada manajemen akun |
| `ACCOUNT_ALREADY_ACTIVE` | 409 | Akun sudah aktif |
| `ACCOUNT_ALREADY_INACTIVE` | 409 | Akun sudah nonaktif |
| `STALE_DATA` | 409 | `updated_at` sudah berubah; muat ulang lalu coba lagi |
| `LAST_ADMIN_PROTECTED` | 409 | Operasi akan menghilangkan Admin terakhir |
| `SELF_ACTION_FORBIDDEN` | 403 | Operasi pada akun sendiri tidak diizinkan |
| `ACCOUNT_HAS_HISTORY` | 409 | Akun masih memiliki riwayat data (FK `ON DELETE RESTRICT`) |
| `ROLE_NOT_ASSIGNABLE` | 422 | Role terkunci/tidak boleh diberikan lewat endpoint ini |
| `INVALID_ROLE_COMBINATION` | 422 | Kombinasi role tidak valid atau akun tanpa role |
| `ACCOUNT_TYPE_NOT_EDITABLE` | 422 | Akun siswa tidak boleh diubah/dihapus lewat manajemen akun |
| `INTERNAL_ERROR` | 500 | Kesalahan server |
| `RATE_LIMITED` | 429 | Terlalu banyak request per-IP |
| `OTP_INVALID` | 400 | Kode OTP salah |
| `OTP_EXPIRED` | 400 | Kode OTP kedaluwarsa |
| `OTP_TOO_MANY_ATTEMPTS` | 429 | Percobaan OTP habis |
| `OTP_RESEND_TOO_SOON` | 429 | Cooldown kirim ulang belum lewat |
| `OTP_RESEND_LIMIT` | 429 | Batas kirim per jam tercapai |
| `AUTH_RESET_TOKEN_INVALID` | 401 | Reset token tidak valid/sudah dipakai |
| `AUTH_RESET_TOKEN_EXPIRED` | 401 | Reset token kedaluwarsa |
| `AUTH_PASSWORD_SAME` | 400 | Password baru sama dengan password lama |
| `AUTH_GOOGLE_DISABLED` | 503 | Google Sign-In belum dikonfigurasi |
| `AUTH_GOOGLE_TOKEN_INVALID` | 401 | ID token Google tidak valid |
| `AUTH_GOOGLE_EMAIL_UNVERIFIED` | 401 | Email Google belum terverifikasi |

### Kode detail field (`errors[].code`)

| Code | Keterangan |
|---|---|
| `FIELD_REQUIRED` | Field wajib diisi |
| `INVALID_EMAIL` | Format email tidak valid |
| `TOO_SHORT` | Nilai lebih pendek dari batas minimum |
| `TOO_LONG` | Nilai lebih panjang dari batas maksimum |
| `INVALID_NISN` | NISN bukan 10 digit angka |
| `INVALID_USERNAME` | Username bukan 3-30 karakter huruf kecil/angka/titik/underscore |
| `INVALID_OTP_FORMAT` | Kode OTP bukan 6 karakter `A-Z`/`2-9` |
| `PASSWORD_TOO_SHORT` | Password kurang dari 8 karakter |
| `PASSWORD_TOO_LONG` | Password lebih dari 72 karakter |
| `PASSWORD_NEEDS_LOWER` | Password belum memuat huruf kecil |
| `PASSWORD_NEEDS_UPPER` | Password belum memuat huruf besar |
| `PASSWORD_NEEDS_DIGIT` | Password belum memuat angka |
| `PASSWORD_NEEDS_SYMBOL` | Password belum memuat simbol |
| `PASSWORD_CONTAINS_WHITESPACE` | Password mengandung spasi |
| `PASSWORD_MISMATCH` | `confirm_password` tidak sama dengan password baru |
| `EMAIL_TAKEN` | Email sudah terdaftar saat registrasi |
| `EMAIL_ALREADY_EXISTS` | Email sudah terdaftar saat manajemen akun |
| `NISN_TAKEN` | NISN sudah terdaftar |
| `USERNAME_TAKEN` | Username sudah dipakai |
| `INVALID_ID` | Path `id` bukan bilangan positif |
| `INVALID_PAGE` | `page` bukan bilangan bulat ≥ 1 |
| `INVALID_LIMIT` | `limit` bukan 1–100 |
| `INVALID_STATUS` | `status` bukan `active`/`inactive` |
| `INVALID_INACTIVE_REASON` | `inactive_reason` tidak valid; `graduated` hanya untuk siswa dan `no_class` tidak diterima lewat API manual |
| `INVALID_BOOLEAN` | Query boolean bukan `true`/`false` |
| `ROLE_NOT_FOUND` | Ada nama role yang tidak dikenal |
| `ROLE_NOT_ASSIGNABLE` | Role terkunci/tidak boleh diberikan lewat endpoint ini |
| `INVALID_ROLE_COMBINATION` | Role staf+siswa tercampur, `siswa_inti` tanpa `siswa`, atau akun tanpa role |
| `ACCOUNT_TYPE_NOT_EDITABLE` | Akun siswa tidak boleh diubah/dihapus lewat manajemen akun |

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
