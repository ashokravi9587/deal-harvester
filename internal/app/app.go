package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ashokravi9587/deal-harvester/internal/config"
	"github.com/ashokravi9587/deal-harvester/internal/deals"
)

type Searcher interface {
	Search(context.Context, string) ([]deals.Product, error)
}
type Notifier interface {
	Send(context.Context, []deals.Product) error
}

func Run(ctx context.Context, c config.Config, s Searcher, n Notifier, logger *slog.Logger) error {
	var retained []deals.Product
	extracted, searches := 0, 0
	deliveryMode := "slack"
	if mode, ok := n.(interface{ DeliveryMode() string }); ok {
		deliveryMode = mode.DeliveryMode()
	}
	profiles := c.Profiles
	if len(profiles) == 0 {
		for _, category := range c.Categories {
			profiles = append(profiles, config.Profile{Name: category, Queries: []string{category}})
		}
	}
	for _, profile := range profiles {
		for _, query := range profile.Queries {
			products, err := s.Search(ctx, query)
			if err != nil {
				return fmt.Errorf("search profile %q: %w", profile.Name, err)
			}
			extracted += len(products)
			searches++
			filtered, stats := deals.FilterWithStats(products, c.Retailers, c.MinRating, c.MinDiscount, c.Weights)
			reasons := map[string]int{"retailer": stats.Retailer, "invalid_product": stats.InvalidProduct, "rating": stats.Rating, "discount": stats.Discount, "price_cap": 0, "required_title": 0, "excluded_title": 0}
			var matched []deals.Product
			for _, product := range filtered {
				if reason := profile.RejectionReason(product); reason != "" {
					reasons[reason]++
				} else {
					matched = append(matched, product)
				}
			}
			logger.Info("search completed", "profile", profile.Name, "query", query, "extracted", len(products), "filtered_out", len(products)-len(matched), "retained", len(matched), "rejected_by_reason", reasons)
			retained = append(retained, matched...)
		}
	}
	unique := deals.Dedupe(retained)
	top := deals.Rank(unique, c.MaxDeals)
	counts := []any{"searches", searches, "extracted", extracted, "filtered_out", extracted - len(retained), "retained", len(retained), "duplicates_removed", len(retained) - len(unique), "unique_deals", len(unique), "omitted_by_limit", len(unique) - len(top), "selected_for_digest", len(top), "delivery_mode", deliveryMode}
	logger.Info("digest prepared", counts...)
	if err := n.Send(ctx, top); err != nil {
		fields := append(append([]any{}, counts...), "sent_to_digest", 0, "delivery_status", "failed", "error", err)
		logger.Error("digest delivery failed", fields...)
		return err
	}
	sent := len(top)
	status := "delivered"
	message := "digest delivered"
	if deliveryMode == "preview" {
		sent = 0
		status = "previewed"
		message = "digest previewed"
	}
	logger.Info(message, append(counts, "sent_to_digest", sent, "delivery_status", status)...)
	return nil
}
