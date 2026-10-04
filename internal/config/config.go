package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/ashokravi9587/deal-harvester/internal/deals"
	"github.com/joho/godotenv"
)

type Config struct {
	SerpAPIKey, SlackWebhook, Location string
	Categories, Retailers              []string
	MaxDeals                           int
	MinRating, MinDiscount             float64
	Weights                            deals.Weights
	Profiles                           []Profile
}

// Load reads .env from the current working directory without changing the process
// environment. Explicit environment variables, even empty ones, take precedence.
func Load(dryRun bool) (Config, error) {
	return loadFile(".env", os.LookupEnv, dryRun)
}

func loadFile(path string, lookup func(string) (string, bool), dryRun bool) (Config, error) {
	values, err := godotenv.Read(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		// Parser errors may contain secret values, so never include the raw error.
		return Config{}, fmt.Errorf("cannot read .env file: check permissions and KEY=value syntax")
	}
	return Parse(func(key string) string {
		if value, present := lookup(key); present {
			return value
		}
		return values[key]
	}, dryRun)
}
func Parse(get func(string) string, dryRun bool) (Config, error) {
	c := Config{SerpAPIKey: strings.TrimSpace(get("SERPAPI_KEY")), SlackWebhook: strings.TrimSpace(get("SLACK_WEBHOOK_URL")), Location: "San Jose, California, United States", MaxDeals: 10, MinRating: 4, MinDiscount: 0, Weights: deals.DefaultWeights()}
	if c.SerpAPIKey == "" {
		return c, fmt.Errorf("SERPAPI_KEY is required")
	}
	if !dryRun {
		u, err := url.Parse(c.SlackWebhook)
		if err != nil || u.Scheme != "https" || (u.Host != "hooks.slack.com" && u.Host != "hooks.slack-gov.com") || !strings.HasPrefix(u.Path, "/services/") || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
			return c, fmt.Errorf("SLACK_WEBHOOK_URL must be a Slack HTTPS incoming webhook")
		}
	}
	list := func(key, fallback string) []string {
		value := get(key)
		if value == "" {
			value = fallback
		}
		var out []string
		for _, v := range strings.Split(value, ",") {
			if v = strings.TrimSpace(v); v != "" {
				out = append(out, v)
			}
		}
		return out
	}
	c.Categories = list("SEARCH_CATEGORIES", "electronics,home,kitchen,toys,tools,outdoor")
	c.Retailers = list("ALLOWED_RETAILERS", "Amazon,Walmart,Target")
	if len(c.Categories) == 0 || len(c.Retailers) == 0 {
		return c, fmt.Errorf("categories and retailers must not be empty")
	}
	for _, r := range c.Retailers {
		if deals.CanonicalRetailer(r) == "" {
			return c, fmt.Errorf("ALLOWED_RETAILERS contains an unsupported retailer")
		}
	}
	if v := get("SEARCH_LOCATION"); v != "" {
		c.Location = v
	}
	if v := get("MAX_DEALS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 40 {
			return c, fmt.Errorf("MAX_DEALS must be between 1 and 40")
		}
		c.MaxDeals = n
	}
	fields := []struct {
		key    string
		target *float64
		max    float64
	}{
		{"MIN_RATING", &c.MinRating, 5}, {"MIN_DISCOUNT", &c.MinDiscount, 100},
		{"SCORE_DISCOUNT_WEIGHT", &c.Weights.Discount, 1000}, {"SCORE_RATING_WEIGHT", &c.Weights.Rating, 1000}, {"SCORE_REVIEW_WEIGHT", &c.Weights.Reviews, 1000}, {"SCORE_VALUE_WEIGHT", &c.Weights.Value, 1000},
	}
	for _, f := range fields {
		if v := get(f.key); v != "" {
			n, err := strconv.ParseFloat(v, 64)
			if err != nil || math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n > f.max {
				return c, fmt.Errorf("%s must be between 0 and %g", f.key, f.max)
			}
			*f.target = n
		}
	}
	if path := strings.TrimSpace(get("SEARCH_PROFILES_FILE")); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return c, fmt.Errorf("cannot read SEARCH_PROFILES_FILE")
		}
		if err := json.Unmarshal(data, &c.Profiles); err != nil || len(c.Profiles) == 0 {
			return c, fmt.Errorf("SEARCH_PROFILES_FILE must contain a nonempty JSON profile array")
		}
		for i := range c.Profiles {
			if err := c.Profiles[i].Validate(); err != nil {
				return c, fmt.Errorf("invalid search profile %d: %w", i+1, err)
			}
		}
	}
	return c, nil
}

// Profile applies category-specific constraints in addition to global thresholds.
type Profile struct {
	Name             string   `json:"name"`
	Queries          []string `json:"queries"`
	MaxPrice         float64  `json:"max_price"`
	RequiredPatterns []string `json:"required_patterns"`
	ExcludePatterns  []string `json:"exclude_patterns"`
}

func (p Profile) Validate() error {
	if strings.TrimSpace(p.Name) == "" || len(p.Queries) == 0 {
		return fmt.Errorf("name and queries are required")
	}
	for _, q := range p.Queries {
		if strings.TrimSpace(q) == "" {
			return fmt.Errorf("queries must not be empty")
		}
	}
	if p.MaxPrice < 0 || math.IsNaN(p.MaxPrice) || math.IsInf(p.MaxPrice, 0) {
		return fmt.Errorf("max_price must be finite and nonnegative")
	}
	for _, pattern := range append(append([]string{}, p.RequiredPatterns...), p.ExcludePatterns...) {
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("invalid title pattern")
		}
	}
	return nil
}
func (p Profile) Matches(product deals.Product) bool { return p.RejectionReason(product) == "" }
func (p Profile) RejectionReason(product deals.Product) string {
	if p.MaxPrice > 0 && product.Price >= p.MaxPrice {
		return "price_cap"
	}
	for _, pattern := range p.RequiredPatterns {
		if matched, _ := regexp.MatchString(pattern, product.Name); !matched {
			return "required_title"
		}
	}
	for _, pattern := range p.ExcludePatterns {
		if matched, _ := regexp.MatchString(pattern, product.Name); matched {
			return "excluded_title"
		}
	}
	return ""
}

// LoadSlackWebhook loads only Slack configuration; no search credentials or profiles are needed.
func LoadSlackWebhook() (string, error) {
	values, err := godotenv.Read(".env")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("cannot read .env file: check permissions and KEY=value syntax")
	}
	webhook, present := os.LookupEnv("SLACK_WEBHOOK_URL")
	if !present {
		webhook = values["SLACK_WEBHOOK_URL"]
	}
	c, err := Parse(func(key string) string {
		switch key {
		case "SERPAPI_KEY":
			return "unused-for-slack-test"
		case "SLACK_WEBHOOK_URL":
			return webhook
		default:
			return ""
		}
	}, false)
	return c.SlackWebhook, err
}
