package app

import (
	"context"
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
