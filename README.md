# HRIS Backend

Backend modular monolith untuk aplikasi HRIS berdasarkan `../aluraplikasi.md`. Stack: Go, Gin, GORM, PostgreSQL, JWT, Argon2id, Redis, dan Asynq.

## Menjalankan lokal

1. Salin konfigurasi: `cp .env.example .env`.
2. Ganti `JWT_SECRET`, `POSTGRES_PASSWORD`, dan `SEED_ADMIN_PASSWORD` di `.env`.
3. Jalankan layanan: `make up`.
4. Buat data permission, role, master awal, dan Super Admin: `make seed`.
5. API tersedia di `http://localhost:8080`; health check di `GET /health`.

Jika PostgreSQL dan Redis sudah berjalan sebagai container terpisah di host, gunakan `DATABASE_URL` dan `REDIS_ADDRESS` dengan `localhost`, kemudian jalankan:

```bash
make seed
make run
```

Target `make seed` menjalankan seeder langsung dari host. `make seed-docker` hanya digunakan bersama stack PostgreSQL/Redis bawaan `docker-compose.yml`.

Migration bersifat versioned di folder `migrations`, dicatat pada tabel `schema_migrations`, dan tidak menggunakan `AutoMigrate`. Jangan menjalankan migration `down` pada data penting tanpa backup.

## Keputusan autentikasi

Access token JWT berlaku singkat dan dikirim frontend melalui `Authorization: Bearer ...`. Refresh token adalah nilai acak opaque; hanya hash SHA-256 yang disimpan di database dan token aslinya disimpan pada cookie `HttpOnly`, `SameSite=Lax` di path `/api/auth`. Refresh melakukan rotation dan logout merevoke session terkait. Pada production wajib gunakan HTTPS dan `REFRESH_COOKIE_SECURE=true`.

## Policy MVP yang perlu diketahui

- Server time pada `APP_TIMEZONE` menjadi sumber waktu attendance.
- Karena work schedule, weekend, dan holiday calendar belum didefinisikan, durasi leave dihitung sebagai hari kalender inklusif.
- Leave lintas tahun ditolak agar satu request tidak membebani dua record balance tahunan.
- Leave type yang mewajibkan attachment ditolak sampai workflow upload tervalidasi ditambahkan; file upload termasuk scope lanjutan.
- Employee dan user adalah entity terpisah. User harus dihubungkan melalui `employee.user_id` sebelum fitur self-service digunakan.
- Master/historical entity tidak menyediakan hard delete pada MVP; gunakan status aktif/nonaktif.

## Modul

- `internal/auth`, `internal/rbac`: login, refresh rotation, logout, current user, role/permission.
- `internal/employee`, `department`, `position`: organisasi dan validasi hierarchy manager.
- `internal/attendance`: clock-in/out berbasis server dan histori terpaginated.
- `internal/leave`: request, overlap, balance, manager/HR approval, cancellation, transaction dan row lock.
- `internal/notification`, `internal/audit`: notifikasi in-app dan audit trail sensitif.
- `cmd/api`, `cmd/worker`, `cmd/seed`: proses API, Asynq worker, dan seed idempotent.

Kontrak endpoint dan permission ada di [docs/api.md](docs/api.md).

## Pemeriksaan

```bash
make fmt
make test
make build
```
