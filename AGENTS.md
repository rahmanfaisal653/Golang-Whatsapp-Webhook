# AGENTS.md — BotVista WhatsApp Gateway & Webhook Notifier

> **PANDUAN OPERASIONAL & ARSITEKTUR PENGETAHUAN (KNOWLEDGE BASE) AI AGENT**  
> Berkas ini adalah **single source of truth** bagi AI Agent (Antigravity CLI `agy`, Cursor, Claude Code, Cline, dll.) yang bertugas memelihara, mengompilasi, mengembangkan fitur, atau mendiagnosis gateway **BotVista WhatsApp** (`Golang-Whatsapp-Webhook`).

---

## 1. Topologi Lingkungan & Identitas Sistem

| Entitas | Detail / Nilai | Catatan Operasional |
|---|---|---|
| **Nama Proyek / Brand** | **BotVista WhatsApp Gateway** (Nouvem GoWA) | Multi-Device WhatsApp Engine & Webhook Notifier Hub |
| **Server Host Runtime** | **Minibox2** (Ubuntu Server Linux x86_64) | IP LAN: `192.168.100.186` \| ZeroTier: `100.90.80.110` |
| **Mount / Storage NAS** | **Synology NAS DS923+** (DSM 7.2) | Root repo: `/volume1/web/Nouvem/Golang-Whatsapp-Webhook` |
| **Port HTTP Server** | **`18080`** (Internal LAN & Localhost) | HTTP REST API & Web UI Dashboard |
| **Public Gateway URL** | `https://kroomhook.kroombox.com` | Ingress publik Cloudflare Tunnel (Edge Anycast) |
| **Endpoint Notifier** | `POST http://192.168.100.186:18080/notify` | Endpoint notifikasi serbaguna (backward-compatible) |
| **Process Manager** | **PM2** di Minibox2: `nouvem-wa-webhook` | Binary file: `./gowa-nouvem` |
| **Bahasa & Library Inti** | **Go (Golang 1.21+)** + **`go.mau.fi/whatsmeow`** | Native Multi-Device Protocol (Noise Handshake / Protobuf) |
| **Penyimpanan Sesi** | SQLite (WAL mode): `src/session/whatsmeow.db` | Tahan reboot, menyimpan token pairing & keys |
| **Penyimpanan Fitur** | JSON terstruktur: `src/session/features.json` | Konfigurasi global, auto-reply, registri webhook |

---

## 2. Peta Direktori & Tanggung Jawab Modul

```text
/volume1/web/Nouvem/Golang-Whatsapp-Webhook/
├── main.go                     # Entrypoint aplikasi, context lifecycle, graceful shutdown, wiring modul
├── gowa-nouvem                 # Binary Linux executable hasil kompilasi Go (di-run oleh PM2)
├── go.mod / go.sum             # Dependensi Go (whatsmeow, sqlite3, qr, protobuf)
├── README.md                   # Quickstart ringkas proyek
├── docs/                       # Dokumentasi integrasi eksternal & petunjuk notifier API
│   ├── integration.md          # Panduan integrasi sistem luar ke /notify
│   └── notifier.md             # Format JSON & contoh curl notifier
├── commands/                   # Command handler perintah WhatsApp bawaan (prefix '.')
│   └── owner/
│       ├── basic.go            # .about, .menu, .uptime, .id
│       ├── groups.go           # .groups (daftar grup yang diikuti beserta JID-nya)
│       ├── ping.go             # .ping (responsivitas bot 'pong')
│       └── test.go             # .test (uji coba webhook loopback lokal)
└── src/
    ├── features/               # Engine konfigurasi & registri fitur dinamis
    │   └── features.go         # Model GlobalSettings, Builtins, Customs, Webhooks, RWMutex, persistence
    ├── lib/                    # Utilitas parser, formatting, dan handler WhatsApp
    │   ├── commands.go         # Dispatcher pesan masuk, dynamic client provider, custom auto-responder
    │   └── jid.go              # Normalisasi nomor telepon ke JID WhatsApp (@s.whatsapp.net / @g.us)
    ├── notify/                 # Handler lama notifier (legacy)
    │   └── listen.go
    ├── session/                # Database sesi aktif & file konfigurasi live
    │   ├── whatsmeow.db        # SQLite DB sesi autentikasi WhatsApp
    │   ├── whatsmeow.db-wal    # Write-Ahead Log SQLite
    │   ├── whatsmeow.db-shm    # Shared memory index SQLite
    │   └── features.json       # State penyimpanan fitur & integrasi webhook live
    └── web/                    # Server dashboard Web UI & REST API manajemen
        ├── manager.go          # WhatsAppManager: koneksi, QR code emitter, event handling, in-memory logs
        ├── server.go           # REST API HTTP mux, CORS, route /api/*, interceptor /notify
        └── ui.html             # Dashboard interaktif Web UI (Glassmorphism design, auto-refresh)
```

---

## 3. Arsitektur & Subsistem Utama

### 3.1 WhatsApp Multi-Device Engine (`src/web/manager.go`)
- Menggunakan library `whatsmeow` dengan storage backend SQLite `sqlstore.New("sqlite3", ...)`.
- **Koneksi Otomatis**: Mendeteksi apakah sesi di `whatsmeow.db` sudah terpasang. Jika ya, langsung `Connect()`. Jika belum, memancarkan QR channel untuk dipindai melalui dashboard web atau CLI.
- **Dynamic Client Provider**: Menggunakan `GetClient()` dinamis di event handler pesan masuk. Saat user melakukan logout dan memindai nomor baru, reference client terbarukan secara otomatis tanpa perlu me-restart proses binary.

### 3.2 Feature & Rule Management (`src/features/features.go`)
- **Global Settings**:
  - `bot_enabled`: Saklar utama apakah bot merespons pesan masuk.
  - `webhook_enabled`: Saklar utama izin penerimaan notifikasi via endpoint `/notify`.
  - `outgoing_webhook_url`: URL tujuan forward otomatis pesan chat WhatsApp yang diterima ke sistem luar (duplex chatbot / n8n / agent).
  - `group_response`: Izin merespons di dalam grup WhatsApp.
  - `private_response`: Izin merespons di chat personal / DM.
  - `auto_reconnect`: Automasi rekoneksi saat websocket terputus.
- **Built-in Commands Registry**:
  - Pengaturan status aktif/nonaktif masing-masing command `.ping`, `.about`, `.menu`, `.uptime`, `.id`, `.groups`, `.test` secara granular.
- **Custom Auto-Responders**:
  - Trigger teks dengan mode pencocokan: `exact` (sama persis), `prefix` (awalan kata), atau `contains` (mengandung frasa).
  - Variabel placeholder dinamis: `{sender}` (nama/nomor pengirim), `{time}` (jam WIB saat ini).
  - Target pembatasan: `all`, `group_only`, atau `private_only`.

### 3.3 Registered Webhook Integrations Hub
- Registri terpadu untuk melacak seluruh sistem luar yang mengirim notifikasi via BotVista.
- **Entri Default**:
  1. `wh_kp_guard_nas`: Pemantau suhu fisik CPU (>=86°C sustained, >=92°C emergency), suhu drive storage (>60°C), dan beban Synology DS923+. Target: `120363404628369352@g.us` (Grup KroomBox).
  2. `wh_kolabpanel_deploy`: Notifikasi otomatis deployment, SSL renew, dan site build di server hosting KolabPanel. Target: `120363404628369352@g.us` (Grup KroomBox).
- **Interseptor Otomatis `TouchWebhook`**:
  - Setiap request valid yang masuk ke `/notify`, server mengecek parameter `req.To` dan `req.Source`. Timestamp `LastUsed` pada registri webhook terkait otomatis diperbarui menjadi waktu live request tersebut.

---

## 4. Katalog REST API & Spesifikasi Endpoint

### 4.1 Notifier Universal (`POST /notify`)
Endpoint utama untuk aplikasi eksternal (KolabPanel, KP-Guard, script backend) mengirim notifikasi WhatsApp.

* **URL**: `POST http://192.168.100.186:18080/notify` (atau via tunnel `https://kroomhook.kroombox.com/notify`)
* **Headers**: `Content-Type: application/json`
* **Request Body**:
  ```json
  {
    "to": "120363404628369352@g.us",
    "message": "🔥 [KP-GUARD NAS] Suhu CPU 88°C",
    "source": "KP-Guard NAS"
  }
  ```
* **Normalisasi Penerima (`to`) Otomatis**:
  - Nomor personal lokal Indonesia: `082218542122` atau `6282218542122` otomatis dinormalisasi menjadi `6282218542122@s.whatsapp.net`.
  - Format internasional: `+628...` otomatis dibersihkan karakternya menjadi format JID standar.
  - Grup WhatsApp: JID berakhiran `@g.us` langsung diteruskan tanpa modifikasi.
* **Success Response (200 OK)**:
  ```json
  {
    "success": true,
    "status": "DELIVERED",
    "message": "notify applied",
    "to": "120363404628369352@g.us",
    "time": "15:23:26"
  }
  ```
* **Error Response**:
  - `503 Service Unavailable`: Jika fitur webhook dimatikan di dashboard (`webhook_enabled: false`).
  - `400 Bad Request`: Format JSON rusak, nomor tujuan kosong, atau pesan kosong.
  - `502 Bad Gateway`: Pengiriman WhatsApp gagal (koneksi terputus / nomor tidak terdaftar).

---

### 4.2 Manajemen Dashboard & Sesi WhatsApp

| Endpoint | Method | Fungsi & Keterangan |
|---|---|---|
| `/` | `GET` | Web UI Dashboard interaktif BotVista Glassmorphism. |
| `/api/status` | `GET` | Cek status koneksi: `{"connected": true, "user": "628...:12@s.whatsapp.net", "has_session": true}`. |
| `/api/qr.png` | `GET` | Menghasilkan gambar QR code PNG real-time untuk pairing nomor WhatsApp baru. |
| `/api/logout` | `POST` | Memutus pairing sesi aktif, menghapus session DB, dan mereset status ke unlinked. |
| `/api/reconnect` | `POST` | Memaksa koneksi ulang websocket ke server WhatsApp. |
| `/api/groups` | `GET` | Mengambil daftar seluruh grup WhatsApp yang diikuti bot beserta nama dan JID `@g.us`. |
| `/api/logs` | `GET` | Mengambil 100 log transaksi & aktivitas pesan terakhir dari memori server. |
| `/api/send` | `POST` | Mengirim pesan langsung manual dari dashboard UI. Payload: `{"to": "...", "message": "..."}`. |

---

### 4.3 Manajemen Fitur & Konfigurasi

| Endpoint | Method | Fungsi & Payload |
|---|---|---|
| `/api/features` | `GET` | Mengambil seluruh konfigurasi settings, built-in commands, customs, dan webhooks. |
| `/api/features/settings` | `POST` | Mengupdate pengaturan global (`GlobalSettings`). Body: `{"bot_enabled": true, "webhook_enabled": true, ...}`. |
| `/api/features/builtins/toggle` | `POST` | Toggle aktif/nonaktif command bawaan. Body: `{"id": "ping", "enabled": true}`. |
| `/api/features/customs` | `POST` | Membuat custom auto-responder baru. Body: `{"trigger": "halo", "match_type": "exact", "response": "...", "target": "all"}`. |
| `/api/features/customs/toggle` | `POST` | Mengaktifkan/menonaktifkan auto-responder tertentu. Body: `{"id": "feat_welcome", "enabled": false}`. |
| `/api/features/customs/update` | `POST` | Mengubah isi trigger atau template balasan pesan custom. |
| `/api/features/customs/delete` | `POST` | Menghapus aturan custom auto-responder berdasarkan ID. |

---

### 4.4 Manajemen Registri Webhook Terdaftar

| Endpoint | Method | Fungsi & Payload |
|---|---|---|
| `/api/webhooks` | `GET` | Menampilkan daftar seluruh integrasi webhook terdaftar. |
| `/api/webhooks` | `POST` | Mendaftarkan integrasi webhook baru ke katalog registry. |
| `/api/webhooks/toggle` | `POST` | Menyalakan/mematikan webhook integrasi tertentu (`{"id": "wh_kp_guard_nas", "enabled": true}`). |
| `/api/webhooks/update` | `POST` | Memperbarui nama, deskripsi, source, atau target webhook. |
| `/api/webhooks/delete` | `POST` | Menghapus integrasi webhook dari registry (`{"id": "..."}`). |
| `/api/webhooks/test` | `POST` | **Test Ping Instan**: Mengirim pesan uji koneksi ke target WhatsApp dan memperbarui `LastUsed`. Body: `{"id": "wh_kp_guard_nas", "name": "KP-Guard NAS", "target": "120363404628369352@g.us", "source": "Synology NAS DS923+"}`. |

---

## 5. Invarian Kritis & Catatan Arsitektur (Anti-Bug Guard)

### ⚠️ 1. Deadlock Prevention pada Golang `sync.RWMutex`
> [!CAUTION]
> Di dalam `src/features/features.go`, fungsi `save()` menggunakan `m.mu.RLock()` untuk men-serialize JSON.  
> **DILARANG KERAS** memanggil `m.save()` saat write lock `m.mu.Lock()` masih aktif!  
> Melakukan hal tersebut akan memicu **recursive locking deadlock** di Go runtime yang melumpuhkan seluruh goroutine web server.  
> **Pola yang Benar**: Selalu lepas write lock dengan `m.mu.Unlock()` sebelum memanggil `m.save()`.

### ⚠️ 2. Hot-Swapping Nomor WhatsApp Tanpa Restart
- Di masa lalu, jika bot melakukan logout dan pengguna memindai nomor baru, auto-responder custom sempat berhenti merespons karena handler menangkap instance client WhatsApp yang lama (stale pointer).
- `commands.go` menggunakan fungsi getter dinamis:
  ```go
  client := getClient()
  if client == nil || !client.IsConnected() {
      return
  }
  ```
  Ini menjamin auto-responder selalu merujuk ke koneksi WhatsApp yang aktif.

### ⚠️ 3. Normalisasi JID WhatsApp Penerima
- Library `whatsmeow` mensyaratkan tipe JID terstruktur (`types.NewJID(phone, types.DefaultUserServer)`).
- Mengirim pesan langsung ke string nomor mentah (misal: `6282218542122`) akan gagal dengan error `can't send message to unknown server`.
- Gunakan selalu helper `src/lib/jid.go` (`ParseJID` / `NormalizeTarget`) untuk mengonversi string ke JID resmi sebelum dispatching ke WhatsApp socket.

### ⚠️ 4. Proteksi Spamming KP-Guard NAS (Thermal Soak Debouncing)
- Integrasi pelindung server NAS Synology (`/usr/local/bin/kp-load-guard.sh`) diatur dengan prinsip:
  1. **Zero Alert pada Beban CPU Biasa**: Lonjakan CPU usage % tidak pernah mengirim WhatsApp alert.
  2. **Wajib Panas Berkelanjutan (Debounce Window)**: Suhu CPU harus bertahan >=86°C selama minimal 32 detik berturut-turut sebelum alert dikirim.
  3. **Anti-Flapping Cooldown**: Jeda antar alert minimal 15 menit (900s) dengan masa tenang 5 menit (300s) pasca recovery.

---

## 6. Prosedur Operasional (Runbook & Kompilasi)

### 6.1 Kompilasi Binary Go
Kompilasi dieksekusi di Minibox2 atau workstation Linux x86_64:
```bash
# Pindah ke direktori proyek
cd /volume1/web/Nouvem/Golang-Whatsapp-Webhook

# Build binary mandiri tanpa VCS dirty warning
go build -buildvcs=false -o gowa-nouvem .

# Pastikan izin eksekusi aktif
chmod +x gowa-nouvem
```

### 6.2 Manajemen Servis PM2 (di Minibox2 `192.168.100.186`)
```bash
# Cek status proses
pm2 status nouvem-wa-webhook

# Restart servis setelah compile ulang binary
pm2 restart nouvem-wa-webhook

# Pantau log interaksi WhatsApp real-time
pm2 logs nouvem-wa-webhook --lines 50

# Reload bersih jika file sesi database direset
pm2 restart nouvem-wa-webhook --update-env
```

### 6.3 Prosedur Ganti / Pairing Nomor WhatsApp Baru
1. Buka browser ke `http://192.168.100.186:18080/` (atau `https://kroomhook.kroombox.com`).
2. Jika masih terhubung ke nomor lama, klik tombol **Logout WhatsApp** di kartu status.
3. Setelah status berubah menjadi *Unlinked / Standby*, klik tombol **Generate QR Code**.
4. Di aplikasi WhatsApp HP: buka **Perangkat Tertaut (Linked Devices)** -> **Tautkan Perangkat** -> scan QR code di layar.
5. Status otomatis berubah menjadi **Connected** warna hijau. Bot siap seketika tanpa perlu restart PM2.

### 6.4 Uji Coba Cepat via CLI / cURL
```bash
# Test ping webhook ke grup KroomBox
curl -X POST http://192.168.100.186:18080/api/webhooks/test \
  -H "Content-Type: application/json" \
  -d '{"id":"wh_kp_guard_nas","name":"KP-Guard NAS","target":"120363404628369352@g.us","source":"CLI Manual Test"}'

# Kirim pesan kustom via endpoint universal /notify
curl -X POST http://192.168.100.186:18080/notify \
  -H "Content-Type: application/json" \
  -d '{"to":"120363404628369352@g.us","message":"Halo dari testing cURL BotVista!","source":"Terminal"}'
```
