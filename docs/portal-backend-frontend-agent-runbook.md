# Runbook Portal Backend, Frontend, dan Windows Agent

Panduan ini menjelaskan alur menjalankan IT Portal (backend + frontend) dan memasang IT Portal Agent pada Windows 10, Windows 11, atau Windows Server 2019+.

## 1. Komponen dan lokasi project

| Komponen | Lokasi default | Teknologi |
| --- | --- | --- |
| IT Portal backend + frontend | `C:\Users\Winarko\Documents\IT Sop` | Next.js, Prisma, PostgreSQL, Bun |
| IT Portal Agent | `C:\Users\Winarko\Documents\IT Portal Agent` | Go, SQLite, Windows Service |

Portal memakai satu aplikasi Next.js untuk backend (App Router/API routes) dan frontend. Tidak ada server frontend terpisah yang wajib dijalankan.

## 2. Prasyarat

Pasang dan pastikan tersedia:

- PostgreSQL aktif dan dapat diakses.
- Bun untuk Portal (`bun --version`).
- Go untuk build Agent (`go version`).
- PowerShell 5.1 atau lebih baru.
- Hak Administrator Windows untuk instalasi service Agent.

## 3. Menjalankan Portal untuk development

Buka PowerShell pertama:

```powershell
cd "C:\Users\Winarko\Documents\IT Sop"
bun install
Copy-Item .env.example .env
```

Edit `.env`. Nilai minimal yang harus benar:

```dotenv
DATABASE_URL="postgresql://postgres:postgres@localhost:5432/it_portal?schema=public"
BETTER_AUTH_SECRET="gunakan-secret-minimal-32-karakter"
BETTER_AUTH_URL="http://localhost:3000"
PUBLIC_APP_URL="http://localhost:3000"
APP_URL="http://localhost:3000"
APP_ENV="development"
CMDB_CREDENTIAL_KEY="gunakan-key-minimal-32-karakter"
```

Buat database `it_portal` terlebih dahulu jika belum ada, lalu jalankan migrasi dan seed:

```powershell
bunx prisma generate
bunx prisma migrate dev
bun run db:seed
```

Jalankan backend dan frontend Portal:

```powershell
bun run dev
```

Portal tersedia di [http://localhost:3000](http://localhost:3000). Hentikan dengan `Ctrl+C`.

Perintah validasi:

```powershell
bun run typecheck
bun run test
bun run build
```

## 4. Menjalankan Portal seperti production

Build dilakukan dari folder Portal:

```powershell
cd "C:\Users\Winarko\Documents\IT Sop"
bun install
bunx prisma generate
bunx prisma migrate deploy
bun run build
bun run start
```

`bun run start` menjalankan Next.js production pada port default `3000`. Gunakan reverse proxy HTTPS di depan Portal untuk deployment enterprise. Jangan gunakan `BETTER_AUTH_SECRET` atau encryption key development di production.

## 5. Menyiapkan Enrollment Key di Portal

Agent yang baru dipasang harus memakai Enrollment Key yang diterbitkan Portal:

1. Login sebagai administrator.
2. Buka `/agents/enrollment`.
3. Generate key dengan masa berlaku dan batas registrasi yang sesuai.
4. Salin plaintext key saat dialog masih terbuka. Key hanya ditampilkan sekali.
5. Jangan memasukkan key ke source code atau commit Git.

Untuk development, Portal dapat membuat Development Enrollment Key hanya ketika `APP_ENV=development`. Jangan gunakan key development untuk production.

## 6. Build Windows Agent

Dari PowerShell pada folder Agent:

```powershell
cd "C:\Users\Winarko\Documents\IT Portal Agent"
go test ./...
go build ./...
.\scripts\build-windows-amd64.ps1
```

Executable hasil build berada di:

```text
C:\Users\Winarko\Documents\IT Portal Agent\dist\itportal-agent.exe
```

Build script menetapkan `GOOS=windows` dan `GOARCH=amd64`.

## 7. Konfigurasi Agent sebelum registrasi

Salin executable ke folder deployment, misalnya `C:\Program Files\ITPortalAgent`, lalu buka PowerShell **Run as Administrator**:

```powershell
New-Item -ItemType Directory -Force "C:\Program Files\ITPortalAgent"
Copy-Item ".\dist\itportal-agent.exe" "C:\Program Files\ITPortalAgent\itportal-agent.exe"
```

Instalasi atau startup pertama membuat:

```text
C:\ProgramData\ITPortalAgent\config.yaml
C:\ProgramData\ITPortalAgent\agent.db
C:\ProgramData\ITPortalAgent\logs\
C:\ProgramData\ITPortalAgent\cache\
C:\ProgramData\ITPortalAgent\downloads\
C:\ProgramData\ITPortalAgent\queue\
```

Edit `C:\ProgramData\ITPortalAgent\config.yaml` dan isi hanya konfigurasi bootstrap:

```yaml
portal_url: "http://localhost:3000"
enrollment_key: "TEMPEL-KEY-DARI-PORTAL-DI-SINI"
tls_validation: true
proxy: ""
log_level: "info"
```

Untuk agent pada komputer lain, ganti `portal_url` dengan URL Portal yang dapat dijangkau komputer tersebut. Pada production gunakan `https://` dan biarkan `tls_validation: true`.

Agent menyimpan token hasil registrasi secara terenkripsi dengan Windows DPAPI di SQLite. Jangan menambahkan `agent_uuid`, `agent_token`, atau konfigurasi operasional Portal ke YAML.

## 8. Install dan jalankan Windows Service

Masih dari PowerShell Administrator:

```powershell
cd "C:\Program Files\ITPortalAgent"
.\itportal-agent.exe install
.\itportal-agent.exe start
```

Service name: `ITPortalAgent`  
Display name: `IT Portal Agent`

Periksa status registrasi dan heartbeat:

```powershell
.\itportal-agent.exe status
```

Registrasi manual (opsional, biasanya dilakukan otomatis saat service start):

```powershell
.\itportal-agent.exe register
```

Setelah registrasi berhasil, Agent menjalankan heartbeat otomatis, mengirim inventory hardware, dan mengirim software inventory sesuai interval konfigurasi Portal.

## 9. Perintah operasional Agent

```powershell
.\itportal-agent.exe start
.\itportal-agent.exe stop
.\itportal-agent.exe restart
.\itportal-agent.exe status
.\itportal-agent.exe inventory
.\itportal-agent.exe software
.\itportal-agent.exe reload-config
.\itportal-agent.exe config
.\itportal-agent.exe version
.\itportal-agent.exe uninstall
```

`inventory` dan `software` menjalankan collection segera. Jika Portal sedang tidak tersedia, payload disimpan di queue SQLite dan dicoba kembali oleh Sync Engine.

## 10. Verifikasi end-to-end

Di komputer Agent:

```powershell
.\itportal-agent.exe status
Get-Service ITPortalAgent
Get-Content "C:\ProgramData\ITPortalAgent\logs\agent.log" -Tail 100
```

Di Portal:

1. Buka `/agents` dan pastikan Agent terlihat.
2. Buka detail Agent untuk melihat heartbeat dan inventory.
3. Jika Agent belum memiliki Asset, buka `/discovery`.
4. Administrator memilih **Create Asset**, **Match Existing Asset**, atau **Ignore**. Asset tidak dibuat otomatis.
5. Setelah Asset dikelola, hubungan Agent → Asset → User terlihat dari halaman terkait.

## 11. Troubleshooting

### Agent gagal registrasi

- Pastikan `portal_url` benar dan port Portal dapat dijangkau.
- Pastikan Enrollment Key masih `Active`, belum expired, dan belum exhausted.
- Pastikan waktu Windows benar untuk validasi timestamp/signature.
- Baca `agent.log` dan jalankan `register` dari PowerShell Administrator.

### Heartbeat `PENDING` atau `OFFLINE`

- Pastikan service benar-benar running: `Get-Service ITPortalAgent`.
- Pastikan Agent sudah registered dan token DPAPI dapat dibaca oleh service account.
- Periksa queue dan log; payload offline akan dikirim ulang otomatis.

### Portal tidak dapat terhubung ke PostgreSQL

- Uji service PostgreSQL.
- Periksa `DATABASE_URL` di `.env`.
- Jalankan `bunx prisma migrate status`.
- Jangan menjalankan `prisma migrate reset` pada database production.

### Service perlu dihapus

```powershell
.\itportal-agent.exe stop
.\itportal-agent.exe uninstall
```

Uninstall service tidak otomatis menghapus data `C:\ProgramData\ITPortalAgent`; simpan folder tersebut jika diperlukan untuk audit atau troubleshooting.

## 12. Batasan sprint saat ini

Fondasi ini belum menjalankan remote script, command execution, auto-update, policy execution, screenshot upload, atau software remediation. Semua fitur tersebut tetap menjadi sprint lanjutan.
