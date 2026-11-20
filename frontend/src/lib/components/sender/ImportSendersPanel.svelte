<script lang="ts">
	import { supermanApi } from '$lib/superman-api.js';
	import {
		IMPORT_TEMPLATE,
		downloadTextFile,
		parseSenderCsv,
		type ImportRow,
		type SenderCustomer,
		type SenderGateway
	} from '$lib/sender-admin.js';

	interface Props {
		gateways: SenderGateway[];
		customers: SenderCustomer[];
		onDone: (message: string) => void;
		onCancel: () => void;
	}

	interface ImportResult {
		message: string;
		data?: { created: number; updated: number; failed: number; errors: { row: number; sender_id: string; message: string }[] };
	}

	let { gateways, customers, onDone, onCancel }: Props = $props();

	let fileName = $state('');
	let rows = $state<ImportRow[]>([]);
	let defaultGatewayId = $state('');
	let defaultType = $state('masking');
	let customerId = $state('');
	let customerTerm = $state('');
	let customerOpen = $state(false);
	let submitting = $state(false);
	let error = $state<string | null>(null);
	let rowErrors = $state<{ row: number; sender_id: string; message: string }[]>([]);
	let dragging = $state(false);

	let selectedCustomer = $derived(customers.find((c) => String(c.id) === customerId) ?? null);
	let matchingCustomers = $derived(
		customers
			.filter((c) => {
				const q = customerTerm.trim().toLowerCase();
				return !q || c.name.toLowerCase().includes(q) || (c.email ?? '').toLowerCase().includes(q);
			})
			.slice(0, 50)
	);

	async function readFile(file: File | undefined) {
		error = null;
		rowErrors = [];
		if (!file) return;
		if (!/\.(csv|txt)$/i.test(file.name)) {
			error = 'Please upload a .csv file. Excel users: use File → Save As → CSV.';
			return;
		}
		if (file.size > 5 * 1024 * 1024) {
			error = 'The file is larger than 5 MB.';
			return;
		}
		const parsed = parseSenderCsv(await file.text());
		if (parsed.length === 0) {
			error = 'The file has no sender rows.';
			return;
		}
		fileName = file.name;
		rows = parsed;
	}

	async function submit() {
		error = null;
		rowErrors = [];
		if (rows.length === 0) {
			error = 'Upload a CSV file first.';
			return;
		}
		const needsDefaultGateway = rows.some((r) => !r.gateway.trim());
		if (needsDefaultGateway && !defaultGatewayId) {
			error = 'Some rows have no gateway. Choose a default gateway.';
			return;
		}

		submitting = true;
		try {
			const res = await supermanApi.post<ImportResult>('/api/v1/superman/senders/import', {
				rows,
				default_gateway_id: defaultGatewayId ? Number(defaultGatewayId) : 0,
				default_type: defaultType,
				user_id: customerId ? Number(customerId) : 0
			});
			if (res.data?.errors?.length) {
				rowErrors = res.data.errors;
			}
			if (!res.data?.failed) {
				onDone(res.message);
			} else {
				error = res.message;
			}
		} catch (err: unknown) {
			const data = (err as { data?: ImportResult['data'] })?.data;
			if (data?.errors?.length) rowErrors = data.errors;
			error = err instanceof Error ? err.message : 'Unable to import senders.';
		} finally {
			submitting = false;
		}
	}

	const inputClass =
		'w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 outline-none focus:border-emerald-600 focus:ring-1 focus:ring-emerald-600 dark:border-gray-700 dark:bg-gray-800 dark:text-white';
</script>

<div class="space-y-4" data-testid="import-senders-panel">
	<button
		type="button"
		onclick={() => downloadTextFile('senders-example.csv', IMPORT_TEMPLATE)}
		class="text-sm font-semibold text-emerald-700 hover:underline dark:text-emerald-400"
	>
		Download example CSV file
	</button>

	<label
		class="flex cursor-pointer flex-col items-center justify-center rounded-lg border border-dashed px-4 py-8 text-sm transition {dragging
			? 'border-emerald-500 bg-emerald-50/60 dark:bg-emerald-950/20'
			: 'border-gray-300 bg-gray-50/60 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800/40'}"
		ondragover={(e) => {
			e.preventDefault();
			dragging = true;
		}}
		ondragleave={() => (dragging = false)}
		ondrop={(e) => {
			e.preventDefault();
			dragging = false;
			void readFile(e.dataTransfer?.files?.[0]);
		}}
	>
		{#if fileName}
			<span class="font-semibold text-gray-900 dark:text-white">{fileName}</span>
			<span class="mt-1 text-xs text-gray-500">{rows.length.toLocaleString()} sender row(s) ready · click to replace</span>
		{:else}
			<span class="text-gray-500">Upload a CSV file</span>
			<span class="mt-1 text-xs text-gray-400">Columns: sender_id, type (optional), gateway (optional)</span>
		{/if}
		<input
			type="file"
			accept=".csv,.txt,text/csv"
			class="sr-only"
			data-testid="import-file"
			onchange={(e) => void readFile(e.currentTarget.files?.[0])}
		/>
	</label>

	<div>
		<label class="mb-1 block text-sm font-medium text-gray-900 dark:text-white" for="import-gateway">Default Gateway (if not in file)</label>
		<select id="import-gateway" bind:value={defaultGatewayId} class={inputClass}>
			<option value="">Select an option</option>
			{#each gateways as gateway (gateway.id)}
				<option value={String(gateway.id)}>{gateway.name}</option>
			{/each}
		</select>
	</div>

	<div>
		<label class="mb-1 block text-sm font-medium text-gray-900 dark:text-white" for="import-type">Default Sender Type (if not in file)</label>
		<select id="import-type" bind:value={defaultType} class={inputClass}>
			<option value="masking">Masking</option>
			<option value="non-masking">Non-masking</option>
		</select>
	</div>

	<div class="relative">
		<label class="mb-1 block text-sm font-medium text-gray-900 dark:text-white" for="import-customer">Assign to Customer (Optional - leave empty for unassigned)</label>
		<div class="relative">
			<input
				id="import-customer"
				type="text"
				autocomplete="off"
				class={inputClass}
				placeholder="Leave empty to keep unassigned"
				value={customerOpen ? customerTerm : (selectedCustomer?.name ?? '')}
				onfocus={() => {
					customerOpen = true;
					customerTerm = '';
				}}
				oninput={(e) => (customerTerm = e.currentTarget.value)}
				onblur={() => setTimeout(() => (customerOpen = false), 150)}
			/>
			{#if customerId}
				<button
					type="button"
					aria-label="Clear customer"
					onclick={() => {
						customerId = '';
						customerTerm = '';
					}}
					class="absolute right-2 top-2 text-gray-400 hover:text-rose-600"
				>×</button>
			{/if}
		</div>
		{#if customerOpen}
			<ul class="absolute z-10 mt-1 max-h-48 w-full overflow-y-auto rounded-lg border border-gray-200 bg-white shadow-lg dark:border-gray-700 dark:bg-gray-900">
				{#each matchingCustomers as customer (customer.id)}
					<li>
						<button
							type="button"
							onmousedown={(e) => {
								e.preventDefault();
								customerId = String(customer.id);
								customerOpen = false;
							}}
							class="block w-full px-3 py-1.5 text-left text-sm hover:bg-gray-50 dark:hover:bg-gray-800"
						>
							{customer.name}
							{#if customer.email}<span class="text-xs text-gray-400"> · {customer.email}</span>{/if}
						</button>
					</li>
				{:else}
					<li class="px-3 py-3 text-center text-sm text-gray-400">No customers found.</li>
				{/each}
			</ul>
		{/if}
		<p class="mt-1.5 text-xs text-gray-500">Caution: Selecting a customer here will assign all imported Sender IDs to that customer. Leave empty to keep unassigned.</p>
	</div>

	{#if error}
		<div role="alert" class="rounded-lg border border-rose-200 bg-rose-50 p-3 text-xs text-rose-700 dark:border-rose-900 dark:bg-rose-950/30 dark:text-rose-300">{error}</div>
	{/if}
	{#if rowErrors.length > 0}
		<ul class="max-h-32 overflow-y-auto rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-800 dark:border-amber-900 dark:bg-amber-950/20 dark:text-amber-300">
			{#each rowErrors.slice(0, 50) as item (item.row)}
				<li>Row {item.row}{item.sender_id ? ` (${item.sender_id})` : ''}: {item.message}</li>
			{/each}
			{#if rowErrors.length > 50}<li>…and {rowErrors.length - 50} more.</li>{/if}
		</ul>
	{/if}

	<div class="flex items-center gap-2">
		<button
			type="button"
			onclick={() => void submit()}
			disabled={submitting}
			data-testid="import-submit"
			class="rounded-lg border border-emerald-600 px-4 py-2 text-sm font-semibold text-emerald-700 transition hover:bg-emerald-50 disabled:opacity-50 dark:text-emerald-400 dark:hover:bg-emerald-950/30"
		>
			{submitting ? 'Importing…' : 'Import'}
		</button>
		<button
			type="button"
			onclick={onCancel}
			class="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800"
		>
			Cancel
		</button>
	</div>
</div>
