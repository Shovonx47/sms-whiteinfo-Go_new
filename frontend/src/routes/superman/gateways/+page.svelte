<script lang="ts">
	import { onMount } from 'svelte';
	import { page as pageStore } from '$app/stores';
	import { goto } from '$app/navigation';
	import { supermanApi } from '$lib/superman-api.js';
	import ExportDropdown from '$lib/components/common/ExportDropdown.svelte';
	import Pagination from '$lib/components/common/Pagination.svelte';
	import type { ExportColumn } from '$lib/utils/export.js';
	import {
		BADGE_TONES,
		costModeLabel,
		driverLabel,
		fmtInt,
		fmtMoney,
		fmtPct,
		priorityTone,
		type GatewayRow
	} from '$lib/gateway-admin.js';

	let gateways = $state<GatewayRow[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let flash = $state<{ type: 'success' | 'error'; text: string } | null>(null);
	let flashTimer: ReturnType<typeof setTimeout> | undefined;

	let search = $state('');
	let trashed = $state<'' | 'with' | 'only'>('');
	let sortKey = $state('');
	let sortDir = $state<'asc' | 'desc'>('asc');
	let currentPage = $state(1);
	let perPage = $state(10);
	let total = $state(0);

	let showFilters = $state(false);
	let showBulk = $state(false);
	let menuId = $state<number | null>(null);
	let selected = $state<number[]>([]);

	let activeFilterCount = $derived(trashed !== '' ? 1 : 0);
	let allOnPageSelected = $derived(gateways.length > 0 && gateways.every((g) => selected.includes(g.id)));
	let requestSeq = 0;

	function notify(type: 'success' | 'error', text: string) {
		flash = { type, text };
		clearTimeout(flashTimer);
		flashTimer = setTimeout(() => (flash = null), 6000);
	}

	function buildQuery(): URLSearchParams {
		const query = new URLSearchParams();
		if (search.trim()) query.set('search', search.trim());
		if (trashed) query.set('trashed', trashed);
		if (sortKey) {
			query.set('sort', sortKey);
			query.set('dir', sortDir);
		}
		return query;
	}

	async function loadGateways() {
		const seq = ++requestSeq;
		loading = true;
		error = null;
		try {
			const query = buildQuery();
			query.set('page', String(currentPage));
			query.set('per_page', String(perPage));
			const res = await supermanApi.get<{ data: GatewayRow[]; meta: { total: number } }>(
				`/api/v1/superman/gateways?${query.toString()}`
			);
			if (seq !== requestSeq) return;
			gateways = res.data ?? [];
			total = res.meta?.total ?? gateways.length;

			const lastPage = Math.max(1, Math.ceil(total / perPage));
			if (currentPage > lastPage) {
				currentPage = lastPage;
				void loadGateways();
			}
		} catch (err: unknown) {
			if (seq !== requestSeq) return;
			error = err instanceof Error ? err.message : 'Unable to load gateways.';
		} finally {
			if (seq === requestSeq) loading = false;
		}
	}

	function refilter() {
		currentPage = 1;
		selected = [];
		void loadGateways();
	}

	let searchTimer: ReturnType<typeof setTimeout> | undefined;
	function debouncedRefilter() {
		clearTimeout(searchTimer);
		searchTimer = setTimeout(refilter, 350);
	}

	function toggleSort(key: string) {
		if (sortKey === key) {
			sortDir = sortDir === 'asc' ? 'desc' : 'asc';
		} else {
			sortKey = key;
			sortDir = 'asc';
		}
		refilter();
	}

	function toggleSelectAll() {
		if (allOnPageSelected) {
			selected = selected.filter((id) => !gateways.some((g) => g.id === id));
		} else {
			selected = [...new Set([...selected, ...gateways.map((g) => g.id)])];
		}
	}

	function toggleSelected(id: number) {
		selected = selected.includes(id) ? selected.filter((v) => v !== id) : [...selected, id];
	}

	async function bulk(action: 'delete' | 'restore' | 'force_delete') {
		showBulk = false;
		if (selected.length === 0) return;
		const verb = action === 'delete' ? 'Delete' : action === 'restore' ? 'Restore' : 'Permanently delete';
		if (!window.confirm(`${verb} ${selected.length} selected gateway(s)?`)) return;
		try {
			const res = await supermanApi.post<{ message: string }>('/api/v1/superman/gateways/bulk', {
				ids: selected,
				action
			});
			notify('success', res.message);
			selected = [];
			void loadGateways();
		} catch (err: unknown) {
			notify('error', err instanceof Error ? err.message : 'Bulk action failed.');
		}
	}

	async function fetchAllForExport(): Promise<GatewayRow[]> {
		const query = buildQuery();
		query.set('export', '1');
		const res = await supermanApi.get<{ data: GatewayRow[] }>(`/api/v1/superman/gateways?${query.toString()}`);
		return res.data ?? [];
	}

	const exportColumns: ExportColumn<GatewayRow>[] = [
		{ header: 'Gateway', getValue: (g) => g.name },
		{ header: 'Route Code', getValue: (g) => g.route_code ?? '—' },
		{ header: 'OTP Priority', getValue: (g) => `Priority ${g.priority}` },
		{ header: 'OTP Route', getValue: (g) => (g.is_otp_enabled ? 'ENABLED' : 'DISABLED') },
		{ header: 'Status', getValue: (g) => (g.is_enabled ? 'ACTIVE' : 'INACTIVE') },
		{ header: 'RPS', getValue: (g) => g.requests_per_second },
		{ header: 'Cost Mode', getValue: (g) => costModeLabel(g.cost_mode) },
		{ header: 'DLR Support', getValue: (g) => (g.dlr_support ? 'YES' : 'NO') },
		{ header: 'Masking Sent', getValue: (g) => fmtInt(g.metrics.masking_sent) },
		{ header: 'Non-Masking Sent', getValue: (g) => fmtInt(g.metrics.non_masking_sent) },
		{ header: 'Total Sent', getValue: (g) => fmtInt(g.metrics.total_sent) },
		{ header: 'Masking Delivered', getValue: (g) => fmtInt(g.metrics.masking_delivered) },
		{ header: 'Non-Masking Delivered', getValue: (g) => fmtInt(g.metrics.non_masking_delivered) },
		{ header: 'Total Delivered', getValue: (g) => fmtInt(g.metrics.total_delivered) },
		{ header: 'DLR Pending', getValue: (g) => fmtInt(g.metrics.dlr_pending) },
		{ header: 'DLR Pending %', getValue: (g) => fmtPct(g.metrics.dlr_pending_pct) },
		{ header: 'Send Failed', getValue: (g) => fmtInt(g.metrics.send_failed) },
		{ header: 'Send Failed %', getValue: (g) => fmtPct(g.metrics.send_failed_pct) },
		{ header: 'DLR Failed', getValue: (g) => fmtInt(g.metrics.dlr_failed) },
		{ header: 'DLR Failed %', getValue: (g) => fmtPct(g.metrics.dlr_failed_pct) },
		{ header: 'Processing', getValue: (g) => fmtInt(g.metrics.processing) },
		{ header: 'Processing %', getValue: (g) => fmtPct(g.metrics.processing_pct) },
		{ header: 'Masking Delivery %', getValue: (g) => fmtPct(g.metrics.masking_delivery_pct) },
		{ header: 'Non-Masking Delivery %', getValue: (g) => fmtPct(g.metrics.non_masking_delivery_pct) },
		{ header: 'Masking Cost', getValue: (g) => fmtMoney(g.metrics.masking_cost, 4) },
		{ header: 'Non-Masking Cost', getValue: (g) => fmtMoney(g.metrics.non_masking_cost, 4) },
		{ header: 'Operator Cost', getValue: (g) => fmtMoney(g.metrics.operator_cost) },
		{ header: 'Dipping Charge', getValue: (g) => fmtMoney(g.metrics.dipping_charge) },
		{ header: 'Third-Party Dipping Cost', getValue: (g) => fmtMoney(g.metrics.third_party_dipping_cost) },
		{ header: 'Total Cost', getValue: (g) => fmtMoney(g.metrics.total_cost) },
		{ header: 'Provider Balance', getValue: (g) => g.metrics.provider_balance ?? 'N/A' },
		{ header: 'Dipping Balance', getValue: (g) => g.metrics.dipping_balance ?? 'N/A' },
		{ header: 'Revenue', getValue: (g) => fmtMoney(g.metrics.revenue) },
		{ header: 'Profit', getValue: (g) => fmtMoney(g.metrics.profit) },
		{ header: 'Verification', getValue: (g) => g.metrics.verification },
		{ header: 'Driver', getValue: (g) => driverLabel(g.driver) }
	];

	onMount(() => {
		const params = $pageStore.url.searchParams;
		const saved = params.get('saved');
		if (saved) {
			notify(
				'success',
				saved === 'created' ? 'Gateway created.' : saved === 'deleted' ? 'Gateway deleted.' : 'Gateway updated.'
			);
			void goto('/superman/gateways', { replaceState: true, noScroll: true });
		}
		void loadGateways();
	});

	const iconButton =
		'inline-flex items-center gap-2 rounded-lg border bg-white px-3.5 py-2 text-sm font-medium shadow-xs transition hover:bg-gray-50 dark:bg-gray-900 dark:hover:bg-gray-800';
	const selectClass =
		'w-full rounded-lg border border-gray-300 bg-white px-2.5 py-1.5 text-sm text-gray-900 outline-none focus:border-emerald-600 focus:ring-1 focus:ring-emerald-600 dark:border-gray-700 dark:bg-gray-800 dark:text-white';
	const badge = 'inline-flex items-center whitespace-nowrap rounded-md border px-2 py-0.5 text-xs font-medium';
	const th = 'whitespace-nowrap px-4 py-3 uppercase tracking-wide';
	const tdNum = 'whitespace-nowrap px-4 py-3.5 text-right tabular-nums text-gray-800 dark:text-gray-200';

	const colSpan = 38;
</script>

<svelte:head>
	<title>Gateways — SMS Whiteinfo Administrator</title>
</svelte:head>

<svelte:window
	onclick={() => {
		menuId = null;
		showFilters = false;
		showBulk = false;
	}}
/>

{#snippet sortHeader(label: string, key: string)}
	<button type="button" onclick={() => toggleSort(key)} class="inline-flex items-center gap-1 uppercase tracking-wide">
		{label}
		<svg
			class="h-3.5 w-3.5 {sortKey === key ? 'text-emerald-600' : 'text-gray-400'} {sortKey === key && sortDir === 'desc' ? 'rotate-180' : ''}"
			fill="none"
			viewBox="0 0 24 24"
			stroke-width="2"
			stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" /></svg
		>
	</button>
{/snippet}

<div class="space-y-5">
	<div class="flex flex-wrap items-end justify-between gap-3">
		<div>
			<nav class="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400" aria-label="Breadcrumb">
				<span>Gateways</span>
				<span>&rsaquo;</span>
				<span>List</span>
			</nav>
			<h1 class="mt-1 text-3xl font-bold tracking-tight text-gray-950 dark:text-white">Gateways</h1>
		</div>

		<div class="flex flex-wrap items-center gap-2">
			<ExportDropdown
				title="Gateways"
				columns={exportColumns}
				getData={fetchAllForExport}
				disabled={total === 0}
				buttonClass="{iconButton} border-emerald-600 text-emerald-700 disabled:cursor-not-allowed disabled:opacity-50 dark:border-emerald-600 dark:text-emerald-400"
			/>
			<a
				href="/superman/gateways/create"
				data-testid="new-gateway"
				class="{iconButton} border-emerald-600 font-semibold text-emerald-700 dark:text-emerald-400"
			>
				New gateway
			</a>
		</div>
	</div>

	{#if flash}
		<div
			role="status"
			data-testid="flash"
			class="flex items-center justify-between rounded-lg border px-4 py-3 text-sm {flash.type === 'success'
				? 'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-300'
				: 'border-rose-200 bg-rose-50 text-rose-800 dark:border-rose-900 dark:bg-rose-950/30 dark:text-rose-300'}"
		>
			<span>{flash.text}</span>
			<button type="button" onclick={() => (flash = null)} aria-label="Dismiss" class="ml-4 text-lg leading-none opacity-60 hover:opacity-100">×</button>
		</div>
	{/if}

	<div class="rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
		<!-- Toolbar -->
		<div class="flex flex-wrap items-center justify-between gap-3 px-4 py-3">
			<div class="min-h-9">
				{#if selected.length > 0}
					<div class="relative flex items-center gap-3 text-sm" data-testid="bulk-bar">
						<span class="font-medium text-gray-700 dark:text-gray-200">{selected.length} selected</span>
						<button
							type="button"
							onclick={(e) => {
								e.stopPropagation();
								showBulk = !showBulk;
							}}
							class="rounded-lg border border-emerald-600 px-3 py-1.5 text-sm font-medium text-emerald-700 hover:bg-emerald-50 dark:text-emerald-400 dark:hover:bg-emerald-950/30"
						>
							Bulk actions ▾
						</button>
						<button type="button" onclick={() => (selected = [])} class="text-xs font-semibold text-gray-500 hover:text-gray-800">Deselect all</button>
						{#if showBulk}
							<div class="absolute left-24 top-full z-30 mt-1 w-52 rounded-lg border border-gray-200 bg-white p-1 shadow-lg dark:border-gray-800 dark:bg-gray-900">
								<button type="button" onclick={() => void bulk('delete')} class="flex w-full rounded px-3 py-1.5 text-sm text-rose-600 hover:bg-rose-50 dark:hover:bg-rose-950/30">Delete selected</button>
								<button type="button" onclick={() => void bulk('force_delete')} class="flex w-full rounded px-3 py-1.5 text-sm text-rose-700 hover:bg-rose-50 dark:hover:bg-rose-950/30">Force delete selected</button>
								<button type="button" onclick={() => void bulk('restore')} class="flex w-full rounded px-3 py-1.5 text-sm text-emerald-700 hover:bg-emerald-50 dark:hover:bg-emerald-950/30">Restore selected</button>
							</div>
						{/if}
					</div>
				{/if}
			</div>

			<div class="ml-auto flex items-center gap-2">
				<div class="relative w-64 max-w-full">
					<svg class="pointer-events-none absolute left-3 top-2.5 h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" /></svg>
					<input
						type="search"
						bind:value={search}
						oninput={debouncedRefilter}
						placeholder="Search"
						aria-label="Search gateways"
						data-testid="global-search"
						class="w-full rounded-lg border border-gray-300 bg-white py-2 pl-9 pr-3 text-sm text-gray-900 outline-none focus:border-emerald-600 focus:ring-1 focus:ring-emerald-600 dark:border-gray-700 dark:bg-gray-800 dark:text-white"
					/>
				</div>

				<div class="relative">
					<button
						type="button"
						aria-label="Filters"
						aria-expanded={showFilters}
						onclick={(e) => {
							e.stopPropagation();
							showFilters = !showFilters;
						}}
						data-testid="filters-toggle"
						class="relative rounded-lg p-2 text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-800"
					>
						<svg class="h-5 w-5" fill="currentColor" viewBox="0 0 24 24"><path d="M12 3c2.755 0 5.455.232 8.083.678.533.09.917.556.917 1.096v1.044a2.25 2.25 0 0 1-.659 1.591l-5.432 5.432a2.25 2.25 0 0 0-.659 1.591v2.927a2.25 2.25 0 0 1-1.244 2.013L9.75 21v-6.568a2.25 2.25 0 0 0-.659-1.591L3.659 7.409A2.25 2.25 0 0 1 3 5.818V4.774c0-.54.384-1.006.917-1.096A48.32 48.32 0 0 1 12 3Z" /></svg>
						<span
							data-testid="filter-count"
							class="absolute -right-1 -top-1 flex h-4 min-w-4 items-center justify-center rounded-full px-1 text-[10px] font-bold {activeFilterCount > 0 ? 'bg-emerald-600 text-white' : 'border border-gray-300 bg-white text-gray-500 dark:border-gray-600 dark:bg-gray-900'}"
						>{activeFilterCount}</span>
					</button>
					{#if showFilters}
						<div
							class="absolute right-0 z-30 mt-2 w-64 space-y-3 rounded-xl border border-gray-200 bg-white p-4 shadow-xl dark:border-gray-700 dark:bg-gray-900"
							role="dialog"
							aria-label="Filters"
							tabindex="-1"
							onclick={(e) => e.stopPropagation()}
							onkeydown={(e) => e.stopPropagation()}
						>
							<div class="flex items-center justify-between">
								<h3 class="text-sm font-semibold text-gray-900 dark:text-white">Filters</h3>
								<button
									type="button"
									onclick={() => {
										trashed = '';
										refilter();
									}}
									class="text-xs font-semibold text-emerald-700 hover:underline dark:text-emerald-400">Reset</button
								>
							</div>
							<label class="block text-xs font-medium text-gray-600 dark:text-gray-300"
								>Trashed
								<select bind:value={trashed} onchange={refilter} class="{selectClass} mt-1">
									<option value="">Without trashed records</option>
									<option value="with">With trashed records</option>
									<option value="only">Only trashed records</option>
								</select>
							</label>
						</div>
					{/if}
				</div>
			</div>
		</div>

		{#if error}
			<div role="alert" class="mx-4 mb-3 rounded-lg border border-rose-200 bg-rose-50 p-3 text-sm text-rose-700 dark:border-rose-900 dark:bg-rose-950/30 dark:text-rose-300">{error}</div>
		{/if}

		<div class="overflow-x-auto border-t border-gray-200 dark:border-gray-800">
			<table class="w-full text-left text-sm" data-testid="gateways-table">
				<thead>
					<tr class="bg-gray-50/70 text-xs font-semibold text-gray-700 dark:bg-gray-800/40 dark:text-gray-200">
						<th class="w-12 px-4 py-3">
							<input type="checkbox" checked={allOnPageSelected} onchange={toggleSelectAll} aria-label="Select all on this page" class="h-4 w-4 rounded border-gray-300 accent-emerald-600" />
						</th>
						<th class={th}>{@render sortHeader('Gateway', 'name')}</th>
						<th class={th}>Route Code</th>
						<th class={th}>{@render sortHeader('OTP Priority', 'priority')}</th>
						<th class={th}>OTP Route</th>
						<th class={th}>Status</th>
						<th class={th}>{@render sortHeader('RPS', 'requests_per_second')}</th>
						<th class={th}>Cost Mode</th>
						<th class={th}>DLR Support</th>
						<th class={th}>Masking Sent</th>
						<th class={th}>Non-Masking Sent</th>
						<th class={th}>Total Sent</th>
						<th class={th}>Masking Delivered</th>
						<th class={th}>Non-Masking Delivered</th>
						<th class={th}>Total Delivered</th>
						<th class={th}>DLR Pending</th>
						<th class={th}>DLR Pending %</th>
						<th class={th}>Send Failed</th>
						<th class={th}>Send Failed %</th>
						<th class={th}>DLR Failed</th>
						<th class={th}>DLR Failed %</th>
						<th class={th}>Processing</th>
						<th class={th}>Processing %</th>
						<th class={th}>Masking Delivery %</th>
						<th class={th}>Non-Masking Delivery %</th>
						<th class={th}>Masking Cost</th>
						<th class={th}>Non-Masking Cost</th>
						<th class={th}>Operator Cost</th>
						<th class={th}>Dipping Charge</th>
						<th class={th}>Third-Party Dipping Cost</th>
						<th class={th}>Total Cost</th>
						<th class={th}>Provider Balance</th>
						<th class={th}>Dipping Balance</th>
						<th class={th}>Revenue</th>
						<th class={th}>Profit</th>
						<th class={th}>Verification</th>
						<th class={th}>Driver</th>
						<th class="w-14 px-4 py-3"><span class="sr-only">Actions</span></th>
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-100 dark:divide-gray-800 {loading && gateways.length > 0 ? 'opacity-60' : ''}">
					{#each gateways as g (g.id)}
						{@const m = g.metrics}
						<tr
							onclick={(e) => {
								const el = e.target as HTMLElement;
								if (el.closest('button,a,input,label')) return;
								void goto(`/superman/gateways/${g.id}`);
							}}
							class="cursor-pointer transition-colors hover:bg-gray-50/70 dark:hover:bg-gray-800/30 {selected.includes(g.id) ? 'bg-emerald-50/40 dark:bg-emerald-950/10' : ''} {g.deleted_at ? 'opacity-60' : ''}"
						>
							<td class="px-4 py-3.5">
								<input type="checkbox" checked={selected.includes(g.id)} onchange={() => toggleSelected(g.id)} aria-label="Select {g.name}" class="h-4 w-4 rounded border-gray-300 accent-emerald-600" />
							</td>
							<td class="whitespace-nowrap px-4 py-3.5 font-bold text-gray-900 dark:text-white">{g.name}</td>
							<td class="whitespace-nowrap px-4 py-3.5">
								{#if g.route_code}
									<span class="{badge} {BADGE_TONES.info}">{g.route_code}</span>
								{:else}
									<span class="text-gray-400">—</span>
								{/if}
							</td>
							<td class="whitespace-nowrap px-4 py-3.5"><span class="{badge} {BADGE_TONES[priorityTone(g.priority)]}">Priority {g.priority}</span></td>
							<td class="whitespace-nowrap px-4 py-3.5"><span class="{badge} {g.is_otp_enabled ? BADGE_TONES.success : BADGE_TONES.gray}">{g.is_otp_enabled ? 'ENABLED' : 'DISABLED'}</span></td>
							<td class="whitespace-nowrap px-4 py-3.5"><span class="{badge} {g.is_enabled ? BADGE_TONES.success : BADGE_TONES.danger}">{g.is_enabled ? 'ACTIVE' : 'INACTIVE'}</span></td>
							<td class="whitespace-nowrap px-4 py-3.5 text-center tabular-nums text-gray-800 dark:text-gray-200">{g.requests_per_second}</td>
							<td class="whitespace-nowrap px-4 py-3.5"><span class="{badge} {BADGE_TONES.gray}">{costModeLabel(g.cost_mode)}</span></td>
							<td class="whitespace-nowrap px-4 py-3.5 text-center"><span class="{badge} {g.dlr_support ? BADGE_TONES.success : BADGE_TONES.gray}">{g.dlr_support ? 'YES' : 'NO'}</span></td>
							<td class={tdNum}>{fmtInt(m.masking_sent)}</td>
							<td class={tdNum}>{fmtInt(m.non_masking_sent)}</td>
							<td class="{tdNum} font-bold">{fmtInt(m.total_sent)}</td>
							<td class={tdNum}>{fmtInt(m.masking_delivered)}</td>
							<td class={tdNum}>{fmtInt(m.non_masking_delivered)}</td>
							<td class="{tdNum} font-bold">{fmtInt(m.total_delivered)}</td>
							<td class={tdNum}>{fmtInt(m.dlr_pending)}</td>
							<td class={tdNum}>{fmtPct(m.dlr_pending_pct)}</td>
							<td class={tdNum}>{fmtInt(m.send_failed)}</td>
							<td class={tdNum}>{fmtPct(m.send_failed_pct)}</td>
							<td class={tdNum}>{fmtInt(m.dlr_failed)}</td>
							<td class={tdNum}>{fmtPct(m.dlr_failed_pct)}</td>
							<td class={tdNum}>{fmtInt(m.processing)}</td>
							<td class={tdNum}>{fmtPct(m.processing_pct)}</td>
							<td class={tdNum}>{fmtPct(m.masking_delivery_pct)}</td>
							<td class={tdNum}>{fmtPct(m.non_masking_delivery_pct)}</td>
							<td class={tdNum}>{fmtMoney(m.masking_cost, 4)}</td>
							<td class={tdNum}>{fmtMoney(m.non_masking_cost, 4)}</td>
							<td class="whitespace-nowrap px-4 py-3.5 text-right"><span class="{badge} {BADGE_TONES.warning}">{fmtMoney(m.operator_cost)}</span></td>
							<td class="whitespace-nowrap px-4 py-3.5 text-right"><span class="{badge} {BADGE_TONES.info}">{fmtMoney(m.dipping_charge)}</span></td>
							<td class={tdNum}>{fmtMoney(m.third_party_dipping_cost)}</td>
							<td class="whitespace-nowrap px-4 py-3.5 text-right"><span class="{badge} {BADGE_TONES.danger} font-bold">{fmtMoney(m.total_cost)}</span></td>
							<td class={tdNum}>{m.provider_balance ?? 'N/A'}</td>
							<td class={tdNum}>{m.dipping_balance ?? 'N/A'}</td>
							<td class="whitespace-nowrap px-4 py-3.5 text-right"><span class="{badge} {BADGE_TONES.success} font-bold">{fmtMoney(m.revenue)}</span></td>
							<td class="whitespace-nowrap px-4 py-3.5 text-right"><span class="{badge} {m.profit >= 0 ? BADGE_TONES.success : BADGE_TONES.danger} font-bold">{fmtMoney(m.profit)}</span></td>
							<td class="whitespace-nowrap px-4 py-3.5"><span class="{badge} {BADGE_TONES[m.verification_color] ?? BADGE_TONES.gray}">{m.verification}</span></td>
							<td class="whitespace-nowrap px-4 py-3.5"><span class="{badge} {BADGE_TONES.success}">{driverLabel(g.driver)}</span></td>
							<td class="relative px-4 py-3.5 text-right">
								<button
									type="button"
									aria-label="Actions for {g.name}"
									onclick={(e) => {
										e.stopPropagation();
										menuId = menuId === g.id ? null : g.id;
									}}
									class="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-emerald-600/60 text-emerald-700 hover:bg-emerald-50 dark:border-emerald-700 dark:text-emerald-400 dark:hover:bg-emerald-950/30"
								>
									<svg class="h-4 w-4" fill="currentColor" viewBox="0 0 24 24"><circle cx="12" cy="5" r="1.6" /><circle cx="12" cy="12" r="1.6" /><circle cx="12" cy="19" r="1.6" /></svg>
								</button>
								{#if menuId === g.id}
									<div class="absolute right-4 top-full z-20 w-36 rounded-lg border border-gray-200 bg-white p-1 text-left shadow-lg dark:border-gray-800 dark:bg-gray-900">
										<a href="/superman/gateways/{g.id}" class="flex w-full rounded px-3 py-1.5 text-sm text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-gray-800">View</a>
										<a href="/superman/gateways/{g.id}/edit" class="flex w-full rounded px-3 py-1.5 text-sm text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-gray-800">Edit</a>
									</div>
								{/if}
							</td>
						</tr>
					{:else}
						<tr>
							<td colspan={colSpan} class="px-4 py-14 text-center text-sm text-gray-400">
								{loading ? 'Loading gateways…' : search || activeFilterCount > 0 ? 'No gateways match your filters.' : 'No gateways yet.'}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		<Pagination
			{total}
			page={currentPage}
			{perPage}
			perPageOptions={[10, 25, 50]}
			{loading}
			onPageChange={(p) => {
				currentPage = p;
				void loadGateways();
			}}
			onPerPageChange={(size) => {
				perPage = size;
				refilter();
			}}
		/>
	</div>
</div>
