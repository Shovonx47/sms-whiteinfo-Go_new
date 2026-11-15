<script lang="ts">
	import { api } from '$lib/api.js';
	import { toast } from '$lib/stores/toast.svelte.js';
	import { auth } from '$lib/stores/auth.svelte.js';
	import { onMount } from 'svelte';

	interface Deposit {
		id: number;
		amount: string;
		payment_method: string;
		transaction_id: string;
		status: string;
		paid_at: string | null;
		created_at: string;
	}

	interface Meta {
		total: number;
		per_page: number;
		current_page: number;
	}

	let deposits = $state<Deposit[]>([]);
	let meta = $state<Meta>({ total: 0, per_page: 10, current_page: 1 });
	let loading = $state(true);
	let search = $state('');
	let perPage = $state(10);

	// Make Deposit Modal
	let makeDepositModalOpen = $state(false);
	let depositAmount = $state('');
	let depositing = $state(false);

	// View Deposit Modal
	let selectedDeposit = $state<Deposit | null>(null);

	// Sort state
	let sortColumn = $state('id');
	let sortDirection = $state<'asc' | 'desc'>('desc');

	function toggleSort(col: string) {
		if (sortColumn === col) {
			sortDirection = sortDirection === 'asc' ? 'desc' : 'asc';
		} else {
			sortColumn = col;
			sortDirection = 'desc';
		}
	}

	function formatDateTime(isoDate: string | null): string {
		if (!isoDate) return '—';
		const date = new Date(isoDate);
		if (Number.isNaN(date.getTime())) return '—';
		return date.toLocaleString('en-GB', {
			day: 'numeric',
			month: 'short',
			year: 'numeric',
			hour: '2-digit',
			minute: '2-digit',
			second: '2-digit',
			hour12: false
		});
	}

	function methodLabel(method: string | null): string {
		if (!method) return '—';
		if (method === 'manual') return 'Manual';
		if (method === 'piprapay') return 'PipraPay';
		return method;
	}

	async function load(p = 1) {
		loading = true;
		try {
			const params = new URLSearchParams({ page: String(p), per_page: String(perPage) });
			if (search) params.set('search', search);
			const data = await api.get<{ data: Deposit[]; meta: Meta }>(`/api/v1/customer/deposits?${params}`);
			deposits = data.data || [];
			meta = data.meta || { total: 0, per_page: perPage, current_page: 1 };
		} catch {
			deposits = [];
			meta = { total: 0, per_page: perPage, current_page: 1 };
		} finally {
			loading = false;
		}
	}

	let searchTimer: ReturnType<typeof setTimeout> | null = null;
	function handleSearchInput() {
		if (searchTimer) clearTimeout(searchTimer);
		searchTimer = setTimeout(() => load(1), 300);
	}

	let lastPage = $derived(Math.max(1, (meta as any).last_page ?? Math.ceil((meta.total || 0) / perPage)));

	onMount(() => {
		const params = new URLSearchParams(window.location.search);
		if (params.get('deposit') === '1') {
			depositAmount = '';
			makeDepositModalOpen = true;
		}
		const paymentStatus = params.get('payment_status');
		if (paymentStatus) {
			if (paymentStatus === 'completed') {
				toast.success('Payment Completed', 'Your deposit has been credited to your balance.');
				auth.refresh();
			} else if (paymentStatus === 'pending') {
				toast.info('Payment Pending', 'Payment is being verified. Balance will update shortly.');
			} else if (paymentStatus === 'refunded') {
				toast.info('Deposit Refunded', 'This deposit was refunded.');
			} else if (paymentStatus === 'failed') {
				toast.error('Payment Failed', 'Your payment failed or was cancelled.');
			}
			window.history.replaceState({}, '', window.location.pathname);
		}
		load();
	});

	async function handleMakeDeposit() {
		const amt = parseFloat(depositAmount);
		if (isNaN(amt) || amt < 50 || amt > 100000) {
			toast.error('Invalid Amount', 'Minimum deposit is BDT 50 and maximum is BDT 100,000.');
			return;
		}

		depositing = true;
		try {
			const res = await api.post<{ redirect_url?: string; success: boolean; data?: any }>('/api/v1/customer/deposits', {
				amount: amt,
				gateway: 'piprapay'
			});
			if (res.redirect_url) {
				window.location.href = res.redirect_url;
			} else {
				toast.success('Deposit Initiated', `Deposit of BDT ${amt.toFixed(2)} created.`);
				makeDepositModalOpen = false;
				depositAmount = '';
				load(1);
			}
		} catch (err: unknown) {
			const msg = err instanceof Error ? err.message : 'Failed to initiate deposit.';
			toast.error('Deposit Error', msg);
		} finally {
			depositing = false;
		}
	}

	let filteredDeposits = $derived.by(() => {
		let list = [...deposits];
		if (search) {
			const q = search.toLowerCase();
			list = list.filter(
				(d) =>
					d.transaction_id?.toLowerCase().includes(q) ||
					d.payment_method?.toLowerCase().includes(q) ||
					d.status?.toLowerCase().includes(q) ||
					d.amount?.includes(q)
			);
		}
		list.sort((a, b) => {
			let valA: any = a[sortColumn as keyof Deposit] ?? '';
			let valB: any = b[sortColumn as keyof Deposit] ?? '';
			if (sortColumn === 'amount') {
				valA = parseFloat(valA) || 0;
				valB = parseFloat(valB) || 0;
			}
			if (valA < valB) return sortDirection === 'asc' ? -1 : 1;
			if (valA > valB) return sortDirection === 'asc' ? 1 : -1;
			return 0;
		});
		return list;
	});
</script>

<svelte:head>
	<title>Deposits - White INFO</title>
</svelte:head>

<div class="space-y-6">
	<!-- Breadcrumb -->
	<nav class="flex items-center gap-2 text-xs font-medium text-gray-500">
		<a href="/billing/deposits" class="hover:text-gray-700">Deposits</a>
		<svg class="h-3 w-3 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
			<path stroke-linecap="round" stroke-linejoin="round" d="m8.25 4.5 7.5 7.5-7.5 7.5" />
		</svg>
		<span class="text-gray-900 font-semibold">List</span>
	</nav>

	<!-- Page Heading & Make Deposit button -->
	<div class="flex items-center justify-between">
		<h1 class="text-2xl font-bold tracking-tight text-gray-950">Deposits</h1>
		<button
			type="button"
			onclick={() => { depositAmount = ''; makeDepositModalOpen = true; }}
			class="inline-flex items-center gap-1.5 rounded-lg border border-teal-600 bg-white px-3.5 py-2 text-sm font-semibold text-teal-600 shadow-sm transition hover:bg-teal-50 cursor-pointer"
		>
			<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
			</svg>
			Top Up
		</button>
	</div>

	<!-- Main Card (White rounded with shadow) -->
	<div class="rounded-xl border border-gray-200 bg-white shadow-sm overflow-hidden">
		<!-- Toolbar: Search + Column Toggle -->
		<div class="flex items-center justify-end gap-2 border-b border-gray-100 p-3 bg-white">
			<div class="relative w-64">
				<div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-2.5">
					<svg class="h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
					</svg>
				</div>
				<input
					type="text"
					bind:value={search}
					oninput={handleSearchInput}
					placeholder="Search"
					class="w-full rounded-lg border border-gray-300 bg-white py-1.5 pr-3 pl-8 text-xs text-gray-900 placeholder-gray-400 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600"
				/>
			</div>
			<button
				type="button"
				class="rounded-lg border border-gray-300 p-1.5 text-gray-500 hover:bg-gray-50 hover:text-gray-700 cursor-pointer"
				title="Toggle columns"
			>
				<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="M9 4.5v15m6-15v15m-10.5-15h15a2.25 2.25 0 0 1 2.25 2.25v10.5a2.25 2.25 0 0 1-2.25 2.25h-15a2.25 2.25 0 0 1-2.25-2.25V6.75A2.25 2.25 0 0 1 4.5 4.5Z" />
				</svg>
			</button>
		</div>

		<!-- Table -->
		<div class="overflow-x-auto">
			<table class="w-full text-left border-collapse text-xs">
				<thead>
					<tr class="border-b border-gray-200 bg-gray-50/70 text-gray-600 font-semibold tracking-wider uppercase text-[11px]">
						<th class="py-3 px-4 cursor-pointer select-none hover:text-gray-900" onclick={() => toggleSort('amount')}>
							<div class="flex items-center gap-1">
								<span>AMOUNT</span>
								<svg class="h-3.5 w-3.5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
									<path stroke-linecap="round" stroke-linejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" />
								</svg>
							</div>
						</th>
						<th class="py-3 px-4 cursor-pointer select-none hover:text-gray-900" onclick={() => toggleSort('payment_method')}>
							<div class="flex items-center gap-1">
								<span>PAYMENT METHOD</span>
								<svg class="h-3.5 w-3.5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
									<path stroke-linecap="round" stroke-linejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" />
								</svg>
							</div>
						</th>
						<th class="py-3 px-4 font-semibold text-gray-600">TRX ID</th>
						<th class="py-3 px-4 cursor-pointer select-none hover:text-gray-900" onclick={() => toggleSort('status')}>
							<div class="flex items-center gap-1">
								<span>STATUS</span>
								<svg class="h-3.5 w-3.5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
									<path stroke-linecap="round" stroke-linejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" />
								</svg>
							</div>
						</th>
						<th class="py-3 px-4 font-semibold text-gray-600">PAID AT</th>
						<th class="py-3 px-4 font-semibold text-gray-600">CREATED AT</th>
						<th class="py-3 px-4 text-right"></th>
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-100 bg-white">
					{#if loading}
						<tr>
							<td colspan="7" class="py-12 text-center text-gray-500">
								<div class="inline-block h-6 w-6 animate-spin rounded-full border-2 border-gray-300 border-t-teal-600"></div>
								<p class="mt-2 text-xs">Loading deposits...</p>
							</td>
						</tr>
					{:else if filteredDeposits.length === 0}
						<tr>
							<td colspan="7" class="py-12 text-center text-gray-500">
								<p class="text-sm font-medium text-gray-700">No deposits found</p>
								<p class="mt-1 text-xs text-gray-400">Click "Top Up" above to recharge your account balance.</p>
							</td>
						</tr>
					{:else}
						{#each filteredDeposits as deposit (deposit.id)}
							<tr class="hover:bg-gray-50/60 transition">
								<td class="py-3 px-4 font-medium text-gray-900 whitespace-nowrap">
									BDT {Number(deposit.amount).toFixed(2)}
								</td>
								<td class="py-3 px-4 whitespace-nowrap">
									<span class="inline-flex items-center gap-1.5 rounded-full bg-teal-50 px-2.5 py-0.5 text-xs font-medium text-teal-700 border border-teal-100">
										<span class="h-1.5 w-1.5 rounded-full bg-teal-600"></span>
										{methodLabel(deposit.payment_method)}
									</span>
								</td>
								<td class="py-3 px-4 font-mono text-gray-700 whitespace-nowrap text-xs">
									{deposit.transaction_id || '—'}
								</td>
								<td class="py-3 px-4 whitespace-nowrap">
									{#if deposit.status === 'completed'}
										<span class="inline-flex items-center gap-1 rounded-full bg-emerald-50 px-2.5 py-0.5 text-xs font-medium text-emerald-700 border border-emerald-100">
											<svg class="h-3 w-3 text-emerald-600" fill="none" viewBox="0 0 24 24" stroke-width="2.5" stroke="currentColor">
												<path stroke-linecap="round" stroke-linejoin="round" d="m4.5 12.75 6 6 9-13.5" />
											</svg>
											Completed
										</span>
									{:else if deposit.status === 'pending'}
										<span class="inline-flex items-center gap-1 rounded-full bg-amber-50 px-2.5 py-0.5 text-xs font-medium text-amber-700 border border-amber-100">
											<svg class="h-3 w-3 text-amber-600" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
												<path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
											</svg>
											Pending
										</span>
									{:else}
										<span class="inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-xs font-medium border {deposit.status === 'refunded' ? 'bg-gray-50 text-gray-700 border-gray-200' : 'bg-red-50 text-red-700 border-red-100'}">
											<svg class="h-3 w-3 {deposit.status === 'refunded' ? 'text-gray-500' : 'text-red-600'}" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
												<path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" />
											</svg>
											{deposit.status.charAt(0).toUpperCase() + deposit.status.slice(1)}
										</span>
									{/if}
								</td>
								<td class="py-3 px-4 text-gray-600 whitespace-nowrap">
									{deposit.paid_at ? formatDateTime(deposit.paid_at) : '—'}
								</td>
								<td class="py-3 px-4 text-gray-600 whitespace-nowrap">
									{formatDateTime(deposit.created_at)}
								</td>
								<td class="py-3 px-4 text-right whitespace-nowrap">
									<button
										type="button"
										onclick={() => (selectedDeposit = deposit)}
										class="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-700 cursor-pointer"
										title="View actions"
									>
										<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
											<path stroke-linecap="round" stroke-linejoin="round" d="M12 6.75a.75.75 0 1 1 0-1.5.75.75 0 0 1 0 1.5ZM12 12.75a.75.75 0 1 1 0-1.5.75.75 0 0 1 0 1.5ZM12 18.75a.75.75 0 1 1 0-1.5.75.75 0 0 1 0 1.5Z" />
										</svg>
									</button>
								</td>
							</tr>
						{/each}
					{/if}
				</tbody>
			</table>
		</div>

		<!-- Filament Pagination Footer -->
		<div class="flex items-center justify-between border-t border-gray-200 px-4 py-3 bg-white text-xs text-gray-600">
			<div>
				Showing {filteredDeposits.length > 0 ? (meta.current_page - 1) * perPage + 1 : 0} to {Math.min(meta.current_page * perPage, meta.total || filteredDeposits.length)} of {meta.total || filteredDeposits.length} results
			</div>
			<div class="flex items-center gap-3">
				<div class="flex items-center gap-2">
					<span class="text-xs text-gray-500">Per page</span>
					<select
						bind:value={perPage}
						onchange={() => load(1)}
						class="rounded-lg border border-gray-300 bg-white py-1 px-2 text-xs text-gray-700 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600"
					>
						<option value={10}>10</option>
						<option value={25}>25</option>
						<option value={50}>50</option>
					</select>
				</div>
				{#if lastPage > 1}
					<div class="flex items-center gap-1">
						<button
							type="button"
							disabled={meta.current_page <= 1}
							onclick={() => load(meta.current_page - 1)}
							class="rounded-lg border border-gray-300 bg-white px-2.5 py-1 text-xs font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-40 cursor-pointer"
						>
							Previous
						</button>
						<span class="px-2 text-xs text-gray-500">Page {meta.current_page} / {lastPage}</span>
						<button
							type="button"
							disabled={meta.current_page >= lastPage}
							onclick={() => load(meta.current_page + 1)}
							class="rounded-lg border border-gray-300 bg-white px-2.5 py-1 text-xs font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-40 cursor-pointer"
						>
							Next
						</button>
					</div>
				{/if}
			</div>
		</div>
	</div>
</div>

<!-- Add Balance Modal (Matching Screenshot 8) -->
{#if makeDepositModalOpen}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 animate-fade-in backdrop-blur-xs">
		<div class="relative w-full max-w-md rounded-2xl bg-white p-6 shadow-xl border border-gray-100">
			<!-- Header with Taka Icon -->
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

			<!-- Form -->
			<form onsubmit={(e) => { e.preventDefault(); handleMakeDeposit(); }} class="mt-6 space-y-4">
				<div>
					<label for="deposit-amount-input" class="block text-xs font-semibold text-gray-700">Amount<span class="text-red-500">*</span></label>
					<div class="mt-1.5 flex rounded-lg border border-gray-300 overflow-hidden shadow-xs focus-within:border-teal-600 focus-within:ring-1 focus-within:ring-teal-600">
						<span class="inline-flex items-center px-3 bg-gray-50 text-xs font-semibold text-gray-500 border-r border-gray-300">
							BDT
						</span>
						<input
							id="deposit-amount-input"
							type="number"
							min="50"
							max="100000"
							step="1"
							bind:value={depositAmount}
							placeholder="Enter amount to deposit"
							required
							class="w-full py-2 px-3 text-sm text-gray-900 placeholder-gray-400 focus:outline-none"
						/>
					</div>
				</div>

				<!-- Actions -->
				<div class="mt-6 flex items-center justify-end gap-3 pt-2">
					<button
						type="button"
						onclick={() => (makeDepositModalOpen = false)}
						class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-xs font-semibold text-gray-700 hover:bg-gray-50 transition cursor-pointer"
					>
						Cancel
					</button>
					<button
						type="submit"
						disabled={depositing}
						class="rounded-lg bg-teal-600 px-4 py-2 text-xs font-semibold text-white shadow-sm hover:bg-teal-700 transition disabled:opacity-50 cursor-pointer"
					>
						{depositing ? 'Processing...' : 'Add Balance'}
					</button>
				</div>
			</form>
		</div>
	</div>
{/if}

<!-- View Details Modal -->
{#if selectedDeposit}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 animate-fade-in backdrop-blur-xs">
		<div class="relative w-full max-w-lg rounded-2xl bg-white p-6 shadow-xl border border-gray-100">
			<div class="flex items-center justify-between pb-3 border-b border-gray-200">
				<h3 class="text-base font-bold text-gray-900">Deposit Details</h3>
				<button
					type="button"
					onclick={() => (selectedDeposit = null)}
					class="text-gray-400 hover:text-gray-600 cursor-pointer"
				>
					<svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" />
					</svg>
				</button>
			</div>

			<div class="mt-4 space-y-3 text-xs">
				<div class="flex justify-between py-1 border-b border-gray-50">
					<span class="text-gray-500 font-medium">Amount</span>
					<span class="font-semibold text-gray-900">BDT {Number(selectedDeposit.amount).toFixed(2)}</span>
				</div>
				<div class="flex justify-between py-1 border-b border-gray-50">
					<span class="text-gray-500 font-medium">Payment Method</span>
					<span class="text-gray-900">{methodLabel(selectedDeposit.payment_method)}</span>
				</div>
				<div class="flex justify-between py-1 border-b border-gray-50">
					<span class="text-gray-500 font-medium">Status</span>
					<span class="font-semibold capitalize text-teal-700">{selectedDeposit.status}</span>
				</div>
				<div class="flex justify-between py-1 border-b border-gray-50">
					<span class="text-gray-500 font-medium">Paid At</span>
					<span class="text-gray-700">{selectedDeposit.paid_at ? new Date(selectedDeposit.paid_at).toLocaleString() : 'Not paid yet'}</span>
				</div>
			</div>

			<div class="mt-6 flex justify-end">
				<button
					type="button"
					onclick={() => (selectedDeposit = null)}
					class="rounded-lg bg-gray-100 px-4 py-2 text-xs font-semibold text-gray-700 hover:bg-gray-200 transition cursor-pointer"
				>
					Close
				</button>
			</div>
		</div>
	</div>
{/if}
