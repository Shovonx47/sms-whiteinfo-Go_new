<script lang="ts">
	import { supermanApi } from '$lib/superman-api.js';

	interface DeliveryRow {
		id: number;
		recipient: string;
		sender_id: string | null;
		gateway: string | null;
		gateway_message_id: string | null;
		status: string;
		sent_at: string | null;
		delivered_at: string | null;
	}

	let phone = $state('');
	let searching = $state(false);
	let searched = $state(false);
	let rows = $state<DeliveryRow[]>([]);

	function format(value: string | null, fallback: string) {
		return value ? new Date(value).toLocaleString() : fallback;
	}

	async function check() {
		if (!phone.trim()) return;
		searching = true;
		try {
			const response = await supermanApi.get<{ data: DeliveryRow[] }>(
				`/api/v1/superman/dlr/check?phone=${encodeURIComponent(phone.trim())}`
			);
			rows = response.data ?? [];
		} catch {
			rows = [];
		} finally {
			searched = true;
			searching = false;
		}
	}
</script>

<svelte:head>
	<title>Handset Delivery Status Checker — SMS Whiteinfo Administrator</title>
</svelte:head>

<div class="space-y-4">
	<h1 class="text-3xl font-bold tracking-tight text-gray-950 dark:text-white">Handset Delivery Status Checker</h1>

	<div class="rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
		<div class="border-b border-gray-100 px-6 py-4 dark:border-gray-800">
			<h2 class="text-sm font-bold text-gray-900 dark:text-white">Live Handset DLR Delivery Verification</h2>
			<p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
				Lookup the delivery status of recent messages dispatched to any destination mobile number.
			</p>
		</div>

		<form
			class="flex max-w-xl gap-3 px-6 py-5"
			onsubmit={(event) => {
				event.preventDefault();
				void check();
			}}
		>
			<input
				type="text"
				bind:value={phone}
				placeholder="Enter recipient number (e.g. 01712345678)"
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
				{searching ? 'Checking…' : 'Check Delivery'}
			</button>
		</form>
	</div>

	{#if searched}
		<div class="rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
			<div class="border-b border-gray-100 px-6 py-4 dark:border-gray-800">
				<h2 class="text-sm font-bold text-gray-900 dark:text-white">Recent Messages Dispatched to "{phone}"</h2>
			</div>

			{#if rows.length > 0}
				<div class="overflow-x-auto px-6 py-4">
					<table class="w-full text-left text-sm text-gray-600 dark:text-gray-300">
						<thead class="bg-gray-50 text-xs font-semibold uppercase text-gray-500 dark:bg-gray-800 dark:text-gray-400">
							<tr>
								<th class="px-4 py-3">Message ID</th>
								<th class="px-4 py-3">Sender</th>
								<th class="px-4 py-3">Gateway</th>
								<th class="px-4 py-3">Gateway DLR ID</th>
								<th class="px-4 py-3">Status</th>
								<th class="px-4 py-3">Dispatched At</th>
								<th class="px-4 py-3">Delivered At</th>
							</tr>
						</thead>
						<tbody class="divide-y divide-gray-200 dark:divide-gray-700">
							{#each rows as row (row.id)}
								<tr class="hover:bg-gray-50 dark:hover:bg-gray-800/50">
									<td class="px-4 py-3 font-mono font-bold text-gray-900 dark:text-white">#{row.id}</td>
									<td class="px-4 py-3 font-semibold text-gray-800 dark:text-gray-200">{row.sender_id ?? 'System'}</td>
									<td class="px-4 py-3 text-xs">{row.gateway ?? 'Default'}</td>
									<td class="px-4 py-3 font-mono text-xs">{row.gateway_message_id ?? 'N/A'}</td>
									<td class="px-4 py-3">
										<span class="inline-flex rounded px-2 py-0.5 text-xs font-bold uppercase {row.status === 'delivered'
											? 'bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-400'
											: 'bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-400'}">
											{row.status}
										</span>
									</td>
									<td class="px-4 py-3 text-xs">{format(row.sent_at, 'Pending')}</td>
									<td class="px-4 py-3 text-xs font-medium">{format(row.delivered_at, 'Pending / Not Received')}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{:else}
				<p class="px-6 py-4 text-center text-sm text-gray-500">No messages sent to this destination number.</p>
			{/if}
		</div>
	{/if}
</div>
