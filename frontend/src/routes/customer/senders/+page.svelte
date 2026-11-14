<script lang="ts">
	import { api } from '$lib/api.js';
	import { onMount } from 'svelte';

	interface Sender {
		id: number;
		sender_id: string;
		type: string;
		status?: string;
		created_at: string;
	}

	interface Meta {
		total: number;
		per_page: number;
		current_page: number;
		last_page?: number;
	}

	let senders = $state<Sender[]>([]);
	let meta = $state<Meta>({ total: 0, per_page: 10, current_page: 1, last_page: 1 });
	let loading = $state(true);
	let search = $state('');
	let perPage = $state(10);
	let currentPage = $state(1);

	async function load(p = 1) {
		loading = true;
		currentPage = p;
		try {
			const params = new URLSearchParams({ page: String(p), per_page: String(perPage) });
			if (search.trim()) params.set('search', search.trim());
			const res = await api.get<{ data: Sender[]; meta: Meta }>(`/api/v1/customer/senders?${params}`);
			senders = res.data ?? [];
			meta = res.meta ?? { total: senders.length, per_page: perPage, current_page: p, last_page: 1 };
		} catch {
			senders = [];
			meta = { total: 0, per_page: perPage, current_page: 1, last_page: 1 };
		} finally {
			loading = false;
		}
	}

	onMount(() => load(1));

	let searchTimer: ReturnType<typeof setTimeout>;
	function handleSearchInput() {
		clearTimeout(searchTimer);
		searchTimer = setTimeout(() => load(1), 300);
	}

	function formatSenderType(type?: string): { label: string; masking: boolean } {
		const lower = (type ?? '').toLowerCase().trim();
		switch (lower) {
			case 'alphanumeric':
			case 'masking':
				return { label: 'Alphanumeric (Masking)', masking: true };
			case 'shortcode':
				return { label: 'Shortcode', masking: true };
			case 'numeric':
			case 'non_masking':
			case 'non-masking':
			case 'number':
			default:
				return { label: 'Numeric (Non-Masking)', masking: false };
		}
	}

	let totalPages = $derived(Math.max(1, Math.ceil((meta.total || 0) / perPage)));
</script>

<svelte:head>
	<title>Sender ID List - White INFO</title>
</svelte:head>

<div class="space-y-6">
	<!-- Breadcrumb -->
	<nav class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
		<a href="/messaging/senders" class="hover:text-gray-800 dark:hover:text-gray-200 transition">Sender ID List</a>
		<svg class="h-3 w-3 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
			<path stroke-linecap="round" stroke-linejoin="round" d="M8.25 4.5l7.5 7.5-7.5 7.5" />
		</svg>
		<span class="text-gray-900 dark:text-gray-100 font-medium">List</span>
	</nav>

	<!-- Header -->
	<div>
		<h1 class="text-2xl font-bold tracking-tight text-gray-900 dark:text-gray-100">Sender ID List</h1>
	</div>

	<!-- Main Table Card -->
	<div class="rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900 overflow-hidden">
		<!-- Search Toolbar -->
		<div class="flex items-center justify-end gap-2 p-3.5 border-b border-gray-100 dark:border-gray-800">
			<div class="relative w-64">
				<input
					type="text"
					bind:value={search}
					oninput={handleSearchInput}
					placeholder="Search Sender ID..."
					class="w-full rounded-lg border border-gray-200 bg-white py-1.5 pl-8 pr-3 text-xs text-gray-800 placeholder-gray-400 focus:border-teal-500 focus:outline-hidden dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200"
				/>
				<svg class="h-3.5 w-3.5 text-gray-400 absolute left-2.5 top-2.5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
				</svg>
			</div>
		</div>

		<!-- Table -->
		<div class="overflow-x-auto">
			<table class="w-full text-left text-xs">
				<thead class="bg-gray-50 dark:bg-gray-800/80 border-b border-gray-200 dark:border-gray-700 text-gray-700 dark:text-gray-300 font-semibold">
					<tr>
						<th class="px-4 py-3">Sender ID</th>
						<th class="px-4 py-3">Type</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-100 dark:divide-gray-800">
					{#if loading}
						<tr>
							<td colspan="2" class="px-4 py-8 text-center text-gray-400">
								<div class="flex items-center justify-center gap-2">
									<div class="h-4 w-4 animate-spin rounded-full border-2 border-teal-600 border-t-transparent"></div>
									<span>Loading Sender IDs…</span>
								</div>
							</td>
						</tr>
					{:else if senders.length === 0}
						<tr>
							<td colspan="2" class="px-4 py-12 text-center text-gray-500 dark:text-gray-400">
								No Sender IDs found.
							</td>
						</tr>
					{:else}
						{#each senders as s (s.id)}
							{@const st = formatSenderType(s.type)}
							<tr class="hover:bg-gray-50/70 dark:hover:bg-gray-800/50 transition">
								<td class="px-4 py-3 font-mono font-semibold text-gray-900 dark:text-gray-100">
									{s.sender_id}
								</td>
								<td class="px-4 py-3">
									<span class="inline-flex items-center rounded-md px-2.5 py-0.5 text-[11px] font-semibold border {st.masking ? 'bg-purple-50 text-purple-700 border-purple-200 dark:bg-purple-950/40 dark:text-purple-300 dark:border-purple-800' : 'bg-sky-50 text-sky-700 border-sky-200 dark:bg-sky-950/40 dark:text-sky-300 dark:border-sky-800'}">
										{st.label}
									</span>
								</td>
							</tr>
						{/each}
					{/if}
				</tbody>
			</table>
		</div>

		<!-- Footer Pagination Toolbar -->
		<div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 p-3.5 border-t border-gray-100 dark:border-gray-800 text-xs text-gray-600 dark:text-gray-400">
			<div>
				{#if meta.total === 1}
					Showing 1 result
				{:else if meta.total > 1}
					Showing {((currentPage - 1) * perPage) + 1} to {Math.min(currentPage * perPage, meta.total)} of {meta.total} results
				{:else}
					No results
				{/if}
			</div>

			<div class="flex items-center gap-3">
				<div class="flex items-center gap-1.5">
					<span>Per page</span>
					<select
						bind:value={perPage}
						onchange={() => load(1)}
						class="rounded-md border border-gray-300 bg-white py-1 px-2 text-xs text-gray-800 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200"
					>
						<option value={10}>10</option>
						<option value={25}>25</option>
						<option value={50}>50</option>
					</select>
				</div>
				{#if totalPages > 1}
					<div class="flex items-center gap-1">
						<button
							type="button"
							disabled={currentPage <= 1}
							onclick={() => load(currentPage - 1)}
							class="rounded border border-gray-200 px-2 py-1 text-xs disabled:opacity-40 cursor-pointer"
						>
							Prev
						</button>
						<span class="px-2 text-xs">{currentPage} / {totalPages}</span>
						<button
							type="button"
							disabled={currentPage >= totalPages}
							onclick={() => load(currentPage + 1)}
							class="rounded border border-gray-200 px-2 py-1 text-xs disabled:opacity-40 cursor-pointer"
						>
							Next
						</button>
					</div>
				{/if}
			</div>
		</div>
	</div>
</div>
