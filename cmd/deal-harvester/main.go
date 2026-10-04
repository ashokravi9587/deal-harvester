package main

import (
	"context"
	"encoding/json"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/ashokravi9587/deal-harvester/internal/app"
	"github.com/ashokravi9587/deal-harvester/internal/config"
	"github.com/ashokravi9587/deal-harvester/internal/deals"
	"github.com/ashokravi9587/deal-harvester/internal/notification"
	"github.com/ashokravi9587/deal-harvester/internal/search"
)

type preview struct{}

func (preview) DeliveryMode() string { return "preview" }

func (preview) Send(_ context.Context, p []deals.Product) error {
	return json.NewEncoder(os.Stdout).Encode(notification.Format(p))
}
func main() {
	dry := flag.Bool("dry-run", false, "search and print Slack payload without sending it")
	slackTest := flag.Bool("slack-test", false, "send a sample Slack digest without searching SerpAPI")
	flag.Parse()
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	if *slackTest {
		if *dry {
			logger.Error("use either -slack-test or -dry-run")
			os.Exit(1)
		}
		webhook, err := config.LoadSlackWebhook()
		if err != nil {
			logger.Error("Slack configuration failed", "error", err)
			os.Exit(1)
		}
		client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		notifier := &notification.Slack{Client: client, Webhook: webhook}
		sample := []deals.Product{{Name: "TEST ONLY: Sample deal — Slack integration works", Retailer: "Target", Price: 9.99, OldPrice: 19.99, Rating: 4.5, Reviews: 100, URL: "https://example.com", Delivery: "Sample data; this is not a real offer."}}
		if err := notifier.Send(ctx, sample); err != nil {
			logger.Error("Slack test failed", "error", err)
			os.Exit(1)
		}
		logger.Info("Slack test delivered; no SerpAPI searches performed")
		return
	}
	c, err := config.Load(*dry)
	if err != nil {
		logger.Error("configuration failed", "error", err)
		os.Exit(1)
	}
	client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(signalCtx, 5*time.Minute)
	defer cancel()
	var notifier app.Notifier = &notification.Slack{Client: client, Webhook: c.SlackWebhook}
	if *dry {
		notifier = preview{}
		logger.Info("dry run enabled; Slack delivery disabled")
	}
	err = app.Run(ctx, c, &search.SerpAPI{Client: client, APIKey: c.SerpAPIKey, Location: c.Location}, notifier, logger)
	if err != nil {
		logger.Error("run failed", "error", err)
		os.Exit(1)
	}
}
