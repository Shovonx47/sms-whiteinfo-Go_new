<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte.js';
	import { api } from '$lib/api.js';
	import { toast } from '$lib/components/Toast.svelte';
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';

	type PeriodKey = 'today' | '7_days' | '30_days' | 'all_time';

	interface PeriodSummary {
		total_sent: number;
		successful: number;
		delivered: number;
		sent_telco: number;
		failed: number;
		cost: number;
		success_rate: number;
		delivery_rate: number;
		failed_rate: number;
	}

	interface RecentMessage {
		id: number;
		msg_id: string;
		sender_id: string;
		to: string;
		body: string;
		status: string;
		cost: number;
		parts: number;
		created_at: string;
		sent_at: string | null;
		delivered_at: string | null;
	}

	interface ScheduledBatchItem {
		id: number;
		sender_db_id?: number | null;
		sender_id: string;
		recipient_preview: string;
		recipients_list?: string;
		total_recipients: number;
		body: string;
		source_type: string;
		status: string;
		scheduled_at: string | null;
		created_at: string;
	}

	interface DashboardStats {
		balance: number;
		messages_today: number;
		successful_today: number;
		delivered_today: number;
		sent_telco_today: number;
		failed_today: number;
		cost_today: number;
		success_rate_today: number;
		delivery_rate: number;
		messages_all_time: number;
		successful_all_time: number;
		delivered_all_time: number;
		sent_telco_all_time: number;
		failed_all_time: number;
		cost_all_time: number;
		scheduled_count: number;
		contacts_total: number;
		groups_total: number;
		templates_total: number;
		periods: Record<PeriodKey, PeriodSummary>;
		recent_messages: RecentMessage[];
		scheduled_messages: ScheduledBatchItem[];
	}

	const emptyPeriod: PeriodSummary = {
		total_sent: 0,
		successful: 0,
		delivered: 0,
		sent_telco: 0,
		failed: 0,
		cost: 0,
		success_rate: 0,
		delivery_rate: 0,
		failed_rate: 0
	};

	let stats = $state<DashboardStats | null>(null);
	let loadingStats = $state(true);
	let selectedPeriod = $state<PeriodKey>('today');

	// Column visibility toggles & sorting for the two tables
	let showRecentColMenu = $state(false);
	let recentCols = $state({
		recipient: true,
		sender_id: true,
		body: true,
		status: true,
		cost: true,
		sent_at: false
	});

	let showScheduledColMenu = $state(false);
	let scheduledCols = $state({
		sender_id: true,
		recipient: true,
		recipients: true,
		body: true,
		scheduled_for: true,
		status: true
	});
	let scheduledSortDir = $state<'asc' | 'desc'>('asc');

	let pollTimer: ReturnType<typeof setInterval> | null = null;

	async function loadStats(silent = false) {
		if (!silent) loadingStats = true;
		try {
			const res = await api.get<{ data: DashboardStats }>('/api/v1/customer/dashboard/stats');
			stats = res.data;
		} catch {
			if (!stats) {
				stats = {
					balance: Number(auth.balance ?? 0),
					messages_today: 0,
					successful_today: 0,
					delivered_today: 0,
					sent_telco_today: 0,
					failed_today: 0,
					cost_today: 0,
					success_rate_today: 100,
					delivery_rate: 0,
					messages_all_time: 0,
					successful_all_time: 0,
					delivered_all_time: 0,
					sent_telco_all_time: 0,
					failed_all_time: 0,
					cost_all_time: 0,
					scheduled_count: 0,
					contacts_total: 0,
					groups_total: 0,
					templates_total: 0,
					periods: {
						today: { ...emptyPeriod },
						'7_days': { ...emptyPeriod },
						'30_days': { ...emptyPeriod },
						all_time: { ...emptyPeriod }
					},
					recent_messages: [],
					scheduled_messages: []
				};
			}
		} finally {
			loadingStats = false;
		}
	}

	onMount(() => {
		loadStats();
		pollTimer = setInterval(() => loadStats(true), 15000);
	});

	onDestroy(() => {
		if (pollTimer) clearInterval(pollTimer);
	});

	function formatMoney(val: number | string): string {
		const num = Number(val);
		if (Number.isNaN(num)) return '0.00';
		return new Intl.NumberFormat('en-US', {
			minimumFractionDigits: 2,
			maximumFractionDigits: 2
		}).format(num);
	}

	function formatInt(val: number | string | undefined | null): string {
		const num = Number(val ?? 0);
		if (Number.isNaN(num)) return '0';
		return new Intl.NumberFormat('en-US').format(num);
	}

	function formatDateTime(iso: string | null): string {
		if (!iso) return '—';
		return new Date(iso).toLocaleString('en-GB', {
			day: '2-digit',
			month: 'short',
			year: 'numeric',
			hour: '2-digit',
			minute: '2-digit',
			hour12: true
		});
	}

	function formatRelativeTime(iso: string | null): string {
		if (!iso) return '';
		const target = new Date(iso).getTime();
		const diffSec = Math.round((target - Date.now()) / 1000);
		if (Math.abs(diffSec) < 60) return diffSec >= 0 ? 'in a few seconds' : 'just now';
		const absMin = Math.round(Math.abs(diffSec) / 60);
		if (absMin < 60) return diffSec >= 0 ? `in ${absMin} min` : `${absMin} min ago`;
		const absHr = Math.round(absMin / 60);
		if (absHr < 24) return diffSec >= 0 ? `in ${absHr} hr` : `${absHr} hr ago`;
		const absDays = Math.round(absHr / 24);
		return diffSec >= 0 ? `in ${absDays} days` : `${absDays} days ago`;
	}

	async function copyText(text: string, label = 'Recipient') {
		if (!text || text === '—') return;
		try {
			await navigator.clipboard.writeText(text);
			toast.success('Copied', `${label} copied to clipboard.`);
		} catch {
			// Ignore clipboard failures
		}
	}

	const currentBalance = $derived(stats ? stats.balance : Number(auth.balance ?? 0));

	const overviewCards = $derived([
		{
			title: 'CURRENT BALANCE',
			value: `BDT ${formatMoney(currentBalance)}`,
			description: 'Available for campaigns',
			icon: '/images/icons/wallet.svg',
			href: '/customer/deposits',
			bgGradient: 'from-emerald-500/10 to-teal-500/5',
			borderColor: 'hover:border-emerald-500/40',
			iconGlow: 'bg-emerald-500/10 ring-emerald-500/20',
			badgeColor: 'text-emerald-700 dark:text-emerald-400 bg-emerald-50 dark:bg-emerald-950/60'
		},
		{
			title: 'MESSAGES TODAY',
			value: formatInt(stats?.messages_today ?? 0),
			description: `${formatInt(stats?.messages_all_time ?? 0)} total messages sent`,
			icon: '/images/icons/messages-today.svg',
			href: '/customer/messages',
			bgGradient: 'from-blue-500/10 to-sky-500/5',
			borderColor: 'hover:border-blue-500/40',
			iconGlow: 'bg-blue-500/10 ring-blue-500/20',
			badgeColor: 'text-blue-700 dark:text-blue-400 bg-blue-50 dark:bg-blue-950/60'
		},
		{
			title: 'SUCCESS RATE',
			value: `${stats?.success_rate_today ?? 100}%`,
			description: `${formatInt(stats?.successful_today ?? 0)} sent successfully`,
			icon: '/images/icons/success-rate.svg',
			href: '/customer/messages',
			bgGradient: 'from-green-500/10 to-emerald-500/5',
			borderColor: 'hover:border-green-500/40',
			iconGlow: 'bg-green-500/10 ring-green-500/20',
			badgeColor: 'text-green-700 dark:text-green-400 bg-green-50 dark:bg-green-950/60'
		},
		{
			title: 'FAILED TODAY',
			value: formatInt(stats?.failed_today ?? 0),
			description: `${formatInt(stats?.failed_all_time ?? 0)} total failed all-time`,
			icon: '/images/icons/failed-today.svg',
			href: '/customer/messages?status=failed,dlr_failed',
			bgGradient:
				(stats?.failed_today ?? 0) > 0
					? 'from-rose-500/10 to-red-500/5'
					: 'from-gray-500/5 to-transparent',
			borderColor:
				(stats?.failed_today ?? 0) > 0
					? 'hover:border-rose-500/40'
					: 'hover:border-gray-400/30',
			iconGlow:
				(stats?.failed_today ?? 0) > 0
					? 'bg-rose-500/10 ring-rose-500/20'
					: 'bg-rose-500/10 ring-rose-500/20',
			badgeColor:
				(stats?.failed_today ?? 0) > 0
					? 'text-rose-700 dark:text-rose-400 bg-rose-50 dark:bg-rose-950/60'
					: 'text-gray-600 dark:text-gray-400 bg-gray-100 dark:bg-gray-800'
		},
		{
			title: 'COST TODAY',
			value: `${formatMoney(stats?.cost_today ?? 0)} BDT`,
			description: "Today's spend",
			icon: '/images/icons/cost-today.svg',
			href: '/customer/deposits',
			bgGradient: 'from-amber-500/10 to-yellow-500/5',
			borderColor: 'hover:border-amber-500/40',
			iconGlow: 'bg-amber-500/10 ring-amber-500/20',
			badgeColor: 'text-amber-700 dark:text-amber-400 bg-amber-50 dark:bg-amber-950/60'
		},
		{
			title: 'SCHEDULED SMS',
			value: formatInt(stats?.scheduled_count ?? 0),
			description:
				(stats?.scheduled_count ?? 0) > 0
					? `${formatInt(stats?.scheduled_count ?? 0)} upcoming in queue`
					: 'No upcoming messages',
			icon: '/images/icons/scheduled.svg',
			href: '/customer/scheduled-messages',
			bgGradient: 'from-violet-500/10 to-purple-500/5',
			borderColor: 'hover:border-violet-500/40',
			iconGlow: 'bg-violet-500/10 ring-violet-500/20',
			badgeColor: 'text-violet-700 dark:text-violet-400 bg-violet-50 dark:bg-violet-950/60'
		},
		{
			title: 'TOTAL CONTACTS',
			value: formatInt(stats?.contacts_total ?? 0),
			description: 'In your lists',
			icon: '/images/icons/contacts.svg',
			href: '/customer/contacts',
			bgGradient: 'from-indigo-500/10 to-violet-500/5',
			borderColor: 'hover:border-indigo-500/40',
			iconGlow: 'bg-indigo-500/10 ring-indigo-500/20',
			badgeColor: 'text-indigo-700 dark:text-indigo-400 bg-indigo-50 dark:bg-indigo-950/60'
		},
		{
			title: 'GROUPS',
			value: formatInt(stats?.groups_total ?? 0),
			description: 'Contact groups',
			icon: '/images/icons/contacts.svg',
			href: '/customer/groups',
			bgGradient: 'from-violet-500/10 to-purple-500/5',
			borderColor: 'hover:border-violet-500/40',
			iconGlow: 'bg-violet-500/10 ring-violet-500/20',
			badgeColor: 'text-violet-700 dark:text-violet-400 bg-violet-50 dark:bg-violet-950/60'
		},
		{
			title: 'TEMPLATES',
			value: formatInt(stats?.templates_total ?? 0),
			description: 'Message templates',
			icon: '/images/icons/templates.svg',
			href: '/customer/templates',
			bgGradient: 'from-cyan-500/10 to-teal-500/5',
			borderColor: 'hover:border-cyan-500/40',
			iconGlow: 'bg-cyan-500/10 ring-cyan-500/20',
			badgeColor: 'text-cyan-700 dark:text-cyan-400 bg-cyan-50 dark:bg-cyan-950/60'
		}
	]);

	const periodTabs: { key: PeriodKey; label: string }[] = [
		{ key: 'today', label: 'Today' },
		{ key: '7_days', label: '7 Days' },
		{ key: '30_days', label: '30 Days' },
		{ key: 'all_time', label: 'All Time' }
	];

	const activePeriodStats = $derived<PeriodSummary>(
		stats?.periods?.[selectedPeriod] ?? emptyPeriod
	);

	const deliveredBarWidth = $derived(
		activePeriodStats.total_sent > 0
			? Math.max(0, Math.min(100, (activePeriodStats.delivered / activePeriodStats.total_sent) * 100))
			: 0
	);
	const sentTelcoBarWidth = $derived(
		activePeriodStats.total_sent > 0
			? Math.max(0, Math.min(100, (activePeriodStats.sent_telco / activePeriodStats.total_sent) * 100))
			: 0
	);
	const failedBarWidth = $derived(
		activePeriodStats.total_sent > 0
			? Math.max(0, Math.min(100, (activePeriodStats.failed / activePeriodStats.total_sent) * 100))
			: 0
	);

	const recentMessages = $derived<RecentMessage[]>(stats?.recent_messages ?? []);

	const sortedScheduledMessages = $derived.by(() => {
		const list = [...(stats?.scheduled_messages ?? [])];
		list.sort((a, b) => {
			const ta = a.scheduled_at ? new Date(a.scheduled_at).getTime() : 0;
			const tb = b.scheduled_at ? new Date(b.scheduled_at).getTime() : 0;
			return scheduledSortDir === 'asc' ? ta - tb : tb - ta;
		});
		return list;
	});

	function toggleScheduledSort() {
		scheduledSortDir = scheduledSortDir === 'asc' ? 'desc' : 'asc';
	}
</script>

<svelte:head>
	<title>Dashboard - White INFO</title>
	<meta name="description" content="Your SMS Whiteinfo customer dashboard." />
</svelte:head>

<div class="space-y-6">
	<!-- Page Header: Dashboard + Top Up -->
	<div class="flex items-center justify-between gap-4">
		<h1 class="text-2xl sm:text-3xl font-extrabold tracking-tight text-gray-950 dark:text-white">
			Dashboard
		</h1>
		<a
			href="/customer/deposits?deposit=1"
			id="btn-top-up"
			class="inline-flex items-center gap-2 rounded-lg border border-teal-600/80 bg-white px-3.5 py-2 text-sm font-semibold text-teal-700 shadow-2xs transition hover:bg-teal-50 dark:border-teal-500/60 dark:bg-gray-900 dark:text-teal-400 dark:hover:bg-teal-950/40"
		>
			<span class="inline-flex h-5 w-5 items-center justify-center rounded-full border border-teal-600 text-[11px] font-bold text-teal-700 dark:border-teal-400 dark:text-teal-400">
				৳
			</span>
			Top Up
		</a>
	</div>

	<!-- 9 Overview Cards Grid (3x3) -->
	<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4 sm:gap-5">
		{#each overviewCards as card (card.title)}
			<a
				href={card.href}
				class="group relative flex flex-col justify-between overflow-hidden rounded-2xl border border-gray-200/80 bg-white p-5 shadow-xs transition-all duration-200 hover:-translate-y-0.5 hover:shadow-md dark:border-gray-800 dark:bg-gray-900 {card.borderColor}"
			>
				<!-- Subtle ambient gradient background on hover -->
				<div
					class="pointer-events-none absolute inset-0 bg-gradient-to-br {card.bgGradient} opacity-60 transition-opacity duration-200 group-hover:opacity-100"
				></div>

				<!-- Card Header: Title + Value + Designed SVG Icon -->
				<div class="relative z-10 flex items-start justify-between gap-3">
					<div class="min-w-0 flex-1">
						<span class="text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
							{card.title}
						</span>
						{#if loadingStats && !stats}
							<div class="mt-2 h-8 w-32 animate-pulse rounded-lg bg-gray-100 dark:bg-gray-800"></div>
						{:else}
							<div class="mt-1 truncate text-2xl sm:text-[26px] font-extrabold tracking-tight text-gray-950 dark:text-white">
								{card.value}
							</div>
						{/if}
					</div>

					<!-- Multi-Color Designed Image Icon Container -->
					<div class="relative shrink-0">
						<div
							class="flex h-12 w-12 sm:h-14 sm:w-14 items-center justify-center rounded-2xl ring-1 {card.iconGlow} shadow-xs transition-transform duration-200 group-hover:scale-105"
						>
							<img
								src={card.icon}
								alt={card.title}
								class="h-9 w-9 sm:h-10 sm:w-10 object-contain drop-shadow-xs select-none"
								loading="lazy"
							/>
						</div>
					</div>
				</div>

				<!-- Card Footer: Descriptive Badge -->
				<div class="relative z-10 mt-3 flex items-center justify-between border-t border-gray-100 pt-2.5 dark:border-gray-800/80">
					{#if loadingStats && !stats}
						<div class="h-5 w-40 animate-pulse rounded bg-gray-100 dark:bg-gray-800"></div>
					{:else}
						<span
							class="inline-flex items-center gap-1.5 rounded-md px-2 py-0.5 text-xs font-medium {card.badgeColor}"
						>
							<span class="h-1.5 w-1.5 rounded-full bg-current opacity-75"></span>
							<span class="max-w-[240px] truncate">{card.description}</span>
						</span>
					{/if}

					<svg
						class="h-4 w-4 -translate-x-1 text-gray-400 opacity-0 transition-all duration-200 group-hover:translate-x-0 group-hover:opacity-100 dark:text-gray-500"
						fill="none"
						viewBox="0 0 24 24"
						stroke-width="2"
						stroke="currentColor"
					>
						<path stroke-linecap="round" stroke-linejoin="round" d="M8.25 4.5l7.5 7.5-7.5 7.5" />
					</svg>
				</div>
			</a>
		{/each}
	</div>

	<!-- SMS Delivery & Success Report Widget -->
	<div class="rounded-2xl border border-gray-200/80 bg-white p-5 sm:p-6 shadow-xs dark:border-gray-800 dark:bg-gray-900">
		<!-- Widget Header -->
		<div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
			<div class="flex items-center gap-3.5">
				<div
					class="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl border border-emerald-200 bg-emerald-50/70 text-emerald-600 dark:border-emerald-800/60 dark:bg-emerald-950/50 dark:text-emerald-400"
				>
					<svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
					</svg>
				</div>
				<div>
					<h2 class="text-base font-bold text-gray-950 dark:text-white">
						SMS Delivery &amp; Success Report
					</h2>
					<p class="text-xs text-gray-500 dark:text-gray-400">
						Live SMS delivery and success status report
					</p>
				</div>
			</div>

			<!-- Period Switcher Tabs -->
			<div class="inline-flex items-center self-start sm:self-auto rounded-xl bg-gray-100 p-1 dark:bg-gray-800">
				{#each periodTabs as tab (tab.key)}
					<button
						type="button"
						onclick={() => (selectedPeriod = tab.key)}
						class="rounded-lg px-3 py-1.5 text-xs transition-all {selectedPeriod === tab.key
							? 'bg-white font-semibold text-gray-900 shadow-2xs dark:bg-gray-900 dark:text-white'
							: 'font-medium text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-gray-200'}"
					>
						{tab.label}
					</button>
				{/each}
			</div>
		</div>

		<!-- 4 Period Metric Cards -->
		<div class="mt-5 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
			<!-- 1. Total Sent -->
			<div class="rounded-xl border border-gray-100 bg-gray-50/70 p-4 dark:border-gray-800 dark:bg-gray-800/40">
				<div class="text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
					TOTAL SENT
				</div>
				<div class="mt-1.5 text-2xl font-extrabold text-gray-950 dark:text-white">
					{formatInt(activePeriodStats.total_sent)}
				</div>
				<div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
					Cost: {formatMoney(activePeriodStats.cost)} BDT
				</div>
			</div>

			<!-- 2. Successful SMS -->
			<div class="rounded-xl border border-emerald-300/80 bg-emerald-50/30 p-4 dark:border-emerald-800/60 dark:bg-emerald-950/20">
				<div class="flex items-center justify-between gap-2">
					<span class="text-[11px] font-bold uppercase tracking-wider text-emerald-700 dark:text-emerald-400">
						SUCCESSFUL SMS
					</span>
					<span class="rounded-md bg-emerald-100 px-1.5 py-0.5 text-[11px] font-bold text-emerald-700 dark:bg-emerald-900/60 dark:text-emerald-300">
						{activePeriodStats.success_rate}%
					</span>
				</div>
				<div class="mt-1.5 text-2xl font-extrabold text-emerald-700 dark:text-emerald-400">
					{formatInt(activePeriodStats.successful)}
				</div>
				<div class="mt-1 text-xs font-medium text-emerald-600 dark:text-emerald-400">
					{formatInt(activePeriodStats.delivered)} handset delivered
				</div>
			</div>

			<!-- 3. Failed SMS -->
			<div class="rounded-xl border border-gray-200/90 bg-white p-4 dark:border-gray-800 dark:bg-gray-900">
				<div class="text-[11px] font-bold uppercase tracking-wider text-rose-700 dark:text-rose-400">
					FAILED SMS
				</div>
				<div class="mt-1.5 text-2xl font-extrabold text-rose-800 dark:text-rose-400">
					{formatInt(activePeriodStats.failed)}
				</div>
				<div class="mt-1 text-xs font-medium text-rose-600 dark:text-rose-400">
					{activePeriodStats.failed > 0 ? `${activePeriodStats.failed_rate}% failure rate` : 'No failures'}
				</div>
			</div>

			<!-- 4. Delivery Rate -->
			<div class="rounded-xl border border-gray-200/90 bg-white p-4 dark:border-gray-800 dark:bg-gray-900">
				<div class="text-[11px] font-bold uppercase tracking-wider text-blue-700 dark:text-blue-400">
					DELIVERY RATE
				</div>
				<div class="mt-1.5 text-2xl font-extrabold text-blue-600 dark:text-blue-400">
					{activePeriodStats.delivery_rate}%
				</div>
				<div class="mt-1 text-xs font-medium text-blue-600 dark:text-blue-400">
					Handset confirmed
				</div>
			</div>
		</div>

		<!-- Delivery Breakdown Bar -->
		<div class="mt-5">
			<div class="mb-2 flex items-center justify-between text-xs text-gray-500 dark:text-gray-400">
				<span>Delivery Breakdown</span>
				<span>{formatInt(activePeriodStats.total_sent)} messages</span>
			</div>
			<div class="flex h-2.5 w-full overflow-hidden rounded-full bg-gray-100 dark:bg-gray-800">
				{#if deliveredBarWidth > 0}
					<div
						class="h-full bg-emerald-500 transition-all duration-300"
						style="width: {deliveredBarWidth}%"
						title="Delivered: {activePeriodStats.delivered}"
					></div>
				{/if}
				{#if sentTelcoBarWidth > 0}
					<div
						class="h-full bg-sky-500 transition-all duration-300"
						style="width: {sentTelcoBarWidth}%"
						title="Sent (Telco): {activePeriodStats.sent_telco}"
					></div>
				{/if}
				{#if failedBarWidth > 0}
					<div
						class="h-full bg-rose-500 transition-all duration-300"
						style="width: {failedBarWidth}%"
						title="Failed: {activePeriodStats.failed}"
					></div>
				{/if}
			</div>
			<div class="mt-2.5 flex flex-wrap items-center gap-5 text-xs text-gray-600 dark:text-gray-400">
				<span class="inline-flex items-center gap-1.5">
					<span class="h-2.5 w-2.5 rounded-full bg-emerald-500"></span>
					Delivered: <strong class="font-bold text-gray-900 dark:text-white">{formatInt(activePeriodStats.delivered)}</strong>
				</span>
				<span class="inline-flex items-center gap-1.5">
					<span class="h-2.5 w-2.5 rounded-full bg-sky-500"></span>
					Sent (Telco): <strong class="font-bold text-gray-900 dark:text-white">{formatInt(activePeriodStats.sent_telco)}</strong>
				</span>
				<span class="inline-flex items-center gap-1.5">
					<span class="h-2.5 w-2.5 rounded-full bg-rose-500"></span>
					Failed: <strong class="font-bold text-gray-900 dark:text-white">{formatInt(activePeriodStats.failed)}</strong>
				</span>
			</div>
		</div>

		<!-- Footer Action Links -->
		<div class="mt-5 flex flex-wrap items-center justify-between gap-3 border-t border-gray-100 pt-4 dark:border-gray-800">
			<a
				href="/customer/messages?status=delivered,sent"
				class="inline-flex items-center gap-1.5 rounded-lg bg-emerald-50 px-3 py-1.5 text-xs font-semibold text-emerald-700 transition hover:bg-emerald-100 dark:bg-emerald-950/60 dark:text-emerald-300 dark:hover:bg-emerald-900/60"
			>
				<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke-width="2.5" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="m4.5 12.75 6 6 9-13.5" />
				</svg>
				View Successful Messages ({formatInt(activePeriodStats.successful)})
			</a>
			<a
				href="/customer/messages"
				class="inline-flex items-center gap-1 text-xs font-medium text-gray-500 transition hover:text-gray-900 dark:text-gray-400 dark:hover:text-white"
			>
				Detailed Message Log →
			</a>
		</div>
	</div>

	<!-- Recent SMS Table Widget -->
	<div class="overflow-hidden rounded-2xl border border-gray-200/80 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
		<!-- Widget Header -->
		<div class="flex items-center justify-between border-b border-gray-100 px-6 py-4 dark:border-gray-800">
			<h3 class="text-base font-bold text-gray-950 dark:text-white">Recent SMS</h3>
			<a
				href="/customer/messages"
				class="inline-flex items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-3 py-1.5 text-xs font-semibold text-gray-700 shadow-2xs transition hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200 dark:hover:bg-gray-700"
			>
				<svg class="h-3.5 w-3.5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="M13.5 4.5 21 12m0 0-7.5 7.5M21 12H3" />
				</svg>
				View All Messages
			</a>
		</div>

		<!-- Sub-toolbar (Column toggle) -->
		<div class="relative flex items-center justify-end border-b border-gray-100 px-6 py-2.5 dark:border-gray-800">
			<button
				type="button"
				onclick={() => (showRecentColMenu = !showRecentColMenu)}
				class="rounded-lg p-1.5 text-gray-400 transition hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-gray-800 dark:hover:text-gray-200"
				aria-label="Toggle columns"
				title="Toggle columns"
			>
				<svg class="h-4.5 w-4.5" viewBox="0 0 20 20" fill="currentColor">
					<path d="M2 4.75A.75.75 0 0 1 2.75 4h2.5a.75.75 0 0 1 .75.75v10.5a.75.75 0 0 1-.75.75h-2.5a.75.75 0 0 1-.75-.75V4.75ZM8 4.75A.75.75 0 0 1 8.75 4h2.5a.75.75 0 0 1 .75.75v10.5a.75.75 0 0 1-.75.75h-2.5a.75.75 0 0 1-.75-.75V4.75ZM14.75 4a.75.75 0 0 0-.75.75v10.5c0 .414.336.75.75.75h2.5a.75.75 0 0 0 .75-.75V4.75a.75.75 0 0 0-.75-.75h-2.5Z" />
				</svg>
			</button>
			{#if showRecentColMenu}
				<div class="absolute right-6 top-10 z-20 w-48 rounded-xl border border-gray-200 bg-white p-2 shadow-lg dark:border-gray-700 dark:bg-gray-800">
					<p class="px-2 py-1 text-[10px] font-bold uppercase tracking-wider text-gray-400">Columns</p>
					<label class="flex cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 text-xs text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-gray-700">
						<input type="checkbox" bind:checked={recentCols.recipient} class="rounded text-teal-600" />
						Recipient
					</label>
					<label class="flex cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 text-xs text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-gray-700">
						<input type="checkbox" bind:checked={recentCols.sender_id} class="rounded text-teal-600" />
						Sender ID
					</label>
					<label class="flex cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 text-xs text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-gray-700">
						<input type="checkbox" bind:checked={recentCols.body} class="rounded text-teal-600" />
						Message Content
					</label>
					<label class="flex cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 text-xs text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-gray-700">
						<input type="checkbox" bind:checked={recentCols.status} class="rounded text-teal-600" />
						Status
					</label>
					<label class="flex cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 text-xs text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-gray-700">
						<input type="checkbox" bind:checked={recentCols.cost} class="rounded text-teal-600" />
						Cost
					</label>
					<label class="flex cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 text-xs text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-gray-700">
						<input type="checkbox" bind:checked={recentCols.sent_at} class="rounded text-teal-600" />
						Sent At
					</label>
				</div>
			{/if}
		</div>

		<!-- Table -->
		<div class="overflow-x-auto">
			<table class="w-full text-left text-sm">
				<thead>
					<tr class="border-b border-gray-100 bg-gray-50/40 text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:border-gray-800 dark:bg-gray-800/30 dark:text-gray-400">
						{#if recentCols.recipient}
							<th class="px-6 py-3">RECIPIENT</th>
						{/if}
						{#if recentCols.sender_id}
							<th class="px-6 py-3">SENDER ID</th>
						{/if}
						{#if recentCols.body}
							<th class="px-6 py-3">MESSAGE CONTENT</th>
						{/if}
						{#if recentCols.status}
							<th class="px-6 py-3">STATUS</th>
						{/if}
						{#if recentCols.cost}
							<th class="px-6 py-3">COST</th>
						{/if}
						{#if recentCols.sent_at}
							<th class="px-6 py-3">SENT AT</th>
						{/if}
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-100 dark:divide-gray-800">
					{#if loadingStats && !stats}
						{#each { length: 4 } as _, i (i)}
							<tr>
								<td colspan="6" class="px-6 py-4">
									<div class="h-5 w-full animate-pulse rounded bg-gray-100 dark:bg-gray-800"></div>
								</td>
							</tr>
						{/each}
					{:else if recentMessages.length === 0}
						<tr>
							<td colspan="6" class="px-6 py-12 text-center">
								<div class="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-gray-100 text-gray-400 dark:bg-gray-800">
									<svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
										<path stroke-linecap="round" stroke-linejoin="round" d="M6 12 3.269 3.125A59.769 59.769 0 0 1 21.485 12 59.768 59.768 0 0 1 3.27 20.875L5.999 12Zm0 0h7.5" />
									</svg>
								</div>
								<p class="mt-3 text-sm font-bold text-gray-900 dark:text-white">No sent messages yet</p>
								<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">Messages you send will appear here.</p>
							</td>
						</tr>
					{:else}
						{#each recentMessages as msg (msg.id)}
							<tr
								class="cursor-pointer transition hover:bg-gray-50/70 dark:hover:bg-gray-800/40"
								onclick={() => goto('/customer/messages')}
							>
								{#if recentCols.recipient}
									<td class="whitespace-nowrap px-6 py-3.5">
										<button
											type="button"
											onclick={(e) => {
												e.stopPropagation();
												copyText(msg.to, 'Recipient number');
											}}
											title="Click to copy"
											class="inline-flex items-center rounded-md bg-emerald-50 px-2.5 py-1 text-xs font-semibold text-emerald-700 ring-1 ring-inset ring-emerald-600/20 transition hover:bg-emerald-100 dark:bg-emerald-950/60 dark:text-emerald-300 dark:ring-emerald-500/30"
										>
											{msg.to}
										</button>
									</td>
								{/if}
								{#if recentCols.sender_id}
									<td class="whitespace-nowrap px-6 py-3.5">
										{#if msg.sender_id && msg.sender_id !== '—'}
											<span class="inline-flex items-center rounded-md bg-teal-50 px-2.5 py-1 text-xs font-semibold text-teal-700 ring-1 ring-inset ring-teal-600/20 dark:bg-teal-950/60 dark:text-teal-300 dark:ring-teal-500/30">
												{msg.sender_id}
											</span>
										{:else}
											<span class="text-xs text-gray-400">—</span>
										{/if}
									</td>
								{/if}
								{#if recentCols.body}
									<td class="max-w-md px-6 py-3.5 text-sm text-gray-800 dark:text-gray-200" title={msg.body}>
										<span class="line-clamp-1">
											{msg.body && msg.body.length > 60 ? msg.body.slice(0, 60) + '...' : msg.body}
										</span>
									</td>
								{/if}
								{#if recentCols.status}
									<td class="whitespace-nowrap px-6 py-3.5">
										{#if msg.status === 'delivered'}
											<span class="inline-flex items-center gap-1 rounded-md bg-emerald-50 px-2.5 py-1 text-xs font-semibold text-emerald-700 ring-1 ring-inset ring-emerald-600/20 dark:bg-emerald-950/60 dark:text-emerald-300">
												<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
													<path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
												</svg>
												Delivered
											</span>
										{:else if msg.status === 'sent'}
											<span class="inline-flex items-center gap-1 rounded-md bg-sky-50 px-2.5 py-1 text-xs font-semibold text-sky-700 ring-1 ring-inset ring-sky-600/20 dark:bg-sky-950/60 dark:text-sky-300">
												<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
													<path stroke-linecap="round" stroke-linejoin="round" d="m4.5 12.75 6 6 9-13.5" />
												</svg>
												Sent
											</span>
										{:else if msg.status === 'failed' || msg.status === 'dlr_failed'}
											<span class="inline-flex items-center gap-1 rounded-md bg-rose-50 px-2.5 py-1 text-xs font-semibold text-rose-700 ring-1 ring-inset ring-rose-600/20 dark:bg-rose-950/60 dark:text-rose-300">
												<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
													<path stroke-linecap="round" stroke-linejoin="round" d="m9.75 9.75 4.5 4.5m0-4.5-4.5 4.5M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
												</svg>
												Failed
											</span>
										{:else}
											<span class="inline-flex items-center gap-1 rounded-md bg-amber-50 px-2.5 py-1 text-xs font-semibold text-amber-700 ring-1 ring-inset ring-amber-600/20 dark:bg-amber-950/60 dark:text-amber-300">
												<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
													<path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
												</svg>
												{msg.status}
											</span>
										{/if}
									</td>
								{/if}
								{#if recentCols.cost}
									<td class="whitespace-nowrap px-6 py-3.5 text-sm font-medium text-gray-800 dark:text-gray-200">
										BDT {formatMoney(msg.cost)}
									</td>
								{/if}
								{#if recentCols.sent_at}
									<td class="whitespace-nowrap px-6 py-3.5 text-xs text-gray-500 dark:text-gray-400">
										{formatDateTime(msg.created_at)}
									</td>
								{/if}
							</tr>
						{/each}
					{/if}
				</tbody>
			</table>
		</div>
	</div>

	<!-- Upcoming Scheduled SMS Table Widget -->
	<div class="overflow-hidden rounded-2xl border border-gray-200/80 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
		<!-- Widget Header -->
		<div class="flex items-center justify-between border-b border-gray-100 px-6 py-4 dark:border-gray-800">
			<h3 class="text-base font-bold text-gray-950 dark:text-white">Upcoming Scheduled SMS</h3>
			<a
				href="/customer/scheduled-messages"
				class="inline-flex items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-3 py-1.5 text-xs font-semibold text-gray-700 shadow-2xs transition hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200 dark:hover:bg-gray-700"
			>
				<svg class="h-3.5 w-3.5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="M13.5 4.5 21 12m0 0-7.5 7.5M21 12H3" />
				</svg>
				View All Scheduled
			</a>
		</div>

		<!-- Sub-toolbar (Column toggle) -->
		<div class="relative flex items-center justify-end border-b border-gray-100 px-6 py-2.5 dark:border-gray-800">
			<button
				type="button"
				onclick={() => (showScheduledColMenu = !showScheduledColMenu)}
				class="rounded-lg p-1.5 text-gray-400 transition hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-gray-800 dark:hover:text-gray-200"
				aria-label="Toggle scheduled columns"
				title="Toggle columns"
			>
				<svg class="h-4.5 w-4.5" viewBox="0 0 20 20" fill="currentColor">
					<path d="M2 4.75A.75.75 0 0 1 2.75 4h2.5a.75.75 0 0 1 .75.75v10.5a.75.75 0 0 1-.75.75h-2.5a.75.75 0 0 1-.75-.75V4.75ZM8 4.75A.75.75 0 0 1 8.75 4h2.5a.75.75 0 0 1 .75.75v10.5a.75.75 0 0 1-.75.75h-2.5a.75.75 0 0 1-.75-.75V4.75ZM14.75 4a.75.75 0 0 0-.75.75v10.5c0 .414.336.75.75.75h2.5a.75.75 0 0 0 .75-.75V4.75a.75.75 0 0 0-.75-.75h-2.5Z" />
				</svg>
			</button>
			{#if showScheduledColMenu}
				<div class="absolute right-6 top-10 z-20 w-48 rounded-xl border border-gray-200 bg-white p-2 shadow-lg dark:border-gray-700 dark:bg-gray-800">
					<p class="px-2 py-1 text-[10px] font-bold uppercase tracking-wider text-gray-400">Columns</p>
					<label class="flex cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 text-xs text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-gray-700">
						<input type="checkbox" bind:checked={scheduledCols.sender_id} class="rounded text-teal-600" />
						Sender ID
					</label>
					<label class="flex cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 text-xs text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-gray-700">
						<input type="checkbox" bind:checked={scheduledCols.recipient} class="rounded text-teal-600" />
						Recipient
					</label>
					<label class="flex cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 text-xs text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-gray-700">
						<input type="checkbox" bind:checked={scheduledCols.recipients} class="rounded text-teal-600" />
						Recipients
					</label>
					<label class="flex cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 text-xs text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-gray-700">
						<input type="checkbox" bind:checked={scheduledCols.body} class="rounded text-teal-600" />
						Message Content
					</label>
					<label class="flex cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 text-xs text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-gray-700">
						<input type="checkbox" bind:checked={scheduledCols.scheduled_for} class="rounded text-teal-600" />
						Scheduled For
					</label>
					<label class="flex cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 text-xs text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-gray-700">
						<input type="checkbox" bind:checked={scheduledCols.status} class="rounded text-teal-600" />
						Status
					</label>
				</div>
			{/if}
		</div>

		<!-- Table -->
		<div class="overflow-x-auto">
			<table class="w-full text-left text-sm">
				<thead>
					<tr class="border-b border-gray-100 bg-gray-50/40 text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:border-gray-800 dark:bg-gray-800/30 dark:text-gray-400">
						{#if scheduledCols.sender_id}
							<th class="px-6 py-3">SENDER ID</th>
						{/if}
						{#if scheduledCols.recipient}
							<th class="px-6 py-3">RECIPIENT</th>
						{/if}
						{#if scheduledCols.recipients}
							<th class="px-6 py-3">RECIPIENTS</th>
						{/if}
						{#if scheduledCols.body}
							<th class="px-6 py-3">MESSAGE CONTENT</th>
						{/if}
						{#if scheduledCols.scheduled_for}
							<th class="px-6 py-3">
								<button
									type="button"
									onclick={toggleScheduledSort}
									class="inline-flex items-center gap-1 font-bold uppercase tracking-wider text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200"
								>
									SCHEDULED FOR
									<svg
										class="h-3.5 w-3.5 transition-transform {scheduledSortDir === 'desc' ? 'rotate-180' : ''}"
										fill="none"
										viewBox="0 0 24 24"
										stroke-width="2"
										stroke="currentColor"
									>
										<path stroke-linecap="round" stroke-linejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" />
									</svg>
								</button>
							</th>
						{/if}
						{#if scheduledCols.status}
							<th class="px-6 py-3">STATUS</th>
						{/if}
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-100 dark:divide-gray-800">
					{#if loadingStats && !stats}
						<tr>
							<td colspan="6" class="px-6 py-8">
								<div class="h-6 w-full animate-pulse rounded bg-gray-100 dark:bg-gray-800"></div>
							</td>
						</tr>
					{:else if sortedScheduledMessages.length === 0}
						<tr>
							<td colspan="6" class="px-6 py-14 text-center">
								<div class="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-gray-100 text-gray-400 dark:bg-gray-800">
									<svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
										<path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
									</svg>
								</div>
								<p class="mt-3.5 text-sm font-bold text-gray-900 dark:text-white">
									No upcoming scheduled messages
								</p>
								<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
									You do not have any scheduled messages in queue.
								</p>
								<div class="mt-4">
									<a
										href="/customer/messages/quick?schedule=1"
										class="inline-flex items-center gap-1.5 rounded-lg border border-teal-600/80 bg-white px-3.5 py-2 text-xs font-semibold text-teal-700 shadow-2xs transition hover:bg-teal-50 dark:border-teal-500/60 dark:bg-gray-900 dark:text-teal-400 dark:hover:bg-teal-950/40"
									>
										<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor">
											<path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
										</svg>
										Schedule New SMS
									</a>
								</div>
							</td>
						</tr>
					{:else}
						{#each sortedScheduledMessages as item (item.id)}
							<tr
								class="cursor-pointer transition hover:bg-gray-50/70 dark:hover:bg-gray-800/40"
								onclick={() => goto('/customer/scheduled-messages')}
							>
								{#if scheduledCols.sender_id}
									<td class="whitespace-nowrap px-6 py-3.5">
										{#if item.sender_id && item.sender_id !== '—'}
											<span class="inline-flex items-center rounded-md bg-teal-50 px-2.5 py-1 text-xs font-semibold text-teal-700 ring-1 ring-inset ring-teal-600/20 dark:bg-teal-950/60 dark:text-teal-300">
												{item.sender_id}
											</span>
										{:else}
											<span class="text-xs text-gray-400">—</span>
										{/if}
									</td>
								{/if}
								{#if scheduledCols.recipient}
									<td class="whitespace-nowrap px-6 py-3.5">
										<button
											type="button"
											onclick={(e) => {
												e.stopPropagation();
												copyText(item.recipient_preview, 'Recipient');
											}}
											class="inline-flex items-center rounded-md bg-emerald-50 px-2.5 py-1 text-xs font-semibold text-emerald-700 ring-1 ring-inset ring-emerald-600/20 transition hover:bg-emerald-100 dark:bg-emerald-950/60 dark:text-emerald-300"
										>
											{item.recipient_preview}
										</button>
									</td>
								{/if}
								{#if scheduledCols.recipients}
									<td class="whitespace-nowrap px-6 py-3.5 font-semibold text-gray-900 dark:text-white">
										{formatInt(item.total_recipients)}
									</td>
								{/if}
								{#if scheduledCols.body}
									<td class="max-w-md px-6 py-3.5 text-sm text-gray-800 dark:text-gray-200" title={item.body}>
										<span class="line-clamp-1">
											{item.body && item.body.length > 50 ? item.body.slice(0, 50) + '...' : item.body}
										</span>
									</td>
								{/if}
								{#if scheduledCols.scheduled_for}
									<td class="whitespace-nowrap px-6 py-3.5">
										<div class="text-xs font-medium text-gray-900 dark:text-white">
											{formatDateTime(item.scheduled_at)}
										</div>
										<div class="text-[11px] text-gray-500 dark:text-gray-400">
											{formatRelativeTime(item.scheduled_at)}
										</div>
									</td>
								{/if}
								{#if scheduledCols.status}
									<td class="whitespace-nowrap px-6 py-3.5">
										{#if item.status === 'paused'}
											<span class="inline-flex items-center gap-1 rounded-md bg-amber-50 px-2.5 py-1 text-xs font-semibold text-amber-700 ring-1 ring-inset ring-amber-600/20 dark:bg-amber-950/60 dark:text-amber-300">
												Paused
											</span>
										{:else}
											<span class="inline-flex items-center gap-1 rounded-md bg-violet-50 px-2.5 py-1 text-xs font-semibold text-violet-700 ring-1 ring-inset ring-violet-600/20 dark:bg-violet-950/60 dark:text-violet-300">
												Scheduled
											</span>
										{/if}
									</td>
								{/if}
							</tr>
						{/each}
					{/if}
				</tbody>
			</table>
		</div>
	</div>
</div>
