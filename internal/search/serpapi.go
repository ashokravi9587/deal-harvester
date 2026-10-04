package search

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/ashokravi9587/deal-harvester/internal/deals"
)

type SerpAPI struct {
	Client                     *http.Client
	APIKey, Location, Endpoint string
}

func (s *SerpAPI) Search(ctx context.Context, category string) ([]deals.Product, error) {
	endpoint := s.Endpoint
	if endpoint == "" {
		endpoint = "https://serpapi.com/search.json"
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid search endpoint")
	}
	q := u.Query()
	for k, v := range map[string]string{"engine": "google_shopping", "api_key": s.APIKey, "q": category + " deals", "location": s.Location, "hl": "en", "gl": "us"} {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create search request")
	}
	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("SerpAPI request failed (transport or timeout)")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SerpAPI returned HTTP %d", resp.StatusCode)
	}
	var data struct {
		Error    string `json:"error"`
		Metadata struct {
			Status string `json:"status"`
		} `json:"search_metadata"`
		Results []struct {
			ID          string  `json:"product_id"`
			Title       string  `json:"title"`
			Source      string  `json:"source"`
			Price       float64 `json:"extracted_price"`
			OldPrice    float64 `json:"extracted_old_price"`
			Rating      float64 `json:"rating"`
			Reviews     int     `json:"reviews"`
			ProductLink string  `json:"product_link"`
			Link        string  `json:"link"`
			Delivery    string  `json:"delivery"`
		} `json:"shopping_results"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(nil, resp.Body, 8<<20)).Decode(&data); err != nil {
		return nil, fmt.Errorf("SerpAPI returned an invalid response")
	}
	if data.Error != "" || (data.Metadata.Status != "" && data.Metadata.Status != "Success") {
		return nil, fmt.Errorf("SerpAPI reported an unsuccessful search")
	}
	var out []deals.Product
	for _, r := range data.Results {
		link := r.Link
		if !deals.SafeURL(link) {
			link = r.ProductLink
		}
		out = append(out, deals.Product{ID: r.ID, Name: r.Title, Retailer: r.Source, Price: r.Price, OldPrice: r.OldPrice, Rating: r.Rating, Reviews: r.Reviews, URL: link, Delivery: r.Delivery})
	}
	return out, nil
}
