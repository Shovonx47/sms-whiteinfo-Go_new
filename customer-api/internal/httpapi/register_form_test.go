package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestParseRegisterFormMultipart(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	fields := map[string]string{
		"name":                     "Test Customer",
		"email":                    "test@example.com",
		"phone":                    "01712345678",
		"password":                 "Password123!",
		"password_confirmation":    "Password123!",
		"email_verification_token": "email-token",
		"phone_verification_token": "phone-token",
		"nid_number":               "1234567890",
		"business_name":            "Test Business",
		"trade_license_number":     "TL-123",
		"business_address":         "Dhaka",
		"plan_slug":                "starter",
		"terms":                    "1",
		"live_photo":               "data:image/jpeg;base64,AA==",
	}
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("write field %s: %v", key, err)
		}
	}

	pngBytes := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	for _, name := range []string{"front_nid", "back_nid", "trade_license_file"} {
		part, err := writer.CreateFormFile(name, name+".png")
		if err != nil {
			t.Fatalf("create file %s: %v", name, err)
		}
		if _, err := part.Write(pngBytes); err != nil {
			t.Fatalf("write file %s: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	app := fiber.New()
	app.Post("/parse", func(context fiber.Ctx) error {
		form, err := parseRegisterForm(context)
		if err != nil {
			return context.Status(422).JSON(fiber.Map{"error": err.Error()})
		}

		return context.JSON(fiber.Map{
			"name":       form.Name,
			"email":      form.Email,
			"phone":      form.Phone,
			"terms":      form.Terms,
			"plan_slug":  form.PlanSlug,
			"front_name": form.FrontNID.Filename,
			"back_name":  form.BackNID.Filename,
			"trade_name": form.TradeLicenseFile.Filename,
		})
	})

	request := httptest.NewRequest("POST", "/parse", bytes.NewReader(body.Bytes()))
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := app.Test(request, fiber.TestConfig{Timeout: 0})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer response.Body.Close()

	raw, _ := io.ReadAll(response.Body)
	if response.StatusCode != 200 {
		t.Fatalf("expected 200, got %d: %s", response.StatusCode, raw)
	}

	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if parsed["name"] != "Test Customer" || parsed["email"] != "test@example.com" {
		t.Fatalf("unexpected fields: %v", parsed)
	}
	if parsed["terms"] != true {
		t.Fatalf("expected terms true, got %v", parsed["terms"])
	}
	if parsed["front_name"] != "front_nid.png" || parsed["trade_name"] != "trade_license_file.png" {
		t.Fatalf("file parts not parsed: %v", parsed)
	}
}

func TestPhoneVariants(t *testing.T) {
	variants := phoneVariants("+8801712345678")
	want := map[string]bool{
		"+8801712345678": true,
		"8801712345678":  true,
		"01712345678":    true,
	}
	if len(variants) != len(want) {
		t.Fatalf("expected %d variants, got %d (%v)", len(want), len(variants), variants)
	}
	for _, variant := range variants {
		if !want[variant] {
			t.Fatalf("unexpected variant %q", variant)
		}
	}
}
