# Webhook notifier Whatsapp

Minimal WhatsApp notifier using Go + whatsmeow.

```text
WhatsApp bot runs on a server
apps POST to /notify
bot sends the message to a WhatsApp private/group chat
```

## Features

- WhatsApp QR login / saved session
- Embedded dashboard (login required): live status, QR login, API key management, send-test form
- Public docs at `/docs` explaining how to use the webhook
- Authenticated notifier API: `POST /notify` (requires an `X-API-Key`)
- Utility commands: `.ping`, `.about`, `.menu`, `.uptime`, `.id`, `.groups`, `.test`

## Project structure

```text
main.go                    app boot, WA connect, command registration
src/app/app.go             WhatsApp client wrapper + UI-facing state (QR, status)
src/web/server.go          HTTP server: dashboard, docs, /notify
src/web/handlers.go        HTTP handlers
src/web/auth.go            admin login session + API key store
src/web/docs.go            renders docs/*.md as HTML
src/web/static/            embedded dashboard UI (app.html, style.css, icon.svg)
src/lib/                   WhatsApp command router + JID helpers
commands/owner/*.go        .ping .about .menu .uptime .id .groups .test
docs/usage.md              docs shown at /docs
src/session/               WhatsApp session DB + API keys, ignored by git
```

## Run

```bash
export ADMIN_EMAIL='admin@gmail.com'          # dashboard login email
export ADMIN_PASSWORD='choose-a-strong-password'
export LISTEN_ADDR='127.0.0.1:18080'          # optional, this is the default
go run .
```

Then open:

```text
http://127.0.0.1:18080/        dashboard (login required)
http://127.0.0.1:18080/docs    public docs
```

If the bot is not linked yet, the dashboard shows a QR. Scan it from
WhatsApp → Linked devices → Link a device. The session is saved in
`src/session/whatsmeow.db`.

## Notify API

```text
POST https://kroomhook.kroombox.com/notify
Content-Type: application/json
X-API-Key: YOUR_API_KEY
```

```json
{
  "to": "628123456789@s.whatsapp.net",
  "message": "Hello from my app"
}
```

`to` is a private number (`<number>@s.whatsapp.net`) or a group ID
(`<group-id>@g.us`). On success the response is `notify applied`.

See `docs/usage.md` (also at `/docs`) for full examples in curl, JavaScript,
PHP, Go and Postman.

## WhatsApp commands

```text
.ping    check bot
.about   bot info
.menu    command list
.uptime  bot runtime
.id      show sender/chat JID
.groups  list joined groups + group JID
.test    send "notify applied" to the current chat
```

## Verification

```bash
gofmt -w .
go vet ./...
go test ./...
go build ./...
```

## Security notes

- `src/session/` contains WhatsApp credentials and API keys. Do not commit or share it.
- The server binds to `127.0.0.1` by default; expose it only behind a proxy/tunnel.
- `/notify` requires an `X-API-Key`; the dashboard requires the admin login.
- This uses an unofficial WhatsApp library; avoid spammy sending.
