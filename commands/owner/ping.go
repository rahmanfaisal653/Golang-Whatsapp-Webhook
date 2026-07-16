package owner

import (
	"context"
	"fmt"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// Command represents a chat command.
type Command interface {
	Name() string
	OwnerOnly() bool       // if true, only the configured owner can run this
	AdminOnly() bool       // if true, only group admins can run this (owner always bypasses)
	OwnerOnlyMsg() string  // custom reject message when OwnerOnly blocks a non-owner; empty = default
	AdminOnlyMsg() string  // custom reject message when AdminOnly blocks a non-admin; empty = default
	Execute(ctx context.Context, client *whatsmeow.Client, msg *events.Message) error
}

// PingCommand replies "pong" to ".ping".
type PingCommand struct{}

func (PingCommand) Name() string        { return ".ping" }
func (PingCommand) OwnerOnly() bool     { return false }
func (PingCommand) AdminOnly() bool     { return false }
func (PingCommand) OwnerOnlyMsg() string { return "" }
func (PingCommand) AdminOnlyMsg() string { return "" }

func (PingCommand) Execute(ctx context.Context, client *whatsmeow.Client, msg *events.Message) error {
	_, err := client.SendMessage(ctx, msg.Info.Chat, &waE2E.Message{Conversation: proto.String("pong")})
	if err != nil {
		return fmt.Errorf("send pong: %w", err)
	}
	return nil
}
