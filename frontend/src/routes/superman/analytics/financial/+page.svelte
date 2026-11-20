<script lang="ts">
	import { onMount } from 'svelte';
	import { supermanApi } from '$lib/superman-api.js';

	interface PeriodStats {
		deposits: number;
		customer_spend: number;
		gateway_cost: number;
		pending_deposits: number;
		transactions: number;
		system_balance: number;
	}

	interface RevenueStats {
		total_deposits: number;
		pending_deposits: number;
		total_spent: number;
		system_balance: number;
	}

	interface UserStats {
		new_users_30d: number;
		pending_approvals: number;
		api_users: number;
		total_balance: number;
	}

	interface DepositTrendPoint {
		date: string;
		label: string;
		total: number;
	}

	interface SpendingTrendPoint {
		date: string;
		label: string;
		revenue: number;
		cost: number;
	}

	interface DepositMethodPoint {
		method: string;
		label: string;
		count: number;
		total: number;
	}

	interface TransactionVolumePoint {
		date: string;
		label: string;
		credits: number;
		debits: number;
	}

	interface TopCustomer {
		id: number;
		name: string;
		email: string;
		status: string;
		balance: number;
		period_spent: number | null;
		messages_count: number;
	}

	interface FinancialAnalyticsData {
		period_stats: PeriodStats;
		revenue_stats: RevenueStats;
		user_stats: UserStats;
		deposit_trend: DepositTrendPoint[];
		spending_trend: SpendingTrendPoint[];
		deposits_by_method: DepositMethodPoint[];
		transaction_volume: TransactionVolumePoint[];
		top_customers: TopCustomer[];
	}

	const DATE_RANGE_OPTIONS = [
		{ value: 'today', label: 'Today' },
		{ value: 'yesterday', label: 'Yesterday' },
		{ value: 'this_week', label: 'This Week' },
		{ value: 'last_7_days', label: 'Last 7 Days' },
		{ value: 'this_month', label: 'This Month' },
		{ value: 'last_30_days', label: 'Last 30 Days' },
		{ value: 'this_year', label: 'This Year' },
		{ value: 'custom', label: 'Custom Range' }
	];

	const DONUT_COLORS = ['#10b981', '#0ea5e9', '#f59e0b', '#8b5cf6', '#ec4899'];

	let dateRange = $state('last_30_days');
	let dateFrom = $state('');
	let dateTo = $state('');
	let customerSearch = $state('');
	let sortKey = $state<'name' | 'balance' | 'period_spent' | 'messages_count'>('period_spent');
	let sortDir = $state<'asc' | 'desc'>('desc');

	let data = $state<FinancialAnalyticsData | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	// Interactive chart hover states
	let hoverDepositIdx = $state<number | null>(null);
	let hoverSpendIdx = $state<number | null>(null);
	let hoverMethodIdx = $state<number | null>(null);
	let hoverTxIdx = $state<number | null>(null);

	let requestSeq = 0;

	const fmtMoney = (n: number): string =>
		Number(n || 0).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
	const fmtInt = (n: number): string => Math.round(Number(n || 0)).toLocaleString('en-US');

	async function loadAnalytics() {
		const seq = ++requestSeq;
		loading = true;
		error = null;
		try {
			const params = new URLSearchParams({
				date_range: dateRange,
				sort: sortKey,
				dir: sortDir
			});
			if (dateRange === 'custom') {
				if (dateFrom) params.set('date_from', dateFrom);
				if (dateTo) params.set('date_to', dateTo);
			}
			if (customerSearch.trim()) params.set('search', customerSearch.trim());

			const res = await supermanApi.get<{ data: FinancialAnalyticsData }>(
				`/api/v1/superman/analytics/financial?${params.toString()}`
			);
			if (seq !== requestSeq) return;
			data = res.data;
		} catch (err: unknown) {
			if (seq !== requestSeq) return;
			error = err instanceof Error ? err.message : 'Failed to load financial analytics.';
		} finally {
			if (seq === requestSeq) loading = false;
		}
	}

	let searchTimer: ReturnType<typeof setTimeout> | undefined;
	function onSearchInput() {
		clearTimeout(searchTimer);
		searchTimer = setTimeout(() => void loadAnalytics(), 300);
	}

	function toggleSort(key: 'name' | 'balance' | 'period_spent' | 'messages_count') {
		if (sortKey === key) {
			sortDir = sortDir === 'asc' ? 'desc' : 'asc';
		} else {
			sortKey = key;
			sortDir = key === 'name' ? 'asc' : 'desc';
		}
		void loadAnalytics();
	}

	onMount(() => {
		const today = new Date();
		const thirtyAgo = new Date(today.getTime() - 30 * 86400 * 1000);
		dateTo = today.toISOString().slice(0, 10);
		dateFrom = thirtyAgo.toISOString().slice(0, 10);
		void loadAnalytics();
	});

	// --- Chart geometry helpers ---
	const W = 560;
	const H = 260;
	const PAD_L = 56;
	const PAD_R = 16;
	const PAD_T = 16;
	const PAD_B = 52;
	const PLOT_W = W - PAD_L - PAD_R;
	const PLOT_H = H - PAD_T - PAD_B;

	function niceScale(maxValue: number, tickCount = 6): { max: number; ticks: number[] } {
		if (!Number.isFinite(maxValue) || maxValue <= 0) {
			return { max: 5, ticks: [0, 1, 2, 3, 4, 5] };
		}
		const roughStep = maxValue / tickCount;
		const mag = Math.pow(10, Math.floor(Math.log10(roughStep)));
		const residual = roughStep / mag;
		const niceFactor = residual <= 1 ? 1 : residual <= 2 ? 2 : residual <= 2.5 ? 2.5 : residual <= 5 ? 5 : 10;
		const step = niceFactor * mag;
		const top = Math.ceil(maxValue / step) * step;
		const ticks: number[] = [];
		for (let v = 0; v <= top + step * 0.001; v += step) {
			ticks.push(Math.round(v * 100) / 100);
		}
		return { max: top, ticks };
	}

	function labelStep(count: number): number {
		if (count <= 14) return 1;
		return Math.ceil(count / 14);
	}

	function smoothPath(points: Array<{ x: number; y: number }>, baselineY: number): string {
		if (points.length === 0) return '';
		if (points.length === 1) return `M ${points[0].x} ${points[0].y}`;
		let d = `M ${points[0].x.toFixed(1)} ${points[0].y.toFixed(1)}`;
		const tension = 0.25;
		for (let i = 0; i < points.length - 1; i++) {
			const p0 = points[Math.max(0, i - 1)];
			const p1 = points[i];
			const p2 = points[i + 1];
			const p3 = points[Math.min(points.length - 1, i + 2)];
			const cp1x = p1.x + (p2.x - p0.x) * tension;
			const cp1y = Math.min(baselineY, Math.max(PAD_T, p1.y + (p2.y - p0.y) * tension));
			const cp2x = p2.x - (p3.x - p1.x) * tension;
			const cp2y = Math.min(baselineY, Math.max(PAD_T, p2.y - (p3.y - p1.y) * tension));
			d += ` C ${cp1x.toFixed(1)} ${cp1y.toFixed(1)}, ${cp2x.toFixed(1)} ${cp2y.toFixed(1)}, ${p2.x.toFixed(1)} ${p2.y.toFixed(1)}`;
		}
		return d;
	}

	function areaPath(points: Array<{ x: number; y: number }>, baselineY: number): string {
		if (points.length === 0) return '';
		const line = smoothPath(points, baselineY);
		const last = points[points.length - 1];
		const first = points[0];
		return `${line} L ${last.x.toFixed(1)} ${baselineY.toFixed(1)} L ${first.x.toFixed(1)} ${baselineY.toFixed(1)} Z`;
	}

	interface DonutSlice {
		label: string;
		total: number;
		count: number;
		pct: number;
		color: string;
		path: string;
		isFullRing: boolean;
	}

	function buildDonutSlices(items: DepositMethodPoint[]): DonutSlice[] {
		const grandTotal = items.reduce((acc, i) => acc + Math.max(0, i.total), 0);
		if (grandTotal <= 0) return [];
		const cx = 130;
		const cy = 125;
		const rOut = 108;
		const rIn = 54;
		const active = items.filter((i) => i.total > 0);
		let angle = -Math.PI / 2;

		return active.map((item, idx) => {
			const frac = item.total / grandTotal;
			const color = DONUT_COLORS[idx % DONUT_COLORS.length];
			if (active.length === 1 || frac >= 0.9999) {
				const d = [
					`M ${cx} ${cy - rOut}`,
					`A ${rOut} ${rOut} 0 1 1 ${cx} ${cy + rOut}`,
					`A ${rOut} ${rOut} 0 1 1 ${cx} ${cy - rOut}`,
					`M ${cx} ${cy - rIn}`,
					`A ${rIn} ${rIn} 0 1 0 ${cx} ${cy + rIn}`,
					`A ${rIn} ${rIn} 0 1 0 ${cx} ${cy - rIn}`,
					'Z'
				].join(' ');
				return { label: item.label, total: item.total, count: item.count, pct: 100, color, path: d, isFullRing: true };
			}
			const sweep = frac * Math.PI * 2;
			const start = angle;
			const end = angle + sweep;
			angle = end;
			const largeArc = sweep > Math.PI ? 1 : 0;
			const x1 = cx + rOut * Math.cos(start);
			const y1 = cy + rOut * Math.sin(start);
			const x2 = cx + rOut * Math.cos(end);
			const y2 = cy + rOut * Math.sin(end);
			const x3 = cx + rIn * Math.cos(end);
			const y3 = cy + rIn * Math.sin(end);
			const x4 = cx + rIn * Math.cos(start);
			const y4 = cy + rIn * Math.sin(start);
			const d = [
				`M ${x1.toFixed(2)} ${y1.toFixed(2)}`,
				`A ${rOut} ${rOut} 0 ${largeArc} 1 ${x2.toFixed(2)} ${y2.toFixed(2)}`,
				`L ${x3.toFixed(2)} ${y3.toFixed(2)}`,
				`A ${rIn} ${rIn} 0 ${largeArc} 0 ${x4.toFixed(2)} ${y4.toFixed(2)}`,
				'Z'
			].join(' ');
			return {
				label: item.label,
				total: item.total,
				count: item.count,
				pct: Math.round(frac * 1000) / 10,
				color,
				path: d,
				isFullRing: false
			};
		});
	}

	let depositScale = $derived(niceScale(Math.max(0, ...(data?.deposit_trend ?? []).map((p) => p.total))));
	let spendScale = $derived(
		niceScale(Math.max(0, ...(data?.spending_trend ?? []).flatMap((p) => [p.revenue, p.cost])))
	);
	let txScale = $derived(
		niceScale(Math.max(0, ...(data?.transaction_volume ?? []).flatMap((p) => [p.credits, p.debits])))
	);
	let donutSlices = $derived(buildDonutSlices(data?.deposits_by_method ?? []));

	let txCreditPoints = $derived.by(() => {
		const rows = data?.transaction_volume ?? [];
		if (rows.length === 0) return [];
		return rows.map((row, idx) => ({
			x: rows.length === 1 ? PAD_L + PLOT_W / 2 : PAD_L + (idx / (rows.length - 1)) * PLOT_W,
			y: PAD_T + PLOT_H - (row.credits / txScale.max) * PLOT_H
		}));
	});

	let txDebitPoints = $derived.by(() => {
		const rows = data?.transaction_volume ?? [];
		if (rows.length === 0) return [];
		return rows.map((row, idx) => ({
			x: rows.length === 1 ? PAD_L + PLOT_W / 2 : PAD_L + (idx / (rows.length - 1)) * PLOT_W,
			y: PAD_T + PLOT_H - (row.debits / txScale.max) * PLOT_H
		}));
	});

	function statusBadgeClass(status: string): string {
		switch (status.toLowerCase()) {
			case 'approved':
			case 'active':
				return 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-300';
			case 'pending':
				return 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-300';
			default:
				return 'border-rose-200 bg-rose-50 text-rose-700 dark:border-rose-900 dark:bg-rose-950/40 dark:text-rose-300';
		}
	}

	function formatStatus(status: string): string {
		if (!status) return 'Unknown';
		return status.charAt(0).toUpperCase() + status.slice(1).toLowerCase();
	}

	const statCard = 'rounded-xl border border-gray-200 bg-white p-6 shadow-xs dark:border-gray-800 dark:bg-gray-900';
	const chartCard = 'rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900';
</script>

<svelte:head>
	<title>Financial Analytics — SMS Whiteinfo Administrator</title>
</svelte:head>

{#snippet sortArrow(key: 'name' | 'balance' | 'period_spent' | 'messages_count')}
	<svg
		class="h-3.5 w-3.5 transition-transform {sortKey === key ? 'text-emerald-600' : 'text-gray-400'} {sortKey === key && sortDir === 'asc' ? 'rotate-180' : ''}"
		fill="none"
		viewBox="0 0 24 24"
		stroke-width="2"
		stroke="currentColor"
	>
		<path stroke-linecap="round" stroke-linejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" />
	</svg>
{/snippet}

<div class="space-y-6">
	<!-- Page Heading -->
	<div>
		<h1 class="text-3xl font-bold tracking-tight text-gray-950 dark:text-white">Financial Analytics</h1>
	</div>

	<!-- Date Range Filter (matches Filament HasDateFilter) -->
	<div class="grid grid-cols-1 gap-4 md:grid-cols-12">
		<div class="md:col-span-4 xl:col-span-3">
			<label for="financial-date-range" class="block text-sm font-medium text-gray-800 dark:text-gray-200">
				Date Range
			</label>
			<select
				id="financial-date-range"
				bind:value={dateRange}
				onchange={() => void loadAnalytics()}
				class="mt-1.5 w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-xs outline-none focus:border-emerald-600 focus:ring-1 focus:ring-emerald-600 dark:border-gray-700 dark:bg-gray-900 dark:text-white"
			>
				{#each DATE_RANGE_OPTIONS as opt (opt.value)}
					<option value={opt.value}>{opt.label}</option>
				{/each}
			</select>
		</div>

		{#if dateRange === 'custom'}
			<div class="md:col-span-4 xl:col-span-3">
				<label for="financial-date-from" class="block text-sm font-medium text-gray-800 dark:text-gray-200">
					From
				</label>
				<input
					id="financial-date-from"
					type="date"
					bind:value={dateFrom}
					onchange={() => void loadAnalytics()}
					class="mt-1.5 w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-xs outline-none focus:border-emerald-600 focus:ring-1 focus:ring-emerald-600 dark:border-gray-700 dark:bg-gray-900 dark:text-white"
				/>
			</div>
			<div class="md:col-span-4 xl:col-span-3">
				<label for="financial-date-to" class="block text-sm font-medium text-gray-800 dark:text-gray-200">
					To
				</label>
				<input
					id="financial-date-to"
					type="date"
					bind:value={dateTo}
					onchange={() => void loadAnalytics()}
					class="mt-1.5 w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-xs outline-none focus:border-emerald-600 focus:ring-1 focus:ring-emerald-600 dark:border-gray-700 dark:bg-gray-900 dark:text-white"
				/>
			</div>
		{/if}
	</div>

	{#if error}
		<div role="alert" class="rounded-xl border border-rose-200 bg-rose-50 p-4 text-sm text-rose-700 dark:border-rose-900 dark:bg-rose-950/30 dark:text-rose-300">
			{error}
		</div>
	{/if}

	{#if loading && !data}
		<div class="grid grid-cols-1 gap-4 md:grid-cols-3">
			{#each Array(6) as _}
				<div class="{statCard} h-32 animate-pulse bg-gray-100/60 dark:bg-gray-800/40"></div>
			{/each}
		</div>
	{:else if data}
		<!-- Widget 1: FinancialStatsWidget (3 columns x 2 rows) -->
		<div class="grid grid-cols-1 gap-4 md:grid-cols-3 {loading ? 'opacity-70 transition-opacity' : ''}">
			<!-- Deposits -->
			<div class={statCard}>
				<div class="flex items-center gap-2 text-sm font-medium text-gray-500 dark:text-gray-400">
					<svg class="h-4 w-4 shrink-0 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M2.25 18.75a60.07 60.07 0 0 1 15.797 2.101c.727.198 1.453-.342 1.453-1.096V18.75M3.75 4.5v.75A.75.75 0 0 1 3 6h-.75m0 0v-.375c0-.621.504-1.125 1.125-1.125H20.25M2.25 6v9m18-10.5v.75c0 .414.336.75.75.75h.75m-1.5-1.5h.375c.621 0 1.125.504 1.125 1.125v9.75c0 .621-.504 1.125-1.125 1.125h-.375m1.5-1.5H21a.75.75 0 0 0-.75.75v.75m0 0H3.75m0 0h-.375a1.125 1.125 0 0 1-1.125-1.125V15m1.5 1.5v-.75A.75.75 0 0 0 3 15h-.75M15 10.5a3 3 0 1 1-6 0 3 3 0 0 1 6 0Zm3 0h.008v.008H18V10.5Zm-12 0h.008v.008H6V10.5Z" />
					</svg>
					<span>Deposits</span>
				</div>
				<p class="mt-2 text-3xl font-semibold tracking-tight tabular-nums text-gray-950 dark:text-white">
					{fmtMoney(data.period_stats.deposits)} BDT
				</p>
				<p class="mt-1.5 text-sm font-medium text-emerald-600 dark:text-emerald-400">Completed in period</p>
			</div>

			<!-- Customer Spend -->
			<div class={statCard}>
				<div class="flex items-center gap-2 text-sm font-medium text-gray-500 dark:text-gray-400">
					<svg class="h-4 w-4 shrink-0 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M2.25 3h1.386c.51 0 .955.343 1.087.835l.383 1.437M7.5 14.25a3 3 0 0 0-3 3h15.75m-12.75-3h11.218c1.121-2.3 2.1-4.684 2.924-7.138a60.114 60.114 0 0 0-16.536-1.84M7.5 14.25 5.106 5.272M6 20.25a.75.75 0 1 1-1.5 0 .75.75 0 0 1 1.5 0Zm12.75 0a.75.75 0 1 1-1.5 0 .75.75 0 0 1 1.5 0Z" />
					</svg>
					<span>Customer Spend</span>
				</div>
				<p class="mt-2 text-3xl font-semibold tracking-tight tabular-nums text-gray-950 dark:text-white">
					{fmtMoney(data.period_stats.customer_spend)} BDT
				</p>
				<p class="mt-1.5 text-sm font-medium text-amber-600 dark:text-amber-400">Message revenue</p>
			</div>

			<!-- Gateway Cost -->
			<div class={statCard}>
				<div class="flex items-center gap-2 text-sm font-medium text-gray-500 dark:text-gray-400">
					<svg class="h-4 w-4 shrink-0 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M2.25 6 9 12.75l4.286-4.286a11.948 11.948 0 0 1 4.306 6.43l.776 2.898m0 0 3.182-5.511m-3.182 5.51-5.511-3.181" />
					</svg>
					<span>Gateway Cost</span>
				</div>
				<p class="mt-2 text-3xl font-semibold tracking-tight tabular-nums text-gray-950 dark:text-white">
					{fmtMoney(data.period_stats.gateway_cost)} BDT
				</p>
				<p class="mt-1.5 text-sm font-medium text-rose-600 dark:text-rose-400">Platform cost</p>
			</div>

			<!-- Pending Deposits -->
			<div class={statCard}>
				<div class="flex items-center gap-2 text-sm font-medium text-gray-500 dark:text-gray-400">
					<svg class="h-4 w-4 shrink-0 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
					</svg>
					<span>Pending Deposits</span>
				</div>
				<p class="mt-2 text-3xl font-semibold tracking-tight tabular-nums text-gray-950 dark:text-white">
					{fmtMoney(data.period_stats.pending_deposits)} BDT
				</p>
				<p class="mt-1.5 text-sm font-medium text-sky-600 dark:text-sky-400">Awaiting approval</p>
			</div>

			<!-- Transactions -->
			<div class={statCard}>
				<div class="flex items-center gap-2 text-sm font-medium text-gray-500 dark:text-gray-400">
					<svg class="h-4 w-4 shrink-0 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M7.5 21 3 16.5m0 0L7.5 12M3 16.5h13.5m0-13.5L21 7.5m0 0L16.5 12M21 7.5H7.5" />
					</svg>
					<span>Transactions</span>
				</div>
				<p class="mt-2 text-3xl font-semibold tracking-tight tabular-nums text-gray-950 dark:text-white">
					{fmtInt(data.period_stats.transactions)}
				</p>
				<p class="mt-1.5 text-sm font-medium text-gray-500 dark:text-gray-400">In selected period</p>
			</div>

			<!-- System Balance -->
			<div class={statCard}>
				<div class="flex items-center gap-2 text-sm font-medium text-gray-500 dark:text-gray-400">
					<svg class="h-4 w-4 shrink-0 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M21 12a2.25 2.25 0 0 0-2.25-2.25H15a3 3 0 1 1-6 0H5.25A2.25 2.25 0 0 0 3 12m18 0v6a2.25 2.25 0 0 1-2.25 2.25H5.25A2.25 2.25 0 0 1 3 18v-6m18 0V9M3 12V9m18 0a2.25 2.25 0 0 0-2.25-2.25H5.25A2.25 2.25 0 0 0 3 9m18 0V6a2.25 2.25 0 0 0-2.25-2.25H5.25A2.25 2.25 0 0 0 3 6v3" />
					</svg>
					<span>System Balance</span>
				</div>
				<p class="mt-2 text-3xl font-semibold tracking-tight tabular-nums text-gray-950 dark:text-white">
					{fmtMoney(data.period_stats.system_balance)} BDT
				</p>
				<p class="mt-1.5 text-sm font-medium text-emerald-600 dark:text-emerald-400">Total user balances</p>
			</div>
		</div>

		<!-- Widget 2: RevenueStatsWidget (4 columns) -->
		<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
			<div class={statCard}>
				<div class="flex items-center gap-2 text-sm font-medium text-gray-500 dark:text-gray-400">
					<svg class="h-4 w-4 shrink-0 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M2.25 18.75a60.07 60.07 0 0 1 15.797 2.101c.727.198 1.453-.342 1.453-1.096V18.75M3.75 4.5v.75A.75.75 0 0 1 3 6h-.75m0 0v-.375c0-.621.504-1.125 1.125-1.125H20.25M2.25 6v9m18-10.5v.75c0 .414.336.75.75.75h.75m-1.5-1.5h.375c.621 0 1.125.504 1.125 1.125v9.75c0 .621-.504 1.125-1.125 1.125h-.375m1.5-1.5H21a.75.75 0 0 0-.75.75v.75m0 0H3.75m0 0h-.375a1.125 1.125 0 0 1-1.125-1.125V15m1.5 1.5v-.75A.75.75 0 0 0 3 15h-.75M15 10.5a3 3 0 1 1-6 0 3 3 0 0 1 6 0Zm3 0h.008v.008H18V10.5Zm-12 0h.008v.008H6V10.5Z" />
					</svg>
					<span>Total Deposits</span>
				</div>
				<p class="mt-2 text-3xl font-semibold tracking-tight tabular-nums text-gray-950 dark:text-white">
					{fmtMoney(data.revenue_stats.total_deposits)} BDT
				</p>
				<p class="mt-1.5 text-sm font-medium text-emerald-600 dark:text-emerald-400">Completed deposits</p>
			</div>

			<div class={statCard}>
				<div class="flex items-center gap-2 text-sm font-medium text-gray-500 dark:text-gray-400">
					<svg class="h-4 w-4 shrink-0 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
					</svg>
					<span>Pending Deposits</span>
				</div>
				<p class="mt-2 text-3xl font-semibold tracking-tight tabular-nums text-gray-950 dark:text-white">
					{fmtMoney(data.revenue_stats.pending_deposits)} BDT
				</p>
				<p class="mt-1.5 text-sm font-medium text-amber-600 dark:text-amber-400">Awaiting approval</p>
			</div>

			<div class={statCard}>
				<div class="flex items-center gap-2 text-sm font-medium text-gray-500 dark:text-gray-400">
					<svg class="h-4 w-4 shrink-0 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M2.25 6 9 12.75l4.286-4.286a11.948 11.948 0 0 1 4.306 6.43l.776 2.898m0 0 3.182-5.511m-3.182 5.51-5.511-3.181" />
					</svg>
					<span>Total Spent</span>
				</div>
				<p class="mt-2 text-3xl font-semibold tracking-tight tabular-nums text-gray-950 dark:text-white">
					{fmtMoney(data.revenue_stats.total_spent)} BDT
				</p>
				<p class="mt-1.5 text-sm font-medium text-rose-600 dark:text-rose-400">Message costs</p>
			</div>

			<div class={statCard}>
				<div class="flex items-center gap-2 text-sm font-medium text-gray-500 dark:text-gray-400">
					<svg class="h-4 w-4 shrink-0 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M21 12a2.25 2.25 0 0 0-2.25-2.25H15a3 3 0 1 1-6 0H5.25A2.25 2.25 0 0 0 3 12m18 0v6a2.25 2.25 0 0 1-2.25 2.25H5.25A2.25 2.25 0 0 1 3 18v-6m18 0V9M3 12V9m18 0a2.25 2.25 0 0 0-2.25-2.25H5.25A2.25 2.25 0 0 0 3 9m18 0V6a2.25 2.25 0 0 0-2.25-2.25H5.25A2.25 2.25 0 0 0 3 6v3" />
					</svg>
					<span>System Balance</span>
				</div>
				<p class="mt-2 text-3xl font-semibold tracking-tight tabular-nums text-gray-950 dark:text-white">
					{fmtMoney(data.revenue_stats.system_balance)} BDT
				</p>
				<p class="mt-1.5 text-sm font-medium text-emerald-600 dark:text-emerald-400">Total user balances</p>
			</div>
		</div>

		<!-- Widget 3: UserStatsWidget (4 columns) -->
		<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
			<div class={statCard}>
				<div class="flex items-center gap-2 text-sm font-medium text-gray-500 dark:text-gray-400">
					<svg class="h-4 w-4 shrink-0 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M18 7.5v3m0 0v3m0-3h3m-3 0h-3m-2.25-4.125a3.375 3.375 0 1 1-6.75 0 3.375 3.375 0 0 1 6.75 0ZM3 19.235v-.11a6.375 6.375 0 0 1 12.75 0v.109A12.318 12.318 0 0 1 9.374 21c-2.331 0-4.512-.645-6.374-1.766Z" />
					</svg>
					<span>New Users (30d)</span>
				</div>
				<p class="mt-2 text-3xl font-semibold tracking-tight tabular-nums text-gray-950 dark:text-white">
					{fmtInt(data.user_stats.new_users_30d)}
				</p>
				<p class="mt-1.5 text-sm font-medium text-gray-500 dark:text-gray-400">Recent registrations</p>
			</div>

			<div class={statCard}>
				<div class="flex items-center gap-2 text-sm font-medium text-gray-500 dark:text-gray-400">
					<svg class="h-4 w-4 shrink-0 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
					</svg>
					<span>Pending Approvals</span>
				</div>
				<p class="mt-2 text-3xl font-semibold tracking-tight tabular-nums text-gray-950 dark:text-white">
					{fmtInt(data.user_stats.pending_approvals)}
				</p>
				<p class="mt-1.5 text-sm font-medium text-amber-600 dark:text-amber-400">Awaiting review</p>
			</div>

			<div class={statCard}>
				<div class="flex items-center gap-2 text-sm font-medium text-gray-500 dark:text-gray-400">
					<svg class="h-4 w-4 shrink-0 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M15.75 5.25a3 3 0 0 1 3 3m3 0a6 6 0 0 1-7.029 5.912c-.563-.097-1.159.026-1.563.43L10.5 17.25H8.25v2.25H6v2.25H2.25v-2.818c0-.597.237-1.17.659-1.591l6.499-6.499c.404-.404.527-1 .43-1.563A6 6 0 1 1 21.75 8.25Z" />
					</svg>
					<span>API Users</span>
				</div>
				<p class="mt-2 text-3xl font-semibold tracking-tight tabular-nums text-gray-950 dark:text-white">
					{fmtInt(data.user_stats.api_users)}
				</p>
				<p class="mt-1.5 text-sm font-medium text-sky-600 dark:text-sky-400">With API access</p>
			</div>

			<div class={statCard}>
				<div class="flex items-center gap-2 text-sm font-medium text-gray-500 dark:text-gray-400">
					<svg class="h-4 w-4 shrink-0 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M12 21v-8.25M15.75 21v-8.25M8.25 21v-8.25M3 9l9-6 9 6m-1.5 12V10.332A48.36 48.36 0 0 0 12 9.75c-2.551 0-5.056.2-7.5.582V21M3 21h18M12 6.75h.008v.008H12V6.75Z" />
					</svg>
					<span>Total Balance</span>
				</div>
				<p class="mt-2 text-3xl font-semibold tracking-tight tabular-nums text-gray-950 dark:text-white">
					{fmtMoney(data.user_stats.total_balance)} BDT
				</p>
				<p class="mt-1.5 text-sm font-medium text-emerald-600 dark:text-emerald-400">Across all users</p>
			</div>
		</div>

		<!-- Charts Grid (2 columns x 2 rows) -->
		<div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
			<!-- Widget 4: Deposit Trend -->
			<section class={chartCard}>
				<header class="border-b border-gray-100 px-6 py-4 dark:border-gray-800">
					<h2 class="text-base font-semibold text-gray-950 dark:text-white">Deposit Trend</h2>
					<p class="mt-0.5 text-sm text-gray-500 dark:text-gray-400">Daily deposit amounts</p>
				</header>
				<div class="p-5">
					{#if data.deposit_trend.length === 0}
						<div class="flex h-64 items-center justify-center text-sm text-gray-400">No completed deposits in selected period.</div>
					{:else}
						{@const rows = data.deposit_trend}
						{@const slot = PLOT_W / rows.length}
						{@const barW = Math.max(4, Math.min(26, slot * 0.62))}
						{@const step = labelStep(rows.length)}
						<div class="relative">
							<svg viewBox="0 0 {W} {H}" class="h-64 w-full overflow-visible" role="img" aria-label="Deposit Trend Chart">
								{#each depositScale.ticks as tick (tick)}
									{@const y = PAD_T + PLOT_H - (tick / depositScale.max) * PLOT_H}
									<line x1={PAD_L} y1={y} x2={W - PAD_R} y2={y} stroke="currentColor" class="text-gray-200 dark:text-gray-800" stroke-width="1" />
									<text x={PAD_L - 8} y={y + 4} text-anchor="end" class="fill-gray-500 text-[10px] dark:fill-gray-400">{fmtInt(tick)}</text>
								{/each}
								{#each rows as row, idx (row.date)}
									{@const cx = PAD_L + idx * slot + slot / 2}
									{@const bh = (row.total / depositScale.max) * PLOT_H}
									{@const by = PAD_T + PLOT_H - bh}
									<g
										onmouseenter={() => (hoverDepositIdx = idx)}
										onmouseleave={() => (hoverDepositIdx = null)}
										role="presentation"
									>
										<rect x={PAD_L + idx * slot} y={PAD_T} width={slot} height={PLOT_H} fill="transparent" />
										<rect
											x={cx - barW / 2}
											y={by}
											width={barW}
											height={Math.max(0, bh)}
											fill={hoverDepositIdx === idx ? 'rgba(16, 185, 129, 0.85)' : 'rgba(16, 185, 129, 0.6)'}
											stroke="#10b981"
											stroke-width="1.5"
											rx="1"
										/>
									</g>
									{#if idx % step === 0}
										<text
											x={cx}
											y={PAD_T + PLOT_H + 14}
											text-anchor="end"
											transform="rotate(-45 {cx} {PAD_T + PLOT_H + 14})"
											class="fill-gray-500 text-[10px] dark:fill-gray-400"
										>{row.label}</text>
									{/if}
								{/each}
							</svg>
							{#if hoverDepositIdx !== null && rows[hoverDepositIdx]}
								{@const item = rows[hoverDepositIdx]}
								<div class="pointer-events-none absolute right-4 top-2 rounded-lg border border-gray-200 bg-gray-900/95 px-3 py-1.5 text-xs text-white shadow-lg">
									<div class="font-semibold">{item.label}</div>
									<div class="text-emerald-300">Deposits: {fmtMoney(item.total)} BDT</div>
								</div>
							{/if}
						</div>
						<div class="mt-2 flex items-center justify-center gap-2 text-xs text-gray-600 dark:text-gray-400">
							<span class="inline-block h-3 w-3 rounded-[2px] border border-emerald-500 bg-emerald-500/60"></span>
							<span>Deposits (BDT)</span>
						</div>
					{/if}
				</div>
			</section>

			<!-- Widget 5: Spending Trend -->
			<section class={chartCard}>
				<header class="border-b border-gray-100 px-6 py-4 dark:border-gray-800">
					<h2 class="text-base font-semibold text-gray-950 dark:text-white">Spending Trend</h2>
					<p class="mt-0.5 text-sm text-gray-500 dark:text-gray-400">Daily customer spending</p>
				</header>
				<div class="p-5">
					{#if data.spending_trend.length === 0}
						<div class="flex h-64 items-center justify-center text-sm text-gray-400">No spending activity in selected period.</div>
					{:else}
						{@const rows = data.spending_trend}
						{@const slot = PLOT_W / rows.length}
						{@const barW = Math.max(3, Math.min(14, slot * 0.36))}
						{@const step = labelStep(rows.length)}
						<div class="relative">
							<svg viewBox="0 0 {W} {H}" class="h-64 w-full overflow-visible" role="img" aria-label="Spending Trend Chart">
								{#each spendScale.ticks as tick (tick)}
									{@const y = PAD_T + PLOT_H - (tick / spendScale.max) * PLOT_H}
									<line x1={PAD_L} y1={y} x2={W - PAD_R} y2={y} stroke="currentColor" class="text-gray-200 dark:text-gray-800" stroke-width="1" />
									<text x={PAD_L - 8} y={y + 4} text-anchor="end" class="fill-gray-500 text-[10px] dark:fill-gray-400">{fmtInt(tick)}</text>
								{/each}
								{#each rows as row, idx (row.date)}
									{@const cx = PAD_L + idx * slot + slot / 2}
									{@const revH = (row.revenue / spendScale.max) * PLOT_H}
									{@const costH = (row.cost / spendScale.max) * PLOT_H}
									<g
										onmouseenter={() => (hoverSpendIdx = idx)}
										onmouseleave={() => (hoverSpendIdx = null)}
										role="presentation"
									>
										<rect x={PAD_L + idx * slot} y={PAD_T} width={slot} height={PLOT_H} fill="transparent" />
										<rect
											x={cx - barW - 1}
											y={PAD_T + PLOT_H - revH}
											width={barW}
											height={Math.max(0, revH)}
											fill={hoverSpendIdx === idx ? 'rgba(14, 165, 233, 0.9)' : 'rgba(14, 165, 233, 0.65)'}
											stroke="#0ea5e9"
											stroke-width="1.2"
											rx="1"
										/>
										<rect
											x={cx + 1}
											y={PAD_T + PLOT_H - costH}
											width={barW}
											height={Math.max(0, costH)}
											fill={hoverSpendIdx === idx ? 'rgba(239, 68, 68, 0.9)' : 'rgba(239, 68, 68, 0.65)'}
											stroke="#ef4444"
											stroke-width="1.2"
											rx="1"
										/>
									</g>
									{#if idx % step === 0}
										<text
											x={cx}
											y={PAD_T + PLOT_H + 14}
											text-anchor="end"
											transform="rotate(-45 {cx} {PAD_T + PLOT_H + 14})"
											class="fill-gray-500 text-[10px] dark:fill-gray-400"
										>{row.label}</text>
									{/if}
								{/each}
							</svg>
							{#if hoverSpendIdx !== null && rows[hoverSpendIdx]}
								{@const item = rows[hoverSpendIdx]}
								<div class="pointer-events-none absolute right-4 top-2 rounded-lg border border-gray-200 bg-gray-900/95 px-3 py-1.5 text-xs text-white shadow-lg">
									<div class="font-semibold">{item.label}</div>
									<div class="text-sky-300">Revenue: {fmtMoney(item.revenue)} BDT</div>
									<div class="text-rose-300">Gateway Cost: {fmtMoney(item.cost)} BDT</div>
								</div>
							{/if}
						</div>
						<div class="mt-2 flex flex-wrap items-center justify-center gap-5 text-xs text-gray-600 dark:text-gray-400">
							<span class="inline-flex items-center gap-2">
								<span class="inline-block h-3 w-3 rounded-[2px] border border-sky-500 bg-sky-500/60"></span>
								Revenue (BDT)
							</span>
							<span class="inline-flex items-center gap-2">
								<span class="inline-block h-3 w-3 rounded-[2px] border border-rose-500 bg-rose-500/60"></span>
								Gateway Cost (BDT)
							</span>
						</div>
					{/if}
				</div>
			</section>

			<!-- Widget 6: Deposits by Method -->
			<section class={chartCard}>
				<header class="border-b border-gray-100 px-6 py-4 dark:border-gray-800">
					<h2 class="text-base font-semibold text-gray-950 dark:text-white">Deposits by Method</h2>
					<p class="mt-0.5 text-sm text-gray-500 dark:text-gray-400">Payment method distribution</p>
				</header>
				<div class="p-5">
					{#if donutSlices.length === 0}
						<div class="flex h-64 items-center justify-center text-sm text-gray-400">No deposits by payment method in selected period.</div>
					{:else}
						<div class="relative flex flex-col items-center">
							<svg viewBox="0 0 260 250" class="h-64 w-64" role="img" aria-label="Deposits by Method Doughnut Chart">
								{#each donutSlices as slice, idx (slice.label)}
									<path
										d={slice.path}
										fill={slice.color}
										fill-rule={slice.isFullRing ? 'evenodd' : 'nonzero'}
										stroke="#ffffff"
										stroke-width="2"
										opacity={hoverMethodIdx === null || hoverMethodIdx === idx ? 1 : 0.72}
										class="cursor-pointer transition-opacity"
										onmouseenter={() => (hoverMethodIdx = idx)}
										onmouseleave={() => (hoverMethodIdx = null)}
										role="presentation"
									/>
								{/each}
							</svg>
							{#if hoverMethodIdx !== null && donutSlices[hoverMethodIdx]}
								{@const s = donutSlices[hoverMethodIdx]}
								<div class="pointer-events-none absolute right-2 top-2 rounded-lg border border-gray-200 bg-gray-900/95 px-3 py-1.5 text-xs text-white shadow-lg">
									<div class="font-semibold">{s.label}</div>
									<div>{fmtMoney(s.total)} BDT ({s.pct}%)</div>
									<div class="text-gray-300">{fmtInt(s.count)} deposits</div>
								</div>
							{/if}
						</div>
						<div class="mt-2 flex flex-wrap items-center justify-center gap-5 text-xs text-gray-600 dark:text-gray-400">
							{#each donutSlices as slice (slice.label)}
								<span class="inline-flex items-center gap-2">
									<span class="inline-block h-3 w-3 rounded-[2px]" style="background-color: {slice.color};"></span>
									{slice.label}
								</span>
							{/each}
						</div>
					{/if}
				</div>
			</section>

			<!-- Widget 7: Transaction Volume -->
			<section class={chartCard}>
				<header class="border-b border-gray-100 px-6 py-4 dark:border-gray-800">
					<h2 class="text-base font-semibold text-gray-950 dark:text-white">Transaction Volume</h2>
					<p class="mt-0.5 text-sm text-gray-500 dark:text-gray-400">Daily credit vs debit transactions</p>
				</header>
				<div class="p-5">
					{#if data.transaction_volume.length === 0}
						<div class="flex h-64 items-center justify-center text-sm text-gray-400">No transactions in selected period.</div>
					{:else}
						{@const rows = data.transaction_volume}
						{@const base = PAD_T + PLOT_H}
						{@const step = labelStep(rows.length)}
						<div class="relative">
							<svg viewBox="0 0 {W} {H}" class="h-64 w-full overflow-visible" role="img" aria-label="Transaction Volume Chart">
								{#each txScale.ticks as tick (tick)}
									{@const y = PAD_T + PLOT_H - (tick / txScale.max) * PLOT_H}
									<line x1={PAD_L} y1={y} x2={W - PAD_R} y2={y} stroke="currentColor" class="text-gray-200 dark:text-gray-800" stroke-width="1" />
									<text x={PAD_L - 8} y={y + 4} text-anchor="end" class="fill-gray-500 text-[10px] dark:fill-gray-400">{fmtInt(tick)}</text>
								{/each}

								<!-- Debits area & line -->
								<path d={areaPath(txDebitPoints, base)} fill="rgba(239, 68, 68, 0.1)" />
								<path d={smoothPath(txDebitPoints, base)} fill="none" stroke="#ef4444" stroke-width="2" />

								<!-- Credits area & line -->
								<path d={areaPath(txCreditPoints, base)} fill="rgba(16, 185, 129, 0.12)" />
								<path d={smoothPath(txCreditPoints, base)} fill="none" stroke="#10b981" stroke-width="2" />

								<!-- Points & hover hit columns -->
								{#each rows as row, idx (row.date)}
									{@const cp = txCreditPoints[idx]}
									{@const dp = txDebitPoints[idx]}
									{@const colW = rows.length > 1 ? PLOT_W / (rows.length - 1) : PLOT_W}
									{#if hoverTxIdx === idx}
										<line x1={cp.x} y1={PAD_T} x2={cp.x} y2={base} stroke="#9ca3af" stroke-dasharray="3 3" stroke-width="1" />
									{/if}
									<circle cx={dp.x} cy={dp.y} r={hoverTxIdx === idx ? 4.5 : 3} fill="#ef4444" stroke="#ffffff" stroke-width="1" />
									<circle cx={cp.x} cy={cp.y} r={hoverTxIdx === idx ? 4.5 : 3} fill="#10b981" stroke="#ffffff" stroke-width="1" />
									<rect
										x={cp.x - colW / 2}
										y={PAD_T}
										width={colW}
										height={PLOT_H}
										fill="transparent"
										onmouseenter={() => (hoverTxIdx = idx)}
										onmouseleave={() => (hoverTxIdx = null)}
										role="presentation"
									/>
									{#if idx % step === 0}
										<text
											x={cp.x}
											y={base + 14}
											text-anchor="end"
											transform="rotate(-45 {cp.x} {base + 14})"
											class="fill-gray-500 text-[10px] dark:fill-gray-400"
										>{row.label}</text>
									{/if}
								{/each}
							</svg>
							{#if hoverTxIdx !== null && rows[hoverTxIdx]}
								{@const item = rows[hoverTxIdx]}
								<div class="pointer-events-none absolute right-4 top-2 rounded-lg border border-gray-200 bg-gray-900/95 px-3 py-1.5 text-xs text-white shadow-lg">
									<div class="font-semibold">{item.label}</div>
									<div class="text-emerald-300">Credits: {fmtInt(item.credits)}</div>
									<div class="text-rose-300">Debits: {fmtInt(item.debits)}</div>
								</div>
							{/if}
						</div>
						<div class="mt-2 flex flex-wrap items-center justify-center gap-5 text-xs text-gray-600 dark:text-gray-400">
							<span class="inline-flex items-center gap-2">
								<span class="inline-block h-3 w-3 rounded-[2px] border border-emerald-500 bg-emerald-500/15"></span>
								Credits
							</span>
							<span class="inline-flex items-center gap-2">
								<span class="inline-block h-3 w-3 rounded-[2px] border border-rose-500 bg-rose-500/15"></span>
								Debits
							</span>
						</div>
					{/if}
				</div>
			</section>
		</div>

		<!-- Widget 8: Top Customers by Period -->
		<section class={chartCard}>
			<header class="border-b border-gray-100 px-6 py-4 dark:border-gray-800">
				<h2 class="text-base font-semibold text-gray-950 dark:text-white">Top Customers by Period</h2>
			</header>

			<div class="flex justify-end px-4 py-3">
				<div class="relative w-64 max-w-full">
					<svg class="pointer-events-none absolute left-3 top-2.5 h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
					</svg>
					<input
						type="search"
						bind:value={customerSearch}
						oninput={onSearchInput}
						placeholder="Search"
						aria-label="Search top customers"
						class="w-full rounded-lg border border-gray-300 bg-white py-2 pl-9 pr-3 text-sm text-gray-900 outline-none focus:border-emerald-600 focus:ring-1 focus:ring-emerald-600 dark:border-gray-700 dark:bg-gray-800 dark:text-white"
					/>
				</div>
			</div>

			<div class="overflow-x-auto border-t border-gray-200 dark:border-gray-800">
				<table class="w-full text-left text-sm">
					<thead>
						<tr class="bg-gray-50/70 text-xs font-semibold uppercase tracking-wide text-gray-700 dark:bg-gray-800/40 dark:text-gray-200">
							<th class="whitespace-nowrap px-6 py-3.5">
								<button type="button" onclick={() => toggleSort('name')} class="inline-flex items-center gap-1 uppercase">
									Customer
									{@render sortArrow('name')}
								</button>
							</th>
							<th class="whitespace-nowrap px-6 py-3.5">Email</th>
							<th class="whitespace-nowrap px-6 py-3.5">Status</th>
							<th class="whitespace-nowrap px-6 py-3.5">
								<button type="button" onclick={() => toggleSort('balance')} class="inline-flex items-center gap-1 uppercase">
									Balance
									{@render sortArrow('balance')}
								</button>
							</th>
							<th class="whitespace-nowrap px-6 py-3.5">
								<button type="button" onclick={() => toggleSort('period_spent')} class="inline-flex items-center gap-1 uppercase">
									Period Spent
									{@render sortArrow('period_spent')}
								</button>
							</th>
							<th class="whitespace-nowrap px-6 py-3.5">
								<button type="button" onclick={() => toggleSort('messages_count')} class="inline-flex items-center gap-1 uppercase">
									Messages
									{@render sortArrow('messages_count')}
								</button>
							</th>
						</tr>
					</thead>
					<tbody class="divide-y divide-gray-100 dark:divide-gray-800">
						{#each data.top_customers as c (c.id)}
							<tr class="transition-colors hover:bg-gray-50/70 dark:hover:bg-gray-800/30">
								<td class="whitespace-nowrap px-6 py-4 font-medium text-gray-900 dark:text-white">
									<a href="/superman/users/customers/{c.id}" class="hover:text-emerald-700 hover:underline dark:hover:text-emerald-400">
										{c.name}
									</a>
								</td>
								<td class="whitespace-nowrap px-6 py-4 text-gray-700 dark:text-gray-300">{c.email}</td>
								<td class="whitespace-nowrap px-6 py-4">
									<span class="inline-flex items-center gap-1.5 rounded-md border px-2 py-0.5 text-xs font-medium {statusBadgeClass(c.status)}">
										{#if c.status.toLowerCase() === 'approved' || c.status.toLowerCase() === 'active'}
											<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
												<path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
											</svg>
										{:else if c.status.toLowerCase() === 'pending'}
											<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
												<path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
											</svg>
										{:else}
											<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
												<path stroke-linecap="round" stroke-linejoin="round" d="M18.364 18.364A9 9 0 0 0 5.636 5.636m12.728 12.728A9 9 0 0 1 5.636 5.636m12.728 12.728L5.636 5.636" />
											</svg>
										{/if}
										{formatStatus(c.status)}
									</span>
								</td>
								<td class="whitespace-nowrap px-6 py-4 tabular-nums text-gray-800 dark:text-gray-200">
									BDT {fmtMoney(c.balance)}
								</td>
								<td class="whitespace-nowrap px-6 py-4 tabular-nums font-medium text-rose-600 dark:text-rose-400">
									{#if c.period_spent !== null}
										BDT {fmtMoney(c.period_spent)}
									{/if}
								</td>
								<td class="whitespace-nowrap px-6 py-4 tabular-nums text-gray-800 dark:text-gray-200">
									{fmtInt(c.messages_count)}
								</td>
							</tr>
						{:else}
							<tr>
								<td colspan="6" class="px-6 py-12 text-center text-sm text-gray-400">
									No customers found for the selected filters.
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		</section>
	{/if}
</div>
