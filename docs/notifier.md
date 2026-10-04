# API Reference

The webhook exposes a single public endpoint for sending WhatsApp messages.

## Endpoint

```text
POST https://kroomhook.kroombox.com/notify
```

## Headers

```text
Content-Type: application/json
X-API-Key: YOUR_API_KEY
```

The `X-API-Key` header is required. You can also send the key as a bearer token:

```text
Authorization: Bearer YOUR_API_KEY
```

## Request body

```json
{
  "to": "628123456789@s.whatsapp.net",
  "message": "Hello from my app"
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `to` | string | yes | Target JID — a private number or a group ID |
| `message` | string | yes | Text to deliver |

### The `to` field

| Target | Value | Example |
|---|---|---|
| Private chat | `<number>@s.whatsapp.net` | `628123456789@s.whatsapp.net` |
| Group chat | `<group-id>@g.us` | `120363000000000000@g.us` |

For group targets the bot must already be a member of the group.

## Responses

| Status | Body | Meaning |
|---|---|---|
| `200` | `notify applied` | message delivered |
| `400` | `invalid json` | request body is not valid JSON |
| `400` | `invalid notify request` | `to` or `message` missing/empty, or bad JID |
| `401` | `unauthorized` | missing or invalid API key |
| `405` | `method not allowed` | endpoint called with a method other than `POST` |
| `502` | `send whatsapp: ...` | the bot could not deliver the message |

## Example

```bash
curl -X POST https://kroomhook.kroombox.com/notify \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: YOUR_API_KEY' \
  -d '{"to":"628123456789@s.whatsapp.net","message":"Hello"}'
```

```text
notify applied
```

## Notes

- Create and revoke API keys on the dashboard.
- Anyone with a valid API key can send messages through the bot — keep keys secret.
- The bot must be connected to WhatsApp for messages to be delivered.
- For group targets, the bot must be inside the group before sending.
