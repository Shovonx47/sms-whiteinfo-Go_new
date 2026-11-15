package httpapi

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"testing"
)

func TestBuildMessagesPDFHasValidCrossReferences(t *testing.T) {
	data := buildMessagesPDF([]messagePDFRow{
		{id: 1, to: "8801700000001", senderID: "WHITEINFO", body: "Hello (world)", parts: 1, cost: 0.5, status: "delivered", operator: "Grameenphone", submitted: "2026-11-16 10:00:00"},
		{id: 2, to: "8801800000002", senderID: "WHITEINFO", body: "Unicode বাংলা message", parts: 1, cost: 0.5, status: "failed", operator: "Robi", submitted: "2026-11-16 10:01:00"},
	})

	if !bytes.HasPrefix(data, []byte("%PDF-1.4")) {
		t.Fatal("PDF does not start with a PDF header")
	}
	if !bytes.Contains(data, []byte("/F1 3 0 R")) || !bytes.Contains(data, []byte("/F2 4 0 R")) {
		t.Fatal("PDF page resources do not reference both fonts")
	}

	startXref := bytes.LastIndex(data, []byte("startxref"))
	if startXref < 0 {
		t.Fatal("PDF has no startxref")
	}
	match := regexp.MustCompile(`startxref\n(\d+)`).FindSubmatch(data[startXref:])
	if match == nil {
		t.Fatal("PDF startxref has no offset")
	}
	offset, _ := strconv.Atoi(string(match[1]))
	if offset <= 0 || offset >= len(data) || !bytes.HasPrefix(data[offset:], []byte("xref")) {
		t.Fatalf("startxref offset %d does not point at the xref table", offset)
	}

	xref := data[offset:]
	lines := bytes.Split(xref, []byte("\n"))
	if len(lines) < 3 {
		t.Fatal("xref table is truncated")
	}
	counts := bytes.Fields(lines[1])
	if len(counts) != 2 {
		t.Fatalf("invalid xref header %q", lines[1])
	}
	total, _ := strconv.Atoi(string(counts[1]))
	seen := 0
	for _, line := range lines[2:] {
		if len(line) < 18 || string(line[17:18]) != "n" {
			continue
		}
		seen++
		objOffset, err := strconv.Atoi(string(bytes.TrimSpace(line[:10])))
		if err != nil {
			t.Fatalf("invalid object offset %q", line[:10])
		}
		expected := []byte(fmt.Sprintf("%d 0 obj", seen))
		if objOffset <= 0 || objOffset >= len(data) || !bytes.HasPrefix(data[objOffset:], expected) {
			t.Fatalf("object %d offset %d does not point at %q", seen, objOffset, expected)
		}
	}
	if seen != total-1 {
		t.Fatalf("xref lists %d objects, expected %d", seen, total-1)
	}
}

func TestBuildMessagesPDFPaginates(t *testing.T) {
	rows := make([]messagePDFRow, 29)
	for i := range rows {
		rows[i] = messagePDFRow{id: int64(i + 1), to: "8801700000001", status: "delivered"}
	}
	data := buildMessagesPDF(rows)
	if !bytes.Contains(data, []byte("/Count 2")) {
		t.Fatal("29 rows should produce a two-page PDF")
	}
}
