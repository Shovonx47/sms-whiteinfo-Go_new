<script lang="ts">
	import { api } from '$lib/api.js';
	import { toast } from '$lib/stores/toast.svelte.js';
	import { onMount } from 'svelte';

	interface DailyMetric {
		date?: string;
		date_val: string;
		total: number;
		successful: number;
		delivered: number;
		failed: number;
		delivery_rate?: number;
		cost: number;
	}

	interface AnalyticsData {
		period: {
			from: string;
			to: string;
		};
		stats: {
			total: number;
			successful: number;
			delivered: number;
			failed: number;
			success_rate: number;
			total_cost: number;
			total_parts: number;
			avg_parts: number;
		};
		campaigns: {
			total: number;
			processing: number;
			completed: number;
			pending: number;
		};
		daily: DailyMetric[];
		operators?: { name: string; count: number; percent: number }[];
	}

	let data = $state<AnalyticsData | null>(null);
	let loading = $state(true);
	type RangeKey = 'today' | 'yesterday' | 'this_week' | '7d' | '30d' | 'this_month' | 'this_year' | 'custom';
	let range = $state<RangeKey>('7d');
	let customFrom = $state('');
	let customTo = $state('');
	let exportModalOpen = $state(false);
	let exportFormat = $state<'csv' | 'pdf'>('csv');
	let exporting = $state(false);

	async function handleExport() {
		exporting = true;
		try {
			const params = new URLSearchParams({ range, format: exportFormat });
			if (range === 'custom') {
				params.set('from', customFrom);
				params.set('to', customTo);
			}
			const blob = await api.download(`/api/v1/customer/analytics/messaging/export?${params}`);
			const url = URL.createObjectURL(blob);
			const a = document.createElement('a');
			a.href = url;
			a.download = `messaging-analytics-${range}.${exportFormat === 'pdf' ? 'pdf' : 'csv'}`;
			document.body.appendChild(a);
			a.click();
			a.remove();
			URL.revokeObjectURL(url);
			toast.success('Export Ready', 'Your analytics report download has started.');
			exportModalOpen = false;
		} catch (err: any) {
			toast.error('Export Failed', err?.message || 'Unable to export analytics report.');
		} finally {
			exporting = false;
		}
	}

	async function loadAnalytics() {
		loading = true;
		try {
			const params = new URLSearchParams({ range });
			if (range === 'custom') {
				params.set('from', customFrom);
				params.set('to', customTo);
			}
			const res = await api.get<{ data: AnalyticsData }>(`/api/v1/customer/analytics/messaging?${params}`);
			data = res.data;
		} catch {
			data = null;
		} finally {
			loading = false;
		}
	}

	onMount(() => loadAnalytics());

	let maxDailyTotal = $derived(
		data?.daily ? Math.max(...data.daily.map((d) => d.total), 1) : 1
	);
</script>

<svelte:head>
	<title>Messaging Analytics - White INFO</title>
</svelte:head>

<div class="space-y-6">
	<!-- Page Header -->
	<div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
		<div>
			<h1 class="text-xl font-bold tracking-tight text-gray-950 dark:text-white">Delivery & Messaging Analytics</h1>
			<p class="text-xs text-gray-500 dark:text-gray-400">Deep performance metrics, delivery throughput, and carrier cost distribution.</p>
		</div>

		<!-- Date Range Toggle + Export As -->
		<div class="flex flex-wrap items-center gap-2">
			<button
				type="button"
				onclick={() => (exportModalOpen = true)}
				class="inline-flex items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-3 py-1.5 text-xs font-semibold text-gray-700 shadow-xs transition hover:bg-gray-50 cursor-pointer dark:border-gray-800 dark:bg-gray-900 dark:text-gray-200 dark:hover:bg-gray-800"
			>
				<svg class="h-3.5 w-3.5 text-gray-500" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5M16.5 12 12 16.5m0 0L7.5 12m4.5 4.5V3" />
				</svg>
				Export As
			</button>
			<select
				bind:value={range}
				onchange={loadAnalytics}
				class="rounded-lg border border-gray-200 bg-white px-3 py-1.5 text-xs font-semibold text-gray-700 shadow-xs dark:border-gray-800 dark:bg-gray-900 dark:text-gray-200"
			>
				<option value="today">Today</option>
				<option value="yesterday">Yesterday</option>
				<option value="this_week">This Week</option>
				<option value="7d">Last 7 Days</option>
				<option value="30d">Last 30 Days</option>
				<option value="this_month">This Month</option>
				<option value="this_year">This Year</option>
				<option value="custom">Custom Range</option>
			</select>
			{#if range === 'custom'}
				<input type="date" bind:value={customFrom} onchange={loadAnalytics} class="rounded-lg border border-gray-200 bg-white px-2 py-1.5 text-xs dark:border-gray-800 dark:bg-gray-900 dark:text-gray-200" />
				<span class="text-xs text-gray-400">to</span>
				<input type="date" bind:value={customTo} onchange={loadAnalytics} class="rounded-lg border border-gray-200 bg-white px-2 py-1.5 text-xs dark:border-gray-800 dark:bg-gray-900 dark:text-gray-200" />
			{/if}
		</div>
	</div>

	<!-- Main KPI Cards -->
	<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
		<!-- Total Messages -->
		<div class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-800 dark:bg-gray-900">
			<span class="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">Total Dispatched</span>
			<div class="mt-2 flex items-baseline gap-2">
				<span class="text-3xl font-extrabold text-gray-950 dark:text-white font-mono">{data?.stats.total.toLocaleString() ?? '—'}</span>
				<span class="text-xs text-teal-600 dark:text-teal-400">messages</span>
			</div>
			<p class="mt-2 text-[11px] text-gray-500">Sent &amp; queued in selected period</p>
		</div>

		<!-- Delivery Rate -->
		<div class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-800 dark:bg-gray-900">
			<span class="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">Delivery Rate</span>
			<div class="mt-2 flex items-baseline gap-2">
				<span class="text-3xl font-extrabold text-emerald-600 dark:text-emerald-400 font-mono">{data?.stats.success_rate ?? 0}%</span>
				<span class="text-xs text-emerald-600 dark:text-emerald-400">DLR verified</span>
			</div>
			<p class="mt-2 text-[11px] text-gray-500">{data?.stats.failed.toLocaleString() ?? 0} bounced / failed</p>
		</div>

		<!-- Total Cost -->
		<div class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-800 dark:bg-gray-900">
			<span class="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">Total Spend</span>
			<div class="mt-2 flex items-baseline gap-2">
				<span class="text-3xl font-extrabold text-teal-600 dark:text-teal-400 font-mono">৳ {data?.stats.total_cost.toFixed(2) ?? '0.00'}</span>
				<span class="text-xs text-gray-500">BDT</span>
			</div>
			<p class="mt-2 text-[11px] text-gray-500">{data?.stats.total_parts.toLocaleString() ?? 0} total SMS parts billed</p>
		</div>

		<!-- Average Parts -->
		<div class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-800 dark:bg-gray-900">
			<span class="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">Avg Parts / Message</span>
			<div class="mt-2 flex items-baseline gap-2">
				<span class="text-3xl font-extrabold text-gray-950 dark:text-white font-mono">{data?.stats.avg_parts ?? 1.0}</span>
				<span class="text-xs text-gray-500">parts/SMS</span>
			</div>
			<p class="mt-2 text-[11px] text-gray-500">GSM-7 & Unicode combined</p>
		</div>
	</div>

	<!-- Daily Throughput Chart (Visual Bar Representation) -->
	<div class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm space-y-4 dark:border-gray-800 dark:bg-gray-900">
		<div class="flex items-center justify-between">
			<div>
				<h2 class="text-sm font-bold text-gray-950 dark:text-white">Daily Dispatch Volume & Throughput</h2>
				<p class="text-xs text-gray-500 dark:text-gray-400">Daily breakdown of total dispatched SMS vs. successful deliveries.</p>
			</div>
			<div class="flex items-center gap-4 text-xs">
				<div class="flex items-center gap-1.5">
					<span class="h-2.5 w-2.5 rounded-full bg-teal-600"></span>
					<span class="text-gray-700 dark:text-gray-300">Total</span>
				</div>
				<div class="flex items-center gap-1.5">
					<span class="h-2.5 w-2.5 rounded-full bg-emerald-500"></span>
					<span class="text-gray-700 dark:text-gray-300">Delivered</span>
				</div>
			</div>
		</div>

		{#if data?.daily}
			<div class="pt-6 pb-2">
				<div class="flex items-end justify-between gap-3 h-48">
					{#each data.daily as d (d.date_val || d.date)}
						{@const heightPercent = Math.round((d.total / maxDailyTotal) * 100)}
						{@const deliveredPercent = d.total > 0 ? Math.round((d.delivered / d.total) * 100) : 0}
						<div class="flex-1 flex flex-col items-center gap-2 h-full justify-end group">
							<!-- Tooltip on hover -->
							<div class="opacity-0 group-hover:opacity-100 transition duration-200 pointer-events-none mb-1 text-center bg-gray-900 border border-gray-800 px-2.5 py-1.5 rounded-lg text-[10px] text-gray-100 shadow-xl">
								<div class="font-semibold">Total: {d.total}</div>
								<div class="text-emerald-400">Delivered: {d.delivered}</div>
								<div class="text-teal-400">৳{d.cost}</div>
							</div>

							<!-- Bar Stack -->
							<div class="w-full max-w-[42px] bg-gray-100 rounded-t-lg overflow-hidden flex flex-col justify-end transition-all duration-300 hover:brightness-105 dark:bg-gray-800" style="height: {heightPercent}%;">
								<div class="w-full bg-teal-600/30 flex-1"></div>
								<div class="w-full bg-teal-600" style="height: {deliveredPercent}%;"></div>
							</div>

							<!-- Label -->
							<span class="text-[10px] font-mono text-gray-500 dark:text-gray-400 truncate w-full text-center">
								{d.date_val ? d.date_val.slice(5) : d.date}
							</span>
						</div>
					{/each}
				</div>
			</div>
		{/if}
	</div>

	<!-- Delivery Rate Trend -->
	<div class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm space-y-4 dark:border-gray-800 dark:bg-gray-900">
		<div class="flex items-center justify-between">
			<div>
				<h2 class="text-sm font-bold text-gray-950 dark:text-white">Delivery Success Rate Trend (%)</h2>
				<p class="text-xs text-gray-500 dark:text-gray-400">Daily percentage of sent &amp; confirmed delivered SMS.</p>
			</div>
		</div>
		{#if data?.daily?.length}
			<div class="grid grid-cols-2 gap-3 sm:grid-cols-4 md:grid-cols-7 pt-2">
				{#each data.daily as d (d.date_val || d.date)}
					{@const rate = d.total > 0 ? Math.round(((d.successful ?? d.delivered) / d.total) * 1000) / 10 : 0}
					<div class="rounded-lg border border-gray-200 bg-gray-50/70 p-3 dark:border-gray-800 dark:bg-gray-950/60">
						<div class="flex items-center justify-between text-[11px] mb-1.5">
							<span class="font-mono text-gray-600 dark:text-gray-400">{d.date || d.date_val.slice(5)}</span>
							<span class="font-mono font-bold {rate >= 90 ? 'text-emerald-600' : rate >= 70 ? 'text-amber-600' : 'text-red-600'}">{rate}%</span>
						</div>
						<div class="h-1.5 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-gray-800">
							<div class="h-full {rate >= 90 ? 'bg-emerald-500' : rate >= 70 ? 'bg-amber-500' : 'bg-red-500'}" style="width: {Math.min(rate, 100)}%;"></div>
						</div>
					</div>
				{/each}
			</div>
		{:else if !loading}
			<p class="text-xs text-gray-400 py-4 text-center">No delivery data recorded in the selected period.</p>
		{/if}
	</div>

	<!-- Operator Distribution -->
	<div class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm space-y-4 dark:border-gray-800 dark:bg-gray-900">
		<h2 class="text-sm font-bold text-gray-950 dark:text-white">Mobile Network Operator Share</h2>
		<p class="text-xs text-gray-500 dark:text-gray-400">Message delivery distribution across major Bangladesh telecommunication carriers.</p>

		{#if data?.operators?.length}
			<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4 pt-2">
				{#each data.operators as op (op.name)}
					<div class="rounded-xl border border-gray-200 bg-gray-50/70 p-4 space-y-2 dark:border-gray-800 dark:bg-gray-950/60">
						<div class="flex items-center justify-between text-xs">
							<span class="font-semibold text-gray-800 dark:text-gray-200">{op.name}</span>
							<span class="font-mono text-teal-600 dark:text-teal-400 font-bold">{op.percent}%</span>
						</div>
						<div class="h-2 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-gray-800">
							<div class="h-full bg-teal-600" style="width: {op.percent}%;"></div>
						</div>
						<p class="text-[11px] text-gray-500 font-mono">{op.count.toLocaleString()} messages</p>
					</div>
				{/each}
			</div>
		{:else if !loading}
			<p class="text-xs text-gray-400 py-4 text-center">No operator breakdown recorded in the selected period.</p>
		{/if}
	</div>
</div>

<!-- Export Modal -->
{#if exportModalOpen}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-xs">
		<div class="w-full max-w-md rounded-2xl bg-white p-6 shadow-xl border border-gray-100 dark:bg-gray-900 dark:border-gray-800">
			<div class="flex items-start justify-between mb-4">
				<div>
					<h3 class="text-base font-bold text-gray-950 dark:text-white">Export Messaging Analytics</h3>
					<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">Select your preferred report format to download.</p>
				</div>
				<button type="button" onclick={() => (exportModalOpen = false)} class="text-gray-400 hover:text-gray-600 cursor-pointer" aria-label="Close">
					<svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" />
					</svg>
				</button>
			</div>

			<div class="space-y-2.5 my-4">
				<label class="flex items-center gap-3 rounded-xl border p-3 cursor-pointer transition {exportFormat === 'csv' ? 'border-teal-600 bg-teal-50/50 dark:bg-teal-500/10' : 'border-gray-200 dark:border-gray-800'}">
					<input type="radio" bind:group={exportFormat} value="csv" class="text-teal-600 focus:ring-teal-600" />
					<div>
						<div class="text-xs font-semibold text-gray-900 dark:text-white">Excel / CSV Spreadsheet (.csv)</div>
						<div class="text-[11px] text-gray-500">UTF-8 CSV with daily breakdown &amp; spend</div>
					</div>
				</label>
				<label class="flex items-center gap-3 rounded-xl border p-3 cursor-pointer transition {exportFormat === 'pdf' ? 'border-teal-600 bg-teal-50/50 dark:bg-teal-500/10' : 'border-gray-200 dark:border-gray-800'}">
					<input type="radio" bind:group={exportFormat} value="pdf" class="text-teal-600 focus:ring-teal-600" />
					<div>
						<div class="text-xs font-semibold text-gray-900 dark:text-white">PDF Document (.pdf)</div>
						<div class="text-[11px] text-gray-500">Summary KPIs and daily breakdown report</div>
					</div>
				</label>
			</div>

			<div class="flex justify-end gap-2 pt-2">
				<button
					type="button"
					onclick={() => (exportModalOpen = false)}
					class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-xs font-semibold text-gray-700 hover:bg-gray-50 cursor-pointer"
				>
					Cancel
				</button>
				<button
					type="button"
					disabled={exporting}
					onclick={handleExport}
					class="rounded-lg bg-teal-600 px-4 py-2 text-xs font-semibold text-white hover:bg-teal-700 disabled:opacity-50 cursor-pointer"
				>
					{exporting ? 'Preparing...' : 'Download Report'}
				</button>
			</div>
		</div>
	</div>
{/if}
