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
			filtered := deals.Filter(products, c.Retailers, c.MinRating, c.MinDiscount, c.Weights)
			var matched []deals.Product
			for _, product := range filtered {
				if profile.Matches(product) {
					matched = append(matched, product)
				}
			}
			logger.Info("search completed", "profile", profile.Name, "query", query, "retrieved", len(products), "retained", len(matched))
			retained = append(retained, matched...)
		}
	}
	unique := deals.Dedupe(retained)
	logger.Info("deduplication completed", "removed", len(retained)-len(unique))
	top := deals.Rank(unique, c.MaxDeals)
	logger.Info("deals selected", "count", len(top))
	if err := n.Send(ctx, top); err != nil {
		return err
	}
	logger.Info("digest delivered", "count", len(top))
	return nil
}
