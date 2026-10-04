package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/ashokravi9587/deal-harvester/internal/deals"
)

type Slack struct {
	Client  *http.Client
	Webhook string
}
type Block struct {
	Type string `json:"type"`
	Text Text   `json:"text"`
}
type Text struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	Verbatim bool   `json:"verbatim,omitempty"`
}
type Payload struct {
	Text        string  `json:"text"`
	Blocks      []Block `json:"blocks"`
	UnfurlLinks bool    `json:"unfurl_links"`
	UnfurlMedia bool    `json:"unfurl_media"`
}

func escape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}
func clip(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
func Format(products []deals.Product) Payload {
	p := Payload{Text: fmt.Sprintf("Deal Harvester: today's best deals (%d)", len(products))}
	p.Blocks = append(p.Blocks, Block{Type: "header", Text: Text{Type: "plain_text", Text: "Today's Best Deals — Deal Harvester"}})
	if len(products) == 0 {
		p.Blocks = append(p.Blocks, Block{Type: "section", Text: Text{Type: "mrkdwn", Text: "No deals met the configured thresholds today.", Verbatim: true}})
	}
	for _, d := range products {
		discount := "Discount unavailable"
		if d.OldPrice > d.Price {
			discount = fmt.Sprintf("Was $%.2f · %.0f%% off", d.OldPrice, d.Discount())
		}
		rating := "Rating unavailable"
		if d.Rating > 0 {
			rating = fmt.Sprintf("Rating %.1f · %d reviews", d.Rating, d.Reviews)
		}
		text := fmt.Sprintf("*%s — $%.2f*\n%s · %s\n%s", escape(clip(d.Name, 300)), d.Price, escape(d.Retailer), discount, rating)
		if d.Delivery != "" {
			text += "\n" + escape(clip(d.Delivery, 200))
		}
		if deals.SafeURL(d.URL) && len(d.URL) < 1000 {
			text += "\n<" + escape(d.URL) + "|View Deal>"
		}
		p.Blocks = append(p.Blocks, Block{Type: "section", Text: Text{Type: "mrkdwn", Text: text, Verbatim: true}})
	}
	return p
}
func (s *Slack) Send(ctx context.Context, products []deals.Product) error {
	body, err := json.Marshal(Format(products))
	if err != nil {
		return fmt.Errorf("cannot encode Slack digest")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.Webhook, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("cannot create Slack request")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.Client.Do(req)
	if err != nil {
		return fmt.Errorf("Slack delivery failed (transport or timeout)")
	}
	defer resp.Body.Close()
	response, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if resp.StatusCode != http.StatusOK {
		code := strings.TrimSpace(string(response))
		switch code {
		case "invalid_payload", "invalid_blocks", "no_text", "too_many_attachments", "action_prohibited", "channel_not_found", "channel_is_archived", "invalid_token", "no_service", "no_service_id", "no_team", "team_disabled", "posting_to_general_channel_denied", "user_not_found", "rollup_error":
			return fmt.Errorf("Slack delivery failed: HTTP %d (%s)", resp.StatusCode, code)
		default:
			return fmt.Errorf("Slack delivery failed: HTTP %d (unrecognized response)", resp.StatusCode)
		}
	}
	if err != nil || strings.TrimSpace(string(response)) != "ok" {
		return fmt.Errorf("Slack did not acknowledge the digest")
	}
	return nil
}
