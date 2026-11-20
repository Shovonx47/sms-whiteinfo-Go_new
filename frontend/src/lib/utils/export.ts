import { jsPDF } from 'jspdf';
import autoTable from 'jspdf-autotable';

export interface ExportColumn<T = any> {
	header: string;
	key?: keyof T | string;
	getValue?: (row: T) => any;
}

export interface ExportOptions {
	title: string;
	filename?: string;
	filterSummary?: string;
	/** Optional label/value tiles printed above the table (PDF only). */
	summary?: { label: string; value: string }[];
}

/**
 * Clean and format cell value to plain string
 */
export function formatCellValue(raw: any, forCsv = false): string {
	if (raw === null || raw === undefined) return '';

	// If it's a date object
	if (raw instanceof Date) {
		return raw.toLocaleString();
	}

	let str = String(raw).trim();

	// Strip HTML tags if any exist
	if (str.includes('<') && str.includes('>')) {
		str = str.replace(/<[^>]*>/g, '').trim();
	}

	return str;
}

/**
 * Format string for safe and clean CSV export in Excel
 */
export function formatCsvCell(str: string): string {
	if (!str) return '""';

	// Replace Taka symbol with BDT for maximum compatibility
	const cleanStr = str.replace(/৳/g, 'BDT ').trim();

	// 1. Check if it's a long number or phone number (e.g. 8809640913002 or +8801713257258)
	// In Excel, long numbers get converted to exponential scientific notation (e.g. 8.8096E+12).
	// To prevent this without showing formula errors, format as ="<number>"
	const isPhoneWithPlus = /^\+\d{8,20}$/.test(cleanStr);
	const isLongDigits = /^\d{10,30}$/.test(cleanStr);

	if (isPhoneWithPlus || isLongDigits) {
		return `="${cleanStr}"`;
	}

	// 2. Guard against CSV formula injection
	// If text starts with =, +, -, @ and is not a plain number
	let sanitized = cleanStr;
	const startsWithDangerous = /^[=+\-@]/.test(sanitized);
	const isPlainNumber = /^[+\-]?\d+(\.\d+)?$/.test(sanitized);

	if (startsWithDangerous && !isPlainNumber) {
		sanitized = `'${sanitized}`;
	}

	// 3. Escape internal double quotes by doubling them
	const escaped = sanitized.replace(/"/g, '""');
	return `"${escaped}"`;
}

/**
 * Generate formatted filename with timestamp
 * e.g. "gateways_2026-10-31_14-58.csv"
 */
export function generateFilename(title: string, extension: 'csv' | 'pdf' | 'xls'): string {
	const sanitized = title
		.toLowerCase()
		.replace(/[^a-z0-9]+/g, '-')
		.replace(/^-+|-+$/g, '');

	const now = new Date();
	const y = now.getFullYear();
	const m = String(now.getMonth() + 1).padStart(2, '0');
	const d = String(now.getDate()).padStart(2, '0');
	const h = String(now.getHours()).padStart(2, '0');
	const min = String(now.getMinutes()).padStart(2, '0');

	const timestamp = `${y}-${m}-${d}_${h}-${min}`;
	return `${sanitized || 'export'}_${timestamp}.${extension}`;
}

/**
 * Export table data to CSV file and trigger download
 */
export function exportToCSV<T>(
	columns: ExportColumn<T>[],
	data: T[],
	options: ExportOptions
): void {
	if (!data || data.length === 0) {
		throw new Error('No data available to export.');
	}

	// Filter out actions or empty headers
	const validColumns = columns.filter((col) => col.header && col.header.toLowerCase() !== 'actions');

	const headerRow = validColumns.map((col) => formatCsvCell(col.header)).join(',');

	const dataRows = data.map((row) => {
		return validColumns
			.map((col) => {
				let rawValue: any;
				if (col.getValue) {
					rawValue = col.getValue(row);
				} else if (col.key && typeof row === 'object' && row !== null) {
					rawValue = (row as any)[col.key];
				} else {
					rawValue = '';
				}
				const formatted = formatCellValue(rawValue, true);
				return formatCsvCell(formatted);
			})
			.join(',');
	});

	// Prepend UTF-8 BOM (\uFEFF) so Excel parses Bengali and special characters correctly
	const csvContent = '\uFEFF' + [headerRow, ...dataRows].join('\r\n');
	const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
	const filename = options.filename || generateFilename(options.title, 'csv');

	const link = document.createElement('a');
	const url = URL.createObjectURL(blob);
	link.setAttribute('href', url);
	link.setAttribute('download', filename);
	document.body.appendChild(link);
	link.click();
	document.body.removeChild(link);
	URL.revokeObjectURL(url);
}

function escapeXml(value: string): string {
	return value
		.replace(/&/g, '&amp;')
		.replace(/</g, '&lt;')
		.replace(/>/g, '&gt;')
		.replace(/"/g, '&quot;')
		.replace(/'/g, '&apos;');
}

/**
 * Export table data to an Excel-compatible SpreadsheetML workbook (.xls)
 */
export function exportToExcel<T>(
	columns: ExportColumn<T>[],
	data: T[],
	options: ExportOptions
): void {
	if (!data || data.length === 0) {
		throw new Error('No data available to export.');
	}

	const validColumns = columns.filter((col) => col.header && col.header.toLowerCase() !== 'actions');

	const headerXml = validColumns
		.map((col) => `<Cell ss:StyleID="Header"><Data ss:Type="String">${escapeXml(col.header)}</Data></Cell>`)
		.join('');

	const rowsXml = data
		.map((row) => {
			const cells = validColumns
				.map((col) => {
					const raw = col.getValue
						? col.getValue(row)
						: col.key && typeof row === 'object' && row !== null
							? (row as any)[col.key]
							: '';
					const text = formatCellValue(raw, true).replace(/৳/g, 'BDT ');
					return `<Cell><Data ss:Type="String">${escapeXml(text)}</Data></Cell>`;
				})
				.join('');
			return `<Row>${cells}</Row>`;
		})
		.join('');

	const sheetName = escapeXml((options.title || 'Report').replace(/[\\/?*\[\]:]/g, ' ').trim().slice(0, 31) || 'Report');
	const xml = `<?xml version="1.0" encoding="UTF-8"?>
<?mso-application progid="Excel.Sheet"?>
<Workbook xmlns="urn:schemas-microsoft-com:office:spreadsheet"
 xmlns:ss="urn:schemas-microsoft-com:office:spreadsheet">
 <Styles>
  <Style ss:ID="Header">
   <Font ss:Bold="1" ss:Color="#111827"/>
   <Interior ss:Color="#F3F4F6" ss:Pattern="Solid"/>
  </Style>
 </Styles>
 <Worksheet ss:Name="${sheetName}">
  <Table>
   <Row>${headerXml}</Row>
   ${rowsXml}
  </Table>
 </Worksheet>
</Workbook>`;

	const blob = new Blob(['\uFEFF' + xml], { type: 'application/vnd.ms-excel;charset=utf-8;' });
	const filename = options.filename || generateFilename(options.title, 'xls');

	const link = document.createElement('a');
	const url = URL.createObjectURL(blob);
	link.setAttribute('href', url);
	link.setAttribute('download', filename);
	document.body.appendChild(link);
	link.click();
	document.body.removeChild(link);
	URL.revokeObjectURL(url);
}

/**
 * Export table data to PDF file and trigger download
 */
export function exportToPDF<T>(
	columns: ExportColumn<T>[],
	data: T[],
	options: ExportOptions
): void {
	if (!data || data.length === 0) {
		throw new Error('No data available to export.');
	}

	const validColumns = columns.filter((col) => col.header && col.header.toLowerCase() !== 'actions');

	// Create Landscape A4 document
	const doc = new jsPDF({
		orientation: 'landscape',
		unit: 'pt',
		format: 'a4'
	});

	const pageWidth = doc.internal.pageSize.getWidth();
	const pageHeight = doc.internal.pageSize.getHeight();

	// Title
	doc.setFontSize(14);
	doc.setFont('helvetica', 'bold');
	doc.setTextColor(17, 24, 39); // Gray 900
	doc.text(options.title, 40, 36);

	// Subtitle / Metadata
	doc.setFontSize(8.5);
	doc.setFont('helvetica', 'normal');
	doc.setTextColor(107, 114, 128); // Gray 500
	const now = new Date().toLocaleString();
	let metaText = `Generated on ${now} | Total records: ${data.length}`;
	if (options.filterSummary) {
		metaText += ` | Filters: ${options.filterSummary}`;
	}
	doc.text(metaText, 40, 50);

	// Top line divider
	doc.setDrawColor(229, 231, 235);
	doc.setLineWidth(0.75);
	doc.line(40, 58, pageWidth - 40, 58);

	// Optional summary tiles
	let tableStartY = 68;
	if (options.summary && options.summary.length > 0) {
		const tiles = options.summary;
		const gap = 8;
		const tileWidth = (pageWidth - 80 - gap * (tiles.length - 1)) / tiles.length;
		tiles.forEach((tile, index) => {
			const x = 40 + index * (tileWidth + gap);
			doc.setDrawColor(229, 231, 235);
			doc.setFillColor(249, 250, 251);
			doc.roundedRect(x, 68, tileWidth, 38, 4, 4, 'FD');
			doc.setFont('helvetica', 'normal');
			doc.setFontSize(7);
			doc.setTextColor(107, 114, 128);
			doc.text(tile.label.toUpperCase(), x + 8, 81);
			doc.setFont('helvetica', 'bold');
			doc.setFontSize(11);
			doc.setTextColor(17, 24, 39);
			doc.text(tile.value, x + 8, 98);
		});
		tableStartY = 120;
	}

	// Table headers and rows
	const head = [validColumns.map((col) => col.header)];
	const body = data.map((row) => {
		return validColumns.map((col) => {
			let rawValue: any;
			if (col.getValue) {
				rawValue = col.getValue(row);
			} else if (col.key && typeof row === 'object' && row !== null) {
				rawValue = (row as any)[col.key];
			} else {
				rawValue = '';
			}
			// Replace Taka symbol with BDT for reliable standard font rendering
			return formatCellValue(rawValue).replace(/৳/g, 'BDT ');
		});
	});

	autoTable(doc, {
		startY: tableStartY,
		margin: { left: 40, right: 40, top: 40, bottom: 40 },
		head,
		body,
		theme: 'grid',
		styles: {
			font: 'helvetica',
			fontSize: 8,
			textColor: [31, 41, 55],
			cellPadding: 4.5,
			lineColor: [229, 231, 235],
			lineWidth: 0.5,
			overflow: 'linebreak'
		},
		headStyles: {
			fillColor: [243, 244, 246], // Gray 100
			textColor: [55, 65, 81], // Gray 700
			fontStyle: 'bold',
			fontSize: 8,
			lineColor: [209, 213, 219],
			lineWidth: 0.5
		},
		alternateRowStyles: {
			fillColor: [250, 250, 252]
		},
		didDrawPage: (hookData) => {
			// Footer on every page
			const pageNumber = hookData.pageNumber;
			doc.setFontSize(7.5);
			doc.setTextColor(156, 163, 175); // Gray 400
			doc.text('Generated by Sendora SMS Platform', 40, pageHeight - 20);
			const pageStr = `Page ${pageNumber}`;
			const textWidth = doc.getTextWidth(pageStr);
			doc.text(pageStr, pageWidth - 40 - textWidth, pageHeight - 20);
		}
	});

	const filename = options.filename || generateFilename(options.title, 'pdf');
	doc.save(filename);
}

export const exportToPdf = exportToPDF;

/**
 * Simple compat helper: exportToCsv(filename, rows)
 * rows is an array of plain objects — keys become column headers.
 */
export function exportToCsv(filename: string, rows: Record<string, unknown>[]): void {
	if (!rows || rows.length === 0) {
		throw new Error('No data available to export.');
	}
	const keys = Object.keys(rows[0]);
	const headerRow = keys.map((k) => formatCsvCell(k)).join(',');
	const dataRows = rows.map((row) =>
		keys.map((k) => formatCsvCell(formatCellValue(row[k], true))).join(',')
	);
	const csvContent = '\uFEFF' + [headerRow, ...dataRows].join('\r\n');
	const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
	const link = document.createElement('a');
	const url = URL.createObjectURL(blob);
	link.setAttribute('href', url);
	link.setAttribute('download', filename);
	document.body.appendChild(link);
	link.click();
	document.body.removeChild(link);
	URL.revokeObjectURL(url);
}

