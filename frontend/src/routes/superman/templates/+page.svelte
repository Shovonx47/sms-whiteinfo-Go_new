<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { supermanApi } from '$lib/superman-api.js';
	import ExportDropdown from '$lib/components/common/ExportDropdown.svelte';
	import Pagination from '$lib/components/common/Pagination.svelte';
	import type { ExportColumn } from '$lib/utils/export.js';
	import {
		type AdminTemplate,
		formatDate
	} from '$lib/template-admin.js';

	let templates = $state<AdminTemplate[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let flash = $state<{ type: 'success' | 'error'; text: string } | null>(null);
	let flashTimer: ReturnType<typeof setTimeout> | undefined;

	let search = $state('');
	let approvalFilter = $state<string>('all');
	let trashed = $state<'' | 'with' | 'only'>('');
	let sortKey = $state('created_at');
	let sortDir = $state<'asc' | 'desc'>('desc');
	let currentPage = $state(1);
	let perPage = $state(10);
	let total = $state(0);

	let showFilters = $state(false);
	let showBulk = $state(false);
	let activeMenuId = $state<number | null>(null);
	let selected = $state<number[]>([]);

	// Inline approve / reject modal states
	let approveModalTemplate = $state<AdminTemplate | null>(null);
	let rejectModalTemplate = $state<AdminTemplate | null>(null);
	let rejectionReason = $state('');
	let submittingAction = $state(false);

	let activeFilterCount = $derived(
		(approvalFilter !== 'all' ? 1 : 0) + (trashed !== '' ? 1 : 0)
	);

	let allOnPageSelected = $derived(
		templates.length > 0 && templates.every((t) => selected.includes(t.id))
	);

	let requestSeq = 0;

	function notify(type: 'success' | 'error', text: string) {
		flash = { type, text };
		clearTimeout(flashTimer);
		flashTimer = setTimeout(() => (flash = null), 6000);
	}

	function buildQuery(): URLSearchParams {
		const q = new URLSearchParams();
		if (search.trim()) q.set('search', search.trim());
		if (approvalFilter && approvalFilter !== 'all') q.set('approval', approvalFilter);
		if (trashed) q.set('trashed', trashed);
		if (sortKey) {
			q.set('sort', sortKey);
			q.set('dir', sortDir);
		}
		return q;
	}

	async function loadTemplates() {
		const seq = ++requestSeq;
		loading = true;
		error = null;
		try {
			const query = buildQuery();
			query.set('page', String(currentPage));
			query.set('per_page', String(perPage));
			const res = await supermanApi.get<{
				data: AdminTemplate[];
				meta: { total: number; page: number; per_page: number };
			}>(`/api/v1/superman/templates?${query.toString()}`);

			if (seq !== requestSeq) return;
			templates = res.data ?? [];
			total = res.meta?.total ?? templates.length;

			const lastPage = Math.max(1, Math.ceil(total / perPage));
			if (currentPage > lastPage && total > 0) {
				currentPage = lastPage;
				void loadTemplates();
			}
		} catch (err: unknown) {
			if (seq !== requestSeq) return;
			error = err instanceof Error ? err.message : 'Unable to load Bulk SMS Templates.';
		} finally {
			if (seq === requestSeq) loading = false;
		}
	}

	function refilter() {
		currentPage = 1;
		selected = [];
		void loadTemplates();
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
			selected = selected.filter((id) => !templates.some((t) => t.id === id));
		} else {
			selected = [...new Set([...selected, ...templates.map((t) => t.id)])];
		}
	}

	function toggleSelected(id: number) {
		selected = selected.includes(id) ? selected.filter((v) => v !== id) : [...selected, id];
	}

	async function bulk(action: 'delete' | 'restore' | 'force_delete') {
		showBulk = false;
		if (selected.length === 0) return;
		const verb =
			action === 'delete'
				? 'Delete'
				: action === 'restore'
					? 'Restore'
					: 'Permanently delete';
		if (!window.confirm(`${verb} ${selected.length} selected template(s)?`)) return;
		try {
			const res = await supermanApi.post<{ message: string }>('/api/v1/superman/templates/bulk', {
				ids: selected,
				action
			});
			notify('success', res.message);
			selected = [];
			void loadTemplates();
		} catch (err: unknown) {
			notify('error', err instanceof Error ? err.message : 'Bulk action failed.');
		}
	}

	async function handleApproveConfirm() {
		if (!approveModalTemplate) return;
		submittingAction = true;
		try {
			await supermanApi.post(`/api/v1/superman/templates/${approveModalTemplate.id}/approve`, {});
			notify(
				'success',
				`Bulk SMS Template '${approveModalTemplate.name}' approved successfully.`
			);
			approveModalTemplate = null;
			void loadTemplates();
		} catch (err: unknown) {
			notify('error', err instanceof Error ? err.message : 'Approval failed.');
		} finally {
			submittingAction = false;
		}
	}

	async function handleRejectConfirm() {
		if (!rejectModalTemplate) return;
		if (!rejectionReason.trim()) {
			notify('error', 'Please provide a reason for rejection.');
			return;
		}
		submittingAction = true;
		try {
			await supermanApi.post(`/api/v1/superman/templates/${rejectModalTemplate.id}/reject`, {
				reason: rejectionReason.trim()
			});
			notify(
				'success',
				`Bulk SMS Template '${rejectModalTemplate.name}' has been rejected.`
			);
			rejectModalTemplate = null;
			rejectionReason = '';
			void loadTemplates();
		} catch (err: unknown) {
			notify('error', err instanceof Error ? err.message : 'Rejection failed.');
		} finally {
			submittingAction = false;
		}
	}

	async function handleDeleteSingle(template: AdminTemplate) {
		activeMenuId = null;
		if (!window.confirm(`Delete Bulk SMS Template "${template.name}"?`)) return;
		try {
			await supermanApi.delete(`/api/v1/superman/templates/${template.id}`);
			notify('success', 'Bulk SMS Template deleted.');
			void loadTemplates();
		} catch (err: unknown) {
			notify('error', err instanceof Error ? err.message : 'Unable to delete template.');
		}
	}

	async function fetchAllForExport(): Promise<AdminTemplate[]> {
		const query = buildQuery();
		query.set('export', '1');
		const res = await supermanApi.get<{ data: AdminTemplate[] }>(
			`/api/v1/superman/templates?${query.toString()}`
		);
		return res.data ?? [];
	}

	const exportColumns: ExportColumn<AdminTemplate>[] = [
		{ header: 'Customer', getValue: (t) => t.customer },
		{ header: 'Template Name', getValue: (t) => t.name },
		{ header: 'Message Template', getValue: (t) => t.body },
		{ header: 'Approval', getValue: (t) => t.approval.toUpperCase() },
		{ header: 'Reviewed By', getValue: (t) => t.reviewed_by ?? '—' },
		{ header: 'Reviewed At', getValue: (t) => formatDate(t.approved_at) },
		{ header: 'Rejection Reason', getValue: (t) => t.rejection_reason ?? '—' },
		{ header: 'Created At', getValue: (t) => formatDate(t.created_at) }
	];

	function closeAllMenus() {
		activeMenuId = null;
		showBulk = false;
	}

	onMount(() => {
		void loadTemplates();
	});
</script>

<svelte:window onclick={closeAllMenus} />

<svelte:head>
	<title>Bulk SMS Templates — Superman Administrator</title>
</svelte:head>

<div class="space-y-4">
	<!-- Flash message -->
	{#if flash}
		<div
			class="flex items-center justify-between rounded-lg border px-4 py-3 text-sm shadow-xs transition-all {flash.type ===
			'success'
				? 'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-200'
				: 'border-rose-200 bg-rose-50 text-rose-800 dark:border-rose-800 dark:bg-rose-950/40 dark:text-rose-200'}"
		>
			<span>{flash.text}</span>
			<button
				onclick={() => (flash = null)}
				class="text-xs font-semibold hover:underline"
				aria-label="Dismiss">✕</button
			>
		</div>
	{/if}

	<!-- Filament Header: Breadcrumbs + Title + Top Actions -->
	<div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
		<div>
			<nav class="flex items-center gap-1.5 text-xs text-gray-500 dark:text-gray-400">
				<span>Bulk SMS Templates</span>
				<span class="text-gray-400">›</span>
				<span class="font-medium text-gray-700 dark:text-gray-300">List</span>
			</nav>
			<h1 class="mt-1 text-2xl font-bold tracking-tight text-gray-950 dark:text-white">
				Bulk SMS Templates
			</h1>
		</div>

		<div class="flex items-center gap-2">
			<!-- Export Action Dropdown -->
			<ExportDropdown
				title="Bulk SMS Templates"
				columns={exportColumns}
				getData={fetchAllForExport}
				disabled={loading}
			/>

			<!-- New Bulk SMS Template button -->
			<a
				href="/superman/templates/create"
				class="inline-flex items-center gap-1.5 rounded-lg border border-emerald-600 bg-emerald-600 px-3.5 py-2 text-xs font-semibold text-white shadow-xs transition hover:bg-emerald-700 focus:outline-hidden focus:ring-2 focus:ring-emerald-500 dark:border-emerald-500 dark:bg-emerald-600 dark:hover:bg-emerald-700"
			>
				<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
				</svg>
				<span>New Bulk SMS Template</span>
			</a>
		</div>
	</div>

	<!-- Main Table Card with Search & Filters -->
	<div
		class="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900"
	>
		<!-- Table Header Controls: Search + Filters -->
		<div
			class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 p-3.5 dark:border-gray-800"
		>
			<!-- Left: Bulk actions if records selected -->
			<div class="flex items-center gap-2">
				{#if selected.length > 0}
					<div class="relative" onclick={(e) => e.stopPropagation()}>
						<button
							onclick={() => (showBulk = !showBulk)}
							class="inline-flex items-center gap-1.5 rounded-lg border border-gray-300 bg-white px-3 py-1.5 text-xs font-semibold text-gray-700 shadow-xs hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200"
						>
							<span>Bulk actions ({selected.length})</span>
							<svg class="h-3.5 w-3.5 text-gray-400" viewBox="0 0 20 20" fill="currentColor">
								<path
									fill-rule="evenodd"
									d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z"
									clip-rule="evenodd"
								/>
							</svg>
						</button>
						{#if showBulk}
							<div
								class="absolute left-0 z-30 mt-1 w-44 rounded-lg border border-gray-200 bg-white py-1 shadow-lg dark:border-gray-700 dark:bg-gray-800"
							>
								{#if trashed === 'only'}
									<button
										onclick={() => bulk('restore')}
										class="flex w-full items-center px-3 py-2 text-left text-xs font-medium text-emerald-600 hover:bg-gray-50 dark:hover:bg-gray-700"
									>
										Restore selected
									</button>
									<button
										onclick={() => bulk('force_delete')}
										class="flex w-full items-center px-3 py-2 text-left text-xs font-medium text-rose-600 hover:bg-gray-50 dark:hover:bg-gray-700"
									>
										Permanently delete
									</button>
								{:else}
									<button
										onclick={() => bulk('delete')}
										class="flex w-full items-center px-3 py-2 text-left text-xs font-medium text-rose-600 hover:bg-gray-50 dark:hover:bg-gray-700"
									>
										Delete selected
									</button>
								{/if}
							</div>
						{/if}
					</div>
				{/if}
			</div>

			<!-- Right: Search Input + Filter Button -->
			<div class="flex items-center gap-2">
				<div class="relative w-64 sm:w-72">
					<svg
						class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400"
						fill="none"
						viewBox="0 0 24 24"
						stroke="currentColor"
					>
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							stroke-width="2"
							d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
						/>
					</svg>
					<input
						type="search"
						bind:value={search}
						oninput={debouncedRefilter}
						placeholder="Search"
						class="w-full rounded-lg border border-gray-200 bg-white py-1.5 pl-9 pr-3 text-xs outline-hidden transition focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500 dark:border-gray-700 dark:bg-gray-800 dark:text-white"
					/>
				</div>

				<!-- Filter popover button with badge -->
				<div class="relative" onclick={(e) => e.stopPropagation()}>
					<button
						onclick={() => (showFilters = !showFilters)}
						class="relative inline-flex items-center justify-center rounded-lg border border-gray-200 bg-white p-2 text-gray-600 shadow-xs hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300"
						title="Filters"
						aria-label="Filter"
					>
						<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
							<path
								stroke-linecap="round"
								stroke-linejoin="round"
								stroke-width="2"
								d="M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z"
							/>
						</svg>
						<span
							class="absolute -top-1.5 -right-1.5 flex h-4 w-4 items-center justify-center rounded-full bg-emerald-600 text-[10px] font-bold text-white"
						>
							{activeFilterCount}
						</span>
					</button>

					{#if showFilters}
						<div
							class="absolute right-0 z-30 mt-2 w-72 rounded-xl border border-gray-200 bg-white p-4 shadow-xl dark:border-gray-700 dark:bg-gray-800"
						>
							<div class="flex items-center justify-between pb-3 border-b border-gray-100 dark:border-gray-700">
								<h3 class="text-xs font-semibold text-gray-900 dark:text-white">Filters</h3>
								{#if activeFilterCount > 0}
									<button
										onclick={() => {
											approvalFilter = 'all';
											trashed = '';
											refilter();
										}}
										class="text-[11px] font-medium text-emerald-600 hover:underline"
									>
										Reset
									</button>
								{/if}
							</div>

							<div class="mt-3 space-y-3">
								<div>
									<label
										for="filter-approval"
										class="block text-[11px] font-medium text-gray-700 dark:text-gray-300"
									>
										Approval Status
									</label>
									<select
										id="filter-approval"
										bind:value={approvalFilter}
										onchange={refilter}
										class="mt-1 w-full rounded-lg border border-gray-200 bg-white px-2.5 py-1.5 text-xs text-gray-900 dark:border-gray-700 dark:bg-gray-800 dark:text-white"
									>
										<option value="all">All</option>
										<option value="pending">Pending</option>
										<option value="approved">Approved</option>
										<option value="rejected">Rejected</option>
									</select>
								</div>

								<div>
									<label
										for="filter-trashed"
										class="block text-[11px] font-medium text-gray-700 dark:text-gray-300"
									>
										Trashed Records
									</label>
									<select
										id="filter-trashed"
										bind:value={trashed}
										onchange={refilter}
										class="mt-1 w-full rounded-lg border border-gray-200 bg-white px-2.5 py-1.5 text-xs text-gray-900 dark:border-gray-700 dark:bg-gray-800 dark:text-white"
									>
										<option value="">Without trashed records</option>
										<option value="with">With trashed records</option>
										<option value="only">Only trashed records</option>
									</select>
								</div>
							</div>
						</div>
					{/if}
				</div>
			</div>
		</div>

		<!-- Table View -->
		{#if loading}
			<div class="flex flex-col items-center justify-center py-20 text-gray-400">
				<svg class="h-8 w-8 animate-spin text-emerald-600" viewBox="0 0 24 24" fill="none">
					<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"
					></circle>
					<path
						class="opacity-75"
						fill="currentColor"
						d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
					></path>
				</svg>
				<p class="mt-3 text-xs">Loading Bulk SMS Templates...</p>
			</div>
		{:else if error}
			<div class="p-8 text-center text-xs text-rose-600 dark:text-rose-400">
				<p class="font-semibold">{error}</p>
				<button
					onclick={() => void loadTemplates()}
					class="mt-3 rounded-lg border border-rose-300 bg-white px-3 py-1.5 text-xs font-medium text-rose-700 shadow-xs hover:bg-rose-50"
				>
					Retry
				</button>
			</div>
		{:else if templates.length === 0}
			<div class="py-16 text-center text-xs text-gray-500 dark:text-gray-400">
				No bulk SMS templates found.
			</div>
		{:else}
			<div class="overflow-x-auto">
				<table class="w-full text-left text-xs">
					<thead>
						<tr
							class="border-b border-gray-100 bg-gray-50/50 text-[11px] font-semibold uppercase tracking-wider text-gray-500 dark:border-gray-800 dark:bg-gray-800/50 dark:text-gray-400"
						>
							<th class="w-10 px-4 py-3">
								<input
									type="checkbox"
									checked={allOnPageSelected}
									onchange={toggleSelectAll}
									class="rounded border-gray-300 text-emerald-600 focus:ring-emerald-500 dark:border-gray-700 dark:bg-gray-800"
									aria-label="Select all"
								/>
							</th>
							<th
								class="cursor-pointer select-none px-4 py-3 hover:text-gray-700 dark:hover:text-gray-200"
								onclick={() => toggleSort('customer')}
							>
								<div class="flex items-center gap-1">
									<span>CUSTOMER</span>
									{#if sortKey === 'customer'}
										<span class="text-emerald-600">{sortDir === 'asc' ? '▲' : '▼'}</span>
									{:else}
										<span class="text-gray-300">↕</span>
									{/if}
								</div>
							</th>
							<th
								class="cursor-pointer select-none px-4 py-3 hover:text-gray-700 dark:hover:text-gray-200"
								onclick={() => toggleSort('name')}
							>
								<div class="flex items-center gap-1">
									<span>TEMPLATE NAME</span>
									{#if sortKey === 'name'}
										<span class="text-emerald-600">{sortDir === 'asc' ? '▲' : '▼'}</span>
									{:else}
										<span class="text-gray-300">↕</span>
									{/if}
								</div>
							</th>
							<th class="px-4 py-3">MESSAGE TEMPLATE</th>
							<th
								class="cursor-pointer select-none px-4 py-3 hover:text-gray-700 dark:hover:text-gray-200"
								onclick={() => toggleSort('approval')}
							>
								<div class="flex items-center gap-1">
									<span>APPROVAL</span>
									{#if sortKey === 'approval'}
										<span class="text-emerald-600">{sortDir === 'asc' ? '▲' : '▼'}</span>
									{:else}
										<span class="text-gray-300">↕</span>
									{/if}
								</div>
							</th>
							<th class="px-4 py-3">REVIEWED BY</th>
							<th class="px-4 py-3">APPROVED AT</th>
							<th class="px-4 py-3">REJECTION REASON</th>
							<th class="px-4 py-3">CREATED AT</th>
							<th class="px-4 py-3 text-right">ACTIONS</th>
						</tr>
					</thead>
					<tbody class="divide-y divide-gray-100 dark:divide-gray-800">
						{#each templates as t (t.id)}
							<tr
								class="transition hover:bg-gray-50/60 dark:hover:bg-gray-800/40 {selected.includes(
									t.id
								)
									? 'bg-emerald-50/40 dark:bg-emerald-950/20'
									: ''}"
							>
								<td class="w-10 px-4 py-3">
									<input
										type="checkbox"
										checked={selected.includes(t.id)}
										onchange={() => toggleSelected(t.id)}
										class="rounded border-gray-300 text-emerald-600 focus:ring-emerald-500 dark:border-gray-700 dark:bg-gray-800"
										aria-label="Select row"
									/>
								</td>
								<td class="px-4 py-3 text-gray-800 dark:text-gray-200">
									{t.customer || '—'}
								</td>
								<td class="px-4 py-3 font-bold text-gray-950 dark:text-white">
									{t.name}
								</td>
								<td
									class="max-w-xs truncate px-4 py-3 text-gray-600 dark:text-gray-300"
									title={t.body}
								>
									{t.body}
								</td>
								<td class="px-4 py-3">
									{#if t.approval === 'approved'}
										<span
											class="inline-flex items-center gap-1 rounded-full border border-emerald-200 bg-emerald-50 px-2.5 py-0.5 text-[11px] font-semibold text-emerald-700 dark:border-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-300"
										>
											<svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
												<path
													stroke-linecap="round"
													stroke-linejoin="round"
													stroke-width="2.5"
													d="M5 13l4 4L19 7"
												/>
											</svg>
											Approved
										</span>
									{:else if t.approval === 'rejected'}
										<span
											class="inline-flex items-center gap-1 rounded-full border border-rose-200 bg-rose-50 px-2.5 py-0.5 text-[11px] font-semibold text-rose-700 dark:border-rose-800 dark:bg-rose-950/40 dark:text-rose-300"
										>
											<svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
												<path
													stroke-linecap="round"
													stroke-linejoin="round"
													stroke-width="2"
													d="M6 18L18 6M6 6l12 12"
												/>
											</svg>
											Rejected
										</span>
									{:else}
										<span
											class="inline-flex items-center gap-1 rounded-full border border-amber-200 bg-amber-50 px-2.5 py-0.5 text-[11px] font-semibold text-amber-700 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-300"
										>
											<svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
												<path
													stroke-linecap="round"
													stroke-linejoin="round"
													stroke-width="2"
													d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
												/>
											</svg>
											Pending
										</span>
									{/if}
								</td>
								<td class="px-4 py-3 text-gray-700 dark:text-gray-300">{t.reviewed_by || '—'}</td>
								<td class="px-4 py-3 text-gray-500">{t.approved_at ? formatDate(t.approved_at) : '—'}</td>
								<td class="max-w-50 truncate px-4 py-3 text-rose-600" title={t.rejection_reason || ''}>{t.rejection_reason || '—'}</td>
								<td class="px-4 py-3 text-gray-400">{formatDate(t.created_at)}</td>
								<td class="px-4 py-3 text-right">
									<div class="flex items-center justify-end gap-1.5">
										<!-- Inline Approval buttons if pending -->
										{#if t.approval === 'pending'}
											<button
												onclick={() => (approveModalTemplate = t)}
												class="inline-flex items-center gap-1 rounded-lg border border-emerald-600 bg-white px-2.5 py-1 text-[11px] font-semibold text-emerald-600 transition hover:bg-emerald-50 focus:outline-hidden dark:bg-transparent dark:hover:bg-emerald-950/30"
											>
												<svg
													class="h-3.5 w-3.5"
													fill="none"
													viewBox="0 0 24 24"
													stroke="currentColor"
												>
													<path
														stroke-linecap="round"
														stroke-linejoin="round"
														stroke-width="2"
														d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
													/>
												</svg>
												Approve
											</button>
											<button
												onclick={() => {
													rejectModalTemplate = t;
													rejectionReason = '';
												}}
												class="inline-flex items-center gap-1 rounded-lg border border-rose-600 bg-white px-2.5 py-1 text-[11px] font-semibold text-rose-600 transition hover:bg-rose-50 focus:outline-hidden dark:bg-transparent dark:hover:bg-rose-950/30"
											>
												<svg
													class="h-3.5 w-3.5"
													fill="none"
													viewBox="0 0 24 24"
													stroke="currentColor"
												>
													<path
														stroke-linecap="round"
														stroke-linejoin="round"
														stroke-width="2"
														d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z"
													/>
												</svg>
												Reject
											</button>
										{/if}

										<!-- 3-dots action menu -->
										<div class="relative" onclick={(e) => e.stopPropagation()}>
											<button
												onclick={() =>
													(activeMenuId = activeMenuId === t.id ? null : t.id)}
												class="rounded-lg p-1.5 text-gray-500 hover:bg-gray-100 hover:text-gray-700 dark:text-gray-400 dark:hover:bg-gray-800 dark:hover:text-gray-200"
												aria-label="Actions"
											>
												<svg class="h-4 w-4" fill="currentColor" viewBox="0 0 20 20">
													<path
														d="M10 6a2 2 0 110-4 2 2 0 010 4zM10 12a2 2 0 110-4 2 2 0 010 4zM10 18a2 2 0 110-4 2 2 0 010 4z"
													/>
												</svg>
											</button>

											{#if activeMenuId === t.id}
												<div
													class="absolute right-0 z-30 mt-1 w-40 rounded-lg border border-gray-200 bg-white py-1 shadow-lg dark:border-gray-700 dark:bg-gray-800"
												>
													<a
														href={`/superman/templates/${t.id}`}
														class="flex items-center gap-2 px-3 py-1.5 text-xs text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-gray-700"
													>
														<svg
															class="h-3.5 w-3.5 text-gray-400"
															fill="none"
															viewBox="0 0 24 24"
															stroke="currentColor"
														>
															<path
																stroke-linecap="round"
																stroke-linejoin="round"
																stroke-width="2"
																d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
															/>
															<path
																stroke-linecap="round"
																stroke-linejoin="round"
																stroke-width="2"
																d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
															/>
														</svg>
														<span>View</span>
													</a>
													<a
														href={`/superman/templates/${t.id}/edit`}
														class="flex items-center gap-2 px-3 py-1.5 text-xs text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-gray-700"
													>
														<svg
															class="h-3.5 w-3.5 text-gray-400"
															fill="none"
															viewBox="0 0 24 24"
															stroke="currentColor"
														>
															<path
																stroke-linecap="round"
																stroke-linejoin="round"
																stroke-width="2"
																d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
															/>
														</svg>
														<span>Edit</span>
													</a>

													{#if t.approval === 'rejected'}
														<button
															onclick={() => {
																activeMenuId = null;
																approveModalTemplate = t;
															}}
															class="flex w-full items-center gap-2 px-3 py-1.5 text-xs text-emerald-600 hover:bg-gray-50 dark:hover:bg-gray-700"
														>
															<svg
																class="h-3.5 w-3.5"
																fill="none"
																viewBox="0 0 24 24"
																stroke="currentColor"
															>
																<path
																	stroke-linecap="round"
																	stroke-linejoin="round"
																	stroke-width="2"
																	d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
																/>
															</svg>
															<span>Re-Approve</span>
														</button>
													{/if}

													<button
														onclick={() => handleDeleteSingle(t)}
														class="flex w-full items-center gap-2 px-3 py-1.5 text-xs text-rose-600 hover:bg-gray-50 dark:hover:bg-gray-700"
													>
														<svg
															class="h-3.5 w-3.5"
															fill="none"
															viewBox="0 0 24 24"
															stroke="currentColor"
														>
															<path
																stroke-linecap="round"
																stroke-linejoin="round"
																stroke-width="2"
																d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
															/>
														</svg>
														<span>Delete</span>
													</button>
												</div>
											{/if}
										</div>
									</div>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>

			<!-- Pagination Footer -->
			<div
				class="flex flex-col items-center justify-between gap-3 border-t border-gray-100 px-4 py-3 sm:flex-row dark:border-gray-800"
			>
				<div class="text-xs text-gray-500 dark:text-gray-400">
					Showing {total === 0 ? 0 : (currentPage - 1) * perPage + 1} to {Math.min(
						currentPage * perPage,
						total
					)} of {total} results
				</div>

				<div class="flex items-center gap-4">
					<div class="flex items-center gap-2">
						<label for="per-page-select" class="text-xs text-gray-500 dark:text-gray-400">
							Per page
						</label>
						<select
							id="per-page-select"
							bind:value={perPage}
							onchange={() => {
								currentPage = 1;
								void loadTemplates();
							}}
							class="rounded-lg border border-gray-200 bg-white px-2.5 py-1 text-xs text-gray-700 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200"
						>
							<option value={10}>10</option>
							<option value={25}>25</option>
							<option value={50}>50</option>
						</select>
					</div>

					<Pagination
						{total}
						page={currentPage}
						{perPage}
						onPageChange={(p) => {
							currentPage = p;
							void loadTemplates();
						}}
					/>
				</div>
			</div>
		{/if}
	</div>
</div>

<!-- Approve Confirmation Modal -->
{#if approveModalTemplate}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-xs"
		onclick={() => (approveModalTemplate = null)}
	>
		<div
			class="w-full max-w-md rounded-2xl border border-gray-200 bg-white p-6 shadow-2xl dark:border-gray-800 dark:bg-gray-900"
			onclick={(e) => e.stopPropagation()}
		>
			<h2 class="text-base font-bold text-gray-950 dark:text-white">
				{approveModalTemplate.approval === 'rejected' ? 'Re-Approve' : 'Approve'} Bulk SMS Template
			</h2>
			<p class="mt-2 text-xs text-gray-600 dark:text-gray-300">
				Once approved, the customer will be able to dispatch campaigns with this template immediately
				without waiting for daily admin approvals. The customer will no longer be able to edit its
				contents.
			</p>

			<div class="mt-4 rounded-lg bg-gray-50 p-3 text-xs dark:bg-gray-800/60">
				<p><strong class="font-semibold text-gray-800 dark:text-gray-200">Customer:</strong> {approveModalTemplate.customer}</p>
				<p class="mt-1"><strong class="font-semibold text-gray-800 dark:text-gray-200">Template:</strong> {approveModalTemplate.name}</p>
			</div>

			<div class="mt-6 flex justify-end gap-2">
				<button
					onclick={() => (approveModalTemplate = null)}
					disabled={submittingAction}
					class="rounded-lg border border-gray-300 bg-white px-3.5 py-2 text-xs font-semibold text-gray-700 hover:bg-gray-50 disabled:opacity-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200"
				>
					Cancel
				</button>
				<button
					onclick={handleApproveConfirm}
					disabled={submittingAction}
					class="inline-flex items-center gap-1.5 rounded-lg bg-emerald-600 px-4 py-2 text-xs font-semibold text-white shadow-xs hover:bg-emerald-700 disabled:opacity-50"
				>
					{#if submittingAction}
						<svg class="h-3.5 w-3.5 animate-spin" viewBox="0 0 24 24" fill="none">
							<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
							<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
						</svg>
					{/if}
					<span>Approve</span>
				</button>
			</div>
		</div>
	</div>
{/if}

<!-- Reject Modal with Reason Input -->
{#if rejectModalTemplate}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-xs"
		onclick={() => (rejectModalTemplate = null)}
	>
		<div
			class="w-full max-w-md rounded-2xl border border-gray-200 bg-white p-6 shadow-2xl dark:border-gray-800 dark:bg-gray-900"
			onclick={(e) => e.stopPropagation()}
		>
			<h2 class="text-base font-bold text-gray-950 dark:text-white">
				Reject Bulk SMS Template
			</h2>
			<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
				Please provide the reason why this template was rejected so the customer can fix it.
			</p>

			<div class="mt-4">
				<label
					for="rejection-reason"
					class="block text-xs font-medium text-gray-800 dark:text-gray-200"
				>
					Reason for Rejection <span class="text-rose-500">*</span>
				</label>
				<textarea
					id="rejection-reason"
					bind:value={rejectionReason}
					rows="3"
					placeholder="e.g. Promotional text violates regulatory compliance. Please update and resubmit."
					class="mt-1.5 w-full rounded-lg border border-gray-300 p-2.5 text-xs text-gray-900 outline-hidden transition focus:border-rose-500 focus:ring-1 focus:ring-rose-500 dark:border-gray-700 dark:bg-gray-800 dark:text-white"
				></textarea>
			</div>

			<div class="mt-6 flex justify-end gap-2">
				<button
					onclick={() => (rejectModalTemplate = null)}
					disabled={submittingAction}
					class="rounded-lg border border-gray-300 bg-white px-3.5 py-2 text-xs font-semibold text-gray-700 hover:bg-gray-50 disabled:opacity-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200"
				>
					Cancel
				</button>
				<button
					onclick={handleRejectConfirm}
					disabled={submittingAction || !rejectionReason.trim()}
					class="inline-flex items-center gap-1.5 rounded-lg bg-rose-600 px-4 py-2 text-xs font-semibold text-white shadow-xs hover:bg-rose-700 disabled:opacity-50"
				>
					{#if submittingAction}
						<svg class="h-3.5 w-3.5 animate-spin" viewBox="0 0 24 24" fill="none">
							<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
							<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
						</svg>
					{/if}
					<span>Reject</span>
				</button>
			</div>
		</div>
	</div>
{/if}
