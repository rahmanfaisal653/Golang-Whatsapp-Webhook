# Integration Guide

This guide shows how to send WhatsApp messages from your application through
this webhook. Pick the example for your language, replace `YOUR_API_KEY` with
your real key and adjust the target and message.

## Before you start

You need:

- An **API key** — create one on the dashboard (**API keys → Create key**).
- A **target** — a phone number (private chat) or a group ID (group chat).

## Target format

| Target | What to use | Example |
|---|---|---|
| Private chat | phone number, international format (no `+`, no spaces) | `628123456789` |
| Group chat | group ID ending in `@g.us` | `120363000000000000@g.us` |

Private numbers are sent to `<number>@s.whatsapp.net`. For groups, the bot must
already be a member of the group.

## Endpoint

```text
POST https://kroomhook.kroombox.com/notify
```

Headers:

```text
Content-Type: application/json
X-API-Key: YOUR_API_KEY
```

Body:

```json
{
  "to": "628123456789@s.whatsapp.net",
  "message": "Hello from my app"
}
```

Success response:

```text
notify applied
```

## curl

```bash
curl -X POST https://kroomhook.kroombox.com/notify \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: YOUR_API_KEY' \
  -d '{"to":"628123456789@s.whatsapp.net","message":"Hello from curl"}'
```

Send to a group:

```bash
curl -X POST https://kroomhook.kroombox.com/notify \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: YOUR_API_KEY' \
  -d '{"to":"120363000000000000@g.us","message":"Hello group"}'
```

## JavaScript / Node

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

Real-world example — notify on a new ticket:

```js
app.post("/tickets", async (req, res) => {
  const { name, email, subject, message } = req.body;

  await fetch("https://kroomhook.kroombox.com/notify", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "X-API-Key": process.env.WEBHOOK_API_KEY
    },
    body: JSON.stringify({
      to: "628123456789@s.whatsapp.net",
      message: `🎫 New Ticket\nName: ${name}\nEmail: ${email}\nSubject: ${subject}\nMessage: ${message}`
    })
  });

  res.send("ticket created");
});
```

## PHP

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

## Go

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "os"
)

func main() {
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

    fmt.Println(res.Status) // 200 OK
}
```

## Postman

1. Method: `POST`
2. URL: `https://kroomhook.kroombox.com/notify`
3. Headers:
   - `Content-Type: application/json`
   - `X-API-Key: YOUR_API_KEY`
4. Body → raw → JSON:

```json
{
  "to": "628123456789@s.whatsapp.net",
  "message": "Hello from Postman"
}
```

Expected response: `notify applied`

## Common errors

| Response | Meaning | Fix |
|---|---|---|
| `401 unauthorized` | missing or wrong API key | check the `X-API-Key` header |
| `400 invalid json` | body is not valid JSON | validate the JSON body |
| `400 invalid notify request` | `to` or `message` is missing/empty | include both fields |
| `502 send whatsapp: ...` | the bot could not deliver | check the target format, and that the bot is in the group |

## Tips

- Keep your API key secret — anyone with a valid key can send messages through
  the bot. Store it in an environment variable, never in client-side code.
- Revoke a leaked key on the dashboard and create a new one.
- Do not send messages in a loop; WhatsApp may rate-limit or block the number.
