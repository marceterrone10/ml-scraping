package parser

import (
	"strings"
	"testing"
)

func TestParseProductList(t *testing.T) {
	body := []byte(`{"product_list":[{"id":"MLA1825910161","name":"Toyota Corolla 1.8 Xei Mt 136cv","image":"http://http2.mlstatic.com/D_857704-I.webp","item_offered":{"price":16000000,"price_currency":"ARS","url":"https://auto.mercadolibre.com.ar/MLA-1825910161-toyota-corolla-18-xei-mt-136cv-_JM"},"brand_attribute":{"name":"Toyota"},"type":"Product"}]}`)

	listings := parseProductList(body, "MLA")
	if len(listings) != 1 {
		t.Fatalf("expected 1 listing, got %d", len(listings))
	}
	l := listings[0]
	if l.Title != "Toyota Corolla 1.8 Xei Mt 136cv" {
		t.Fatalf("unexpected title: %q", l.Title)
	}
	if l.Price != 16000000 {
		t.Fatalf("unexpected price: %v", l.Price)
	}
	if !strings.Contains(l.URL, "auto.mercadolibre.com.ar") {
		t.Fatalf("unexpected url: %q", l.URL)
	}
}
