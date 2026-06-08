package parser

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/marceloterrone/car-scrapper/internal/models"
)

var (
	preloadedStateRE = regexp.MustCompile(`(?:window\.)?__PRELOADED_STATE__\s*=\s*(\{.+?\})\s*;?\s*</script>`)
	melidataRE       = regexp.MustCompile(`melidata\("add",\s*"event_data",\s*(\{.+?\})\)`)
	itemIDRE         = regexp.MustCompile(`(ML[A-Z]\d+)`)
	yearRE           = regexp.MustCompile(`\b(19|20)\d{2}\b`)
	kmRE             = regexp.MustCompile(`([\d.]+)\s*(?:km|Kms?|kilómetros?)`)
)

// ParsePage extracts listings from a MercadoLibre search page body.
func ParsePage(body []byte, site string) ([]models.Listing, error) {
	if listings := parseEmbeddedJSON(body, site); len(listings) > 0 {
		return listings, nil
	}

	listings := parseHTML(body, site)
	if len(listings) == 0 {
		return nil, fmt.Errorf("no listings found in page (blocked or empty results)")
	}
	return listings, nil
}

func parseEmbeddedJSON(body []byte, site string) []models.Listing {
	for _, re := range []*regexp.Regexp{preloadedStateRE, melidataRE} {
		match := re.FindSubmatch(body)
		if len(match) < 2 {
			continue
		}
		var data any
		if err := json.Unmarshal(match[1], &data); err != nil {
			continue
		}
		if listings := walkForListings(data, site); len(listings) > 0 {
			return listings
		}
	}

	// Fallback: scan script tags for inline JSON state blobs.
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil
	}
	var listings []models.Listing
	doc.Find("script").Each(func(_ int, s *goquery.Selection) {
		text := s.Text()
		if !strings.Contains(text, "MLA") && !strings.Contains(text, "MLB") && !strings.Contains(text, "MLM") {
			return
		}
		for _, candidate := range findJSONRoots(text) {
			var obj any
			if err := json.Unmarshal([]byte(candidate), &obj); err != nil {
				continue
			}
			if found := walkForListings(obj, site); len(found) > len(listings) {
				listings = found
			}
		}
	})
	return listings
}

func findJSONRoots(text string) []string {
	var roots []string
	for i, ch := range text {
		if ch != '{' && ch != '[' {
			continue
		}
		end := matchBracket(text[i:])
		if end <= 0 {
			continue
		}
		candidate := text[i : i+end]
		if strings.Contains(candidate, `"permalink"`) || strings.Contains(candidate, `"price"`) {
			roots = append(roots, candidate)
		}
	}
	return roots
}

func matchBracket(s string) int {
	if len(s) == 0 {
		return 0
	}
	open := s[0]
	var close byte
	switch open {
	case '{':
		close = '}'
	case '[':
		close = ']'
	default:
		return 0
	}

	depth := 0
	inString := false
	escaped := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return 0
}

func walkForListings(data any, site string) []models.Listing {
	best := []models.Listing{}
	walk(data, func(arr []map[string]any) {
		listings := mapsToListings(arr, site)
		if len(listings) > len(best) {
			best = listings
		}
	})
	return best
}

func walk(data any, onArray func([]map[string]any)) {
	switch v := data.(type) {
	case map[string]any:
		for _, val := range v {
			walk(val, onArray)
		}
		if isListingArray(v) {
			onArray([]map[string]any{v})
		}
	case []any:
		if len(v) == 0 {
			return
		}
		maps := make([]map[string]any, 0, len(v))
		for _, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				walk(item, onArray)
				continue
			}
			if looksLikeListing(m) {
				maps = append(maps, m)
			} else {
				walk(m, onArray)
			}
		}
		if len(maps) >= 3 {
			onArray(maps)
		}
		for _, item := range v {
			walk(item, onArray)
		}
	}
}

func isListingArray(m map[string]any) bool {
	return looksLikeListing(m)
}

func looksLikeListing(m map[string]any) bool {
	id, _ := m["id"].(string)
	title, _ := m["title"].(string)
	if id == "" || title == "" {
		return false
	}
	if !itemIDRE.MatchString(id) {
		return false
	}
	_, hasPrice := m["price"]
	_, hasPermalink := m["permalink"]
	return hasPrice || hasPermalink
}

func mapsToListings(items []map[string]any, site string) []models.Listing {
	now := time.Now().UTC()
	listings := make([]models.Listing, 0, len(items))
	seen := make(map[string]struct{})

	for _, item := range items {
		listing := mapToListing(item, site, now)
		if listing.ID == "" {
			continue
		}
		if _, ok := seen[listing.ID]; ok {
			continue
		}
		seen[listing.ID] = struct{}{}
		listings = append(listings, listing)
	}
	return listings
}

func mapToListing(item map[string]any, site string, scrapedAt time.Time) models.Listing {
	id, _ := item["id"].(string)
	title, _ := item["title"].(string)
	permalink, _ := item["permalink"].(string)
	condition, _ := item["condition"].(string)
	currency, _ := item["currency_id"].(string)
	location := extractLocation(item)
	attrs := extractAttributes(item)

	listing := models.Listing{
		ID:            id,
		Title:         title,
		Price:         toFloat(item["price"]),
		OriginalPrice: toFloat(item["original_price"]),
		Currency:      currency,
		Condition:     condition,
		Location:      location,
		URL:           permalink,
		ImageURL:      firstPicture(item),
		Attributes:    attrs,
		Site:          site,
		ScrapedAt:     scrapedAt,
	}

	listing.Brand = attrs["BRAND"]
	if listing.Brand == "" {
		listing.Brand = attrs["brand"]
	}
	listing.Model = attrs["MODEL"]
	if listing.Model == "" {
		listing.Model = attrs["model"]
	}
	listing.Year = toInt(attrs["CAR_YEAR"])
	if listing.Year == 0 {
		listing.Year = toInt(attrs["YEAR"])
	}
	listing.Kilometers = toInt(attrs["KILOMETERS"])
	if listing.Kilometers == 0 {
		listing.Kilometers = toInt(attrs["KILOMETRAJE"])
	}

	if listing.URL == "" && id != "" {
		listing.URL = buildItemURL(site, id, title)
	}

	return listing
}

func extractLocation(item map[string]any) string {
	if loc, ok := item["location"].(map[string]any); ok {
		parts := []string{}
		for _, key := range []string{"city_name", "state_name", "neighborhood_name"} {
			if v, ok := loc[key].(string); ok && v != "" {
				parts = append(parts, v)
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, ", ")
		}
	}
	if addr, ok := item["seller_address"].(map[string]any); ok {
		if city, ok := addr["city"].(map[string]any); ok {
			if name, ok := city["name"].(string); ok {
				return name
			}
		}
	}
	return ""
}

func extractAttributes(item map[string]any) map[string]string {
	attrs := map[string]string{}
	raw, ok := item["attributes"].([]any)
	if !ok {
		return attrs
	}
	for _, a := range raw {
		m, ok := a.(map[string]any)
		if !ok {
			continue
		}
		id, _ := m["id"].(string)
		value, _ := m["value_name"].(string)
		if id != "" && value != "" {
			attrs[id] = value
		}
	}
	return attrs
}

func firstPicture(item map[string]any) string {
	if thumb, ok := item["thumbnail"].(string); ok {
		return thumb
	}
	if pics, ok := item["pictures"].([]any); ok && len(pics) > 0 {
		if pic, ok := pics[0].(map[string]any); ok {
			if url, ok := pic["url"].(string); ok {
				return url
			}
		}
	}
	return ""
}

func parseHTML(body []byte, site string) []models.Listing {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil
	}

	now := time.Now().UTC()
	var listings []models.Listing
	seen := make(map[string]struct{})

	selectors := []string{
		"li.ui-search-layout__item",
		"div.ui-search-result",
		"div.poly-card",
	}

	for _, sel := range selectors {
		doc.Find(sel).Each(func(_ int, card *goquery.Selection) {
			listing := parseCard(card, site, now)
			if listing.ID == "" {
				return
			}
			if _, ok := seen[listing.ID]; ok {
				return
			}
			seen[listing.ID] = struct{}{}
			listings = append(listings, listing)
		})
		if len(listings) > 0 {
			break
		}
	}

	return listings
}

func parseCard(card *goquery.Selection, site string, scrapedAt time.Time) models.Listing {
	link := card.Find("a.poly-component__title, h2 a, a.ui-search-link").First()
	href, _ := link.Attr("href")
	title := strings.TrimSpace(link.Text())

	id := ""
	if href != "" {
		if m := itemIDRE.FindStringSubmatch(href); len(m) > 1 {
			id = m[1]
		}
	}

	priceText := card.Find(".andes-money-amount__fraction").First().Text()
	currency := strings.TrimSpace(card.Find(".andes-money-amount__currency-symbol").First().Text())
	location := strings.TrimSpace(card.Find(".poly-component__location, .ui-search-item__location").First().Text())

	attrs := map[string]string{}
	card.Find(".poly-attributes_list__item, .ui-search-item__group__element").Each(func(_ int, el *goquery.Selection) {
		text := strings.TrimSpace(el.Text())
		if text == "" {
			return
		}
		attrs[text] = text
	})

	listing := models.Listing{
		ID:         id,
		Title:      title,
		Price:      parsePrice(priceText),
		Currency:   currency,
		Location:   location,
		URL:        href,
		ImageURL:   card.Find("img").First().AttrOr("src", ""),
		Attributes: attrs,
		Site:       site,
		ScrapedAt:  scrapedAt,
	}

	if listing.URL == "" && id != "" {
		listing.URL = buildItemURL(site, id, title)
	}

	// Parse year/km from attribute chips or title.
	combined := title + " " + strings.Join(mapValues(attrs), " ")
	if y := yearRE.FindString(combined); y != "" {
		listing.Year = toInt(y)
	}
	if km := kmRE.FindStringSubmatch(combined); len(km) > 1 {
		listing.Kilometers = toInt(strings.ReplaceAll(km[1], ".", ""))
	}

	return listing
}

func mapValues(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

func parsePrice(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, ",", ".")
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case string:
		return parsePrice(n)
	default:
		return 0
	}
}

func toInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case string:
		s := strings.ReplaceAll(n, ".", "")
		s = strings.TrimSpace(s)
		i, _ := strconv.Atoi(s)
		return i
	default:
		return 0
	}
}

func buildItemURL(site, id, title string) string {
	slug := slugify(title)
	domain := siteDomain(site)
	return fmt.Sprintf("https://auto.%s/%s-%s", domain, strings.ReplaceAll(id, "MLA", "MLA-"), slug)
}

func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.TrimSpace(s)
	repl := regexp.MustCompile(`[^a-z0-9]+`)
	s = repl.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 60 {
		s = s[:60]
		s = strings.TrimRight(s, "-")
	}
	return s
}

func siteDomain(site string) string {
	switch site {
	case "MLB":
		return "mercadolivre.com.br"
	case "MLM":
		return "mercadolibre.com.mx"
	case "MLC":
		return "mercadolibre.cl"
	case "MCO":
		return "mercadolibre.com.co"
	case "MLU":
		return "mercadolibre.com.uy"
	default:
		return "mercadolibre.com.ar"
	}
}
