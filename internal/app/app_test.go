package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/ashokravi9587/deal-harvester/internal/config"
	"github.com/ashokravi9587/deal-harvester/internal/deals"
	"io"
	"log/slog"
	"testing"
)

type fakeSearch struct {
	fail  bool
	calls int
}

func (s *fakeSearch) Search(context.Context, string) ([]deals.Product, error) {
	s.calls++
	if s.fail {
		return nil, errors.New("search failed")
	}
	return []deals.Product{{ID: "1", Name: "Toaster", Retailer: "Target", Price: 25, OldPrice: 50, Rating: 4.5, URL: "https://example.com/p"}}, nil
}

type fakeNotify struct {
	calls    int
	products []deals.Product
	fail     bool
}

func (n *fakeNotify) Send(_ context.Context, p []deals.Product) error {
	n.calls++
	n.products = p
	if n.fail {
		return errors.New("delivery failed")
	}
	return nil
}
func TestRun(t *testing.T) {
	c := config.Config{Categories: []string{"home", "kitchen"}, Retailers: []string{"Target"}, MaxDeals: 10, Weights: deals.DefaultWeights()}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := &fakeSearch{}
	n := &fakeNotify{}
	if err := Run(context.Background(), c, s, n, logger); err != nil {
		t.Fatal(err)
	}
	if s.calls != 2 || n.calls != 1 || len(n.products) != 1 {
		t.Fatalf("workflow calls %d %d products %d", s.calls, n.calls, len(n.products))
	}
	s = &fakeSearch{fail: true}
	n = &fakeNotify{}
	if err := Run(context.Background(), c, s, n, logger); err == nil || n.calls != 0 {
		t.Fatal("search failure should prevent delivery")
	}
	n = &fakeNotify{fail: true}
	if err := Run(context.Background(), c, &fakeSearch{}, n, logger); err == nil {
		t.Fatal("delivery failure ignored")
	}
}

type previewNotify struct{ fakeNotify }

func (*previewNotify) DeliveryMode() string { return "preview" }
func TestRunSummaryCounts(t *testing.T) {
	for _, mode := range []string{"slack", "preview", "failed"} {
		var logs bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logs, nil))
		c := config.Config{Categories: []string{"a", "b"}, Retailers: []string{"Target"}, MaxDeals: 1, Weights: deals.DefaultWeights()}
		var n Notifier = &fakeNotify{fail: mode == "failed"}
		if mode == "preview" {
			n = &previewNotify{}
		}
		err := Run(context.Background(), c, &fakeSearch{}, n, logger)
		if (err != nil) != (mode == "failed") {
			t.Fatalf("unexpected error %v", err)
		}
		decoder := json.NewDecoder(&logs)
		var last map[string]any
		for decoder.More() {
			var row map[string]any
			if err := decoder.Decode(&row); err != nil {
				t.Fatal(err)
			}
			last = row
		}
		sent := float64(1)
		if mode != "slack" {
			sent = 0
		}
		if last["extracted"] != float64(2) || last["filtered_out"] != float64(0) || last["duplicates_removed"] != float64(1) || last["selected_for_digest"] != float64(1) || last["sent_to_digest"] != sent {
			t.Fatalf("summary %+v", last)
		}
	}
}
