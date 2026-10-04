
# Webhook notifier Whatsapp

Minimal WhatsApp notifier using Go + whatsmeow.

Purpose:

```text
WhatsApp bot runs locally/server-side
local HTTP API receives notify requests
bot sends message to WhatsApp private/group chat
```

No config system. No build step. Just a tiny notifier with an embedded dashboard.

## Features

- WhatsApp QR login / saved session
- **Web UI**: public docs page (`/`) and admin dashboard (`/admin`)
- **Admin dashboard**: live status, QR login in the browser, API key management, send-test form
- Authenticated notifier API: `POST /notify` (requires an API key)
- Utility commands: `.ping`, `.about`, `.menu`, `.uptime`, `.id`, `.groups`, `.test`

## Project structure

```text
main.go                    app boot, WA connect, command registration
src/app/app.go             WhatsApp client wrapper + UI-facing state (QR, status)
src/web/server.go          HTTP server: docs, admin, /notify
src/web/handlers.go        HTTP handlers + /notify endpoint
src/web/auth.go            admin login session + API key store
src/web/docs.go            renders README/docs as HTML for the docs page
src/web/static/            embedded dashboard UI (app.html, style.css, icon.svg)
src/lib/commands.go         WhatsApp command router
src/lib/jid.go              shared JID helpers
commands/owner/*.go        .ping .about .menu .uptime .id .groups .test
src/session/                WhatsApp session DB + API keys, ignored by git
```

## Run

```bash
# Admin login for the dashboard (if unset, a random password is printed on start)
export ADMIN_PASSWORD='choose-a-strong-password'
# Optional: bind address (default 127.0.0.1:18080)
export LISTEN_ADDR='127.0.0.1:18080'
go run .
```

Then open:

```text
http://127.0.0.1:18080/         dashboard (login required)
http://127.0.0.1:18080/docs     public docs
```

On the admin page, if the bot is not linked yet a QR appears. Scan it from
WhatsApp:

```text
WhatsApp → Linked devices → Link a device
```

After login, session is saved in:

```text
src/session/whatsmeow.db
```

## Local notifier API

Server:

```text
https://kroomhook.kroombox.com/notify
```

Method:

```text
POST
```

Headers:

```text
Content-Type: application/json
X-API-Key: <key created on the admin dashboard>
```

Body:

```json
{
  "to": "120363xxxxx@g.us",
  "message": "notify applied"
}
```

Response on success:

```text
notify applied
```

### curl example

```bash
curl -X POST https://kroomhook.kroombox.com/notify \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: YOUR_KEY' \
  -d '{"to":"120363xxxxx@g.us","message":"notify applied"}'
```

### Postman example

```text
Method: POST
URL: https://kroomhook.kroombox.com/notify
Headers:
  Content-Type: application/json
  X-API-Key: YOUR_KEY
Body → raw → JSON:
{
  "to": "120363xxxxx@g.us",
  "message": "notify applied"
}
```

## WhatsApp commands

```text
.ping    check bot
.about   bot info
.menu    command list
.uptime  bot runtime
.id      show sender/chat JID
.groups  list joined WhatsApp groups + group JID
.test    send "notify applied" to the current chat (proves the send path)
```

## Getting target JID

Private chat:

```text
.id
```

Use:

```text
Sender: 628xxx@s.whatsapp.net
```

Group:

```text
.groups
```

Use group JID:

```text
120363xxxxx@g.us
```

Bot must be inside target group.

## How `.test` works

User sends:

```text
.test
```

Flow:

```text
.test command
→ notify.Send() with the current chat JID
→ notifier sends "notify applied" back to that chat
```

This proves:

```text
WA command router works
send path works
notifier can send a WA message
```

## More docs

```text
docs/getting-started.md   overview — how to use the webhook
docs/integration.md       integration examples (curl, JS, PHP, Go, Postman)
docs/notifier.md          API reference
```

## Verification

```bash
gofmt -w .
go test ./...
go build ./...
```

## Security notes

- `src/session/` contains WhatsApp credentials and API keys. Do not commit/share it.
- The server binds to `127.0.0.1` by default; expose it only behind a proxy/tunnel.
- `/notify` requires an `X-API-Key`; the dashboard requires the admin password.
- This uses an unofficial WhatsApp library; avoid spammy sending.
