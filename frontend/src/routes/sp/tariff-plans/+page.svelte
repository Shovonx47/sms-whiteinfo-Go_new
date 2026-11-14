<script lang="ts">
	import { onMount } from 'svelte';
	import { spApi, SpApiError } from '$lib/sp-api.js';
	import { spAuth } from '$lib/stores/spAuth.svelte.js';

	interface Plan { id: number; name: string; slug: string; type: string; status: string; currency: string; description: string; customers_count: number; }
	interface Operator { id: number; name: string; }
	interface Rate { operator_id: number; operator: string; masking_price: number; non_masking_price: number; }

	let plans = $state<Plan[]>([]);
	let operators = $state<Operator[]>([]);
	let loading = $state(true);
	let error = $state('');
	let success = $state('');
	let modalOpen = $state(false);
	let editing = $state<Plan | null>(null);
	let name = $state('');
	let slug = $state('');
	let planType = $state('standard');
	let status = $state('draft');
	let currency = $state('BDT');
	let description = $state('');
	let rates = $state<Rate[]>([]);
	let saving = $state(false);

	async function load() {
		loading = true;
		error = '';
		try {
			const [planRes, operatorRes] = await Promise.all([
				spApi.get<{ data: Plan[] }>('/api/v1/sp/tariff-plans'),
				spApi.get<{ data: Operator[] }>('/api/v1/sp/operators')
			]);
			plans = planRes.data ?? [];
			operators = operatorRes.data ?? [];
		} catch (e) {
			error = e instanceof SpApiError ? e.message : 'Unable to load tariff plans.';
		} finally {
			loading = false;
		}
	}

	function emptyRates(): Rate[] {
		return operators.map((operator) => ({ operator_id: operator.id, operator: operator.name, masking_price: 0, non_masking_price: 0 }));
	}

	function openCreate() {
		editing = null; name = ''; slug = ''; planType = 'standard'; status = 'draft'; currency = 'BDT'; description = '';
		rates = emptyRates(); error = ''; modalOpen = true;
	}

	async function openEdit(plan: Plan) {
		error = '';
		try {
			const res = await spApi.get<{ data: Plan & { rates: Rate[] } }>(`/api/v1/sp/tariff-plans/${plan.id}`);
			editing = plan;
			name = res.data.name; slug = res.data.slug; planType = res.data.type; status = res.data.status; currency = res.data.currency; description = res.data.description;
			rates = operators.map((operator) => {
				const rate = (res.data.rates ?? []).find((item) => item.operator_id === operator.id);
				return { operator_id: operator.id, operator: operator.name, masking_price: rate?.masking_price ?? 0, non_masking_price: rate?.non_masking_price ?? 0 };
			});
			modalOpen = true;
		} catch (e) {
			error = e instanceof SpApiError ? e.message : 'Unable to open plan.';
		}
	}

	async function save(e: SubmitEvent) {
		e.preventDefault();
		saving = true; error = '';
		try {
			const body = { name, slug, type: planType, status, currency, description, rates };
			if (editing) await spApi.put(`/api/v1/sp/tariff-plans/${editing.id}`, body);
			else await spApi.post('/api/v1/sp/tariff-plans', body);
			success = editing ? 'Tariff plan updated.' : 'Tariff plan created.';
			modalOpen = false;
			await load();
			setTimeout(() => (success = ''), 5000);
		} catch (e) {
			error = e instanceof SpApiError ? e.message : 'Unable to save tariff plan.';
		} finally {
			saving = false;
		}
	}

	async function archive(plan: Plan) {
		if (!confirm(`Archive "${plan.name}"? It can no longer be assigned to new customers.`)) return;
		try {
			await spApi.post(`/api/v1/sp/tariff-plans/${plan.id}/archive`);
			success = 'Plan archived.';
			await load();
			setTimeout(() => (success = ''), 5000);
		} catch (e) {
			error = e instanceof SpApiError ? e.message : 'Unable to archive plan.';
		}
	}

	onMount(load);
</script>

<svelte:head><title>Tariff Plans — SMS Whiteinfo S P Reseller</title></svelte:head>

<div class="space-y-4">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-bold tracking-tight text-gray-950 dark:text-white">Tariff Plans</h1>
			<p class="text-xs text-gray-500 dark:text-gray-400">Retail plans and per-operator rates assigned to your customers.</p>
		</div>
		{#if spAuth.can('billing.manage')}
			<button type="button" onclick={openCreate} class="rounded-lg bg-amber-500 px-3 py-1.5 text-xs font-semibold text-white hover:bg-amber-600">Add Plan</button>
		{/if}
	</div>

	{#if error}<div class="rounded-xl border border-rose-200 bg-rose-50 p-3 text-xs text-rose-700 dark:border-rose-900 dark:bg-rose-950/40 dark:text-rose-300">{error}</div>{/if}
	{#if success}<div class="rounded-xl border border-emerald-200 bg-emerald-50 p-3 text-xs text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-300">{success}</div>{/if}

	<div class="overflow-x-auto rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
		<table class="w-full text-left text-xs">
			<thead><tr class="border-b border-gray-100 text-[10px] uppercase text-gray-400 dark:border-gray-800">
				<th class="px-4 py-3">Plan</th><th class="px-4 py-3">Slug</th><th class="px-4 py-3">Type</th><th class="px-4 py-3">Status</th><th class="px-4 py-3">Customers</th><th class="px-4 py-3 text-right">Actions</th>
			</tr></thead>
			<tbody class="divide-y divide-gray-100 dark:divide-gray-800">
				{#if loading}
					<tr><td colspan="6" class="px-4 py-10 text-center text-gray-400">Loading tariff plans…</td></tr>
				{:else if plans.length === 0}
					<tr><td colspan="6" class="px-4 py-10 text-center text-gray-400">No tariff plans yet.</td></tr>
				{:else}
					{#each plans as plan (plan.id)}
						<tr>
							<td class="px-4 py-3 font-semibold text-gray-900 dark:text-white">{plan.name}<div class="text-[11px] font-normal text-gray-400">{plan.description || '—'}</div></td>
							<td class="px-4 py-3 font-mono">{plan.slug}</td>
							<td class="px-4 py-3">{plan.type}</td>
							<td class="px-4 py-3"><span class="rounded-full bg-amber-100 px-2 py-0.5 text-[10px] font-semibold text-amber-800 dark:bg-amber-950 dark:text-amber-300">{plan.status}</span></td>
							<td class="px-4 py-3">{plan.customers_count}</td>
							<td class="px-4 py-3 text-right whitespace-nowrap">
								{#if spAuth.can('billing.manage')}
									<button type="button" onclick={() => openEdit(plan)} class="rounded-lg border border-gray-200 px-2 py-1 font-semibold text-gray-600 dark:border-gray-700 dark:text-gray-300">Edit</button>
									{#if plan.status !== 'archived'}<button type="button" onclick={() => archive(plan)} class="ml-1 rounded-lg border border-amber-200 px-2 py-1 font-semibold text-amber-700 dark:border-amber-900">Archive</button>{/if}
								{/if}
							</td>
						</tr>
					{/each}
				{/if}
			</tbody>
		</table>
	</div>
</div>

{#if modalOpen}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-gray-950/50 p-4">
		<div class="max-h-[90vh] w-full max-w-3xl overflow-y-auto space-y-4 rounded-2xl bg-white p-6 shadow-2xl dark:bg-gray-900">
			<div class="flex items-center justify-between border-b border-gray-100 pb-3 dark:border-gray-800">
				<h3 class="text-sm font-bold text-gray-900 dark:text-white">{editing ? 'Edit Tariff Plan' : 'Add Tariff Plan'}</h3>
				<button type="button" onclick={() => (modalOpen = false)} class="text-gray-400">✕</button>
			</div>
			{#if error}<p class="rounded-lg bg-rose-50 p-3 text-xs text-rose-700 dark:bg-rose-950/40 dark:text-rose-300">{error}</p>{/if}
			<form onsubmit={save} class="space-y-4 text-xs">
				<div class="grid gap-3 sm:grid-cols-2">
					<div><label for="plan-name" class="font-medium text-gray-700 dark:text-gray-300">Name</label><input id="plan-name" bind:value={name} required class="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2 dark:border-gray-700 dark:bg-gray-800 dark:text-white" /></div>
					<div><label for="plan-slug" class="font-medium text-gray-700 dark:text-gray-300">Slug</label><input id="plan-slug" bind:value={slug} placeholder="auto from name" class="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2 dark:border-gray-700 dark:bg-gray-800 dark:text-white" /></div>
					<div><label for="plan-type" class="font-medium text-gray-700 dark:text-gray-300">Type</label><select id="plan-type" bind:value={planType} class="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2 dark:border-gray-700 dark:bg-gray-800 dark:text-white"><option value="standard">Standard</option><option value="custom">Custom</option></select></div>
					<div><label for="plan-status" class="font-medium text-gray-700 dark:text-gray-300">Status</label><select id="plan-status" bind:value={status} class="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2 dark:border-gray-700 dark:bg-gray-800 dark:text-white"><option value="draft">Draft</option><option value="active">Active</option><option value="archived">Archived</option></select></div>
					<div><label for="plan-currency" class="font-medium text-gray-700 dark:text-gray-300">Currency</label><input id="plan-currency" bind:value={currency} class="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2 dark:border-gray-700 dark:bg-gray-800 dark:text-white" /></div>
				</div>
				<div><label for="plan-description" class="font-medium text-gray-700 dark:text-gray-300">Description</label><textarea id="plan-description" bind:value={description} rows="2" class="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2 dark:border-gray-700 dark:bg-gray-800 dark:text-white"></textarea></div>
				<div>
					<p class="mb-2 font-semibold text-gray-700 dark:text-gray-300">Per-operator rates (BDT)</p>
					<div class="space-y-2">
						{#each rates as rate}
							<div class="grid grid-cols-3 items-center gap-2">
								<span class="text-gray-600 dark:text-gray-300">{rate.operator}</span>
								<label class="flex items-center gap-1">Masking <input type="number" step="0.0001" bind:value={rate.masking_price} class="w-full rounded-lg border border-gray-300 px-2 py-1 dark:border-gray-700 dark:bg-gray-800 dark:text-white" /></label>
								<label class="flex items-center gap-1">Non-masking <input type="number" step="0.0001" bind:value={rate.non_masking_price} class="w-full rounded-lg border border-gray-300 px-2 py-1 dark:border-gray-700 dark:bg-gray-800 dark:text-white" /></label>
							</div>
						{/each}
					</div>
				</div>
				<div class="flex justify-end gap-2">
					<button type="button" onclick={() => (modalOpen = false)} class="rounded-lg border border-gray-200 px-3 py-1.5 font-semibold text-gray-600 dark:border-gray-700 dark:text-gray-300">Cancel</button>
					<button type="submit" disabled={saving} class="rounded-lg bg-amber-500 px-4 py-1.5 font-semibold text-white disabled:opacity-50">{saving ? 'Saving…' : 'Save'}</button>
				</div>
			</form>
		</div>
	</div>
{/if}
