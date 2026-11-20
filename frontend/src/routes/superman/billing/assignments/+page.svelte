<script lang="ts">
	import { onMount } from 'svelte';
	import { supermanApi } from '$lib/superman-api.js';

	interface TariffPlan {
		id: number;
		name: string;
		type: string;
		status: string;
	}

	interface CustomerItem {
		user_id: number;
		customer_name: string;
		customer_email: string;
		customer_phone: string;
		plan_id: number | null;
		plan_name: string | null;
		plan_type: string | null;
		effective_from: string | null;
		assigned_by: string | null;
		status: string | null;
	}

	interface AssignmentHistory {
		id: number;
		plan_name: string;
		plan_type: string;
		status: string;
		effective_from: string;
		assigned_by: string | null;
		created_at: string;
		notes: string | null;
	}

	let customers = $state<CustomerItem[]>([]);
	let activePlans = $state<TariffPlan[]>([]);
	let loading = $state(true);

	// Currently selected customer for assignment / details
	let selectedCustomerId = $state<number | null>(null);
	let selectedCustomer = $derived(customers.find((c) => c.user_id === selectedCustomerId) || null);

	let assignmentHistory = $state<AssignmentHistory[]>([]);
	let historyLoading = $state(false);

	// Form inputs for new assignment
	let newPlanId = $state<number | ''>('');
	let effectiveFrom = $state(new Date().toISOString().slice(0, 10));
	let assignmentNotes = $state('');
	let submitting = $state(false);
	let successMsg = $state<string | null>(null);
	let errorMsg = $state<string | null>(null);

	async function loadInitial() {
		loading = true;
		try {
			const [custRes, plansRes] = await Promise.all([
				supermanApi.get<{ data: CustomerItem[] }>('/api/v1/superman/billing/assignments'),
				supermanApi.get<{ data: TariffPlan[] }>('/api/v1/superman/billing/tariff-plans')
			]);
			customers = custRes.data || [];
			activePlans = (plansRes.data || []).filter((p) => p.status === 'active');
		} catch (err) {
			console.error('Failed to load assignments data:', err);
		} finally {
			loading = false;
		}
	}

	async function selectCustomer(userId: number) {
		selectedCustomerId = userId;
		newPlanId = '';
		effectiveFrom = new Date().toISOString().slice(0, 10);
		assignmentNotes = '';
		successMsg = null;
		errorMsg = null;

		historyLoading = true;
		try {
			const res = await supermanApi.get<{ data: AssignmentHistory[] }>(
				`/api/v1/superman/billing/assignments/${userId}`
			);
			assignmentHistory = res.data || [];
		} catch (err) {
			console.error('Failed to load assignment history:', err);
		} finally {
			historyLoading = false;
		}
	}

	async function reloadCustomers() {
		const res = await supermanApi.get<{ data: CustomerItem[] }>('/api/v1/superman/billing/assignments');
		customers = res.data || [];
	}

	async function handleAssignPlan(e: SubmitEvent) {
		e.preventDefault();
		if (!selectedCustomerId || !newPlanId) {
			errorMsg = 'Please select a tariff plan to assign.';
			return;
		}

		submitting = true;
		errorMsg = null;
		successMsg = null;
		try {
			const res = await supermanApi.post<{ message: string }>('/api/v1/superman/billing/assignments', {
				user_id: selectedCustomerId,
				tariff_plan_id: Number(newPlanId),
				effective_from: effectiveFrom,
				notes: assignmentNotes.trim() || null
			});

			successMsg = res.message;
			assignmentNotes = '';
			newPlanId = '';

			await Promise.all([selectCustomer(selectedCustomerId), reloadCustomers()]);
		} catch (err: unknown) {
			errorMsg = err instanceof Error ? err.message : 'Failed to assign tariff plan.';
		} finally {
			submitting = false;
		}
	}

	onMount(() => {
		loadInitial();
	});

	function formatDate(value: string | null | undefined) {
		if (!value) return '—';
		const raw = value.includes('T') ? value.slice(0, 10) : value;
		const parts = raw.split('-');
		if (parts.length === 3) {
			const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
			const month = Number(parts[1]) - 1;
			if (month >= 0 && month < 12) {
				return `${Number(parts[2])} ${months[month]} ${parts[0]}`;
			}
		}
		return value;
	}
</script>

<svelte:head>
	<title>User-wise Tariff Assignment — SMS Whiteinfo Administrator</title>
</svelte:head>

<div class="space-y-5">
	<h1 class="text-2xl font-bold tracking-tight text-gray-950 dark:text-white">User-wise Tariff Assignment</h1>

	<!-- Select Customer -->
	<div class="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
		<div class="border-b border-gray-100 px-5 py-4 dark:border-gray-800">
			<h2 class="text-base font-bold text-gray-900 dark:text-white">Select Customer</h2>
			<p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
				Search for a customer to view and manage their tariff plan assignment.
			</p>
		</div>

		<div class="p-5">
			<select
				value={selectedCustomerId ?? ''}
				onchange={(e) => {
					const val = (e.currentTarget as HTMLSelectElement).value;
					if (val) void selectCustomer(Number(val));
					else selectedCustomerId = null;
				}}
				class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2.5 text-sm font-medium text-gray-900 focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500 focus:outline-hidden dark:border-gray-700 dark:bg-gray-800 dark:text-white"
			>
				<option value="">
					{loading ? 'Loading customers…' : '— Select a customer —'}
				</option>
				{#each customers as c (c.user_id)}
					<option value={c.user_id}>
						{c.customer_name}{c.customer_email ? ` (${c.customer_email})` : ''}
					</option>
				{/each}
			</select>
		</div>
	</div>

	{#if selectedCustomer}
		<!-- Current Tariff Plan -->
		<div class="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
			<div class="border-b border-gray-100 px-5 py-4 dark:border-gray-800">
				<h2 class="text-base font-bold text-gray-900 dark:text-white">Current Tariff Plan</h2>
			</div>

			<div class="grid grid-cols-1 gap-6 p-5 sm:grid-cols-3">
				<div>
					<p class="text-xs text-gray-500 dark:text-gray-400">Plan Name</p>
					<p class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">
						{selectedCustomer.plan_name || 'Default Legacy'}
					</p>
				</div>
				<div>
					<p class="text-xs text-gray-500 dark:text-gray-400">Plan Type</p>
					<p class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">
						{selectedCustomer.plan_type || 'Standard'}
					</p>
				</div>
				<div>
					<p class="text-xs text-gray-500 dark:text-gray-400">Effective From</p>
					<p class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">
						{formatDate(selectedCustomer.effective_from)}
					</p>
				</div>
			</div>
		</div>

		<!-- Assign / Change Plan -->
		<div class="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
			<div class="border-b border-gray-100 px-5 py-4 dark:border-gray-800">
				<h2 class="text-base font-bold text-gray-900 dark:text-white">Assign / Change Plan</h2>
				<p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
					Selecting a new plan will immediately supersede the current one. The effective date controls when billing switches.
				</p>
			</div>

			<form onsubmit={handleAssignPlan} class="p-5">
				{#if successMsg}
					<div class="mb-4 rounded-lg bg-emerald-50 px-3 py-2 text-xs font-semibold text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300">
						{successMsg}
					</div>
				{/if}
				{#if errorMsg}
					<div class="mb-4 rounded-lg bg-rose-50 px-3 py-2 text-xs font-semibold text-rose-800 dark:bg-rose-950 dark:text-rose-300">
						{errorMsg}
					</div>
				{/if}

				<div class="grid grid-cols-1 gap-4 lg:grid-cols-3">
					<div>
						<label for="new-plan" class="mb-1 block text-xs font-semibold text-gray-700 dark:text-gray-300">
							New Tariff Plan <span class="text-rose-500">*</span>
						</label>
						<select
							id="new-plan"
							bind:value={newPlanId}
							required
							class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm font-medium text-gray-900 focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500 focus:outline-hidden dark:border-gray-700 dark:bg-gray-800 dark:text-white"
						>
							<option value="">— Select a plan —</option>
							{#each activePlans as p (p.id)}
								<option value={p.id}>{p.name} ({p.type})</option>
							{/each}
						</select>
					</div>

					<div>
						<label for="effective-from" class="mb-1 block text-xs font-semibold text-gray-700 dark:text-gray-300">
							Effective From <span class="text-rose-500">*</span>
						</label>
						<input
							id="effective-from"
							type="date"
							bind:value={effectiveFrom}
							required
							class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm font-mono text-gray-900 focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500 focus:outline-hidden dark:border-gray-700 dark:bg-gray-800 dark:text-white"
						/>
					</div>

					<div>
						<label for="assign-notes" class="mb-1 block text-xs font-semibold text-gray-700 dark:text-gray-300">
							Notes (optional)
						</label>
						<input
							id="assign-notes"
							type="text"
							bind:value={assignmentNotes}
							placeholder="e.g. Upgraded to standard plan per request"
							class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500 focus:outline-hidden dark:border-gray-700 dark:bg-gray-800 dark:text-white"
						/>
					</div>
				</div>

				<div class="mt-4">
					<button
						type="submit"
						disabled={submitting || !newPlanId}
						class="inline-flex items-center gap-2 rounded-lg bg-emerald-500 px-4 py-2 text-sm font-semibold text-white shadow-xs transition hover:bg-emerald-600 disabled:cursor-not-allowed disabled:opacity-50"
					>
						<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
						</svg>
						{submitting ? 'Assigning…' : 'Assign Plan'}
					</button>
				</div>
			</form>
		</div>

		<!-- Assignment History -->
		<div class="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
			<div class="border-b border-gray-100 px-5 py-4 dark:border-gray-800">
				<h2 class="text-base font-bold text-gray-900 dark:text-white">Assignment History</h2>
				<p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
					All past and current plan assignments for this customer.
				</p>
			</div>

			<div class="overflow-x-auto">
				<table class="w-full text-left text-xs">
					<thead>
						<tr class="border-b border-gray-100 text-[10px] font-semibold uppercase text-gray-400 dark:border-gray-800">
							<th class="px-5 py-3">Plan</th>
							<th class="px-4 py-3">Status</th>
							<th class="px-4 py-3">Effective From</th>
							<th class="px-4 py-3">Assigned By</th>
							<th class="px-5 py-3">Notes</th>
						</tr>
					</thead>
					<tbody class="divide-y divide-gray-100 dark:divide-gray-800">
						{#each assignmentHistory as h (h.id)}
							<tr class="hover:bg-gray-50/50 dark:hover:bg-gray-800/40">
								<td class="px-5 py-3">
									<p class="font-semibold text-gray-900 dark:text-white">{h.plan_name}</p>
									<p class="text-[10px] text-gray-400">{h.plan_type}</p>
								</td>
								<td class="px-4 py-3">
									<span class="inline-flex rounded-full px-2 py-0.5 text-[10px] font-semibold capitalize {h.status === 'active'
										? 'bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300'
										: 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400'}">
										{h.status}
									</span>
								</td>
								<td class="px-4 py-3 text-gray-700 dark:text-gray-300">{formatDate(h.effective_from)}</td>
								<td class="px-4 py-3 text-gray-600 dark:text-gray-400">{h.assigned_by || 'System'}</td>
								<td class="px-5 py-3 text-gray-500 dark:text-gray-400">{h.notes || '—'}</td>
							</tr>
						{:else}
							<tr>
								<td colspan="5" class="px-5 py-10 text-center text-xs text-gray-400">
									{historyLoading ? 'Loading assignment history…' : 'No plan assignments recorded for this customer yet.'}
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		</div>
	{/if}
</div>
