package httpapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"smswhiteinfo/customer-api/internal/auth"
)

func TestInitiateDepositValidations(t *testing.T) {
	t.Setenv("PIPRAPAY_API_KEY", "")
	t.Setenv("PIPRAPAY_BASE_URL", "")

	app := fiber.New()
	s := &Server{}
	app.Post("/deposits", func(c fiber.Ctx) error {
		c.Locals("customer", auth.Claims{UserID: 42})
		return s.initiateDeposit(c)
	})

	cases := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{"below minimum", `{"amount":49,"gateway":"piprapay"}`, http.StatusUnprocessableEntity},
		{"above maximum", `{"amount":100001,"gateway":"piprapay"}`, http.StatusUnprocessableEntity},
		{"unsupported gateway", `{"amount":100,"gateway":"unknown"}`, http.StatusUnprocessableEntity},
		{"unconfigured gateway", `{"amount":100,"gateway":"piprapay"}`, http.StatusServiceUnavailable},
	}

	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodPost, "/deposits", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("%s: unexpected test error: %v", tc.name, err)
		}
		if resp.StatusCode != tc.wantStatus {
			t.Fatalf("%s: expected HTTP %d, got %d", tc.name, tc.wantStatus, resp.StatusCode)
		}
	}
}

func TestBuildMessagingAnalyticsPDF(t *testing.T) {
	pdfBytes := buildMessagingAnalyticsPDF(
		"Test Customer",
		"customer@example.com",
		"01 Nov 2026 - 15 Nov 2026",
		messagingAnalyticsStats{
			Total:       100,
			Successful:  95,
			Delivered:   90,
			Failed:      5,
			SuccessRate: 95.0,
			TotalCost:   33.25,
			TotalParts:  110,
			AvgParts:    1.1,
			Unicode:     10,
		},
		[]messagingDailyPoint{
			{
				Date:        "Nov 15",
				DateVal:     "2026-11-15",
				Total:       100,
				Successful:  95,
				Delivered:   90,
				Failed:      5,
				SuccessRate: 95.0,
				Cost:        33.25,
			},
		},
	)

	if !bytes.HasPrefix(pdfBytes, []byte("%PDF-1.4")) {
		t.Fatalf("expected valid PDF header")
	}
	if !bytes.Contains(pdfBytes, []byte("Messaging Analytics Report")) {
		t.Fatalf("expected report title inside PDF stream")
	}
}
