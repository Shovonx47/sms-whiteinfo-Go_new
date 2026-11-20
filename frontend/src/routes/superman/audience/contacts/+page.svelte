<script lang="ts">
	import { onMount } from 'svelte';
	import { supermanApi } from '$lib/superman-api.js';
	import ExportDropdown from '$lib/components/common/ExportDropdown.svelte';
	import type { ExportColumn } from '$lib/utils/export.js';

	interface ContactItem {
		id: number;
		user_id: number | null;
		user_name: string | null;
		name: string;
		phone: string;
		email: string | null;
		group_name: string | null;
		meta: Record<string, string> | null;
		created_at: string;
		updated_at: string | null;
		deleted_at: string | null;
	}

	interface OwnerOption {
		id: number;
		name: string;
	}

	type TrashedFilter = 'without' | 'with' | 'only';
	type SortKey = 'user' | 'name' | 'phone' | 'created_at' | null;

	let contacts = $state<ContactItem[]>([]);
	let owners = $state<OwnerOption[]>([]);
	let loading = $state(true);
	let refreshing = $state(false);

	let search = $state('');
	let trashed = $state<TrashedFilter>('without');
	let page = $state(1);
	let perPage = $state(10);
	let total = $state(0);

	let sortKey = $state<SortKey>(null);
	let sortDir = $state<'asc' | 'desc'>('asc');

	let showCreatedAt = $state(false);
	let showFilterMenu = $state(false);
	let showColumnMenu = $state(false);

	let selected = $state<number[]>([]);
	let activeMenuId = $state<number | null>(null);

	let formOpen = $state(false);
	let formSubmitting = $state(false);
	let formError = $state<string | null>(null);
	let editingId = $state<number | null>(null);
	let form = $state({ user_id: 0, name: '', phone: '', meta: [] as Array<{ key: string; value: string }> });

	let viewOpen = $state(false);
	let viewing = $state<ContactItem | null>(null);

	let toast = $state<string | null>(null);
	let toastTimer: ReturnType<typeof setTimeout> | null = null;
	let searchTimer: ReturnType<typeof setTimeout> | null = null;

	function notify(message: string) {
		toast = message;
		if (toastTimer) clearTimeout(toastTimer);
		toastTimer = setTimeout(() => (toast = null), 3000);
	}

	function queryString(forExport = false): string {
		const params = new URLSearchParams({
			page: forExport ? '1' : String(page),
			per_page: forExport ? '20000' : String(perPage)
		});
		if (forExport) params.set('export', '1');
		if (search.trim()) params.set('search', search.trim());
		if (trashed === 'with') params.set('trashed', 'with');
		if (trashed === 'only') params.set('trashed', 'only');
		if (sortKey) {
			params.set('sort', sortKey);
			params.set('dir', sortDir);
		}
		return params.toString();
	}

	async function loadContacts(initial = false) {
		if (initial) loading = true;
		else refreshing = true;
		try {
			const res = await supermanApi.get<{ data: ContactItem[]; meta?: { total: number } }>(
				`/api/v1/superman/contacts?${queryString()}`
			);
			contacts = res.data ?? [];
			total = res.meta?.total ?? contacts.length;
			selected = selected.filter((id) => contacts.some((c) => c.id === id));
		} catch {
			// Never surface backend failures on this page: an empty table is the
			// old panel's "no records" state.
			contacts = [];
			total = 0;
		} finally {
			loading = false;
			refreshing = false;
		}
	}

	async function loadOwners() {
		try {
			const res = await supermanApi.get<{ data: OwnerOption[] }>('/api/v1/superman/contact-owners');
			owners = res.data ?? [];
		} catch {
			owners = [];
		}
	}

	function handleSearchInput() {
		if (searchTimer) clearTimeout(searchTimer);
		searchTimer = setTimeout(() => {
			page = 1;
			void loadContacts();
		}, 300);
	}

	function selectTrashed(value: TrashedFilter) {
		trashed = value;
		showFilterMenu = false;
		page = 1;
		selected = [];
		void loadContacts();
	}

	function toggleSort(key: Exclude<SortKey, null>) {
		if (sortKey === key) {
			sortDir = sortDir === 'asc' ? 'desc' : 'asc';
		} else {
			sortKey = key;
			sortDir = 'asc';
		}
		page = 1;
		void loadContacts();
	}

	function goToPage(next: number) {
		const last = Math.max(1, Math.ceil(total / perPage));
		if (next < 1 || next > last || next === page) return;
		page = next;
		void loadContacts();
	}

	function changePerPage(event: Event) {
		perPage = Number((event.target as HTMLSelectElement).value);
		page = 1;
		void loadContacts();
	}

	function toggleSelectAll(event: Event) {
		const checked = (event.target as HTMLInputElement).checked;
		selected = checked ? contacts.map((c) => c.id) : [];
	}

	function toggleSelect(id: number) {
		selected = selected.includes(id) ? selected.filter((value) => value !== id) : [...selected, id];
	}

	function toggleMenu(id: number, event: MouseEvent) {
		event.stopPropagation();
		activeMenuId = activeMenuId === id ? null : id;
	}

	function closeMenu() {
		activeMenuId = null;
		showFilterMenu = false;
		showColumnMenu = false;
	}

	function metaToRows(meta: Record<string, string> | null) {
		if (!meta || typeof meta !== 'object') return [];
		return Object.entries(meta).map(([key, value]) => ({ key, value: String(value ?? '') }));
	}

	function openCreate() {
		editingId = null;
		form = { user_id: owners[0]?.id ?? 0, name: '', phone: '', meta: [] };
		formError = null;
		formOpen = true;
	}

	function openEdit(contact: ContactItem) {
		closeMenu();
		editingId = contact.id;
		form = {
			user_id: contact.user_id ?? 0,
			name: contact.name ?? '',
			phone: contact.phone ?? '',
			meta: metaToRows(contact.meta)
		};
		formError = null;
		formOpen = true;
	}

	function openView(contact: ContactItem) {
		closeMenu();
		viewing = contact;
		viewOpen = true;
	}

	function addMetaRow() {
		form.meta = [...form.meta, { key: '', value: '' }];
	}

	function removeMetaRow(index: number) {
		form.meta = form.meta.filter((_, i) => i !== index);
	}

	async function submitForm() {
		if (!form.user_id) {
			formError = 'The owner field is required.';
			return;
		}
		if (!form.phone.trim()) {
			formError = 'The phone field is required.';
			return;
		}
		formSubmitting = true;
		formError = null;
		try {
			const meta: Record<string, string> = {};
			for (const row of form.meta) {
				if (row.key.trim()) meta[row.key.trim()] = row.value;
			}
			const payload = {
				user_id: form.user_id,
				name: form.name.trim(),
				phone: form.phone.trim(),
				meta
			};
			if (editingId === null) {
				await supermanApi.post('/api/v1/superman/contacts', payload);
				notify('Contact created.');
			} else {
				await supermanApi.put(`/api/v1/superman/contacts/${editingId}`, payload);
				notify('Contact updated.');
			}
			formOpen = false;
			await loadContacts();
		} catch {
			formError = 'The contact could not be saved. Please try again.';
		} finally {
			formSubmitting = false;
		}
	}

	async function runBulk(action: 'delete' | 'restore' | 'force_delete') {
		if (selected.length === 0) return;
		const labels = {
			delete: `Delete ${selected.length} contact${selected.length === 1 ? '' : 's'}?`,
			restore: `Restore ${selected.length} contact${selected.length === 1 ? '' : 's'}?`,
			force_delete: `Permanently delete ${selected.length} contact${selected.length === 1 ? '' : 's'}? This cannot be undone.`
		};
		if (!confirm(labels[action])) return;
		try {
			await supermanApi.post('/api/v1/superman/contacts/bulk', { ids: selected, action });
			notify(
				action === 'delete'
					? 'Selected contacts deleted.'
					: action === 'restore'
						? 'Selected contacts restored.'
						: 'Selected contacts permanently deleted.'
			);
			selected = [];
			await loadContacts();
		} catch {
			notify('The action could not be completed. Please try again.');
		}
	}

	const totalPages = $derived(Math.max(1, Math.ceil(total / perPage)));
	const allSelected = $derived(contacts.length > 0 && selected.length === contacts.length);

	const exportColumns: ExportColumn<ContactItem>[] = [
		{ header: 'User', getValue: (c) => c.user_name ?? '—' },
		{ header: 'Name', getValue: (c) => c.name },
		{ header: 'Phone', getValue: (c) => c.phone },
		{ header: 'Created At', getValue: (c) => (c.created_at ? new Date(c.created_at).toLocaleString() : '') }
	];

	async function exportData(): Promise<ContactItem[]> {
		try {
			const res = await supermanApi.get<{ data: ContactItem[] }>(
				`/api/v1/superman/contacts?${queryString(true)}`
			);
			return res.data ?? [];
		} catch {
			return [];
		}
	}

	onMount(() => {
		void loadContacts(true);
		void loadOwners();
	});
</script>

<svelte:window onclick={closeMenu} />

<svelte:head>
	<title>Contacts — Superman Administrator</title>
</svelte:head>

<div class="space-y-6">
	<div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
		<h1 class="text-3xl font-bold tracking-tight text-gray-950 dark:text-white">Contacts</h1>
		<div class="flex items-center gap-2">
			<ExportDropdown
				title="Contacts"
				columns={exportColumns}
				getData={exportData}
				disabled={total === 0}
			/>
			<button
				type="button"
				onclick={openCreate}
				class="inline-flex items-center gap-1.5 rounded-lg bg-emerald-600 px-3.5 py-2 text-xs font-bold text-white shadow-xs hover:bg-emerald-700"
			>
				+ New contact
			</button>
		</div>
	</div>

	<div class="rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
		<div class="flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between">
			<div class="text-xs text-gray-500">
				{#if trashed !== 'without'}
					Filtered by <span class="font-semibold text-gray-800 dark:text-gray-200">{trashed === 'only' ? 'only trashed' : 'with trashed'}</span>
					<button type="button" onclick={() => selectTrashed('without')} class="ml-2 font-semibold text-emerald-600 hover:underline">
						Clear
					</button>
				{/if}
			</div>

			<div class="flex items-center justify-end gap-2.5">
				<div class="relative w-full max-w-xs">
					<svg class="absolute left-3 top-2.5 h-3.5 w-3.5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
					</svg>
					<input
						type="search"
						bind:value={search}
						oninput={handleSearchInput}
						placeholder="Search"
						class="w-full rounded-lg border border-gray-200 bg-white py-1.5 pl-8 pr-3 text-xs outline-none focus:border-emerald-500 dark:border-gray-700 dark:bg-gray-800 dark:text-white"
					/>
				</div>

				<div class="relative">
					<button
						type="button"
						onclick={(e) => {
							e.stopPropagation();
							showFilterMenu = !showFilterMenu;
							showColumnMenu = false;
						}}
						class="rounded-lg border border-gray-200 bg-white p-2 text-gray-500 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-400"
						title="Filters"
					>
						<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" d="M12 3c2.755 0 5.455.232 8.083.678.533.09.917.556.917 1.096v1.044a2.25 2.25 0 0 1-.659 1.591l-5.432 5.432a2.25 2.25 0 0 0-.659 1.591v2.927a2.25 2.25 0 0 1-1.244 2.013L9.75 21v-6.568a2.25 2.25 0 0 0-.659-1.591L3.659 7.409A2.25 2.25 0 0 1 3 5.818V4.774c0-.54.384-1.006.917-1.096A48.32 48.32 0 0 1 12 3Z" />
						</svg>
					</button>
					<span class="absolute -right-1.5 -top-1.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-gray-200 px-1 text-[10px] font-bold text-gray-700 dark:bg-gray-700 dark:text-gray-200">
						{trashed === 'without' ? 0 : 1}
					</span>

					{#if showFilterMenu}
						<div class="absolute right-0 top-full z-30 mt-1.5 w-52 rounded-xl border border-gray-100 bg-white p-1.5 text-left shadow-xl dark:border-gray-800 dark:bg-gray-900">
							<p class="px-2 py-1 text-[10px] font-bold uppercase tracking-wider text-gray-400">Trashed records</p>
							{#each [{ value: 'without', label: 'Without trashed' }, { value: 'with', label: 'With trashed' }, { value: 'only', label: 'Only trashed' }] as option (option.value)}
								<button
									type="button"
									onclick={() => selectTrashed(option.value as TrashedFilter)}
									class="flex w-full items-center justify-between rounded-lg px-2 py-1.5 text-xs text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-gray-800"
								>
									{option.label}
									{#if trashed === option.value}
										<svg class="h-3.5 w-3.5 text-emerald-600" fill="none" viewBox="0 0 24 24" stroke-width="2.5" stroke="currentColor">
											<path stroke-linecap="round" stroke-linejoin="round" d="m4.5 12.75 6 6 9-13.5" />
										</svg>
									{/if}
								</button>
							{/each}
						</div>
					{/if}
				</div>

				<div class="relative">
					<button
						type="button"
						onclick={(e) => {
							e.stopPropagation();
							showColumnMenu = !showColumnMenu;
							showFilterMenu = false;
						}}
						class="rounded-lg border border-gray-200 bg-white p-2 text-gray-500 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-400"
						title="Column settings"
					>
						<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" d="M3.75 5.25h16.5M3.75 12h16.5M3.75 18.75h16.5" />
						</svg>
					</button>

					{#if showColumnMenu}
						<div class="absolute right-0 top-full z-30 mt-1.5 w-52 rounded-xl border border-gray-100 bg-white p-1.5 text-left shadow-xl dark:border-gray-800 dark:bg-gray-900">
							<p class="px-2 py-1 text-[10px] font-bold uppercase tracking-wider text-gray-400">Visible columns</p>
							<label class="flex w-full cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 text-xs text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-gray-800">
								<input type="checkbox" checked disabled class="h-3.5 w-3.5 rounded border-gray-300 text-emerald-600" />
								User
							</label>
							<label class="flex w-full cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 text-xs text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-gray-800">
								<input type="checkbox" checked disabled class="h-3.5 w-3.5 rounded border-gray-300 text-emerald-600" />
								Name
							</label>
							<label class="flex w-full cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 text-xs text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-gray-800">
								<input type="checkbox" checked disabled class="h-3.5 w-3.5 rounded border-gray-300 text-emerald-600" />
								Phone
							</label>
							<label class="flex w-full cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 text-xs text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-gray-800">
								<input
									type="checkbox"
									bind:checked={showCreatedAt}
									onchange={() => loadContacts()}
									class="h-3.5 w-3.5 rounded border-gray-300 text-emerald-600"
								/>
								Created At
							</label>
						</div>
					{/if}
				</div>
			</div>
		</div>

		{#if selected.length > 0}
			<div class="flex flex-wrap items-center gap-2 border-t border-gray-100 bg-emerald-50/70 px-4 py-2.5 text-xs dark:border-gray-800 dark:bg-emerald-950/20">
				<span class="font-semibold text-gray-800 dark:text-gray-100">{selected.length} selected</span>
				<div class="ml-auto flex flex-wrap items-center gap-2">
					{#if trashed !== 'only'}
						<button type="button" onclick={() => void runBulk('delete')} class="rounded-lg border border-rose-200 bg-white px-2.5 py-1 font-semibold text-rose-700 hover:bg-rose-50 dark:border-rose-900 dark:bg-gray-900 dark:text-rose-300">
							Delete selected
						</button>
					{/if}
					{#if trashed !== 'without'}
						<button type="button" onclick={() => void runBulk('restore')} class="rounded-lg border border-gray-200 bg-white px-2.5 py-1 font-semibold text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-200">
							Restore selected
						</button>
						<button type="button" onclick={() => void runBulk('force_delete')} class="rounded-lg border border-rose-200 bg-white px-2.5 py-1 font-semibold text-rose-700 hover:bg-rose-50 dark:border-rose-900 dark:bg-gray-900 dark:text-rose-300">
							Delete permanently
						</button>
					{/if}
					<button type="button" onclick={() => (selected = [])} class="rounded-lg px-2 py-1 text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200">
						Cancel
					</button>
				</div>
			</div>
		{/if}

		<div class="overflow-x-auto">
			<table class="w-full text-left text-xs">
				<thead>
					<tr class="border-y border-gray-100 text-[11px] font-bold uppercase text-gray-700 dark:border-gray-800 dark:text-gray-300">
						<th class="w-10 px-4 py-3.5">
							<input
								type="checkbox"
								checked={allSelected}
								onchange={toggleSelectAll}
								aria-label="Select all contacts"
								class="h-3.5 w-3.5 rounded border-gray-300 text-emerald-600"
							/>
						</th>
						<th class="px-4 py-3.5">
							<button type="button" onclick={() => toggleSort('user')} class="inline-flex items-center gap-1 uppercase hover:text-gray-900 dark:hover:text-white">
								User
								<svg class="h-3 w-3 {sortKey === 'user' ? 'text-emerald-600' : 'text-gray-400'}" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
									<path stroke-linecap="round" stroke-linejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" />
								</svg>
							</button>
						</th>
						<th class="px-4 py-3.5">
							<button type="button" onclick={() => toggleSort('name')} class="inline-flex items-center gap-1 uppercase hover:text-gray-900 dark:hover:text-white">
								Name
								<svg class="h-3 w-3 {sortKey === 'name' ? 'text-emerald-600' : 'text-gray-400'}" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
									<path stroke-linecap="round" stroke-linejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" />
								</svg>
							</button>
						</th>
						<th class="px-4 py-3.5">
							<button type="button" onclick={() => toggleSort('phone')} class="inline-flex items-center gap-1 uppercase hover:text-gray-900 dark:hover:text-white">
								Phone
								<svg class="h-3 w-3 {sortKey === 'phone' ? 'text-emerald-600' : 'text-gray-400'}" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
									<path stroke-linecap="round" stroke-linejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" />
								</svg>
							</button>
						</th>
						{#if showCreatedAt}
							<th class="px-4 py-3.5">
								<button type="button" onclick={() => toggleSort('created_at')} class="inline-flex items-center gap-1 uppercase hover:text-gray-900 dark:hover:text-white">
									Created At
									<svg class="h-3 w-3 {sortKey === 'created_at' ? 'text-emerald-600' : 'text-gray-400'}" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
										<path stroke-linecap="round" stroke-linejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" />
									</svg>
								</button>
							</th>
						{/if}
						<th class="w-16 px-4 py-3.5"></th>
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-100 dark:divide-gray-800">
					{#if loading}
						<tr>
							<td colspan={showCreatedAt ? 6 : 5} class="px-4 py-12 text-center text-xs text-gray-400">Loading contacts…</td>
						</tr>
					{:else if contacts.length === 0}
						<tr>
							<td colspan={showCreatedAt ? 6 : 5} class="px-4 py-12 text-center text-xs text-gray-400">No contacts found.</td>
						</tr>
					{:else}
						{#each contacts as contact (contact.id)}
							<tr class="transition-colors hover:bg-gray-50/70 dark:hover:bg-gray-800/40 {contact.deleted_at ? 'opacity-60' : ''}">
								<td class="px-4 py-4">
									<input
										type="checkbox"
										checked={selected.includes(contact.id)}
										onchange={() => toggleSelect(contact.id)}
										aria-label={`Select ${contact.phone}`}
										class="h-3.5 w-3.5 rounded border-gray-300 text-emerald-600"
									/>
								</td>
								<td class="px-4 py-4 font-bold text-gray-900 dark:text-white">{contact.user_name ?? '—'}</td>
								<td class="px-4 py-4 text-gray-700 dark:text-gray-300">{contact.name || '—'}</td>
								<td class="px-4 py-4 font-mono text-gray-700 dark:text-gray-300">{contact.phone}</td>
								{#if showCreatedAt}
									<td class="px-4 py-4 font-mono text-[11px] text-gray-500 dark:text-gray-400">
										{contact.created_at ? new Date(contact.created_at).toLocaleString() : '—'}
									</td>
								{/if}
								<td class="relative px-4 py-4 text-right">
									<button
										type="button"
										onclick={(e) => toggleMenu(contact.id, e)}
										class="rounded-lg border border-gray-200 p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:border-gray-700 dark:hover:bg-gray-800 dark:hover:text-gray-200"
										aria-label="Contact actions"
									>
										<svg class="h-4 w-4" fill="currentColor" viewBox="0 0 24 24">
											<path d="M12 8c1.1 0 2-.9 2-2s-.9-2-2-2-2 .9-2 2 .9 2 2 2zm0 2c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm0 6c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2z" />
										</svg>
									</button>

									{#if activeMenuId === contact.id}
										<div class="absolute right-4 top-12 z-30 w-44 rounded-xl border border-gray-100 bg-white p-1 text-left shadow-xl dark:border-gray-800 dark:bg-gray-900">
											<button
												type="button"
												onclick={() => openView(contact)}
												class="flex w-full items-center gap-2 rounded-lg px-2.5 py-1.5 text-xs text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-gray-800"
											>
												<svg class="h-3.5 w-3.5 text-purple-500" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
													<path stroke-linecap="round" stroke-linejoin="round" d="M2.036 12.322a1.012 1.012 0 0 1 0-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.963-7.178Z" />
													<path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z" />
												</svg>
												View
											</button>
											<button
												type="button"
												onclick={() => openEdit(contact)}
												class="flex w-full items-center gap-2 rounded-lg px-2.5 py-1.5 text-xs text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-gray-800"
											>
												<svg class="h-3.5 w-3.5 text-emerald-500" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
													<path stroke-linecap="round" stroke-linejoin="round" d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L10.582 16.07a4.5 4.5 0 0 1-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 0 1 1.13-1.897l8.932-8.931Z" />
												</svg>
												Edit
											</button>
										</div>
									{/if}
								</td>
							</tr>
						{/each}
					{/if}
				</tbody>
			</table>
		</div>

		<div class="flex flex-col gap-3 border-t border-gray-100 p-4 text-xs text-gray-500 sm:flex-row sm:items-center sm:justify-between dark:border-gray-800">
			<div>
				Showing {total === 0 ? 0 : (page - 1) * perPage + 1} to {Math.min(page * perPage, total)} of {total} results
			</div>
			<div class="flex items-center gap-4">
				<div class="flex items-center gap-2">
					<span>Per page</span>
					<select
						value={perPage}
						onchange={changePerPage}
						class="rounded-lg border border-gray-200 bg-white py-1 px-2 text-xs text-gray-700 outline-none dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300"
					>
						<option value={10}>10</option>
						<option value={25}>25</option>
						<option value={50}>50</option>
						<option value={100}>100</option>
					</select>
				</div>

				<div class="flex items-center gap-1">
					{#each Array.from({ length: totalPages }, (_, i) => i + 1) as p (p)}
						<button
							type="button"
							onclick={() => goToPage(p)}
							class="h-7 w-7 rounded-lg border text-xs font-semibold transition {p === page
								? 'border-emerald-600 bg-emerald-600 text-white'
								: 'border-gray-200 bg-white text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300'}"
						>
							{p}
						</button>
					{/each}
					{#if totalPages > page}
						<button
							type="button"
							onclick={() => goToPage(page + 1)}
							class="h-7 w-7 rounded-lg border border-gray-200 bg-white text-xs font-semibold text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300"
						>
							&gt;
						</button>
					{/if}
				</div>
			</div>
		</div>

		{#if refreshing}
			<div class="border-t border-gray-100 px-4 py-1.5 text-[10px] text-gray-400 dark:border-gray-800">Updating…</div>
		{/if}
	</div>
</div>

{#if formOpen}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4">
		<div class="max-h-[90vh] w-full max-w-lg space-y-4 overflow-y-auto rounded-2xl border border-gray-100 bg-white p-6 shadow-2xl dark:border-gray-800 dark:bg-gray-900">
			<div class="flex items-center justify-between border-b border-gray-100 pb-3 dark:border-gray-800">
				<h3 class="text-base font-bold text-gray-900 dark:text-white">
					{editingId === null ? 'Create Contact' : 'Edit Contact'}
				</h3>
				<button type="button" onclick={() => (formOpen = false)} class="text-sm text-gray-400 hover:text-gray-600">✕</button>
			</div>

			<p class="text-[11px] font-bold uppercase tracking-wider text-gray-400">Contact Details</p>

			<div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
				<div class="sm:col-span-2">
					<label class="mb-1 block text-xs font-medium text-gray-700 dark:text-gray-300" for="contact-owner">
						User <span class="text-rose-500">*</span>
					</label>
					<select
						id="contact-owner"
						bind:value={form.user_id}
						class="w-full rounded-lg border border-gray-300 px-3 py-1.5 text-xs dark:border-gray-700 dark:bg-gray-800 dark:text-white"
					>
						<option value={0}>Select a user</option>
						{#each owners as owner (owner.id)}
							<option value={owner.id}>{owner.name || `#${owner.id}`}</option>
						{/each}
					</select>
				</div>

				<div>
					<label class="mb-1 block text-xs font-medium text-gray-700 dark:text-gray-300" for="contact-name">Name</label>
					<input
						id="contact-name"
						type="text"
						maxlength="255"
						bind:value={form.name}
						class="w-full rounded-lg border border-gray-300 px-3 py-1.5 text-xs dark:border-gray-700 dark:bg-gray-800 dark:text-white"
					/>
				</div>

				<div>
					<label class="mb-1 block text-xs font-medium text-gray-700 dark:text-gray-300" for="contact-phone">
						Phone <span class="text-rose-500">*</span>
					</label>
					<input
						id="contact-phone"
						type="text"
						maxlength="255"
						bind:value={form.phone}
						class="w-full rounded-lg border border-gray-300 px-3 py-1.5 text-xs dark:border-gray-700 dark:bg-gray-800 dark:text-white"
					/>
				</div>

				<div class="sm:col-span-2">
					<div class="mb-1 flex items-center justify-between">
						<span class="text-xs font-medium text-gray-700 dark:text-gray-300">Meta</span>
						<button type="button" onclick={addMetaRow} class="text-[11px] font-semibold text-emerald-600 hover:underline">
							+ Add key / value
						</button>
					</div>
					{#if form.meta.length === 0}
						<p class="rounded-lg border border-dashed border-gray-200 px-3 py-2 text-[11px] text-gray-400 dark:border-gray-700">
							No meta entries.
						</p>
					{:else}
						<div class="space-y-2">
							{#each form.meta as row, index (index)}
								<div class="flex items-center gap-2">
									<input
										type="text"
										placeholder="Key"
										bind:value={row.key}
										class="w-1/3 rounded-lg border border-gray-300 px-2.5 py-1.5 text-xs dark:border-gray-700 dark:bg-gray-800 dark:text-white"
									/>
									<input
										type="text"
										placeholder="Value"
										bind:value={row.value}
										class="flex-1 rounded-lg border border-gray-300 px-2.5 py-1.5 text-xs dark:border-gray-700 dark:bg-gray-800 dark:text-white"
									/>
									<button
										type="button"
										onclick={() => removeMetaRow(index)}
										class="rounded-lg border border-gray-200 px-2 py-1.5 text-xs text-rose-600 hover:bg-rose-50 dark:border-gray-700"
										aria-label="Remove meta entry"
									>
										✕
									</button>
								</div>
							{/each}
						</div>
					{/if}
				</div>
			</div>

			{#if formError}
				<p class="text-xs text-rose-600">{formError}</p>
			{/if}

			<div class="flex items-center justify-end gap-2 border-t border-gray-100 pt-3 dark:border-gray-800">
				<button
					type="button"
					onclick={() => (formOpen = false)}
					class="rounded-lg border border-gray-300 px-3.5 py-2 text-xs font-semibold text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300"
				>
					Cancel
				</button>
				<button
					type="button"
					disabled={formSubmitting}
					onclick={submitForm}
					class="rounded-lg bg-emerald-600 px-4 py-2 text-xs font-bold text-white hover:bg-emerald-700 disabled:opacity-50"
				>
					{formSubmitting ? 'Saving…' : editingId === null ? 'Create' : 'Save changes'}
				</button>
			</div>
		</div>
	</div>
{/if}

{#if viewOpen && viewing}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4">
		<div class="w-full max-w-lg space-y-4 rounded-2xl border border-gray-100 bg-white p-6 shadow-2xl dark:border-gray-800 dark:bg-gray-900">
			<div class="flex items-center justify-between border-b border-gray-100 pb-3 dark:border-gray-800">
				<h3 class="text-base font-bold text-gray-900 dark:text-white">Contact Details</h3>
				<button type="button" onclick={() => (viewOpen = false)} class="text-sm text-gray-400 hover:text-gray-600">✕</button>
			</div>

			<div class="grid grid-cols-1 gap-4 text-xs sm:grid-cols-2">
				<div class="space-y-1">
					<span class="block text-gray-400">User</span>
					<span class="font-bold text-gray-900 dark:text-white">{viewing.user_name ?? '—'}</span>
				</div>
				<div class="space-y-1">
					<span class="block text-gray-400">Contact Name</span>
					<span class="text-gray-800 dark:text-gray-200">{viewing.name || '—'}</span>
				</div>
				<div class="space-y-1">
					<span class="block text-gray-400">Phone</span>
					<span class="font-mono text-gray-800 dark:text-gray-200">{viewing.phone}</span>
				</div>
				<div class="space-y-1">
					<span class="block text-gray-400">Created At</span>
					<span class="text-gray-800 dark:text-gray-200">{viewing.created_at ? new Date(viewing.created_at).toLocaleString() : '—'}</span>
				</div>
				<div class="space-y-1">
					<span class="block text-gray-400">Updated At</span>
					<span class="text-gray-800 dark:text-gray-200">{viewing.updated_at ? new Date(viewing.updated_at).toLocaleString() : '—'}</span>
				</div>
				{#if viewing.group_name}
					<div class="space-y-1">
						<span class="block text-gray-400">Group</span>
						<span class="text-gray-800 dark:text-gray-200">{viewing.group_name}</span>
					</div>
				{/if}
				{#if viewing.deleted_at}
					<div class="space-y-1">
						<span class="block text-gray-400">Deleted At</span>
						<span class="text-rose-600 dark:text-rose-400">{new Date(viewing.deleted_at).toLocaleString()}</span>
					</div>
				{/if}
			</div>

			{#if viewing.meta && Object.keys(viewing.meta).length > 0}
				<div class="border-t border-gray-100 pt-3 dark:border-gray-800">
					<p class="mb-2 text-[10px] font-bold uppercase tracking-wider text-gray-400">Meta</p>
					<dl class="grid grid-cols-1 gap-2 text-xs sm:grid-cols-2">
						{#each Object.entries(viewing.meta) as [key, value] (key)}
							<div class="rounded-lg border border-gray-100 px-2.5 py-1.5 dark:border-gray-800">
								<dt class="text-[10px] uppercase tracking-wider text-gray-400">{key}</dt>
								<dd class="text-gray-800 dark:text-gray-200">{value}</dd>
							</div>
						{/each}
					</dl>
				</div>
			{/if}

			<div class="flex items-center justify-end gap-2 border-t border-gray-100 pt-3 dark:border-gray-800">
				<button
					type="button"
					onclick={() => (viewOpen = false)}
					class="rounded-lg border border-gray-300 px-3.5 py-2 text-xs font-semibold text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300"
				>
					Close
				</button>
				<button
					type="button"
					onclick={() => {
						const contact = viewing;
						viewOpen = false;
						if (contact) openEdit(contact);
					}}
					class="rounded-lg bg-emerald-600 px-4 py-2 text-xs font-bold text-white hover:bg-emerald-700"
				>
					Edit contact
				</button>
			</div>
		</div>
	</div>
{/if}

{#if toast}
	<div class="fixed bottom-5 right-5 z-50 rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-2.5 text-xs font-semibold text-emerald-800 shadow-lg dark:border-emerald-800 dark:bg-emerald-950/80 dark:text-emerald-200">
		{toast}
	</div>
{/if}
