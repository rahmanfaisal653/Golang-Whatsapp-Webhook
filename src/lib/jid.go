package lib

import (
	"errors"
	"fmt"
	"strings"

	"go.mau.fi/whatsmeow/types"
)

// SenderJID resolves the sender to the canonical phone-number JID.
// When a message arrives via LID, Sender is the LID address and
// SenderAlt holds the real phone-number JID (e.g. 62851...@s.whatsapp.net).
// ToNonAD() strips device suffixes like :57.
func SenderJID(info types.MessageInfo) types.JID {
	if info.Sender.Server == "lid" && !info.SenderAlt.IsEmpty() {
		return info.SenderAlt.ToNonAD()
	}
	return info.Sender.ToNonAD()
}

// ChatJID resolves the chat to the canonical phone-number JID.
// When a direct-message chat is addressed via LID, Chat is the LID
// address and RecipientAlt holds the real phone-number JID.
func ChatJID(info types.MessageInfo) types.JID {
	if info.Chat.Server == "lid" && !info.RecipientAlt.IsEmpty() {
		return info.RecipientAlt.ToNonAD()
	}
	return info.Chat.ToNonAD()
}

// CleanPhoneNumber normalizes any phone number string to international digits without '+' or symbols.
// Example: "0822-1854-2122" -> "6282218542122", "+62 822..." -> "62822..."
func CleanPhoneNumber(raw string) string {
	raw = strings.TrimSpace(raw)
	var sb strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			sb.WriteRune(r)
		}
	}
	cleaned := sb.String()

	if strings.HasPrefix(cleaned, "0") && len(cleaned) > 1 {
		cleaned = "62" + cleaned[1:]
	} else if strings.HasPrefix(cleaned, "8") && len(cleaned) > 7 {
		cleaned = "62" + cleaned
	}
	return cleaned
}

// ParseRecipientJID converts any user input into a valid WhatsApp JID with appropriate server domain.
// Supports:
// - Group JID: "120363028292@g.us"
// - User JID: "6282218542122@s.whatsapp.net"
// - LID JID: "15736793755777@lid"
// - Raw Numbers: "6282218542122", "082218542122", "+62 822-1854-2122"
func ParseRecipientJID(target string) (types.JID, error) {
	trimmed := strings.TrimSpace(target)
	if trimmed == "" {
		return types.JID{}, errors.New("recipient target cannot be empty")
	}

	// If already has an explicit server (e.g. contains '@')
	if strings.Contains(trimmed, "@") {
		jid, err := types.ParseJID(trimmed)
		if err != nil {
			return types.JID{}, fmt.Errorf("invalid JID format: %w", err)
		}
		if jid.IsEmpty() || jid.Server == "" {
			return types.JID{}, fmt.Errorf("JID server domain is missing for '%s'", trimmed)
		}
		return jid.ToNonAD(), nil
	}

	// Plain phone number without domain
	num := CleanPhoneNumber(trimmed)
	if len(num) < 6 {
		return types.JID{}, fmt.Errorf("invalid phone number '%s': too short", target)
	}

	return types.NewJID(num, types.DefaultUserServer), nil
}
