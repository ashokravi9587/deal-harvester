# Deal Harvester

A lightweight Go application that searches SerpAPI Google Shopping, ranks Amazon/Walmart/Target offers, and sends one daily Slack digest. No database, UI, direct retailer scraping, and uses godotenv for local configuration.

## Local setup

Requires Go 1.24 or newer. Create a [SerpAPI key](https://serpapi.com/manage-api-key) and a [Slack incoming webhook](https://docs.slack.dev/messaging/sending-messages-using-incoming-webhooks/): create a Slack app in your workspace, enable Incoming Webhooks, and authorize a channel.

In PowerShell, from this repository:

```powershell
Copy-Item .env.example .env
# Edit .env and fill in SERPAPI_KEY and SLACK_WEBHOOK_URL
go run ./cmd/deal-harvester -dry-run
go run ./cmd/deal-harvester
```

Dry run calls SerpAPI (and consumes search credits) but only prints the Slack JSON. It does not require a webhook. The application automatically reads `.env` from the current working directory. Run commands from the repository root. The local `.env` is ignored by Git; `.env.example` contains the shareable defaults. Existing shell environment variables take precedence, including explicitly empty values. A missing `.env` is allowed (for example in GitHub Actions); an unreadable or malformed file fails with a sanitized error. Quoted values, comments, and `export KEY=value` syntax are supported. To use updated file values after setting variables in PowerShell, remove the overrides with `Remove-Item Env:SERPAPI_KEY, Env:SLACK_WEBHOOK_URL -ErrorAction SilentlyContinue`. Never commit secrets or paste them into logs.

## Configuration

| Variable | Default | Meaning |
| --- | --- | --- |
| `SERPAPI_KEY` | required | SerpAPI secret |
| `SLACK_WEBHOOK_URL` | required except dry run | Slack incoming webhook secret |
| `SEARCH_CATEGORIES` | electronics,home,kitchen,toys,tools,outdoor | Comma-separated searches; each adds `deals` |
| `MAX_DEALS` | 10 | Digest size, 1–40 |
| `ALLOWED_RETAILERS` | Amazon,Walmart,Target | Subset of supported retailers |
| `MIN_RATING` | 4 | Minimum rating, 0–5; 0 permits missing ratings |
| `MIN_DISCOUNT` | 0 | Minimum percentage, 0–100; 0 permits unknown discounts |
| `SEARCH_LOCATION` | San Jose, California, United States | Shopping location |
| `SCORE_DISCOUNT_WEIGHT` | 1 | Discount contribution |
| `SCORE_RATING_WEIGHT` | 2 | Rating contribution |
| `SCORE_REVIEW_WEIGHT` | 1 | Review count contribution |
| `SCORE_VALUE_WEIGHT` | 2 | Price contribution |

Weight values must be finite numbers from 0 to 1000. Blank variables use defaults.

## Ranking and limitations

`score = discount% × discountWeight + rating × ratingWeight + log10(reviews + 1) × reviewWeight + 100/(100 + price) × valueWeight`.

The price term is a small affordability heuristic, not a measure of product quality. Original prices and ratings are optional in SerpAPI; missing original prices produce zero discount and remain eligible with the default minimum discount of zero. Sale labels alone never increase the score. Advertised discounts are not independently verified and there is no historical tracking.

Retailer names (including `.com` aliases) are matched exactly, excluding third-party sources such as `Amazon Marketplace` rather than assuming their identity. Valid offers are grouped by product ID or normalized title across categories and retailers; the highest-scoring offer wins. Identical titles can occasionally combine variants, so check product details before buying. Ties use lower price, name, retailer, then URL. Links prefer `link` when provided and fall back to `product_link`, which may be a Google Shopping page. Prices assume USD with `gl=us`. Each category retrieves the initial result page; there is no pagination or extra product-detail lookup.

If any category fails, the run exits with an error before sending a potentially misleading partial digest. Zero qualifying offers produces a clear no-deals message. Slack failures exit nonzero. HTTP calls time out after 30 seconds, the entire run after five minutes, and requests are not automatically retried to avoid duplicate digests and additional API charges. Structured logs include search counts, filtering, deduplication, selection, and delivery; remote response bodies and credential-bearing URLs are not logged.

## GitHub Actions

1. Push these files to your repository's default branch.
2. In **Settings → Secrets and variables → Actions**, create repository secrets `SERPAPI_KEY` and `SLACK_WEBHOOK_URL`.
3. Optionally set repository variables matching the configuration table.
4. Use **Actions → Daily deals → Run workflow** to verify delivery.

The daily workflow runs at **7:15 a.m. America/Los_Angeles**, including daylight saving changes, using GitHub's timezone-aware schedule. Change `.github/workflows/daily-deals.yml` to adjust the time. Schedules run from the default branch and can be delayed by GitHub; inactive public repositories can have schedules disabled. Six default categories mean six searches per run (roughly 180 per 30 days), subject to SerpAPI caching and plan billing. CI tests pull requests and pushes without accessing secrets.

## Validation and structure

```powershell
go test ./...
go vet ./...
go build ./cmd/deal-harvester
```

`cmd/deal-harvester` wires environment configuration and HTTP clients. `internal/app` orchestrates interface-based search and notification adapters. `internal/search`, `internal/deals`, `internal/notification`, and `internal/config` separate API parsing, scoring/filtering/deduplication, Slack formatting, and configuration. Tests use local HTTP servers and mocks without live credentials.

API references: [SerpAPI shopping results](https://serpapi.com/shopping-results), [Slack incoming webhooks](https://docs.slack.dev/messaging/sending-messages-using-incoming-webhooks/), [GitHub schedules](https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows#schedule).

## Category-specific searches

`SEARCH_PROFILES_FILE=search-profiles.json` loads the provided profiles instead of `SEARCH_CATEGORIES`. Edit that JSON file to change queries, strict price caps, required title patterns (all must match), and excluded patterns (any match rejects an offer). A `max_price` of zero or an omitted cap means unlimited. All profiles still obey global retailer, rating, and discount filters. Clear `SEARCH_PROFILES_FILE` to return to the category list locally; the daily workflow defaults to this profile file.

The configured profiles search 65-inch 4K TVs strictly below $200; gardening pots, railing/hanging planters, soil, and raised garden beds with no price cap; and Halloween/Christmas decor strictly below $20. Eight queries mean eight SerpAPI searches per run. Profile names, queries and conditions are safe to commit; keep secrets only in `.env` or GitHub secrets. A custom profile file must exist in the repository for GitHub Actions.

TV titles must explicitly mention 65 inches, TV/television, and 4K/UHD/Ultra HD; common accessories are excluded. Holiday relevance comes from the search query; decor profiles do not require holiday or decoration keywords in titles, but still exclude costume and candy titles. These conservative title checks can omit genuine matches with incomplete titles and cannot independently verify retailer specifications, shipping, taxes, or variants. Price caps apply to the listed item price before shipping and tax. Default `MIN_RATING=4` excludes missing ratings; your local `.env` has `MIN_RATING=0`. `MIN_DISCOUNT=0` permits offers without original prices; reported discounts still contribute to ranking. Looser holiday matching may admit unrelated items returned by Google Shopping, so inspect the product before buying.

## Test Slack alone

Run: `go run ./cmd/deal-harvester -slack-test`. This reads only Slack configuration, sends one clearly labeled sample digest, and uses zero SerpAPI searches. It does not require SERPAPI_KEY or a profile file. Do not combine it with -dry-run.
