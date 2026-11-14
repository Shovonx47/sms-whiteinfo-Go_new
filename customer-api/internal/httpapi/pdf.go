package httpapi

import (
	"bytes"
	"fmt"
	"strings"
	"time"
)

// messagePDFRow is a printable subset of the message export row.
type messagePDFRow struct {
	id        int64
	to        string
	senderID  string
	body      string
	parts     int
	cost      float64
	status    string
	reason    string
	operator  string
	submitted string
	delivered string
}

func pdfEscape(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "(", "\\(")
	value = strings.ReplaceAll(value, ")", "\\)")
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	var builder strings.Builder
	for _, r := range value {
		if r < 32 || r > 126 {
			builder.WriteByte('?')
			continue
		}
		builder.WriteRune(r)
	}
	return builder.String()
}

func pdfText(x, y float64, font string, size int, text string) string {
	return fmt.Sprintf("BT /%s %d Tf %.1f %.1f Td (%s) Tj ET\n", font, size, x, y, pdfEscape(text))
}

func pdfTruncate(value string, limit int) string {
	value = strings.ReplaceAll(value, "\n", " ")
	if len(value) <= limit {
		return value
	}
	return value[:limit-1] + "…"
}

// buildMessagesPDF renders the export rows into a landscape PDF without
// external dependencies. Non-ASCII characters are rendered as "?".
func buildMessagesPDF(rows []messagePDFRow) []byte {
	const (
		pageWidth  = 842.0
		pageHeight = 595.0
		rowsPerPage = 28
	)

	pages := [][]messagePDFRow{}
	for start := 0; start < len(rows) || start == 0; start += rowsPerPage {
		end := start + rowsPerPage
		if end > len(rows) {
			end = len(rows)
		}
		pages = append(pages, rows[start:end])
		if end == len(rows) {
			break
		}
	}

	streams := make([]string, 0, len(pages))
	for pageIndex, pageRows := range pages {
		var content strings.Builder
		content.WriteString(pdfText(32, 552, "F2", 16, "Sent Messages Report"))
		content.WriteString(pdfText(32, 532, "F1", 8,
			fmt.Sprintf("Generated %s  •  Page %d of %d", time.Now().Format("02 Jan 2006 15:04"), pageIndex+1, len(pages))))

		columns := []struct {
			x     float64
			label string
		}{
			{32, "Message ID"}, {92, "Recipient"}, {170, "Sender"}, {232, "Content"},
			{470, "Parts"}, {505, "Cost"}, {555, "Status"}, {625, "Operator"},
			{700, "Submitted"},
		}
		headerY := 506.0
		content.WriteString(pdfText(32, headerY, "F2", 7, "Message ID"))
		content.WriteString(pdfText(92, headerY, "F2", 7, "Recipient"))
		content.WriteString(pdfText(170, headerY, "F2", 7, "Sender"))
		content.WriteString(pdfText(232, headerY, "F2", 7, "Content"))
		content.WriteString(pdfText(470, headerY, "F2", 7, "Parts"))
		content.WriteString(pdfText(505, headerY, "F2", 7, "Cost"))
		content.WriteString(pdfText(555, headerY, "F2", 7, "Status"))
		content.WriteString(pdfText(625, headerY, "F2", 7, "Operator"))
		content.WriteString(pdfText(700, headerY, "F2", 7, "Submitted"))
		_ = columns

		y := headerY - 14
		for _, row := range pageRows {
			content.WriteString(pdfText(32, y, "F1", 7, fmt.Sprintf("MSG-%06d", row.id)))
			content.WriteString(pdfText(92, y, "F1", 7, pdfTruncate(row.to, 14)))
			content.WriteString(pdfText(170, y, "F1", 7, pdfTruncate(row.senderID, 12)))
			content.WriteString(pdfText(232, y, "F1", 7, pdfTruncate(row.body, 46)))
			content.WriteString(pdfText(470, y, "F1", 7, fmt.Sprintf("%d", row.parts)))
			content.WriteString(pdfText(505, y, "F1", 7, fmt.Sprintf("%.4f", row.cost)))
			content.WriteString(pdfText(555, y, "F1", 7, pdfTruncate(row.status, 11)))
			content.WriteString(pdfText(625, y, "F1", 7, pdfTruncate(row.operator, 12)))
			content.WriteString(pdfText(700, y, "F1", 7, pdfTruncate(row.submitted, 17)))
			y -= 16
		}

		streams = append(streams, content.String())
	}

	var buffer bytes.Buffer
	buffer.WriteString("%PDF-1.4\n")
	offsets := make([]int, 0)
	objectCount := 3 + len(pages)*2
	writeObject := func(number int, body string) {
		offsets = append(offsets, buffer.Len())
		fmt.Fprintf(&buffer, "%d 0 obj\n%s\nendobj\n", number, body)
	}

	pagesKids := make([]string, 0, len(pages))
	pageObjectStart := 4
	for index := range pages {
		pagesKids = append(pagesKids, fmt.Sprintf("%d 0 R", pageObjectStart+index*2))
	}
	writeObject(1, "<< /Type /Catalog /Pages 2 0 R >>")
	writeObject(2, fmt.Sprintf("<< /Type /Pages /Count %d /Kids [%s] >>", len(pages), strings.Join(pagesKids, " ")))
	writeObject(3, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	_ = objectCount

	contentObject := 4 + len(pages)*2
	for index, stream := range streams {
		pageObject := pageObjectStart + index*2
		writeObject(pageObject, fmt.Sprintf(
			"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.0f %.0f] /Resources << /Font << /F1 3 0 R /F2 %d 0 R >> >> /Contents %d 0 R >>",
			pageWidth, pageHeight, contentObject+index, contentObject+index,
		))
		contentOffset := buffer.Len()
		fmt.Fprintf(&buffer, "%d 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj\n", contentObject+index, len(stream), stream)
		offsets = append(offsets, contentOffset)
	}

	boldFontObject := 3 + len(pages)*2 + 1
	// Font object 3 is Helvetica; add Helvetica-Bold as the final object.
	writeObject(boldFontObject, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold >>")

	xrefOffset := buffer.Len()
	totalObjects := boldFontObject
	fmt.Fprintf(&buffer, "xref\n0 %d\n0000000000 65535 f \n", totalObjects+1)
	for i := 0; i < totalObjects; i++ {
		fmt.Fprintf(&buffer, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&buffer, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", totalObjects+1, xrefOffset)

	return buffer.Bytes()
}
