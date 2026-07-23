package owner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/local/whatsmeow-base/src/notify"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"
)

// Postman test:
// POST http://127.0.0.1:18080/notify
// Headers: Content-Type: application/json
// Body raw JSON:
// {"to":"120363xxxxx@g.us","message":"notify applied"}

type TestCommand struct{ publicCommand }

func (TestCommand) Name() string { return ".test" }

func (TestCommand) Execute(ctx context.Context, client *whatsmeow.Client, msg *events.Message) error {
	payload, err := json.Marshal(notify.Request{To: chatJID(msg.Info).String(), Message: "notify applied"})
	if err != nil {
		return err
	}
	res, err := http.Post("http://"+notify.Addr+"/notify", "application/json", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("call notifier: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("notifier status: %s", res.Status)
	}
	return nil
}
