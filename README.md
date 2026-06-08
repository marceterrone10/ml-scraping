# MercadoLibre Cars Scraper

A Go scraper for MercadoLibre vehicle listings using [Colly](https://github.com/gocolly/colly).

Supports Argentina (MLA), Brazil (MLB), Mexico (MLM), Chile (MLC), Colombia (MCO), and Uruguay (MLU).

## Features

- Scrapes car listings from MercadoLibre search pages
- Extracts embedded page JSON (primary) with HTML fallback
- Parses title, price, year, kilometers, location, images, and attributes
- Paginates results automatically (48 listings per page)
- Exports to JSON or CSV
- Rate limiting and realistic browser headers

## Requirements

- Go 1.21+
- Browser cookies exported from a logged-in MercadoLibre session

## Install

```bash
go mod download
go build -o car-scrapper ./cmd/car-scrapper
```

## Setup

1. Log in to [mercadolibre.com.ar](https://www.mercadolibre.com.ar) in your browser
2. Export cookies with an extension (e.g. "Get cookies.txt LOCALLY")
3. Save as `cookies.json` in the project root
4. Create `.env`:

```env
ML_COOKIES=cookies.json
```

## Usage

```bash
# Scrape all cars (first page, Argentina)
./car-scrapper

# Search for Toyota Corolla, 3 pages
./car-scrapper -query "toyota corolla" -pages 3

# Brazil site, CSV output
./car-scrapper -site MLB -query "fiat argo" -output output/cars.csv

# Slower requests (helps avoid anti-bot blocks)
./car-scrapper -query "ford ranger" -delay 4s -verbose
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-site` | `MLA` | MercadoLibre site ID |
| `-query` | `""` | Search terms |
| `-pages` | `1` | Pages to scrape |
| `-delay` | `2s` | Delay between requests |
| `-output` | `output/listings.json` | Output path (`.json` or `.csv`) |
| `-cookies` | `$ML_COOKIES` | Browser cookies file (**required**) |
| `-proxy` | `""` | HTTP proxy URL |
| `-verbose` | `false` | Debug logging |

## Output example

```json
{
  "query": "toyota corolla",
  "site": "MLA",
  "total_found": 48,
  "pages_scraped": 1,
  "listings": [
    {
      "id": "MLA1234567890",
      "title": "Toyota Corolla 2020 2.0 Xei",
      "brand": "Toyota",
      "model": "Corolla",
      "year": 2020,
      "kilometers": 45000,
      "price": 18500000,
      "currency": "ARS",
      "location": "CABA, Capital Federal",
      "url": "https://auto.mercadolibre.com.ar/...",
      "site": "MLA"
    }
  ]
}
```

## Anti-bot block

MercadoLibre redirects unauthenticated automated traffic to a login wall. If blocked:

- Export fresh cookies while logged in
- Increase `-delay` (e.g. `4s`)
- Use `-proxy` with a residential proxy

## Project structure

```
.
├── cmd/car-scrapper/        # CLI entrypoint
├── internal/
│   ├── config/
│   │   ├── config.go        # App and scraper configuration
│   │   └── env.go           # Environment variable helpers
│   ├── models/listing.go    # Data types
│   ├── parser/parser.go     # JSON + HTML parsing
│   └── scraper/
│       ├── scraper.go       # Colly crawler
│       └── cookies.go       # Cookie file loader
```