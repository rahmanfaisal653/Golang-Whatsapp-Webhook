package web

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"

	"github.com/local/whatsmeow-base/src/lib"
)

// SendPayload defines a unified, rich payload supporting text, media, location, contact, and status.
type SendPayload struct {
	Type         string  `json:"type,omitempty"`          // "text", "image", "document", "video", "audio", "location", "contact", "status"
	To           string  `json:"to"`                      // recipient number/group JID or "status@broadcast"
	Message      string  `json:"message,omitempty"`       // text message or caption
	MediaURL     string  `json:"media_url,omitempty"`     // remote URL or local path
	MediaData    string  `json:"media_data,omitempty"`    // base64 data URI or raw base64 string
	FileName     string  `json:"file_name,omitempty"`     // document filename
	MimeType     string  `json:"mime_type,omitempty"`     // optional override
	Latitude     float64 `json:"latitude,omitempty"`      // for location
	Longitude    float64 `json:"longitude,omitempty"`     // for location
	LocationName string  `json:"location_name,omitempty"` // venue name
	Address      string  `json:"address,omitempty"`       // venue address
	ContactName  string  `json:"contact_name,omitempty"`  // vcard contact name
	ContactPhone string  `json:"contact_phone,omitempty"` // vcard contact phone
	ScheduleAt   string  `json:"schedule_at,omitempty"`   // optional ISO8601 or YYYY-MM-DD HH:MM
	Source       string  `json:"source,omitempty"`
}

// SendRequest is a backward-compatible alias of SendPayload
type SendRequest = SendPayload

// fetchMediaBytes extracts binary data and content type from base64 data URI or URL/file path.
func fetchMediaBytes(mediaURL, mediaData, userMime string) ([]byte, string, error) {
	// 1. Base64 payload (direct upload from browser)
	if strings.TrimSpace(mediaData) != "" {
		data := strings.TrimSpace(mediaData)
		mime := userMime
		if strings.HasPrefix(data, "data:") {
			parts := strings.SplitN(data, ",", 2)
			if len(parts) == 2 {
				header := parts[0]
				data = parts[1]
				if mime == "" && strings.Contains(header, ";") {
					mimeParts := strings.Split(strings.TrimPrefix(header, "data:"), ";")
					if len(mimeParts) > 0 {
						mime = strings.TrimSpace(mimeParts[0])
					}
				}
			}
		}
		decoded, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			return nil, "", fmt.Errorf("decode base64 media: %w", err)
		}
		if mime == "" {
			mime = http.DetectContentType(decoded)
		}
		return decoded, mime, nil
	}

	// 2. Remote URL or Local File
	urlStr := strings.TrimSpace(mediaURL)
	if urlStr == "" {
		return nil, "", errors.New("neither media_url nor media_data provided")
	}

	if strings.HasPrefix(urlStr, "http://") || strings.HasPrefix(urlStr, "https://") {
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Get(urlStr)
		if err != nil {
			return nil, "", fmt.Errorf("download media from URL: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, "", fmt.Errorf("download media failed with HTTP status %d", resp.StatusCode)
		}

		// Cap maximum media download to 50MB to protect memory
		bytes, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024))
		if err != nil {
			return nil, "", fmt.Errorf("read media content: %w", err)
		}

		mime := userMime
		if mime == "" {
			mime = resp.Header.Get("Content-Type")
		}
		if mime == "" || strings.HasPrefix(mime, "application/octet-stream") {
			mime = http.DetectContentType(bytes)
		}
		return bytes, mime, nil
	}

	// 3. Local filesystem path
	bytes, err := os.ReadFile(urlStr)
	if err != nil {
		return nil, "", fmt.Errorf("read local media file '%s': %w", urlStr, err)
	}
	mime := userMime
	if mime == "" {
		mime = http.DetectContentType(bytes)
	}
	return bytes, mime, nil
}

// BuildMessage constructs a whatsmeow protobuf message based on SendPayload.
func (m *WhatsAppManager) BuildMessage(ctx context.Context, p SendPayload) (*waE2E.Message, error) {
	msgType := strings.ToLower(strings.TrimSpace(p.Type))
	if msgType == "" {
		msgType = "text"
	}

	msg := &waE2E.Message{}

	switch msgType {
	case "text":
		text := strings.TrimSpace(p.Message)
		if text == "" {
			return nil, errors.New("text message cannot be empty")
		}
		msg.Conversation = proto.String(text)

	case "location":
		if p.Latitude == 0 && p.Longitude == 0 {
			return nil, errors.New("latitude and longitude required for location message")
		}
		loc := &waE2E.LocationMessage{
			DegreesLatitude:  proto.Float64(p.Latitude),
			DegreesLongitude: proto.Float64(p.Longitude),
		}
		if p.LocationName != "" {
			loc.Name = proto.String(p.LocationName)
		}
		if p.Address != "" {
			loc.Address = proto.String(p.Address)
		}
		msg.LocationMessage = loc

	case "contact":
		name := strings.TrimSpace(p.ContactName)
		phone := strings.TrimSpace(p.ContactPhone)
		if name == "" || phone == "" {
			return nil, errors.New("contact_name and contact_phone required for contact message")
		}
		cleanPhone := lib.CleanPhoneNumber(phone)
		vcard := fmt.Sprintf("BEGIN:VCARD\nVERSION:3.0\nFN:%s\nTEL;type=CELL;type=VOICE;waid=%s:+%s\nEND:VCARD", name, cleanPhone, cleanPhone)
		msg.ContactMessage = &waE2E.ContactMessage{
			DisplayName: proto.String(name),
			Vcard:       proto.String(vcard),
		}

	case "status":
		// WhatsApp status update (StatusBroadcast)
		if strings.TrimSpace(p.MediaURL) != "" || strings.TrimSpace(p.MediaData) != "" {
			bytes, mime, err := fetchMediaBytes(p.MediaURL, p.MediaData, p.MimeType)
			if err != nil {
				return nil, err
			}
			client := m.GetClient()
			if client == nil {
				return nil, errors.New("whatsapp client not connected")
			}
			isVid := strings.HasPrefix(mime, "video/")
			mediaType := whatsmeow.MediaImage
			if isVid {
				mediaType = whatsmeow.MediaVideo
			}
			uploaded, err := client.Upload(ctx, bytes, mediaType)
			if err != nil {
				return nil, fmt.Errorf("upload status media: %w", err)
			}
			if isVid {
				msg.VideoMessage = &waE2E.VideoMessage{
					URL:           proto.String(uploaded.URL),
					DirectPath:    proto.String(uploaded.DirectPath),
					MediaKey:      uploaded.MediaKey,
					FileEncSHA256: uploaded.FileEncSHA256,
					FileSHA256:    uploaded.FileSHA256,
					FileLength:    proto.Uint64(uploaded.FileLength),
					Mimetype:      proto.String(mime),
					Caption:       proto.String(p.Message),
				}
			} else {
				msg.ImageMessage = &waE2E.ImageMessage{
					URL:           proto.String(uploaded.URL),
					DirectPath:    proto.String(uploaded.DirectPath),
					MediaKey:      uploaded.MediaKey,
					FileEncSHA256: uploaded.FileEncSHA256,
					FileSHA256:    uploaded.FileSHA256,
					FileLength:    proto.Uint64(uploaded.FileLength),
					Mimetype:      proto.String(mime),
					Caption:       proto.String(p.Message),
				}
			}
		} else {
			text := strings.TrimSpace(p.Message)
			if text == "" {
				return nil, errors.New("status text cannot be empty")
			}
			msg.ExtendedTextMessage = &waE2E.ExtendedTextMessage{
				Text: proto.String(text),
			}
		}

	case "image", "document", "video", "audio":
		bytes, mime, err := fetchMediaBytes(p.MediaURL, p.MediaData, p.MimeType)
		if err != nil {
			return nil, err
		}
		client := m.GetClient()
		if client == nil {
			return nil, errors.New("whatsapp client not connected")
		}

		var waType whatsmeow.MediaType
		switch msgType {
		case "image":
			waType = whatsmeow.MediaImage
			if !strings.HasPrefix(mime, "image/") {
				mime = "image/jpeg"
			}
		case "document":
			waType = whatsmeow.MediaDocument
		case "video":
			waType = whatsmeow.MediaVideo
			if !strings.HasPrefix(mime, "video/") {
				mime = "video/mp4"
			}
		case "audio":
			waType = whatsmeow.MediaAudio
			if !strings.HasPrefix(mime, "audio/") {
				mime = "audio/ogg; codecs=opus"
			}
		}

		uploaded, err := client.Upload(ctx, bytes, waType)
		if err != nil {
			return nil, fmt.Errorf("upload media to whatsapp: %w", err)
		}

		switch msgType {
		case "image":
			msg.ImageMessage = &waE2E.ImageMessage{
				URL:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uploaded.FileLength),
				Mimetype:      proto.String(mime),
				Caption:       proto.String(p.Message),
			}
		case "document":
			fileName := strings.TrimSpace(p.FileName)
			if fileName == "" {
				fileName = "document"
			}
			msg.DocumentMessage = &waE2E.DocumentMessage{
				URL:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uploaded.FileLength),
				Mimetype:      proto.String(mime),
				Title:         proto.String(fileName),
				FileName:      proto.String(fileName),
				Caption:       proto.String(p.Message),
			}
		case "video":
			msg.VideoMessage = &waE2E.VideoMessage{
				URL:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uploaded.FileLength),
				Mimetype:      proto.String(mime),
				Caption:       proto.String(p.Message),
			}
		case "audio":
			msg.AudioMessage = &waE2E.AudioMessage{
				URL:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uploaded.FileLength),
				Mimetype:      proto.String(mime),
				PTT:           proto.Bool(true),
			}
		}

	default:
		return nil, fmt.Errorf("unsupported message type '%s'", msgType)
	}

	return msg, nil
}
