// Package notify defines the JSON payload used by the /notify endpoint and
// sends WhatsApp messages through the shared client.
package notify

import (
	"context"
	"errors"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// Request is the /notify JSON body.
type Request struct {
	To      string `json:"to"`
	Message string `json:"message"`
}

// Send delivers one text message to the JID in req.To.
func Send(ctx context.Context, client *whatsmeow.Client, req Request) error {
	jid, err := types.ParseJID(req.To)
	if err != nil || jid.IsEmpty() {
		return errors.New("invalid target jid")
	}
	if req.Message == "" {
		return errors.New("empty message")
	}
	if _, err := client.SendMessage(ctx, jid.ToNonAD(), &waE2E.Message{Conversation: proto.String(req.Message)}); err != nil {
		return err
	}
	return nil
}
