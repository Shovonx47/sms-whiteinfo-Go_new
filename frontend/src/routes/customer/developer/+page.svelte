<script lang="ts">
	import { api } from '$lib/api.js';
	import { toast } from '$lib/stores/toast.svelte.js';
	import { onMount } from 'svelte';

	interface ApiToken {
		id: number;
		name: string;
		token?: string;
		last_used_at: string | null;
		expires_at: string | null;
		created_at: string;
	}

	let tokens = $state<ApiToken[]>([]);
	let loading = $state(true);
	let searchQuery = $state('');

	// Create Modal
	let createModalOpen = $state(false);
	let tokenName = $state('');
	let submitting = $state(false);

	// Display newly created token
	let newlyCreatedToken = $state<string | null>(null);

	// Revoke confirm
	let revokeTarget = $state<ApiToken | null>(null);
	let revokeMenuOpen = $state<number | null>(null);

	async function load() {
		loading = true;
		try {
			const data = await api.get<{ data: ApiToken[] }>('/api/v1/customer/tokens');
			tokens = data.data;
		} catch {
			tokens = [];
		} finally {
			loading = false;
		}
	}

	onMount(() => load());

	async function handleCreateToken() {
		if (!tokenName.trim()) {
			toast.error('Validation error', 'Token name is required.');
			return;
		}

		submitting = true;
		try {
			const res = await api.post<{ token: string; data: ApiToken }>('/api/v1/customer/tokens', {
				name: tokenName
			});
			newlyCreatedToken = res.token;
			tokens = [res.data, ...tokens];
			toast.success('Token Generated', 'Make sure to copy your new token now.');
			tokenName = '';
		} catch (err: unknown) {
			const message = err instanceof Error ? err.message : 'Failed to generate token';
			toast.error('Token Generation Failed', message);
		} finally {
			submitting = false;
		}
	}

	async function handleRevoke(token: ApiToken) {
		revokeMenuOpen = null;
		revokeTarget = null;
		try {
			await api.delete(`/api/v1/customer/tokens/${token.id}`);
			tokens = tokens.filter((t) => t.id !== token.id);
			toast.success('Token Revoked', 'The API token has been deleted.');
		} catch (err: unknown) {
			const message = err instanceof Error ? err.message : 'Failed to revoke token';
			toast.error('Revoke Failed', message);
		}
	}

	async function copyToClipboard(text: string) {
		try {
			await navigator.clipboard.writeText(text);
			toast.success('Copied!', 'Token copied to clipboard.');
		} catch {
			toast.info('Copy', text);
		}
	}

	let filteredTokens = $derived(
		searchQuery.trim()
			? tokens.filter((t) => t.name.toLowerCase().includes(searchQuery.toLowerCase()))
			: tokens
	);

	function formatDate(val: string | null) {
		if (!val) return '—';
		return new Date(val).toLocaleDateString('en-GB', { day: '2-digit', month: 'short', year: 'numeric' });
	}

	function timeAgo(val: string | null) {
		if (!val) return 'Never';
		const diff = Date.now() - new Date(val).getTime();
		const mins = Math.floor(diff / 60000);
		if (mins < 1) return 'Just now';
		if (mins < 60) return `${mins}m ago`;
		const hrs = Math.floor(mins / 60);
		if (hrs < 24) return `${hrs}h ago`;
		const days = Math.floor(hrs / 24);
		return `${days}d ago`;
	}
</script>

<svelte:head>
	<title>API Tokens (API Keys) - White INFO</title>
</svelte:head>

<!-- Breadcrumb -->
<div class="mb-2 flex items-center gap-1 text-xs text-gray-500">
	<span class="font-medium text-gray-700">API Tokens (API Keys)</span>
	<span class="text-gray-400">›</span>
	<span>List</span>
</div>

<!-- Page Header -->
<div class="mb-6 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
	<h1 class="text-2xl font-bold text-gray-900">API Tokens (API Keys)</h1>
	<button
		type="button"
		onclick={() => { newlyCreatedToken = null; createModalOpen = true; }}
		class="inline-flex items-center gap-2 rounded-lg border border-teal-600 bg-white px-4 py-2 text-sm font-semibold text-teal-700 shadow-sm transition hover:bg-teal-50"
	>
		New API Token (API Key)
	</button>
</div>

<!-- Table Card -->
<div class="rounded-xl border border-gray-200 bg-white shadow-sm overflow-hidden">
	<!-- Search + Column Toggle bar -->
	<div class="flex items-center justify-end gap-2 px-4 py-3 border-b border-gray-100">
		<div class="relative">
			<svg class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 15.803 7.5 7.5 0 0016.803 15.803z" />
			</svg>
			<input
				type="search"
				bind:value={searchQuery}
				placeholder="Search"
				class="w-52 rounded-lg border border-gray-200 bg-gray-50 py-1.5 pl-9 pr-3 text-sm text-gray-700 placeholder-gray-400 focus:border-teal-500 focus:outline-none focus:ring-1 focus:ring-teal-500"
			/>
		</div>
		<!-- Column toggle icon -->
		<button type="button" aria-label="Toggle columns" class="rounded-lg border border-gray-200 bg-gray-50 p-1.5 text-gray-500 hover:text-gray-700 transition">
			<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" d="M9 4.5v15m6-15v15M3.75 9h16.5M3.75 15h16.5" />
			</svg>
		</button>
	</div>

	<!-- Table -->
	<table class="w-full text-sm text-left">
		<thead class="bg-white border-b border-gray-100">
			<tr>
				<th class="px-6 py-3 text-xs font-semibold uppercase tracking-wide text-gray-500">
					<span class="flex items-center gap-1">
						TOKEN NAME
						<svg class="h-3 w-3 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
						</svg>
					</span>
				</th>
				<th class="px-6 py-3 text-xs font-semibold uppercase tracking-wide text-gray-500">
					<span class="flex items-center gap-1">
						LAST USED
						<svg class="h-3 w-3 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
						</svg>
					</span>
				</th>
				<th class="px-6 py-3 text-xs font-semibold uppercase tracking-wide text-gray-500">
					<span class="flex items-center gap-1">
						EXPIRES AT
						<svg class="h-3 w-3 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
						</svg>
					</span>
				</th>
				<th class="px-6 py-3"></th>
			</tr>
		</thead>
		<tbody>
			{#if loading}
				<tr>
					<td colspan="4" class="px-6 py-16 text-center">
						<div class="flex flex-col items-center gap-3">
							<svg class="h-6 w-6 animate-spin text-teal-600" fill="none" viewBox="0 0 24 24">
								<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
								<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
							</svg>
						</div>
					</td>
				</tr>
			{:else if filteredTokens.length === 0}
				<tr>
					<td colspan="4" class="px-6 py-20 text-center">
						<div class="flex flex-col items-center gap-3">
							<div class="flex h-14 w-14 items-center justify-center rounded-full border-2 border-gray-200 bg-gray-50">
								<svg class="h-6 w-6 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
									<path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
								</svg>
							</div>
							<p class="text-sm text-gray-500">No API Tokens (API Keys)</p>
						</div>
					</td>
				</tr>
			{:else}
				{#each filteredTokens as token (token.id)}
					<tr class="border-t border-gray-100 hover:bg-gray-50 transition">
						<td class="px-6 py-3 font-medium text-gray-900">{token.name}</td>
						<td class="px-6 py-3 text-gray-600">{timeAgo(token.last_used_at)}</td>
						<td class="px-6 py-3 text-gray-600">{formatDate(token.expires_at)}</td>
						<td class="px-6 py-3 text-right">
							<div class="relative inline-block">
								<button
									type="button"
									aria-label="Actions"
									onclick={() => revokeMenuOpen = revokeMenuOpen === token.id ? null : token.id}
									class="rounded-lg border border-gray-200 bg-white p-1.5 text-gray-500 hover:text-gray-700 transition"
								>
									<svg class="h-4 w-4" fill="currentColor" viewBox="0 0 20 20">
										<circle cx="10" cy="4" r="1.5" />
										<circle cx="10" cy="10" r="1.5" />
										<circle cx="10" cy="16" r="1.5" />
									</svg>
								</button>
								{#if revokeMenuOpen === token.id}
									<div class="absolute right-0 z-50 mt-1 w-36 rounded-lg border border-gray-200 bg-white shadow-lg">
										<button
											type="button"
											onclick={() => { revokeTarget = token; revokeMenuOpen = null; }}
											class="flex w-full items-center gap-2 px-4 py-2.5 text-sm text-red-600 hover:bg-red-50 rounded-lg transition"
										>
											<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
												<path stroke-linecap="round" stroke-linejoin="round" d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0" />
											</svg>
											Revoke
										</button>
									</div>
								{/if}
							</div>
						</td>
					</tr>
				{/each}
			{/if}
		</tbody>
	</table>

	<!-- Pagination footer (simple) -->
	{#if !loading && filteredTokens.length > 0}
		<div class="flex items-center justify-between border-t border-gray-100 px-6 py-3 text-xs text-gray-500">
			<span>Showing 1 to {filteredTokens.length} of {filteredTokens.length} results</span>
			<div class="flex items-center gap-2">
				<span>Per page</span>
				<select class="rounded border border-gray-200 px-2 py-1 text-xs focus:outline-none">
					<option>10</option>
					<option>25</option>
					<option>50</option>
				</select>
			</div>
		</div>
	{/if}
</div>

<!-- Generate Token Modal -->
{#if createModalOpen}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/40"
		onclick={(e) => { if (e.target === e.currentTarget) createModalOpen = false; }}
		onkeydown={(e) => { if (e.key === 'Escape') createModalOpen = false; }}
		role="dialog"
		tabindex="-1"
		aria-modal="true"
	>
		<div class="w-full max-w-md rounded-xl bg-white shadow-xl mx-4">
			<div class="border-b border-gray-100 px-6 py-4 flex items-center justify-between">
				<div>
					<h2 class="text-base font-semibold text-gray-900">Generate API Token</h2>
					<p class="text-xs text-gray-500 mt-0.5">Create a personal Bearer access key for your application.</p>
				</div>
				<button type="button" aria-label="Close" onclick={() => createModalOpen = false} class="rounded-lg p-1.5 text-gray-400 hover:text-gray-600">
					<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
					</svg>
				</button>
			</div>
			<div class="px-6 py-5">
				{#if newlyCreatedToken}
					<div class="space-y-4">
						<div class="rounded-xl border border-emerald-200 bg-emerald-50 p-4">
							<p class="text-xs font-semibold text-emerald-700">Token Generated Successfully!</p>
							<p class="mt-1 text-xs text-gray-600">Copy this token now. For security purposes, you will not be able to view it again.</p>
							<div class="mt-3 flex items-center gap-2">
								<input
									type="text"
									readonly
									value={newlyCreatedToken}
									class="flex-1 font-mono text-xs rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-emerald-700 focus:outline-none"
								/>
								<button
									type="button"
									onclick={() => copyToClipboard(newlyCreatedToken!)}
									class="rounded-lg bg-teal-600 px-3 py-2 text-xs font-semibold text-white shadow-sm transition hover:bg-teal-500"
								>
									Copy
								</button>
							</div>
						</div>
						<div class="flex justify-end pt-1">
							<button
								type="button"
								onclick={() => { createModalOpen = false; newlyCreatedToken = null; }}
								class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 transition"
							>
								Done
							</button>
						</div>
					</div>
				{:else}
					<form onsubmit={(e) => { e.preventDefault(); handleCreateToken(); }} class="space-y-4">
						<div>
							<label for="token-name" class="mb-1.5 block text-xs font-semibold uppercase tracking-wide text-gray-700">Token Name / Service</label>
							<input
								id="token-name"
								type="text"
								bind:value={tokenName}
								placeholder="e.g. ERP Backend Gateway, Mobile App"
								class="w-full rounded-lg border border-gray-300 bg-white px-3.5 py-2.5 text-sm text-gray-900 placeholder-gray-400 focus:border-teal-500 focus:outline-none focus:ring-1 focus:ring-teal-500"
							/>
						</div>
						<div class="flex justify-end gap-3 pt-1">
							<button
								type="button"
								onclick={() => createModalOpen = false}
								class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 transition"
							>
								Cancel
							</button>
							<button
								type="submit"
								disabled={submitting}
								class="rounded-lg bg-teal-600 px-4 py-2 text-sm font-semibold text-white shadow-sm hover:bg-teal-500 transition disabled:opacity-50"
							>
								{submitting ? 'Generating...' : 'Create Token'}
							</button>
						</div>
					</form>
				{/if}
			</div>
		</div>
	</div>
{/if}

<!-- Revoke Confirm Modal -->
{#if revokeTarget}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/40"
		onclick={(e) => { if (e.target === e.currentTarget) revokeTarget = null; }}
		onkeydown={(e) => { if (e.key === 'Escape') revokeTarget = null; }}
		role="dialog"
		tabindex="-1"
		aria-modal="true"
	>
		<div class="w-full max-w-sm rounded-xl bg-white shadow-xl mx-4 p-6">
			<h2 class="text-base font-semibold text-gray-900">Revoke Token?</h2>
			<p class="mt-2 text-sm text-gray-600">Are you sure you want to revoke <strong class="text-gray-800">{revokeTarget.name}</strong>? This action cannot be undone.</p>
			<div class="mt-5 flex justify-end gap-3">
				<button type="button" onclick={() => revokeTarget = null} class="rounded-lg border border-gray-300 px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 transition">Cancel</button>
				<button
					type="button"
					onclick={() => { const t = revokeTarget; if (t) handleRevoke(t); }}
					class="rounded-lg bg-red-600 px-4 py-2 text-sm font-semibold text-white hover:bg-red-500 transition"
				>
					Revoke
				</button>
			</div>
		</div>
	</div>
{/if}

<!-- Close menus on outside click -->
{#if revokeMenuOpen !== null}
	<!-- svelte-ignore a11y_click_outside -->
	<div
		class="fixed inset-0 z-40"
		onclick={() => revokeMenuOpen = null}
		onkeydown={(e) => { if (e.key === 'Escape') revokeMenuOpen = null; }}
		role="presentation"
	></div>
{/if}
