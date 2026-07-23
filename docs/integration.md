# Integration Guide

Use GoWA when another app needs to send WhatsApp notifications.

```text
your app → POST https://kroomhook.kroombox.com/notify → GoWA → WhatsApp message
```

Your app does not connect to WhatsApp directly. It only sends JSON to the GoWA endpoint.

## Public endpoint

```text
POST https://kroomhook.kroombox.com/notify
```

Headers:

```text
Content-Type: application/json
```

Body:

```json
{
  "to": "120363xxxxx@g.us",
  "message": "Hello from my app"
}
```

Success response:

```text
notify applied
```

## Get target JID

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

## Postman test

Method:

```text
POST
```

URL:

```text
https://kroomhook.kroombox.com/notify
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

Expected:

```text
notify applied
```

## curl example

```bash
curl -X POST https://kroomhook.kroombox.com/notify \
  -H 'Content-Type: application/json' \
  -d '{"to":"120363xxxxx@g.us","message":"Hello from curl"}'
```

## JavaScript / Node example

```js
const res = await fetch("https://kroomhook.kroombox.com/notify", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({
    to: "120363xxxxx@g.us",
    message: "Hello from Node"
  })
});

console.log(await res.text());
```

Ticket form example:

```js
app.post("/tickets", async (req, res) => {
  const { name, email, subject, message } = req.body;

  await fetch("https://kroomhook.kroombox.com/notify", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      to: "120363xxxxx@g.us",
      message: `🎫 New Ticket
Name: ${name}
Email: ${email}
Subject: ${subject}
Message: ${message}`
    })
  });

  res.send("ticket created");
});
```

## PHP example

```php
<?php
$payload = json_encode([
    "to" => "120363xxxxx@g.us",
    "message" => "Hello from PHP"
]);

$ch = curl_init("https://kroomhook.kroombox.com/notify");
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

    res, err := http.Post("https://kroomhook.kroombox.com/notify", "application/json", bytes.NewReader(payload))
    if err != nil {
        panic(err)
    }
    defer res.Body.Close()

    fmt.Println(res.Status)
}
```

## Common errors

### File not found

Cause:

```text
request reached web server, but not GoWA /notify route
```

Usually:

```text
subdomain points to wrong server
Nginx vhost not loaded
HTTPS config handled by another server block
wrong endpoint URL
```

Fix:

```text
check Cloudflare DNS target
check Nginx server_name
check /notify proxy config
reload Nginx
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

## Security note

Current public endpoint has no auth.

```text
Anyone who knows the URL can send WhatsApp messages through the bot.
```

Use only for trusted testing. Add auth before serious/public use.
