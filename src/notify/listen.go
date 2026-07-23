package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

const Addr = "127.0.0.1:18080"

type Request struct {
	To      string `json:"to"`
	Message string `json:"message"`
}

func Start(ctx context.Context, client *whatsmeow.Client) {
	mux := http.NewServeMux()
	mux.HandleFunc("/notify", func(w http.ResponseWriter, r *http.Request) {
		handle(w, r, client)
	})
	server := &http.Server{Addr: Addr, Handler: mux}
	go func() {
		<-ctx.Done()
		_ = server.Shutdown(context.Background())
	}()
	log.Printf("notifier listening on http://%s/notify", Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("notifier: %v", err)
	}
}

func handle(w http.ResponseWriter, r *http.Request, client *whatsmeow.Client) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req Request
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	jid, err := types.ParseJID(req.To)
	if err != nil || jid.IsEmpty() || req.Message == "" {
		http.Error(w, "invalid notify request", http.StatusBadRequest)
		return
	}
	if _, err := client.SendMessage(r.Context(), jid.ToNonAD(), &waE2E.Message{Conversation: proto.String(req.Message)}); err != nil {
		http.Error(w, fmt.Sprintf("send whatsapp: %v", err), http.StatusBadGateway)
		return
	}
	fmt.Fprint(w, "notify applied")
}
