package notify

import (
	"context"
	"os"
	"strings"

	"go.mau.fi/whatsmeow"
)

var Addr = getAddr()

func getAddr() string {
	if addr := os.Getenv("ADDR"); addr != "" {
		return addr
	}
	if port := os.Getenv("PORT"); port != "" {
		if !strings.Contains(port, ":") {
			return "127.0.0.1:" + port
		}
		return port
	}
	return "127.0.0.1:18080"
}

type Request struct {
	To      string `json:"to"`
	Message string `json:"message"`
}

// Start is retained for backwards compatibility.
// The unified server in src/web handles /notify.
func Start(ctx context.Context, client *whatsmeow.Client) {
}
