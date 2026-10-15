# E-Absensi — Dokumentasi API (Epic 1)

Referensi kontrak API untuk **tim frontend (Mobile, Web, Desktop)**. Dokumen ini berisi **konvensi response, alur autentikasi, daftar endpoint, kode error, role, dan rate limit**.

> Cara menjalankan server, konfigurasi `.env`, skenario uji manual, dan CORS ada di **[README.md](README.md)**.

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
