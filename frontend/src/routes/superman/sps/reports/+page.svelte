<script lang="ts">
	import { onMount } from 'svelte';
	import { supermanApi } from '$lib/superman-api.js';
	import ExportDropdown from '$lib/components/common/ExportDropdown.svelte';
	import type { ExportColumn } from '$lib/utils/export.js';

	interface ExecutiveReport {
		today_sms?: number;
		month_sms?: number;
		total_dispatches?: number;
		delivered_count?: number;
		delivery_rate_percent?: number;
		failed_count?: number;
		gross_revenue?: number;
		total_operator_cost?: number;
		net_gross_profit?: number;
		net_profit?: number;
		profit_margin_percent?: number;
		delivered_pct?: number;
		failed_pct?: number;
	}

	interface SpExportRow {
		metric: string;
		value: string;
		notes: string;
	}

	const exportColumns: ExportColumn<SpExportRow>[] = [
		{ header: 'Metric', getValue: (r) => r.metric },
		{ header: 'Value', getValue: (r) => r.value },
		{ header: 'Notes', getValue: (r) => r.notes }
	];

	let data = $state<ExecutiveReport | null>(null);
	let loading = $state(true);

	function num(value: number | undefined, fallback = 0): number {
		return typeof value === 'number' && Number.isFinite(value) ? value : fallback;
	}

	function getSpExportRows(): SpExportRow[] {
		if (!data) return [];
		const total = num(data.total_dispatches, num(data.month_sms));
		const delivered = num(data.delivered_count);
		const gross = num(data.gross_revenue);
		const profit = num(data.net_profit, num(data.net_gross_profit));
		const deliveredPct = num(data.delivered_pct, num(data.delivery_rate_percent));
		const failedPct = num(data.failed_pct, total > 0 ? (num(data.failed_count) / total) * 100 : 0);
		return [
			{ metric: 'Total Dispatches', value: total.toLocaleString(), notes: `${delivered.toLocaleString()} delivered` },
			{ metric: 'Carrier Cost', value: `BDT ${num(data.total_operator_cost).toFixed(2)}`, notes: 'Upstream gateway cost' },
			{ metric: 'Gross Billed', value: `BDT ${gross.toFixed(2)}`, notes: 'Reseller & platform gross billing' },
			{ metric: 'Net Profit', value: `BDT ${profit.toFixed(2)}`, notes: `${num(data.profit_margin_percent).toFixed(1)}% gross margin` },
			{ metric: 'Delivery Success', value: `${deliveredPct.toFixed(1)}%`, notes: `Failed/Rejected: ${failedPct.toFixed(1)}%` }
		];
	}

	onMount(async () => {
		try {
			const res = await supermanApi.get<{ data: ExecutiveReport }>('/api/v1/superman/sps/reports');
			data = res.data ?? null;
		} catch (e) {
			console.error(e);
		} finally {
			loading = false;
		}
	});
</script>

<div class="space-y-6">
	<!-- Page Header -->
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-xl font-bold text-gray-900 dark:text-white">Service Provider (SP) Performance Reports</h1>
			<p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
				Aggregated SMS volumes, operator delivery ratios, and financial yield across all reseller tiers.
			</p>
		</div>
		<div class="flex items-center gap-2">
			<ExportDropdown
				title="Service Provider Performance Reports"
				columns={exportColumns}
				getData={getSpExportRows}
				disabled={!data}
			/>
			<a
				href="/superman/reports/reseller"
				class="inline-flex items-center gap-2 rounded-lg bg-emerald-600 px-3 py-1.5 text-xs font-semibold text-white shadow-xs hover:bg-emerald-700 transition"
			>
				Detailed Reseller Report
			</a>
		</div>
	</div>

	{#if loading}
		<div class="p-8 text-center text-xs text-gray-400">Loading performance report metrics...</div>
	{:else if data}
		<div class="grid grid-cols-1 md:grid-cols-4 gap-4">
			<div class="bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-800 rounded-xl p-5 shadow-xs">
				<span class="text-xs font-medium text-gray-500 uppercase tracking-wider">Total Dispatches</span>
				<p class="text-3xl font-bold text-gray-900 dark:text-white mt-1">{num(data.total_dispatches, num(data.today_sms)).toLocaleString()}</p>
				<span class="text-[11px] text-gray-400 mt-1 block">{num(data.delivered_count).toLocaleString()} delivered</span>
			</div>
			<div class="bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-800 rounded-xl p-5 shadow-xs">
				<span class="text-xs font-medium text-blue-600 uppercase tracking-wider">Operator Carrier Cost</span>
				<p class="text-3xl font-bold text-blue-600 mt-1">৳{num(data.total_operator_cost).toFixed(2)}</p>
				<span class="text-[11px] text-gray-400 mt-1 block">Upstream gateway settlement</span>
			</div>
			<div class="bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-800 rounded-xl p-5 shadow-xs">
				<span class="text-xs font-medium text-emerald-600 uppercase tracking-wider">Gross Billed</span>
				<p class="text-3xl font-bold text-emerald-600 mt-1">৳{num(data.gross_revenue).toFixed(2)}</p>
				<span class="text-[11px] text-emerald-600 mt-1 block">Net Profit: ৳{num(data.net_profit, num(data.net_gross_profit)).toFixed(2)}</span>
			</div>
			<div class="bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-800 rounded-xl p-5 shadow-xs">
				<span class="text-xs font-medium text-purple-600 uppercase tracking-wider">Delivery Success</span>
				<p class="text-3xl font-bold text-purple-600 mt-1">{num(data.delivered_pct, num(data.delivery_rate_percent)).toFixed(1)}%</p>
				<span class="text-[11px] text-gray-400 mt-1 block">Failed/Rejected: {num(data.failed_count).toLocaleString()} msgs</span>
			</div>
		</div>
	{/if}
</div>

