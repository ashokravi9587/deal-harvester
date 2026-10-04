package config

import (
	"github.com/ashokravi9587/deal-harvester/internal/deals"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfiguration(t *testing.T) {
	env := map[string]string{"SERPAPI_KEY": "test", "SLACK_WEBHOOK_URL": "https://hooks.slack.com/services/test"}
	get := func(k string) string { return env[k] }
	c, err := Parse(get, false)
	if err != nil || c.MaxDeals != 10 || c.MinDiscount != 0 || len(c.Categories) != 6 {
		t.Fatalf("defaults %+v %v", c, err)
	}
	for key, value := range map[string]string{"MIN_RATING": "NaN", "MIN_DISCOUNT": "101", "MAX_DEALS": "0", "ALLOWED_RETAILERS": "Amazon fake", "SCORE_VALUE_WEIGHT": "-1", "SEARCH_CATEGORIES": ", ,"} {
		env[key] = value
		if _, err := Parse(get, false); err == nil {
			t.Fatalf("accepted invalid %s", key)
		}
		delete(env, key)
	}
	delete(env, "SLACK_WEBHOOK_URL")
	if _, err := Parse(get, true); err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(get, false); err == nil {
		t.Fatal("missing webhook accepted")
	}
	delete(env, "SERPAPI_KEY")
	if _, err := Parse(get, true); err == nil {
		t.Fatal("missing key accepted")
	}
}

func TestEnvFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	content := "# local settings\nSERPAPI_KEY=local-key\nSLACK_WEBHOOK_URL=https://hooks.slack.com/services/test\nSEARCH_LOCATION=\"San Jose, California, United States\"\nMAX_DEALS=3\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	absent := func(string) (string, bool) { return "", false }
	c, err := loadFile(path, absent, false)
	if err != nil || c.SerpAPIKey != "local-key" || c.MaxDeals != 3 || c.Location != "San Jose, California, United States" {
		t.Fatalf("dotenv config mismatch: %v", err)
	}
	override := func(key string) (string, bool) {
		if key == "MAX_DEALS" {
			return "8", true
		}
		return "", false
	}
	c, err = loadFile(path, override, false)
	if err != nil || c.MaxDeals != 8 {
		t.Fatal("environment did not override file")
	}
	empty := func(key string) (string, bool) {
		if key == "SERPAPI_KEY" {
			return "", true
		}
		return "", false
	}
	if _, err := loadFile(path, empty, true); err == nil {
		t.Fatal("explicit empty environment should override file")
	}
	missing := func(key string) (string, bool) {
		if key == "SERPAPI_KEY" {
			return "shell-key", true
		}
		return "", false
	}
	if _, err := loadFile(filepath.Join(t.TempDir(), "missing"), missing, true); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("BAD!KEY=private-secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadFile(path, absent, true); err == nil || strings.Contains(err.Error(), "private-secret") {
		t.Fatal("malformed file must fail without exposing values")
	}
}

func TestRequestedProfiles(t *testing.T) {
	data, err := os.ReadFile("../../search-profiles.json")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "profiles.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{"SERPAPI_KEY": "test", "SEARCH_PROFILES_FILE": path}
	c, err := Parse(func(k string) string { return env[k] }, true)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		profile int
		title   string
		price   float64
		want    bool
	}{
		{0, `Brand 65-inch 4K Smart TV`, 199.99, true},
		{0, `Brand 65" UHD Television`, 180, true},
		{0, `Brand 65-inch 4K TV`, 200, false},
		{0, `Brand 55-inch 4K TV`, 150, false},
		{0, `Brand 65-inch HD TV`, 150, false},
		{0, `65-inch 4K TV Wall Mount`, 20, false},
		{1, `Raised Garden Bed`, 80, true}, {1, `Raised Beds`, 80, true}, {1, `Railing planter`, 30, true}, {1, `Hanging plant pots`, 30, true}, {1, `Potting soil`, 30, true}, {1, `Cooking pots`, 30, false},
		{2, `Halloween ghost decorations`, 19.99, true}, {2, `Halloween ghost decorations`, 20, false}, {2, `Halloween costume`, 10, false},
		{2, `Spooky LED String Lights`, 12, true},
		{2, `Halloween Table Runner`, 15, true},
		{2, `Spooky LED String Lights`, 20, false},
		{3, `Holiday Table Runner`, 15, true},
		{3, `Christmas candy`, 10, false},
		{3, `Christmas tree ornaments`, 19.99, true}, {3, `Christmas tree ornaments`, 20, false}, {3, `Kitchen towels`, 10, true},
	}
	for _, tc := range cases {
		if got := c.Profiles[tc.profile].Matches(deals.Product{Name: tc.title, Price: tc.price}); got != tc.want {
			t.Errorf("%s price %.2f got %v want %v", tc.title, tc.price, got, tc.want)
		}
	}
	env["SEARCH_PROFILES_FILE"] = path + "missing"
	if _, err := Parse(func(k string) string { return env[k] }, true); err == nil {
		t.Fatal("missing profile file accepted")
	}
	if err := (Profile{Name: "bad", Queries: []string{"test"}, RequiredPatterns: []string{"["}}).Validate(); err == nil {
		t.Fatal("invalid pattern accepted")
	}
}
