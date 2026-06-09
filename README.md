# MercadoLibre Cars Scraper

A Go scraper for MercadoLibre vehicle listings using [Colly](https://github.com/gocolly/colly).

Supports Argentina (MLA), Brazil (MLB), Mexico (MLM), Chile (MLC), Colombia (MCO), and Uruguay (MLU).

## Features

- Scrapes **cars and trucks** from the MercadoLibre autos vertical (`autos.mercadolibre.com.ar`)
- Parses the embedded `product_list` JSON used on autos search pages
- Filters out marketplace parts, accessories, and ads
- Parses title, price, year, kilometers, brand, location, images, and attributes
- Paginates results automatically (48 listings per page)
- Batch mode: scrape multiple brand/model pairs with a worker pool
- Exports to JSON or CSV
- Rate limiting and realistic browser headers

## How search works

Single-query mode splits `-query` into brand and model (first word = brand, rest = model):

```text
-query "toyota corolla"  →  https://autos.mercadolibre.com.ar/toyota/corolla
```

Batch mode (`-jobs`) uses explicit `brand` and `model` fields from a JSON file when building URLs. Each job runs in a worker that calls Colly independently.

Vehicle listings link to `auto.mercadolibre.com.ar/...-_JM`. Results from `www.mercadolibre.com.ar/up/...` (parts catalog) are discarded.

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
# All cars on the autos homepage (first page, Argentina)
./car-scrapper

# Search for Toyota Corolla, 3 pages
./car-scrapper -query "toyota corolla" -pages 3

# Scrape multiple models with 3 concurrent workers
./car-scrapper -jobs jobs.example.json -pages 2 -workers 3

# Brazil site, CSV output
./car-scrapper -site MLB -query "fiat argo" -output output/cars.csv

# Slower requests (helps avoid anti-bot blocks)
./car-scrapper -query "ford ranger" -delay 4s -verbose
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-site` | `MLA` | MercadoLibre site ID |
| `-query` | `""` | Search terms (split into brand/model) |
| `-jobs` | `""` | JSON file with multiple brand/model jobs |
| `-workers` | `3` | Concurrent workers when using `-jobs` |
| `-pages` | `1` | Pages to scrape per job |
| `-delay` | `2s` | Delay between requests |
| `-output` | `output/listings.json` | Output path (`.json` or `.csv`) |
| `-cookies` | `$ML_COOKIES` | Browser cookies file (**required**) |
| `-proxy` | `""` | HTTP proxy URL |
| `-verbose` | `false` | Debug logging |

### Jobs file format

Create a JSON array. Each entry needs a `query` or both `brand` and `model`:

```json
[
  {"brand": "Toyota", "model": "Corolla", "query": "toyota corolla"},
  {"brand": "Ford", "model": "Ranger"}
]
```

See [jobs.example.json](jobs.example.json) for a full example.

## Output

### Single query

```json
{
  "query": "toyota corolla",
  "site": "MLA",
  "total_found": 48,
  "pages_scraped": 1,
  "listings": [
    {
      "id": "MLA1825910161",
      "title": "Toyota Corolla 1.8 Xei Mt 136cv",
      "brand": "Toyota",
      "price": 16000000,
      "currency": "ARS",
      "url": "https://auto.mercadolibre.com.ar/MLA-1825910161-toyota-corolla-18-xei-mt-136cv-_JM",
      "site": "MLA"
    }
  ]
}
```

### Batch (`-jobs`)

```json
{
  "site": "MLA",
  "total_jobs": 5,
  "total_found": 240,
  "listings": [],
  "results_by_query": [
    {
      "query": "toyota corolla",
      "total_found": 48,
      "listings": []
    }
  ],
  "errors": []
}
```

- `listings`: deduplicated vehicles across all jobs
- `results_by_query`: per-job breakdown
- `errors`: jobs that failed without stopping the batch

## Anti-bot block

MercadoLibre redirects unauthenticated automated traffic to a login wall. If blocked:

- Export fresh cookies while logged in
- Increase `-delay` (e.g. `4s`)
- Use `-proxy` with a residential proxy
- Reduce `-workers` in batch mode

## Architecture

Sequence diagrams of the main flows.

### Overview (single vs batch)

```mermaid
sequenceDiagram
    actor User
    participant main
    participant config
    participant pool
    participant scraper
    participant FS as filesystem

    User->>main: ./car-scrapper [flags]
    main->>config: Load()
    config-->>main: Config

    alt -jobs file provided
        main->>pool: LoadJobs(path)
        pool-->>main: []Job
        main->>pool: New(workers, cfg).Run(jobs)
        loop each worker consumes jobsCh
            pool->>scraper: New(cfg per job).Run()
            scraper-->>pool: SearchResult or error
        end
        pool-->>main: []Result
        main->>pool: MergeResults(site, results)
        pool-->>main: BatchResult
    else single -query
        main->>scraper: New(cfg).Run()
        scraper-->>main: SearchResult
    end

    main->>FS: writeOutput(JSON or CSV)
    FS-->>main: ok
    main-->>User: log done
```

### Scrape flow (single mode and each batch job)

Every scrape — whether invoked from `main` or a pool worker — follows this path:

```mermaid
sequenceDiagram
    participant caller as main or pool.worker
    participant scraper
    participant Colly
    participant ML as ML Autos
    participant parser

    caller->>scraper: New(ScraperConfig)
    caller->>scraper: Run()

    scraper->>scraper: newCollector(cookies, delay, proxy)
    scraper->>Colly: warmup → Visit(autosHome)
    Colly->>ML: GET autos.mercadolibre.com.ar/
    ML-->>Colly: HTML
    Colly-->>scraper: OnResponse (check blocked)

    scraper->>scraper: buildSearchURL(1..N)
    Note over scraper: autos.mercadolibre.com.ar/{brand}/{model}

    loop page 1 to MaxPages
        scraper->>Colly: Visit(searchURL)
        Colly->>ML: GET (headers + cookies)
        ML-->>Colly: HTML with product_list
        Colly->>scraper: OnResponse(body)
        scraper->>parser: ParsePage(body, site)
        parser-->>scraper: []Listing
        scraper->>scraper: append listings (mutex)
    end

    scraper->>Colly: Wait()
    scraper->>scraper: DedupeListings()
    scraper-->>caller: SearchResult
```

### Parser pipeline

```mermaid
sequenceDiagram
    participant scraper
    participant parser
    participant autos as parseProductList
    participant filter as filterVehicles

    scraper->>parser: ParsePage(body)
    parser->>autos: extract "product_list":[...]
    autos-->>parser: []Listing raw
    parser->>filter: IsVehicleListing per item
    Note over filter: reject /up/, ads, click1.<br/>accept auto.mercadolibre
    filter-->>parser: []Listing vehicles

    alt product_list empty
        parser->>parser: parseEmbeddedJSON (fallback)
        parser->>parser: parseHTML (fallback)
    end

    parser-->>scraper: listings or error
```

### Worker pool (batch mode)

```mermaid
sequenceDiagram
    participant main
    participant pool
    participant producer as goroutine producer
    participant closer as goroutine closer
    participant W1 as Worker 1
    participant W2 as Worker N
    participant jobsCh as jobsCh
    participant resultsCh as resultsCh
    participant scraper

    main->>pool: Run(jobs)

    par launch workers
        pool->>W1: go worker(jobsCh, resultsCh)
        pool->>W2: go worker(jobsCh, resultsCh)
    end

    pool->>producer: go enqueue jobs
    producer->>jobsCh: job1, job2, ... jobN
    producer->>jobsCh: close()

    par workers compete for jobs
        W1->>jobsCh: receive job
        W1->>scraper: Run()
        scraper-->>W1: SearchResult
        W1->>resultsCh: Result
        W2->>jobsCh: receive job
        W2->>scraper: Run()
    end

    pool->>closer: go wg.Wait then close(resultsCh)
    closer->>closer: wg.Wait()
    closer->>resultsCh: close()

    pool->>resultsCh: range and collect
    pool-->>main: []Result
    main->>pool: MergeResults()
    Note over pool: dedupe globally, collect errors
```

## Project structure

```
.
├── cmd/car-scrapper/           # CLI entrypoint
├── jobs.example.json           # Sample batch jobs file
├── internal/
│   ├── config/
│   │   ├── config.go           # Flags and scraper settings
│   │   └── env.go              # Environment variable helpers
│   ├── models/listing.go       # Listing, SearchResult, BatchResult
│   ├── parser/
│   │   ├── autos.go            # product_list parser for autos pages
│   │   ├── vehicle.go          # Vehicle vs parts filtering
│   │   └── parser.go           # JSON + HTML fallbacks
│   ├── pool/
│   │   ├── pool.go             # Worker pool (WaitGroup + job queue)
│   │   └── jobs.go             # Jobs file loader
│   └── scraper/
│       ├── scraper.go          # Colly crawler, autos URL builder
│       └── cookies.go          # Cookie file loader
```
