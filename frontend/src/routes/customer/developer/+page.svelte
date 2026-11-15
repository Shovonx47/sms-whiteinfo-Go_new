<script lang="ts">
	import { api } from '$lib/api.js';
	import Modal from '$lib/components/Modal.svelte';
	import { toast } from '$lib/stores/toast.svelte.js';
	import { onMount } from 'svelte';

	interface ApiToken {
		id: number;
		name: string;
		last_used_at: string | null;
		expires_at: string | null;
		created_at: string;
	}

	interface Meta {
		total: number;
		per_page: number;
		current_page: number;
	}

	let tokens = $state<ApiToken[]>([]);
	let meta = $state<Meta>({ total: 0, per_page: 10, current_page: 1 });
	let loading = $state(true);
	let searchQuery = $state('');
	let selectedIds = $state<number[]>([]);

	let createModalOpen = $state(false);
	let tokenName = $state('');
	let expiresAt = $state('30');
	let createError = $state('');
	let submitting = $state(false);

	let tokenModalOpen = $state(false);
	let newlyCreatedToken = $state<string | null>(null);

	let editOpen = $state(false);
	let editTarget = $state<ApiToken | null>(null);
	let editName = $state('');
	let editError = $state('');
	let savingEdit = $state(false);

	let deleteOpen = $state(false);
	let deleteTarget = $state<ApiToken | null>(null);
	let deleteName = $state('');
	let deleteError = $state('');
	let deleting = $state(false);

	let bulkDeleteOpen = $state(false);
	let bulkDeleting = $state(false);

	async function load(p = meta.current_page) {
		loading = true;
		selectedIds = [];
		try {
			const params = new URLSearchParams({
				page: String(p),
				per_page: String(meta.per_page)
			});
			if (searchQuery.trim()) params.set('search', searchQuery.trim());
			const data = await api.get<{ data: ApiToken[]; meta: Meta }>(
				`/api/v1/customer/tokens?${params}`
			);
			tokens = data.data ?? [];
			meta = data.meta ?? { total: 0, per_page: meta.per_page, current_page: p };
		} catch {
			tokens = [];
			meta = { total: 0, per_page: meta.per_page, current_page: p };
		} finally {
			loading = false;
		}
	}

	let searchTimer: ReturnType<typeof setTimeout> | null = null;
	function handleSearchInput() {
		if (searchTimer) clearTimeout(searchTimer);
		searchTimer = setTimeout(() => load(1), 300);
	}

	onMount(() => load(1));

	function openCreate() {
		tokenName = '';
		expiresAt = '30';
		createError = '';
		createModalOpen = true;
	}

	async function handleCreateToken() {
		createError = '';
		const name = tokenName.trim();
		if (!name) {
			createError = 'The token name field is required.';
			return;
		}
		if (name.length > 255) {
			createError = 'The token name must not be greater than 255 characters.';
			return;
		}

		submitting = true;
		try {
			const res = await api.post<{ token: string; data: ApiToken }>('/api/v1/customer/tokens', {
				name,
				expires_at: expiresAt === 'never' ? 'never' : Number(expiresAt)
			});
			newlyCreatedToken = res.token.includes('|') ? res.token.split('|').slice(1).join('|') : res.token;
			createModalOpen = false;
			tokenModalOpen = true;
			await load(1);
		} catch (err) {
			createError = err instanceof Error ? err.message : 'Failed to create token.';
		} finally {
			submitting = false;
		}
	}

	async function copyToClipboard(text: string) {
		try {
			await navigator.clipboard.writeText(text);
			toast.success('Copied', 'Token copied to clipboard.');
		} catch {
			toast.info('Copy', text);
		}
	}

	function openEdit(token: ApiToken) {
		editTarget = token;
		editName = token.name;
		editError = '';
		editOpen = true;
	}

	async function handleRename() {
		if (!editTarget) return;
		editError = '';
		const name = editName.trim();
		if (!name) {
			editError = 'The token name field is required.';
			return;
		}
		if (name.length > 255) {
			editError = 'The token name must not be greater than 255 characters.';
			return;
		}
		savingEdit = true;
		try {
			await api.put(`/api/v1/customer/tokens/${editTarget.id}`, { name });
			editOpen = false;
			editTarget = null;
			toast.success('Saved', 'The access token has been updated.');
			await load(meta.current_page);
		} catch (err) {
			editError = err instanceof Error ? err.message : 'Failed to update token.';
		} finally {
			savingEdit = false;
		}
	}

	function openDelete(token: ApiToken) {
		deleteTarget = token;
		deleteName = '';
		deleteError = '';
		deleteOpen = true;
	}

	async function handleDelete() {
		if (!deleteTarget) return;
		deleteError = '';
		if (deleteName !== deleteTarget.name) {
			deleteError = 'Token name does not match.';
			return;
		}
		deleting = true;
		try {
			await api.delete(`/api/v1/customer/tokens/${deleteTarget.id}`);
			deleteOpen = false;
			deleteTarget = null;
			toast.success('Deleted', 'The access token has been deleted.');
			await load(meta.current_page);
		} catch (err) {
			deleteError = err instanceof Error ? err.message : 'Failed to delete token.';
		} finally {
			deleting = false;
		}
	}

	function toggleAll() {
		selectedIds = selectedIds.length === tokens.length ? [] : tokens.map((t) => t.id);
	}

	function toggleOne(id: number) {
		selectedIds = selectedIds.includes(id)
			? selectedIds.filter((item) => item !== id)
			: [...selectedIds, id];
	}

	async function handleBulkDelete() {
		if (selectedIds.length === 0) return;
		bulkDeleting = true;
		try {
			const res = await api.post<{ deleted: number }>('/api/v1/customer/tokens/bulk-delete', {
				ids: selectedIds
			});
			toast.success('Deleted', `${res.deleted} API token(s) revoked.`);
			bulkDeleteOpen = false;
			await load(1);
		} catch (err) {
			toast.error('Delete failed', err instanceof Error ? err.message : 'Failed to delete tokens.');
		} finally {
			bulkDeleting = false;
		}
	}

	function formatDateTime(value: string | null) {
		if (!value) return '—';
		const date = new Date(value);
		if (Number.isNaN(date.getTime())) return '—';
		return date.toLocaleString('en-US', {
			month: 'short',
			day: '2-digit',
			year: 'numeric',
			hour: 'numeric',
			minute: '2-digit',
			hour12: true
		});
	}

	let totalPages = $derived(Math.max(1, Math.ceil((meta.total || 0) / (meta.per_page || 10))));
</script>

<svelte:head>
	<title>Access Tokens - White INFO</title>
</svelte:head>

<div class="space-y-6">
	<div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
		<h1 class="text-2xl font-bold tracking-tight text-gray-950 dark:text-white">Access Tokens</h1>
		<button
			type="button"
			onclick={openCreate}
			class="inline-flex items-center gap-2 rounded-lg bg-teal-600 px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:bg-teal-500"
		>
			<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
			</svg>
			New access token
		</button>
	</div>

	<div class="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-sm dark:border-gray-800 dark:bg-gray-900">
		<div class="flex items-center justify-between gap-3 border-b border-gray-100 px-4 py-3 dark:border-gray-800">
			<div class="flex items-center gap-3">
				{#if selectedIds.length > 0}
					<button
						type="button"
						onclick={() => (bulkDeleteOpen = true)}
						class="rounded-lg bg-rose-600 px-3 py-1.5 text-xs font-semibold text-white hover:bg-rose-500"
					>
						Delete selected ({selectedIds.length})
					</button>
				{/if}
			</div>
			<div class="relative">
				<input
					type="text"
					bind:value={searchQuery}
					oninput={handleSearchInput}
					placeholder="Search"
					class="w-52 rounded-lg border border-gray-300 bg-white py-1.5 pr-3 pl-8 text-sm text-gray-700 placeholder-gray-400 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200"
				/>
				<svg class="pointer-events-none absolute top-2.5 left-2.5 h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
				</svg>
			</div>
		</div>

		<div class="overflow-x-auto">
			<table class="w-full text-left text-sm">
				<thead class="border-b border-gray-200 bg-gray-50/70 text-xs font-semibold tracking-wide text-gray-600 uppercase dark:border-gray-700 dark:bg-gray-800/70 dark:text-gray-300">
					<tr>
						<th class="w-10 px-4 py-3">
							<input type="checkbox" checked={tokens.length > 0 && selectedIds.length === tokens.length} onchange={toggleAll} aria-label="Select all tokens" />
						</th>
						<th class="px-4 py-3">Token Name</th>
						<th class="px-4 py-3">Last Used</th>
						<th class="px-4 py-3">Expires At</th>
						<th class="px-4 py-3 text-right"></th>
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-100 dark:divide-gray-800">
					{#if loading}
						<tr>
							<td colspan="5" class="px-4 py-16 text-center text-sm text-gray-500">
								<div class="inline-block h-6 w-6 animate-spin rounded-full border-2 border-gray-300 border-t-teal-600"></div>
							</td>
						</tr>
					{:else if tokens.length === 0}
						<tr>
							<td colspan="5" class="px-4 py-16 text-center text-sm text-gray-500">No access tokens found.</td>
						</tr>
					{:else}
						{#each tokens as token (token.id)}
							<tr class="transition hover:bg-gray-50/70 dark:hover:bg-gray-800/40">
								<td class="px-4 py-3">
									<input type="checkbox" checked={selectedIds.includes(token.id)} onchange={() => toggleOne(token.id)} aria-label={`Select ${token.name}`} />
								</td>
								<td class="px-4 py-3 font-medium text-gray-900 dark:text-gray-100">{token.name}</td>
								<td class="px-4 py-3 text-gray-600 dark:text-gray-400">{token.last_used_at ? formatDateTime(token.last_used_at) : 'Never Used'}</td>
								<td class="px-4 py-3 text-gray-600 dark:text-gray-400">{token.expires_at ? formatDateTime(token.expires_at) : 'Never Expires'}</td>
								<td class="px-4 py-3 text-right whitespace-nowrap">
									<div class="flex items-center justify-end gap-1">
										<button type="button" onclick={() => openEdit(token)} class="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-teal-700 dark:hover:bg-gray-700" title="Edit">
											<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
												<path stroke-linecap="round" stroke-linejoin="round" d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L10.582 16.07a4.5 4.5 0 0 1-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 0 1 1.13-1.897l8.932-8.931Z" />
											</svg>
										</button>
										<button type="button" onclick={() => openDelete(token)} class="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-rose-600 dark:hover:bg-gray-700" title="Delete">
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

		<div class="flex flex-col gap-3 border-t border-gray-200 px-4 py-3 text-xs text-gray-500 sm:flex-row sm:items-center sm:justify-between dark:border-gray-700">
			<div class="flex items-center gap-2">
				<span>Per page</span>
				<select
					value={meta.per_page}
					onchange={(e) => { meta.per_page = Number((e.target as HTMLSelectElement).value); load(1); }}
					class="rounded border border-gray-300 px-2 py-1 text-xs dark:border-gray-700 dark:bg-gray-800"
				>
					<option value="10">10</option>
					<option value="25">25</option>
					<option value="50">50</option>
				</select>
			</div>
			<div class="flex items-center gap-3">
				<span>
					{#if meta.total === 0}
						No results
					{:else}
						Showing {(meta.current_page - 1) * meta.per_page + 1} to {Math.min(meta.current_page * meta.per_page, meta.total)} of {meta.total} results
					{/if}
				</span>
				{#if totalPages > 1}
					<div class="flex items-center gap-1">
						<button type="button" disabled={meta.current_page <= 1} onclick={() => load(meta.current_page - 1)} class="rounded border border-gray-300 px-2 py-1 disabled:opacity-40 dark:border-gray-700">&lsaquo;</button>
						<span class="px-1">Page {meta.current_page} / {totalPages}</span>
						<button type="button" disabled={meta.current_page >= totalPages} onclick={() => load(meta.current_page + 1)} class="rounded border border-gray-300 px-2 py-1 disabled:opacity-40 dark:border-gray-700">&rsaquo;</button>
					</div>
				{/if}
			</div>
		</div>
	</div>
</div>

<Modal bind:open={createModalOpen} title="Create Access Token">
	<form onsubmit={(e) => { e.preventDefault(); handleCreateToken(); }} class="space-y-4">
		{#if createError}
			<p class="rounded-lg border border-rose-200 bg-rose-50 p-3 text-xs text-rose-700">{createError}</p>
		{/if}
		<div>
			<label for="token-name" class="mb-1.5 block text-xs font-semibold text-gray-700 dark:text-gray-300">Token Name <span class="text-red-500">*</span></label>
			<input id="token-name" type="text" bind:value={tokenName} placeholder="Enter a name for the token" maxlength="255" required class="w-full rounded-lg border border-gray-300 bg-white px-3.5 py-2.5 text-sm text-gray-900 placeholder-gray-400 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100" />
		</div>
		<div>
			<label for="token-expiry" class="mb-1.5 block text-xs font-semibold text-gray-700 dark:text-gray-300">Expiration Date <span class="text-red-500">*</span></label>
			<select id="token-expiry" bind:value={expiresAt} required class="w-full rounded-lg border border-gray-300 bg-white px-3.5 py-2.5 text-sm text-gray-900 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100">
				<option value="1">1 Day</option>
				<option value="7">7 Days</option>
				<option value="30">1 Month</option>
				<option value="90">3 Months</option>
				<option value="180">6 Months</option>
				<option value="365">1 Year</option>
				<option value="never">Never</option>
			</select>
		</div>
		<div class="flex justify-end gap-3 pt-2">
			<button type="button" onclick={() => (createModalOpen = false)} class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300">Cancel</button>
			<button type="submit" disabled={submitting} class="rounded-lg bg-teal-600 px-4 py-2 text-sm font-semibold text-white hover:bg-teal-500 disabled:opacity-50">{submitting ? 'Creating...' : 'Create'}</button>
		</div>
	</form>
</Modal>

<Modal bind:open={tokenModalOpen} title="View Token">
	<div class="space-y-4">
		<div>
			<label for="new-token" class="mb-1.5 block text-xs font-semibold text-gray-700 dark:text-gray-300">Your New Access Token</label>
			<div class="flex items-center gap-2">
				<input id="new-token" type="text" readonly value={newlyCreatedToken ?? ''} class="w-full rounded-lg border border-gray-300 bg-gray-50 px-3.5 py-2.5 font-mono text-xs text-gray-900 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100" />
				<button type="button" onclick={() => newlyCreatedToken && copyToClipboard(newlyCreatedToken)} class="rounded-lg bg-teal-600 px-3 py-2 text-xs font-semibold text-white hover:bg-teal-500">Copy</button>
			</div>
		</div>
		<div class="rounded-lg border border-amber-200 bg-amber-50 p-4 dark:border-amber-800 dark:bg-amber-950/30">
			<p class="text-sm font-semibold text-amber-800 dark:text-amber-300">Head's up!</p>
			<p class="mt-1 text-xs text-amber-700 dark:text-amber-400">This is the only time you will be able to view or copy this token, so be sure to copy it now and store it in a secure location.</p>
		</div>
		<div class="flex justify-end pt-1">
			<button type="button" onclick={() => { tokenModalOpen = false; newlyCreatedToken = null; }} class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300">Close</button>
		</div>
	</div>
</Modal>

<Modal bind:open={editOpen} title="Edit Access Token">
	<form onsubmit={(e) => { e.preventDefault(); handleRename(); }} class="space-y-4">
		{#if editError}
			<p class="rounded-lg border border-rose-200 bg-rose-50 p-3 text-xs text-rose-700">{editError}</p>
		{/if}
		<div>
			<label for="edit-token-name" class="mb-1.5 block text-xs font-semibold text-gray-700 dark:text-gray-300">Token Name <span class="text-red-500">*</span></label>
			<input id="edit-token-name" type="text" bind:value={editName} maxlength="255" required class="w-full rounded-lg border border-gray-300 bg-white px-3.5 py-2.5 text-sm text-gray-900 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100" />
		</div>
		<div class="flex justify-end gap-3 pt-2">
			<button type="button" onclick={() => (editOpen = false)} class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300">Cancel</button>
			<button type="submit" disabled={savingEdit} class="rounded-lg bg-teal-600 px-4 py-2 text-sm font-semibold text-white hover:bg-teal-500 disabled:opacity-50">{savingEdit ? 'Saving...' : 'Save changes'}</button>
		</div>
	</form>
</Modal>

<Modal bind:open={deleteOpen} title="Delete Access Token">
	<div class="space-y-4">
		<p class="text-sm text-gray-600 dark:text-gray-300">
			This action will permanently delete the access token <strong>{deleteTarget?.name ?? ''}</strong>. To continue, type the token name below.
		</p>
		{#if deleteError}
			<p class="rounded-lg border border-rose-200 bg-rose-50 p-3 text-xs text-rose-700">{deleteError}</p>
		{/if}
		<div>
			<label for="delete-token-name" class="mb-1.5 block text-xs font-semibold text-gray-700 dark:text-gray-300">Token name <span class="text-red-500">*</span></label>
			<input id="delete-token-name" type="text" bind:value={deleteName} placeholder={deleteTarget?.name ?? ''} class="w-full rounded-lg border border-gray-300 bg-white px-3.5 py-2.5 text-sm text-gray-900 focus:border-rose-600 focus:outline-none focus:ring-1 focus:ring-rose-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100" />
		</div>
		<div class="flex justify-end gap-3 pt-2">
			<button type="button" onclick={() => (deleteOpen = false)} class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300">Cancel</button>
			<button type="button" onclick={handleDelete} disabled={deleting || deleteName !== (deleteTarget?.name ?? '')} class="rounded-lg bg-rose-600 px-4 py-2 text-sm font-semibold text-white hover:bg-rose-500 disabled:opacity-50">{deleting ? 'Deleting...' : 'Delete'}</button>
		</div>
	</div>
</Modal>

<Modal bind:open={bulkDeleteOpen} title="Delete Selected Access Tokens">
	<div class="space-y-4">
		<p class="text-sm text-gray-600 dark:text-gray-300">Are you sure you want to delete the selected access tokens? This action cannot be undone.</p>
		<div class="flex justify-end gap-3 pt-2">
			<button type="button" onclick={() => (bulkDeleteOpen = false)} class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300">Cancel</button>
			<button type="button" onclick={handleBulkDelete} disabled={bulkDeleting} class="rounded-lg bg-rose-600 px-4 py-2 text-sm font-semibold text-white hover:bg-rose-500 disabled:opacity-50">{bulkDeleting ? 'Deleting...' : 'Delete'}</button>
		</div>
	</div>
</Modal>
