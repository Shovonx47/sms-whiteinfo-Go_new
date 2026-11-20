<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { supermanApi } from '$lib/superman-api.js';
	import ExportDropdown from '$lib/components/common/ExportDropdown.svelte';
	import type { ExportColumn } from '$lib/utils/export.js';

	interface KYCDashboardStats {
		total_kyc: number;
		pending: number;
		under_review: number;
		verified: number;
		rejected: number;
		resubmit_required: number;
		today_submissions: number;
		today_approved: number;
	}

	interface KYCCustomer {
		id: number;
		full_name: string;
		email: string;
		mobile: string;
		nid_number: string | null;
		kyc_status: string;
		submitted_at: string | null;
		verified_at: string | null;
	}

	interface DashboardResponse {
		stats: KYCDashboardStats;
		data: KYCCustomer[];
		meta: {
			page: number;
			per_page: number;
			total: number;
			total_pages: number;
		};
	}

	let stats = $state<KYCDashboardStats>({
		total_kyc: 0,
		pending: 0,
		under_review: 0,
		verified: 0,
		rejected: 0,
		resubmit_required: 0,
		today_submissions: 0,
		today_approved: 0
	});

	let customers = $state<KYCCustomer[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	// Filters & Pagination
	let search = $state('');
	let statusFilter = $state('all');
	let currentPage = $state(1);
	let perPage = $state(10);
	let totalRecords = $state(0);
	let totalPages = $state(1);

	// Three-dot action menu tracking
	let activeMenuId = $state<number | null>(null);

	// Quick Document Modal
	let inspectModalOpen = $state(false);
	let inspectingCustomer = $state<KYCCustomer | null>(null);
	let inspectLoading = $state(false);
	let inspectData = $state<any>(null);

	let searchTimeout: ReturnType<typeof setTimeout> | null = null;

	function handleSearchInput() {
		if (searchTimeout) clearTimeout(searchTimeout);
		searchTimeout = setTimeout(() => {
			currentPage = 1;
			void loadDashboardData();
		}, 300);
	}

	function selectStatusFilter(status: string) {
		statusFilter = status;
		currentPage = 1;
		void loadDashboardData();
	}

	async function loadDashboardData() {
		loading = true;
		error = null;
		try {
			const queryParams = new URLSearchParams({
				page: currentPage.toString(),
				per_page: perPage.toString()
			});
			if (statusFilter && statusFilter !== 'all') {
				queryParams.set('status', statusFilter);
			}
			if (search.trim()) {
				queryParams.set('search', search.trim());
			}

			const response = await supermanApi.get<DashboardResponse>(`/api/v1/superman/kyc/dashboard?${queryParams.toString()}`);
			stats = response.stats;
			customers = response.data;
			totalRecords = response.meta.total;
			totalPages = response.meta.total_pages;
		} catch (err: unknown) {
			error = err instanceof Error ? err.message : 'Unable to load KYC Dashboard data.';
		} finally {
			loading = false;
		}
	}

	function goToPage(page: number) {
		if (page < 1 || page > totalPages || page === currentPage) return;
		currentPage = page;
		void loadDashboardData();
	}

	function handlePerPageChange(event: Event) {
		const target = event.target as HTMLSelectElement;
		perPage = Number(target.value);
		currentPage = 1;
		void loadDashboardData();
	}

	function toggleMenu(id: number, event: MouseEvent) {
		event.stopPropagation();
		activeMenuId = activeMenuId === id ? null : id;
	}

	function closeMenu() {
		activeMenuId = null;
	}

	async function openInspect(cust: KYCCustomer, event?: MouseEvent) {
		if (event) event.stopPropagation();
		closeMenu();
		inspectingCustomer = cust;
		inspectModalOpen = true;
		inspectLoading = true;
		inspectData = null;
		try {
			const res = await supermanApi.get<{ data: any }>(`/api/v1/superman/kyc/requests/${cust.id}`);
			inspectData = res.data;
		} catch (err) {
			// If not in requests table yet, fall back to customer detail
			try {
				const resCust = await supermanApi.get<{ data: any }>(`/api/v1/superman/customers/${cust.id}`);
				inspectData = {
					request: {
						customer_name: resCust.data.name,
						email: resCust.data.email,
						phone: resCust.data.phone,
						nid_number: resCust.data.nid_number,
						business_name: resCust.data.company
					},
					front_nid: resCust.data.front_nid,
					back_nid: resCust.data.back_nid,
					live_photo: resCust.data.live_photo,
					trade_license_file: resCust.data.trade_license
				};
			} catch {
				// ignore
			}
		} finally {
			inspectLoading = false;
		}
	}

	async function handleApprove(id: number, name: string, event?: MouseEvent) {
		if (event) event.stopPropagation();
		closeMenu();
		if (!confirm(`Are you sure you want to approve KYC for ${name}?`)) return;
		try {
			await supermanApi.post(`/api/v1/superman/customers/${id}/approve-kyc`);
			inspectModalOpen = false;
			await loadDashboardData();
			alert(`KYC for ${name} has been approved.`);
		} catch (err: unknown) {
			alert(err instanceof Error ? err.message : 'Approval failed.');
		}
	}

	async function handleReject(id: number, name: string, event?: MouseEvent) {
		if (event) event.stopPropagation();
		closeMenu();
		const reason = prompt(`Reason for rejecting KYC for ${name}:`);
		if (!reason) return;
		try {
			await supermanApi.post(`/api/v1/superman/customers/${id}/reject-kyc`, { reason });
			inspectModalOpen = false;
			await loadDashboardData();
			alert(`KYC for ${name} has been rejected.`);
		} catch (err: unknown) {
			alert(err instanceof Error ? err.message : 'Rejection failed.');
		}
	}

	async function handleResubmit(id: number, name: string, event?: MouseEvent) {
		if (event) event.stopPropagation();
		closeMenu();
		const reason = prompt(`Specify what needs correction for ${name}:`);
		if (!reason) return;
		const fields = prompt('Which documents need re-uploading? (e.g. Front NID, Selfie):') ?? 'All';
		try {
			await supermanApi.post(`/api/v1/superman/customers/${id}/resubmit-kyc`, { reason, fields });
			inspectModalOpen = false;
			await loadDashboardData();
			alert(`Resubmission request sent to ${name}.`);
		} catch (err: unknown) {
			alert(err instanceof Error ? err.message : 'Resubmission request failed.');
		}
	}

	function formatDateTime(str: string | null): string {
		if (!str) return '—';
		const d = new Date(str);
		if (isNaN(d.getTime())) return str;
		return d.toLocaleDateString('en-US', {
			month: 'short',
			day: '2-digit',
			year: 'numeric'
		}) + ' ' + d.toTimeString().slice(0, 8);
	}

	onMount(() => {
		void loadDashboardData();
	});

	let sortKey = $state<'kyc_status' | 'submitted_at' | null>(null);
	let sortDir = $state<'asc' | 'desc'>('asc');

	function toggleSort(key: 'kyc_status' | 'submitted_at') {
		if (sortKey === key) {
			sortDir = sortDir === 'asc' ? 'desc' : 'asc';
			return;
		}
		sortKey = key;
		sortDir = 'asc';
	}

	const sortedCustomers = $derived.by(() => {
		if (!sortKey) return customers;
		return [...customers].sort((a, b) => {
			const left = (sortKey === 'kyc_status' ? a.kyc_status : a.submitted_at) ?? '';
			const right = (sortKey === 'kyc_status' ? b.kyc_status : b.submitted_at) ?? '';
			return sortDir === 'asc' ? String(left).localeCompare(String(right)) : String(right).localeCompare(String(left));
		});
	});

	const exportColumns: ExportColumn<KYCCustomer>[] = [
		{ header: 'Customer Name', getValue: (c) => c.full_name },
		{ header: 'Email', getValue: (c) => c.email },
		{ header: 'Mobile', getValue: (c) => c.mobile },
		{ header: 'NID Number', getValue: (c) => c.nid_number || '—' },
		{ header: 'KYC Status', getValue: (c) => c.kyc_status.toUpperCase() },
		{ header: 'Submitted At', getValue: (c) => (c.submitted_at ? new Date(c.submitted_at).toLocaleString() : '—') },
		{ header: 'Verified At', getValue: (c) => (c.verified_at ? new Date(c.verified_at).toLocaleString() : '—') }
	];
</script>
<svelte:window onclick={closeMenu} />

<svelte:head>
	<title>KYC Dashboard — SMS Whiteinfo Administrator</title>
</svelte:head>

<div class="space-y-6">
	<div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
		<h1 class="text-3xl font-bold tracking-tight text-gray-950 dark:text-white">KYC Dashboard</h1>
		<div class="flex items-center gap-2">
			<button
				type="button"
				onclick={() => void loadDashboardData()}
				class="inline-flex items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-3 py-2 text-xs font-semibold text-gray-700 shadow-2xs hover:bg-gray-50 dark:border-gray-800 dark:bg-gray-900 dark:text-gray-300 transition"
			>
				<svg class="h-3.5 w-3.5 {loading ? 'animate-spin' : ''}" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0 3.181 3.183a8.25 8.25 0 0 0 13.803-3.7M4.031 9.865a8.25 8.25 0 0 1 13.803-3.7l3.181 3.182m0-4.991v4.99" />
				</svg>
				Refresh
			</button>
			<ExportDropdown
				title="KYC Dashboard Customers"
				columns={exportColumns}
				getData={() => customers}
				disabled={customers.length === 0}
			/>
		</div>
	</div>

	<!-- 8 KPI metric cards, four per row like the original panel -->
	<div class="grid grid-cols-1 gap-6 sm:grid-cols-2 xl:grid-cols-4">
		<button
			type="button"
			onclick={() => selectStatusFilter('all')}
			class="flex min-h-30 flex-col items-center justify-center rounded-xl border border-gray-200 bg-white px-4 py-7 text-center shadow-2xs transition hover:border-gray-300 dark:border-gray-800 dark:bg-gray-900 {statusFilter === 'all' ? 'ring-2 ring-emerald-500/40' : ''}"
		>
			<span class="text-4xl font-extrabold leading-none text-gray-950 dark:text-white">{stats.total_kyc}</span>
			<span class="mt-2 text-[11px] font-bold uppercase tracking-wider text-gray-500">TOTAL KYC</span>
		</button>

		<button
			type="button"
			onclick={() => selectStatusFilter('pending')}
			class="flex min-h-30 flex-col items-center justify-center rounded-xl border border-gray-200 bg-white px-4 py-7 text-center shadow-2xs transition hover:border-gray-300 dark:border-gray-800 dark:bg-gray-900 {statusFilter === 'pending' ? 'ring-2 ring-amber-500/40' : ''}"
		>
			<span class="text-4xl font-extrabold leading-none text-gray-950 dark:text-white">{stats.pending}</span>
			<span class="mt-2 text-[11px] font-bold uppercase tracking-wider text-gray-500">PENDING</span>
		</button>

		<button
			type="button"
			onclick={() => selectStatusFilter('under_review')}
			class="flex min-h-30 flex-col items-center justify-center rounded-xl border border-amber-200 bg-amber-50/60 px-4 py-7 text-center shadow-2xs transition hover:border-amber-300 dark:border-amber-900/60 dark:bg-amber-950/20 {statusFilter === 'under_review' ? 'ring-2 ring-amber-500' : ''}"
		>
			<span class="text-4xl font-extrabold leading-none text-amber-500">{stats.under_review}</span>
			<span class="mt-2 text-[11px] font-bold uppercase tracking-wider text-amber-600">UNDER REVIEW</span>
		</button>

		<button
			type="button"
			onclick={() => selectStatusFilter('verified')}
			class="flex min-h-30 flex-col items-center justify-center rounded-xl border border-emerald-200 bg-emerald-50/50 px-4 py-7 text-center shadow-2xs transition hover:border-emerald-300 dark:border-emerald-900/60 dark:bg-emerald-950/20 {statusFilter === 'verified' ? 'ring-2 ring-emerald-500' : ''}"
		>
			<span class="text-4xl font-extrabold leading-none text-emerald-600">{stats.verified}</span>
			<span class="mt-2 text-[11px] font-bold uppercase tracking-wider text-emerald-600">VERIFIED</span>
		</button>

		<button
			type="button"
			onclick={() => selectStatusFilter('rejected')}
			class="flex min-h-30 flex-col items-center justify-center rounded-xl border border-rose-200 bg-rose-50/50 px-4 py-7 text-center shadow-2xs transition hover:border-rose-300 dark:border-rose-900/60 dark:bg-rose-950/20 {statusFilter === 'rejected' ? 'ring-2 ring-rose-500/40' : ''}"
		>
			<span class="text-4xl font-extrabold leading-none text-rose-600">{stats.rejected}</span>
			<span class="mt-2 text-[11px] font-bold uppercase tracking-wider text-rose-600">REJECTED</span>
		</button>

		<button
			type="button"
			onclick={() => selectStatusFilter('resubmit_required')}
			class="flex min-h-30 flex-col items-center justify-center rounded-xl border border-blue-200 bg-blue-50/50 px-4 py-7 text-center shadow-2xs transition hover:border-blue-300 dark:border-blue-900/60 dark:bg-blue-950/20 {statusFilter === 'resubmit_required' ? 'ring-2 ring-blue-500/40' : ''}"
		>
			<span class="text-4xl font-extrabold leading-none text-blue-600">{stats.resubmit_required}</span>
			<span class="mt-2 text-[11px] font-bold uppercase tracking-wider text-blue-600">RESUBMIT REQUIRED</span>
		</button>

		<div class="flex min-h-30 flex-col items-center justify-center rounded-xl border border-indigo-200 bg-indigo-50/50 px-4 py-7 text-center shadow-2xs dark:border-indigo-900/60 dark:bg-indigo-950/20">
			<span class="text-4xl font-extrabold leading-none text-indigo-600">{stats.today_submissions}</span>
			<span class="mt-2 text-[11px] font-bold uppercase tracking-wider text-indigo-600">TODAY'S SUBMISSIONS</span>
		</div>

		<div class="flex min-h-30 flex-col items-center justify-center rounded-xl border border-teal-200 bg-teal-50/50 px-4 py-7 text-center shadow-2xs dark:border-teal-900/60 dark:bg-teal-950/20">
			<span class="text-4xl font-extrabold leading-none text-teal-600">{stats.today_approved}</span>
			<span class="mt-2 text-[11px] font-bold uppercase tracking-wider text-teal-600">TODAY'S APPROVED</span>
		</div>
	</div>

	<!-- Records table -->
	<div class="rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
		<div class="flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between">
			<div class="text-xs text-gray-500">
				{#if statusFilter !== 'all'}
					Filtered by <span class="font-semibold text-gray-800 dark:text-gray-200">{statusFilter.replace('_', ' ')}</span>
					<button type="button" onclick={() => selectStatusFilter('all')} class="ml-2 font-semibold text-emerald-600 hover:underline">Clear</button>
				{/if}
			</div>

			<div class="flex items-center justify-end gap-2.5">
				<div class="relative w-full max-w-xs">
					<svg class="absolute left-3 top-2.5 h-3.5 w-3.5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
					</svg>
					<input
						type="search"
						bind:value={search}
						oninput={handleSearchInput}
						placeholder="Search"
						class="w-full rounded-lg border border-gray-200 bg-white py-1.5 pl-8 pr-3 text-xs outline-none focus:border-emerald-500 dark:border-gray-700 dark:bg-gray-800 dark:text-white transition"
					/>
				</div>

				<div class="relative">
					<button
						type="button"
						onclick={() => selectStatusFilter(statusFilter === 'all' ? 'under_review' : 'all')}
						class="rounded-lg border border-gray-200 bg-white p-2 text-gray-500 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-400 transition"
						title="Filter by review status"
					>
						<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" d="M12 3c2.755 0 5.455.232 8.083.678.533.09.917.556.917 1.096v1.044a2.25 2.25 0 0 1-.659 1.591l-5.432 5.432a2.25 2.25 0 0 0-.659 1.591v2.927a2.25 2.25 0 0 1-1.244 2.013L9.75 21v-6.568a2.25 2.25 0 0 0-.659-1.591L3.659 7.409A2.25 2.25 0 0 1 3 5.818V4.774c0-.54.384-1.006.917-1.096A48.32 48.32 0 0 1 12 3Z" />
						</svg>
					</button>
					<span class="absolute -right-1.5 -top-1.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-gray-200 px-1 text-[10px] font-bold text-gray-700 dark:bg-gray-700 dark:text-gray-200">
						{statusFilter === 'all' ? 0 : 1}
					</span>
				</div>

				<button
					type="button"
					class="rounded-lg border border-gray-200 bg-white p-2 text-gray-500 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-400 transition"
					title="Column settings"
				>
					<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M3.75 5.25h16.5M3.75 12h16.5M3.75 18.75h16.5" />
					</svg>
				</button>
			</div>
		</div>

		{#if error}
			<div class="p-6 text-center text-xs text-rose-600">{error}</div>
		{:else}
			<div class="overflow-x-auto">
				<table class="w-full text-left text-xs">
					<thead>
						<tr class="border-y border-gray-100 text-[11px] font-bold uppercase text-gray-700 dark:border-gray-800 dark:text-gray-300">
							<th class="px-5 py-3.5">FULL NAME</th>
							<th class="px-5 py-3.5">EMAIL</th>
							<th class="px-5 py-3.5">MOBILE</th>
							<th class="px-5 py-3.5">NID NUMBER</th>
							<th class="px-5 py-3.5">
								<button type="button" onclick={() => toggleSort('kyc_status')} class="inline-flex items-center gap-1 uppercase hover:text-gray-900 dark:hover:text-white">
									KYC STATUS
									<svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
										<path stroke-linecap="round" stroke-linejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" />
									</svg>
								</button>
							</th>
							<th class="px-5 py-3.5">
								<button type="button" onclick={() => toggleSort('submitted_at')} class="inline-flex items-center gap-1 uppercase hover:text-gray-900 dark:hover:text-white">
									SUBMITTED AT
									<svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
										<path stroke-linecap="round" stroke-linejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" />
									</svg>
								</button>
							</th>
							<th class="px-5 py-3.5"></th>
						</tr>
					</thead>
					<tbody class="divide-y divide-gray-100 dark:divide-gray-800">
						{#if customers.length === 0}
							<tr>
								<td colspan="7" class="px-5 py-12 text-center text-xs text-gray-400">
									No KYC records found matching criteria.
								</td>
							</tr>
						{:else}
							{#each sortedCustomers as cust (cust.id)}
								<tr
									onclick={() => goto(`/superman/users/customers/${cust.id}`)}
									class="cursor-pointer transition-colors hover:bg-gray-50/70 dark:hover:bg-gray-800/40"
								>
									<td class="px-5 py-5 font-bold text-gray-900 dark:text-white">{cust.full_name}</td>
									<td class="px-5 py-5 text-gray-600 dark:text-gray-300">{cust.email}</td>
									<td class="px-5 py-5 font-mono text-gray-700 dark:text-gray-300">{cust.mobile}</td>
									<td class="px-5 py-5 font-mono text-gray-700 dark:text-gray-300">{cust.nid_number ?? '—'}</td>
									<td class="px-5 py-5">
										{#if cust.kyc_status === 'verified'}
											<span class="inline-flex items-center gap-1 rounded-full border border-emerald-300 bg-emerald-50 px-2.5 py-1 text-[11px] font-semibold text-emerald-700 dark:border-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-300">
												<svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke-width="2.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="m4.5 12.75 6 6 9-13.5" /></svg>
												Verified
											</span>
										{:else if cust.kyc_status === 'under_review'}
											<span class="inline-flex items-center gap-1 rounded-full border border-amber-300 bg-amber-50 px-2.5 py-1 text-[11px] font-semibold text-amber-700 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-300">
												<span class="h-1.5 w-1.5 rounded-full bg-amber-500"></span>
												Under Review
											</span>
										{:else if cust.kyc_status === 'resubmit_required'}
											<span class="inline-flex items-center gap-1 rounded-full border border-blue-300 bg-blue-50 px-2.5 py-1 text-[11px] font-semibold text-blue-700 dark:border-blue-800 dark:bg-blue-950/40 dark:text-blue-300">
												<span class="h-1.5 w-1.5 rounded-full bg-blue-500"></span>
												Resubmit Required
											</span>
										{:else if cust.kyc_status === 'rejected'}
											<span class="inline-flex items-center gap-1 rounded-full border border-rose-300 bg-rose-50 px-2.5 py-1 text-[11px] font-semibold text-rose-700 dark:border-rose-800 dark:bg-rose-950/40 dark:text-rose-300">
												<span class="h-1.5 w-1.5 rounded-full bg-rose-500"></span>
												Rejected
											</span>
										{:else}
											<span class="inline-flex items-center gap-1 rounded-full border border-gray-300 bg-gray-50 px-2.5 py-1 text-[11px] font-semibold text-gray-700 dark:border-gray-700 dark:bg-gray-800/40 dark:text-gray-300">
												<span class="h-1.5 w-1.5 rounded-full bg-gray-400"></span>
												Pending
											</span>
										{/if}
									</td>
									<td class="px-5 py-5 font-mono text-[11px] text-gray-500 dark:text-gray-400">
										{formatDateTime(cust.submitted_at ?? cust.verified_at)}
									</td>
									<td class="relative px-5 py-5 text-right">
										<button
											type="button"
											onclick={(e) => toggleMenu(cust.id, e)}
											class="rounded-lg border border-gray-200 p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:border-gray-700 dark:hover:bg-gray-800 dark:hover:text-gray-200 transition"
										>
											<svg class="h-4 w-4" fill="currentColor" viewBox="0 0 24 24">
												<path d="M12 8c1.1 0 2-.9 2-2s-.9-2-2-2-2 .9-2 2 .9 2 2 2zm0 2c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm0 6c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2z" />
											</svg>
										</button>

										{#if activeMenuId === cust.id}
											<div class="absolute right-5 top-14 z-30 w-44 rounded-xl border border-gray-100 bg-white p-1 text-left shadow-xl dark:border-gray-800 dark:bg-gray-900">
												<button
													type="button"
													onclick={(e) => openInspect(cust, e)}
													class="flex w-full items-center gap-2 rounded-lg px-2.5 py-1.5 text-xs text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-gray-800"
												>
													<svg class="h-3.5 w-3.5 text-purple-500" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M2.036 12.322a1.012 1.012 0 0 1 0-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.963-7.178Z" /><path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z" /></svg>
													Inspect Documents
												</button>
												<button
													type="button"
													onclick={() => goto(`/superman/users/customers/${cust.id}`)}
													class="flex w-full items-center gap-2 rounded-lg px-2.5 py-1.5 text-xs text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-gray-800"
												>
													<svg class="h-3.5 w-3.5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M15.75 6a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0ZM4.501 20.118a7.5 7.5 0 0 1 14.998 0A17.933 17.933 0 0 1 12 21.75c-2.676 0-5.216-.584-7.499-1.632Z" /></svg>
													View Customer Profile
												</button>
												<div class="my-1 border-t border-gray-100 dark:border-gray-800"></div>
												{#if cust.kyc_status !== 'verified'}
													<button
														type="button"
														onclick={(e) => handleApprove(cust.id, cust.full_name, e)}
														class="flex w-full items-center gap-2 rounded-lg px-2.5 py-1.5 text-xs text-emerald-600 hover:bg-emerald-50 dark:text-emerald-400 dark:hover:bg-emerald-950/40"
													>
														<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="m4.5 12.75 6 6 9-13.5" /></svg>
														Approve KYC
													</button>
												{/if}
												<button
													type="button"
													onclick={(e) => handleResubmit(cust.id, cust.full_name, e)}
													class="flex w-full items-center gap-2 rounded-lg px-2.5 py-1.5 text-xs text-blue-600 hover:bg-blue-50 dark:text-blue-400 dark:hover:bg-blue-950/40"
												>
													<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0 3.181 3.183a8.25 8.25 0 0 0 13.803-3.7M4.031 9.865a8.25 8.25 0 0 1 13.803-3.7l3.181 3.182m0-4.991v4.99" /></svg>
													Request Resubmit
												</button>
												<button
													type="button"
													onclick={(e) => handleReject(cust.id, cust.full_name, e)}
													class="flex w-full items-center gap-2 rounded-lg px-2.5 py-1.5 text-xs text-rose-600 hover:bg-rose-50 dark:text-rose-400 dark:hover:bg-rose-950/40"
												>
													<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" /></svg>
													Reject KYC
												</button>
											</div>
										{/if}
									</td>
								</tr>
							{/each}
						{/if}
					</tbody>
				</table>
			</div>

			<div class="flex flex-col gap-3 border-t border-gray-100 p-4 text-xs text-gray-500 sm:flex-row sm:items-center sm:justify-between dark:border-gray-800">
				<div>
					Showing {totalRecords === 0 ? 0 : (currentPage - 1) * perPage + 1} to {Math.min(currentPage * perPage, totalRecords)} of {totalRecords} results
				</div>
				<div class="flex items-center gap-4">
					<div class="flex items-center gap-2">
						<span>Per page</span>
						<select
							value={perPage}
							onchange={handlePerPageChange}
							class="rounded-lg border border-gray-200 bg-white py-1 px-2 text-xs text-gray-700 outline-none dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300"
						>
							<option value={10}>10</option>
							<option value={25}>25</option>
							<option value={50}>50</option>
						</select>
					</div>

					<div class="flex items-center gap-1">
						{#each Array.from({ length: totalPages }, (_, i) => i + 1) as p}
							<button
								type="button"
								onclick={() => goToPage(p)}
								class="h-7 w-7 rounded-lg border text-xs font-semibold transition {p === currentPage ? 'border-emerald-600 bg-emerald-600 text-white' : 'border-gray-200 bg-white text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300'}"
							>
								{p}
							</button>
						{/each}
						{#if totalPages > currentPage}
							<button
								type="button"
								onclick={() => goToPage(currentPage + 1)}
								class="h-7 w-7 rounded-lg border border-gray-200 bg-white text-xs font-semibold text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300 transition"
							>
								&gt;
							</button>
						{/if}
					</div>
				</div>
			</div>
		{/if}
	</div>
</div>

<!-- Inspect Documents Modal -->
{#if inspectModalOpen && inspectingCustomer}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4">
		<div class="max-h-[90vh] w-full max-w-2xl space-y-4 overflow-y-auto rounded-2xl border border-gray-100 bg-white p-6 shadow-2xl dark:border-gray-800 dark:bg-gray-900">
			<div class="flex items-center justify-between border-b border-gray-100 pb-3 dark:border-gray-800">
				<div>
					<h3 class="text-base font-bold text-gray-900 dark:text-white">KYC Document Inspector</h3>
					<p class="text-xs text-gray-500">Examine uploaded credentials for {inspectingCustomer.full_name}</p>
				</div>
				<button onclick={() => (inspectModalOpen = false)} class="text-sm text-gray-400 hover:text-gray-600">✕</button>
			</div>

			{#if inspectLoading}
				<div class="py-12 text-center text-xs text-gray-400">Loading document details…</div>
			{:else}
				<div class="grid grid-cols-1 gap-4 text-xs sm:grid-cols-2">
					<div class="space-y-1">
						<span class="block font-medium text-gray-400">Customer</span>
						<span class="font-semibold text-gray-900 dark:text-white">{inspectingCustomer.full_name}</span>
						<div class="text-[11px] text-gray-500">{inspectingCustomer.email} • {inspectingCustomer.mobile}</div>
					</div>
					<div class="space-y-1">
						<span class="block font-medium text-gray-400">NID / Smart Card</span>
						<span class="font-mono font-bold text-gray-800 dark:text-gray-200">{inspectingCustomer.nid_number ?? '—'}</span>
					</div>
				</div>

				<div class="border-t border-gray-100 pt-3 dark:border-gray-800">
					<h4 class="mb-2 text-[10px] font-bold uppercase tracking-wider text-gray-400">Uploaded Verification Files</h4>
					<div class="grid grid-cols-1 gap-3 text-xs sm:grid-cols-2">
						<div class="space-y-2 rounded-xl border border-gray-200 bg-gray-50/50 p-3 dark:border-gray-800 dark:bg-gray-800/40">
							<span class="text-[11px] font-semibold text-gray-900 dark:text-white">Front National ID</span>
							{#if inspectData?.front_nid}
								<img src={inspectData.front_nid} alt="Front NID" class="h-32 w-full rounded-lg border border-gray-200 object-cover dark:border-gray-700" />
							{:else}
								<div class="flex h-32 items-center justify-center rounded-lg border border-dashed border-gray-200 text-xs text-gray-400">No file uploaded</div>
							{/if}
						</div>

						<div class="space-y-2 rounded-xl border border-gray-200 bg-gray-50/50 p-3 dark:border-gray-800 dark:bg-gray-800/40">
							<span class="text-[11px] font-semibold text-gray-900 dark:text-white">Back National ID</span>
							{#if inspectData?.back_nid}
								<img src={inspectData.back_nid} alt="Back NID" class="h-32 w-full rounded-lg border border-gray-200 object-cover dark:border-gray-700" />
							{:else}
								<div class="flex h-32 items-center justify-center rounded-lg border border-dashed border-gray-200 text-xs text-gray-400">No file uploaded</div>
							{/if}
						</div>

						<div class="space-y-2 rounded-xl border border-gray-200 bg-gray-50/50 p-3 dark:border-gray-800 dark:bg-gray-800/40">
							<span class="text-[11px] font-semibold text-gray-900 dark:text-white">Live Facial Photo</span>
							{#if inspectData?.live_photo}
								<img src={inspectData.live_photo} alt="Live Facial Photo" class="h-32 w-full rounded-lg border border-gray-200 object-cover dark:border-gray-700" />
							{:else}
								<div class="flex h-32 items-center justify-center rounded-lg border border-dashed border-gray-200 text-xs text-gray-400">No file uploaded</div>
							{/if}
						</div>

						<div class="space-y-2 rounded-xl border border-gray-200 bg-gray-50/50 p-3 dark:border-gray-800 dark:bg-gray-800/40">
							<span class="text-[11px] font-semibold text-gray-900 dark:text-white">Trade License File</span>
							{#if inspectData?.trade_license_file}
								<img src={inspectData.trade_license_file} alt="Trade License" class="h-32 w-full rounded-lg border border-gray-200 object-cover dark:border-gray-700" />
							{:else}
								<div class="flex h-32 items-center justify-center rounded-lg border border-dashed border-gray-200 text-xs text-gray-400">Optional / None</div>
							{/if}
						</div>
					</div>
				</div>

				<div class="flex flex-wrap items-center justify-between gap-2 border-t border-gray-100 pt-3 dark:border-gray-800">
					<button
						type="button"
						onclick={() => handleResubmit(inspectingCustomer!.id, inspectingCustomer!.full_name)}
						class="rounded-lg bg-amber-500 px-3 py-1.5 text-xs font-bold text-white transition hover:bg-amber-600"
					>
						Request Resubmission
					</button>

					<div class="flex items-center gap-2">
						<button
							type="button"
							onclick={() => handleReject(inspectingCustomer!.id, inspectingCustomer!.full_name)}
							class="rounded-lg border border-rose-300 px-3 py-1.5 text-xs font-bold text-rose-700 transition hover:bg-rose-50 dark:border-rose-800 dark:text-rose-300"
						>
							Reject
						</button>
						<button
							type="button"
							onclick={() => handleApprove(inspectingCustomer!.id, inspectingCustomer!.full_name)}
							class="rounded-lg bg-emerald-600 px-4 py-1.5 text-xs font-bold text-white shadow-xs transition hover:bg-emerald-700"
						>
							Approve KYC
						</button>
					</div>
				</div>
			{/if}
		</div>
	</div>
{/if}
