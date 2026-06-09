package parser

import (
	"strings"

	"github.com/marceloterrone/car-scrapper/internal/models"
)

const minVehiclePriceARS = 500_000

// IsVehicleListing reports whether a parsed listing is a car/truck ad (not parts or accessories).
func IsVehicleListing(l models.Listing) bool {
	url := strings.ToLower(strings.TrimSpace(l.URL))

	if isExcludedURL(url) {
		return false
	}

	if isAutoVerticalURL(url) {
		return true
	}

	if l.Kilometers > 0 {
		return true
	}

	if l.Year > 0 && l.Price >= minVehiclePriceARS {
		return true
	}

	if l.Price >= minVehiclePriceARS && l.Location != "" {
		return true
	}

	return false
}

func isExcludedURL(url string) bool {
	if url == "" {
		return false
	}
	excluded := []string{
		"click1.",
		"mclics",
		"/up/",
		"/p/",
		"articulo.mercadolibre",
	}
	for _, marker := range excluded {
		if strings.Contains(url, marker) {
			return true
		}
	}
	return false
}

func isAutoVerticalURL(url string) bool {
	return strings.Contains(url, "auto.mercadolibre") ||
		strings.Contains(url, "auto.mercadolivre")
}

func looksLikeVehicleItem(m map[string]any) bool {
	if !looksLikeListing(m) {
		return false
	}

	permalink, _ := m["permalink"].(string)
	if isExcludedURL(strings.ToLower(permalink)) {
		return false
	}
	if isAutoVerticalURL(strings.ToLower(permalink)) {
		return true
	}

	if domainID, _ := m["domain_id"].(string); domainID != "" {
		upper := strings.ToUpper(domainID)
		if strings.Contains(upper, "CARS") || strings.Contains(upper, "VEHICLE") {
			return true
		}
	}

	attrs := extractAttributes(m)
	if attrs["KILOMETERS"] != "" || attrs["KILOMETRAJE"] != "" {
		return true
	}
	if attrs["CAR_YEAR"] != "" || attrs["VEHICLE_YEAR"] != "" {
		return true
	}

	price := toFloat(m["price"])
	if price >= minVehiclePriceARS {
		if attrs["BRAND"] != "" || attrs["MODEL"] != "" {
			return true
		}
	}

	return false
}

func filterVehicles(listings []models.Listing) []models.Listing {
	if len(listings) == 0 {
		return nil
	}
	out := make([]models.Listing, 0, len(listings))
	for _, l := range listings {
		if IsVehicleListing(l) {
			out = append(out, l)
		}
	}
	return out
}
