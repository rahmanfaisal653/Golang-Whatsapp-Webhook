# Whatsmeow Go Base

Minimal WhatsApp bot base using [whatsmeow](https://github.com/tulir/whatsmeow).

## Requirements

- Go 1.25+ (upstream currently specifies Go 1.25)
- A WhatsApp account to link
- Terminal access for the QR code

## Run

```powershell
go run .
```

First run prints a QR code. Open WhatsApp, go to **Linked devices**, then scan it. The local session persists in `data/whatsmeow.db`; it is excluded from Git.

Send `.ping` from another account/chat. The bot replies `pong`.

## Reset session

Stop the process. Delete `data/whatsmeow.db` plus any `-wal` and `-shm` files. Run again to link a new device.

## Verify

```powershell
gofmt -w main.go main_test.go
go test ./...
go vet ./...
go build ./...
```

## Security

The SQLite database contains WhatsApp device credentials. Do not commit, share, or serve `data/`. This base intentionally has no arbitrary shell execution, runtime JavaScript evaluation, HTTP server, secrets, or external database.

`ponytail:` commands use one pure matcher; replace it with a registry only when the command surface grows.