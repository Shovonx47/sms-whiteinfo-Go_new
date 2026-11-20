package httpapi

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

func reportRange(t *testing.T, query string) (string, string, string) {
	t.Helper()

	app := fiber.New()
	app.Get("/report", func(context fiber.Ctx) error {
		start, end, period, err := depositReportRange(context)
		if err != nil {
			return context.Status(400).JSON(fiber.Map{"message": err.Error()})
		}

		return context.JSON(fiber.Map{
			"from":   start.Format("2006-01-02 15:04:05"),
			"to":     end.Format("2006-01-02 15:04:05"),
			"period": period,
		})
	})

	response, err := app.Test(httptest.NewRequest("GET", "/report"+query, nil), fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if response.StatusCode != 200 {
		t.Fatalf("expected 200 for %q, got %d", query, response.StatusCode)
	}

	var payload struct {
		From   string `json:"from"`
		To     string `json:"to"`
		Period string `json:"period"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	return payload.From, payload.To, payload.Period
}

func TestDepositReportRangeCustom(t *testing.T) {
	from, to, period := reportRange(t, "?period=custom&from=2026-11-01&to=2026-11-18")
	if period != "custom" {
		t.Fatalf("expected custom period, got %q", period)
	}
	if from != "2026-11-01 00:00:00" {
		t.Fatalf("unexpected start: %q", from)
	}
	if to != "2026-11-19 00:00:00" {
		t.Fatalf("expected exclusive end the day after 'to', got %q", to)
	}
}

func TestDepositReportRangeReversedCustomFallsBackToFrom(t *testing.T) {
	from, to, _ := reportRange(t, "?period=custom&from=2026-11-10&to=2026-11-02")
	if from != "2026-11-10 00:00:00" {
		t.Fatalf("unexpected start: %q", from)
	}
	if to != "2026-11-11 00:00:00" {
		t.Fatalf("reversed range should collapse to a single day, got %q", to)
	}
}

func TestDepositReportRangeInvalidCustomRejected(t *testing.T) {
	app := fiber.New()
	app.Get("/report", func(context fiber.Ctx) error {
		if _, _, _, err := depositReportRange(context); err != nil {
			return context.Status(400).JSON(fiber.Map{"message": err.Error()})
		}
		return context.SendStatus(200)
	})

	response, err := app.Test(httptest.NewRequest("GET", "/report?period=custom&from=nope&to=2026-11-18", nil), fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if response.StatusCode != 400 {
		t.Fatalf("expected 400 for invalid dates, got %d", response.StatusCode)
	}
}

func TestParseDepositTimeAcceptsFormAndDateInputs(t *testing.T) {
	cases := []string{
		"2026-11-18T13:45:00Z",
		"2026-11-18T13:45",
		"2026-11-18 13:45:05",
		"2026-11-18 13:45",
		"2026-11-18",
	}

	for _, value := range cases {
		parsed, err := parseDepositTime(value)
		if err != nil {
			t.Fatalf("expected %q to parse, got error: %v", value, err)
		}
		if parsed.Year() != 2026 || parsed.Month() != time.November || parsed.Day() != 18 {
			t.Fatalf("unexpected parsed date for %q: %v", value, parsed)
		}
	}

	if _, err := parseDepositTime("not-a-date"); err == nil {
		t.Fatal("expected an error for an invalid date")
	}
}

func TestDepositReportRangeSevenDaysIsInclusive(t *testing.T) {
	from, to, period := reportRange(t, "?period=7days")
	if period != "7days" {
		t.Fatalf("unexpected period: %q", period)
	}
	if from >= to {
		t.Fatalf("start %q must be before end %q", from, to)
	}
}
