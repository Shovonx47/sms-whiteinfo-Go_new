<script lang="ts">
	import { api } from '$lib/api.js';
	import { toast } from '$lib/stores/toast.svelte.js';
	import { computeSmsStats } from '$lib/utils/sms.js';
	import { onMount } from 'svelte';

	interface Template {
		id: number;
		name: string;
		body: string;
		status: string;
		created_at: string;
		rejection_reason?: string;
	}

	interface Meta {
		total: number;
		per_page: number;
		current_page: number;
	}

	let templates = $state<Template[]>([]);
	let meta = $state<Meta>({ total: 0, per_page: 10, current_page: 1 });
	let loading = $state(true);
	let search = $state('');
	let approvalFilter = $state('');
	let selectedIds = $state<number[]>([]);
	let bulkDeleting = $state(false);
	let perPage = $state(10);
	let currentPage = $state(1);
	let totalPages = $derived(Math.max(1, Math.ceil((meta.total || 0) / perPage)));

	interface PricingRate {
		masking_rate: number | null;
		non_masking_rate: number | null;
	}
	let pricingRates = $state<PricingRate[]>([]);
	let newBody = $state('');
	let createStats = $derived(computeSmsStats(newBody));
	let detectedVariables = $derived([...new Set([...newBody.matchAll(/{(\w+)}/g)].map((m) => m[1]))]);

	function formatRateRange(masking: boolean): string {
		const values = pricingRates
			.map((rate) => (masking ? rate.masking_rate : rate.non_masking_rate))
			.filter((value): value is number => value !== null && value !== undefined);
		if (values.length === 0) return '—';
		const min = Math.min(...values);
		const max = Math.max(...values);
		const fmt = (value: number) => `৳${value.toFixed(2)}`;
		return min === max ? fmt(min) : `${fmt(min)}–${fmt(max)}`;
	}

	let allSelected = $derived(
		templates.length > 0 && templates.every((t) => selectedIds.includes(t.id))
	);

	function toggleSelectAll() {
		if (allSelected) {
			selectedIds = [];
		} else {
			selectedIds = templates.map((t) => t.id);
		}
	}

	function toggleSelect(id: number) {
		if (selectedIds.includes(id)) {
			selectedIds = selectedIds.filter((x) => x !== id);
		} else {
			selectedIds = [...selectedIds, id];
		}
	}

	async function handleBulkDelete() {
		if (selectedIds.length === 0 || bulkDeleting) return;
		bulkDeleting = true;
		try {
			await api.post('/api/v1/customer/templates/bulk-delete', { ids: selectedIds });
			toast.success('Templates Deleted', `Deleted ${selectedIds.length} selected template(s).`);
			selectedIds = [];
			await load(1);
		} catch (err: unknown) {
			toast.error('Bulk Delete Error', err instanceof Error ? err.message : 'Failed to delete selected templates.');
		} finally {
			bulkDeleting = false;
		}
	}

	// Modals
	let createModalOpen = $state(false);
	let newName = $state('');
	let submitting = $state(false);

	let selectedTemplate = $state<Template | null>(null);
	let deleteConfirmOpen = $state(false);
	let templateToDelete = $state<Template | null>(null);

	let editModalOpen = $state(false);
	let editingTemplate = $state<Template | null>(null);
	let editName = $state('');
	let editBody = $state('');
	let savingEdit = $state(false);
	let editStats = $derived(computeSmsStats(editBody));

	function openEdit(tmpl: Template) {
		editingTemplate = tmpl;
		editName = tmpl.name;
		editBody = tmpl.body;
		editModalOpen = true;
	}

	async function handleUpdateTemplate() {
		if (!editingTemplate || !editName.trim() || !editBody.trim()) {
			toast.error('Validation error', 'Template name and body are required.');
			return;
		}
		savingEdit = true;
		try {
			await api.put(`/api/v1/customer/templates/${editingTemplate.id}`, {
				name: editName.trim(),
				body: editBody.trim()
			});
			toast.success('Template Updated', 'Your template changes were saved.');
			editModalOpen = false;
			editingTemplate = null;
			load(currentPage);
		} catch (err: unknown) {
			toast.error('Update Error', err instanceof Error ? err.message : 'Failed to update template.');
		} finally {
			savingEdit = false;
		}
	}

	async function load(p = 1) {
		loading = true;
		currentPage = p;
		try {
			const params = new URLSearchParams({ page: String(p), per_page: String(perPage) });
			if (search) params.set('search', search);
			if (approvalFilter) params.set('approval', approvalFilter);
			const data = await api.get<{ data: Template[]; meta: Meta }>(`/api/v1/customer/templates?${params}`);
			templates = data.data || [];
			meta = data.meta || { total: 0, per_page: perPage, current_page: p };
		} catch {
			templates = [];
			meta = { total: 0, per_page: perPage, current_page: p };
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		load(1);
		api
			.get<{ data: PricingRate[] }>('/api/v1/customer/pricing')
			.then((res) => (pricingRates = res.data ?? []))
			.catch(() => (pricingRates = []));
	});

	async function handleCreateTemplate() {
		if (!newName.trim() || !newBody.trim()) {
			toast.error('Validation error', 'Template name and body are required.');
			return;
		}

		submitting = true;
					try {
			const res = await api.post<{ message?: string; data?: { status?: string } }>('/api/v1/customer/templates', {
				name: newName.trim(),
				body: newBody.trim()
			});
			if (res?.data?.status === 'pending') {
				toast.success(
					'Bulk SMS Template Submitted for Approval',
					'Your template has been submitted to the admin for approval. Once approved, you can use it to send Bulk SMS campaigns anytime without waiting for admin approval.'
				);
			} else {
				toast.success('Bulk SMS Template Created', 'Your template has been created and is ready to use.');
			}
			createModalOpen = false;
			newName = '';
			newBody = '';
			load(1);
		} catch (err: unknown) {
			const msg = err instanceof Error ? err.message : 'Failed to create template.';
			toast.error('Create Error', msg);
		} finally {
			submitting = false;
		}
	}

	async function handleDeleteTemplate() {
		if (!templateToDelete) return;
		try {
			await api.delete(`/api/v1/customer/templates/${templateToDelete.id}`);
			toast.success('Template Deleted', 'Template was deleted.');
			deleteConfirmOpen = false;
			templateToDelete = null;
			load(1);
		} catch (err: unknown) {
			const msg = err instanceof Error ? err.message : 'Failed to delete template.';
			toast.error('Delete Error', msg);
		}
	}

	let filteredTemplates = $derived.by(() => {
		if (!search) return templates;
		const q = search.toLowerCase();
		return templates.filter(
			(t) =>
				t.name?.toLowerCase().includes(q) ||
				t.body?.toLowerCase().includes(q)
		);
	});
</script>

<svelte:head>
	<title>Bulk SMS Templates - White INFO</title>
</svelte:head>

<div class="space-y-6">
	<!-- Breadcrumb -->
	<nav class="flex items-center gap-2 text-xs font-medium text-gray-500">
		<a href="/messaging/templates" class="hover:text-gray-700">Bulk SMS Templates</a>
		<svg class="h-3 w-3 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
			<path stroke-linecap="round" stroke-linejoin="round" d="m8.25 4.5 7.5 7.5-7.5 7.5" />
		</svg>
		<span class="text-gray-900 font-semibold">List</span>
	</nav>

	<!-- Page Heading & New template button -->
	<div class="flex items-center justify-between">
		<h1 class="text-2xl font-bold tracking-tight text-gray-950">Bulk SMS Templates</h1>
		<button
			type="button"
			onclick={() => { newName = ''; newBody = ''; createModalOpen = true; }}
			class="inline-flex items-center gap-1.5 rounded-lg bg-teal-600 px-3.5 py-2 text-xs font-semibold text-white shadow-sm transition hover:bg-teal-700 cursor-pointer"
		>
			<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
			</svg>
			New Bulk SMS Template
		</button>
	</div>

	<!-- Main Card -->
	<div class="rounded-xl border border-gray-200 bg-white shadow-sm overflow-hidden">
		<!-- Toolbar: Filter, Bulk Delete & Search -->
		<div class="flex flex-wrap items-center justify-between gap-2 border-b border-gray-100 p-3 bg-white">
			<div class="flex items-center gap-2">
				<select
					bind:value={approvalFilter}
					onchange={() => load(1)}
					aria-label="Filter by approval status"
					class="rounded-lg border border-gray-300 bg-white py-1.5 px-2.5 text-xs text-gray-800 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600"
				>
					<option value="">All Approval Statuses</option>
					<option value="approved">Approved</option>
					<option value="pending">Pending</option>
					<option value="rejected">Rejected</option>
				</select>
				{#if selectedIds.length > 0}
					<button
						type="button"
						disabled={bulkDeleting}
						onclick={handleBulkDelete}
						class="inline-flex items-center gap-1 rounded-lg bg-red-600 px-3 py-1.5 text-xs font-semibold text-white shadow-xs hover:bg-red-700 disabled:opacity-50 cursor-pointer"
					>
						{bulkDeleting ? 'Deleting…' : `Delete Selected (${selectedIds.length})`}
					</button>
				{/if}
			</div>
			<div class="relative w-64">
				<div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-2.5">
					<svg class="h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
					</svg>
				</div>
				<input
					type="text"
					bind:value={search}
					onkeydown={(e) => { if (e.key === 'Enter') load(1); }}
					placeholder="Search"
					class="w-full rounded-lg border border-gray-300 bg-white py-1.5 pr-3 pl-8 text-xs text-gray-900 placeholder-gray-400 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600"
				/>
			</div>
		</div>

		<!-- Table -->
		<div class="overflow-x-auto">
			<table class="w-full text-left border-collapse text-xs">
				<thead>
					<tr class="border-b border-gray-200 bg-gray-50/70 text-gray-600 font-semibold tracking-wider uppercase text-[11px]">
						<th class="w-10 py-3 px-4">
							<input
								type="checkbox"
								checked={allSelected}
								onchange={toggleSelectAll}
								class="rounded border-gray-300 text-teal-600 focus:ring-teal-500 cursor-pointer"
								aria-label="Select all templates"
							/>
						</th>
						<th class="py-3 px-4">TEMPLATE NAME</th>
						<th class="py-3 px-4">MESSAGE TEMPLATE</th>
						<th class="py-3 px-4">STATUS</th>
						<th class="py-3 px-4">CREATED AT</th>
						<th class="py-3 px-4 text-right"></th>
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-100 bg-white">
					{#if loading}
						<tr>
							<td colspan="6" class="py-12 text-center text-gray-500">
								<div class="inline-block h-6 w-6 animate-spin rounded-full border-2 border-gray-300 border-t-teal-600"></div>
								<p class="mt-2 text-xs">Loading templates...</p>
							</td>
						</tr>
					{:else if filteredTemplates.length === 0}
						<tr>
							<td colspan="6" class="py-12 text-center text-gray-500">
								<p class="text-sm font-medium text-gray-700">No templates found</p>
								<p class="mt-1 text-xs text-gray-400">Click "New template" above to create reusable SMS drafts.</p>
							</td>
						</tr>
					{:else}
						{#each filteredTemplates as tmpl (tmpl.id)}
							<tr class="hover:bg-gray-50/70 transition">
								<td class="py-3 px-4">
									<input
										type="checkbox"
										checked={selectedIds.includes(tmpl.id)}
										onchange={() => toggleSelect(tmpl.id)}
										class="rounded border-gray-300 text-teal-600 focus:ring-teal-500 cursor-pointer"
										aria-label="Select template {tmpl.name}"
									/>
								</td>
								<td class="py-3 px-4 font-semibold text-gray-900 whitespace-nowrap">
									{tmpl.name}
								</td>
								<td class="py-3 px-4 text-gray-700 max-w-md truncate" title={tmpl.body}>
									{tmpl.body}
								</td>
								<td class="py-3 px-4 whitespace-nowrap">
									{#if tmpl.status === 'approved'}
										<span class="inline-flex items-center gap-1 rounded-md bg-emerald-50 px-2 py-0.5 text-xs font-semibold text-emerald-700 border border-emerald-200">
											<svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke-width="2.5" stroke="currentColor">
												<path stroke-linecap="round" stroke-linejoin="round" d="m4.5 12.75 6 6 9-13.5" />
											</svg>
											Approved
										</span>
									{:else if tmpl.status === 'rejected'}
										<span class="inline-flex items-center gap-1 rounded-md bg-red-50 px-2 py-0.5 text-xs font-semibold text-red-700 border border-red-200" title={tmpl.rejection_reason || 'Rejected by admin'}>
											<svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
												<path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" />
											</svg>
											Rejected
										</span>
									{:else}
										<span class="inline-flex items-center gap-1 rounded-md bg-amber-50 px-2 py-0.5 text-xs font-semibold text-amber-700 border border-amber-200">
											<svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
												<path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
											</svg>
											Pending
										</span>
									{/if}
								</td>
								<td class="py-3 px-4 text-gray-600 whitespace-nowrap">
									{new Date(tmpl.created_at).toLocaleString('en-GB', { day: '2-digit', month: 'short', year: 'numeric' })}
								</td>
								<td class="py-3 px-4 text-right whitespace-nowrap">
									<div class="flex items-center justify-end gap-1">
										{#if tmpl.status !== 'approved'}
											<button
												type="button"
												onclick={() => openEdit(tmpl)}
												class="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-teal-700 cursor-pointer"
												title="Edit"
											>
												<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
													<path stroke-linecap="round" stroke-linejoin="round" d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L10.582 16.07a4.5 4.5 0 0 1-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 0 1 1.13-1.897l8.932-8.931Zm0 0L19.5 7.125" />
												</svg>
											</button>
										{/if}
										<button
											type="button"
											onclick={() => (selectedTemplate = tmpl)}
											class="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-700 cursor-pointer"
											title="View"
										>
											<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
												<path stroke-linecap="round" stroke-linejoin="round" d="M2.036 12.322a1.012 1.012 0 0 1 0-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.963-7.178Z" />
												<path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z" />
											</svg>
										</button>
										<button
											type="button"
											onclick={() => { templateToDelete = tmpl; deleteConfirmOpen = true; }}
											class="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-red-600 cursor-pointer"
											title="Delete"
										>
											<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
												<path stroke-linecap="round" stroke-linejoin="round" d="m14.74 9-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0" />
											</svg>
										</button>
									</div>
								</td>
							</tr>
						{/each}
					{/if}
				</tbody>
			</table>
		</div>

		<!-- Pagination Footer -->
		<div class="flex items-center justify-between border-t border-gray-200 px-4 py-3 bg-white text-xs text-gray-600">
			<div>
				Showing {filteredTemplates.length > 0 ? (currentPage - 1) * perPage + 1 : 0} to {Math.min(currentPage * perPage, meta.total)} of {meta.total} results
			</div>
			<div class="flex items-center gap-3">
				{#if totalPages > 1}
					<div class="flex items-center gap-1">
						<button type="button" disabled={currentPage <= 1} onclick={() => load(currentPage - 1)} class="rounded border border-gray-200 px-2 py-1 text-xs disabled:opacity-40 cursor-pointer">Prev</button>
						<span class="px-2 text-xs">{currentPage} / {totalPages}</span>
						<button type="button" disabled={currentPage >= totalPages} onclick={() => load(currentPage + 1)} class="rounded border border-gray-200 px-2 py-1 text-xs disabled:opacity-40 cursor-pointer">Next</button>
					</div>
				{/if}
				<span class="text-xs text-gray-500">Per page</span>
				<select
					bind:value={perPage}
					onchange={() => load(1)}
					class="rounded-lg border border-gray-300 bg-white py-1 px-2 text-xs text-gray-700 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600"
				>
					<option value={10}>10</option>
					<option value={25}>25</option>
					<option value={50}>50</option>
				</select>
			</div>
		</div>
	</div>
</div>

<!-- Create Template Modal -->
{#if createModalOpen}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 animate-fade-in backdrop-blur-xs">
		<div class="relative w-full max-w-lg rounded-2xl bg-white p-6 shadow-xl border border-gray-100">
			<h3 class="text-base font-bold text-gray-900">Create Bulk SMS Template</h3>
			<form onsubmit={(e) => { e.preventDefault(); handleCreateTemplate(); }} class="mt-4 space-y-4">
				<div class="rounded-xl border border-sky-200 bg-sky-50 p-3 text-xs leading-relaxed text-sky-900">
					<p class="font-semibold">Admin Approval &amp; Instant Sending Workflow</p>
					<p class="mt-1">When you create a Bulk SMS Template, it is submitted to the Superman admin for approval. Once approved, the template is permanently locked against edits, and you can dispatch campaigns with this approved template anytime without waiting for daily admin approvals!</p>
				</div>
				<div>
					<label for="template-name-input" class="block text-xs font-semibold text-gray-700">Template Name<span class="text-red-500">*</span></label>
					<input
						id="template-name-input"
						type="text"
						bind:value={newName}
						required
						placeholder="e.g. OTP Verification or Invoice Alert"
						class="mt-1.5 w-full rounded-lg border border-gray-300 bg-white py-2 px-3 text-xs text-gray-900 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600"
					/>
				</div>
				<div>
					<label for="template-body-input" class="block text-xs font-semibold text-gray-700">Message Body<span class="text-red-500">*</span></label>
					<textarea
						id="template-body-input"
						bind:value={newBody}
						rows={6}
						required
						placeholder={'বাংলায় আপনার এসএমএস লিখুন... You can use placeholders like {name}, {code}, {amount}...'}
						class="mt-1.5 w-full rounded-lg border border-gray-300 bg-white py-2 px-3 text-xs text-gray-900 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600"
					></textarea>
				</div>
				<div class="rounded-lg border border-gray-200 bg-gray-50 p-2 text-[11px] text-gray-600">
					<span class="font-medium text-gray-700">Remaining:</span> {createStats.remaining}/{createStats.perPartTotal}
					<span class="ml-4 font-medium text-gray-700">SMS Count:</span> <span class="font-bold text-teal-700">{createStats.parts}</span>
					<span class="ml-4 font-medium text-gray-700">Encoding:</span> {createStats.encoding}
					<span class="ml-4 font-medium text-gray-700">Non-masking:</span> {formatRateRange(false)}/SMS
					<span class="ml-4 font-medium text-gray-700">Masking:</span> {formatRateRange(true)}/SMS
				</div>
				<div class="rounded-lg border border-amber-300/60 bg-amber-50 p-2 text-[11px] leading-relaxed text-amber-800">
					<strong class="font-semibold text-amber-900">BTRC নির্দেশনা:</strong> বিটিআরসি (BTRC) নিয়মানুযায়ী সকল প্রকার প্রমোশনাল ও বাল্ক এসএমএস বাংলায় পাঠানো বাধ্যতামূলক।
				</div>
				{#if detectedVariables.length}
					<div>
						<p class="text-xs font-semibold text-gray-700">Variables / Placeholders</p>
						<div class="mt-1.5 flex flex-wrap gap-1.5">
							{#each detectedVariables as variable (variable)}
								<span class="rounded-md border border-teal-200 bg-teal-50 px-2 py-0.5 text-[11px] font-mono font-medium text-teal-700">{variable}</span>
							{/each}
						</div>
						<p class="mt-1 text-[10px] text-gray-500">Variables enclosed in curly braces in your message body are detected automatically.</p>
					</div>
				{/if}
				<div class="mt-6 flex items-center justify-end gap-3 pt-2">
					<button
						type="button"
						onclick={() => (createModalOpen = false)}
						class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-xs font-semibold text-gray-700 hover:bg-gray-50 cursor-pointer"
					>
						Cancel
					</button>
					<button
						type="submit"
						disabled={submitting}
						class="rounded-lg bg-teal-600 px-4 py-2 text-xs font-semibold text-white shadow-sm hover:bg-teal-700 transition disabled:opacity-50 cursor-pointer"
					>
						{submitting ? 'Saving...' : 'Create'}
					</button>
				</div>
			</form>
		</div>
	</div>
{/if}

<!-- View Template Modal -->
{#if selectedTemplate}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 animate-fade-in backdrop-blur-xs">
		<div class="relative w-full max-w-md rounded-2xl bg-white p-6 shadow-xl border border-gray-100">
			<div class="flex items-center justify-between pb-3 border-b border-gray-100">
				<h3 class="text-base font-bold text-gray-900">{selectedTemplate.name}</h3>
				<button
					type="button"
					onclick={() => (selectedTemplate = null)}
					class="text-gray-400 hover:text-gray-600 cursor-pointer"
				>
					<svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" />
					</svg>
				</button>
			</div>

			<div class="mt-4 space-y-3 text-xs">
				<div class="flex justify-between py-1 border-b border-gray-50">
					<span class="text-gray-500 font-medium">Status</span>
					<span class="font-semibold capitalize text-teal-700">{selectedTemplate.status}</span>
				</div>
				<div class="flex justify-between py-1 border-b border-gray-50">
					<span class="text-gray-500 font-medium">Created At</span>
					<span class="text-gray-700">{new Date(selectedTemplate.created_at).toLocaleString()}</span>
				</div>
				{#if selectedTemplate.status === 'rejected' && selectedTemplate.rejection_reason}
					<div class="py-1">
						<span class="block text-gray-500 font-medium mb-1">Rejection Reason</span>
						<div class="rounded-lg bg-rose-50 p-3 text-xs text-rose-700 border border-rose-200">{selectedTemplate.rejection_reason}</div>
					</div>
				{/if}
				<div class="py-2">
					<span class="block text-gray-500 font-medium mb-1">Message Body</span>
					<div class="rounded-lg bg-gray-50 p-3 text-xs text-gray-800 leading-relaxed border border-gray-200">
						{selectedTemplate.body}
					</div>
				</div>
			</div>

			<div class="mt-6 flex justify-end">
				<button
					type="button"
					onclick={() => (selectedTemplate = null)}
					class="rounded-lg bg-gray-100 px-4 py-2 text-xs font-semibold text-gray-700 hover:bg-gray-200 cursor-pointer"
				>
					Close
				</button>
			</div>
		</div>
	</div>
{/if}

<!-- Delete Confirmation Modal -->
{#if deleteConfirmOpen && templateToDelete}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 animate-fade-in backdrop-blur-xs">
		<div class="relative w-full max-w-sm rounded-2xl bg-white p-6 shadow-xl border border-gray-100">
			<h3 class="text-base font-bold text-gray-900">Delete Template</h3>
			<p class="mt-2 text-xs text-gray-600">
				Are you sure you want to delete <span class="font-semibold text-gray-900">"{templateToDelete.name}"</span>?
			</p>
			<div class="mt-6 flex items-center justify-end gap-3">
				<button
					type="button"
					onclick={() => { deleteConfirmOpen = false; templateToDelete = null; }}
					class="rounded-lg border border-gray-300 bg-white px-3.5 py-1.5 text-xs font-semibold text-gray-700 hover:bg-gray-50 cursor-pointer"
				>
					Cancel
				</button>
				<button
					type="button"
					onclick={handleDeleteTemplate}
					class="rounded-lg bg-red-600 px-3.5 py-1.5 text-xs font-semibold text-white shadow-xs hover:bg-red-700 cursor-pointer"
				>
					Delete
				</button>
			</div>
		</div>
	</div>
{/if}

<!-- Edit Template Modal -->
{#if editModalOpen && editingTemplate}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 animate-fade-in backdrop-blur-xs">
		<div class="relative w-full max-w-lg rounded-2xl bg-white p-6 shadow-xl border border-gray-100">
			<h3 class="text-base font-bold text-gray-900">Edit Bulk SMS Template</h3>
			<form onsubmit={(e) => { e.preventDefault(); handleUpdateTemplate(); }} class="mt-4 space-y-4">
				<div class="rounded-xl border border-sky-200 bg-sky-50 p-3 text-xs leading-relaxed text-sky-900">
					<p class="font-semibold">Admin Approval &amp; Instant Sending Workflow</p>
					<p class="mt-1">When you create a Bulk SMS Template, it is submitted to the Superman admin for approval. Once approved, the template is permanently locked against edits, and you can dispatch campaigns with this approved template anytime without waiting for daily admin approvals!</p>
				</div>
				<div>
					<label for="edit-template-name" class="block text-xs font-semibold text-gray-700">Template Name<span class="text-red-500">*</span></label>
					<input id="edit-template-name" type="text" bind:value={editName} required class="mt-1.5 w-full rounded-lg border border-gray-300 bg-white py-2 px-3 text-xs text-gray-900 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600" />
				</div>
				<div>
					<label for="edit-template-body" class="block text-xs font-semibold text-gray-700">Message Body<span class="text-red-500">*</span></label>
					<textarea id="edit-template-body" bind:value={editBody} rows={6} required placeholder={'বাংলায় আপনার এসএমএস লিখুন... You can use placeholders like {name}, {code}, {amount}...'} class="mt-1.5 w-full rounded-lg border border-gray-300 bg-white py-2 px-3 text-xs text-gray-900 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600"></textarea>
				</div>
				<div class="rounded-lg border border-gray-200 bg-gray-50 p-2 text-[11px] text-gray-600">
					<span class="font-medium text-gray-700">Remaining:</span> {editStats.remaining}/{editStats.perPartTotal}
					<span class="ml-4 font-medium text-gray-700">SMS Count:</span> <span class="font-bold text-teal-700">{editStats.parts}</span>
					<span class="ml-4 font-medium text-gray-700">Encoding:</span> {editStats.encoding}
				</div>
				<p class="text-[11px] text-amber-700 bg-amber-50 border border-amber-200 rounded-lg p-2">Once the admin approves a template it becomes locked and can no longer be edited.</p>
				<div class="mt-6 flex items-center justify-end gap-3 pt-2">
					<button type="button" onclick={() => { editModalOpen = false; editingTemplate = null; }} class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-xs font-semibold text-gray-700 hover:bg-gray-50 cursor-pointer">Cancel</button>
					<button type="submit" disabled={savingEdit} class="rounded-lg bg-teal-600 px-4 py-2 text-xs font-semibold text-white shadow-sm hover:bg-teal-700 transition disabled:opacity-50 cursor-pointer">{savingEdit ? 'Saving...' : 'Save'}</button>
				</div>
			</form>
		</div>
	</div>
{/if}
