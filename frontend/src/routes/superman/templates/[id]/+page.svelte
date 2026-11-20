<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { supermanApi } from '$lib/superman-api.js';
	import {
		type AdminTemplate,
		formatDate
	} from '$lib/template-admin.js';

	const templateId = $derived(page.params.id);

	let template = $state<AdminTemplate | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	async function loadTemplate() {
		loading = true;
		error = null;
		try {
			const res = await supermanApi.get<{ data: AdminTemplate }>(
				`/api/v1/superman/templates/${templateId}`
			);
			template = res.data;
		} catch (err: unknown) {
			error = err instanceof Error ? err.message : 'Failed to load template.';
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		void loadTemplate();
	});
</script>

<svelte:head>
	<title>View Bulk SMS Template — Superman Administrator</title>
</svelte:head>

<div class="mx-auto max-w-4xl space-y-5">
	<!-- Breadcrumb + Title + Top Action -->
	<div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
		<div>
			<nav class="flex items-center gap-1.5 text-xs text-gray-500 dark:text-gray-400">
				<a href="/superman/templates" class="hover:text-gray-800 dark:hover:text-gray-200">
					Bulk SMS Templates
				</a>
				<span class="text-gray-400">›</span>
				<span class="text-gray-600 dark:text-gray-300">{template?.name || '...'}</span>
				<span class="text-gray-400">›</span>
				<span class="font-medium text-gray-700 dark:text-gray-300">View</span>
			</nav>
			<h1 class="mt-1 text-2xl font-bold tracking-tight text-gray-950 dark:text-white">
				View Bulk SMS Template
			</h1>
		</div>

		{#if template}
			<a
				href={`/superman/templates/${template.id}/edit`}
				class="inline-flex items-center gap-1.5 rounded-lg border border-gray-300 bg-white px-3.5 py-2 text-xs font-semibold text-gray-700 shadow-xs transition hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200 dark:hover:bg-gray-700"
			>
				<svg class="h-4 w-4 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
					<path
						stroke-linecap="round"
						stroke-linejoin="round"
						stroke-width="2"
						d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
					/>
				</svg>
				<span>Edit</span>
			</a>
		{/if}
	</div>

	<!-- Loading / Error -->
	{#if loading}
		<div class="flex items-center justify-center py-20 text-gray-400">
			<svg class="h-8 w-8 animate-spin text-emerald-600" viewBox="0 0 24 24" fill="none">
				<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
				<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
			</svg>
			<span class="ml-3 text-xs">Loading template details...</span>
		</div>
	{:else if error}
		<div
			class="rounded-xl border border-rose-200 bg-rose-50 p-4 text-xs font-medium text-rose-800 dark:border-rose-800/60 dark:bg-rose-950/40 dark:text-rose-300"
		>
			{error}
		</div>
	{:else if template}
		<!-- Section: Details (Filament Infolist) -->
		<div
			class="overflow-hidden rounded-2xl border border-gray-200 bg-white p-6 shadow-xs dark:border-gray-800 dark:bg-gray-900"
		>
			<div class="border-b border-gray-100 pb-4 dark:border-gray-800">
				<h2 class="text-base font-bold text-gray-950 dark:text-white">
					Details
				</h2>
			</div>

			<div class="mt-6 grid grid-cols-1 gap-6 sm:grid-cols-2">
				<!-- Customer -->
				<div>
					<span class="block text-xs font-medium text-gray-500 dark:text-gray-400">Customer</span>
					<span class="mt-1 block text-sm font-bold text-gray-950 dark:text-white">
						{template.customer || '—'}
					</span>
				</div>

				<!-- Template Name -->
				<div>
					<span class="block text-xs font-medium text-gray-500 dark:text-gray-400">Template Name</span>
					<span class="mt-1 block text-sm font-semibold text-gray-900 dark:text-gray-100">
						{template.name}
					</span>
				</div>

				<!-- Approval Status -->
				<div>
					<span class="block text-xs font-medium text-gray-500 dark:text-gray-400">Approval Status</span>
					<div class="mt-1">
						{#if template.approval === 'approved'}
							<span
								class="inline-flex items-center gap-1 rounded-full border border-emerald-200 bg-emerald-50 px-2.5 py-0.5 text-xs font-semibold text-emerald-700 dark:border-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-300"
							>
								<svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M5 13l4 4L19 7" />
								</svg>
								Approved
							</span>
						{:else if template.approval === 'rejected'}
							<span
								class="inline-flex items-center gap-1 rounded-full border border-rose-200 bg-rose-50 px-2.5 py-0.5 text-xs font-semibold text-rose-700 dark:border-rose-800 dark:bg-rose-950/40 dark:text-rose-300"
							>
								<svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
								</svg>
								Rejected
							</span>
						{:else}
							<span
								class="inline-flex items-center gap-1 rounded-full border border-amber-200 bg-amber-50 px-2.5 py-0.5 text-xs font-semibold text-amber-700 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-300"
							>
								<svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
									<path
										stroke-linecap="round"
										stroke-linejoin="round"
										stroke-width="2"
										d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
									/>
								</svg>
								Pending
							</span>
						{/if}
					</div>
				</div>

				<!-- Reviewed By -->
				<div>
					<span class="block text-xs font-medium text-gray-500 dark:text-gray-400">Reviewed By</span>
					<span class="mt-1 block text-xs font-medium text-gray-800 dark:text-gray-200">
						{template.reviewed_by ?? '—'}
					</span>
				</div>

				<!-- Message Body (full width) -->
				<div class="sm:col-span-2">
					<span class="block text-xs font-medium text-gray-500 dark:text-gray-400">Message Body</span>
					<div
						class="mt-1.5 whitespace-pre-wrap rounded-xl border border-gray-200 bg-gray-50/70 p-4 text-xs leading-relaxed text-gray-900 dark:border-gray-800 dark:bg-gray-800/60 dark:text-gray-100"
					>
						{template.body}
					</div>
				</div>

				<!-- Rejection Reason (only if rejected) -->
				{#if template.approval === 'rejected'}
					<div class="sm:col-span-2">
						<span class="block text-xs font-medium text-rose-600 dark:text-rose-400">Rejection Reason</span>
						<div
							class="mt-1.5 rounded-xl border border-rose-200 bg-rose-50/60 p-4 text-xs leading-relaxed text-rose-900 dark:border-rose-900/60 dark:bg-rose-950/30 dark:text-rose-200"
						>
							{template.rejection_reason || 'No reason provided.'}
						</div>
					</div>
				{/if}

				<!-- Variables -->
				{#if template.variables && template.variables.length > 0}
					<div class="sm:col-span-2">
						<span class="block text-xs font-medium text-gray-500 dark:text-gray-400">Variables</span>
						<div class="mt-1.5 flex flex-wrap gap-1.5">
							{#each template.variables as v}
								<span
									class="rounded-md bg-emerald-50 px-2 py-0.5 text-xs font-semibold text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300"
								>
									&#123;{v}&#125;
								</span>
							{/each}
						</div>
					</div>
				{/if}

				<!-- Created At -->
				<div>
					<span class="block text-xs font-medium text-gray-500 dark:text-gray-400">Created At</span>
					<span class="mt-1 block text-xs text-gray-800 dark:text-gray-200">
						{formatDate(template.created_at)}
					</span>
				</div>

				<!-- Reviewed At -->
				<div>
					<span class="block text-xs font-medium text-gray-500 dark:text-gray-400">Reviewed At</span>
					<span class="mt-1 block text-xs text-gray-800 dark:text-gray-200">
						{formatDate(template.approved_at)}
					</span>
				</div>
			</div>

			<div class="mt-8 border-t border-gray-100 pt-4 dark:border-gray-800">
				<a
					href="/superman/templates"
					class="inline-flex items-center gap-1.5 text-xs font-semibold text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-gray-200"
				>
					<span>← Back to Bulk SMS Templates</span>
				</a>
			</div>
		</div>
	{/if}
</div>
