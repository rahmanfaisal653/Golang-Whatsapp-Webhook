# Overview

This webhook lets any application send WhatsApp messages through a connected
WhatsApp bot — you don't need to write any WhatsApp integration code.

```text
your app ──POST /notify──▶ this webhook ──▶ WhatsApp message
```

You make one HTTP request and the webhook delivers the message to a **private
chat** or a **group chat**.

## How to use it

### 1. Get an API key

Ask the operator for an API key, or create one yourself on the dashboard
(**API keys → Create key**). Every request must include it in the
`X-API-Key` header.

### 2. Choose a target

| Target | Format | Example |
|---|---|---|
| Private chat | phone number in international format | `628123456789` |
| Group chat | group ID ending in `@g.us` | `120363000000000000@g.us` |

For a group, the bot must already be a member of that group.

### 3. Send a request

```bash
curl -X POST https://kroomhook.kroombox.com/notify \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: YOUR_API_KEY' \
  -d '{"to":"628123456789@s.whatsapp.net","message":"Hello from my app"}'
```

On success you get:

```text
notify applied
```

## Next steps

- **Integration Guide** — step-by-step examples for curl, JavaScript, PHP, Go and Postman.
- **API Reference** — full endpoint, fields, responses and error codes.
