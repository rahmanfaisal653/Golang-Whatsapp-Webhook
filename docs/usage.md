# How to use this webhook

Send WhatsApp messages from any app through this webhook — no WhatsApp
integration code needed.

```text
your app ──POST /notify──▶ this webhook ──▶ WhatsApp message
```

You make one HTTP request and the webhook delivers the message to a private
chat or a group chat.

## 1. Get an API key

Ask the operator for a key, or create one on the dashboard (**API keys →
Create key**). Send it in the `X-API-Key` header on every request.

## 2. Choose a target

| Target | Value | Example |
|---|---|---|
| Private chat | `<number>@s.whatsapp.net` | `628123456789@s.whatsapp.net` |
| Group chat | `<group-id>@g.us` | `120363000000000000@g.us` |

Phone numbers are in international format (no `+`, no spaces). For a group,
the bot must already be a member of that group.

## 3. Send a request

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

| Field | Type | Required | Description |
|---|---|---|---|
| `to` | string | yes | target JID — a private number or a group ID |
| `message` | string | yes | text to deliver |

On success you get:

```text
notify applied
```

## Examples

### curl

```bash
curl -X POST https://kroomhook.kroombox.com/notify \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: YOUR_API_KEY' \
  -d '{"to":"628123456789@s.whatsapp.net","message":"Hello from curl"}'
```

### JavaScript / Node

```js
const res = await fetch("https://kroomhook.kroombox.com/notify", {
  method: "POST",
  headers: {
    "Content-Type": "application/json",
    "X-API-Key": process.env.WEBHOOK_API_KEY
  },
  body: JSON.stringify({
    to: "628123456789@s.whatsapp.net",
    message: "Hello from Node"
  })
});

console.log(await res.text()); // notify applied
```

### PHP

```php
<?php
$payload = json_encode([
    "to" => "628123456789@s.whatsapp.net",
    "message" => "Hello from PHP"
]);

$ch = curl_init("https://kroomhook.kroombox.com/notify");
curl_setopt_array($ch, [
    CURLOPT_POST => true,
    CURLOPT_HTTPHEADER => [
        "Content-Type: application/json",
        "X-API-Key: " . getenv("WEBHOOK_API_KEY")
    ],
    CURLOPT_POSTFIELDS => $payload,
    CURLOPT_RETURNTRANSFER => true,
]);

$response = curl_exec($ch);
curl_close($ch);

echo $response; // notify applied
```

### Go

```go
payload, _ := json.Marshal(map[string]string{
    "to":      "628123456789@s.whatsapp.net",
    "message": "Hello from Go",
})

req, _ := http.NewRequest("POST", "https://kroomhook.kroombox.com/notify", bytes.NewReader(payload))
req.Header.Set("Content-Type", "application/json")
req.Header.Set("X-API-Key", os.Getenv("WEBHOOK_API_KEY"))

res, err := http.DefaultClient.Do(req)
if err != nil {
    panic(err)
}
defer res.Body.Close()
```

### Postman

1. Method: `POST`
2. URL: `https://kroomhook.kroombox.com/notify`
3. Headers: `Content-Type: application/json` and `X-API-Key: YOUR_API_KEY`
4. Body → raw → JSON (same JSON as above)

## Responses

| Status | Body | Meaning |
|---|---|---|
| `200` | `notify applied` | message delivered |
| `400` | `invalid json` | request body is not valid JSON |
| `400` | `invalid notify request` | `to` or `message` missing/empty, or bad JID |
| `401` | `unauthorized` | missing or invalid API key |
| `502` | `send whatsapp: ...` | the bot could not deliver the message |

## Notes

- Keep your API key secret — anyone with a valid key can send messages through
  the bot. Store it in an environment variable, never in client-side code.
- Revoke a leaked key on the dashboard and create a new one.
- The bot must be connected to WhatsApp for messages to be delivered.
- Do not send messages in a loop; WhatsApp may rate-limit or block the number.
