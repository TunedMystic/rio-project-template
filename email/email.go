package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// Sender delivers a plain-text email.
type Sender interface {
	Send(ctx context.Context, to, subject, textBody string) error
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

func (c Console) Send(ctx context.Context, to, subject, textBody string) error {
	c.Log.Printf("[email] to=%s subject=%q\n%s", to, subject, textBody)
	return nil
}

// Postmark sends via the Postmark REST API.
type Postmark struct {
	Token   string
	From    string
	BaseURL string
	Client  *http.Client
}

func (p Postmark) Send(ctx context.Context, to, subject, textBody string) error {
	payload, _ := json.Marshal(map[string]string{
		"From":          p.From,
		"To":            to,
		"Subject":       subject,
		"TextBody":      textBody,
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
