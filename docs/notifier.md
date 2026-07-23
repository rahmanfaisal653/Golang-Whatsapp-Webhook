# Notifier API

Local HTTP API for sending WhatsApp messages through the running bot.

## Endpoint

```text
POST http://127.0.0.1:18080/notify
```

## Request

```json
{
  "to": "120363xxxxx@g.us",
  "message": "notify applied"
}
```

Fields:

```text
to       WhatsApp target JID
message  text sent to WhatsApp
```

Target can be:

```text
628xxx@s.whatsapp.net   private chat
120363xxxxx@g.us        group chat
```

## Response

Success:

```text
notify applied
```

Errors:

```text
405 method not allowed
400 invalid json
400 invalid notify request
502 send whatsapp: ...
```

## curl

```bash
curl -X POST http://127.0.0.1:18080/notify \
  -H 'Content-Type: application/json' \
  -d '{"to":"120363xxxxx@g.us","message":"notify applied"}'
```

## Postman

```text
Method: POST
URL: http://127.0.0.1:18080/notify
Headers:
  Content-Type: application/json
Body:
  raw → JSON
```

Body:

```json
{
  "to": "120363xxxxx@g.us",
  "message": "notify applied"
}
```

## Test from WhatsApp

Send to bot:

```text
.test
```

Expected:

```text
notify applied
```

## Notes

- API is local-only: `127.0.0.1`.
- Bot must be connected to WhatsApp.
- Bot must be inside target group before sending to group JID.
