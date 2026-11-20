<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { supermanApi } from '$lib/superman-api.js';
	import {
		computeSmsStats,
		extractVariables,
		type CustomerOption
	} from '$lib/template-admin.js';

	let customers = $state<CustomerOption[]>([]);
	let loadingOptions = $state(true);

	let userId = $state<number | ''>('');
	let name = $state('');
	let body = $state('');
	let manualVariables = $state<string[]>([]);
	let newVariableInput = $state('');

	let submitting = $state(false);
	let error = $state<string | null>(null);
	let flash = $state<{ type: 'success' | 'error'; text: string } | null>(null);
	let flashTimer: ReturnType<typeof setTimeout> | undefined;

	let smsStats = $derived(computeSmsStats(body));
	let bodyVariables = $derived(extractVariables(body));

	let allVariables = $derived.by<string[]>(() => {
		const set = new Set([...bodyVariables, ...manualVariables]);
		return Array.from(set);
	});

	function notify(type: 'success' | 'error', text: string) {
		flash = { type, text };
		clearTimeout(flashTimer);
		flashTimer = setTimeout(() => (flash = null), 5000);
	}

	async function loadOptions() {
		loadingOptions = true;
		try {
			const res = await supermanApi.get<{ data: { customers: CustomerOption[] } }>(
				'/api/v1/superman/templates/options'
			);
			customers = res.data?.customers ?? [];
		} catch (err: unknown) {
			error = err instanceof Error ? err.message : 'Failed to load customers.';
		} finally {
			loadingOptions = false;
		}
	}

	function addManualVariable() {
		const val = newVariableInput.trim().replace(/^\{|\}$/g, '');
		if (val && !allVariables.includes(val)) {
			manualVariables = [...manualVariables, val];
			newVariableInput = '';
		}
	}

	function removeVariable(v: string) {
		manualVariables = manualVariables.filter((item) => item !== v);
	}

	async function submit(createAnother = false) {
		error = null;
		if (!userId) {
			error = 'The customer field is required.';
			return;
		}
		if (!name.trim()) {
			error = 'The template name field is required.';
			return;
		}
		if (!body.trim()) {
			error = 'The message body field is required.';
			return;
		}

		submitting = true;
		try {
			await supermanApi.post('/api/v1/superman/templates', {
				user_id: Number(userId),
				name: name.trim(),
				body: body.trim(),
				variables: allVariables
			});

			if (createAnother) {
				notify('success', 'Bulk SMS Template created successfully.');
				name = '';
				body = '';
				manualVariables = [];
				newVariableInput = '';
			} else {
				await goto('/superman/templates');
			}
		} catch (err: unknown) {
			error = err instanceof Error ? err.message : 'Unable to create template.';
		} finally {
			submitting = false;
		}
	}

	onMount(() => {
		void loadOptions();
	});
</script>

<svelte:head>
	<title>Create Bulk SMS Template — Superman Administrator</title>
</svelte:head>

<div class="mx-auto max-w-4xl space-y-5">
	<!-- Flash message -->
	{#if flash}
		<div
			class="flex items-center justify-between rounded-lg border px-4 py-3 text-sm shadow-xs {flash.type ===
			'success'
				? 'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-200'
				: 'border-rose-200 bg-rose-50 text-rose-800 dark:border-rose-800 dark:bg-rose-950/40 dark:text-rose-200'}"
		>
			<span>{flash.text}</span>
			<button onclick={() => (flash = null)} class="text-xs font-semibold hover:underline">✕</button>
		</div>
	{/if}

	<!-- Breadcrumb + Title -->
	<div>
		<nav class="flex items-center gap-1.5 text-xs text-gray-500 dark:text-gray-400">
			<a href="/superman/templates" class="hover:text-gray-800 dark:hover:text-gray-200">
				Bulk SMS Templates
			</a>
			<span class="text-gray-400">›</span>
			<span class="font-medium text-gray-700 dark:text-gray-300">Create</span>
		</nav>
		<h1 class="mt-1 text-2xl font-bold tracking-tight text-gray-950 dark:text-white">
			Create Bulk SMS Template
		</h1>
	</div>

	<!-- Error Alert -->
	{#if error}
		<div
			class="rounded-xl border border-rose-200 bg-rose-50 p-4 text-xs font-medium text-rose-800 dark:border-rose-800/60 dark:bg-rose-950/40 dark:text-rose-300"
		>
			{error}
		</div>
	{/if}

	<!-- Section Card: Bulk SMS Template Details -->
	<div
		class="overflow-hidden rounded-2xl border border-gray-200 bg-white p-6 shadow-xs dark:border-gray-800 dark:bg-gray-900"
	>
		<div class="border-b border-gray-100 pb-4 dark:border-gray-800">
			<h2 class="text-base font-bold text-gray-950 dark:text-white">
				Bulk SMS Template Details
			</h2>
			<p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
				Create or edit reusable message templates for customer bulk SMS campaigns.
			</p>
		</div>

		<form
			onsubmit={(e) => {
				e.preventDefault();
				void submit(false);
			}}
			class="mt-6 space-y-5"
		>
			<!-- Row 1: Customer (select) & Template Name (text) -->
			<div class="grid grid-cols-1 gap-5 sm:grid-cols-2">
				<div>
					<label for="template-customer" class="block text-xs font-semibold text-gray-800 dark:text-gray-200">
						Customer <span class="text-rose-500">*</span>
					</label>
					{#if loadingOptions}
						<div class="mt-1.5 text-xs text-gray-400">Loading customers...</div>
					{:else}
						<select
							id="template-customer"
							bind:value={userId}
							required
							class="mt-1.5 w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-xs text-gray-900 outline-hidden transition focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500 dark:border-gray-700 dark:bg-gray-800 dark:text-white"
						>
							<option value="">Select a customer</option>
							{#each customers as c}
								<option value={c.id}>{c.name} ({c.email})</option>
							{/each}
						</select>
					{/if}
				</div>

				<div>
					<label for="template-name" class="block text-xs font-semibold text-gray-800 dark:text-gray-200">
						Template Name <span class="text-rose-500">*</span>
					</label>
					<input
						type="text"
						id="template-name"
						bind:value={name}
						placeholder="e.g. Eid Promotion, Order Confirmation"
						required
						maxlength="255"
						class="mt-1.5 w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-xs text-gray-900 outline-hidden transition focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500 dark:border-gray-700 dark:bg-gray-800 dark:text-white"
					/>
				</div>
			</div>

			<!-- Row 2: Message Body (full width) -->
			<div>
				<label for="template-body" class="block text-xs font-semibold text-gray-800 dark:text-gray-200">
					Message Body <span class="text-rose-500">*</span>
				</label>
				<textarea
					id="template-body"
					bind:value={body}
					rows="6"
					placeholder="বাংলায় আপনার এসএমএস লিখুন... e.g. প্রিয় &#123;name&#125;, আপনার অফার..."
					required
					class="mt-1.5 w-full rounded-lg border border-gray-300 p-3 text-xs leading-relaxed text-gray-900 outline-hidden transition focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500 dark:border-gray-700 dark:bg-gray-800 dark:text-white"
				></textarea>

				<!-- SMS Stats Bar (from sms-stats.blade.php) -->
				<div class="mt-2 space-y-2">
					<div
						class="flex flex-wrap items-center gap-x-5 gap-y-1 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-xs text-gray-600 dark:border-gray-800 dark:bg-gray-800/60 dark:text-gray-400"
					>
						<div class="flex items-center gap-1.5">
							<span class="font-medium text-gray-700 dark:text-gray-300">Remaining:</span>
							<span>{smsStats.remaining}/{smsStats.total}</span>
						</div>
						<div class="flex items-center gap-1.5">
							<span class="font-medium text-gray-700 dark:text-gray-300">SMS Count:</span>
							<span class="font-bold text-emerald-600 dark:text-emerald-400">{smsStats.parts}</span>
						</div>
						<div class="flex items-center gap-1.5">
							<span class="font-medium text-gray-700 dark:text-gray-300">Encoding:</span>
							<span class="font-semibold {smsStats.encoding === 'Unicode' ? 'text-amber-600' : 'text-blue-600'}">
								{smsStats.encoding}
							</span>
						</div>
						<div class="flex items-center gap-1.5">
							<span class="font-medium text-gray-700 dark:text-gray-300">Non-masking:</span>
							<span>—/SMS</span>
						</div>
						<div class="flex items-center gap-1.5">
							<span class="font-medium text-gray-700 dark:text-gray-300">Masking:</span>
							<span>—/SMS</span>
						</div>
					</div>

					<!-- BTRC Compliance Notice -->
					<div
						class="flex items-center gap-2 rounded-lg border border-amber-500/25 bg-amber-500/10 px-3 py-2 text-xs leading-relaxed text-amber-800 dark:text-amber-300"
					>
						<svg
							class="h-4 w-4 shrink-0 text-amber-600 dark:text-amber-400"
							fill="none"
							viewBox="0 0 24 24"
							stroke="currentColor"
						>
							<path
								stroke-linecap="round"
								stroke-linejoin="round"
								stroke-width="2"
								d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
							/>
						</svg>
						<span>
							<strong class="font-semibold text-amber-900 dark:text-amber-200">BTRC নির্দেশনা:</strong>
							বিটিআরসি (BTRC) নিয়মানুযায়ী সকল প্রকার প্রমোশনাল ও বাল্ক এসএমএস বাংলায় পাঠানো বাধ্যতামূলক।
						</span>
					</div>
				</div>
			</div>

			<!-- Row 3: Variables / Placeholders (tags input) -->
			<div>
				<label for="var-input" class="block text-xs font-semibold text-gray-800 dark:text-gray-200">
					Variables / Placeholders
				</label>
				<div
					class="mt-1.5 min-h-[42px] w-full rounded-lg border border-gray-300 bg-white p-2 dark:border-gray-700 dark:bg-gray-800"
				>
					<div class="flex flex-wrap items-center gap-1.5">
						{#each allVariables as v}
							<span
								class="inline-flex items-center gap-1 rounded-md bg-emerald-50 px-2 py-1 text-xs font-semibold text-emerald-700 dark:bg-emerald-950/50 dark:text-emerald-300"
							>
								<span>&#123;{v}&#125;</span>
								<button
									type="button"
									onclick={() => removeVariable(v)}
									class="text-emerald-600 hover:text-emerald-800 dark:text-emerald-400"
									aria-label="Remove variable"
								>
									✕
								</button>
							</span>
						{/each}
						<input
							id="var-input"
							type="text"
							bind:value={newVariableInput}
							onkeydown={(e) => {
								if (e.key === 'Enter' || e.key === ',') {
									e.preventDefault();
									addManualVariable();
								}
							}}
							placeholder={allVariables.length === 0 ? "Add placeholder e.g. name, code, amount" : "Type variable and press Enter"}
							class="min-w-[180px] flex-1 bg-transparent text-xs text-gray-900 outline-hidden dark:text-white"
						/>
					</div>
				</div>
				<p class="mt-1 text-[11px] text-gray-500 dark:text-gray-400">
					Variables in &#123;curly_braces&#125; will be automatically extracted.
				</p>
			</div>

			<!-- Form Actions -->
			<div class="flex items-center gap-3 pt-4 border-t border-gray-100 dark:border-gray-800">
				<button
					type="submit"
					disabled={submitting}
					class="inline-flex items-center gap-1.5 rounded-lg bg-emerald-600 px-4 py-2 text-xs font-semibold text-white shadow-xs transition hover:bg-emerald-700 disabled:opacity-50"
				>
					{#if submitting}
						<svg class="h-3.5 w-3.5 animate-spin" viewBox="0 0 24 24" fill="none">
							<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
							<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
						</svg>
					{/if}
					<span>Create</span>
				</button>

				<button
					type="button"
					onclick={() => void submit(true)}
					disabled={submitting}
					class="rounded-lg border border-gray-300 bg-white px-3.5 py-2 text-xs font-semibold text-gray-700 shadow-xs transition hover:bg-gray-50 disabled:opacity-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200 dark:hover:bg-gray-700"
				>
					Create & create another
				</button>

				<a
					href="/superman/templates"
					class="rounded-lg px-3.5 py-2 text-xs font-semibold text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-gray-200"
				>
					Cancel
				</a>
			</div>
		</form>
	</div>
</div>
