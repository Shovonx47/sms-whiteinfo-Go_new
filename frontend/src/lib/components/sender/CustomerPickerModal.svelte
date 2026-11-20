<script lang="ts">
	import type { SenderCustomer } from '$lib/sender-admin.js';

	interface Props {
		customers: SenderCustomer[];
		selectedId: number | null;
		onSelect: (customer: SenderCustomer | null) => void;
		onClose: () => void;
	}

	let { customers, selectedId, onSelect, onClose }: Props = $props();

	let term = $state('');
	let filtered = $derived(
		customers
			.filter((c) => {
				const q = term.trim().toLowerCase();
				if (!q) return true;
				return c.name.toLowerCase().includes(q) || (c.email ?? '').toLowerCase().includes(q);
			})
			.slice(0, 200)
	);
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && onClose()} />

<div class="fixed inset-0 z-50 flex items-center justify-center bg-gray-950/50 p-4" role="presentation" onclick={onClose}>
	<div
		class="w-full max-w-md rounded-xl border border-gray-200 bg-white p-5 shadow-xl dark:border-gray-800 dark:bg-gray-900"
		role="dialog"
		aria-modal="true"
		aria-label="Select customer"
		tabindex="-1"
		onclick={(e) => e.stopPropagation()}
		onkeydown={(e) => e.stopPropagation()}
	>
		<div class="flex items-center justify-between">
			<h3 class="text-base font-semibold text-gray-950 dark:text-white">Select customer</h3>
			<button type="button" onclick={onClose} aria-label="Close" class="rounded-lg p-1 text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800">
				<svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" /></svg>
			</button>
		</div>

		<!-- svelte-ignore a11y_autofocus -->
		<input
			type="search"
			bind:value={term}
			autofocus
			placeholder="Search by name or email…"
			class="mt-3 w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm outline-none focus:border-emerald-600 focus:ring-1 focus:ring-emerald-600 dark:border-gray-700 dark:bg-gray-800 dark:text-white"
		/>

		<ul class="mt-3 max-h-72 divide-y divide-gray-100 overflow-y-auto rounded-lg border border-gray-200 dark:divide-gray-800 dark:border-gray-700">
			{#each filtered as customer (customer.id)}
				<li>
					<button
						type="button"
						onclick={() => onSelect(customer)}
						class="flex w-full items-center justify-between px-3 py-2 text-left text-sm hover:bg-gray-50 dark:hover:bg-gray-800/60 {selectedId === customer.id ? 'bg-emerald-50 dark:bg-emerald-950/20' : ''}"
					>
						<span class="min-w-0">
							<span class="block truncate font-medium text-gray-900 dark:text-white">{customer.name}</span>
							{#if customer.email}<span class="block truncate text-xs text-gray-400">{customer.email}</span>{/if}
						</span>
						{#if selectedId === customer.id}
							<svg class="h-4 w-4 shrink-0 text-emerald-600" fill="none" viewBox="0 0 24 24" stroke-width="2.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="m4.5 12.75 6 6 9-13.5" /></svg>
						{/if}
					</button>
				</li>
			{:else}
				<li class="px-3 py-6 text-center text-sm text-gray-400">No customers found.</li>
			{/each}
		</ul>

		<div class="mt-4 flex justify-between">
			<button type="button" onclick={() => onSelect(null)} class="text-xs font-semibold text-gray-500 hover:text-rose-600">Clear selection</button>
			<button type="button" onclick={onClose} class="rounded-lg border border-gray-300 px-3.5 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800">Close</button>
		</div>
	</div>
</div>
