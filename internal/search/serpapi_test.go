package search

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("engine") != "google_shopping" || q.Get("api_key") != "secret" || q.Get("q") != "kitchen deals" || q.Get("gl") != "us" {
			t.Errorf("unexpected parameters")
		}
		w.Write([]byte(`{"search_metadata":{"status":"Success"},"shopping_results":[{"product_id":"12","title":"Toaster","source":"Walmart","extracted_price":25,"extracted_old_price":50,"rating":4.5,"reviews":30,"product_link":"https://google.com/product/12","delivery":"Free shipping"}]}`))
	}))
	defer server.Close()
	s := SerpAPI{Client: server.Client(), APIKey: "secret", Location: "San Jose", Endpoint: server.URL}
	products, err := s.Search(context.Background(), "kitchen")
	if err != nil || len(products) != 1 {
		t.Fatalf("result %+v %v", products, err)
	}
	p := products[0]
	if p.Price != 25 || p.OldPrice != 50 || p.Reviews != 30 || p.URL != "https://google.com/product/12" || p.Delivery != "Free shipping" {
		t.Fatalf("parse %+v", p)
	}
}
func TestSearchErrorsRedactSecrets(t *testing.T) {
	for _, body := range []string{`{"error":"secret"}`, `{"search_metadata":{"status":"Processing"}}`, `not-json`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
		s := SerpAPI{Client: server.Client(), APIKey: "secret", Endpoint: server.URL}
		_, err := s.Search(context.Background(), "test")
		server.Close()
		if err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatalf("unsafe error %v", err)
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(429) }))
	defer server.Close()
	s := SerpAPI{Client: server.Client(), APIKey: "secret", Endpoint: server.URL}
	if _, err := s.Search(context.Background(), "test"); err == nil {
		t.Fatal("HTTP failure ignored")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Search(ctx, "test"); err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("transport error %v", err)
	}
}
