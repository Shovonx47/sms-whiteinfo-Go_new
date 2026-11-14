package httpapi

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

var templateVarPlaceholderRe = regexp.MustCompile(`\\\{[a-zA-Z0-9_]+\\\}`)

func (server *Server) acquireSubmitLock(userID int64, action string) bool {
	key := fmt.Sprintf("%s:%d", action, userID)
	now := time.Now()
	server.submitLockMu.Lock()
	defer server.submitLockMu.Unlock()

	if prev, ok := server.submitLocks[key]; ok && now.Sub(prev) < 5*time.Second {
		return false
	}
	server.submitLocks[key] = now
	return true
}

func (server *Server) requiresApprovalForCampaign(userID int64, templateID *int64, body string) bool {
	if !server.customerRequiresApproval(userID) {
		return false
	}
	if templateID == nil || *templateID <= 0 {
		return true
	}

	var tmplBody, approval string
	err := server.db.QueryRow(`
		SELECT body, COALESCE(approval, 'pending')
		FROM templates
		WHERE id = ? AND user_id = ? AND deleted_at IS NULL
	`, *templateID, userID).Scan(&tmplBody, &approval)
	if err != nil || approval != "approved" {
		return true
	}

	return !messageMatchesTemplate(body, tmplBody)
}

func messageMatchesTemplate(body, templateBody string) bool {
	trimmedBody := strings.TrimSpace(body)
	trimmedTemplate := strings.TrimSpace(templateBody)
	if trimmedBody == trimmedTemplate {
		return true
	}

	escaped := regexp.QuoteMeta(trimmedTemplate)
	pattern := templateVarPlaceholderRe.ReplaceAllString(escaped, ".*?")
	re, err := regexp.Compile("(?su)^" + pattern + "$")
	if err != nil {
		return false
	}
	return re.MatchString(trimmedBody)
}

func buildCustomerMessagesWhere(userID int64, context fiber.Ctx) (string, []any) {
	conditions := []string{"m.user_id = ?"}
	args := []any{userID}

	if search := strings.TrimSpace(context.Query("search")); search != "" {
		conditions = append(conditions, "(m.to LIKE ? OR m.body LIKE ? OR s.sender_id LIKE ?)")
		like := "%" + search + "%"
		args = append(args, like, like, like)
	}
	if sender := strings.TrimSpace(context.Query("sender_id")); sender != "" {
		conditions = append(conditions, "(CAST(m.sender_id AS CHAR) = ? OR s.sender_id = ?)")
		args = append(args, sender, sender)
	}
	if status := strings.TrimSpace(context.Query("status")); status != "" && status != "all" {
		parts := strings.Split(status, ",")
		var valid []string
		for _, p := range parts {
			if p = strings.TrimSpace(p); p != "" {
				valid = append(valid, p)
			}
		}
		if len(valid) == 1 {
			conditions = append(conditions, "m.status = ?")
			args = append(args, valid[0])
		} else if len(valid) > 1 {
			placeholders := strings.TrimSuffix(strings.Repeat("?,", len(valid)), ",")
			conditions = append(conditions, "m.status IN ("+placeholders+")")
			for _, v := range valid {
				args = append(args, v)
			}
		}
	}

	dateFrom := strings.TrimSpace(context.Query("date_from"))
	if dateFrom == "" {
		dateFrom = strings.TrimSpace(context.Query("created_from"))
	}
	if dateFrom != "" {
		conditions = append(conditions, "DATE(m.created_at) >= ?")
		args = append(args, dateFrom)
	}

	dateTo := strings.TrimSpace(context.Query("date_to"))
	if dateTo == "" {
		dateTo = strings.TrimSpace(context.Query("created_until"))
	}
	if dateTo != "" {
		conditions = append(conditions, "DATE(m.created_at) <= ?")
		args = append(args, dateTo)
	}

	return "WHERE " + strings.Join(conditions, " AND "), args
}

func looksLikeHeaderRow(row []string) bool {
	if len(row) == 0 {
		return false
	}
	for _, cell := range row {
		trimmed := strings.TrimSpace(cell)
		if trimmed == "" {
			continue
		}
		// Any alphabetic header keyword (phone, name, etc.) indicates a header row.
		for _, r := range trimmed {
			if unicode.IsLetter(r) {
				return true
			}
		}
	}
	return false
}

func readSpreadsheetRows(filePath string) ([][]string, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	var rawRows [][]string

	switch ext {
	case ".csv", ".txt":
		f, openErr := os.Open(filePath)
		if openErr != nil {
			return nil, fmt.Errorf("cannot read uploaded CSV file: %w", openErr)
		}
		defer f.Close()

		reader := csv.NewReader(f)
		reader.FieldsPerRecord = -1
		for {
			rec, rErr := reader.Read()
			if rErr == io.EOF {
				break
			}
			if rErr != nil {
				continue
			}
			cleaned := make([]string, len(rec))
			nonEmpty := false
			for i, cell := range rec {
				val := strings.TrimPrefix(strings.TrimSpace(cell), "\uFEFF")
				cleaned[i] = val
				if val != "" {
					nonEmpty = true
				}
			}
			if nonEmpty {
				rawRows = append(rawRows, cleaned)
			}
		}
	case ".xlsx":
		xf, openErr := excelize.OpenFile(filePath)
		if openErr != nil {
			return nil, fmt.Errorf("cannot read uploaded Excel file: %w", openErr)
		}
		defer xf.Close()

		sheets := xf.GetSheetList()
		if len(sheets) == 0 {
			return nil, fmt.Errorf("uploaded Excel workbook contains no sheets")
		}
		rows, rowsErr := xf.GetRows(sheets[0])
		if rowsErr != nil {
			return nil, fmt.Errorf("failed to read sheet rows: %w", rowsErr)
		}
		for _, rec := range rows {
			cleaned := make([]string, len(rec))
			nonEmpty := false
			for i, cell := range rec {
				val := strings.TrimSpace(cell)
				cleaned[i] = val
				if val != "" {
					nonEmpty = true
				}
			}
			if nonEmpty {
				rawRows = append(rawRows, cleaned)
			}
		}
	case ".xls":
		return nil, fmt.Errorf("legacy .xls binary spreadsheets are not supported; please save your file as .xlsx or .csv")
	default:
		return nil, fmt.Errorf("unsupported file type %s; upload .csv, .txt, or .xlsx", ext)
	}

	if len(rawRows) == 0 {
		return nil, fmt.Errorf("uploaded file contains no rows")
	}
	return rawRows, nil
}

func parseUploadedFilePreview(filePath string, sampleSize int) (headers []string, sample []map[string]string, totalRows int, hasHeaderRow bool, err error) {
	rawRows, err := readSpreadsheetRows(filePath)
	if err != nil {
		return nil, nil, 0, false, err
	}

	firstRow := rawRows[0]
	hasHeaderRow = looksLikeHeaderRow(firstRow)
	headers = make([]string, len(firstRow))
	if hasHeaderRow {
		for i, h := range firstRow {
			if h != "" {
				headers[i] = h
			} else {
				headers[i] = fmt.Sprintf("Column %d", i+1)
			}
		}
	} else {
		for i := range firstRow {
			headers[i] = fmt.Sprintf("Column %d", i+1)
		}
	}

	dataStart := 0
	if hasHeaderRow {
		dataStart = 1
	}

	sample = []map[string]string{}
	for i := dataStart; i < len(rawRows); i++ {
		totalRows++
		if len(sample) < sampleSize {
			rowMap := make(map[string]string, len(headers))
			for j, h := range headers {
				if j < len(rawRows[i]) {
					rowMap[h] = rawRows[i][j]
				} else {
					rowMap[h] = ""
				}
			}
			sample = append(sample, rowMap)
		}
	}

	return headers, sample, totalRows, hasHeaderRow, nil
}

func (server *Server) uploadRecipientPreview(context fiber.Ctx) error {
	fileHeader, err := context.FormFile("file")
	if err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Please select a recipient file to upload."})
	}
	if fileHeader.Size > 100*1024*1024 {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Uploaded file must not exceed 100MB."})
	}

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	switch ext {
	case ".csv", ".txt", ".xlsx", ".xls":
	default:
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Unsupported file type. Upload a CSV, TXT, or XLSX file."})
	}

	relTemp := filepath.ToSlash(filepath.Join("uploads", "recipient-files", uuid.NewString()+ext))
	absTemp := filepath.Join(server.storageRoot(), filepath.FromSlash(relTemp))
	if err := os.MkdirAll(filepath.Dir(absTemp), 0755); err != nil {
		return context.Status(http.StatusInternalServerError).JSON(fiber.Map{"message": "Unable to prepare upload directory."})
	}

	src, err := fileHeader.Open()
	if err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Unable to read uploaded file."})
	}
	defer src.Close()

	dst, err := os.Create(absTemp)
	if err != nil {
		return context.Status(http.StatusInternalServerError).JSON(fiber.Map{"message": "Unable to store uploaded file."})
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		_ = os.Remove(absTemp)
		return context.Status(http.StatusInternalServerError).JSON(fiber.Map{"message": "Failed while saving uploaded file."})
	}
	dst.Close()

	headers, sample, totalRows, hasHeaderRow, parseErr := parseUploadedFilePreview(absTemp, 5)
	if parseErr != nil {
		_ = os.Remove(absTemp)
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": parseErr.Error()})
	}
	if totalRows == 0 {
		_ = os.Remove(absTemp)
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Uploaded file does not contain any recipient rows."})
	}

	return context.JSON(fiber.Map{
		"data": fiber.Map{
			"temp_path":      relTemp,
			"headers":        headers,
			"sample":         sample,
			"total_rows":     totalRows,
			"has_header_row": hasHeaderRow,
		},
	})
}

func (server *Server) exportBatches(context fiber.Ctx) error {
	claims := customer(context)

	where, args := buildCustomerMessagesWhere(claims.UserID, context)
	where += " AND m.batch_id IS NOT NULL"

	format := strings.ToLower(strings.TrimSpace(context.Query("format", "csv")))
	limitClause := ""
	if format == "pdf" {
		limitClause = " LIMIT 500"
	}

	rows, err := server.db.Query(`
		SELECT m.id, m.to, COALESCE(s.sender_id, ''), m.body, m.parts, m.cost, m.status,
		       COALESCE(m.failed_reason, ''), COALESCE(m.operator_name, ''), m.created_at, m.sent_at, m.delivered_at,
		       COALESCE(
		           (SELECT t.balance_before FROM transactions t
		            WHERE t.user_id = m.user_id
		              AND ((t.transactionable_type IN ('message_batch', 'App\\Models\\Batch') AND t.transactionable_id = m.batch_id)
		                OR (t.transactionable_type IN ('message', 'App\\Models\\Message') AND t.transactionable_id = m.id))
		            ORDER BY t.id DESC LIMIT 1),
		           0.00
		       ) as previous_balance,
		       COALESCE(
		           NULLIF(m.sms_type, ''),
		           CASE
		               WHEN b.source_type = 'manual' AND (b.total_recipients = 1 OR b.total_recipients IS NULL) THEN 'Transactional'
		               WHEN b.source_type = 'single' OR m.batch_id IS NULL THEN 'Transactional'
		               ELSE 'Promotional'
		           END
		       ) as sms_type
		FROM messages m
		LEFT JOIN senders s ON s.id = m.sender_id
		LEFT JOIN batches b ON b.id = m.batch_id
		`+where+`
		ORDER BY m.id DESC`+limitClause, args...)
	if err != nil {
		return databaseError(context, err)
	}
	defer rows.Close()

	var (
		buffer  bytes.Buffer
		writer  *csv.Writer
		pdfRows []messagePDFRow
	)
	if format != "pdf" {
		writer = csv.NewWriter(&buffer)
		_ = writer.Write([]string{
			"Message ID", "Recipient Number", "Sender ID", "Content", "SMS Type",
			"Previous Balance (BDT)", "Cost (BDT)", "Parts", "Status", "Failure Reason",
			"Operator", "Submit Time", "Delivery Time",
		})
	}

	for rows.Next() {
		var id int64
		var parts int
		var to, senderID, body, status, reason, operator, smsType string
		var cost, prevBalance float64
		var createdAt time.Time
		var sentAt, deliveredAt *time.Time
		if rows.Scan(&id, &to, &senderID, &body, &parts, &cost, &status, &reason, &operator, &createdAt, &sentAt, &deliveredAt, &prevBalance, &smsType) != nil {
			continue
		}
		if operator == "" {
			operator = operatorFromNumber(to)
		}
		submitted := createdAt.Format("2006-01-02 15:04:05")
		if sentAt != nil {
			submitted = sentAt.Format("2006-01-02 15:04:05")
		}
		delivered := ""
		if deliveredAt != nil {
			delivered = deliveredAt.Format("2006-01-02 15:04:05")
		}
		if format == "pdf" {
			pdfRows = append(pdfRows, messagePDFRow{
				id:        id,
				to:        to,
				senderID:  senderID,
				body:      body,
				parts:     parts,
				cost:      cost,
				status:    status,
				reason:    reason,
				operator:  operator,
				submitted: submitted,
				delivered: delivered,
			})
			continue
		}
		_ = writer.Write([]string{
			fmt.Sprintf("MSG-%06d", id), to, senderID, body, smsType,
			fmt.Sprintf("%.2f", prevBalance), fmt.Sprintf("%.4f", cost), strconv.Itoa(parts),
			status, reason, operator, submitted, delivered,
		})
	}

	stamp := time.Now().Format("2006-01-02-150405")
	if format == "pdf" {
		context.Set("Content-Type", "application/pdf")
		context.Set("Content-Disposition", "attachment; filename=sent-message-logs-"+stamp+".pdf")
		return context.Send(buildMessagesPDF(pdfRows))
	}

	writer.Flush()
	context.Set("Content-Type", "text/csv; charset=utf-8")
	context.Set("Content-Disposition", "attachment; filename=sent-message-logs-"+stamp+".csv")
	return context.Send(buffer.Bytes())
}

func (server *Server) downloadBatchFailedReport(context fiber.Ctx) error {
	claims := customer(context)
	batchID := context.Params("id")

	var relPath *string
	err := server.db.QueryRow(
		"SELECT failed_report_path FROM batches WHERE id = ? AND user_id = ?",
		batchID, claims.UserID,
	).Scan(&relPath)
	if err != nil || relPath == nil || strings.TrimSpace(*relPath) == "" {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Failed recipients report not found."})
	}

	cleanRel := filepath.Clean(filepath.FromSlash(strings.TrimSpace(*relPath)))
	rootAbs, _ := filepath.Abs(server.storageRoot())
	fullPath := cleanRel
	if !filepath.IsAbs(cleanRel) {
		fullPath = filepath.Join(rootAbs, cleanRel)
	}
	if !strings.HasPrefix(fullPath, rootAbs+string(filepath.Separator)) && fullPath != rootAbs {
		return context.Status(http.StatusForbidden).JSON(fiber.Map{"message": "Invalid report path."})
	}

	if _, statErr := os.Stat(fullPath); statErr != nil {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Report file no longer exists on disk."})
	}

	context.Set("Content-Type", "text/csv; charset=utf-8")
	context.Set("Content-Disposition", fmt.Sprintf("attachment; filename=batch-%s-failed-recipients.csv", batchID))
	return context.SendFile(fullPath)
}
