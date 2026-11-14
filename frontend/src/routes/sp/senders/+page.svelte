<script lang="ts">
	import { onMount } from 'svelte';
	import { spApi, SpApiError } from '$lib/sp-api.js';
	import { spAuth } from '$lib/stores/spAuth.svelte.js';

	interface Sender {
		id: number;
		sender_id: string;
		type: string;
		is_enabled: boolean;
		approval_status: string;
		rejection_reason: string;
		gateway: string;
		customers_count: number;
		created_at: string;
	}

	let senders = $state<Sender[]>([]);
	let loading = $state(true);
	let error = $state('');
	let success = $state('');
	let modalOpen = $state(false);
	let senderID = $state('');
	let senderType = $state('non_masking');
	let saving = $state(false);

	async function load() {
		loading = true;
		error = '';
		try {
			const res = await spApi.get<{ data: Sender[] }>('/api/v1/sp/senders');
			senders = res.data ?? [];
		} catch (e) {
			error = e instanceof SpApiError ? e.message : 'Unable to load senders.';
		} finally {
			loading = false;
		}
	}

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		saving = true; error = '';
		try {
			await spApi.post('/api/v1/sp/senders', { sender_id: senderID, type: senderType });
			success = 'Sender ID submitted for Superman approval.';
			modalOpen = false; senderID = '';
			await load();
			setTimeout(() => (success = ''), 5000);
		} catch (e) {
			error = e instanceof SpApiError ? e.message : 'Unable to submit sender ID.';
		} finally {
			saving = false;
		}
	}

	onMount(load);
</script>

<svelte:head><title>Senders — SMS Whiteinfo S P Reseller</title></svelte:head>

<div class="space-y-4">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-bold tracking-tight text-gray-950 dark:text-white">Senders</h1>
			<p class="text-xs text-gray-500 dark:text-gray-400">Request Sender IDs for Superman approval. Approved senders can then be assigned to your customers.</p>
		</div>
		{#if spAuth.can('messaging.manage')}
			<button type="button" onclick={() => { modalOpen = true; error = ''; }} class="rounded-lg bg-amber-500 px-3 py-1.5 text-xs font-semibold text-white hover:bg-amber-600">Request Sender ID</button>
		{/if}
	</div>

	{#if error}<div class="rounded-xl border border-rose-200 bg-rose-50 p-3 text-xs text-rose-700 dark:border-rose-900 dark:bg-rose-950/40 dark:text-rose-300">{error}</div>{/if}
	{#if success}<div class="rounded-xl border border-emerald-200 bg-emerald-50 p-3 text-xs text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-300">{success}</div>{/if}

	<div class="overflow-x-auto rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
		<table class="w-full text-left text-xs">
			<thead><tr class="border-b border-gray-100 text-[10px] uppercase text-gray-400 dark:border-gray-800">
				<th class="px-4 py-3">Sender ID</th><th class="px-4 py-3">Type</th><th class="px-4 py-3">Approval</th><th class="px-4 py-3">Gateway</th><th class="px-4 py-3">Enabled</th><th class="px-4 py-3">Customers</th>
			</tr></thead>
			<tbody class="divide-y divide-gray-100 dark:divide-gray-800">
				{#if loading}
					<tr><td colspan="6" class="px-4 py-10 text-center text-gray-400">Loading senders…</td></tr>
				{:else if senders.length === 0}
					<tr><td colspan="6" class="px-4 py-10 text-center text-gray-400">No Sender IDs requested yet.</td></tr>
				{:else}
					{#each senders as sender (sender.id)}
						<tr>
							<td class="px-4 py-3 font-mono font-semibold text-gray-900 dark:text-white">{sender.sender_id}</td>
							<td class="px-4 py-3">{sender.type === 'masking' ? 'Masking' : 'Non-masking'}</td>
							<td class="px-4 py-3"><span class="rounded-full bg-amber-100 px-2 py-0.5 text-[10px] font-semibold text-amber-800 dark:bg-amber-950 dark:text-amber-300">{sender.approval_status}</span>{#if sender.rejection_reason}<div class="mt-1 text-[10px] text-rose-500">{sender.rejection_reason}</div>{/if}</td>
							<td class="px-4 py-3 text-gray-600 dark:text-gray-300">{sender.gateway || 'Awaiting assignment'}</td>
							<td class="px-4 py-3">{sender.is_enabled ? 'Yes' : 'No'}</td>
							<td class="px-4 py-3">{sender.customers_count}</td>
						</tr>
					{/each}
				{/if}
			</tbody>
		</table>
	</div>
</div>

{#if modalOpen}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-gray-950/50 p-4">
		<div class="w-full max-w-md space-y-4 rounded-2xl bg-white p-6 shadow-2xl dark:bg-gray-900">
			<div class="flex items-center justify-between border-b border-gray-100 pb-3 dark:border-gray-800">
				<h3 class="text-sm font-bold text-gray-900 dark:text-white">Request Sender ID</h3>
				<button type="button" onclick={() => (modalOpen = false)} class="text-gray-400">✕</button>
			</div>
			<form onsubmit={submit} class="space-y-3 text-xs">
				<div><label for="sender-id" class="font-medium text-gray-700 dark:text-gray-300">Sender ID</label><input id="sender-id" bind:value={senderID} required placeholder="e.g. 8809617639963 or BrandName" class="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2 dark:border-gray-700 dark:bg-gray-800 dark:text-white" /></div>
				<div><label for="sender-type" class="font-medium text-gray-700 dark:text-gray-300">Type</label><select id="sender-type" bind:value={senderType} class="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2 dark:border-gray-700 dark:bg-gray-800 dark:text-white"><option value="non_masking">Non-Masking (numeric CLI)</option><option value="masking">Masking (alphanumeric brand)</option></select></div>
				<div class="flex justify-end gap-2"><button type="button" onclick={() => (modalOpen = false)} class="rounded-lg border border-gray-200 px-3 py-1.5 font-semibold text-gray-600 dark:border-gray-700 dark:text-gray-300">Cancel</button><button type="submit" disabled={saving} class="rounded-lg bg-amber-500 px-4 py-1.5 font-semibold text-white disabled:opacity-50">{saving ? 'Submitting…' : 'Submit'}</button></div>
			</form>
		</div>
	</div>
{/if}
