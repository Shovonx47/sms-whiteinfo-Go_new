package httpapi

// tableColumns lists a table's columns with a zero-row probe. Unlike
// information_schema lookups this works on both MySQL and PostgreSQL and needs
// no database name. An unknown table returns an empty map.
func tableColumns(server *Server, name string) map[string]bool {
	columns := map[string]bool{}
	if !safeTableName.MatchString(name) {
		return columns
	}

	rows, err := server.db.Query("SELECT * FROM " + name + " WHERE 1 = 0")
	if err != nil {
		return columns
	}
	defer rows.Close()

	names, err := rows.Columns()
	if err != nil {
		return columns
	}
	for _, column := range names {
		columns[column] = true
	}

	return columns
}

// textColumn returns a qualified column reference, or NULL when the column is
// absent, so a query keeps working on an older schema.
func textColumn(columns map[string]bool, table, column string) string {
	if columns[column] {
		return table + "." + column
	}

	return "NULL"
}

// flagColumn renders a boolean column as a portable 0/1 integer. PostgreSQL
// rejects COALESCE(boolean, 0), so the CASE avoids the type mismatch.
func flagColumn(columns map[string]bool, table, column string) string {
	if columns[column] {
		return "CASE WHEN COALESCE(" + table + "." + column + ", FALSE) THEN 1 ELSE 0 END"
	}

	return "0"
}
