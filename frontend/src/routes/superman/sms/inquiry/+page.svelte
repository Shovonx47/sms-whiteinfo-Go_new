<script lang="ts">
	import { supermanApi } from '$lib/superman-api.js';

	interface MessageDetail {
		id: number;
		gateway_message_id: string;
		customer: string;
		to: string;
		sender: string;
		gateway: string;
		status: string;
		parts: number;
		cost: number;
		operator_cost: number;
		dipping_cost: number;
		body: string;
		created_at: string;
		sent_at: string;
		delivered_at: string;
	}

	let searchQuery = $state('');
	let searching = $state(false);
	let searched = $state(false);
	let messageData = $state<MessageDetail | null>(null);

	async function search() {
		if (!searchQuery.trim()) return;
		searching = true;
		try {
			const response = await supermanApi.get<{ data: MessageDetail | null }>(
				`/api/v1/superman/sms/inquiry?q=${encodeURIComponent(searchQuery.trim())}`
			);
			messageData = response.data ?? null;
		} catch {
			messageData = null;
		} finally {
			searched = true;
			searching = false;
		}
	}
</script>

<svelte:head>
	<title>Single Message Deep Inquiry &amp; Audit — SMS Whiteinfo Administrator</title>
</svelte:head>

<div class="space-y-4">
	<h1 class="text-3xl font-bold tracking-tight text-gray-950 dark:text-white">Single Message Deep Inquiry & Audit</h1>

	<div class="rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
		<div class="border-b border-gray-100 px-6 py-4 dark:border-gray-800">
			<h2 class="text-sm font-bold text-gray-900 dark:text-white">Message Lifecycle Inquiry</h2>
			<p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
				Enter internal Message ID or Upstream Gateway Message ID to retrieve delivery timeline, routing details, and financial costs.
			</p>
		</div>

		<form
			class="flex max-w-xl gap-3 px-6 py-5"
			onsubmit={(event) => {
				event.preventDefault();
				void search();
			}}
		>
			<input
				type="text"
				bind:value={searchQuery}
				placeholder="Enter Message ID or Gateway Message ID..."
				required
				class="flex-1 rounded-lg border border-gray-300 px-3 py-2 font-mono text-sm dark:border-gray-700 dark:bg-gray-900 dark:text-white"
			/>
			<button
				type="submit"
				disabled={searching}
				class="inline-flex items-center gap-1.5 rounded-lg bg-emerald-600 px-4 py-2 text-xs font-bold text-white hover:bg-emerald-700 disabled:opacity-60"
			>
				<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
				</svg>
				{searching ? 'Inspecting…' : 'Inspect Message'}
			</button>
		</form>
	</div>

	{#if searched}
		{#if messageData}
			<div class="rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
				<div class="border-b border-gray-100 px-6 py-4 dark:border-gray-800">
					<h2 class="text-sm font-bold text-gray-900 dark:text-white">Message Record #{messageData.id}</h2>
				</div>

				<div class="space-y-6 px-6 py-5">
					<div class="grid grid-cols-1 gap-6 md:grid-cols-3">
						<div class="rounded-xl bg-gray-50 p-4 dark:bg-gray-800">
							<p class="text-xs font-semibold uppercase text-gray-500">Recipient Phone</p>
							<p class="mt-1 font-mono text-lg font-bold text-gray-900 dark:text-white">{messageData.to}</p>
							<p class="mt-2 text-xs text-gray-500">Customer: <span class="font-medium text-gray-900 dark:text-gray-100">{messageData.customer}</span></p>
						</div>

						<div class="rounded-xl bg-gray-50 p-4 dark:bg-gray-800">
							<p class="text-xs font-semibold uppercase text-gray-500">Status &amp; Sender</p>
							<p class="mt-1 text-lg font-bold uppercase text-emerald-600 dark:text-emerald-400">{messageData.status}</p>
							<p class="mt-2 text-xs text-gray-500">Sender ID: <span class="font-mono font-medium text-gray-900 dark:text-gray-100">{messageData.sender}</span></p>
						</div>

						<div class="rounded-xl bg-gray-50 p-4 dark:bg-gray-800">
							<p class="text-xs font-semibold uppercase text-gray-500">Financial Ledger</p>
							<p class="mt-1 text-lg font-bold text-gray-900 dark:text-white">৳{messageData.cost.toFixed(4)} BDT</p>
							<p class="mt-2 text-xs text-gray-500">
								Gateway Cost: ৳{messageData.operator_cost.toFixed(4)} (Parts: {messageData.parts})
							</p>
						</div>
					</div>

					<div class="rounded-xl border border-gray-200 bg-white p-4 dark:border-gray-700 dark:bg-gray-900">
						<p class="mb-2 text-xs font-semibold uppercase text-gray-500">Message Body</p>
						<div class="rounded-lg bg-gray-50 p-3 font-mono text-sm text-gray-800 dark:bg-gray-800 dark:text-gray-200">
							{messageData.body}
						</div>
					</div>

					<div class="grid grid-cols-1 gap-4 text-xs text-gray-600 md:grid-cols-3 dark:text-gray-400">
						<div>Created: <span class="font-semibold text-gray-900 dark:text-gray-100">{messageData.created_at}</span></div>
						<div>Sent to Gateway: <span class="font-semibold text-gray-900 dark:text-gray-100">{messageData.sent_at}</span></div>
						<div>Delivered at Handset: <span class="font-semibold text-gray-900 dark:text-gray-100">{messageData.delivered_at}</span></div>
					</div>
				</div>
			</div>
		{:else}
			<div class="rounded-xl border border-gray-200 bg-white p-8 text-center shadow-xs dark:border-gray-800 dark:bg-gray-900">
				<svg class="mx-auto mb-2 h-10 w-10 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
				</svg>
				<p class="text-base font-semibold text-gray-900 dark:text-white">No Message Found</p>
				<p class="mt-1 text-xs text-gray-500">
					Could not find any message matching "{searchQuery}". Please verify the ID.
				</p>
			</div>
		{/if}
	{/if}
</div>
