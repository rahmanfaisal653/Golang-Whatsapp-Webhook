# Webhook Whatsapp Kroombox

Minimal WhatsApp notifier using Go + whatsmeow.

Purpose:

```text
WhatsApp bot runs locally/server-side
local HTTP API receives notify requests
bot sends message to WhatsApp private/group chat
```

No config system. No dashboard. No public webhook framework. Just a tiny notifier.

## Features

- WhatsApp QR login / saved session
- Local notifier API: `POST /notify`
- WA command `.test` calls local API and sends test notification
- Utility commands: `.ping`, `.about`, `.menu`, `.uptime`, `.id`, `.groups`

## Project structure

```text
main.go                    app boot, WA connect, command registration
src/notify/listen.go        local HTTP notifier server
src/lib/commands.go         WhatsApp command router
src/lib/jid.go              shared JID helpers
commands/owner/ping.go      .ping command
commands/owner/basic.go     .about .menu .uptime .id
commands/owner/groups.go    .groups command
commands/owner/test.go      .test command → POST localhost /notify
src/session/                WhatsApp session DB, ignored by git
```

## Run

```bash
go run .
```

First run prints QR. Scan from WhatsApp:

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
http://127.0.0.1:18080/notify
```

Method:

```text
POST
```

Header:

```text
Content-Type: application/json
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
curl -X POST http://127.0.0.1:18080/notify \
  -H 'Content-Type: application/json' \
  -d '{"to":"120363xxxxx@g.us","message":"notify applied"}'
```

### Postman example

```text
Method: POST
URL: http://127.0.0.1:18080/notify
Headers:
  Content-Type: application/json
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
.test    call localhost /notify and send "notify applied" to current chat
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
→ POST http://127.0.0.1:18080/notify
→ body uses current chat JID
→ notifier sends "notify applied" back to that chat
```

This proves:

```text
WA command router works
local API works
notifier can send WA message
```

## Verification

```bash
gofmt -w main.go commands/owner/*.go src/notify/*.go
go test ./...
go build ./...
```

## Security notes

- `src/session/` contains WhatsApp credentials. Do not commit/share it.
- Notifier binds to `127.0.0.1`, local machine only.
- Do not expose `/notify` publicly without adding auth.
- This uses an unofficial WhatsApp library; avoid spammy sending.
# Golang-Whatsapp-Webhook
