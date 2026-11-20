<script module lang="ts">
	export interface LegacyColumn {
		key: string;
		label: string;
		format?: 'text' | 'mono' | 'number' | 'currency' | 'date' | 'datetime' | 'badge' | 'truncate';
		align?: 'left' | 'right' | 'center';
		hidden?: boolean;
	}

	export interface LegacyAction {
		label: string;
		tone?: 'default' | 'warning' | 'danger' | 'success';
		run: (row: Record<string, any>) => void | Promise<void>;
	}
</script>

<script lang="ts">
	import { onMount } from 'svelte';
	import { supermanApi } from '$lib/superman-api.js';
	import ExportDropdown from '$lib/components/common/ExportDropdown.svelte';
	import type { ExportColumn } from '$lib/utils/export.js';

	interface Props {
		title: string;
		description?: string;
		endpoint: string;
		columns: LegacyColumn[];
		searchable?: boolean;
		statusOptions?: Array<{ value: string; label: string }>;
		defaultStatus?: string;
		emptyText?: string;
		actions?: LegacyAction[];
		showCreatedAt?: boolean;
	}

	let {
		title,
		description = '',
		endpoint,
		columns,
		searchable = true,
		statusOptions = [],
		defaultStatus = '',
		emptyText = 'No records found.',
		actions = [],
		showCreatedAt = false
	}: Props = $props();

	let rows = $state<Record<string, any>[]>([]);
	let loading = $state(true);
	let search = $state('');
	let status = $state(defaultStatus);
	let page = $state(1);
	let perPage = $state(10);
	let total = $state(0);
	let activeMenu = $state<number | null>(null);
	let toast = $state<string | null>(null);
	let searchTimer: ReturnType<typeof setTimeout> | null = null;
	let toastTimer: ReturnType<typeof setTimeout> | null = null;

	const visibleColumns = $derived(columns.filter((column) => !column.hidden || showCreatedAt));

	function notify(message: string) {
		toast = message;
		if (toastTimer) clearTimeout(toastTimer);
		toastTimer = setTimeout(() => (toast = null), 3000);
	}

	// Merge the endpoint's own base filters (for example
	// `?statuses=failed,dlr_failed`) with the toolbar filters instead of
	// appending a second `statuses` parameter, which the API ignored.
	function queryString(forExport = false) {
		const [path, existing = ''] = endpoint.split('?');
		const params = new URLSearchParams(existing);
		params.delete('page');
		params.delete('per_page');
		params.delete('export');
		if (status) params.set('statuses', status);
		if (search.trim()) params.set('search', search.trim());
		params.set('page', forExport ? '1' : String(page));
		params.set('per_page', forExport ? '20000' : String(perPage));
		if (forExport) params.set('export', '1');
		return `${path}?${params.toString()}`;
	}

	async function load(initial = false) {
		if (initial) loading = true;
		try {
			const response = await supermanApi.get<{ data: Record<string, any>[]; meta?: { total: number } }>(queryString());
			rows = response.data ?? [];
			total = response.meta?.total ?? rows.length;
		} catch {
			rows = [];
			total = 0;
		} finally {
			loading = false;
		}
	}

	function onSearchInput() {
		if (searchTimer) clearTimeout(searchTimer);
		searchTimer = setTimeout(() => {
			page = 1;
			void load();
		}, 300);
	}

	function format(row: Record<string, any>, column: LegacyColumn): string {
		const value = row[column.key];
		if (value === null || value === undefined || value === '') return '—';
		switch (column.format) {
			case 'currency':
				return `৳${Number(value).toFixed(4)}`;
			case 'number':
				return String(Number(value));
			case 'date':
				return new Date(String(value)).toLocaleDateString();
			case 'datetime':
				return new Date(String(value)).toLocaleString();
			default:
				return String(value);
		}
	}

	function badgeClass(value: string): string {
		const normalized = value.toLowerCase();
		if (['delivered', 'verified', 'approved', 'sent', 'success', 'active', 'completed'].includes(normalized)) {
			return 'border-emerald-300 bg-emerald-50 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-300';
		}
		if (['failed', 'dlr_failed', 'rejected', 'error', 'inactive', 'suspended'].includes(normalized)) {
			return 'border-rose-300 bg-rose-50 text-rose-700 dark:border-rose-800 dark:bg-rose-950/40 dark:text-rose-300';
		}
		if (['pending', 'sending', 'scheduled', 'paused', 'processing', 'under_review'].includes(normalized)) {
			return 'border-amber-300 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-300';
		}
		return 'border-gray-300 bg-gray-50 text-gray-700 dark:border-gray-700 dark:bg-gray-800/60 dark:text-gray-300';
	}

	async function runAction(action: LegacyAction, row: Record<string, any>) {
		activeMenu = null;
		try {
			await action.run(row);
		} catch {
			notify('The action could not be completed. Please try again.');
		}
	}

	const totalPages = $derived(Math.max(1, Math.ceil(total / perPage)));
	const exportColumns = $derived<ExportColumn<Record<string, any>>[]>(
		visibleColumns.map((column) => ({
			header: column.label,
			getValue: (row) => format(row, column)
		}))
	);

	// Export every row that matches the current filters, not just the page
	// that happens to be on screen.
	async function exportData() {
		try {
			const response = await supermanApi.get<{ data: Record<string, any>[] }>(queryString(true));
			return response.data ?? [];
		} catch {
			return [];
		}
	}

	function changePerPage(event: Event) {
		perPage = Number((event.target as HTMLSelectElement).value);
		page = 1;
		void load();
	}

	function goToPage(next: number) {
		if (next < 1 || next > totalPages || next === page) return;
		page = next;
		void load();
	}

	onMount(() => {
		void load(true);
	});
</script>

<svelte:window onclick={() => (activeMenu = null)} />

<div class="space-y-4">
	<div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
		<div>
			<h1 class="text-3xl font-bold tracking-tight text-gray-950 dark:text-white">{title}</h1>
			{#if description}
				<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{description}</p>
			{/if}
		</div>
		<ExportDropdown title={title} columns={exportColumns} getData={exportData} disabled={rows.length === 0} />
	</div>

	<div class="rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
		<div class="flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between">
			<div class="text-xs text-gray-500">
				{#if status}
					Filtered by <span class="font-semibold text-gray-800 dark:text-gray-200">{status}</span>
					<button type="button" onclick={() => { status = ''; page = 1; void load(); }} class="ml-2 font-semibold text-emerald-600 hover:underline">Clear</button>
				{/if}
			</div>

			<div class="flex items-center justify-end gap-2.5">
				{#if searchable}
					<div class="relative w-full max-w-xs">
						<svg class="absolute left-3 top-2.5 h-3.5 w-3.5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
						</svg>
						<input
							type="search"
							bind:value={search}
							oninput={onSearchInput}
							placeholder="Search"
							class="w-full rounded-lg border border-gray-200 bg-white py-1.5 pl-8 pr-3 text-xs outline-none focus:border-emerald-500 dark:border-gray-700 dark:bg-gray-800 dark:text-white"
						/>
					</div>
				{/if}

				{#if statusOptions.length > 0}
					<select
						value={status}
						onchange={(event) => { status = (event.target as HTMLSelectElement).value; page = 1; void load(); }}
						class="rounded-lg border border-gray-200 bg-white py-1.5 px-2 text-xs text-gray-700 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300"
					>
						<option value="">All statuses</option>
						{#each statusOptions as option (option.value)}
							<option value={option.value}>{option.label}</option>
						{/each}
					</select>
				{/if}
			</div>
		</div>

		<div class="overflow-x-auto">
			<table class="w-full text-left text-xs">
				<thead>
					<tr class="border-y border-gray-100 text-[11px] font-bold uppercase text-gray-700 dark:border-gray-800 dark:text-gray-300">
						{#each visibleColumns as column (column.key)}
							<th class="px-4 py-3.5 {column.align === 'right' ? 'text-right' : column.align === 'center' ? 'text-center' : ''}">{column.label}</th>
						{/each}
						{#if actions.length > 0}
							<th class="w-16 px-4 py-3.5"></th>
						{/if}
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-100 dark:divide-gray-800">
					{#if loading}
						<tr>
							<td colspan={visibleColumns.length + (actions.length > 0 ? 1 : 0)} class="px-4 py-12 text-center text-xs text-gray-400">Loading…</td>
						</tr>
					{:else if rows.length === 0}
						<tr>
							<td colspan={visibleColumns.length + (actions.length > 0 ? 1 : 0)} class="px-4 py-12 text-center text-xs text-gray-400">{emptyText}</td>
						</tr>
					{:else}
						{#each rows as row, index (row.id ?? index)}
							<tr class="transition-colors hover:bg-gray-50/70 dark:hover:bg-gray-800/40">
								{#each visibleColumns as column (column.key)}
									<td class="px-4 py-4 {column.align === 'right' ? 'text-right' : column.align === 'center' ? 'text-center' : ''} {column.format === 'mono' ? 'font-mono' : ''} text-gray-700 dark:text-gray-300">
										{#if column.format === 'badge'}
											<span class="inline-flex items-center rounded-full border px-2 py-0.5 text-[10px] font-semibold capitalize {badgeClass(String(row[column.key] ?? ''))}">
												{String(row[column.key] ?? '—').replaceAll('_', ' ')}
											</span>
										{:else if column.format === 'truncate'}
											<span class="block max-w-xs truncate" title={String(row[column.key] ?? '')}>{format(row, column)}</span>
										{:else}
											{format(row, column)}
										{/if}
									</td>
								{/each}
								{#if actions.length > 0}
									<td class="relative px-4 py-4 text-right">
										<button
											type="button"
											onclick={(event) => { event.stopPropagation(); activeMenu = activeMenu === row.id ? null : row.id; }}
											class="rounded-lg border border-gray-200 p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:border-gray-700 dark:hover:bg-gray-800 dark:hover:text-gray-200"
											aria-label="Row actions"
										>
											<svg class="h-4 w-4" fill="currentColor" viewBox="0 0 24 24">
												<path d="M12 8c1.1 0 2-.9 2-2s-.9-2-2-2-2 .9-2 2 .9 2 2 2zm0 2c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm0 6c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2z" />
											</svg>
										</button>
										{#if activeMenu === row.id}
											<div class="absolute right-4 top-12 z-30 w-48 rounded-xl border border-gray-100 bg-white p-1 text-left shadow-xl dark:border-gray-800 dark:bg-gray-900">
												{#each actions as action (action.label)}
													<button
														type="button"
														onclick={() => void runAction(action, row)}
														class="flex w-full items-center gap-2 rounded-lg px-2.5 py-1.5 text-xs hover:bg-gray-50 dark:hover:bg-gray-800 {action.tone === 'danger' ? 'text-rose-600 dark:text-rose-400' : action.tone === 'warning' ? 'text-amber-600 dark:text-amber-400' : action.tone === 'success' ? 'text-emerald-600 dark:text-emerald-400' : 'text-gray-700 dark:text-gray-200'}"
													>
														{action.label}
													</button>
												{/each}
											</div>
										{/if}
									</td>
								{/if}
							</tr>
						{/each}
					{/if}
				</tbody>
			</table>
		</div>

		<div class="flex flex-col gap-3 border-t border-gray-100 p-4 text-xs text-gray-500 sm:flex-row sm:items-center sm:justify-between dark:border-gray-800">
			<div>Showing {total === 0 ? 0 : (page - 1) * perPage + 1} to {Math.min(page * perPage, total)} of {total} results</div>
			<div class="flex items-center gap-4">
				<div class="flex items-center gap-2">
					<span>Per page</span>
					<select value={perPage} onchange={changePerPage} class="rounded-lg border border-gray-200 bg-white py-1 px-2 text-xs text-gray-700 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300">
						<option value={10}>10</option>
						<option value={25}>25</option>
						<option value={50}>50</option>
						<option value={100}>100</option>
					</select>
				</div>
				<div class="flex items-center gap-1">
					{#each Array.from({ length: totalPages }, (_, i) => i + 1) as p (p)}
						<button
							type="button"
							onclick={() => goToPage(p)}
							class="h-7 w-7 rounded-lg border text-xs font-semibold {p === page ? 'border-emerald-600 bg-emerald-600 text-white' : 'border-gray-200 bg-white text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300'}"
						>
							{p}
						</button>
					{/each}
				</div>
			</div>
		</div>
	</div>
</div>

{#if toast}
	<div class="fixed bottom-5 right-5 z-50 rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-2.5 text-xs font-semibold text-emerald-800 shadow-lg dark:border-emerald-800 dark:bg-emerald-950/80 dark:text-emerald-200">
		{toast}
	</div>
{/if}
