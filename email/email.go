package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// Message is a single outbound email. HTML is the rich body; Text is the
// plain-text fallback sent alongside it.
type Message struct {
	To      string
	Subject string
	HTML    string
	Text    string
}

// Sender delivers an email.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// New returns a Postmark sender when a token is set, else a Console sender that
// logs the message (so local dev needs no email account).
func New(token, from string) Sender {
	if token == "" {
		return Console{Log: log.Default()}
	}
	return Postmark{Token: token, From: from, BaseURL: "https://api.postmarkapp.com", Client: http.DefaultClient}
}

// Console logs emails instead of sending them.
type Console struct{ Log *log.Logger }

func (c Console) Send(ctx context.Context, msg Message) error {
	c.Log.Printf("[email] to=%s subject=%q\n%s", msg.To, msg.Subject, msg.Text)
	return nil
}

// Postmark sends via the Postmark REST API.
type Postmark struct {
	Token   string
	From    string
	BaseURL string
	Client  *http.Client
}

func (p Postmark) Send(ctx context.Context, msg Message) error {
	payload, _ := json.Marshal(map[string]string{
		"From":          p.From,
		"To":            msg.To,
		"Subject":       msg.Subject,
		"HtmlBody":      msg.HTML,
		"TextBody":      msg.Text,
		"MessageStream": "outbound",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+"/email", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Postmark-Server-Token", p.Token)

	resp, err := p.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("postmark: status %d", resp.StatusCode)
	}
	return nil
}
