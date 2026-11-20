<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { supermanApi } from '$lib/superman-api.js';

	interface DepositDetail {
		id: number;
		user_id: number;
		customer: string;
		customer_email: string;
		customer_phone: string;
		company: string;
		amount: number;
		currency: string;
		payment_method: string;
		transaction_id: string | null;
		deposited_by: string | null;
		status: string;
		paid_at: string | null;
		created_at: string;
		gateway_name: string;
		gateway_response: string;
	}

	let deposit = $state<DepositDetail | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let actionLoading = $state(false);
	let notice = $state<{ type: 'success' | 'error'; text: string } | null>(null);
	let showGatewayResponse = $state(false);

	// Edit modal
	let editing = $state(false);
	let editSaving = $state(false);
	let editError = $state<string | null>(null);
	let editAmount = $state(0);
	let editCurrency = $state('BDT');
	let editMethod = $state('manual');
	let editTransactionId = $state('');
	let editStatus = $state('pending');
	let editPaidAt = $state('');

	const depositId = $derived($page.params.id);

	async function loadDeposit() {
		loading = true;
		error = null;
		try {
			const res = await supermanApi.get<{ data: DepositDetail }>(`/api/v1/superman/deposits/${depositId}`);
			deposit = res.data;
		} catch (err: unknown) {
			error = err instanceof Error ? err.message : 'Could not load this deposit.';
		} finally {
			loading = false;
		}
	}

	async function act(action: 'approve' | 'reject') {
		if (!deposit) return;
		const verb = action === 'approve' ? 'Approve' : 'Reject';
		const detail =
			action === 'approve'
				? 'This credits the customer balance immediately.'
				: 'The customer will not be credited.';
		if (!window.confirm(`${verb} deposit of ${formatCurrency(deposit.amount)} for ${deposit.customer}? ${detail}`)) {
			return;
		}

		actionLoading = true;
		notice = null;
		try {
			await supermanApi.post(`/api/v1/superman/deposits/${deposit.id}/${action}`, {});
			notice = { type: 'success', text: `Deposit ${action === 'approve' ? 'approved' : 'rejected'} successfully.` };
			await loadDeposit();
		} catch (err: unknown) {
			notice = { type: 'error', text: err instanceof Error ? err.message : `Failed to ${action} deposit.` };
		} finally {
			actionLoading = false;
		}
	}

	function toDatetimeLocal(value: string): string {
		const parsed = new Date(value);
		if (Number.isNaN(parsed.getTime())) return '';
		const pad = (input: number) => String(input).padStart(2, '0');
		return `${parsed.getFullYear()}-${pad(parsed.getMonth() + 1)}-${pad(parsed.getDate())}T${pad(parsed.getHours())}:${pad(parsed.getMinutes())}`;
	}

	function openEdit() {
		if (!deposit) return;
		editAmount = deposit.amount;
		editCurrency = deposit.currency || 'BDT';
		editMethod = (deposit.payment_method || 'manual').toLowerCase();
		editTransactionId = deposit.transaction_id || '';
		editStatus = deposit.status;
		editPaidAt = deposit.paid_at ? toDatetimeLocal(deposit.paid_at) : '';
		editError = null;
		editing = true;
	}

	async function saveEdit() {
		if (!deposit) return;

		if (!(Number(editAmount) > 0)) {
			editError = 'Amount must be greater than zero.';
			return;
		}

		editSaving = true;
		editError = null;
		try {
			await supermanApi.put(`/api/v1/superman/deposits/${deposit.id}`, {
				amount: Number(editAmount),
				currency: editCurrency.trim().toUpperCase(),
				payment_method: editMethod,
				transaction_id: editTransactionId.trim(),
				status: editStatus,
				paid_at: editPaidAt ? new Date(editPaidAt).toISOString() : ''
			});
			editing = false;
			notice = { type: 'success', text: 'Deposit updated.' };
			await loadDeposit();
		} catch (err: unknown) {
			editError = err instanceof Error ? err.message : 'Could not save this deposit.';
		} finally {
			editSaving = false;
		}
	}

	onMount(() => {
		void loadDeposit();
	});

	function formatCurrency(val: number | null | undefined): string {
		return '৳' + (val || 0).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
	}

	function formatDateTime(value: string | null): string {
		if (!value) return '—';
		return new Date(value).toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' });
	}

	function methodLabel(value: string | null): string {
		const raw = (value || 'manual').toLowerCase();
		if (raw.includes('pipra')) return 'PipraPay';
		if (raw === 'manual' || raw === 'admin' || raw === 'bank') return 'Manual';
		return raw.charAt(0).toUpperCase() + raw.slice(1);
	}

	function prettyGatewayPayload(payload: string): string {
		try {
			return JSON.stringify(JSON.parse(payload), null, 2);
		} catch {
			return payload;
		}
	}

	const reference = $derived(deposit?.transaction_id || (deposit ? `#${deposit.id}` : ''));
</script>

<svelte:head>
	<title>Deposit {reference} — SMS Whiteinfo Administrator</title>
</svelte:head>

<div class="space-y-5">
	<!-- Breadcrumb -->
	<nav class="flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
		<a href="/superman/billing/deposits" class="font-medium hover:text-emerald-600 dark:hover:text-emerald-400">Deposits</a>
		<span>›</span>
		<span class="font-mono">{reference || '…'}</span>
		<span>›</span>
		<span class="font-semibold text-gray-900 dark:text-white">View</span>
	</nav>

	{#if error}
		<div class="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-xs font-semibold text-rose-700 dark:border-rose-900 dark:bg-rose-950/40 dark:text-rose-300">
			{error}
		</div>
	{/if}

	{#if notice}
		<div class="rounded-lg border px-4 py-2 text-xs font-semibold {notice.type === 'success'
			? 'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-300'
			: 'border-rose-200 bg-rose-50 text-rose-800 dark:border-rose-900 dark:bg-rose-950/40 dark:text-rose-300'}">
			{notice.text}
		</div>
	{/if}

	{#if loading && !deposit}
		<div class="rounded-xl border border-gray-200 bg-white p-10 text-center text-xs text-gray-400 dark:border-gray-800 dark:bg-gray-900">
			Loading deposit…
		</div>
	{:else if deposit}
		<!-- Header -->
		<div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
			<h1 class="text-2xl font-bold tracking-tight text-gray-950 dark:text-white">
				View {reference}
			</h1>
			<div class="flex items-center gap-2">
				<a
					href="/superman/billing/deposits"
					class="inline-flex items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-3 py-2 text-xs font-semibold text-gray-700 hover:bg-gray-50 dark:border-gray-800 dark:bg-gray-900 dark:text-gray-300 dark:hover:bg-gray-800"
				>
					Back to list
				</a>
				<button
					type="button"
					onclick={openEdit}
					class="inline-flex items-center gap-1.5 rounded-lg border border-amber-400 bg-white px-3.5 py-2 text-xs font-semibold text-amber-600 transition hover:bg-amber-50 dark:border-amber-500/60 dark:bg-gray-900 dark:text-amber-400 dark:hover:bg-amber-950/30"
				>
					<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L10.582 16.07a4.5 4.5 0 0 1-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 0 1 1.13-1.897l8.932-8.931Zm0 0L19.5 7.125" />
					</svg>
					Edit
				</button>
				{#if deposit.status === 'pending'}
					<button
						type="button"
						onclick={() => act('approve')}
						disabled={actionLoading}
						class="rounded-lg bg-emerald-600 px-3.5 py-2 text-xs font-semibold text-white shadow-xs hover:bg-emerald-500 disabled:opacity-50"
					>
						{actionLoading ? 'Working…' : 'Approve'}
					</button>
					<button
						type="button"
						onclick={() => act('reject')}
						disabled={actionLoading}
						class="rounded-lg border border-rose-200 bg-white px-3.5 py-2 text-xs font-semibold text-rose-600 hover:bg-rose-50 disabled:opacity-50 dark:border-rose-900/40 dark:bg-gray-900 dark:text-rose-400"
					>
						Reject
					</button>
				{/if}
			</div>
		</div>

		<!-- Cards -->
		<div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
			<!-- Customer -->
			<div class="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
				<div class="flex items-start gap-3 border-b border-gray-100 px-5 py-4 dark:border-gray-800">
					<svg class="mt-0.5 h-5 w-5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M15.75 6a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0ZM4.501 20.118a7.5 7.5 0 0 1 14.998 0A17.933 17.933 0 0 1 12 21.75c-2.676 0-5.216-.584-7.499-1.632Z" />
					</svg>
					<div>
						<h2 class="text-base font-bold text-gray-900 dark:text-white">Customer</h2>
						<p class="text-xs text-gray-500 dark:text-gray-400">Customer who made this deposit</p>
					</div>
				</div>
				<div class="grid grid-cols-1 gap-5 px-5 py-5 sm:grid-cols-2">
					<div>
						<p class="text-xs text-gray-500 dark:text-gray-400">Name</p>
						<p class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">{deposit.customer}</p>
					</div>
					<div>
						<p class="text-xs text-gray-500 dark:text-gray-400">Email</p>
						<p class="mt-1 text-sm text-gray-800 dark:text-gray-200">{deposit.customer_email || '—'}</p>
					</div>
					<div>
						<p class="text-xs text-gray-500 dark:text-gray-400">Phone</p>
						<p class="mt-1 text-sm text-gray-800 dark:text-gray-200">{deposit.customer_phone || '—'}</p>
					</div>
					<div>
						<p class="text-xs text-gray-500 dark:text-gray-400">Company</p>
						<p class="mt-1 text-sm text-gray-800 dark:text-gray-200">{deposit.company || '—'}</p>
					</div>
				</div>
			</div>

			<!-- Payment -->
			<div class="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
				<div class="flex items-start gap-3 border-b border-gray-100 px-5 py-4 dark:border-gray-800">
					<svg class="mt-0.5 h-5 w-5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M2.25 8.25h19.5M2.25 9h19.5m-16.5 5.25h6m-6 2.25h3m-3.75 3h15a2.25 2.25 0 0 0 2.25-2.25V6.75A2.25 2.25 0 0 0 19.5 4.5h-15a2.25 2.25 0 0 0-2.25 2.25v10.5A2.25 2.25 0 0 0 4.5 19.5Z" />
					</svg>
					<div>
						<h2 class="text-base font-bold text-gray-900 dark:text-white">Payment</h2>
						<p class="text-xs text-gray-500 dark:text-gray-400">Payment details and status</p>
					</div>
				</div>
				<div class="grid grid-cols-1 gap-5 px-5 py-5 sm:grid-cols-2">
					<div>
						<p class="text-xs text-gray-500 dark:text-gray-400">Amount</p>
						<p class="mt-1 text-base font-bold text-emerald-600 dark:text-emerald-400">
							{deposit.currency} {deposit.amount.toFixed(2)}
						</p>
					</div>
					<div>
						<p class="text-xs text-gray-500 dark:text-gray-400">Currency</p>
						<p class="mt-1 text-sm text-gray-800 dark:text-gray-200">{deposit.currency}</p>
					</div>
					<div>
						<p class="text-xs text-gray-500 dark:text-gray-400">Method</p>
						<p class="mt-1 text-sm text-gray-800 dark:text-gray-200">{methodLabel(deposit.payment_method)}</p>
					</div>
					<div>
						<p class="text-xs text-gray-500 dark:text-gray-400">Transaction ID</p>
						<p class="mt-1 break-all font-mono text-xs text-gray-700 dark:text-gray-300">{deposit.transaction_id || '—'}</p>
					</div>
					<div>
						<p class="text-xs text-gray-500 dark:text-gray-400">Status</p>
						<p class="mt-1">
							<span class="inline-flex rounded-full px-2 py-0.5 text-[11px] font-semibold capitalize {deposit.status === 'completed'
								? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300'
								: deposit.status === 'pending'
									? 'bg-amber-50 text-amber-700 dark:bg-amber-950/40 dark:text-amber-300'
									: 'bg-rose-50 text-rose-700 dark:bg-rose-950/40 dark:text-rose-300'}">
								{deposit.status}
							</span>
						</p>
					</div>
					<div>
						<p class="text-xs text-gray-500 dark:text-gray-400">Paid At</p>
						<p class="mt-1 text-sm text-gray-800 dark:text-gray-200">{deposit.paid_at ? formatDateTime(deposit.paid_at) : 'Not paid yet'}</p>
					</div>
					<div>
						<p class="text-xs text-gray-500 dark:text-gray-400">Deposited By</p>
						<p class="mt-1 text-sm text-gray-800 dark:text-gray-200">{deposit.deposited_by || '—'}</p>
					</div>
					<div>
						<p class="text-xs text-gray-500 dark:text-gray-400">Requested At</p>
						<p class="mt-1 text-sm text-gray-800 dark:text-gray-200">{formatDateTime(deposit.created_at)}</p>
					</div>
				</div>
			</div>
		</div>

		<!-- Gateway response -->
		{#if deposit.gateway_response}
			<div class="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
				<button
					type="button"
					onclick={() => (showGatewayResponse = !showGatewayResponse)}
					class="flex w-full items-center justify-between px-5 py-4 text-left"
				>
					<div class="flex items-start gap-3">
						<svg class="mt-0.5 h-5 w-5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" d="M17.25 6.75 22.5 12l-5.25 5.25m-10.5 0L1.5 12l5.25-5.25m7.5-3-4.5 16.5" />
						</svg>
						<div>
							<h2 class="text-base font-bold text-gray-900 dark:text-white">Gateway Response</h2>
							<p class="text-xs text-gray-500 dark:text-gray-400">
								Raw payload from {deposit.gateway_name ? methodLabel(deposit.gateway_name) : methodLabel(deposit.payment_method)}
							</p>
						</div>
					</div>
					<svg class="h-4 w-4 text-gray-400 transition-transform {showGatewayResponse ? 'rotate-180' : ''}" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" />
					</svg>
				</button>
				{#if showGatewayResponse}
					<pre class="max-h-80 overflow-auto border-t border-gray-100 bg-gray-50 px-5 py-4 text-[11px] leading-relaxed text-gray-700 dark:border-gray-800 dark:bg-gray-950 dark:text-gray-300">{prettyGatewayPayload(deposit.gateway_response)}</pre>
				{/if}
			</div>
		{/if}
	{/if}

	{#if editing && deposit}
		<div class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-gray-900/50 p-4 sm:items-center">
			<div class="w-full max-w-3xl rounded-xl border border-gray-200 bg-white p-6 shadow-2xl dark:border-gray-800 dark:bg-gray-900">
				<div class="flex items-start justify-between gap-4">
					<h2 class="text-base font-bold text-gray-900 dark:text-white">Edit {reference}</h2>
					<button
						type="button"
						onclick={() => (editing = false)}
						class="inline-flex h-7 w-7 items-center justify-center rounded-md text-gray-400 hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-gray-800 dark:hover:text-gray-200"
						title="Close"
					>
						✕
					</button>
				</div>

				<div class="mt-5 grid grid-cols-1 gap-4 sm:grid-cols-2">
					<label class="block">
						<span class="text-xs font-semibold text-gray-700 dark:text-gray-200">Customer</span>
						<input
							type="text"
							value="{deposit.customer}{deposit.customer_email ? ` · ${deposit.customer_email}` : ''}"
							disabled
							class="mt-1 block w-full rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-xs text-gray-500 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-400"
						/>
					</label>

					<label class="block">
						<span class="text-xs font-semibold text-gray-700 dark:text-gray-200">Amount</span>
						<input
							type="number"
							min="0.01"
							step="0.01"
							bind:value={editAmount}
							class="mt-1 block w-full rounded-lg border border-gray-300 px-3 py-2 text-xs text-gray-900 focus:border-emerald-500 focus:outline-hidden dark:border-gray-700 dark:bg-gray-800 dark:text-white"
						/>
					</label>

					<label class="block">
						<span class="text-xs font-semibold text-gray-700 dark:text-gray-200">Currency</span>
						<input
							type="text"
							maxlength="10"
							bind:value={editCurrency}
							class="mt-1 block w-full rounded-lg border border-gray-300 px-3 py-2 text-xs uppercase text-gray-900 focus:border-emerald-500 focus:outline-hidden dark:border-gray-700 dark:bg-gray-800 dark:text-white"
						/>
					</label>

					<label class="block">
						<span class="text-xs font-semibold text-gray-700 dark:text-gray-200">Payment Method</span>
						<select
							bind:value={editMethod}
							class="mt-1 block w-full rounded-lg border border-gray-300 px-3 py-2 text-xs text-gray-900 focus:border-emerald-500 focus:outline-hidden dark:border-gray-700 dark:bg-gray-800 dark:text-white"
						>
							<option value="manual">manual</option>
							<option value="piprapay">piprapay</option>
							<option value="bkash">bkash</option>
							<option value="nagad">nagad</option>
							<option value="rocket">rocket</option>
						</select>
					</label>

					<label class="block">
						<span class="text-xs font-semibold text-gray-700 dark:text-gray-200">
							{deposit.payment_method?.toLowerCase().includes('pipra') ? 'PipraPay ID' : 'Transaction ID'}
						</span>
						<input
							type="text"
							bind:value={editTransactionId}
							class="mt-1 block w-full rounded-lg border border-gray-300 px-3 py-2 font-mono text-xs text-gray-900 focus:border-emerald-500 focus:outline-hidden dark:border-gray-700 dark:bg-gray-800 dark:text-white"
						/>
					</label>

					<label class="block">
						<span class="text-xs font-semibold text-gray-700 dark:text-gray-200">Status</span>
						<select
							bind:value={editStatus}
							disabled={deposit.status === 'completed'}
							class="mt-1 block w-full rounded-lg border border-gray-300 px-3 py-2 text-xs text-gray-900 focus:border-emerald-500 focus:outline-hidden disabled:bg-gray-50 disabled:text-gray-500 dark:border-gray-700 dark:bg-gray-800 dark:text-white"
						>
							<option value="pending">pending</option>
							<option value="completed">completed</option>
							<option value="failed">failed</option>
							<option value="rejected">rejected</option>
						</select>
						{#if deposit.status === 'completed'}
							<span class="mt-1 block text-[10px] text-gray-400">Approved deposits stay completed.</span>
						{/if}
					</label>

					<label class="block">
						<span class="text-xs font-semibold text-gray-700 dark:text-gray-200">Paid At</span>
						<input
							type="datetime-local"
							bind:value={editPaidAt}
							class="mt-1 block w-full rounded-lg border border-gray-300 px-3 py-2 text-xs text-gray-900 focus:border-emerald-500 focus:outline-hidden dark:border-gray-700 dark:bg-gray-800 dark:text-white"
						/>
					</label>

					<div class="sm:col-span-2">
						<span class="text-xs font-semibold text-gray-700 dark:text-gray-200">Gateway Response</span>
						<pre class="mt-1 max-h-40 overflow-auto rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-[11px] leading-relaxed text-gray-600 dark:border-gray-700 dark:bg-gray-950 dark:text-gray-300">{deposit.gateway_response
							? prettyGatewayPayload(deposit.gateway_response)
							: 'No gateway payload recorded.'}</pre>
					</div>
				</div>

				{#if editError}
					<p class="mt-4 rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-xs font-semibold text-rose-700 dark:border-rose-900 dark:bg-rose-950/40 dark:text-rose-300">
						{editError}
					</p>
				{/if}

				<div class="mt-5 flex items-center gap-2">
					<button
						type="button"
						onclick={saveEdit}
						disabled={editSaving}
						class="rounded-lg bg-amber-500 px-4 py-2 text-xs font-semibold text-white shadow-xs hover:bg-amber-400 disabled:opacity-50"
					>
						{editSaving ? 'Saving…' : 'Save changes'}
					</button>
					<button
						type="button"
						onclick={() => (editing = false)}
						disabled={editSaving}
						class="rounded-lg border border-gray-200 bg-white px-4 py-2 text-xs font-semibold text-gray-700 hover:bg-gray-50 disabled:opacity-50 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-300 dark:hover:bg-gray-800"
					>
						Cancel
					</button>
				</div>
			</div>
		</div>
	{/if}
</div>
