package notification

import (
	"context"
	"encoding/json"
	"github.com/ashokravi9587/deal-harvester/internal/deals"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSlack(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p Payload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			t.Error(err)
		}
		if r.Method != "POST" || r.Header.Get("Content-Type") != "application/json" || len(p.Blocks) != 2 || !strings.Contains(p.Blocks[1].Text.Text, "50% off") {
			t.Errorf("unexpected payload %+v", p)
		}
		w.Write([]byte("ok"))
	}))
	defer server.Close()
	s := Slack{Client: server.Client(), Webhook: server.URL}
	p := deals.Product{Name: "A & B <!here>", Retailer: "Target", Price: 20, OldPrice: 40, Rating: 4.7, URL: "https://example.com/p"}
	if err := s.Send(context.Background(), []deals.Product{p}); err != nil {
		t.Fatal(err)
	}
	text := Format([]deals.Product{p}).Blocks[1].Text.Text
	if strings.Contains(text, "<!here>") || !strings.Contains(text, "&amp;") {
		t.Fatal("unsafe escaping")
	}
	if !strings.Contains(Format(nil).Blocks[1].Text.Text, "No deals") {
		t.Fatal("empty digest")
	}
}
func TestSlackFailure(t *testing.T) {
	for _, status := range []int{200, 403, 429} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			w.Write([]byte("private-webhook-secret"))
		}))
		s := Slack{Client: server.Client(), Webhook: server.URL}
		err := s.Send(context.Background(), nil)
		server.Close()
		if err == nil || strings.Contains(err.Error(), "private-webhook-secret") {
			t.Fatalf("unsafe error %v", err)
		}
	}
}

func TestHeaderOmitsMarkdownOnlyFields(t *testing.T) {
	for _, products := range [][]deals.Product{nil, {{Name: "TV", Price: 100, Retailer: "Target"}}} {
		body, err := json.Marshal(Format(products))
		if err != nil {
			t.Fatal(err)
		}
		var payload struct {
			Blocks []struct {
				Text map[string]interface{} `json:"text"`
			} `json:"blocks"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatal(err)
		}
		header := payload.Blocks[0].Text
		if header["type"] != "plain_text" {
			t.Fatal("header must be plain text")
		}
		if _, present := header["verbatim"]; present {
			t.Fatal("plain text must omit verbatim")
		}
		if payload.Blocks[1].Text["verbatim"] != true {
			t.Fatal("Markdown section should retain verbatim")
		}
	}
}

func TestSlackKnownError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid_payload"))
	}))
	defer server.Close()
	s := Slack{Client: server.Client(), Webhook: server.URL}
	err := s.Send(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "invalid_payload") {
		t.Fatalf("missing diagnostic: %v", err)
	}
}
