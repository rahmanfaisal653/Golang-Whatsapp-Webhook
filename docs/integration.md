# Integration Guide

This guide is for beginners who want another app to send WhatsApp messages through GoWA.

## What GoWA does

GoWA is a small bridge:

```text
your app → HTTP POST /notify → GoWA → WhatsApp message
```

Your app does not talk to WhatsApp directly. Your app only sends JSON to GoWA.

## Requirements

- GoWA is running.
- GoWA is connected to WhatsApp.
- Your app runs on the same machine as GoWA.
- You know the target WhatsApp JID.

Why same machine?

```text
GoWA listens on 127.0.0.1:18080
```

`127.0.0.1` means local machine only.

## Step 1 — Run GoWA

In the GoWA project folder:

```bash
go run .
```

Wait until you see:

```text
Connected. Press CTRL+C to stop.
```

Keep this terminal open.

## Step 2 — Get target JID

Private chat:

```text
send .id to bot
```

Use:

```text
Sender: 628xxx@s.whatsapp.net
```

Group chat:

```text
send .groups to bot
```

Use:

```text
120363xxxxx@g.us
```

The bot must already be inside that group.

## Step 3 — Test with Postman

Method:

```text
POST
```

URL:

```text
http://127.0.0.1:18080/notify
```

Headers:

```text
Content-Type: application/json
```

Body → raw → JSON:

```json
{
  "to": "120363xxxxx@g.us",
  "message": "Hello from Postman"
}
```

Expected response:

```text
notify applied
```

Expected WhatsApp message:

```text
Hello from Postman
```

## Step 4 — Test with curl

```bash
curl -X POST http://127.0.0.1:18080/notify \
  -H 'Content-Type: application/json' \
  -d '{"to":"120363xxxxx@g.us","message":"Hello from curl"}'
```

## JavaScript / Node example

```js
const res = await fetch("http://127.0.0.1:18080/notify", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({
    to: "120363xxxxx@g.us",
    message: "Hello from Node"
  })
});

console.log(await res.text());
```

## PHP example

```php
<?php
$payload = json_encode([
    "to" => "120363xxxxx@g.us",
    "message" => "Hello from PHP"
]);

$ch = curl_init("http://127.0.0.1:18080/notify");
curl_setopt_array($ch, [
    CURLOPT_POST => true,
    CURLOPT_HTTPHEADER => ["Content-Type: application/json"],
    CURLOPT_POSTFIELDS => $payload,
    CURLOPT_RETURNTRANSFER => true,
]);

$response = curl_exec($ch);
curl_close($ch);

echo $response;
```

## Go example

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

func main() {
    payload, _ := json.Marshal(map[string]string{
        "to": "120363xxxxx@g.us",
        "message": "Hello from Go",
    })

    res, err := http.Post("http://127.0.0.1:18080/notify", "application/json", bytes.NewReader(payload))
    if err != nil {
        panic(err)
    }
    defer res.Body.Close()

    fmt.Println(res.Status)
}
```

## Common errors

### Connection refused

```text
Failed to connect to 127.0.0.1 port 18080
```

Cause:

```text
GoWA is not running
```

Fix:

```bash
go run .
```

### Invalid notify request

Cause:

```text
missing to
missing message
bad JID format
```

Correct body:

```json
{
  "to": "120363xxxxx@g.us",
  "message": "Hello"
}
```

### send whatsapp: failed to get group members

Cause:

```text
wrong group JID
bot is not inside the group
```

Fix:

```text
invite bot to group
send .groups
copy exact JID
```

### Only works on my laptop, not friend's website

Cause:

```text
127.0.0.1 is local-only
```

If another server must call GoWA, GoWA must run on that server too, or the API must be exposed safely with auth/HTTPS later.

## Simple integration rule

If your app can send this HTTP request:

```json
{
  "to": "target jid",
  "message": "text to send"
}
```

Then your app can send WhatsApp notifications through GoWA.
