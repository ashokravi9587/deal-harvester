package deals

import (
	"math"
	"net/url"
	"sort"
	"strings"
	"unicode"
)

type Product struct {
	ID       string
	Name     string
	Retailer string
	Price    float64
	OldPrice float64
	Rating   float64
	Reviews  int
	URL      string
	Delivery string
	Score    float64
}

type Weights struct{ Discount, Rating, Reviews, Value float64 }

func DefaultWeights() Weights { return Weights{1, 2, 1, 2} }
func (p Product) Discount() float64 {
	if p.Price <= 0 || p.OldPrice <= p.Price {
		return 0
	}
	return 100 * (p.OldPrice - p.Price) / p.OldPrice
}
func Score(p Product, w Weights) float64 {
	if p.Price <= 0 {
		return 0
	}
	return p.Discount()*w.Discount + p.Rating*w.Rating + math.Log10(float64(max(p.Reviews, 0))+1)*w.Reviews + (100/(100+p.Price))*w.Value
}

// Retailer names are matched exactly after normalization, never by substring.
func CanonicalRetailer(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "amazon", "amazon.com":
		return "Amazon"
	case "walmart", "walmart.com":
		return "Walmart"
	case "target", "target.com":
		return "Target"
	}
	return ""
}
func SafeURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Hostname() != "" && u.User == nil && !strings.ContainsAny(s, "<>|\r\n")
}
func Filter(products []Product, allowed []string, minRating, minDiscount float64, w Weights) []Product {
	retailers := map[string]bool{}
	for _, r := range allowed {
		retailers[CanonicalRetailer(r)] = true
	}
	var out []Product
	for _, p := range products {
		p.Name = strings.Join(strings.Fields(p.Name), " ")
		p.Retailer = CanonicalRetailer(p.Retailer)
		if p.Retailer == "" || !retailers[p.Retailer] || p.Name == "" || p.Price <= 0 || math.IsNaN(p.Price) || math.IsInf(p.Price, 0) || !SafeURL(p.URL) || p.Rating < minRating || p.Discount() < minDiscount {
			continue
		}
		p.Score = Score(p, w)
		out = append(out, p)
	}
	return out
}
func normalizedName(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, s)
}
func better(a, b Product) bool {
	if a.Score != b.Score {
		return a.Score > b.Score
	}
	if a.Price != b.Price {
		return a.Price < b.Price
	}
	if a.Name != b.Name {
		return a.Name < b.Name
	}
	if a.Retailer != b.Retailer {
		return a.Retailer < b.Retailer
	}
	return a.URL < b.URL
}

// ID and title aliases catch repeated products across categories and retailers.
// Rank first so the best offer survives each group.
func Dedupe(products []Product) []Product {
	ranked := Rank(products, len(products))
	parent := make([]int, len(ranked))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		if parent[i] != i {
			parent[i] = find(parent[i])
		}
		return parent[i]
	}
	aliases := map[string]int{}
	for i, p := range ranked {
		keys := []string{"name:" + normalizedName(p.Name)}
		if p.ID != "" {
			keys = append(keys, "id:"+p.ID)
		}
		for _, key := range keys {
			if prev, ok := aliases[key]; ok {
				a, b := find(i), find(prev)
				if a > b {
					parent[a] = b
				} else {
					parent[b] = a
				}
			} else {
				aliases[key] = i
			}
		}
	}
	var out []Product
	for i, p := range ranked {
		if find(i) == i {
			out = append(out, p)
		}
	}
	return out
}
func Rank(products []Product, limit int) []Product {
	out := append([]Product(nil), products...)
	sort.SliceStable(out, func(i, j int) bool { return better(out[i], out[j]) })
	if limit < 0 {
		limit = 0
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}
