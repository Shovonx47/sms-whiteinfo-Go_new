<script lang="ts">
	import { onMount } from 'svelte';
	import { spApi, SpApiError } from '$lib/sp-api.js';
	import { spAuth } from '$lib/stores/spAuth.svelte.js';

	interface KYCItem {
		id: number;
		name: string;
		email: string;
		phone: string;
		nid_number: string;
		front_nid: string;
		back_nid: string;
		live_photo: string;
		business_name: string;
		trade_license_number: string;
		trade_license_file: string;
		kyc_status: string;
		kyc_submitted_at: string | null;
		kyc_rejected_reason: string;
	}

	let items = $state<KYCItem[]>([]);
	let loading = $state(true);
	let error = $state('');
	let success = $state('');
	let statusFilter = $state('awaiting_sp_verification');
	let active = $state<KYCItem | null>(null);
	let rejecting = $state(false);
	let rejectReason = $state('');
	let rejectFields = $state<string[]>([]);

	const fieldOptions = [
		{ key: 'nid_number', label: 'NID Number' },
		{ key: 'nid_front', label: 'NID Front Photo' },
		{ key: 'nid_back', label: 'NID Back Photo' },
		{ key: 'live_photo', label: 'Live Selfie' },
		{ key: 'trade_license', label: 'Trade License' },
		{ key: 'other', label: 'Other' }
	];

	async function load() {
		loading = true;
		error = '';
		try {
			const res = await spApi.get<{ data: KYCItem[] }>(`/api/v1/sp/kyc?status=${statusFilter}`);
			items = res.data ?? [];
		} catch (e) {
			error = e instanceof SpApiError ? e.message : 'Unable to load KYC submissions.';
		} finally {
			loading = false;
		}
	}

	async function verify(item: KYCItem) {
		if (!confirm(`Verify ${item.name}'s KYC and forward it to the admin?`)) return;
		try {
			await spApi.post(`/api/v1/sp/kyc/${item.id}/verify`);
			success = 'KYC verified and forwarded to the admin.';
			active = null;
			await load();
			setTimeout(() => (success = ''), 5000);
		} catch (e) {
			error = e instanceof SpApiError ? e.message : 'Unable to verify KYC.';
		}
	}

	function openReject(item: KYCItem) {
		active = item;
		rejecting = true;
		rejectReason = '';
		rejectFields = [];
	}

	async function sendBack() {
		if (!active) return;
		if (rejectFields.length === 0 || !rejectReason.trim()) {
			error = 'Select the fields to correct and provide a reason.';
			return;
		}
		try {
			await spApi.post(`/api/v1/sp/kyc/${active.id}/reject`, {
				fields: rejectFields,
				reason: rejectReason
			});
			success = 'KYC sent back for resubmission.';
			active = null;
			rejecting = false;
			await load();
			setTimeout(() => (success = ''), 5000);
		} catch (e) {
			error = e instanceof SpApiError ? e.message : 'Unable to send KYC back.';
		}
	}

	function toggleField(key: string) {
		rejectFields = rejectFields.includes(key) ? rejectFields.filter((f) => f !== key) : [...rejectFields, key];
	}

	onMount(load);
</script>

<svelte:head><title>KYC Verification — SMS Whiteinfo S P Reseller</title></svelte:head>

<div class="space-y-4">
	<div class="flex flex-wrap items-center justify-between gap-3">
		<div>
			<h1 class="text-2xl font-bold tracking-tight text-gray-950 dark:text-white">Customer KYC Verification</h1>
			<p class="text-xs text-gray-500 dark:text-gray-400">Verify your reseller customers' documents and forward them to the admin for final approval.</p>
		</div>
		<select bind:value={statusFilter} onchange={load} class="rounded-lg border border-gray-300 bg-white px-3 py-1.5 text-xs dark:border-gray-700 dark:bg-gray-900 dark:text-white">
			<option value="awaiting_sp_verification">Awaiting my verification</option>
			<option value="under_review">With admin</option>
			<option value="verified">Verified</option>
			<option value="resubmit_required">Resubmission required</option>
			<option value="rejected">Rejected</option>
			<option value="all">All submissions</option>
		</select>
	</div>

	{#if error}<div class="rounded-xl border border-rose-200 bg-rose-50 p-3 text-xs text-rose-700 dark:border-rose-900 dark:bg-rose-950/40 dark:text-rose-300">{error}</div>{/if}
	{#if success}<div class="rounded-xl border border-emerald-200 bg-emerald-50 p-3 text-xs text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-300">{success}</div>{/if}

	<div class="rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
		<div class="overflow-x-auto">
			<table class="w-full text-left text-xs">
				<thead>
					<tr class="border-b border-gray-100 text-[10px] uppercase text-gray-400 dark:border-gray-800">
						<th class="px-4 py-3">Customer</th>
						<th class="px-4 py-3">NID</th>
						<th class="px-4 py-3">Status</th>
						<th class="px-4 py-3">Submitted</th>
						<th class="px-4 py-3 text-right">Actions</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-100 dark:divide-gray-800">
					{#if loading}
						<tr><td colspan="5" class="px-4 py-10 text-center text-gray-400">Loading KYC submissions…</td></tr>
					{:else if items.length === 0}
						<tr><td colspan="5" class="px-4 py-10 text-center text-gray-400">No KYC submissions found.</td></tr>
					{:else}
						{#each items as item (item.id)}
							<tr class="hover:bg-gray-50/60 dark:hover:bg-gray-800/30">
								<td class="px-4 py-3">
									<div class="font-semibold text-gray-900 dark:text-white">{item.name}</div>
									<div class="text-[11px] text-gray-400">{item.phone || item.email}</div>
								</td>
								<td class="px-4 py-3 font-mono text-gray-600 dark:text-gray-300">{item.nid_number || '—'}</td>
								<td class="px-4 py-3"><span class="rounded-full bg-amber-100 px-2 py-0.5 text-[10px] font-semibold text-amber-800 dark:bg-amber-950 dark:text-amber-300">{item.kyc_status}</span></td>
								<td class="px-4 py-3 text-gray-500">{item.kyc_submitted_at ? new Date(item.kyc_submitted_at).toLocaleString() : '—'}</td>
								<td class="px-4 py-3 text-right whitespace-nowrap">
									<button type="button" onclick={() => { active = item; rejecting = false; }} class="rounded-lg border border-gray-200 px-2 py-1 text-[11px] font-semibold text-gray-600 dark:border-gray-700 dark:text-gray-300">View</button>
									{#if item.kyc_status === 'awaiting_sp_verification' && spAuth.can('customers.edit')}
										<button type="button" onclick={() => verify(item)} class="ml-1 rounded-lg bg-emerald-600 px-2 py-1 text-[11px] font-semibold text-white">Verify</button>
										<button type="button" onclick={() => openReject(item)} class="ml-1 rounded-lg bg-rose-600 px-2 py-1 text-[11px] font-semibold text-white">Send Back</button>
									{/if}
								</td>
							</tr>
						{/each}
					{/if}
				</tbody>
			</table>
		</div>
	</div>
</div>

{#if active}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-gray-950/60 p-4">
		<div class="max-h-[90vh] w-full max-w-3xl overflow-y-auto rounded-2xl bg-white p-6 shadow-2xl dark:bg-gray-900">
			<div class="flex items-center justify-between border-b border-gray-100 pb-3 dark:border-gray-800">
				<h3 class="text-sm font-bold text-gray-900 dark:text-white">KYC Documents — {active.name}</h3>
				<button type="button" onclick={() => { active = null; rejecting = false; }} class="text-gray-400">✕</button>
			</div>

			{#if rejecting}
				<div class="mt-4 space-y-4">
					<div>
						<p class="text-xs font-semibold text-gray-700 dark:text-gray-300">Fields requiring correction</p>
						<div class="mt-2 grid grid-cols-2 gap-2">
							{#each fieldOptions as option}
								<label class="flex items-center gap-2 text-xs text-gray-600 dark:text-gray-300">
									<input type="checkbox" checked={rejectFields.includes(option.key)} onchange={() => toggleField(option.key)} />
									{option.label}
								</label>
							{/each}
						</div>
					</div>
					<div>
						<label for="kyc-reason" class="text-xs font-semibold text-gray-700 dark:text-gray-300">Reason / instructions</label>
						<textarea id="kyc-reason" bind:value={rejectReason} rows="3" class="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2 text-xs dark:border-gray-700 dark:bg-gray-800 dark:text-white"></textarea>
					</div>
					<div class="flex justify-end gap-2">
						<button type="button" onclick={() => (rejecting = false)} class="rounded-lg border border-gray-200 px-3 py-1.5 text-xs font-semibold dark:border-gray-700">Cancel</button>
						<button type="button" onclick={sendBack} class="rounded-lg bg-rose-600 px-3 py-1.5 text-xs font-semibold text-white">Reject & Request Resubmission</button>
					</div>
				</div>
			{:else}
				<div class="mt-4 space-y-4 text-xs">
					<div class="grid gap-3 sm:grid-cols-2">
						<div><p class="text-gray-400">NID Number</p><p class="font-mono font-semibold text-gray-900 dark:text-white">{active.nid_number || '—'}</p></div>
						<div><p class="text-gray-400">Business</p><p class="font-semibold text-gray-900 dark:text-white">{active.business_name || '—'}</p></div>
						<div><p class="text-gray-400">Trade License</p><p class="font-mono font-semibold text-gray-900 dark:text-white">{active.trade_license_number || '—'}</p></div>
						<div><p class="text-gray-400">Status</p><p class="font-semibold text-gray-900 dark:text-white">{active.kyc_status}</p></div>
					</div>
					<div class="grid gap-3 sm:grid-cols-2">
						{#each [{ label: 'NID Front', src: active.front_nid }, { label: 'NID Back', src: active.back_nid }, { label: 'Live Selfie', src: active.live_photo }, { label: 'Trade License', src: active.trade_license_file }] as doc}
							<div>
								<p class="mb-1 text-gray-400">{doc.label}</p>
								{#if doc.src}
									<a href={doc.src} target="_blank" rel="noopener" class="block overflow-hidden rounded-lg border border-gray-200 dark:border-gray-700">
										<img src={doc.src} alt={doc.label} class="h-32 w-full object-cover" />
									</a>
								{:else}
									<p class="rounded-lg border border-dashed border-gray-200 p-4 text-center text-gray-400 dark:border-gray-700">Not uploaded</p>
								{/if}
							</div>
						{/each}
					</div>
					{#if active.kyc_rejected_reason}
						<p class="rounded-lg bg-amber-50 p-3 text-amber-800 dark:bg-amber-950/40 dark:text-amber-300">Previous note: {active.kyc_rejected_reason}</p>
					{/if}
					{#if active.kyc_status === 'awaiting_sp_verification' && spAuth.can('customers.edit')}
						<div class="flex justify-end gap-2">
							<button type="button" onclick={() => openReject(active!)} class="rounded-lg bg-rose-600 px-3 py-1.5 font-semibold text-white">Reject & Send Back</button>
							<button type="button" onclick={() => verify(active!)} class="rounded-lg bg-emerald-600 px-3 py-1.5 font-semibold text-white">Verify & Forward</button>
						</div>
					{/if}
				</div>
			{/if}
		</div>
	</div>
{/if}
