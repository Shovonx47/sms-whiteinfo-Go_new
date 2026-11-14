<script lang="ts">
	import { api } from '$lib/api.js';
	import { onMount, onDestroy } from 'svelte';

	interface LiveBatch {
		id: number;
		status: string;
		sender_id: string;
		body: string;
		total_recipients: number;
		processed: number;
		percent: number;
		created_at: string;
		time_ago: string;
	}

	interface LiveMessage {
		id: number;
		msg_id: string;
		to: string;
		body: string;
		status: string;
		cost: number;
		parts: number;
		sender_id: string;
		operator: string;
		gateway: string;
		action_type: string;
		is_bulk: boolean;
		timestamp: string;
		created_at: string;
		time_ago: string;
	}

	interface QueueApiResponse {
		stats?: {
			in_queue: number;
			msgs_per_min: number;
			active_batches: number;
			live_success: string;
			success_count: number;
			total: number;
		};
		batches?: LiveBatch[];
		messages?: LiveMessage[];
		data?: {
			pending: number;
			processing: number;
			messages: LiveMessage[];
		};
	}

	let stats = $state({
		in_queue: 0,
		msgs_per_min: 0.0,
		active_batches: 0,
		live_success: '0%',
		success_count: 0,
		total: 0
	});

	let batches = $state<LiveBatch[]>([]);
	let messages = $state<LiveMessage[]>([]);
	let loading = $state(true);
	let isPaused = $state(false);

	// Controls & Filters
	let search = $state('');
	let viewMode = $state<'stream' | 'table'>('stream');
	let statusFilter = $state<string | null>(null); // 'pending' | 'sending' | 'dispatched' | null
	let typeFilter = $state<string | null>(null); // 'quick' | 'bulk' | null

	let pollTimer: ReturnType<typeof setInterval> | null = null;

	async function fetchQueue() {
		try {
			const res = await api.get<QueueApiResponse>('/api/v1/customer/live-queue');
			if (res.stats) {
				stats = res.stats;
			} else if (res.data) {
				stats = {
					in_queue: res.data.pending ?? 0,
					msgs_per_min: 0.0,
					active_batches: res.data.processing ?? 0,
					live_success: '0%',
					success_count: 0,
					total: 0
				};
			}

			batches = res.batches ?? [];
			messages = res.messages ?? res.data?.messages ?? [];
		} catch {
			// Zero empty state if API fails or empty - NO fake mock records
			stats = { in_queue: 0, msgs_per_min: 0.0, active_batches: 0, live_success: '0%', success_count: 0, total: 0 };
			batches = [];
			messages = [];
		} finally {
			loading = false;
		}
	}

	function startPolling() {
		isPaused = false;
		fetchQueue();
		if (!pollTimer) {
			pollTimer = setInterval(() => {
				if (!isPaused) fetchQueue();
			}, 3000);
		}
	}

	function togglePause() {
		isPaused = !isPaused;
	}

	onMount(() => {
		startPolling();
	});

	onDestroy(() => {
		if (pollTimer) clearInterval(pollTimer);
	});

	// Filtered messages
	let filteredMessages = $derived.by(() => {
		let list = messages;

		if (statusFilter === 'pending') {
			list = list.filter((m) => m.status === 'pending');
		} else if (statusFilter === 'sending') {
			list = list.filter((m) => m.status === 'sending' || m.status === 'processing');
		} else if (statusFilter === 'dispatched') {
			list = list.filter((m) => m.status === 'sent' || m.status === 'delivered');
		}

		if (typeFilter === 'quick') {
			list = list.filter((m) => !m.is_bulk && m.action_type === 'Quick SMS');
		} else if (typeFilter === 'bulk') {
			list = list.filter((m) => m.is_bulk || m.action_type === 'Bulk SMS');
		}

		if (search.trim()) {
			const q = search.trim().toLowerCase();
			list = list.filter((m) => (m.to && m.to.toLowerCase().includes(q)) || (m.body && m.body.toLowerCase().includes(q)));
		}

		return list;
	});

	function getStatusBadge(status: string) {
		switch (status) {
			case 'pending':
				return { bg: 'bg-amber-50 text-amber-700 border-amber-200 dark:bg-amber-950/40 dark:text-amber-400 dark:border-amber-800', dot: 'bg-amber-500', label: 'IN QUEUE', pulse: true };
			case 'sending':
			case 'processing':
				return { bg: 'bg-blue-50 text-blue-700 border-blue-200 dark:bg-blue-950/40 dark:text-blue-400 dark:border-blue-800', dot: 'bg-blue-500', label: 'SENDING', pulse: true };
			case 'sent':
				return { bg: 'bg-cyan-50 text-cyan-700 border-cyan-200 dark:bg-cyan-950/40 dark:text-cyan-400 dark:border-cyan-800', dot: 'bg-cyan-500', label: 'DISPATCHED', pulse: false };
			case 'delivered':
				return { bg: 'bg-emerald-50 text-emerald-700 border-emerald-200 dark:bg-emerald-950/40 dark:text-emerald-400 dark:border-emerald-800', dot: 'bg-emerald-500', label: 'DELIVERED', pulse: false };
			case 'failed':
			case 'dlr_failed':
				return { bg: 'bg-red-50 text-red-700 border-red-200 dark:bg-red-950/40 dark:text-red-400 dark:border-red-800', dot: 'bg-red-500', label: 'FAILED', pulse: false };
			default:
				return { bg: 'bg-gray-50 text-gray-700 border-gray-200 dark:bg-gray-800 dark:text-gray-300 dark:border-gray-700', dot: 'bg-gray-400', label: status.toUpperCase(), pulse: false };
		}
	}
</script>

<svelte:head>
	<title>Live Queue - White INFO</title>
</svelte:head>

<div class="space-y-6">
	<!-- Page Title -->
	<div>
		<h1 class="text-2xl font-bold tracking-tight text-gray-900 dark:text-gray-100">My Live Queue</h1>
	</div>

	<!-- 4 Stat Cards in Grid (matching Screenshots 4, 5, 6) -->
	<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
		<!-- 1. In Queue Now -->
		<div class="rounded-xl border border-gray-200 bg-white p-5 shadow-xs dark:border-gray-800 dark:bg-gray-900">
			<div class="flex items-center justify-between text-gray-500 dark:text-gray-400">
				<div class="flex items-center gap-1.5 text-xs font-medium">
					<svg class="h-4 w-4 text-amber-500" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="m3.75 13.5 10.5-11.25L12 10.5h8.25L9.75 21.75 12 13.5H3.75Z" />
					</svg>
					<span>In Queue Now</span>
				</div>
			</div>
			<div class="mt-2">
				<p class="text-2xl font-bold text-gray-900 dark:text-gray-100">{stats.in_queue}</p>
				<p class="mt-1 text-xs text-emerald-600 dark:text-emerald-400">Your pending & sending messages</p>
			</div>
		</div>

		<!-- 2. Msgs / Min -->
		<div class="rounded-xl border border-gray-200 bg-white p-5 shadow-xs dark:border-gray-800 dark:bg-gray-900">
			<div class="flex items-center justify-between text-gray-500 dark:text-gray-400">
				<div class="flex items-center gap-1.5 text-xs font-medium">
					<svg class="h-4 w-4 text-sky-500" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M2.25 18 9 11.25l4.306 4.306a11.95 11.95 0 0 1 5.814-5.518l2.74-1.22m0 0-5.94-2.281m5.94 2.28-2.28 5.941" />
					</svg>
					<span>Msgs / Min</span>
				</div>
			</div>
			<div class="mt-2">
				<p class="text-2xl font-bold text-gray-900 dark:text-gray-100">{stats.msgs_per_min.toFixed(1)}</p>
				<p class="mt-1 text-xs text-sky-600 dark:text-sky-400">Your 5-minute average rate</p>
			</div>
		</div>

		<!-- 3. Active Batches -->
		<div class="rounded-xl border border-gray-200 bg-white p-5 shadow-xs dark:border-gray-800 dark:bg-gray-900">
			<div class="flex items-center justify-between text-gray-500 dark:text-gray-400">
				<div class="flex items-center gap-1.5 text-xs font-medium">
					<svg class="h-4 w-4 text-blue-500" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M6 6.878V6a2.25 2.25 0 0 1 2.25-2.25h7.5A2.25 2.25 0 0 1 18 6v.878m-12 0c.235-.083.487-.128.75-.128h10.5c.263 0 .515.045.75.128m-12 0A2.25 2.25 0 0 0 4.5 9v.878m13.5-3A2.25 2.25 0 0 1 19.5 9v.878m-15 0a2.246 2.246 0 0 0-.75.128m15.75 0c-.235-.083-.487-.128-.75-.128m-15 0A2.25 2.25 0 0 0 3 12v4.5A2.25 2.25 0 0 0 5.25 18.75h13.5A2.25 2.25 0 0 0 21 16.5V12a2.25 2.25 0 0 0-1.5-2.122" />
					</svg>
					<span>Active Batches</span>
				</div>
			</div>
			<div class="mt-2">
				<p class="text-2xl font-bold text-gray-900 dark:text-gray-100">{stats.active_batches}</p>
				<p class="mt-1 text-xs text-gray-500">Bulk batches currently processing</p>
			</div>
		</div>

		<!-- 4. Live Success -->
		<div class="rounded-xl border border-gray-200 bg-white p-5 shadow-xs dark:border-gray-800 dark:bg-gray-900">
			<div class="flex items-center justify-between text-gray-500 dark:text-gray-400">
				<div class="flex items-center gap-1.5 text-xs font-medium">
					<svg class="h-4 w-4 text-emerald-500" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
					</svg>
					<span>Live Success</span>
				</div>
			</div>
			<div class="mt-2">
				<p class="text-2xl font-bold text-gray-900 dark:text-gray-100">{stats.live_success}</p>
				<p class="mt-1 text-xs text-rose-500">{stats.success_count}/{stats.total} delivered (5 min)</p>
			</div>
		</div>
	</div>

	<!-- Active Bulk Batches (if any active) -->
	{#if batches.length > 0}
		<div class="rounded-xl border border-gray-200 bg-white p-4 shadow-xs dark:border-gray-800 dark:bg-gray-900 space-y-3">
			<div class="flex items-center justify-between">
				<div class="flex items-center gap-2">
					<span class="flex h-2 w-2 rounded-full bg-cyan-500 animate-ping"></span>
					<h3 class="text-xs font-bold text-gray-900 dark:text-gray-100">Active Bulk Batches</h3>
				</div>
				<span class="text-[10px] font-semibold text-cyan-600 uppercase">LIVE</span>
			</div>
			<div class="divide-y divide-gray-100 dark:divide-gray-800">
				{#each batches as b (b.id)}
					<div class="py-2.5">
						<div class="flex items-center justify-between text-xs mb-1.5">
							<div class="flex items-center gap-2">
								<span class="font-mono text-gray-500">#{b.id}</span>
								<span class="rounded bg-blue-100 text-blue-700 px-1.5 py-0.5 text-[10px] font-bold">{b.status.toUpperCase()}</span>
								<span class="font-mono text-gray-600 dark:text-gray-300">{b.sender_id}</span>
							</div>
							<div class="flex items-center gap-2 font-semibold">
								<span>{b.percent}%</span>
								<span class="text-gray-400 font-normal text-[10px]">{b.time_ago}</span>
							</div>
						</div>
						<div class="w-full bg-gray-100 rounded-full h-1.5 overflow-hidden dark:bg-gray-800">
							<div class="bg-blue-600 h-1.5 rounded-full transition-all duration-500" style="width: {b.percent}%"></div>
						</div>
					</div>
				{/each}
			</div>
		</div>
	{/if}

	<!-- Main Card: My Live SMS Queue -->
	<div class="rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900 overflow-hidden">
		<!-- Header Toolbar -->
		<div class="p-4 border-b border-gray-100 dark:border-gray-800 bg-gray-50/50 dark:bg-gray-800/30">
			<div class="flex flex-col lg:flex-row lg:items-center lg:justify-between gap-3">
				<!-- Title & Live Pill -->
				<div class="flex items-center gap-3">
					<div class="flex h-8 w-8 items-center justify-center rounded-lg bg-emerald-50 text-emerald-600 dark:bg-emerald-950/40 dark:text-emerald-400">
						<svg class="h-4.5 w-4.5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" d="m3.75 13.5 10.5-11.25L12 10.5h8.25L9.75 21.75 12 13.5H3.75Z" />
						</svg>
					</div>
					<div>
						<div class="flex items-center gap-2">
							<h2 class="text-sm font-bold text-gray-900 dark:text-gray-100">My Live SMS Queue</h2>
							{#if !isPaused}
								<span class="inline-flex items-center gap-1 rounded-full border border-emerald-200 bg-emerald-50 px-2 py-0.5 text-[10px] font-bold text-emerald-700 dark:border-emerald-800 dark:bg-emerald-950/50 dark:text-emerald-400">
									<span class="h-1.5 w-1.5 rounded-full bg-emerald-500 animate-ping"></span>
									LIVE (3s)
								</span>
							{:else}
								<span class="inline-flex items-center gap-1 rounded-full border border-amber-200 bg-amber-50 px-2 py-0.5 text-[10px] font-bold text-amber-700 dark:border-amber-800 dark:bg-amber-950/50 dark:text-amber-400">
									PAUSED
								</span>
							{/if}
						</div>
						<p class="text-xs text-gray-500 dark:text-gray-400">
							Real-time view of your SMS messages — pending, in-flight, and recently dispatched
						</p>
					</div>
				</div>

				<!-- Controls: Search, View Mode, Pause -->
				<div class="flex flex-wrap items-center gap-2">
					<div class="relative min-w-[200px]">
						<input
							type="text"
							bind:value={search}
							placeholder="Search recipient or message..."
							class="w-full rounded-lg border border-gray-200 bg-white py-1.5 pl-8 pr-3 text-xs text-gray-800 placeholder-gray-400 focus:border-teal-500 focus:outline-hidden dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200"
						/>
						<svg class="h-3.5 w-3.5 text-gray-400 absolute left-2.5 top-2.5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
						</svg>
					</div>

					<div class="flex items-center rounded-lg border border-gray-200 bg-gray-100 p-0.5 dark:border-gray-700 dark:bg-gray-800">
						<button
							type="button"
							onclick={() => (viewMode = 'stream')}
							class="flex items-center gap-1 rounded-md px-2.5 py-1 text-xs font-medium transition {viewMode === 'stream' ? 'bg-white text-gray-900 shadow-xs dark:bg-gray-700 dark:text-white' : 'text-gray-500 hover:text-gray-700'}"
						>
							<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
								<path stroke-linecap="round" stroke-linejoin="round" d="M8.25 6.75h12M8.25 12h12m-12 5.25h12M3.75 6.75h.007v.008H3.75V6.75Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0ZM3.75 12h.007v.008H3.75V12Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm-.375 5.25h.007v.008H3.75v-.008Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Z" />
							</svg>
							<span>Stream</span>
						</button>
						<button
							type="button"
							onclick={() => (viewMode = 'table')}
							class="flex items-center gap-1 rounded-md px-2.5 py-1 text-xs font-medium transition {viewMode === 'table' ? 'bg-white text-gray-900 shadow-xs dark:bg-gray-700 dark:text-white' : 'text-gray-500 hover:text-gray-700'}"
						>
							<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
								<path stroke-linecap="round" stroke-linejoin="round" d="M3.375 19.5h17.25m-17.25 0a1.125 1.125 0 0 1-1.125-1.125M3.375 19.5h1.5C5.496 19.5 6 18.996 6 18.375m-3.75 0V5.625m0 12.75a1.125 1.125 0 0 0 1.125 1.125m0 0h1.5m-2.625-15c0-.621.504-1.125 1.125-1.125h17.25c.621 0 1.125.504 1.125 1.125v12.75c0 .621-.504 1.125-1.125 1.125H3.375A1.125 1.125 0 0 1 2.25 18.375V5.625Z" />
							</svg>
							<span>Table</span>
						</button>
					</div>

					<button
						type="button"
						onclick={togglePause}
						class="inline-flex items-center gap-1 rounded-lg border border-gray-200 bg-white px-2.5 py-1 text-xs font-medium text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300 cursor-pointer"
					>
						{#if isPaused}
							<svg class="h-3.5 w-3.5 text-emerald-600" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
								<path stroke-linecap="round" stroke-linejoin="round" d="M5.25 5.653c0-.856.917-1.398 1.667-.986l11.54 6.347a1.125 1.125 0 0 1 0 1.972l-11.54 6.347a1.125 1.125 0 0 1-1.667-.986V5.653Z" />
							</svg>
							<span>Resume</span>
						{:else}
							<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
								<path stroke-linecap="round" stroke-linejoin="round" d="M15.75 5.25v13.5m-7.5-13.5v13.5" />
							</svg>
							<span>Pause</span>
						{/if}
					</button>
				</div>
			</div>

			<!-- Filters Bar: STATUS & TYPE -->
			<div class="mt-3 pt-3 border-t border-gray-100 dark:border-gray-800 flex flex-wrap items-center justify-between gap-3 text-xs">
				<!-- Status Filters -->
				<div class="flex items-center gap-1.5">
					<span class="text-[10px] font-bold text-gray-400 uppercase tracking-wider mr-1">STATUS:</span>
					<button
						type="button"
						onclick={() => (statusFilter = null)}
						class="rounded px-2.5 py-0.5 text-xs font-medium transition cursor-pointer {statusFilter === null ? 'bg-gray-900 text-white dark:bg-white dark:text-gray-900' : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-gray-800 dark:text-gray-400'}"
					>
						All Live
					</button>
					<button
						type="button"
						onclick={() => (statusFilter = statusFilter === 'pending' ? null : 'pending')}
						class="rounded px-2.5 py-0.5 text-xs font-medium transition cursor-pointer {statusFilter === 'pending' ? 'bg-amber-600 text-white' : 'bg-amber-50 text-amber-700 hover:bg-amber-100 dark:bg-amber-950/40 dark:text-amber-300'}"
					>
						⏳ Pending
					</button>
					<button
						type="button"
						onclick={() => (statusFilter = statusFilter === 'sending' ? null : 'sending')}
						class="rounded px-2.5 py-0.5 text-xs font-medium transition cursor-pointer {statusFilter === 'sending' ? 'bg-blue-600 text-white' : 'bg-blue-50 text-blue-700 hover:bg-blue-100 dark:bg-blue-950/40 dark:text-blue-300'}"
					>
						⚡ Sending
					</button>
					<button
						type="button"
						onclick={() => (statusFilter = statusFilter === 'dispatched' ? null : 'dispatched')}
						class="rounded px-2.5 py-0.5 text-xs font-medium transition cursor-pointer {statusFilter === 'dispatched' ? 'bg-emerald-600 text-white' : 'bg-emerald-50 text-emerald-700 hover:bg-emerald-100 dark:bg-emerald-950/40 dark:text-emerald-300'}"
					>
						✓ Dispatched
					</button>
				</div>

				<!-- Type Filters -->
				<div class="flex items-center gap-1.5">
					<span class="text-[10px] font-bold text-gray-400 uppercase tracking-wider mr-1">TYPE:</span>
					<button
						type="button"
						onclick={() => (typeFilter = null)}
						class="rounded px-2 py-0.5 text-xs font-medium transition cursor-pointer {typeFilter === null ? 'bg-gray-900 text-white dark:bg-white dark:text-gray-900' : 'text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-800'}"
					>
						All
					</button>
					<button
						type="button"
						onclick={() => (typeFilter = typeFilter === 'quick' ? null : 'quick')}
						class="rounded px-2 py-0.5 text-xs font-medium transition cursor-pointer {typeFilter === 'quick' ? 'bg-amber-600 text-white' : 'text-amber-600 hover:bg-amber-50 dark:hover:bg-amber-950/30'}"
					>
						⚡ Quick SMS
					</button>
					<button
						type="button"
						onclick={() => (typeFilter = typeFilter === 'bulk' ? null : 'bulk')}
						class="rounded px-2 py-0.5 text-xs font-medium transition cursor-pointer {typeFilter === 'bulk' ? 'bg-purple-600 text-white' : 'text-purple-600 hover:bg-purple-50 dark:hover:bg-purple-950/30'}"
					>
						📦 Bulk SMS
					</button>
				</div>
			</div>
		</div>

		<!-- Body: Stream or Table or Empty State -->
		{#if filteredMessages.length === 0}
			<!-- Empty State with Radar Pulse (matching Screenshots 4, 5, 6) -->
			<div class="py-16 px-6 text-center">
				<div class="relative w-16 h-16 mx-auto mb-4 flex items-center justify-center">
					<span class="absolute w-full h-full rounded-full bg-emerald-500/10 animate-ping opacity-75"></span>
					<span class="absolute w-11 h-11 rounded-full bg-emerald-500/20 animate-pulse"></span>
					<div class="relative w-8 h-8 rounded-full bg-emerald-500/30 flex items-center justify-center text-emerald-600 dark:text-emerald-400">
						<svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" d="M9.348 14.652a3.75 3.75 0 0 1 0-5.304m5.304 0a3.75 3.75 0 0 1 0 5.304m-7.425 2.121a6.75 6.75 0 0 1 0-9.546m9.546 0a6.75 6.75 0 0 1 0 9.546M12 12h.008v.008H12V12Z" />
						</svg>
					</div>
				</div>
				<h3 class="text-sm font-bold text-gray-900 dark:text-white tracking-tight">Your queue is idle</h3>
				<p class="text-xs text-gray-500 dark:text-gray-400 max-w-sm mx-auto mt-1">
					No messages are currently in your queue or in-flight. Send a Quick SMS or Bulk SMS batch to see live updates here.
				</p>
				<div class="inline-flex items-center gap-1.5 mt-4 px-3 py-1 rounded-full bg-gray-100 dark:bg-gray-800 text-[11px] text-gray-600 dark:text-gray-300 font-mono">
					<span class="w-1.5 h-1.5 rounded-full bg-emerald-500 animate-ping"></span>
					<span>Listening for your messages · polling every 3s</span>
				</div>
			</div>
		{:else if viewMode === 'stream'}
			<!-- Stream View -->
			<div class="divide-y divide-gray-100 dark:divide-gray-800 max-h-[600px] overflow-y-auto">
				{#each filteredMessages as msg (msg.id)}
					{@const badge = getStatusBadge(msg.status)}
					<div class="p-4 hover:bg-gray-50/70 dark:hover:bg-gray-800/40 transition">
						<div class="flex items-start justify-between gap-3">
							<div class="min-w-0 flex-1">
								<div class="flex items-center gap-2 mb-1">
									<span class="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[10px] font-bold border {badge.bg}">
										<span class="h-1.5 w-1.5 rounded-full {badge.dot} {badge.pulse ? 'animate-ping' : ''}"></span>
										{badge.label}
									</span>
									<span class="text-xs font-mono font-semibold text-gray-900 dark:text-gray-100">{msg.to}</span>
									<span class="text-xs text-gray-400">via</span>
									<span class="text-xs font-medium text-gray-700 dark:text-gray-300">{msg.sender_id}</span>
									<span class="rounded bg-gray-100 text-gray-600 px-1.5 py-0.5 text-[10px] font-medium dark:bg-gray-800 dark:text-gray-400">{msg.action_type}</span>
								</div>
								<p class="text-xs text-gray-600 dark:text-gray-300 line-clamp-2">{msg.body}</p>
							</div>
							<div class="text-right shrink-0">
								<span class="block text-xs font-mono font-medium text-gray-900 dark:text-gray-100">{msg.timestamp}</span>
								<span class="block text-[10px] text-gray-400">{msg.time_ago}</span>
							</div>
						</div>
					</div>
				{/each}
			</div>
		{:else}
			<!-- Table View -->
			<div class="overflow-x-auto max-h-[600px] overflow-y-auto">
				<table class="w-full text-left text-xs">
					<thead class="sticky top-0 bg-gray-50 dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700">
						<tr>
							<th class="px-3 py-2 font-semibold text-gray-600 dark:text-gray-300">Time</th>
							<th class="px-3 py-2 font-semibold text-gray-600 dark:text-gray-300">Status</th>
							<th class="px-3 py-2 font-semibold text-gray-600 dark:text-gray-300">Type</th>
							<th class="px-3 py-2 font-semibold text-gray-600 dark:text-gray-300">Sender</th>
							<th class="px-3 py-2 font-semibold text-gray-600 dark:text-gray-300">Recipient</th>
							<th class="px-3 py-2 font-semibold text-gray-600 dark:text-gray-300">Operator</th>
							<th class="px-3 py-2 font-semibold text-gray-600 dark:text-gray-300">Message</th>
							<th class="px-3 py-2 font-semibold text-gray-600 dark:text-gray-300">Gateway</th>
							<th class="px-3 py-2 font-semibold text-gray-600 dark:text-gray-300">Parts</th>
							<th class="px-3 py-2 font-semibold text-gray-600 dark:text-gray-300">Cost</th>
						</tr>
					</thead>
					<tbody class="divide-y divide-gray-100 dark:divide-gray-800">
						{#each filteredMessages as msg (msg.id)}
							{@const badge = getStatusBadge(msg.status)}
							<tr class="hover:bg-gray-50/70 dark:hover:bg-gray-800/50">
								<td class="px-3 py-2 font-mono text-[11px] text-gray-500 whitespace-nowrap">{msg.timestamp}</td>
								<td class="px-3 py-2 whitespace-nowrap">
									<span class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-bold border {badge.bg}">
										{badge.label}
									</span>
								</td>
								<td class="px-3 py-2 text-gray-600 dark:text-gray-300 whitespace-nowrap">{msg.action_type}</td>
								<td class="px-3 py-2 font-mono text-gray-700 dark:text-gray-300 whitespace-nowrap">{msg.sender_id}</td>
								<td class="px-3 py-2 font-mono text-gray-900 dark:text-gray-100 whitespace-nowrap">{msg.to}</td>
								<td class="px-3 py-2 text-gray-600 dark:text-gray-300 whitespace-nowrap">{msg.operator}</td>
								<td class="px-3 py-2 text-gray-700 dark:text-gray-300 max-w-xs truncate">{msg.body}</td>
								<td class="px-3 py-2 text-gray-500 dark:text-gray-400 whitespace-nowrap">{msg.gateway}</td>
								<td class="px-3 py-2 font-mono text-gray-700 dark:text-gray-300 whitespace-nowrap">{msg.parts}</td>
								<td class="px-3 py-2 font-mono font-semibold text-gray-900 dark:text-gray-100 whitespace-nowrap">BDT {msg.cost.toFixed(2)}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}
	</div>
</div>
