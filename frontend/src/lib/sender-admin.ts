import { supermanApi } from '$lib/superman-api.js';
import { parseCsv } from '$lib/utils/csv.js';

export interface SenderCustomer {
	id: number;
	name: string;
	email?: string | null;
}

export interface SenderGateway {
	id: number;
	name: string;
}

export interface SenderRoute {
	operator_key: string;
	operator_label: string;
	operator_short: string;
	gateway_id: number;
	gateway_name: string;
}

export interface AdminSender {
	id: number;
	sender_id: string;
	type: string;
	masking_category: string;
	is_enabled: boolean;
	approval_status: string;
	rejection_reason?: string | null;
	gateway_id?: number | null;
	gateway_name?: string | null;
	created_at: string;
	users: SenderCustomer[];
	users_count: number;
	messages_count: number;
	routes: SenderRoute[];
}

export interface OperatorDef {
	key: string;
	label: string;
	codes: string;
}

/** The fixed operator set a masking sender has to be routed through. */
export const OPERATORS: OperatorDef[] = [
	{ key: 'gp', label: 'Grameenphone', codes: '013, 017' },
	{ key: 'bl', label: 'Banglalink', codes: '014, 019' },
	{ key: 'tt', label: 'Teletalk', codes: '015' },
	{ key: 'airtel', label: 'Airtel', codes: '016' },
	{ key: 'robi', label: 'Robi', codes: '018' }
];

export function isMaskingType(type: string): boolean {
	return type !== 'numeric';
}

export function typeLabel(type: string): string {
	return isMaskingType(type) ? 'Masking' : 'Non-masking';
}

export async function loadSenderOptions(): Promise<{ gateways: SenderGateway[]; customers: SenderCustomer[] }> {
	const res = await supermanApi.get<{ data: { gateways: SenderGateway[]; customers: SenderCustomer[] } }>(
		'/api/v1/superman/sender-cli/options'
	);
	return { gateways: res.data.gateways ?? [], customers: res.data.customers ?? [] };
}

export function routesSummary(sender: AdminSender): string {
	if (isMaskingType(sender.type) && sender.routes.length > 0) {
		return sender.routes.map((r) => `${r.operator_short}: ${r.gateway_name}`).join(' | ');
	}
	return sender.gateway_name ?? '—';
}

export const IMPORT_TEMPLATE = [
	'sender_id,type,gateway',
	'WHITEINFO,masking,',
	'8809640913002,non-masking,ICON'
].join('\r\n');

export interface ImportRow {
	sender_id: string;
	type: string;
	gateway: string;
	customer: string;
	routes: Record<string, string>;
}

const ROUTE_HEADERS: Record<string, string> = {
	gp: 'gp',
	grameenphone: 'gp',
	bl: 'bl',
	banglalink: 'bl',
	tt: 'tt',
	teletalk: 'tt',
	airtel: 'airtel',
	robi: 'robi'
};

/** Turns CSV text into import rows. The first row is treated as a header when it names a sender column. */
export function parseSenderCsv(text: string): ImportRow[] {
	const table = parseCsv(text);
	if (table.length === 0) return [];

	const normalise = (value: string) => value.toLowerCase().replace(/[^a-z]/g, '');
	const firstRow = table[0].map(normalise);
	const hasHeader = firstRow.some((cell) => ['senderid', 'sender', 'cli', 'id'].includes(cell));

	const columns: Record<string, number> = {};
	const routeColumns: Record<string, number> = {};
	if (hasHeader) {
		firstRow.forEach((cell, index) => {
			if (['senderid', 'sender', 'cli', 'id'].includes(cell) && columns.sender_id === undefined) columns.sender_id = index;
			else if (['type', 'sendertype'].includes(cell)) columns.type = index;
			else if (['gateway', 'gatewayname', 'defaultgateway'].includes(cell)) columns.gateway = index;
			else if (['customer', 'customeremail', 'user', 'useremail'].includes(cell)) columns.customer = index;
			else {
				const stripped = cell.replace(/gateway$/, '');
				if (ROUTE_HEADERS[stripped]) routeColumns[ROUTE_HEADERS[stripped]] = index;
			}
		});
	} else {
		columns.sender_id = 0;
		columns.type = 1;
		columns.gateway = 2;
	}
	if (columns.sender_id === undefined) columns.sender_id = 0;

	const dataRows = hasHeader ? table.slice(1) : table;
	return dataRows.map((cells) => {
		const routes: Record<string, string> = {};
		for (const [key, index] of Object.entries(routeColumns)) routes[key] = cells[index] ?? '';
		return {
			sender_id: cells[columns.sender_id] ?? '',
			type: columns.type !== undefined ? (cells[columns.type] ?? '') : '',
			gateway: columns.gateway !== undefined ? (cells[columns.gateway] ?? '') : '',
			customer: columns.customer !== undefined ? (cells[columns.customer] ?? '') : '',
			routes
		};
	});
}

export function downloadTextFile(filename: string, content: string, mime = 'text/csv;charset=utf-8;') {
	const blob = new Blob(['\uFEFF' + content], { type: mime });
	const url = URL.createObjectURL(blob);
	const link = document.createElement('a');
	link.href = url;
	link.download = filename;
	document.body.appendChild(link);
	link.click();
	document.body.removeChild(link);
	URL.revokeObjectURL(url);
}
