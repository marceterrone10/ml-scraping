package parser

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/marceloterrone/car-scrapper/internal/models"
)

const productListKey = `"product_list":`

// parseProductList extracts listings from autos.mercadolibre search pages.
func parseProductList(body []byte, site string) []models.Listing {
	text := string(body)
	var best []models.Listing

	idx := 0
	for {
		rel := strings.Index(text[idx:], productListKey)
		if rel < 0 {
			break
		}
		start := idx + rel + len(productListKey)
		if start >= len(text) || text[start] != '[' {
			idx = start
			continue
		}

		end := matchBracket(text[start:])
		if end <= 0 {
			idx = start + 1
			continue
		}

		var items []map[string]any
		if err := json.Unmarshal([]byte(text[start:start+end]), &items); err == nil {
			listings := productListToListings(items, site)
			if len(listings) > len(best) {
				best = listings
			}
		}
		idx = start + end
	}

	return filterVehicles(best)
}

func productListToListings(items []map[string]any, site string) []models.Listing {
	now := time.Now().UTC()
	listings := make([]models.Listing, 0, len(items))
	seen := make(map[string]struct{}, len(items))

	for _, item := range items {
		id, _ := item["id"].(string)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}

		name, _ := item["name"].(string)
		image, _ := item["image"].(string)
		offered, _ := item["item_offered"].(map[string]any)

		url, _ := offered["url"].(string)
		if url != "" && !strings.HasPrefix(url, "http") {
			url = "https://" + strings.TrimPrefix(url, "//")
		}

		listing := models.Listing{
			ID:        id,
			Title:     name,
			Price:     toFloat(offered["price"]),
			Currency:  stringField(offered["price_currency"]),
			URL:       url,
			ImageURL:  normalizeImageURL(image),
			Site:      site,
			ScrapedAt: now,
		}

		if ba, ok := item["brand_attribute"].(map[string]any); ok {
			listing.Brand, _ = ba["name"].(string)
		}
		if ma, ok := item["model_attribute"].(map[string]any); ok {
			listing.Model, _ = ma["name"].(string)
		}
		if ya, ok := item["year_attribute"].(map[string]any); ok {
			listing.Year = toInt(ya["name"])
		}
		if ka, ok := item["kilometers_attribute"].(map[string]any); ok {
			listing.Kilometers = toInt(strings.ReplaceAll(stringField(ka["name"]), ".", ""))
		}

		if listing.Kilometers == 0 {
			if km := kmRE.FindStringSubmatch(name); len(km) > 1 {
				listing.Kilometers = toInt(strings.ReplaceAll(km[1], ".", ""))
			}
		}
		if listing.Year == 0 {
			if y := yearRE.FindString(name); y != "" {
				listing.Year = toInt(y)
			}
		}

		listings = append(listings, listing)
	}

	return listings
}

func stringField(v any) string {
	s, _ := v.(string)
	return s
}

func normalizeImageURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "http://http") {
		raw = strings.TrimPrefix(raw, "http://")
	}
	if strings.HasPrefix(raw, "//") {
		return "https:" + raw
	}
	if !strings.HasPrefix(raw, "http") {
		return "https://" + raw
	}
	return raw
}
