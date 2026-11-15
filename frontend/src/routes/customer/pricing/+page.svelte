<script lang="ts">
	import { api } from '$lib/api.js';
	import { onMount } from 'svelte';

	interface OperatorRate {
		id: number;
		name: string;
		country: string;
		codes: string[];
		masking_rate: number | null;
		non_masking_rate: number | null;
	}

	let rates = $state<OperatorRate[]>([]);
	let loading = $state(true);

	async function load() {
		loading = true;
		try {
			const res = await api.get<{ data: OperatorRate[] }>('/api/v1/customer/pricing');
			rates = res.data || [];
		} catch {
			rates = [];
		} finally {
			loading = false;
		}
	}

	onMount(() => load());
</script>

<svelte:head>
	<title>Pricing - White INFO</title>
</svelte:head>

<div class="space-y-6">
	<!-- Breadcrumb -->
	<nav class="flex items-center gap-2 text-xs font-medium text-gray-500">
		<a href="/pricing" class="hover:text-gray-700">Pricing Reports</a>
		<svg class="h-3 w-3 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
			<path stroke-linecap="round" stroke-linejoin="round" d="m8.25 4.5 7.5 7.5-7.5 7.5" />
		</svg>
		<span class="text-gray-900 font-semibold">List</span>
	</nav>

	<!-- Page Heading -->
	<div class="flex items-center justify-between">
		<h1 class="text-2xl font-bold tracking-tight text-gray-950">Pricing Reports</h1>
	</div>

	<!-- Main Card -->
	<div class="rounded-xl border border-gray-200 bg-white shadow-sm overflow-hidden">
		<!-- Table -->
		<div class="overflow-x-auto">
			<table class="w-full text-left border-collapse text-xs">
				<thead>
					<tr class="border-b border-gray-200 bg-gray-50/70 text-gray-600 font-semibold tracking-wider uppercase text-[11px]">
						<th class="py-3 px-4">COUNTRY</th>
						<th class="py-3 px-4">OPERATOR</th>
						<th class="py-3 px-4">CODES</th>
						<th class="py-3 px-4 text-right">MASKING</th>
						<th class="py-3 px-4 text-right">NON MASKING</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-100 bg-white">
					{#if loading}
						<tr>
							<td colspan="5" class="py-12 text-center text-gray-500">
								<div class="inline-block h-6 w-6 animate-spin rounded-full border-2 border-gray-300 border-t-teal-600"></div>
								<p class="mt-2 text-xs">Loading pricing reports...</p>
							</td>
						</tr>
					{:else if rates.length === 0}
						<tr>
							<td colspan="5" class="py-12 text-center text-gray-500">
								<p class="text-sm font-medium text-gray-700">No tariff rates available</p>
								<p class="mt-1 text-xs text-gray-400">Your operator rates will appear here based on your assigned tariff plan.</p>
							</td>
						</tr>
					{:else}
						{#each rates as rate (rate.id)}
							<tr class="hover:bg-gray-50/70 transition">
								<td class="py-3 px-4 text-gray-700 whitespace-nowrap">
									{rate.country || '—'}
								</td>
								<td class="py-3 px-4 font-semibold text-gray-900 whitespace-nowrap">
									{rate.name}
								</td>
								<td class="py-3 px-4 whitespace-nowrap">
									<div class="flex flex-wrap gap-1">
										{#each rate.codes || [] as code}
											<span class="inline-flex items-center rounded-md bg-teal-50 px-2 py-0.5 text-xs font-mono font-medium text-teal-700 border border-teal-200">
												{code}
											</span>
										{/each}
									</div>
								</td>
								<td class="py-3 px-4 font-mono font-semibold text-right text-emerald-600 whitespace-nowrap">
									{rate.masking_rate !== null ? `৳ ${Number(rate.masking_rate).toFixed(4)}` : '—'}
								</td>
								<td class="py-3 px-4 font-mono font-semibold text-right text-gray-900 whitespace-nowrap">
									{rate.non_masking_rate !== null ? `৳ ${Number(rate.non_masking_rate).toFixed(4)}` : '—'}
								</td>
							</tr>
						{/each}
					{/if}
				</tbody>
			</table>
		</div>
	</div>
</div>
