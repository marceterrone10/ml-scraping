package parser

import (
	"testing"

	"github.com/marceloterrone/car-scrapper/internal/models"
)

func TestIsVehicleListing_rejectsParts(t *testing.T) {
	parts := models.Listing{
		ID:    "MLA1604250222",
		Title: "Pomo Bocha Palanca Cambios Toyota Corolla",
		Price: 69753,
		URL:   "https://www.mercadolibre.com.ar/pomo-bocha/up/MLAU123",
	}
	if IsVehicleListing(parts) {
		t.Fatal("expected parts listing to be rejected")
	}
}

func TestIsVehicleListing_acceptsAutoVertical(t *testing.T) {
	car := models.Listing{
		ID:         "MLA2169156508",
		Title:      "Toyota Corolla 2.0 Xei Cvt 170cv",
		Price:      25000000,
		Kilometers: 13000,
		Year:       2024,
		URL:        "https://auto.mercadolibre.com.ar/MLA-2169156508-toyota-corolla-20-xei-_JM",
	}
	if !IsVehicleListing(car) {
		t.Fatal("expected vehicle listing to be accepted")
	}
}

func TestIsVehicleListing_rejectsAds(t *testing.T) {
	ad := models.Listing{
		ID:    "MLA123",
		Title: "Some part",
		Price: 50000,
		URL:   "https://click1.mercadolibre.com.ar/mclics/clicks/external/MLA/count?a=foo",
	}
	if IsVehicleListing(ad) {
		t.Fatal("expected ad URL to be rejected")
	}
}
