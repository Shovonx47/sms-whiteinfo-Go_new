<script lang="ts">
	import { onMount } from 'svelte';
	import { page as pageStore } from '$app/stores';
	import { goto } from '$app/navigation';
	import { supermanApi } from '$lib/superman-api.js';
	import ExportDropdown from '$lib/components/common/ExportDropdown.svelte';
	import Pagination from '$lib/components/common/Pagination.svelte';
	import ImportSendersPanel from '$lib/components/sender/ImportSendersPanel.svelte';
	import type { ExportColumn } from '$lib/utils/export.js';
	import {
		OPERATORS,
		isMaskingType,
		loadSenderOptions,
		routesSummary,
		typeLabel,
		type AdminSender,
		type SenderCustomer,
		type SenderGateway
	} from '$lib/sender-admin.js';

	type ColumnKey = 'type' | 'approval' | 'routes' | 'customers' | 'enabled' | 'created';

	const COLUMN_LABELS: Record<ColumnKey, string> = {
		type: 'Type',
		approval: 'Approval',
		routes: 'Gateways / Routes',
		customers: 'Customer',
		enabled: 'Enabled',
		created: 'Created'
	};

	let senders = $state<AdminSender[]>([]);
	let gateways = $state<SenderGateway[]>([]);
	let customers = $state<SenderCustomer[]>([]);

	let loading = $state(true);
	let error = $state<string | null>(null);
	let flash = $state<{ type: 'success' | 'error'; text: string } | null>(null);
	let flashTimer: ReturnType<typeof setTimeout> | undefined;

	// Query state
	let search = $state('');
	let senderIdSearch = $state('');
	let filterType = $state('');
	let filterApproval = $state('');
	let filterEnabled = $state('');
	let filterGateway = $state('');
	let filterAssignment = $state('');
	let sortKey = $state('');
	let sortDir = $state<'asc' | 'desc'>('asc');
	let currentPage = $state(1);
	let perPage = $state(10);
	let total = $state(0);

	// UI state
	let showFilters = $state(false);
	let showColumns = $state(false);
	let showImport = $state(false);
	let menuId = $state<number | null>(null);
	let selected = $state<number[]>([]);
	let approveTarget = $state<AdminSender | null>(null);
	let approveGatewayId = $state('');
	let approveRoutes = $state<Record<string, string>>({});
	let approveSaving = $state(false);
	let approveError = $state<string | null>(null);
	let visible = $state<Record<ColumnKey, boolean>>({
		type: true,
		approval: true,
		routes: true,
		customers: true,
		enabled: true,
		created: false
	});

	let activeFilterCount = $derived(
		[filterType, filterApproval, filterEnabled, filterGateway, filterAssignment].filter((v) => v !== '').length
	);
	let allOnPageSelected = $derived(senders.length > 0 && senders.every((s) => selected.includes(s.id)));
	let colSpan = $derived(
		3 + (Object.values(visible).filter(Boolean).length) // checkbox + sender id + actions
	);

	let requestSeq = 0;

	function notify(type: 'success' | 'error', text: string) {
		flash = { type, text };
		clearTimeout(flashTimer);
		flashTimer = setTimeout(() => (flash = null), 6000);
	}

	function buildQuery(): URLSearchParams {
		const query = new URLSearchParams();
		if (search.trim()) query.set('search', search.trim());
		if (senderIdSearch.trim()) query.set('sender_id', senderIdSearch.trim());
		if (filterType) query.set('type', filterType);
		if (filterApproval) query.set('status', filterApproval);
		if (filterEnabled) query.set('enabled', filterEnabled);
		if (filterGateway) query.set('gateway_id', filterGateway);
		if (filterAssignment) query.set('assignment', filterAssignment);
		if (sortKey) {
			query.set('sort', sortKey);
			query.set('dir', sortDir);
		}
		return query;
	}

	async function loadSenders() {
		const seq = ++requestSeq;
		loading = true;
		error = null;
		try {
			const query = buildQuery();
			query.set('page', String(currentPage));
			query.set('per_page', String(perPage));
			const res = await supermanApi.get<{ data: AdminSender[]; meta: { total: number } }>(
				`/api/v1/superman/senders?${query.toString()}`
			);
			if (seq !== requestSeq) return;
			senders = res.data ?? [];
			total = res.meta?.total ?? 0;

			// If the requested page no longer exists (e.g. after deleting the last rows) step back.
			const lastPage = Math.max(1, Math.ceil(total / perPage));
			if (currentPage > lastPage) {
				currentPage = lastPage;
				void loadSenders();
			}
		} catch (err: unknown) {
			if (seq !== requestSeq) return;
			error = err instanceof Error ? err.message : 'Unable to load senders.';
		} finally {
			if (seq === requestSeq) loading = false;
		}
	}

	function refilter() {
		currentPage = 1;
		selected = [];
		void loadSenders();
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

	function resetFilters() {
		filterType = filterApproval = filterEnabled = filterGateway = filterAssignment = '';
		refilter();
	}

	function toggleSelectAll() {
		if (allOnPageSelected) {
			selected = selected.filter((id) => !senders.some((s) => s.id === id));
		} else {
			selected = [...new Set([...selected, ...senders.map((s) => s.id)])];
		}
	}

	function toggleSelected(id: number) {
		selected = selected.includes(id) ? selected.filter((v) => v !== id) : [...selected, id];
	}

	async function toggleEnabled(sender: AdminSender) {
		const target = !sender.is_enabled;
		try {
			await supermanApi.patch(`/api/v1/superman/senders/${sender.id}/enabled`, { is_enabled: target });
			sender.is_enabled = target;
			notify('success', `${sender.sender_id} ${target ? 'enabled' : 'disabled'}.`);
		} catch (err: unknown) {
			notify('error', err instanceof Error ? err.message : 'Unable to update the sender.');
		}
	}

	async function approve(sender: AdminSender) {
		menuId = null;
		const needsGateway = !sender.gateway_id || (isMaskingType(sender.type) && sender.routes.length < OPERATORS.length);
		if (needsGateway) {
			approveTarget = sender;
			approveGatewayId = sender.gateway_id ? String(sender.gateway_id) : (gateways[0] ? String(gateways[0].id) : '');
			const nextRoutes: Record<string, string> = {};
			for (const op of OPERATORS) {
				const existing = sender.routes.find((r) => r.operator_key === op.key);
				nextRoutes[op.key] = existing ? String(existing.gateway_id) : approveGatewayId;
			}
			approveRoutes = nextRoutes;
			approveError = null;
			return;
		}
		try {
			const res = await supermanApi.post<{ message: string }>(`/api/v1/superman/senders/${sender.id}/approve`, {});
			notify('success', res.message);
			void loadSenders();
		} catch (err: unknown) {
			notify('error', err instanceof Error ? err.message : 'Unable to approve the sender.');
		}
	}

	async function submitApproval() {
		if (!approveTarget) return;
		approveError = null;
		const body: Record<string, unknown> = {};
		if (isMaskingType(approveTarget.type)) {
			const routePayload: Record<string, number> = {};
			for (const op of OPERATORS) {
				if (!approveRoutes[op.key]) {
					approveError = `Select a gateway for ${op.label}.`;
					return;
				}
				routePayload[op.key] = Number(approveRoutes[op.key]);
			}
			body.routes = routePayload;
			body.gateway_id = routePayload.gp;
		} else {
			if (!approveGatewayId) {
				approveError = 'Select a gateway.';
				return;
			}
			body.gateway_id = Number(approveGatewayId);
		}
		approveSaving = true;
		try {
			const res = await supermanApi.post<{ message: string }>(`/api/v1/superman/senders/${approveTarget.id}/approve`, body);
			approveTarget = null;
			notify('success', res.message);
			void loadSenders();
		} catch (err: unknown) {
			approveError = err instanceof Error ? err.message : 'Unable to approve the sender.';
		} finally {
			approveSaving = false;
		}
	}

	async function reject(sender: AdminSender) {
		const reason = window.prompt(`Rejection reason for "${sender.sender_id}":`);
		if (!reason?.trim()) return;
		try {
			const res = await supermanApi.post<{ message: string }>(`/api/v1/superman/senders/${sender.id}/reject`, { reason });
			notify('success', res.message);
			void loadSenders();
		} catch (err: unknown) {
			notify('error', err instanceof Error ? err.message : 'Unable to reject the sender.');
		}
	}

	async function remove(sender: AdminSender) {
		if (!window.confirm(`Delete sender "${sender.sender_id}"?`)) return;
		try {
			await supermanApi.delete(`/api/v1/superman/senders/${sender.id}`);
			selected = selected.filter((id) => id !== sender.id);
			notify('success', `Sender ${sender.sender_id} deleted.`);
			void loadSenders();
		} catch (err: unknown) {
			notify('error', err instanceof Error ? err.message : 'Unable to delete the sender.');
		}
	}

	async function removeSelected() {
		if (selected.length === 0) return;
		if (!window.confirm(`Delete ${selected.length} selected sender(s)?`)) return;
		try {
			const res = await supermanApi.post<{ message: string }>('/api/v1/superman/senders/bulk-delete', { ids: selected }).catch(async (err) => {
				// Older API builds only expose DELETE /senders with a body.
				throw err;
			});
			notify('success', res.message);
			selected = [];
			void loadSenders();
		} catch (err: unknown) {
			notify('error', err instanceof Error ? err.message : 'Unable to delete the selected senders.');
		}
	}

	async function copyId(sender: AdminSender) {
		try {
			await navigator.clipboard.writeText(sender.sender_id);
			notify('success', `Copied ${sender.sender_id}`);
		} catch {
			notify('error', 'Unable to copy to the clipboard.');
		}
	}

	async function fetchAllForExport(): Promise<AdminSender[]> {
		const query = buildQuery();
		query.set('export', '1');
		const res = await supermanApi.get<{ data: AdminSender[] }>(`/api/v1/superman/senders?${query.toString()}`);
		return res.data ?? [];
	}

	const exportColumns: ExportColumn<AdminSender>[] = [
		{ header: 'Sender ID', getValue: (s) => s.sender_id },
		{ header: 'Type', getValue: (s) => typeLabel(s.type) },
		{ header: 'Approval', getValue: (s) => s.approval_status },
		{ header: 'Gateways / Routes', getValue: (s) => routesSummary(s) },
		{ header: 'Customer', getValue: (s) => s.users.map((u) => u.name).join(', ') || 'Unassigned' },
		{ header: 'Enabled', getValue: (s) => (s.is_enabled ? 'Yes' : 'No') },
		{ header: 'Created', getValue: (s) => new Date(s.created_at).toLocaleDateString() }
	];

	function formatDate(value: string): string {
		const date = new Date(value);
		return Number.isNaN(date.getTime()) ? '—' : date.toLocaleDateString('en-GB', { day: '2-digit', month: 'short', year: 'numeric' });
	}

	onMount(() => {
		const params = $pageStore.url.searchParams;
		const saved = params.get('saved');
		if (saved) {
			notify('success', saved === 'created' ? 'Sender created.' : saved === 'updated' ? 'Sender updated.' : saved);
		}
		if (params.get('import') === '1') showImport = true;
		if (saved || params.get('import')) void goto('/superman/sender-cli/add', { replaceState: true, noScroll: true });

		void loadSenders();
		void loadSenderOptions()
			.then((options) => {
				gateways = options.gateways;
				customers = options.customers;
			})
			.catch(() => {
				/* filters simply stay empty */
			});
	});

	const iconButton =
		'inline-flex items-center gap-2 rounded-lg border bg-white px-3.5 py-2 text-sm font-medium shadow-xs transition hover:bg-gray-50 dark:bg-gray-900 dark:hover:bg-gray-800';
	const selectClass =
		'w-full rounded-lg border border-gray-300 bg-white px-2.5 py-1.5 text-sm text-gray-900 outline-none focus:border-emerald-600 focus:ring-1 focus:ring-emerald-600 dark:border-gray-700 dark:bg-gray-800 dark:text-white';
</script>

<svelte:head>
	<title>Senders — SMS Whiteinfo Administrator</title>
</svelte:head>

<svelte:window
	onclick={() => {
		menuId = null;
		showFilters = false;
		showColumns = false;
	}}
/>

<div class="space-y-5">
	<!-- Header -->
	<div class="flex flex-wrap items-end justify-between gap-3">
		<div>
			<nav class="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400" aria-label="Breadcrumb">
				<span>Senders</span>
				<span>&rsaquo;</span>
				<span>List</span>
			</nav>
			<h1 class="mt-1 text-3xl font-bold tracking-tight text-gray-950 dark:text-white">Senders</h1>
		</div>

		<div class="flex flex-wrap items-center gap-2">
			<button
				type="button"
				onclick={() => (showImport = true)}
				data-testid="import-senders"
				class="{iconButton} border-gray-300 text-gray-800 dark:border-gray-700 dark:text-gray-200"
			>
				<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5m-13.5-9L12 3m0 0 4.5 4.5M12 3v13.5" /></svg>
				Import Senders
			</button>
			<ExportDropdown
				title="Senders"
				columns={exportColumns}
				getData={fetchAllForExport}
				disabled={total === 0}
				buttonClass="{iconButton} border-emerald-600 text-emerald-700 disabled:cursor-not-allowed disabled:opacity-50 dark:border-emerald-600 dark:text-emerald-400"
			/>
			<a
				href="/superman/sender-cli/add/create"
				data-testid="new-sender"
				class="{iconButton} border-emerald-600 font-semibold text-emerald-700 dark:text-emerald-400"
			>
				New sender
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

	<!-- Table card -->
	<div class="rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
		<!-- Toolbar -->
		<div class="flex flex-wrap items-center justify-between gap-3 px-4 py-3">
			<div class="min-h-9">
				{#if selected.length > 0}
					<div class="flex items-center gap-3 text-sm" data-testid="bulk-bar">
						<span class="font-medium text-gray-700 dark:text-gray-200">{selected.length} selected</span>
						<button
							type="button"
							onclick={() => void removeSelected()}
							class="rounded-lg border border-rose-300 px-3 py-1.5 text-sm font-medium text-rose-700 hover:bg-rose-50 dark:border-rose-900 dark:text-rose-400 dark:hover:bg-rose-950/30"
						>
							Delete selected
						</button>
						<button type="button" onclick={() => (selected = [])} class="text-xs font-semibold text-gray-500 hover:text-gray-800">Deselect all</button>
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
						placeholder="Search Sender ID, user, gateway…"
						aria-label="Search senders"
						data-testid="global-search"
						class="w-full rounded-lg border border-gray-300 bg-white py-2 pl-9 pr-3 text-sm text-gray-900 outline-none focus:border-emerald-600 focus:ring-1 focus:ring-emerald-600 dark:border-gray-700 dark:bg-gray-800 dark:text-white"
					/>
				</div>

				<!-- Filters -->
				<div class="relative">
					<button
						type="button"
						aria-label="Filters"
						aria-expanded={showFilters}
						onclick={(e) => {
							e.stopPropagation();
							showFilters = !showFilters;
							showColumns = false;
						}}
						data-testid="filters-toggle"
						class="relative rounded-lg p-2 text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-800"
					>
						<svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M12 3c2.755 0 5.455.232 8.083.678.533.09.917.556.917 1.096v1.044a2.25 2.25 0 0 1-.659 1.591l-5.432 5.432a2.25 2.25 0 0 0-.659 1.591v2.927a2.25 2.25 0 0 1-1.244 2.013L9.75 21v-6.568a2.25 2.25 0 0 0-.659-1.591L3.659 7.409A2.25 2.25 0 0 1 3 5.818V4.774c0-.54.384-1.006.917-1.096A48.32 48.32 0 0 1 12 3Z" /></svg>
						<span
							data-testid="filter-count"
							class="absolute -right-1 -top-1 flex h-4 min-w-4 items-center justify-center rounded-full px-1 text-[10px] font-bold {activeFilterCount > 0 ? 'bg-emerald-600 text-white' : 'border border-gray-300 bg-white text-gray-500 dark:border-gray-600 dark:bg-gray-900'}"
						>{activeFilterCount}</span>
					</button>
					{#if showFilters}
						<div
							class="absolute right-0 z-30 mt-2 w-72 space-y-3 rounded-xl border border-gray-200 bg-white p-4 shadow-xl dark:border-gray-700 dark:bg-gray-900"
							role="dialog"
							aria-label="Filters"
							tabindex="-1"
							onclick={(e) => e.stopPropagation()}
							onkeydown={(e) => e.stopPropagation()}
						>
							<div class="flex items-center justify-between">
								<h3 class="text-sm font-semibold text-gray-900 dark:text-white">Filters</h3>
								<button type="button" onclick={resetFilters} class="text-xs font-semibold text-emerald-700 hover:underline dark:text-emerald-400">Reset</button>
							</div>
							<label class="block text-xs font-medium text-gray-600 dark:text-gray-300">Type
								<select bind:value={filterType} onchange={refilter} class="{selectClass} mt-1">
									<option value="">All</option>
									<option value="masking">Masking</option>
									<option value="non_masking">Non-masking</option>
								</select>
							</label>
							<label class="block text-xs font-medium text-gray-600 dark:text-gray-300">Approval
								<select bind:value={filterApproval} onchange={refilter} class="{selectClass} mt-1">
									<option value="">All</option>
									<option value="approved">Approved</option>
									<option value="pending">Pending</option>
									<option value="rejected">Rejected</option>
								</select>
							</label>
							<label class="block text-xs font-medium text-gray-600 dark:text-gray-300">Enabled
								<select bind:value={filterEnabled} onchange={refilter} class="{selectClass} mt-1">
									<option value="">All</option>
									<option value="yes">Enabled</option>
									<option value="no">Disabled</option>
								</select>
							</label>
							<label class="block text-xs font-medium text-gray-600 dark:text-gray-300">Primary gateway
								<select bind:value={filterGateway} onchange={refilter} class="{selectClass} mt-1">
									<option value="">All</option>
									{#each gateways as gateway (gateway.id)}
										<option value={String(gateway.id)}>{gateway.name}</option>
									{/each}
								</select>
							</label>
							<label class="block text-xs font-medium text-gray-600 dark:text-gray-300">Assignment
								<select bind:value={filterAssignment} onchange={refilter} class="{selectClass} mt-1">
									<option value="">All</option>
									<option value="used">Assigned to a customer</option>
									<option value="unused">Unassigned</option>
								</select>
							</label>
						</div>
					{/if}
				</div>

				<!-- Column toggle -->
				<div class="relative">
					<button
						type="button"
						aria-label="Toggle columns"
						aria-expanded={showColumns}
						onclick={(e) => {
							e.stopPropagation();
							showColumns = !showColumns;
							showFilters = false;
						}}
						class="rounded-lg p-2 text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-800"
					>
						<svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M9 4.5v15m6-15v15M4.5 4.5h15a1.5 1.5 0 0 1 1.5 1.5v12a1.5 1.5 0 0 1-1.5 1.5h-15A1.5 1.5 0 0 1 3 18V6a1.5 1.5 0 0 1 1.5-1.5Z" /></svg>
					</button>
					{#if showColumns}
						<div
							class="absolute right-0 z-30 mt-2 w-56 rounded-xl border border-gray-200 bg-white p-3 shadow-xl dark:border-gray-700 dark:bg-gray-900"
							role="dialog"
							aria-label="Columns"
							tabindex="-1"
							onclick={(e) => e.stopPropagation()}
							onkeydown={(e) => e.stopPropagation()}
						>
							<h3 class="mb-2 text-sm font-semibold text-gray-900 dark:text-white">Columns</h3>
							{#each Object.entries(COLUMN_LABELS) as [key, label] (key)}
								<label class="flex cursor-pointer items-center gap-2 py-1 text-sm text-gray-700 dark:text-gray-200">
									<input type="checkbox" bind:checked={visible[key as ColumnKey]} class="h-4 w-4 rounded border-gray-300 accent-emerald-600" />
									{label}
								</label>
							{/each}
						</div>
					{/if}
				</div>
			</div>
		</div>

		{#if error}
			<div role="alert" class="mx-4 mb-3 rounded-lg border border-rose-200 bg-rose-50 p-3 text-sm text-rose-700 dark:border-rose-900 dark:bg-rose-950/30 dark:text-rose-300">{error}</div>
		{/if}

		<div class="overflow-x-auto border-t border-gray-200 dark:border-gray-800">
			<table class="w-full min-w-[960px] text-left text-sm" data-testid="senders-table">
				<thead>
					<tr class="bg-gray-50/70 text-xs font-semibold text-gray-700 dark:bg-gray-800/40 dark:text-gray-200">
						<th class="w-12 px-4 py-3">
							<input type="checkbox" checked={allOnPageSelected} onchange={toggleSelectAll} aria-label="Select all on this page" class="h-4 w-4 rounded border-gray-300 accent-emerald-600" />
						</th>
						<th class="px-4 py-3">
							<button type="button" onclick={() => toggleSort('sender_id')} class="inline-flex items-center gap-1 uppercase tracking-wide">
								Sender ID
								<svg class="h-3.5 w-3.5 {sortKey === 'sender_id' ? 'text-emerald-600' : 'text-gray-400'} {sortKey === 'sender_id' && sortDir === 'desc' ? 'rotate-180' : ''}" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" /></svg>
							</button>
						</th>
						{#if visible.type}
							<th class="px-4 py-3">
								<button type="button" onclick={() => toggleSort('type')} class="inline-flex items-center gap-1 uppercase tracking-wide">
									Type
									<svg class="h-3.5 w-3.5 {sortKey === 'type' ? 'text-emerald-600' : 'text-gray-400'} {sortKey === 'type' && sortDir === 'desc' ? 'rotate-180' : ''}" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" /></svg>
								</button>
							</th>
						{/if}
						{#if visible.approval}
							<th class="px-4 py-3">
								<button type="button" onclick={() => toggleSort('approval_status')} class="inline-flex items-center gap-1 uppercase tracking-wide">
									Approval
									<svg class="h-3.5 w-3.5 {sortKey === 'approval_status' ? 'text-emerald-600' : 'text-gray-400'} {sortKey === 'approval_status' && sortDir === 'desc' ? 'rotate-180' : ''}" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" /></svg>
								</button>
							</th>
						{/if}
						{#if visible.routes}<th class="px-4 py-3 uppercase tracking-wide">Gateways / Routes</th>{/if}
						{#if visible.customers}<th class="px-4 py-3 uppercase tracking-wide">Customer</th>{/if}
						{#if visible.enabled}
							<th class="px-4 py-3">
								<button type="button" onclick={() => toggleSort('is_enabled')} class="inline-flex items-center gap-1 uppercase tracking-wide">
									Enabled
									<svg class="h-3.5 w-3.5 {sortKey === 'is_enabled' ? 'text-emerald-600' : 'text-gray-400'} {sortKey === 'is_enabled' && sortDir === 'desc' ? 'rotate-180' : ''}" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" /></svg>
								</button>
							</th>
						{/if}
						{#if visible.created}
							<th class="px-4 py-3">
								<button type="button" onclick={() => toggleSort('created_at')} class="inline-flex items-center gap-1 uppercase tracking-wide">
									Created
									<svg class="h-3.5 w-3.5 {sortKey === 'created_at' ? 'text-emerald-600' : 'text-gray-400'} {sortKey === 'created_at' && sortDir === 'desc' ? 'rotate-180' : ''}" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" /></svg>
								</button>
							</th>
						{/if}
						<th class="w-14 px-4 py-3"><span class="sr-only">Actions</span></th>
					</tr>
					<!-- Column search row -->
					<tr>
						<th class="px-4 py-2"></th>
						<th class="px-4 py-2 font-normal">
							<div class="relative w-44">
								<svg class="pointer-events-none absolute left-2.5 top-2 h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" /></svg>
								<input
									type="search"
									bind:value={senderIdSearch}
									oninput={debouncedRefilter}
									placeholder="Search"
									aria-label="Search by sender ID"
									data-testid="sender-id-search"
									class="w-full rounded-lg border border-gray-300 bg-white py-1.5 pl-8 pr-3 text-sm font-normal outline-none focus:border-emerald-600 focus:ring-1 focus:ring-emerald-600 dark:border-gray-700 dark:bg-gray-800 dark:text-white"
								/>
							</div>
						</th>
						<th colspan={colSpan - 2}></th>
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-100 dark:divide-gray-800 {loading && senders.length > 0 ? 'opacity-60' : ''}">
					{#each senders as sender (sender.id)}
						<tr
							onclick={(e) => {
								const el = e.target as HTMLElement;
								if (el.closest('button,a,input,label')) return;
								void goto(`/superman/sender-cli/add/${sender.id}`);
							}}
							class="cursor-pointer transition-colors hover:bg-gray-50/70 dark:hover:bg-gray-800/30 {selected.includes(sender.id) ? 'bg-emerald-50/40 dark:bg-emerald-950/10' : ''}"
						>
							<td class="px-4 py-3.5">
								<input type="checkbox" checked={selected.includes(sender.id)} onchange={() => toggleSelected(sender.id)} aria-label="Select {sender.sender_id}" class="h-4 w-4 rounded border-gray-300 accent-emerald-600" />
							</td>
							<td class="px-4 py-3.5">
								<button
									type="button"
									onclick={() => void copyId(sender)}
									title="Click to copy"
									class="inline-flex items-center gap-1.5 rounded-md border border-emerald-200 bg-emerald-50 px-2 py-1 text-xs font-medium text-emerald-800 hover:bg-emerald-100 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-300"
								>
									<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M15.666 3.888A2.25 2.25 0 0 0 13.5 2.25h-3c-1.03 0-1.9.693-2.166 1.638m7.332 0c.055.194.084.4.084.612v0a.75.75 0 0 1-.75.75H9a.75.75 0 0 1-.75-.75v0c0-.212.03-.418.084-.612m7.332 0c.646.049 1.288.11 1.927.184 1.1.128 1.907 1.077 1.907 2.185V19.5a2.25 2.25 0 0 1-2.25 2.25H6.75A2.25 2.25 0 0 1 4.5 19.5V6.257c0-1.108.806-2.057 1.907-2.185a48.208 48.208 0 0 1 1.927-.184" /></svg>
									{sender.sender_id}
								</button>
							</td>
							{#if visible.type}
								<td class="px-4 py-3.5">
									<span class="inline-flex items-center gap-1.5 rounded-md border border-emerald-200 bg-emerald-50 px-2 py-1 text-xs font-medium text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-300">
										{#if isMaskingType(sender.type)}
											<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="m10.5 21 5.25-11.25L21 21m-9-3h7.5M3 5.621a48.474 48.474 0 0 1 6-.371m0 0c1.12 0 2.233.038 3.334.114M9 5.25V3m3.334 2.364C11.176 10.658 7.69 15.08 3 17.502m9.334-12.138c.896.061 1.785.147 2.666.257m-4.589 8.495a18.023 18.023 0 0 1-3.827-5.802" /></svg>
										{:else}
											<span class="font-bold">#</span>
										{/if}
										{typeLabel(sender.type)}
									</span>
								</td>
							{/if}
							{#if visible.approval}
								<td class="px-4 py-3.5">
									<span
										class="inline-flex items-center gap-1.5 rounded-md border px-2 py-1 text-xs font-medium capitalize {sender.approval_status === 'approved'
											? 'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-300'
											: sender.approval_status === 'pending'
												? 'border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-300'
												: 'border-rose-200 bg-rose-50 text-rose-800 dark:border-rose-900 dark:bg-rose-950/30 dark:text-rose-300'}"
										title={sender.rejection_reason ?? ''}
									>
										<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor">
											{#if sender.approval_status === 'approved'}
												<path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
											{:else if sender.approval_status === 'pending'}
												<path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
											{:else}
												<path stroke-linecap="round" stroke-linejoin="round" d="m9.75 9.75 4.5 4.5m0-4.5-4.5 4.5M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
											{/if}
										</svg>
										{sender.approval_status}
									</span>
								</td>
							{/if}
							{#if visible.routes}
								<td class="max-w-md px-4 py-3.5">
									{#if isMaskingType(sender.type) && sender.routes.length > 0}
										<span class="block max-w-md truncate text-xs text-gray-700 dark:text-gray-300" title={routesSummary(sender)}>{routesSummary(sender)}</span>
									{:else if sender.gateway_name}
										<span class="inline-flex rounded-md border border-emerald-200 bg-emerald-50 px-2 py-1 text-xs font-medium text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-300">{sender.gateway_name}</span>
									{:else}
										<span class="text-xs text-gray-400">—</span>
									{/if}
								</td>
							{/if}
							{#if visible.customers}
								<td class="px-4 py-3.5 text-xs text-gray-700 dark:text-gray-300">
									{#if sender.users_count === 0}
										<span class="text-gray-400">Unassigned</span>
									{:else}
										{sender.users[0]?.name}{sender.users_count > 1 ? ` +${sender.users_count - 1}` : ''}
									{/if}
								</td>
							{/if}
							{#if visible.enabled}
								<td class="px-4 py-3.5">
									<button
										type="button"
										role="switch"
										aria-checked={sender.is_enabled}
										aria-label="Toggle {sender.sender_id}"
										disabled={sender.approval_status !== 'approved'}
										title={sender.approval_status !== 'approved' ? 'Only approved senders can be enabled' : ''}
										onclick={() => void toggleEnabled(sender)}
										class="relative inline-flex h-5 w-9 items-center rounded-full transition disabled:cursor-not-allowed disabled:opacity-40 {sender.is_enabled ? 'bg-emerald-600' : 'bg-gray-300 dark:bg-gray-700'}"
									>
										<span class="inline-block h-4 w-4 transform rounded-full bg-white shadow transition {sender.is_enabled ? 'translate-x-4.5' : 'translate-x-0.5'}"></span>
									</button>
								</td>
							{/if}
							{#if visible.created}
								<td class="whitespace-nowrap px-4 py-3.5 text-xs text-gray-500">{formatDate(sender.created_at)}</td>
							{/if}
							<td class="relative px-4 py-3.5 text-right">
								<button
									type="button"
									aria-label="Actions for {sender.sender_id}"
									onclick={(e) => {
										e.stopPropagation();
										menuId = menuId === sender.id ? null : sender.id;
									}}
									class="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-emerald-600/60 text-emerald-700 hover:bg-emerald-50 dark:border-emerald-700 dark:text-emerald-400 dark:hover:bg-emerald-950/30"
								>
									<svg class="h-4 w-4" fill="currentColor" viewBox="0 0 24 24"><circle cx="12" cy="5" r="1.6" /><circle cx="12" cy="12" r="1.6" /><circle cx="12" cy="19" r="1.6" /></svg>
								</button>
								{#if menuId === sender.id}
									<div class="absolute right-4 top-full z-20 w-40 rounded-lg border border-gray-200 bg-white p-1 text-left shadow-lg dark:border-gray-800 dark:bg-gray-900">
										<a href="/superman/sender-cli/add/{sender.id}" class="flex w-full rounded px-3 py-1.5 text-sm text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-gray-800">Edit</a>
										{#if sender.approval_status === 'pending'}
											<button type="button" onclick={() => void approve(sender)} class="flex w-full rounded px-3 py-1.5 text-sm text-emerald-700 hover:bg-emerald-50 dark:hover:bg-emerald-950/30">Approve</button>
											<button type="button" onclick={() => void reject(sender)} class="flex w-full rounded px-3 py-1.5 text-sm text-amber-700 hover:bg-amber-50 dark:hover:bg-amber-950/30">Reject</button>
										{/if}
										<button type="button" onclick={() => void remove(sender)} class="flex w-full rounded px-3 py-1.5 text-sm text-rose-600 hover:bg-rose-50 dark:hover:bg-rose-950/30">Delete</button>
									</div>
								{/if}
							</td>
						</tr>
					{:else}
						<tr>
							<td colspan={colSpan} class="px-4 py-14 text-center text-sm text-gray-400">
								{loading ? 'Loading senders…' : activeFilterCount > 0 || search || senderIdSearch ? 'No senders match your filters.' : 'No senders yet.'}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		<Pagination
			total={total}
			page={currentPage}
			{perPage}
			{loading}
			onPageChange={(p) => {
				currentPage = p;
				void loadSenders();
			}}
			onPerPageChange={(size) => {
				perPage = size;
				refilter();
			}}
		/>
	</div>
</div>

{#if approveTarget}
	<div class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-gray-950/50 p-4 pt-16" role="presentation" onclick={() => (approveTarget = null)}>
		<div
			class="w-full max-w-lg rounded-xl border border-gray-200 bg-white p-6 shadow-xl dark:border-gray-800 dark:bg-gray-900"
			role="dialog"
			aria-modal="true"
			aria-label="Approve Sender"
			tabindex="-1"
			onclick={(e) => e.stopPropagation()}
			onkeydown={(e) => e.key === 'Escape' && (approveTarget = null)}
		>
			<div class="mb-4 flex items-start justify-between">
				<div>
					<h2 class="text-lg font-semibold text-gray-950 dark:text-white">Approve {approveTarget.sender_id}</h2>
					<p class="mt-0.5 text-xs text-gray-500">Assign gateway routing before approving and enabling this sender.</p>
				</div>
				<button type="button" onclick={() => (approveTarget = null)} aria-label="Close" class="rounded-lg p-1 text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800">
					<svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" /></svg>
				</button>
			</div>

			{#if approveError}
				<div role="alert" class="mb-3 rounded-lg border border-rose-200 bg-rose-50 p-2.5 text-xs text-rose-700 dark:border-rose-900 dark:bg-rose-950/30 dark:text-rose-300">{approveError}</div>
			{/if}

			{#if isMaskingType(approveTarget.type)}
				<div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
					{#each OPERATORS as op (op.key)}
						<label class="block text-xs font-medium text-gray-700 dark:text-gray-300">
							{op.label} ({op.codes})*
							<select bind:value={approveRoutes[op.key]} class="{selectClass} mt-1">
								<option value="">Select Gateway</option>
								{#each gateways as gw (gw.id)}
									<option value={String(gw.id)}>{gw.name}</option>
								{/each}
							</select>
						</label>
					{/each}
				</div>
			{:else}
				<label class="block text-xs font-medium text-gray-700 dark:text-gray-300">
					Gateway*
					<select bind:value={approveGatewayId} class="{selectClass} mt-1">
						<option value="">Select Gateway</option>
						{#each gateways as gw (gw.id)}
							<option value={String(gw.id)}>{gw.name}</option>
						{/each}
					</select>
				</label>
			{/if}

			<div class="mt-5 flex items-center gap-2">
				<button
					type="button"
					disabled={approveSaving}
					onclick={() => void submitApproval()}
					class="rounded-lg border border-emerald-600 px-4 py-2 text-sm font-semibold text-emerald-700 hover:bg-emerald-50 disabled:opacity-50 dark:text-emerald-400 dark:hover:bg-emerald-950/30"
				>
					{approveSaving ? 'Approving…' : 'Approve & Enable'}
				</button>
				<button
					type="button"
					onclick={() => (approveTarget = null)}
					class="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800"
				>
					Cancel
				</button>
			</div>
		</div>
	</div>
{/if}

{#if showImport}
	<div class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-gray-950/50 p-4 pt-16" role="presentation" onclick={() => (showImport = false)}>
		<div
			class="w-full max-w-lg rounded-xl border border-gray-200 bg-white p-6 shadow-xl dark:border-gray-800 dark:bg-gray-900"
			role="dialog"
			aria-modal="true"
			aria-label="Import Senders"
			tabindex="-1"
			onclick={(e) => e.stopPropagation()}
			onkeydown={(e) => e.key === 'Escape' && (showImport = false)}
		>
			<div class="mb-3 flex items-start justify-between">
				<h2 class="text-lg font-semibold text-gray-950 dark:text-white">Import Senders</h2>
				<button type="button" onclick={() => (showImport = false)} aria-label="Close" class="rounded-lg p-1 text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800">
					<svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" /></svg>
				</button>
			</div>
			<ImportSendersPanel
				{gateways}
				{customers}
				onDone={(message) => {
					showImport = false;
					notify('success', message);
					refilter();
				}}
				onCancel={() => (showImport = false)}
			/>
		</div>
	</div>
{/if}
