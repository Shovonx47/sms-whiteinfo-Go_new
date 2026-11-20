<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import SenderForm from '$lib/components/sender/SenderForm.svelte';
	import ImportSendersPanel from '$lib/components/sender/ImportSendersPanel.svelte';
	import { loadSenderOptions, type SenderCustomer, type SenderGateway } from '$lib/sender-admin.js';

	let showImport = $state(false);
	let gateways = $state<SenderGateway[]>([]);
	let customers = $state<SenderCustomer[]>([]);

	onMount(() => {
		void loadSenderOptions()
			.then((options) => {
				gateways = options.gateways;
				customers = options.customers;
			})
			.catch(() => {});
	});
</script>

<svelte:head>
	<title>Create Sender — SMS Whiteinfo Administrator</title>
</svelte:head>

<div class="space-y-5">
	<div class="flex flex-wrap items-end justify-between gap-3">
		<div>
			<nav class="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400" aria-label="Breadcrumb">
				<a href="/superman/sender-cli/add" class="hover:text-emerald-700 dark:hover:text-emerald-400">Senders</a>
				<span>&rsaquo;</span>
				<span>Create</span>
			</nav>
			<h1 class="mt-1 text-3xl font-bold tracking-tight text-gray-950 dark:text-white">Create Sender</h1>
		</div>
		<button
			type="button"
			onclick={() => (showImport = true)}
			data-testid="create-import-btn"
			class="inline-flex items-center gap-2 rounded-lg border border-gray-300 bg-white px-3.5 py-2 text-sm font-medium text-gray-800 shadow-xs transition hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-200 dark:hover:bg-gray-800"
		>
			<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5m-13.5-9L12 3m0 0 4.5 4.5M12 3v13.5" /></svg>
			Bulk Import Senders (CSV/Excel)
		</button>
	</div>

	<SenderForm />
</div>

{#if showImport}
	<div class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-gray-950/50 p-4 pt-16" role="presentation" onclick={() => (showImport = false)}>
		<div
			class="w-full max-w-lg rounded-xl border border-gray-200 bg-white p-6 shadow-xl dark:border-gray-800 dark:bg-gray-900"
			role="dialog"
			aria-modal="true"
			aria-label="Import Senders"
			tabindex="-1"
			onclick={(e) => e.stopPropagation()}
			onkeydown={(e) => e.key === 'Escape' && (showImport = false)}
		>
			<div class="mb-3 flex items-start justify-between">
				<h2 class="text-lg font-semibold text-gray-950 dark:text-white">Import Senders</h2>
				<button type="button" onclick={() => (showImport = false)} aria-label="Close" class="rounded-lg p-1 text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800">
					<svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" /></svg>
				</button>
			</div>
			<ImportSendersPanel
				{gateways}
				{customers}
				onDone={async (message) => {
					showImport = false;
					await goto(`/superman/sender-cli/add?saved=${encodeURIComponent(message)}`);
				}}
				onCancel={() => (showImport = false)}
			/>
		</div>
	</div>
{/if}
