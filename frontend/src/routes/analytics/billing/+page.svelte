<script lang="ts">
	import { api } from '$lib/api.js';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { toast } from '$lib/stores/toast.svelte.js';

	interface Trend { date: string; amount?: number; cost?: number; }
	interface Method { method: string; amount: number; }
	interface Volume { date: string; credits: number; debits: number; }

	interface BillingData {
		period: { from: string; to: string };
		stats: {
			deposits: number;
			spent: number;
			pending_deposits: number;
			transactions: number;
		};
		deposit_trend: Trend[];
		spend_trend: Trend[];
		methods: Method[];
		volume: Volume[];
	}

	let data = $state<BillingData | null>(null);
	let loading = $state(true);
	let range = $state('last_30_days');
	let customFrom = $state('');
	let customTo = $state('');
	let topUpOpen = $state(false);
	let topUpAmount = $state('');
	let toppingUp = $state(false);

	async function handleTopUp() {
		const amt = parseFloat(topUpAmount);
		if (isNaN(amt) || amt < 50 || amt > 100000) {
			toast.error('Invalid Amount', 'Minimum deposit is BDT 50 and maximum is BDT 100,000.');
			return;
		}
		toppingUp = true;
		try {
			const res = await api.post<{ redirect_url?: string; success: boolean; data?: any }>('/api/v1/customer/deposits', {
				amount: amt,
				gateway: 'piprapay'
			});
			if (res.redirect_url) {
				window.location.href = res.redirect_url;
				return;
			}
			toast.success('Deposit Initiated', `Deposit of BDT ${amt.toFixed(2)} created.`);
			topUpOpen = false;
			load();
		} catch (err: any) {
			toast.error('Deposit Error', err?.message || 'Failed to initiate deposit.');
		} finally {
			toppingUp = false;
		}
	}

	const rangeOptions = [
		{ value: 'today', label: 'Today' },
		{ value: 'yesterday', label: 'Yesterday' },
		{ value: 'this_week', label: 'This Week' },
		{ value: 'last_7_days', label: 'Last 7 Days' },
		{ value: 'this_month', label: 'This Month' },
		{ value: 'last_30_days', label: 'Last 30 Days' },
		{ value: 'this_year', label: 'This Year' },
		{ value: 'custom', label: 'Custom Range' }
	];

	function methodLabel(method: string) {
		if (method === 'manual') return 'Manual';
		if (method === 'piprapay') return 'PipraPay';
		return method;
	}

	async function load() {
		loading = true;
		try {
			const params = new URLSearchParams({ range });
			if (range === 'custom') {
				params.set('from', customFrom);
				params.set('to', customTo);
			}
			const res = await api.get<{ data: BillingData }>(`/api/v1/customer/analytics/billing?${params}`);
			data = res.data;
		} catch {
			data = null;
		} finally {
			loading = false;
		}
	}

	onMount(() => load());

	// ── SVG Chart Helpers ──────────────────────────────────────────────────────

	const W = 480;
	const H = 160;
	const PAD = { top: 10, right: 10, bottom: 28, left: 36 };

	function scaleX(i: number, n: number): number {
		if (n <= 1) return PAD.left + (W - PAD.left - PAD.right) / 2;
		return PAD.left + (i / (n - 1)) * (W - PAD.left - PAD.right);
	}
	function scaleY(v: number, max: number): number {
		if (max === 0) return PAD.top + (H - PAD.top - PAD.bottom);
		return PAD.top + (1 - v / max) * (H - PAD.top - PAD.bottom);
	}

	function barWidth(n: number): number {
		if (n === 0) return 20;
		const availW = W - PAD.left - PAD.right;
		return Math.max(4, Math.min(40, (availW / n) * 0.6));
	}

	function yTicks(max: number): number[] {
		if (max === 0) return [0];
		const step = Math.pow(10, Math.floor(Math.log10(max)));
		const niceMax = Math.ceil(max / step) * step;
		return [0, niceMax * 0.25, niceMax * 0.5, niceMax * 0.75, niceMax].map((v) =>
			parseFloat(v.toFixed(2))
		);
	}

	// Deposit Trend (green bars)
	let depositMax = $derived(
		data?.deposit_trend?.length
			? Math.max(...data.deposit_trend.map((d) => d.amount ?? 0), 1)
			: 1
	);

	// Spend Trend (blue bars)
	let spendMax = $derived(
		data?.spend_trend?.length
			? Math.max(...data.spend_trend.map((d) => d.cost ?? 0), 1)
			: 1
	);

	// Doughnut chart
	let methodTotal = $derived(
		(data?.methods ?? []).reduce((s, m) => s + m.amount, 0)
	);

	const DOUGHNUT_COLORS = ['#10b981', '#3b82f6', '#f59e0b', '#ef4444', '#8b5cf6'];

	function doughnutSlices(methods: Method[], total: number): { d: string; color: string; label: string; amount: number }[] {
		if (!methods.length || total === 0) return [];
		let angle = -Math.PI / 2;
		const cx = 90, cy = 90, r = 70, inner = 42;
		return methods.map((m, i) => {
			const frac = m.amount / total;
			const sweep = frac * 2 * Math.PI;
			const x1 = cx + r * Math.cos(angle);
			const y1 = cy + r * Math.sin(angle);
			angle += sweep;
			const x2 = cx + r * Math.cos(angle);
			const y2 = cy + r * Math.sin(angle);
			const ix1 = cx + inner * Math.cos(angle - sweep);
			const iy1 = cy + inner * Math.sin(angle - sweep);
			const ix2 = cx + inner * Math.cos(angle);
			const iy2 = cy + inner * Math.sin(angle);
			const large = sweep > Math.PI ? 1 : 0;
			const d = `M ${x1} ${y1} A ${r} ${r} 0 ${large} 1 ${x2} ${y2} L ${ix2} ${iy2} A ${inner} ${inner} 0 ${large} 0 ${ix1} ${iy1} Z`;
			return { d, color: DOUGHNUT_COLORS[i % DOUGHNUT_COLORS.length], label: methodLabel(m.method), amount: m.amount };
		});
	}

	// Volume (area line chart)
	let volMaxCredits = $derived(
		data?.volume?.length ? Math.max(...data.volume.map((v) => v.credits), 1) : 1
	);
	let volMaxDebits = $derived(
		data?.volume?.length ? Math.max(...data.volume.map((v) => v.debits), 1) : 1
	);
	let volMax = $derived(Math.max(volMaxCredits, volMaxDebits, 1));

	function polyline(pts: { x: number; y: number }[]): string {
		return pts.map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x} ${p.y}`).join(' ');
	}
	function areaPath(pts: { x: number; y: number }[], bottom: number): string {
		if (!pts.length) return '';
		const line = polyline(pts);
		return `${line} L ${pts[pts.length - 1].x} ${bottom} L ${pts[0].x} ${bottom} Z`;
	}
</script>

<svelte:head>
	<title>Billing Analytics - White INFO</title>
</svelte:head>

<div class="space-y-6">
	<!-- Page Header -->
	<div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
		<h1 class="text-2xl font-bold text-gray-900">Billing Analytics</h1>
		<button
			type="button"
			onclick={() => { topUpAmount = ''; topUpOpen = true; }}
			class="inline-flex items-center gap-2 rounded-lg border border-teal-600 bg-white px-4 py-2 text-sm font-semibold text-teal-700 shadow-sm transition hover:bg-teal-50"
		>
			<!-- taka-in-circle icon -->
			<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
				<circle cx="12" cy="12" r="9" />
				<path stroke-linecap="round" stroke-linejoin="round" d="M9 8h4m-4 0a2 2 0 100 4h4m-4 0a2 2 0 000 4h1m3-8v8" />
			</svg>
			Top Up
		</button>
	</div>

	<!-- Date Range -->
	<div class="flex items-center gap-3">
		<label for="billing-range" class="text-sm font-medium text-gray-700">Date Range</label>
		<div class="relative">
			<select
				id="billing-range"
				bind:value={range}
				onchange={() => load()}
				class="appearance-none rounded-lg border border-gray-300 bg-white py-2 pl-3 pr-8 text-sm text-gray-800 focus:border-teal-500 focus:outline-none focus:ring-1 focus:ring-teal-500"
			>
				{#each rangeOptions as opt (opt.value)}
					<option value={opt.value}>{opt.label}</option>
				{/each}
			</select>
			<svg class="pointer-events-none absolute right-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
			</svg>
		</div>
		{#if range === 'custom'}
			<input type="date" bind:value={customFrom} onchange={load} class="rounded-lg border border-gray-300 bg-white px-2 py-1.5 text-xs" />
			<span class="text-xs text-gray-400">to</span>
			<input type="date" bind:value={customTo} onchange={load} class="rounded-lg border border-gray-300 bg-white px-2 py-1.5 text-xs" />
		{/if}
	</div>

	<!-- 4 Stat Cards -->
	<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
		<!-- Deposits -->
		<div class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm">
			<div class="flex items-center gap-2 text-sm text-gray-600">
				<svg class="h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="M2.25 18.75a60.07 60.07 0 0115.797 2.101c.727.198 1.453-.342 1.453-1.096V18.75M3.75 4.5v.75A.75.75 0 013 6h-.75m0 0v-.375c0-.621.504-1.125 1.125-1.125H20.25M2.25 6v9m18-10.5v.75c0 .414.336.75.75.75h.75m-1.5-1.5h.375c.621 0 1.125.504 1.125 1.125v9.75c0 .621-.504 1.125-1.125 1.125h-.375m1.5-1.5H21a.75.75 0 00-.75.75v.75m0 0H3.75m0 0h-.375a1.125 1.125 0 01-1.125-1.125V15m1.5 1.5v-.75A.75.75 0 003 15h-.75M15 10.5a3 3 0 11-6 0 3 3 0 016 0zm3 0h.008v.008H18V10.5zm-12 0h.008v.008H6V10.5z" />
				</svg>
				Deposits
			</div>
			<div class="mt-2 text-3xl font-bold text-gray-900">
				{loading ? '—' : (data?.stats.deposits ?? 0).toFixed(2)} BDT
			</div>
			<p class="mt-1 text-xs font-medium text-green-600">Completed in period</p>
		</div>

		<!-- Spent -->
		<div class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm">
			<div class="flex items-center gap-2 text-sm text-gray-600">
				<svg class="h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="M2.25 6L9 12.75l4.286-4.286a11.948 11.948 0 014.306 6.43l.776 2.898m0 0l3.182-5.511m-3.182 5.51l-5.511-3.181" />
				</svg>
				Spent
			</div>
			<div class="mt-2 text-3xl font-bold text-gray-900">
				{loading ? '—' : (data?.stats.spent ?? 0).toFixed(2)} BDT
			</div>
			<p class="mt-1 text-xs font-medium text-red-500">Message cost in period</p>
		</div>

		<!-- Pending Deposits -->
		<div class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm">
			<div class="flex items-center gap-2 text-sm text-gray-600">
				<svg class="h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 11-18 0 9 9 0 0118 0z" />
				</svg>
				Pending Deposits
			</div>
			<div class="mt-2 text-3xl font-bold text-gray-900">
				{loading ? '—' : (data?.stats.pending_deposits ?? 0).toFixed(2)} BDT
			</div>
			<p class="mt-1 text-xs font-medium text-orange-500">Awaiting approval</p>
		</div>

		<!-- Transactions -->
		<div class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm">
			<div class="flex items-center gap-2 text-sm text-gray-600">
				<svg class="h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="M7.5 21L3 16.5m0 0L7.5 12M3 16.5h13.5m0-13.5L21 7.5m0 0L16.5 12M21 7.5H7.5" />
				</svg>
				Transactions
			</div>
			<div class="mt-2 text-3xl font-bold text-gray-900">
				{loading ? '—' : (data?.stats.transactions ?? 0)}
			</div>
			<p class="mt-1 text-xs text-gray-500">In selected period</p>
		</div>
	</div>

	<!-- 2×2 Chart Grid -->
	<div class="grid grid-cols-1 gap-6 lg:grid-cols-2">

		<!-- Deposit Trend (green bar chart) -->
		<div class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm">
			<h2 class="text-sm font-bold text-gray-900">Deposit Trend</h2>
			<p class="text-xs text-gray-500 mt-0.5 mb-4">Completed deposits per day in the selected period</p>
			{#if !loading && data?.deposit_trend?.length}
				{@const trend = data.deposit_trend}
				{@const n = trend.length}
				{@const bw = barWidth(n)}
				{@const ticks = yTicks(depositMax)}
				<svg viewBox="0 0 {W} {H}" class="w-full h-auto">
					<!-- Y-axis grid lines + labels -->
					{#each ticks as tick}
						{@const y = scaleY(tick, depositMax)}
						<line x1={PAD.left} y1={y} x2={W - PAD.right} y2={y} stroke="#f0fdf4" stroke-width="1" />
						<text x={PAD.left - 4} y={y + 3} font-size="8" fill="#6b7280" text-anchor="end">{tick}</text>
					{/each}
					<!-- Bars -->
					{#each trend as d, i}
						{@const x = scaleX(i, n)}
						{@const barH = scaleY(0, depositMax) - scaleY(d.amount ?? 0, depositMax)}
						<rect
							x={x - bw / 2}
							y={scaleY(d.amount ?? 0, depositMax)}
							width={bw}
							height={Math.max(barH, 0)}
							fill="#10b981"
							rx="2"
						/>
						{#if n <= 15}
							<text x={x} y={H - PAD.bottom + 10} font-size="8" fill="#9ca3af" text-anchor="middle">{d.date}</text>
						{/if}
					{/each}
					<!-- X-axis line -->
					<line x1={PAD.left} y1={H - PAD.bottom} x2={W - PAD.right} y2={H - PAD.bottom} stroke="#e5e7eb" stroke-width="1" />
				</svg>
				<!-- Legend -->
				<div class="mt-2 flex items-center gap-2 text-xs text-gray-500">
					<span class="inline-block h-2.5 w-2.5 rounded-sm bg-green-500"></span>
					Deposits (BDT)
				</div>
			{:else if !loading}
				<div class="flex h-32 items-center justify-center text-xs text-gray-400">No data for selected period</div>
			{:else}
				<div class="flex h-32 items-center justify-center">
					<svg class="h-5 w-5 animate-spin text-teal-500" fill="none" viewBox="0 0 24 24">
						<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
						<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
					</svg>
				</div>
			{/if}
		</div>

		<!-- Spending Trends (blue bar chart) -->
		<div class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm">
			<h2 class="text-sm font-bold text-gray-900">Spending Trends</h2>
			<p class="text-xs text-gray-500 mt-0.5 mb-4">Cost per day in the selected period</p>
			{#if !loading && data?.spend_trend?.length}
				{@const trend = data.spend_trend}
				{@const n = trend.length}
				{@const bw = barWidth(n)}
				{@const ticks = yTicks(spendMax)}
				<svg viewBox="0 0 {W} {H}" class="w-full h-auto">
					{#each ticks as tick}
						{@const y = scaleY(tick, spendMax)}
						<line x1={PAD.left} y1={y} x2={W - PAD.right} y2={y} stroke="#eff6ff" stroke-width="1" />
						<text x={PAD.left - 4} y={y + 3} font-size="8" fill="#6b7280" text-anchor="end">{tick}</text>
					{/each}
					{#each trend as d, i}
						{@const x = scaleX(i, n)}
						{@const barH = scaleY(0, spendMax) - scaleY(d.cost ?? 0, spendMax)}
						<rect
							x={x - bw / 2}
							y={scaleY(d.cost ?? 0, spendMax)}
							width={bw}
							height={Math.max(barH, 0)}
							fill="#3b82f6"
							rx="2"
						/>
						{#if n <= 15}
							<text x={x} y={H - PAD.bottom + 10} font-size="8" fill="#9ca3af" text-anchor="middle">{d.date}</text>
						{/if}
					{/each}
					<line x1={PAD.left} y1={H - PAD.bottom} x2={W - PAD.right} y2={H - PAD.bottom} stroke="#e5e7eb" stroke-width="1" />
				</svg>
				<div class="mt-2 flex items-center gap-2 text-xs text-gray-500">
					<span class="inline-block h-2.5 w-2.5 rounded-sm bg-blue-500"></span>
					Cost (BDT)
				</div>
			{:else if !loading}
				<div class="flex h-32 items-center justify-center text-xs text-gray-400">No data for selected period</div>
			{:else}
				<div class="flex h-32 items-center justify-center">
					<svg class="h-5 w-5 animate-spin text-teal-500" fill="none" viewBox="0 0 24 24">
						<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
						<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
					</svg>
				</div>
			{/if}
		</div>

		<!-- Deposits by Method (doughnut) -->
		<div class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm">
			<h2 class="text-sm font-bold text-gray-900">Deposits by Method</h2>
			<p class="text-xs text-gray-500 mt-0.5 mb-4">Completed deposits by payment method in the selected period</p>
			{#if !loading && data?.methods?.length && methodTotal > 0}
				{@const slices = doughnutSlices(data.methods, methodTotal)}
				<div class="flex items-center gap-6">
					<svg viewBox="0 0 180 180" class="w-44 h-44 flex-shrink-0">
						{#each slices as s}
							<path d={s.d} fill={s.color} />
						{/each}
						<!-- center hole label -->
						<text x="90" y="90" text-anchor="middle" dominant-baseline="middle" font-size="10" fill="#374151" font-weight="600">
							{methodTotal.toFixed(0)} BDT
						</text>
					</svg>
					<div class="space-y-2">
						{#each slices as s}
							<div class="flex items-center gap-2 text-xs">
								<span class="inline-block h-2.5 w-2.5 rounded-sm flex-shrink-0" style="background:{s.color}"></span>
								<span class="text-gray-700">{s.label}</span>
							</div>
						{/each}
					</div>
				</div>
			{:else if !loading}
				<div class="flex h-44 items-center justify-center text-xs text-gray-400">No deposit data for selected period</div>
			{:else}
				<div class="flex h-44 items-center justify-center">
					<svg class="h-5 w-5 animate-spin text-teal-500" fill="none" viewBox="0 0 24 24">
						<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
						<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
					</svg>
				</div>
			{/if}
		</div>

		<!-- Transaction Volume (area line chart: green credits + red debits) -->
		<div class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm">
			<h2 class="text-sm font-bold text-gray-900">Transaction Volume</h2>
			<p class="text-xs text-gray-500 mt-0.5 mb-4">Credits vs debits per day in the selected period</p>
			{#if !loading && data?.volume?.length}
				{@const vol = data.volume}
				{@const n = vol.length}
				{@const ticks = yTicks(volMax)}
				{@const bottom = H - PAD.bottom}
				{@const creditPts = vol.map((v, i) => ({ x: scaleX(i, n), y: scaleY(v.credits, volMax) }))}
				{@const debitPts = vol.map((v, i) => ({ x: scaleX(i, n), y: scaleY(v.debits, volMax) }))}
				<svg viewBox="0 0 {W} {H}" class="w-full h-auto">
					{#each ticks as tick}
						{@const y = scaleY(tick, volMax)}
						<line x1={PAD.left} y1={y} x2={W - PAD.right} y2={y} stroke="#f0fdf4" stroke-width="1" />
						<text x={PAD.left - 4} y={y + 3} font-size="8" fill="#6b7280" text-anchor="end">{tick}</text>
					{/each}
					<!-- Credit area (green) -->
					<path d={areaPath(creditPts, bottom)} fill="#10b981" fill-opacity="0.15" />
					<path d={polyline(creditPts)} fill="none" stroke="#10b981" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
					<!-- Debit area (red) -->
					<path d={areaPath(debitPts, bottom)} fill="#ef4444" fill-opacity="0.15" />
					<path d={polyline(debitPts)} fill="none" stroke="#ef4444" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
					<line x1={PAD.left} y1={bottom} x2={W - PAD.right} y2={bottom} stroke="#e5e7eb" stroke-width="1" />
					{#each vol as v, i}
						{#if n <= 15}
							<text x={scaleX(i, n)} y={H - PAD.bottom + 10} font-size="8" fill="#9ca3af" text-anchor="middle">{v.date}</text>
						{/if}
					{/each}
				</svg>
				<div class="mt-2 flex items-center gap-4 text-xs text-gray-500">
					<span class="flex items-center gap-1.5"><span class="inline-block h-2.5 w-2.5 rounded-sm bg-green-500"></span>Credits (BDT)</span>
					<span class="flex items-center gap-1.5"><span class="inline-block h-2.5 w-2.5 rounded-sm bg-red-500"></span>Debits (BDT)</span>
				</div>
			{:else if !loading}
				<div class="flex h-32 items-center justify-center text-xs text-gray-400">No transaction data for selected period</div>
			{:else}
				<div class="flex h-32 items-center justify-center">
					<svg class="h-5 w-5 animate-spin text-teal-500" fill="none" viewBox="0 0 24 24">
						<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
						<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
					</svg>
				</div>
			{/if}
		</div>
	</div>
</div>

{#if topUpOpen}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-xs">
		<div class="relative w-full max-w-md rounded-2xl bg-white p-6 shadow-xl border border-gray-100">
			<div class="flex items-start gap-4">
				<div class="flex h-12 w-12 shrink-0 items-center justify-center rounded-full bg-gray-100 text-gray-800 text-xl font-bold">
					৳
				</div>
				<div class="min-w-0 flex-1">
					<h3 class="text-lg font-bold text-gray-900">Add Balance</h3>
					<p class="mt-1 text-xs text-gray-500 leading-relaxed">
						Minimum deposit amount is BDT 50 and maximum deposit amount is BDT 100,000.
					</p>
				</div>
			</div>
			<div class="mt-5">
				<label for="billing-topup-amount" class="block text-xs font-semibold text-gray-700 mb-1.5">Amount (BDT)</label>
				<input
					id="billing-topup-amount"
					type="number"
					min="50"
					max="100000"
					step="1"
					bind:value={topUpAmount}
					placeholder="e.g. 500"
					class="w-full rounded-lg border border-gray-300 bg-white py-2 px-3 text-sm text-gray-900 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600"
				/>
			</div>
			<div class="mt-6 flex justify-end gap-2">
				<button
					type="button"
					onclick={() => (topUpOpen = false)}
					class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-xs font-semibold text-gray-700 hover:bg-gray-50 cursor-pointer"
				>
					Cancel
				</button>
				<button
					type="button"
					disabled={toppingUp}
					onclick={handleTopUp}
					class="rounded-lg bg-teal-600 px-4 py-2 text-xs font-semibold text-white hover:bg-teal-700 disabled:opacity-50 cursor-pointer"
				>
					{toppingUp ? 'Redirecting...' : 'Proceed to Pay'}
				</button>
			</div>
		</div>
	</div>
{/if}
