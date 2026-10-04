package deals

import (
	"math"
	"testing"
)

func product() Product {
	return Product{ID: "1", Name: "Headphones", Retailer: "Amazon", Price: 50, OldPrice: 100, Rating: 4.5, Reviews: 999, URL: "https://example.com/product"}
}
func TestDiscount(t *testing.T) {
	for _, c := range []struct{ price, old, want float64 }{{50, 100, 50}, {100, 100, 0}, {120, 100, 0}, {50, 0, 0}, {0, 100, 0}} {
		if got := (Product{Price: c.price, OldPrice: c.old}).Discount(); got != c.want {
			t.Fatalf("discount %v: %v", c, got)
		}
	}
}
func TestScore(t *testing.T) {
	p := product()
	want := 50 + 9 + 3 + 2*100.0/150
	if got := Score(p, DefaultWeights()); math.Abs(got-want) > 1e-9 {
		t.Fatalf("score %g want %g", got, want)
	}
	if Score(p, Weights{Discount: 2}) != 100 {
		t.Fatal("weights not applied")
	}
}
func TestFilter(t *testing.T) {
	good := product()
	good.Retailer = " amazon.com "
	for _, change := range []func(*Product){func(p *Product) { p.Retailer = "Amazon scam" }, func(p *Product) { p.Retailer = "Target" }, func(p *Product) { p.Rating = 3 }, func(p *Product) { p.OldPrice = 0 }, func(p *Product) { p.Price = 0 }, func(p *Product) { p.URL = "javascript:alert(1)" }, func(p *Product) { p.Name = " " }} {
		bad := product()
		change(&bad)
		got := Filter([]Product{good, bad}, []string{"Amazon"}, 4, 10, DefaultWeights())
		if len(got) != 1 || got[0].Retailer != "Amazon" {
			t.Fatalf("filter returned %+v", got)
		}
	}
	unknown := product()
	unknown.OldPrice = 0
	unknown.Rating = 0
	if len(Filter([]Product{unknown}, []string{"Amazon"}, 0, 0, DefaultWeights())) != 1 {
		t.Fatal("zero thresholds should permit missing metadata")
	}
}
func TestDedupeAndRank(t *testing.T) {
	a := product()
	a.Score = 30
	b := a
	b.Retailer = "Walmart"
	b.Price = 40
	b.Score = 40
	c := a
	c.ID = "2"
	c.Name = " HEAD-PHONES "
	c.Score = 20
	d := a
	d.ID = "3"
	d.Name = "Toaster"
	d.Score = 35
	got := Dedupe([]Product{a, c, d, b})
	if len(got) != 2 || got[0].Retailer != "Walmart" {
		t.Fatalf("dedupe %+v", got)
	}
	top := Rank(got, 1)
	if len(top) != 1 || top[0].Score != 40 {
		t.Fatalf("rank %+v", top)
	}
	// An alias bridge must merge two previously distinct groups.
	bridge := d
	bridge.Name = a.Name
	bridge.Score = 10
	if len(Dedupe([]Product{a, d, bridge})) != 1 {
		t.Fatal("alias bridge not merged")
	}
	if len(Rank(got, 0)) != 0 {
		t.Fatal("zero limit")
	}
}

func TestFilterReasonCounts(t *testing.T) {
	good := product()
	retailer := good
	retailer.Retailer = "Other"
	retailer.Price = 0
	invalid := good
	invalid.Price = 0
	rating := good
	rating.Rating = 1
	discount := good
	discount.OldPrice = 0
	out, stats := FilterWithStats([]Product{good, retailer, invalid, rating, discount}, []string{"Amazon"}, 4, 10, DefaultWeights())
	if len(out) != 1 || stats.Retailer != 1 || stats.InvalidProduct != 1 || stats.Rating != 1 || stats.Discount != 1 {
		t.Fatalf("counts: %+v retained %d", stats, len(out))
	}
}
