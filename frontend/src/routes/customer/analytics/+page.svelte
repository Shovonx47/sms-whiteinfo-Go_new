<script lang="ts">
	import { api } from '$lib/api.js';
	import { toast } from '$lib/stores/toast.svelte.js';
	import { onMount } from 'svelte';

	interface DailyMetric {
		date: string;
		date_val: string;
		total: number;
		successful: number;
		delivered: number;
		failed: number;
		delivery_rate: number;
		success_rate: number;
		cost: number;
	}

	interface AnalyticsData {
		period: { from: string; to: string };
		stats: {
			total: number;
			successful: number;
			delivered: number;
			failed: number;
			success_rate: number;
			total_cost: number;
			total_parts: number;
			avg_parts: number;
			unicode: number;
		};
		daily: DailyMetric[];
	}

	type RangeKey =
		| 'today'
		| 'yesterday'
		| 'this_week'
		| 'last_7_days'
		| 'this_month'
		| 'last_30_days'
		| 'this_year'
		| 'custom';

	let data = $state<AnalyticsData | null>(null);
	let loading = $state(true);
	let range = $state<RangeKey>('last_30_days');
	let customFrom = $state('');
	let customTo = $state('');
	let exportModalOpen = $state(false);
	let exportFormat = $state<'csv' | 'pdf'>('csv');
	let exporting = $state(false);

	function queryParams(includeFormat = false) {
		const params = new URLSearchParams({ range });
		if (range === 'custom') {
			params.set('from', customFrom);
			params.set('to', customTo);
		}
		if (includeFormat) params.set('format', exportFormat);
		return params;
	}

	async function handleExport() {
		exporting = true;
		try {
			const blob = await api.download(`/api/v1/customer/analytics/messaging/export?${queryParams(true)}`);
			const url = URL.createObjectURL(blob);
			const stamp = new Date().toISOString().slice(0, 19).replace('T', '-').replace(/:/g, '');
			const a = document.createElement('a');
			a.href = url;
			a.download = `messaging-analytics-${stamp}.${exportFormat === 'pdf' ? 'pdf' : 'csv'}`;
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
			const res = await api.get<{ data: AnalyticsData }>(
				`/api/v1/customer/analytics/messaging?${queryParams()}`
			);
			data = res.data;
		} catch {
			data = null;
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		loadAnalytics();
		// Reference MessageStatsByPeriodWidget polls every 60 seconds.
		const timer = setInterval(loadAnalytics, 60_000);
		return () => clearInterval(timer);
	});

	function num(value: number) {
		return Number(value || 0).toLocaleString('en-US');
	}

	function pct(part: number, total: number) {
		return total > 0 ? Math.round((part / total) * 1000) / 10 : 0;
	}

	function points(values: number[], width = 640, height = 190) {
		if (!values.length) return '';
		const max = Math.max(...values, 1);
		const step = values.length > 1 ? width / (values.length - 1) : 0;
		return values
			.map((value, index) => `${(index * step).toFixed(2)},${(height - (value / max) * (height - 16)).toFixed(2)}`)
			.join(' ');
	}

	function areaPoints(values: number[], width = 640, height = 190) {
		if (!values.length) return '';
		return `0,${height} ${points(values, width, height)} ${width},${height}`;
	}

	function labelEvery(length: number) {
		return Math.max(1, Math.ceil(length / 8));
	}

	let totalMessages = $derived(data?.stats.total ?? 0);
	let successful = $derived(data?.stats.successful ?? 0);
	let delivered = $derived(data?.stats.delivered ?? 0);
	let failed = $derived(data?.stats.failed ?? 0);
	let unicode = $derived(data?.stats.unicode ?? 0);
	let successRate = $derived(data?.stats.success_rate ?? 0);
	let trendValues = $derived((data?.daily ?? []).map((d) => d.total));
	let rateValues = $derived((data?.daily ?? []).map((d) => d.success_rate));
</script>

<svelte:head>
	<title>Messaging Analytics - White INFO</title>
</svelte:head>

<div class="space-y-6">
	<div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
		<h1 class="text-xl font-bold tracking-tight text-gray-950 dark:text-white">Messaging Analytics</h1>

		<div class="flex flex-wrap items-center gap-2">
			<select
				bind:value={range}
				onchange={loadAnalytics}
				class="rounded-lg border border-gray-200 bg-white px-3 py-1.5 text-xs font-semibold text-gray-700 shadow-xs dark:border-gray-800 dark:bg-gray-900 dark:text-gray-200"
			>
				<option value="today">Today</option>
				<option value="yesterday">Yesterday</option>
				<option value="this_week">This Week</option>
				<option value="last_7_days">Last 7 Days</option>
				<option value="this_month">This Month</option>
				<option value="last_30_days">Last 30 Days</option>
				<option value="this_year">This Year</option>
				<option value="custom">Custom Range</option>
			</select>
			{#if range === 'custom'}
				<input type="date" bind:value={customFrom} onchange={loadAnalytics} class="rounded-lg border border-gray-200 bg-white px-2 py-1.5 text-xs dark:border-gray-800 dark:bg-gray-900 dark:text-gray-200" />
				<span class="text-xs text-gray-400">to</span>
				<input type="date" bind:value={customTo} onchange={loadAnalytics} class="rounded-lg border border-gray-200 bg-white px-2 py-1.5 text-xs dark:border-gray-800 dark:bg-gray-900 dark:text-gray-200" />
			{/if}
			<button
				type="button"
				onclick={() => (exportModalOpen = true)}
				class="inline-flex items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-3 py-1.5 text-xs font-semibold text-gray-700 shadow-xs transition hover:bg-gray-50 dark:border-gray-800 dark:bg-gray-900 dark:text-gray-200 dark:hover:bg-gray-800"
			>
				<svg class="h-3.5 w-3.5 text-gray-500" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5M16.5 12 12 16.5m0 0L7.5 12m4.5 4.5V3" />
				</svg>
				Export As
			</button>
		</div>
	</div>

	<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
		<div class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-800 dark:bg-gray-900">
			<p class="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">Total Messages</p>
			<p class="mt-2 font-mono text-3xl font-extrabold text-gray-950 dark:text-white">{loading ? '—' : num(totalMessages)}</p>
			<p class="mt-2 text-[11px] text-gray-500">In selected period</p>
		</div>

		<div class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-800 dark:bg-gray-900">
			<p class="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">Success Rate</p>
			<p class="mt-2 font-mono text-3xl font-extrabold {successRate >= 90 ? 'text-emerald-600' : successRate >= 70 ? 'text-amber-600' : 'text-rose-600'}">{successRate}%</p>
			<p class="mt-2 text-[11px] text-gray-500">{num(successful)} sent or delivered</p>
		</div>

		<div class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-800 dark:bg-gray-900">
			<p class="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">Delivered</p>
			<p class="mt-2 font-mono text-3xl font-extrabold text-gray-950 dark:text-white">{num(delivered)}</p>
			<p class="mt-2 text-[11px] text-gray-500">{totalMessages > 0 ? `${pct(delivered, totalMessages)}% confirmed delivered` : '0% confirmed'}</p>
		</div>

		<div class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-800 dark:bg-gray-900">
			<p class="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">Failed</p>
			<p class="mt-2 font-mono text-3xl font-extrabold text-gray-950 dark:text-white">{num(failed)}</p>
			<p class="mt-2 text-[11px] text-gray-500">{totalMessages > 0 ? `${pct(failed, totalMessages)}% failure rate (send + DLR)` : '0% failure rate'}</p>
		</div>

		<div class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-800 dark:bg-gray-900">
			<p class="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">Total Cost</p>
			<p class="mt-2 font-mono text-3xl font-extrabold text-gray-950 dark:text-white">{Number(data?.stats.total_cost ?? 0).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })} BDT</p>
			<p class="mt-2 text-[11px] text-gray-500">Your spend</p>
		</div>

		<div class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-800 dark:bg-gray-900">
			<p class="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">Avg Parts</p>
			<p class="mt-2 font-mono text-3xl font-extrabold text-gray-950 dark:text-white">{Number(data?.stats.avg_parts ?? 0).toFixed(1)}</p>
			<p class="mt-2 text-[11px] text-gray-500">SMS segments per message</p>
		</div>

		<div class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-800 dark:bg-gray-900">
			<p class="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">Unicode</p>
			<p class="mt-2 font-mono text-3xl font-extrabold text-gray-950 dark:text-white">{num(unicode)}</p>
			<p class="mt-2 text-[11px] text-gray-500">{totalMessages > 0 ? `${pct(unicode, totalMessages)}% of messages` : '0% of messages'}</p>
		</div>
	</div>

	<div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
		<div class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900">
			<h2 class="text-sm font-bold text-gray-950 dark:text-white">Message Trends</h2>
			<p class="text-xs text-gray-500 dark:text-gray-400">Messages per day in the selected period</p>
			{#if data?.daily?.length}
				<div class="mt-4">
					<svg viewBox="0 0 640 210" class="h-52 w-full" preserveAspectRatio="none" role="img" aria-label="Message trends">
						<polygon points={areaPoints(trendValues)} fill="#10b981" fill-opacity="0.1"></polygon>
						<polyline points={points(trendValues)} fill="none" stroke="#10b981" stroke-width="2"></polyline>
					</svg>
					<div class="mt-1 flex justify-between text-[10px] font-mono text-gray-500 dark:text-gray-400">
						{#each data.daily as d, i (d.date_val)}
							{#if i % labelEvery(data.daily.length) === 0 || i === data.daily.length - 1}
								<span>{d.date}</span>
							{/if}
						{/each}
					</div>
				</div>
			{:else}
				<p class="py-8 text-center text-xs text-gray-400">No messages in the selected period.</p>
			{/if}
		</div>

		<div class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900">
			<h2 class="text-sm font-bold text-gray-950 dark:text-white">Delivery Rate Trend</h2>
			<p class="text-xs text-gray-500 dark:text-gray-400">Daily delivery rate in the selected period</p>
			{#if data?.daily?.length}
				<div class="mt-4">
					<svg viewBox="0 0 640 210" class="h-52 w-full" preserveAspectRatio="none" role="img" aria-label="Delivery rate trend">
						<polygon points={areaPoints(rateValues)} fill="#10b981" fill-opacity="0.1"></polygon>
						<polyline points={points(rateValues)} fill="none" stroke="#10b981" stroke-width="2"></polyline>
					</svg>
					<div class="mt-1 flex justify-between text-[10px] font-mono text-gray-500 dark:text-gray-400">
						{#each data.daily as d, i (d.date_val)}
							{#if i % labelEvery(data.daily.length) === 0 || i === data.daily.length - 1}
								<span>{d.date}</span>
							{/if}
						{/each}
					</div>
				</div>
			{:else}
				<p class="py-8 text-center text-xs text-gray-400">No delivery data in the selected period.</p>
			{/if}
		</div>
	</div>
</div>

{#if exportModalOpen}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-xs">
		<div class="w-full max-w-md rounded-2xl border border-gray-100 bg-white p-6 shadow-xl dark:border-gray-800 dark:bg-gray-900">
			<div class="flex items-center justify-between border-b border-gray-100 pb-3 dark:border-gray-800">
				<div>
					<h3 class="text-base font-bold text-gray-950 dark:text-white">Export Messaging Analytics</h3>
					<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">Select your preferred report format to download.</p>
				</div>
				<button type="button" onclick={() => (exportModalOpen = false)} class="text-gray-400 hover:text-gray-600" aria-label="Close">
					<svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" />
					</svg>
				</button>
			</div>

			<div class="my-4 space-y-2.5">
				<label class="flex cursor-pointer items-center gap-3 rounded-xl border p-3 transition {exportFormat === 'csv' ? 'border-teal-600 bg-teal-50/50 dark:bg-teal-500/10' : 'border-gray-200 dark:border-gray-800'}">
					<input type="radio" bind:group={exportFormat} value="csv" class="text-teal-600 focus:ring-teal-600" />
					<div>
						<div class="text-xs font-semibold text-gray-900 dark:text-white">Excel / CSV Spreadsheet (.csv)</div>
						<div class="text-[11px] text-gray-500">UTF-8 CSV with daily breakdown &amp; spend</div>
					</div>
				</label>
				<label class="flex cursor-pointer items-center gap-3 rounded-xl border p-3 transition {exportFormat === 'pdf' ? 'border-teal-600 bg-teal-50/50 dark:bg-teal-500/10' : 'border-gray-200 dark:border-gray-800'}">
					<input type="radio" bind:group={exportFormat} value="pdf" class="text-teal-600 focus:ring-teal-600" />
					<div>
						<div class="text-xs font-semibold text-gray-900 dark:text-white">PDF Document (.pdf)</div>
						<div class="text-[11px] text-gray-500">Summary KPIs and daily breakdown report</div>
					</div>
				</label>
			</div>

			<div class="flex justify-end gap-2 pt-2">
				<button type="button" onclick={() => (exportModalOpen = false)} class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-xs font-semibold text-gray-700 hover:bg-gray-50">
					Cancel
				</button>
				<button type="button" disabled={exporting} onclick={handleExport} class="rounded-lg bg-teal-600 px-4 py-2 text-xs font-semibold text-white hover:bg-teal-700 disabled:opacity-50">
					{exporting ? 'Preparing...' : 'Download Report'}
				</button>
			</div>
		</div>
	</div>
{/if}
