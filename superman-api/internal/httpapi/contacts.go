package httpapi

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

type contactItem struct {
	ID        int64           `json:"id"`
	UserID    *int64          `json:"user_id"`
	UserName  *string         `json:"user_name"`
	Name      string          `json:"name"`
	Phone     string          `json:"phone"`
	Email     *string         `json:"email"`
	GroupName *string         `json:"group_name"`
	Meta      json.RawMessage `json:"meta,omitempty"`
	CreatedAt string          `json:"created_at"`
	UpdatedAt *string         `json:"updated_at"`
	DeletedAt *string         `json:"deleted_at"`
}

func firstExistingTable(server *Server, names ...string) string {
	for _, name := range names {
		if tableExists(server, name) {
			return name
		}
	}

	return ""
}

func inPlaceholders(count int) string {
	return strings.TrimSuffix(strings.Repeat("?,", count), ",")
}

func idArgs(ids []int64) []any {
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}

	return args
}

// userLabelColumn picks the best display column for a user row, matching the
// old relationship('user', 'name') selects.
func userLabelColumn(columns map[string]bool, table string) string {
	switch {
	case columns["name"]:
		return "COALESCE(" + table + ".name, '')"
	case columns["email"]:
		return "COALESCE(" + table + ".email, '')"
	}

	return "''"
}

var bangladeshPhone = regexp.MustCompile(`^(?:\+?88)?0?(1[3-9]\d{8})$`)

// normalizeContactPhone mirrors the E164 cast in the old Contact model, which
// stored Bangladeshi numbers as +8801XXXXXXXXX.
func normalizeContactPhone(raw string) string {
	cleaned := strings.Map(func(r rune) rune {
		if (r >= '0' && r <= '9') || r == '+' {
			return r
		}

		return -1
	}, raw)
	if matches := bangladeshPhone.FindStringSubmatch(cleaned); matches != nil {
		return "+880" + matches[1]
	}

	return cleaned
}

// contacts mirrors the old Filament Audience > Contacts table: owner, name,
// phone and creation date, with search, sorting, a trashed filter, paging and
// export.
func (server *Server) contacts(context fiber.Ctx) error {
	page, perPage := pagination(context)
	if context.Query("export") == "1" {
		page, perPage = 1, 20000
	}
	search := strings.TrimSpace(context.Query("search"))

	empty := func() error {
		return listResponse(context, []contactItem{}, listMeta{Page: page, PerPage: perPage, Total: 0})
	}

	columns := tableColumns(server, "contacts")
	if len(columns) == 0 {
		log.Printf("contacts: contacts table is not readable")
		return empty()
	}

	ownerJoin := ""
	ownerName := "NULL"
	ownerSearch := ""
	if tableExists(server, "users") {
		userColumns := tableColumns(server, "users")
		if userColumns["id"] {
			ownerJoin = " LEFT JOIN users u ON u.id = c.user_id"
			ownerName = userLabelColumn(userColumns, "u")
			ownerSearch = " OR u.name LIKE ?"
			if !userColumns["name"] {
				ownerSearch = ""
			}
		}
	}

	groupTable := firstExistingTable(server, "contact_group", "contact_groups")
	groupExpr := "NULL"
	if groupTable != "" && tableExists(server, "groups") {
		groupExpr = "(SELECT g.name FROM " + groupTable + " cg JOIN `groups` g ON g.id = cg.group_id WHERE cg.contact_id = c.id LIMIT 1)"
	}

	deletedFilter := "WHERE 1 = 1"
	if columns["deleted_at"] {
		deletedFilter = "WHERE c.deleted_at IS NULL"
	}
	where := deletedFilter
	if trashed := strings.TrimSpace(context.Query("trashed")); trashed != "" && columns["deleted_at"] {
		switch trashed {
		case "only":
			where = "WHERE c.deleted_at IS NOT NULL"
		case "with":
			where = "WHERE 1 = 1"
		}
	}

	args := []any{}
	if search != "" {
		where += " AND (c.name LIKE ? OR c.phone LIKE ?" + ownerSearch + ")"
		term := "%" + search + "%"
		args = append(args, term, term)
		if ownerSearch != "" {
			args = append(args, term)
		}
	}

	var total int
	if err := server.db.QueryRow("SELECT COUNT(*) FROM contacts c"+ownerJoin+" "+where, args...).Scan(&total); err != nil {
		log.Printf("contacts count failed: %v", err)
		return empty()
	}

	selectColumns := []string{"c.id", textColumn(columns, "c", "user_id"), "COALESCE(c.name, ''), COALESCE(c.phone, '')", textColumn(columns, "c", "email"), ownerName}
	selectColumns = append(selectColumns, groupExpr, textColumn(columns, "c", "meta"))
	selectColumns = append(selectColumns, textColumn(columns, "c", "created_at"), textColumn(columns, "c", "updated_at"), textColumn(columns, "c", "deleted_at"))

	// Filament sortable columns: user.name, name, phone, created_at.
	direction := "ASC"
	if strings.EqualFold(context.Query("dir"), "desc") {
		direction = "DESC"
	}
	order := ""
	switch strings.TrimSpace(context.Query("sort")) {
	case "user", "user.name":
		if ownerJoin != "" {
			order = " ORDER BY " + ownerName + " " + direction
		}
	case "name":
		if columns["name"] {
			order = " ORDER BY c.name " + direction
		}
	case "phone":
		if columns["phone"] {
			order = " ORDER BY c.phone " + direction
		}
	case "created_at":
		if columns["created_at"] {
			order = " ORDER BY c.created_at " + direction
		}
	}
	if order == "" {
		if columns["created_at"] {
			order = " ORDER BY c.created_at DESC"
		} else {
			order = " ORDER BY c.id DESC"
		}
	}
	order += ", c.id DESC"

	rows, err := server.db.Query(`
		SELECT `+strings.Join(selectColumns, ", ")+`
		FROM contacts c`+ownerJoin+`
		`+where+order+`
		LIMIT ? OFFSET ?
	`, append(args, perPage, (page-1)*perPage)...)
	if err != nil {
		log.Printf("contacts query failed: %v", err)
		return empty()
	}
	defer rows.Close()

	items := make([]contactItem, 0)
	for rows.Next() {
		var item contactItem
		var userID sql.NullInt64
		var email, owner, groupName, meta sql.NullString
		var createdAt, updatedAt, deletedAt sql.NullTime
		if err := rows.Scan(&item.ID, &userID, &item.Name, &item.Phone, &email, &owner, &groupName, &meta, &createdAt, &updatedAt, &deletedAt); err != nil {
			log.Printf("contacts scan failed: %v", err)
			return empty()
		}
		if userID.Valid {
			value := userID.Int64
			item.UserID = &value
		}
		if email.Valid && email.String != "" {
			value := email.String
			item.Email = &value
		}
		if owner.Valid && owner.String != "" {
			value := owner.String
			item.UserName = &value
		}
		if groupName.Valid && groupName.String != "" {
			value := groupName.String
			item.GroupName = &value
		}
		if meta.Valid && meta.String != "" {
			item.Meta = json.RawMessage(meta.String)
		}
		if createdAt.Valid {
			item.CreatedAt = createdAt.Time.Format(time.RFC3339)
		}
		if updatedAt.Valid {
			value := updatedAt.Time.Format(time.RFC3339)
			item.UpdatedAt = &value
		}
		if deletedAt.Valid {
			value := deletedAt.Time.Format(time.RFC3339)
			item.DeletedAt = &value
		}
		items = append(items, item)
	}

	return listResponse(context, items, listMeta{Page: page, PerPage: perPage, Total: total})
}

type contactPayload struct {
	UserID  int64           `json:"user_id"`
	Name    string          `json:"name"`
	Phone   string          `json:"phone"`
	Email   string          `json:"email"`
	GroupID *int64          `json:"group_id"`
	Meta    json.RawMessage `json:"meta"`
}

func (server *Server) createContact(context fiber.Ctx) error {
	payload := contactPayload{}
	if err := json.Unmarshal(context.Body(), &payload); err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request payload."})
	}
	payload.Phone = normalizeContactPhone(strings.TrimSpace(payload.Phone))
	payload.Name = strings.TrimSpace(payload.Name)
	if payload.Phone == "" {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "The phone field is required."})
	}
	if payload.UserID < 1 {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "The owner field is required."})
	}

	columns := tableColumns(server, "contacts")
	if len(columns) == 0 {
		return context.Status(http.StatusServiceUnavailable).JSON(fiber.Map{"message": "Contacts are unavailable right now."})
	}

	names := []string{"user_id", "name", "phone"}
	values := []any{payload.UserID, payload.Name, payload.Phone}
	if columns["email"] {
		names = append(names, "email")
		values = append(values, payload.Email)
	}
	if columns["meta"] && len(payload.Meta) > 0 {
		names = append(names, "meta")
		values = append(values, string(payload.Meta))
	}
	if columns["created_at"] {
		names = append(names, "created_at")
		values = append(values, time.Now())
	}
	if columns["updated_at"] {
		names = append(names, "updated_at")
		values = append(values, time.Now())
	}

	result, err := server.db.Exec("INSERT INTO contacts ("+strings.Join(names, ", ")+") VALUES ("+inPlaceholders(len(values))+")", values...)
	if err != nil {
		return databaseError(context, err)
	}

	id, _ := result.LastInsertId()
	if payload.GroupID != nil && *payload.GroupID > 0 {
		groupTable := firstExistingTable(server, "contact_group", "contact_groups")
		if groupTable != "" {
			groupColumns := tableColumns(server, groupTable)
			if groupColumns["contact_id"] && groupColumns["group_id"] {
				if groupColumns["created_at"] {
					_, _ = server.db.Exec("INSERT INTO "+groupTable+" (contact_id, group_id, created_at) VALUES (?, ?, ?)", id, *payload.GroupID, time.Now())
				} else {
					_, _ = server.db.Exec("INSERT INTO "+groupTable+" (contact_id, group_id) VALUES (?, ?)", id, *payload.GroupID)
				}
			}
		}
	}

	return context.Status(http.StatusCreated).JSON(fiber.Map{
		"message": "Contact created successfully.",
		"data":    fiber.Map{"id": id},
	})
}

func (server *Server) updateContact(context fiber.Ctx) error {
	id, err := strconv.ParseInt(context.Params("id"), 10, 64)
	if err != nil || id < 1 {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Contact not found."})
	}

	payload := contactPayload{}
	if err := json.Unmarshal(context.Body(), &payload); err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request payload."})
	}
	payload.Phone = normalizeContactPhone(strings.TrimSpace(payload.Phone))
	payload.Name = strings.TrimSpace(payload.Name)
	if payload.Phone == "" {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "The phone field is required."})
	}

	columns := tableColumns(server, "contacts")
	if len(columns) == 0 {
		return context.Status(http.StatusServiceUnavailable).JSON(fiber.Map{"message": "Contacts are unavailable right now."})
	}

	sets := []string{"name = ?", "phone = ?"}
	values := []any{payload.Name, payload.Phone}
	if payload.UserID > 0 {
		sets = append(sets, "user_id = ?")
		values = append(values, payload.UserID)
	}
	if columns["email"] {
		sets = append(sets, "email = ?")
		values = append(values, payload.Email)
	}
	if columns["meta"] && len(payload.Meta) > 0 {
		sets = append(sets, "meta = ?")
		values = append(values, string(payload.Meta))
	}
	if columns["updated_at"] {
		sets = append(sets, "updated_at = ?")
		values = append(values, time.Now())
	}
	values = append(values, id)

	if _, err := server.db.Exec("UPDATE contacts SET "+strings.Join(sets, ", ")+" WHERE id = ?", values...); err != nil {
		return databaseError(context, err)
	}

	return context.JSON(fiber.Map{"message": "Contact updated successfully."})
}

func (server *Server) deleteContact(context fiber.Ctx) error {
	id, err := strconv.ParseInt(context.Params("id"), 10, 64)
	if err != nil || id < 1 {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Contact not found."})
	}

	columns := tableColumns(server, "contacts")
	if columns["deleted_at"] {
		_, err = server.db.Exec("UPDATE contacts SET deleted_at = ? WHERE id = ?", time.Now(), id)
	} else {
		_, err = server.db.Exec("DELETE FROM contacts WHERE id = ?", id)
	}
	if err != nil {
		return databaseError(context, err)
	}

	return context.JSON(fiber.Map{"message": "Contact deleted successfully."})
}

type audienceBulkPayload struct {
	IDs    []int64 `json:"ids"`
	Action string  `json:"action"`
}

// bulkContacts implements the Filament Delete / Restore / Force-delete bulk
// actions of the old Audience > Contacts table.
func (server *Server) bulkContacts(context fiber.Ctx) error {
	var payload audienceBulkPayload
	if err := json.Unmarshal(context.Body(), &payload); err != nil || len(payload.IDs) == 0 {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Select at least one contact."})
	}

	columns := tableColumns(server, "contacts")
	if len(columns) == 0 {
		return context.Status(http.StatusServiceUnavailable).JSON(fiber.Map{"message": "Contacts are unavailable right now."})
	}

	placeholders := inPlaceholders(len(payload.IDs))
	var query, message string
	switch payload.Action {
	case "delete":
		if columns["deleted_at"] {
			query = "UPDATE contacts SET deleted_at = NOW() WHERE deleted_at IS NULL AND id IN (" + placeholders + ")"
		} else {
			query = "DELETE FROM contacts WHERE id IN (" + placeholders + ")"
		}
		message = "Selected contacts deleted."
	case "restore":
		if !columns["deleted_at"] {
			return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Nothing to restore."})
		}
		query = "UPDATE contacts SET deleted_at = NULL WHERE deleted_at IS NOT NULL AND id IN (" + placeholders + ")"
		message = "Selected contacts restored."
	case "force_delete":
		if !columns["deleted_at"] {
			query = "DELETE FROM contacts WHERE id IN (" + placeholders + ")"
		} else {
			query = "DELETE FROM contacts WHERE deleted_at IS NOT NULL AND id IN (" + placeholders + ")"
		}
		message = "Selected contacts permanently deleted."
	default:
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Unknown bulk action."})
	}

	result, err := server.db.Exec(query, idArgs(payload.IDs)...)
	if err != nil {
		return databaseError(context, err)
	}
	affected, _ := result.RowsAffected()

	return context.JSON(fiber.Map{"message": message, "affected": affected})
}

type contactGroupItem struct {
	ID            int64   `json:"id"`
	UserID        *int64  `json:"user_id"`
	UserName      *string `json:"user_name"`
	Name          string  `json:"name"`
	Description   *string `json:"description"`
	ContactsCount int64   `json:"contacts_count"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     *string `json:"updated_at"`
	DeletedAt     *string `json:"deleted_at"`
}

// contactGroups mirrors the old Filament Audience > Contact Groups table:
// owner, name and creation date, with search, sorting, trashed filter, paging
// and the contact count used by the old table's record relation.
func (server *Server) contactGroups(context fiber.Ctx) error {
	page, perPage := pagination(context)
	if context.Query("export") == "1" {
		page, perPage = 1, 20000
	}
	search := strings.TrimSpace(context.Query("search"))

	empty := func() error {
		return listResponse(context, []contactGroupItem{}, listMeta{Page: page, PerPage: perPage, Total: 0})
	}

	columns := tableColumns(server, "groups")
	if len(columns) == 0 {
		log.Printf("contact groups: groups table is not readable")
		return empty()
	}

	ownerJoin := ""
	ownerName := "NULL"
	ownerSearch := ""
	if tableExists(server, "users") {
		userColumns := tableColumns(server, "users")
		if userColumns["id"] {
			ownerJoin = " LEFT JOIN users u ON u.id = g.user_id"
			ownerName = userLabelColumn(userColumns, "u")
			if userColumns["name"] {
				ownerSearch = " OR u.name LIKE ?"
			}
		}
	}

	countExpr := "0"
	groupTable := firstExistingTable(server, "contact_group", "contact_groups")
	if groupTable != "" {
		countExpr = "(SELECT COUNT(*) FROM " + groupTable + " cg WHERE cg.group_id = g.id)"
	}

	where := "WHERE 1 = 1"
	if columns["deleted_at"] {
		where = "WHERE g.deleted_at IS NULL"
	}
	if trashed := strings.TrimSpace(context.Query("trashed")); trashed != "" && columns["deleted_at"] {
		switch trashed {
		case "only":
			where = "WHERE g.deleted_at IS NOT NULL"
		case "with":
			where = "WHERE 1 = 1"
		}
	}
	args := []any{}
	if search != "" {
		where += " AND (g.name LIKE ?" + ownerSearch + ")"
		term := "%" + search + "%"
		args = append(args, term)
		if ownerSearch != "" {
			args = append(args, term)
		}
	}

	var total int
	if err := server.db.QueryRow("SELECT COUNT(*) FROM `groups` g"+ownerJoin+" "+where, args...).Scan(&total); err != nil {
		log.Printf("contact groups count failed: %v", err)
		return empty()
	}

	direction := "ASC"
	if strings.EqualFold(context.Query("dir"), "desc") {
		direction = "DESC"
	}
	order := ""
	switch strings.TrimSpace(context.Query("sort")) {
	case "user", "user.name":
		if ownerJoin != "" {
			order = " ORDER BY " + ownerName + " " + direction
		}
	case "name":
		if columns["name"] {
			order = " ORDER BY g.name " + direction
		}
	case "created_at":
		if columns["created_at"] {
			order = " ORDER BY g.created_at " + direction
		}
	}
	if order == "" {
		if columns["created_at"] {
			order = " ORDER BY g.created_at DESC"
		} else {
			order = " ORDER BY g.id DESC"
		}
	}
	order += ", g.id DESC"

	rows, err := server.db.Query(`
		SELECT g.id, `+textColumn(columns, "g", "user_id")+`, COALESCE(g.name, ''), `+ownerName+`,
			`+textColumn(columns, "g", "description")+`, `+countExpr+`,
			`+textColumn(columns, "g", "created_at")+`, `+textColumn(columns, "g", "updated_at")+`, `+textColumn(columns, "g", "deleted_at")+`
		FROM `+"`groups`"+` g`+ownerJoin+`
		`+where+order+`
		LIMIT ? OFFSET ?
	`, append(args, perPage, (page-1)*perPage)...)
	if err != nil {
		log.Printf("contact groups query failed: %v", err)
		return empty()
	}
	defer rows.Close()

	items := make([]contactGroupItem, 0)
	for rows.Next() {
		var item contactGroupItem
		var userID sql.NullInt64
		var owner, description sql.NullString
		var createdAt, updatedAt, deletedAt sql.NullTime
		if err := rows.Scan(&item.ID, &userID, &item.Name, &owner, &description, &item.ContactsCount, &createdAt, &updatedAt, &deletedAt); err != nil {
			log.Printf("contact groups scan failed: %v", err)
			return empty()
		}
		if userID.Valid {
			value := userID.Int64
			item.UserID = &value
		}
		if owner.Valid && owner.String != "" {
			value := owner.String
			item.UserName = &value
		}
		if description.Valid && description.String != "" {
			value := description.String
			item.Description = &value
		}
		if createdAt.Valid {
			item.CreatedAt = createdAt.Time.Format(time.RFC3339)
		}
		if updatedAt.Valid {
			value := updatedAt.Time.Format(time.RFC3339)
			item.UpdatedAt = &value
		}
		if deletedAt.Valid {
			value := deletedAt.Time.Format(time.RFC3339)
			item.DeletedAt = &value
		}
		items = append(items, item)
	}

	return listResponse(context, items, listMeta{Page: page, PerPage: perPage, Total: total})
}

type contactGroupPayload struct {
	UserID      int64  `json:"user_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (server *Server) createContactGroup(context fiber.Ctx) error {
	payload := contactGroupPayload{}
	if err := json.Unmarshal(context.Body(), &payload); err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request payload."})
	}
	payload.Name = strings.TrimSpace(payload.Name)
	if payload.Name == "" {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "The name field is required."})
	}
	if payload.UserID < 1 {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "The owner field is required."})
	}

	columns := tableColumns(server, "groups")
	if len(columns) == 0 {
		return context.Status(http.StatusServiceUnavailable).JSON(fiber.Map{"message": "Contact groups are unavailable right now."})
	}

	names := []string{"user_id", "name"}
	values := []any{payload.UserID, payload.Name}
	if columns["description"] {
		names = append(names, "description")
		values = append(values, payload.Description)
	}
	if columns["created_at"] {
		names = append(names, "created_at")
		values = append(values, time.Now())
	}
	if columns["updated_at"] {
		names = append(names, "updated_at")
		values = append(values, time.Now())
	}

	result, err := server.db.Exec("INSERT INTO `groups` ("+strings.Join(names, ", ")+") VALUES ("+inPlaceholders(len(values))+")", values...)
	if err != nil {
		return databaseError(context, err)
	}
	id, _ := result.LastInsertId()

	return context.Status(http.StatusCreated).JSON(fiber.Map{
		"message": "Contact group created successfully.",
		"data":    fiber.Map{"id": id},
	})
}

func (server *Server) updateContactGroup(context fiber.Ctx) error {
	id, err := strconv.ParseInt(context.Params("id"), 10, 64)
	if err != nil || id < 1 {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Contact group not found."})
	}

	payload := contactGroupPayload{}
	if err := json.Unmarshal(context.Body(), &payload); err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request payload."})
	}
	payload.Name = strings.TrimSpace(payload.Name)
	if payload.Name == "" {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "The name field is required."})
	}

	columns := tableColumns(server, "groups")
	if len(columns) == 0 {
		return context.Status(http.StatusServiceUnavailable).JSON(fiber.Map{"message": "Contact groups are unavailable right now."})
	}

	sets := []string{"name = ?"}
	values := []any{payload.Name}
	if payload.UserID > 0 {
		sets = append(sets, "user_id = ?")
		values = append(values, payload.UserID)
	}
	if columns["description"] {
		sets = append(sets, "description = ?")
		values = append(values, payload.Description)
	}
	if columns["updated_at"] {
		sets = append(sets, "updated_at = ?")
		values = append(values, time.Now())
	}
	values = append(values, id)

	if _, err := server.db.Exec("UPDATE `groups` SET "+strings.Join(sets, ", ")+" WHERE id = ?", values...); err != nil {
		return databaseError(context, err)
	}

	return context.JSON(fiber.Map{"message": "Contact group updated successfully."})
}

func (server *Server) deleteContactGroup(context fiber.Ctx) error {
	id, err := strconv.ParseInt(context.Params("id"), 10, 64)
	if err != nil || id < 1 {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Contact group not found."})
	}

	columns := tableColumns(server, "groups")
	if columns["deleted_at"] {
		_, err = server.db.Exec("UPDATE `groups` SET deleted_at = ? WHERE id = ?", time.Now(), id)
	} else {
		_, err = server.db.Exec("DELETE FROM `groups` WHERE id = ?", id)
	}
	if err != nil {
		return databaseError(context, err)
	}

	return context.JSON(fiber.Map{"message": "Contact group deleted successfully."})
}

// bulkContactGroups implements the Filament Delete / Restore / Force-delete
// bulk actions of the old Audience > Contact Groups table.
func (server *Server) bulkContactGroups(context fiber.Ctx) error {
	var payload audienceBulkPayload
	if err := json.Unmarshal(context.Body(), &payload); err != nil || len(payload.IDs) == 0 {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Select at least one contact group."})
	}

	columns := tableColumns(server, "groups")
	if len(columns) == 0 {
		return context.Status(http.StatusServiceUnavailable).JSON(fiber.Map{"message": "Contact groups are unavailable right now."})
	}

	placeholders := inPlaceholders(len(payload.IDs))
	var query, message string
	switch payload.Action {
	case "delete":
		if columns["deleted_at"] {
			query = "UPDATE `groups` SET deleted_at = NOW() WHERE deleted_at IS NULL AND id IN (" + placeholders + ")"
		} else {
			query = "DELETE FROM `groups` WHERE id IN (" + placeholders + ")"
		}
		message = "Selected contact groups deleted."
	case "restore":
		if !columns["deleted_at"] {
			return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Nothing to restore."})
		}
		query = "UPDATE `groups` SET deleted_at = NULL WHERE deleted_at IS NOT NULL AND id IN (" + placeholders + ")"
		message = "Selected contact groups restored."
	case "force_delete":
		if !columns["deleted_at"] {
			query = "DELETE FROM `groups` WHERE id IN (" + placeholders + ")"
		} else {
			query = "DELETE FROM `groups` WHERE deleted_at IS NOT NULL AND id IN (" + placeholders + ")"
		}
		message = "Selected contact groups permanently deleted."
	default:
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Unknown bulk action."})
	}

	result, err := server.db.Exec(query, idArgs(payload.IDs)...)
	if err != nil {
		return databaseError(context, err)
	}
	affected, _ := result.RowsAffected()

	return context.JSON(fiber.Map{"message": message, "affected": affected})
}

type ownerOption struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// contactOwners feeds the Owner select of the contact and group forms. It reads
// the users table defensively so a schema without deleted_at or name still
// returns the owner list instead of an empty select.
func (server *Server) contactOwners(context fiber.Ctx) error {
	items := make([]ownerOption, 0)
	columns := tableColumns(server, "users")
	if len(columns) == 0 || !columns["id"] {
		return context.JSON(fiber.Map{"data": items})
	}

	where := ""
	if columns["deleted_at"] {
		where = " WHERE deleted_at IS NULL"
	}
	order := " ORDER BY id ASC"
	if columns["name"] {
		order = " ORDER BY name ASC"
	}

	rows, err := server.db.Query(`
		SELECT id, ` + userLabelColumn(columns, "users") + `
		FROM users` + where + order + `
		LIMIT 1000
	`)
	if err != nil {
		log.Printf("contact owners failed: %v", err)
		return context.JSON(fiber.Map{"data": items})
	}
	defer rows.Close()

	for rows.Next() {
		var item ownerOption
		if err := rows.Scan(&item.ID, &item.Name); err == nil {
			items = append(items, item)
		}
	}

	return context.JSON(fiber.Map{"data": items})
}
