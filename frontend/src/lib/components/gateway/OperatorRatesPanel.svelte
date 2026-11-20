<script lang="ts">
	import { onMount } from 'svelte';
	import { supermanApi } from '$lib/superman-api.js';
	import Pagination from '$lib/components/common/Pagination.svelte';
	import { BADGE_TONES } from '$lib/gateway-admin.js';

	interface Props {
		gatewayId: number;
		/** "mno" gateways price masking separately; IPTSP gateways do not. */
		apiType: string;
	}

	let { gatewayId, apiType }: Props = $props();

	interface RateRow {
		id: number;
		operator_id: number;
		operator_name: string;
		route_code: string | null;
		country_name: string;
		masking_price: number | null;
		non_masking_price: number | null;
		dipping_charge: number | null;
		operator_cost: number;
		dipping_cost: number;
	}

	interface OperatorOption {
		id: number;
		name: string;
		country_name: string;
		is_active: boolean;
	}

	let rates = $state<RateRow[]>([]);
	let operators = $state<OperatorOption[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let flash = $state<{ type: 'success' | 'error'; text: string } | null>(null);
	let flashTimer: ReturnType<typeof setTimeout> | undefined;

	let search = $state('');
	let sortKey = $state('');
	let sortDir = $state<'asc' | 'desc'>('asc');
	let currentPage = $state(1);
	let perPage = $state(10);
	let total = $state(0);
	let requestSeq = 0;

	let usesMasking = $derived(apiType === 'mno');
	let nonMaskingLabel = $derived(apiType === 'mno' ? 'MNO Operator Rate' : 'IPTSP Operator Rate');

	// Modal state
	let showModal = $state(false);
	let editingId = $state<number | null>(null);
	let formOperator = $state('');
	let formMasking = $state('');
	let formNonMasking = $state('');
	let formDipping = $state('');
	let formError = $state<string | null>(null);
	let saving = $state(false);

	function notify(type: 'success' | 'error', text: string) {
		flash = { type, text };
		clearTimeout(flashTimer);
		flashTimer = setTimeout(() => (flash = null), 6000);
	}

	async function loadRates() {
		const seq = ++requestSeq;
		loading = true;
		error = null;
		try {
			const query = new URLSearchParams({ page: String(currentPage), per_page: String(perPage) });
			if (search.trim()) query.set('search', search.trim());
			if (sortKey) {
				query.set('sort', sortKey);
				query.set('dir', sortDir);
			}
			const res = await supermanApi.get<{ data: RateRow[]; meta: { total: number } }>(
				`/api/v1/superman/gateways/${gatewayId}/operator-rates?${query.toString()}`
			);
			if (seq !== requestSeq) return;
			rates = res.data ?? [];
			total = res.meta?.total ?? 0;
			const lastPage = Math.max(1, Math.ceil(total / perPage));
			if (currentPage > lastPage) {
				currentPage = lastPage;
				void loadRates();
			}
		} catch (err: unknown) {
			if (seq !== requestSeq) return;
			error = err instanceof Error ? err.message : 'Unable to load operator rates.';
		} finally {
			if (seq === requestSeq) loading = false;
		}
	}

	function refilter() {
		currentPage = 1;
		void loadRates();
	}

	let searchTimer: ReturnType<typeof setTimeout> | undefined;
	function debouncedRefilter() {
		clearTimeout(searchTimer);
		searchTimer = setTimeout(refilter, 350);
	}

	function toggleSort(key: string) {
		if (sortKey === key) sortDir = sortDir === 'asc' ? 'desc' : 'asc';
		else {
			sortKey = key;
			sortDir = 'asc';
		}
		refilter();
	}

	const money = (value: number | null, decimals = 4) => (value === null || value === undefined ? '—' : `৳${value.toFixed(decimals)}`);

	function openCreate() {
		editingId = null;
		formOperator = '';
		formMasking = '';
		formNonMasking = '';
		formDipping = '';
		formError = null;
		showModal = true;
	}

	function openEdit(rate: RateRow) {
		editingId = rate.id;
		formOperator = String(rate.operator_id);
		formMasking = rate.masking_price === null ? '' : String(rate.masking_price);
		formNonMasking = rate.non_masking_price === null ? '' : String(rate.non_masking_price);
		formDipping = rate.dipping_charge === null ? '' : String(rate.dipping_charge);
		formError = null;
		showModal = true;
	}

	const numberOrNull = (value: string): number | null => (value.trim() === '' ? null : Number(value));

	async function save() {
		formError = null;
		if (!formOperator) {
			formError = 'The operator field is required.';
			return;
		}
		for (const [label, value] of [
			['Masking price', formMasking],
			['Non-masking rate', formNonMasking],
			['Dipping charge', formDipping]
		] as const) {
			if (value.trim() !== '' && !Number.isFinite(Number(value))) {
				formError = `${label} must be a number.`;
				return;
			}
		}
		const payload = {
			operator_id: Number(formOperator),
			masking_price: usesMasking ? numberOrNull(formMasking) : null,
			non_masking_price: numberOrNull(formNonMasking),
			dipping_charge: numberOrNull(formDipping)
		};
		saving = true;
		try {
			if (editingId) {
				await supermanApi.put(`/api/v1/superman/gateways/${gatewayId}/operator-rates/${editingId}`, payload);
				notify('success', 'Operator rate updated.');
			} else {
				await supermanApi.post(`/api/v1/superman/gateways/${gatewayId}/operator-rates`, payload);
				notify('success', 'Operator rate created.');
			}
			showModal = false;
			void loadRates();
		} catch (err: unknown) {
			formError = err instanceof Error ? err.message : 'Unable to save the operator rate.';
		} finally {
			saving = false;
		}
	}

	async function remove(rate: RateRow) {
		if (!window.confirm(`Delete the ${rate.operator_name} rate?`)) return;
		try {
			await supermanApi.delete(`/api/v1/superman/gateways/${gatewayId}/operator-rates/${rate.id}`);
			notify('success', 'Operator rate deleted.');
			void loadRates();
		} catch (err: unknown) {
			notify('error', err instanceof Error ? err.message : 'Unable to delete the operator rate.');
		}
	}

	onMount(() => {
		void loadRates();
		void supermanApi
			.get<{ data: OperatorOption[] }>('/api/v1/superman/operators')
			.then((res) => (operators = (res.data ?? []).filter((o) => o.is_active)))
			.catch(() => {
				/* the dropdown simply stays empty */
			});
	});

	const input =
		'mt-1.5 w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-xs outline-none placeholder:text-gray-400 focus:border-emerald-600 focus:ring-1 focus:ring-emerald-600 dark:border-gray-700 dark:bg-gray-800 dark:text-white';
	const th = 'whitespace-nowrap px-4 py-3 uppercase tracking-wide';
	const badge = 'inline-flex items-center whitespace-nowrap rounded-md border px-2 py-0.5 text-xs font-medium';
</script>

{#snippet sortHeader(label: string, key: string)}
	<button type="button" onclick={() => toggleSort(key)} class="inline-flex items-center gap-1 uppercase tracking-wide">
		{label}
		<svg class="h-3.5 w-3.5 {sortKey === key ? 'text-emerald-600' : 'text-gray-400'} {sortKey === key && sortDir === 'desc' ? 'rotate-180' : ''}" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" /></svg>
	</button>
{/snippet}

<section class="rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900" data-testid="operator-rates">
	<header class="flex flex-wrap items-center justify-between gap-3 px-6 py-4">
		<h2 class="text-base font-semibold text-gray-950 dark:text-white">Operator Rates</h2>
		<button type="button" onclick={openCreate} data-testid="new-rate" class="rounded-lg border border-emerald-600 bg-white px-3.5 py-2 text-sm font-semibold text-emerald-700 shadow-xs hover:bg-emerald-50 dark:bg-gray-900 dark:text-emerald-400">
			New gateway operator rate
		</button>
	</header>

	{#if flash}
		<div role="status" data-testid="rate-flash" class="mx-6 mb-3 flex items-center justify-between rounded-lg border px-4 py-2.5 text-sm {flash.type === 'success' ? 'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-300' : 'border-rose-200 bg-rose-50 text-rose-800 dark:border-rose-900 dark:bg-rose-950/30 dark:text-rose-300'}">
			<span>{flash.text}</span>
			<button type="button" onclick={() => (flash = null)} aria-label="Dismiss" class="ml-4 text-lg leading-none opacity-60 hover:opacity-100">×</button>
		</div>
	{/if}

	<div class="flex justify-end px-4 pb-3">
		<div class="relative w-64 max-w-full">
			<svg class="pointer-events-none absolute left-3 top-2.5 h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" /></svg>
			<input type="search" bind:value={search} oninput={debouncedRefilter} placeholder="Search" aria-label="Search operator rates" class="w-full rounded-lg border border-gray-300 bg-white py-2 pl-9 pr-3 text-sm text-gray-900 outline-none focus:border-emerald-600 focus:ring-1 focus:ring-emerald-600 dark:border-gray-700 dark:bg-gray-800 dark:text-white" />
		</div>
	</div>

	{#if error}
		<div role="alert" class="mx-4 mb-3 rounded-lg border border-rose-200 bg-rose-50 p-3 text-sm text-rose-700">{error}</div>
	{/if}

	<div class="overflow-x-auto border-t border-gray-200 dark:border-gray-800">
		<table class="w-full text-left text-sm" data-testid="rates-table">
			<thead>
				<tr class="bg-gray-50/70 text-xs font-semibold text-gray-700 dark:bg-gray-800/40 dark:text-gray-200">
					<th class={th}>{@render sortHeader('Operator', 'operator')}</th>
					<th class={th}>Route Code</th>
					<th class={th}>{@render sortHeader('Country', 'country')}</th>
					{#if usesMasking}<th class={th}>Masking Price</th>{/if}
					<th class={th}>{nonMaskingLabel}</th>
					<th class={th}>Dipping Charge</th>
					<th class={th}>Operator Cost</th>
					<th class={th}>Dipping Cost</th>
					<th class="w-24 px-4 py-3"><span class="sr-only">Actions</span></th>
				</tr>
			</thead>
			<tbody class="divide-y divide-gray-100 dark:divide-gray-800 {loading && rates.length > 0 ? 'opacity-60' : ''}">
				{#each rates as rate (rate.id)}
					<tr class="hover:bg-gray-50/70 dark:hover:bg-gray-800/30">
						<td class="whitespace-nowrap px-4 py-3.5 text-gray-900 dark:text-white">{rate.operator_name}</td>
						<td class="whitespace-nowrap px-4 py-3.5">
							{#if rate.route_code}<span class="{badge} {BADGE_TONES.info}">{rate.route_code}</span>{:else}<span class="text-gray-400">—</span>{/if}
						</td>
						<td class="whitespace-nowrap px-4 py-3.5 text-gray-800 dark:text-gray-200">{rate.country_name}</td>
						{#if usesMasking}<td class="whitespace-nowrap px-4 py-3.5 tabular-nums">{money(rate.masking_price)}</td>{/if}
						<td class="whitespace-nowrap px-4 py-3.5 tabular-nums">{money(rate.non_masking_price)}</td>
						<td class="whitespace-nowrap px-4 py-3.5 tabular-nums">{money(rate.dipping_charge)}</td>
						<td class="whitespace-nowrap px-4 py-3.5"><span class="{badge} {BADGE_TONES.warning}">{rate.operator_cost.toFixed(4)} ৳</span></td>
						<td class="whitespace-nowrap px-4 py-3.5"><span class="{badge} {BADGE_TONES.info}">{rate.dipping_cost.toFixed(4)} ৳</span></td>
						<td class="whitespace-nowrap px-4 py-3.5 text-right">
							<button type="button" onclick={() => openEdit(rate)} aria-label="Edit {rate.operator_name} rate" class="mr-1 inline-flex h-8 w-8 items-center justify-center rounded-lg text-emerald-700 hover:bg-emerald-50 dark:text-emerald-400 dark:hover:bg-emerald-950/30">
								<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L10.582 16.07a4.5 4.5 0 0 1-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 0 1 1.13-1.897l8.932-8.931Zm0 0L19.5 7.125M18 14v4.75A2.25 2.25 0 0 1 15.75 21H5.25A2.25 2.25 0 0 1 3 18.75V8.25A2.25 2.25 0 0 1 5.25 6H10" /></svg>
							</button>
							<button type="button" onclick={() => void remove(rate)} aria-label="Delete {rate.operator_name} rate" class="inline-flex h-8 w-8 items-center justify-center rounded-lg text-rose-600 hover:bg-rose-50 dark:hover:bg-rose-950/30">
								<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="m14.74 9-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0" /></svg>
							</button>
						</td>
					</tr>
				{:else}
					<tr><td colspan={usesMasking ? 9 : 8} class="px-4 py-12 text-center text-sm text-gray-400">{loading ? 'Loading operator rates…' : search ? 'No operator rates match your search.' : 'No operator rates yet.'}</td></tr>
				{/each}
			</tbody>
		</table>
	</div>

	<Pagination
		{total}
		page={currentPage}
		{perPage}
		perPageOptions={[10, 25, 50]}
		{loading}
		onPageChange={(p) => {
			currentPage = p;
			void loadRates();
		}}
		onPerPageChange={(size) => {
			perPage = size;
			refilter();
		}}
	/>
</section>

{#if showModal}
	<div class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-gray-950/50 p-4 pt-20" role="presentation" onclick={() => (showModal = false)}>
		<div class="w-full max-w-lg rounded-xl border border-gray-200 bg-white p-6 shadow-xl dark:border-gray-800 dark:bg-gray-900" role="dialog" aria-modal="true" aria-label={editingId ? 'Edit gateway operator rate' : 'New gateway operator rate'} tabindex="-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.key === 'Escape' && (showModal = false)}>
			<div class="mb-4 flex items-start justify-between">
				<h3 class="text-lg font-semibold text-gray-950 dark:text-white">{editingId ? 'Edit gateway operator rate' : 'New gateway operator rate'}</h3>
				<button type="button" onclick={() => (showModal = false)} aria-label="Close" class="rounded-lg p-1 text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800">
					<svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" /></svg>
				</button>
			</div>

			{#if formError}
				<div role="alert" data-testid="rate-error" class="mb-3 rounded-lg border border-rose-200 bg-rose-50 p-2.5 text-sm text-rose-700 dark:border-rose-900 dark:bg-rose-950/30 dark:text-rose-300">{formError}</div>
			{/if}

			<form
				onsubmit={(e) => {
					e.preventDefault();
					void save();
				}}
				class="space-y-4"
				novalidate
			>
				<div>
					<label for="rate-operator" class="block text-sm font-medium text-gray-900 dark:text-gray-100">Operator <span class="text-rose-600">*</span></label>
					<select id="rate-operator" bind:value={formOperator} disabled={editingId !== null} class="{input} disabled:bg-gray-100 disabled:text-gray-500">
						<option value="">Select an option</option>
						{#each operators as operator (operator.id)}
							<option value={String(operator.id)}>{operator.name} ({operator.country_name})</option>
						{/each}
					</select>
				</div>
				{#if usesMasking}
					<div>
						<label for="rate-masking" class="block text-sm font-medium text-gray-900 dark:text-gray-100">Masking Price (per part)</label>
						<input id="rate-masking" type="number" step="any" bind:value={formMasking} class={input} />
					</div>
				{/if}
				<div>
					<label for="rate-non-masking" class="block text-sm font-medium text-gray-900 dark:text-gray-100">Non-Masking Rate (Per SMS)</label>
					<input id="rate-non-masking" type="number" step="0.0001" bind:value={formNonMasking} class={input} />
				</div>
				<div>
					<label for="rate-dipping" class="block text-sm font-medium text-gray-900 dark:text-gray-100">Dipping Charge (Per Number)</label>
					<input id="rate-dipping" type="number" step="0.0001" bind:value={formDipping} class={input} />
					<p class="mt-1.5 text-sm text-gray-500">The gateway dipping cost charged for this operator per number.</p>
				</div>
				<div class="flex items-center gap-3 pt-2">
					<button type="submit" disabled={saving} data-testid="save-rate" class="rounded-lg border border-emerald-600 bg-white px-4 py-2 text-sm font-semibold text-emerald-700 hover:bg-emerald-50 disabled:opacity-60 dark:bg-gray-900 dark:text-emerald-400">{saving ? 'Saving…' : editingId ? 'Save changes' : 'Create'}</button>
					<button type="button" onclick={() => (showModal = false)} class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-800 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-200">Cancel</button>
				</div>
			</form>
		</div>
	</div>
{/if}
