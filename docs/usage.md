Send WhatsApp messages from any app through this webhook — no WhatsApp
integration code needed.

```text
your app ──POST /notify──▶ this webhook ──▶ WhatsApp message
```

You make one HTTP request and the webhook delivers the message to a private
chat or a group chat. This guide has six parts:

1. Get an API key
2. Choose a target
3. Send a request
4. Code examples
5. Response reference
6. Notes

---

## 1. Get an API key

Every request must be authenticated with an API key.

- Ask the operator for a key, **or**
- Create one yourself on the dashboard: **API Keys → Create key**.

Send the key in the `X-API-Key` header on every request. Keep it secret — anyone
holding a valid key can send messages through the bot.

## 2. Choose a target

The `to` field is the destination. There are two kinds of target:

### 2.1 Private chat (one person)

| Field | Value |
|---|---|
| Format | `<number>@s.whatsapp.net` |
| Example | `628123456789@s.whatsapp.net` |

The number is in international format — no `+`, no spaces, no leading zero.

### 2.2 Group chat

| Field | Value |
|---|---|
| Format | `<group-id>@g.us` |
| Example | `120363000000000000@g.us` |

The bot must **already be a member** of the group.

## 3. Send a request

### 3.1 Endpoint and headers

```text
POST https://kroomhook.kroombox.com/notify
Content-Type: application/json
X-API-Key: YOUR_API_KEY
```

### 3.2 Request body

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

### 3.3 Success response

```text
notify applied
```

## 4. Code examples

### 4.1 curl

```bash
curl -X POST https://kroomhook.kroombox.com/notify \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: YOUR_API_KEY' \
  -d '{"to":"628123456789@s.whatsapp.net","message":"Hello from curl"}'
```

### 4.2 JavaScript / Node

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

### 4.3 PHP

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

### 4.4 Go

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

### 4.5 Postman

1. Method: `POST`
2. URL: `https://kroomhook.kroombox.com/notify`
3. Headers: `Content-Type: application/json` and `X-API-Key: YOUR_API_KEY`
4. Body → raw → JSON (the same JSON as in section 3.2)

## 5. Response reference

| Status | Body | Meaning |
|---|---|---|
| `200` | `notify applied` | message delivered |
| `400` | `invalid json` | request body is not valid JSON |
| `400` | `invalid notify request` | `to` or `message` missing/empty, or bad JID |
| `401` | `unauthorized` | missing or invalid API key |
| `502` | `send whatsapp: ...` | the bot could not deliver the message |

## 6. Notes

- Keep your API key secret. Store it in an environment variable, never in
  client-side code.
- Revoke a leaked key on the dashboard and create a new one.
- The bot must be connected to WhatsApp for messages to be delivered.
- Do not send messages in a loop; WhatsApp may rate-limit or block the number.
