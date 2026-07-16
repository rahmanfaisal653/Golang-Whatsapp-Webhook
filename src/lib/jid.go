package lib

import "go.mau.fi/whatsmeow/types"

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
