export interface AdminTemplate {
	id: number;
	user_id: number;
	customer: string;
	name: string;
	body: string;
	variables: string[];
	approval: 'pending' | 'approved' | 'rejected' | string;
	reviewed_by?: string | null;
	approved_at?: string | null;
	rejection_reason?: string | null;
	created_at: string;
	updated_at: string;
	deleted_at?: string | null;
}

export interface CustomerOption {
	id: number;
	name: string;
	email: string;
}

const GSM_BASIC =
	"@£$¥èéùìòÇ\nØø\rÅåΔ_ΦΓΛΩΠΨΣΘΞ ÆæßÉ !\"#¤%&'()*+,-./0123456789:;<=>?¡ABCDEFGHIJKLMNOPQRSTUVWXYZÄÖÑÜ§¿abcdefghijklmnopqrstuvwxyzäöñüà";
const GSM_EXTENDED = '\f^{}\\[~]|€';
const LIMITS = {
	'GSM-7': { single: 160, concat: 153 },
	Unicode: { single: 70, concat: 67 }
};

export function computeSmsStats(text: string) {
	const str = text || '';
	let isUnicode = false;
	let count = 0;

	for (let i = 0; i < str.length; i++) {
		const ch = str.charAt(i);
		if (GSM_EXTENDED.indexOf(ch) !== -1) {
			count += 2;
		} else if (GSM_BASIC.indexOf(ch) !== -1) {
			count += 1;
		} else {
			isUnicode = true;
			break;
		}
	}

	const encoding = isUnicode ? 'Unicode' : 'GSM-7';
	const lim = LIMITS[encoding];
	const len = isUnicode ? Array.from(str).length : count;

	let parts = 0;
	let total = lim.single;
	let remaining = lim.single;

	if (len === 0) {
		parts = 0;
		total = lim.single;
		remaining = lim.single;
	} else if (len <= lim.single) {
		parts = 1;
		total = lim.single;
		remaining = lim.single - len;
	} else {
		parts = Math.ceil(len / lim.concat);
		total = lim.concat;
		remaining = parts * lim.concat - len;
	}

	return {
		len,
		encoding,
		parts,
		total,
		remaining
	};
}

export function extractVariables(text: string): string[] {
	const matches = text.matchAll(/\{(\w+)\}/g);
	const vars = new Set<string>();
	for (const m of matches) {
		if (m[1]) vars.add(m[1]);
	}
	return Array.from(vars);
}

export function formatDate(iso?: string | null): string {
	if (!iso) return '—';
	try {
		const d = new Date(iso);
		if (isNaN(d.getTime())) return iso;
		return d.toLocaleDateString('en-GB', {
			day: '2-digit',
			month: 'short',
			year: 'numeric',
			hour: '2-digit',
			minute: '2-digit',
			hour12: true
		});
	} catch {
		return iso;
	}
}
