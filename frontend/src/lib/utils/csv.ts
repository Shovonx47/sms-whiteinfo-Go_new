/** Parses CSV text (RFC 4180 style: quoted fields, escaped quotes, CRLF/LF). */
export function parseCsv(text: string): string[][] {
	const rows: string[][] = [];
	let row: string[] = [];
	let field = '';
	let inQuotes = false;

	const source = text.replace(/^\uFEFF/, '');
	for (let i = 0; i < source.length; i++) {
		const ch = source[i];
		if (inQuotes) {
			if (ch === '"') {
				if (source[i + 1] === '"') {
					field += '"';
					i++;
				} else {
					inQuotes = false;
				}
			} else {
				field += ch;
			}
			continue;
		}

		if (ch === '"') {
			inQuotes = true;
		} else if (ch === ',') {
			row.push(field);
			field = '';
		} else if (ch === '\n' || ch === '\r') {
			if (ch === '\r' && source[i + 1] === '\n') i++;
			row.push(field);
			field = '';
			if (row.some((cell) => cell.trim() !== '')) rows.push(row);
			row = [];
		} else {
			field += ch;
		}
	}

	row.push(field);
	if (row.some((cell) => cell.trim() !== '')) rows.push(row);

	return rows.map((cells) => cells.map((cell) => cell.trim()));
}
