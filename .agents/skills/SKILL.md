---
name: whatsmeow-base
description: |
  Conventions, architecture, and rules for the whatsmeow-base WhatsApp bot project.
  Use this skill when adding commands, modifying the handler, or changing project structure.
---

# Whatsmeow Base — Project Conventions

A modular WhatsApp bot built on [whatsmeow](https://github.com/tulir/whatsmeow). Go 1.25+.

## Project Layout

```
whatsmeow-base/
├── main.go                  # entrypoint: wires DB, client, handler, connects
├── main_test.go             # TestSQLiteURI
├── database_test.go         # TestSQLiteStoreOpensWithoutCGO
├── go.mod / go.sum          # module definition + checksums
├── .gitignore               # ignores src/session/ and binaries
├── commands/                # 📁 ALL commands live here
│   └── owner/               #     owner-gated commands (check sender JID)
│       └── ping.go          #     Command interface + PingCommand
└── src/
    ├── session/             # whatsmeow.db (gitignored, contains WA credentials)
    └── lib/
        └── commands.go      # CommandHandler registry (package lib)
```

## Golden Rules

### 1. Every Command Must Go Inside `commands/`

- **Do NOT** put command logic in `main.go`, `src/lib/commands.go`, or any other file.
- Each command lives in its own file inside `commands/` (or a subfolder like `commands/owner/`).
- The `commands/owner/` subfolder is for **owner-only** commands (check sender JID against a configured owner before executing).

### 2. Command Interface

All commands must implement this interface (defined in `commands/owner/ping.go`):

```go
type Command interface {
    Name() string                                                    // e.g. ".ping"
    Execute(ctx context.Context, client *whatsmeow.Client, msg *events.Message) error
}
```

### 3. Registration

- Commands are registered in `main.go` via `handler.Register(owner.SomeCommand{})`.
- Registration automatically logs `"<name> loaded"` to stdout (via `src/lib/commands.go`).
- After all registrations, `handler.LogSummary()` prints total count.

### 4. Handler Architecture

- `src/lib/commands.go` contains the `CommandHandler` struct (package `lib`).
- It matches incoming messages by **exact trimmed-lowercase name** against the registered command map.
- Only messages that match a command are logged; all other messages are silently ignored.

### 5. Database

- Session stored at `src/session/whatsmeow.db` (SQLite, no CGO via `modernc.org/sqlite`).
- URI includes pragmas: `busy_timeout(5000)`, `journal_mode(WAL)`, `foreign_keys(1)`.
- The `sqliteURI()` helper is in `main.go`.

### 6. Styling

- `gofmt` before commit: `gofmt -w main.go main_test.go`
- Run `go vet ./...` and `go test ./...` before committing.

## Adding a New Command

1. Create a file in `commands/` (or `commands/owner/` for owner-only):

```go
package owner

import (
    "context"
    "fmt"
    "go.mau.fi/whatsmeow"
    "go.mau.fi/whatsmeow/proto/waE2E"
    "go.mau.fi/whatsmeow/types/events"
    "google.golang.org/protobuf/proto"
)

type HelloCommand struct{}

func (HelloCommand) Name() string { return ".hello" }

func (HelloCommand) Execute(ctx context.Context, client *whatsmeow.Client, msg *events.Message) error {
    _, err := client.SendMessage(ctx, msg.Info.Chat, &waE2E.Message{Conversation: proto.String("world")})
    if err != nil {
        return fmt.Errorf("send hello: %w", err)
    }
    return nil
}
```

2. Register in `main.go`:

```go
handler.Register(owner.HelloCommand{})
```

That's it — the command is live. No other files need to change.
