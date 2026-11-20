<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { supermanApi } from '$lib/superman-api.js';
	import CustomerPickerModal from '$lib/components/sender/CustomerPickerModal.svelte';
	import ImportSendersPanel from '$lib/components/sender/ImportSendersPanel.svelte';
	import {
		OPERATORS,
		loadSenderOptions,
		type AdminSender,
		type SenderCustomer,
		type SenderGateway
	} from '$lib/sender-admin.js';

	interface Props {
		/** When set the form edits this sender instead of creating one. */
		senderId?: number | null;
	}

	let { senderId = null }: Props = $props();
	let editing = $derived(senderId !== null);

	let mode = $state<'single' | 'bulk'>('single');
	let code = $state('');
	let type = $state<'masking' | 'non-masking'>('masking');
	let enabled = $state('yes');
	let customer = $state<SenderCustomer | null>(null);
	let routes = $state<Record<string, string>>({});
	let singleGateway = $state('');

	let gateways = $state<SenderGateway[]>([]);
	let customers = $state<SenderCustomer[]>([]);
	let loading = $state(true);
	let saving = $state(false);
	let showPicker = $state(false);
	let errorMessage = $state<string | null>(null);
	let successMessage = $state<string | null>(null);
	let fieldErrors = $state<Record<string, string>>({});
	let approvalStatus = $state('approved');

	onMount(async () => {
		try {
			const options = await loadSenderOptions();
			gateways = options.gateways;
			customers = options.customers;

			if (senderId !== null) {
				const res = await supermanApi.get<{ data: AdminSender }>(`/api/v1/superman/senders/${senderId}`);
				const sender = res.data;
				code = sender.sender_id;
				type = sender.type === 'numeric' ? 'non-masking' : 'masking';
				enabled = sender.is_enabled ? 'yes' : 'no';
				approvalStatus = sender.approval_status;
				customer = sender.users[0] ?? null;
				singleGateway = sender.gateway_id ? String(sender.gateway_id) : '';
				const next: Record<string, string> = {};
				for (const route of sender.routes) next[route.operator_key] = String(route.gateway_id);
				routes = next;
			}
		} catch (err: unknown) {
			errorMessage = err instanceof Error ? err.message : 'Unable to load the form options.';
		} finally {
			loading = false;
		}
	});

	function validate(): boolean {
		const errors: Record<string, string> = {};
		const value = code.trim();
		if (!value) {
			errors.code = 'The sender ID field is required.';
		} else if (type === 'masking') {
			if (!/^[A-Za-z0-9 ._-]{3,11}$/.test(value)) errors.code = 'A masking sender must be 3–11 letters, digits, spaces, dots, dashes or underscores.';
			else if (/^[0-9]+$/.test(value)) errors.code = 'A masking sender cannot be purely numeric.';
		} else if (!/^\+?[0-9]{4,20}$/.test(value)) {
			errors.code = 'A non-masking sender must be 4–20 digits.';
		}

		if (type === 'masking') {
			for (const operator of OPERATORS) {
				if (!routes[operator.key]) errors[`route_${operator.key}`] = `Select a gateway for ${operator.label}.`;
			}
		} else if (!singleGateway) {
			errors.gateway = 'Select a gateway.';
		}

		fieldErrors = errors;
		return Object.keys(errors).length === 0;
	}

	function buildPayload() {
		const payload: Record<string, unknown> = {
			sender_id: code.trim(),
			type
		};
		// Never flip the enabled flag of a sender that has not been approved.
		if (!editing || approvalStatus === 'approved') payload.is_enabled = enabled === 'yes';
		if (type === 'masking') {
			const routePayload: Record<string, number> = {};
			for (const operator of OPERATORS) routePayload[operator.key] = Number(routes[operator.key]);
			payload.routes = routePayload;
			payload.gateway_id = routePayload.gp;
		} else {
			payload.gateway_id = Number(singleGateway);
		}
		payload.user_ids = customer ? [customer.id] : [];
		return payload;
	}

	function resetForm() {
		code = '';
		customer = null;
		fieldErrors = {};
	}

	async function save(another: boolean) {
		errorMessage = null;
		successMessage = null;
		if (!validate()) {
			errorMessage = 'Please fix the highlighted fields.';
			return;
		}

		saving = true;
		try {
			const payload = buildPayload();
			if (editing) {
				await supermanApi.put(`/api/v1/superman/senders/${senderId}`, payload);
				await goto('/superman/sender-cli/add?saved=updated');
				return;
			}

			const res = await supermanApi.post<{ message: string }>('/api/v1/superman/senders', payload);
			if (another) {
				successMessage = res.message || 'Sender created.';
				resetForm();
			} else {
				await goto('/superman/sender-cli/add?saved=created');
			}
		} catch (err: unknown) {
			errorMessage = err instanceof Error ? err.message : 'Unable to save the sender.';
		} finally {
			saving = false;
		}
	}

	const inputClass =
		'w-full rounded-lg border bg-white px-3 py-2 text-sm text-gray-900 outline-none transition focus:ring-1 dark:bg-gray-800 dark:text-white';
	const okBorder = 'border-gray-300 focus:border-emerald-600 focus:ring-emerald-600 dark:border-gray-700';
	const badBorder = 'border-rose-500 focus:border-rose-500 focus:ring-rose-500';
</script>

<div class="space-y-4">
	{#if errorMessage}
		<div role="alert" data-testid="form-error" class="rounded-lg border border-rose-200 bg-rose-50 p-3 text-sm text-rose-700 dark:border-rose-900 dark:bg-rose-950/30 dark:text-rose-300">{errorMessage}</div>
	{/if}
	{#if successMessage}
		<div role="status" data-testid="form-success" class="rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-300">{successMessage}</div>
	{/if}

	{#if loading}
		<div class="rounded-xl border border-gray-200 bg-white p-8 text-center text-sm text-gray-400 dark:border-gray-800 dark:bg-gray-900">Loading…</div>
	{:else}
		<form
			class="max-w-3xl space-y-6"
			novalidate
			onsubmit={(e) => {
				e.preventDefault();
				void save(false);
			}}
		>
			<!-- Sender details -->
			<section class="rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
				<h2 class="border-b border-gray-200 px-6 py-4 text-base font-semibold text-gray-950 dark:border-gray-800 dark:text-white">Sender Details</h2>

				<div class="space-y-5 p-6">
					{#if !editing}
						<fieldset>
							<legend class="mb-2 text-sm font-medium text-gray-900 dark:text-white">Creation Mode</legend>
							<div class="flex flex-wrap items-center gap-6">
								<label class="flex cursor-pointer items-center gap-2 text-sm text-gray-800 dark:text-gray-200">
									<input type="radio" name="mode" value="single" bind:group={mode} class="h-4 w-4 accent-emerald-600" />
									Single Sender
								</label>
								<label class="flex cursor-pointer items-center gap-2 text-sm text-gray-800 dark:text-gray-200">
									<input type="radio" name="mode" value="bulk" bind:group={mode} class="h-4 w-4 accent-emerald-600" />
									Bulk Import (CSV / Excel Sheet)
								</label>
							</div>
						</fieldset>
					{/if}

					{#if mode === 'bulk' && !editing}
						<ImportSendersPanel
							{gateways}
							{customers}
							onDone={async (message) => {
								await goto(`/superman/sender-cli/add?saved=${encodeURIComponent(message)}`);
							}}
							onCancel={() => goto('/superman/sender-cli/add')}
						/>
					{:else}
						<div class="grid grid-cols-1 gap-5 sm:grid-cols-2">
							<div>
								<label for="sender-code" class="mb-1 block text-sm font-medium text-gray-900 dark:text-white">Sender ID<span class="text-rose-500">*</span></label>
								<input
									id="sender-code"
									type="text"
									bind:value={code}
									autocomplete="off"
									class="{inputClass} {fieldErrors.code ? badBorder : okBorder}"
									aria-invalid={fieldErrors.code ? 'true' : undefined}
								/>
								{#if fieldErrors.code}<p class="mt-1 text-xs text-rose-600">{fieldErrors.code}</p>{/if}
							</div>
							<div>
								<label for="sender-type" class="mb-1 block text-sm font-medium text-gray-900 dark:text-white">Type<span class="text-rose-500">*</span></label>
								<select id="sender-type" bind:value={type} class="{inputClass} {okBorder}">
									<option value="masking">Masking</option>
									<option value="non-masking">Non-masking</option>
								</select>
							</div>
						</div>

						<div class="max-w-[14rem]">
							<label for="sender-enabled" class="mb-1 block text-sm font-medium text-gray-900 dark:text-white">Enabled</label>
							<select
								id="sender-enabled"
								bind:value={enabled}
								disabled={approvalStatus !== 'approved'}
								class="{inputClass} {okBorder} disabled:opacity-60"
							>
								<option value="yes">Yes</option>
								<option value="no">No</option>
							</select>
							{#if approvalStatus !== 'approved'}
								<p class="mt-1 text-xs text-gray-400">Only approved senders can be enabled.</p>
							{/if}
						</div>

						<fieldset class="rounded-xl border border-gray-200 px-4 pb-4 pt-2 dark:border-gray-800">
							<legend class="px-1 text-sm font-medium text-gray-900 dark:text-white">Assigned Customer (Single Customer Only)</legend>
							<div class="flex flex-wrap items-center gap-3">
								<button
									type="button"
									onclick={() => (showPicker = true)}
									data-testid="select-customer"
									class="inline-flex items-center gap-1.5 text-sm font-semibold text-emerald-700 hover:underline dark:text-emerald-400"
								>
									Select
									<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L6.832 19.82a4.5 4.5 0 0 1-1.897 1.13l-2.685.8.8-2.685a4.5 4.5 0 0 1 1.13-1.897L16.863 4.487Zm0 0L19.5 7.125" /></svg>
								</button>
								{#if customer}
									<span class="inline-flex items-center gap-2 rounded-full border border-emerald-200 bg-emerald-50 px-2.5 py-1 text-xs font-medium text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-300" data-testid="assigned-customer">
										{customer.name}
										<button type="button" aria-label="Remove customer" onclick={() => (customer = null)} class="text-emerald-700/70 hover:text-rose-600">×</button>
									</span>
								{:else}
									<span class="text-xs text-gray-400">Unassigned</span>
								{/if}
							</div>
						</fieldset>

						{#if type === 'masking'}
							<div class="rounded-xl border border-gray-200 dark:border-gray-800">
								<div class="border-b border-gray-200 px-5 py-4 dark:border-gray-800">
									<h3 class="text-sm font-semibold text-gray-950 dark:text-white">Operator Gateway Routing (Mandatory for Masking)</h3>
									<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">Assign dedicated telco gateways for each mobile operator. In masking, every operator gateway is strictly mandatory.</p>
								</div>
								<div class="grid grid-cols-1 gap-4 p-5 sm:grid-cols-3">
									{#each OPERATORS as operator (operator.key)}
										<div class="rounded-xl border border-gray-200 p-4 dark:border-gray-800">
											<label for="route-{operator.key}" class="mb-2 block text-sm font-medium text-gray-900 dark:text-white">
												{operator.label} ({operator.codes})<span class="text-rose-500">*</span>
											</label>
											<select
												id="route-{operator.key}"
												bind:value={routes[operator.key]}
												class="{inputClass} {fieldErrors[`route_${operator.key}`] ? badBorder : okBorder}"
												aria-invalid={fieldErrors[`route_${operator.key}`] ? 'true' : undefined}
											>
												<option value="">Select Gateway</option>
												{#each gateways as gateway (gateway.id)}
													<option value={String(gateway.id)}>{gateway.name}</option>
												{/each}
											</select>
											{#if fieldErrors[`route_${operator.key}`]}<p class="mt-1 text-xs text-rose-600">{fieldErrors[`route_${operator.key}`]}</p>{/if}
										</div>
									{/each}
								</div>
							</div>
						{:else}
							<div class="rounded-xl border border-gray-200 dark:border-gray-800">
								<div class="border-b border-gray-200 px-5 py-4 dark:border-gray-800">
									<h3 class="text-sm font-semibold text-gray-950 dark:text-white">Gateway</h3>
									<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">Non-masking senders use a single gateway for every operator.</p>
								</div>
								<div class="max-w-sm p-5">
									<label for="single-gateway" class="mb-2 block text-sm font-medium text-gray-900 dark:text-white">Gateway<span class="text-rose-500">*</span></label>
									<select id="single-gateway" bind:value={singleGateway} class="{inputClass} {fieldErrors.gateway ? badBorder : okBorder}">
										<option value="">Select Gateway</option>
										{#each gateways as gateway (gateway.id)}
											<option value={String(gateway.id)}>{gateway.name}</option>
										{/each}
									</select>
									{#if fieldErrors.gateway}<p class="mt-1 text-xs text-rose-600">{fieldErrors.gateway}</p>{/if}
								</div>
							</div>
						{/if}
					{/if}
				</div>
			</section>

			{#if !(mode === 'bulk' && !editing)}

				<div class="flex flex-wrap items-center gap-2">
					<button
						type="submit"
						disabled={saving}
						data-testid="save-sender"
						class="rounded-lg border border-emerald-600 px-4 py-2 text-sm font-semibold text-emerald-700 transition hover:bg-emerald-50 disabled:opacity-50 dark:text-emerald-400 dark:hover:bg-emerald-950/30"
					>
						{saving ? 'Saving…' : editing ? 'Save changes' : 'Create'}
					</button>
					{#if !editing}
						<button
							type="button"
							disabled={saving}
							onclick={() => void save(true)}
							data-testid="save-another"
							class="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition hover:bg-gray-50 disabled:opacity-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800"
						>
							Create &amp; create another
						</button>
					{/if}
					<a
						href="/superman/sender-cli/add"
						class="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800"
					>
						Cancel
					</a>
				</div>
			{/if}
		</form>
	{/if}
</div>

{#if showPicker}
	<CustomerPickerModal
		{customers}
		selectedId={customer?.id ?? null}
		onSelect={(picked) => {
			customer = picked;
			showPicker = false;
		}}
		onClose={() => (showPicker = false)}
	/>
{/if}
