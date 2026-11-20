export interface GatewayMetrics {
	masking_sent: number;
	non_masking_sent: number;
	total_sent: number;
	masking_delivered: number | null;
	non_masking_delivered: number | null;
	total_delivered: number | null;
	dlr_pending: number | null;
	dlr_pending_pct: number | null;
	send_failed: number;
	send_failed_pct: number;
	dlr_failed: number | null;
	dlr_failed_pct: number | null;
	processing: number;
	processing_pct: number;
	masking_delivery_pct: number | null;
	non_masking_delivery_pct: number | null;
	masking_cost: number;
	non_masking_cost: number;
	operator_cost: number;
	dipping_charge: number;
	third_party_dipping_cost: number;
	total_cost: number;
	provider_balance: string | null;
	dipping_balance: string | null;
	revenue: number;
	profit: number;
	verification: string;
	verification_color: string;
}

export interface GatewayRow {
	id: number;
	name: string;
	driver: string;
	priority: number;
	is_enabled: boolean;
	is_otp_enabled: boolean;
	requests_per_second: number;
	health_status: string;
	cost_mode: string;
	dlr_support: boolean;
	has_third_party_dipping: boolean;
	route_code: string | null;
	created_at: string;
	deleted_at: string | null;
	metrics: GatewayMetrics;
}

export interface GatewayDetail {
	id: number;
	name: string;
	driver: string;
	route_code: string | null;
	priority: number;
	is_enabled: boolean;
	is_otp_enabled: boolean;
	supports_bulk: boolean;
	requests_per_second: number;
	timeout_seconds: number;
	dlr_support: boolean;
	cost_mode: string;
	dipping_required: boolean;
	has_third_party_dipping: boolean;
	credentials?: Record<string, unknown>;
	config?: Record<string, unknown>;
}

export const DRIVER_LABELS: Record<string, string> = {
	infozillion: 'Infozillion',
	anbernet: 'Anbernet'
};

export function driverLabel(driver: string): string {
	return DRIVER_LABELS[driver] ?? driver.charAt(0).toUpperCase() + driver.slice(1);
}

export const fmtInt = (value: number | null | undefined): string =>
	value === null || value === undefined ? 'N/A' : Math.round(value).toLocaleString('en-US');

export const fmtPct = (value: number | null | undefined): string =>
	value === null || value === undefined ? 'N/A' : `${value.toFixed(2)}%`;

export const fmtMoney = (value: number | null | undefined, decimals = 2): string =>
	value === null || value === undefined
		? 'N/A'
		: `৳${value.toLocaleString('en-US', { minimumFractionDigits: decimals, maximumFractionDigits: decimals })}`;

export function costModeLabel(mode: string): string {
	return mode === 'api_assisted' ? 'Api assisted' : 'Manual';
}

export const BADGE_TONES: Record<string, string> = {
	success:
		'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-300',
	info: 'border-sky-200 bg-sky-50 text-sky-700 dark:border-sky-900 dark:bg-sky-950/40 dark:text-sky-300',
	warning:
		'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-300',
	danger: 'border-rose-200 bg-rose-50 text-rose-700 dark:border-rose-900 dark:bg-rose-950/40 dark:text-rose-300',
	gray: 'border-gray-200 bg-gray-50 text-gray-700 dark:border-gray-700 dark:bg-gray-800/60 dark:text-gray-300'
};

export function priorityTone(priority: number): string {
	if (priority === 1) return 'success';
	if (priority === 2) return 'info';
	if (priority === 3) return 'warning';
	return 'gray';
}

export const INFOZILLION_DEFAULTS = {
	base_url: 'https://api.mnpspbd.com',
	api_type: 'iptsp',
	api_key: 'VQAzoXHxqsiHljWUfY5mk2lcIKIOFMmw',
	transaction_type: 'T'
};
