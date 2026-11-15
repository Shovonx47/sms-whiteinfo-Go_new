<script lang="ts">
	import { api } from '$lib/api.js';
	import { toast } from '$lib/stores/toast.svelte.js';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';

	interface ContactGroup {
		id: number;
		name: string;
		description?: string;
		contacts_count: number;
		created_at: string;
		updated_at?: string;
	}

	interface Meta {
		total: number;
		per_page: number;
		current_page: number;
		last_page?: number;
	}

	let groups = $state<ContactGroup[]>([]);
	let meta = $state<Meta>({ total: 0, per_page: 10, current_page: 1, last_page: 1 });
	let loading = $state(true);
	let search = $state('');
	let perPage = $state(10);
	let currentPage = $state(1);
	let searchTimeout: ReturnType<typeof setTimeout> | null = null;

	let selectedGroupId = $state<number | null>(null);
	let selectedIds = $state<number[]>([]);
	let bulkDeleteOpen = $state(false);
	let bulkDeleting = $state(false);
	let viewModalOpen = $state(false);
	let viewGroup = $state<ContactGroup | null>(null);
	let deleteConfirmOpen = $state(false);
	let groupToDelete = $state<ContactGroup | null>(null);

	async function load(p = 1) {
		loading = true;
		currentPage = p;
		selectedIds = [];
		try {
			const params = new URLSearchParams({ page: String(p), per_page: String(perPage) });
			if (search.trim()) params.set('search', search.trim());
			const data = await api.get<{ data: ContactGroup[]; meta: Meta }>(`/api/v1/customer/groups?${params}`);
			groups = data.data || [];
			meta = data.meta || { total: 0, per_page: perPage, current_page: p, last_page: 1 };
		} catch {
			groups = [];
			meta = { total: 0, per_page: perPage, current_page: p, last_page: 1 };
		} finally {
			loading = false;
		}
	}

	function handleSearchInput() {
		if (searchTimeout) clearTimeout(searchTimeout);
		searchTimeout = setTimeout(() => load(1), 300);
	}

	function toggleSelectAllGroups() {
		if (filteredGroups.length > 0 && selectedIds.length === filteredGroups.length) {
			selectedIds = [];
		} else {
			selectedIds = filteredGroups.map((g) => g.id);
		}
	}

	function toggleSelectGroup(id: number) {
		if (selectedIds.includes(id)) {
			selectedIds = selectedIds.filter((item) => item !== id);
		} else {
			selectedIds = [...selectedIds, id];
		}
	}

	async function handleBulkDeleteGroups() {
		if (selectedIds.length === 0) return;
		bulkDeleting = true;
		try {
			await api.post('/api/v1/customer/groups/bulk-delete', { ids: selectedIds });
			toast.success('Groups Deleted', `${selectedIds.length} group(s) deleted.`);
			bulkDeleteOpen = false;
			selectedIds = [];
			load(1);
		} catch (err: unknown) {
			const msg = err instanceof Error ? err.message : 'Failed to delete selected groups.';
			toast.error('Bulk Delete Error', msg);
		} finally {
			bulkDeleting = false;
		}
	}

	onMount(() => load(1));

	async function handleDeleteGroup() {
		if (!groupToDelete) return;
		try {
			await api.delete(`/api/v1/customer/groups/${groupToDelete.id}`);
			toast.success('Group Deleted', `Group "${groupToDelete.name}" was deleted.`);
			deleteConfirmOpen = false;
			groupToDelete = null;
			load(1);
		} catch (err: unknown) {
			const msg = err instanceof Error ? err.message : 'Failed to delete group.';
			toast.error('Delete Error', msg);
		}
	}

	let filteredGroups = $derived.by(() => {
		if (!search) return groups;
		const q = search.toLowerCase();
		return groups.filter(
			(g) =>
				g.name?.toLowerCase().includes(q) ||
				g.description?.toLowerCase().includes(q)
		);
	});
</script>

<svelte:head>
	<title>Groups - White INFO</title>
</svelte:head>

<div class="space-y-6">
	<!-- Breadcrumb -->
	<nav class="flex items-center gap-2 text-xs font-medium text-gray-500">
		<a href="/audience/groups" class="hover:text-gray-700">Groups</a>
		<svg class="h-3 w-3 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
			<path stroke-linecap="round" stroke-linejoin="round" d="m8.25 4.5 7.5 7.5-7.5 7.5" />
		</svg>
		<span class="text-gray-900 font-semibold">List</span>
	</nav>

	<!-- Page Heading & New group button -->
	<div class="flex items-center justify-between">
		<h1 class="text-2xl font-bold tracking-tight text-gray-950">Groups</h1>
		<a
			href="/audience/groups/create"
			class="inline-flex items-center gap-1.5 rounded-lg bg-teal-600 px-3.5 py-2 text-xs font-semibold text-white shadow-sm transition hover:bg-teal-700 cursor-pointer"
		>
			<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
			</svg>
			New group
		</a>
	</div>

	<!-- Main Card -->
	<div class="rounded-xl border border-gray-200 bg-white shadow-sm overflow-hidden">
		<!-- Toolbar: Bulk Delete + Search -->
		<div class="flex flex-wrap items-center justify-between gap-2 border-b border-gray-100 p-3 bg-white">
			<div>
				{#if selectedIds.length > 0}
					<button
						type="button"
						onclick={() => (bulkDeleteOpen = true)}
						class="inline-flex items-center gap-1.5 rounded-lg border border-red-200 bg-red-50 px-3 py-1.5 text-xs font-semibold text-red-700 hover:bg-red-100 cursor-pointer"
					>
						Delete Selected ({selectedIds.length})
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
					oninput={handleSearchInput}
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
								checked={filteredGroups.length > 0 && selectedIds.length === filteredGroups.length}
								onchange={toggleSelectAllGroups}
								class="rounded border-gray-300 text-teal-600 focus:ring-teal-500 cursor-pointer"
							/>
						</th>
						<th class="py-3 px-4">NAME</th>
						<th class="py-3 px-4">CONTACTS</th>
						<th class="py-3 px-4">DESCRIPTION</th>
						<th class="py-3 px-4">CREATED AT</th>
						<th class="py-3 px-4">UPDATED AT</th>
						<th class="py-3 px-4 text-right"></th>
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-100 bg-white">
					{#if loading}
						<tr>
							<td colspan="7" class="py-12 text-center text-gray-500">
								<div class="inline-block h-6 w-6 animate-spin rounded-full border-2 border-gray-300 border-t-teal-600"></div>
								<p class="mt-2 text-xs">Loading contact groups...</p>
							</td>
						</tr>
					{:else if filteredGroups.length === 0}
						<tr>
							<td colspan="7" class="py-12 text-center text-gray-500">
								<p class="text-sm font-medium text-gray-700">No groups found</p>
								<p class="mt-1 text-xs text-gray-400">Click "New group" above to create an audience segment.</p>
							</td>
						</tr>
					{:else}
						{#each filteredGroups as group (group.id)}
							<tr class="hover:bg-gray-50/70 transition">
								<td class="py-3 px-4">
									<input
										type="checkbox"
										checked={selectedIds.includes(group.id)}
										onchange={() => toggleSelectGroup(group.id)}
										class="rounded border-gray-300 text-teal-600 focus:ring-teal-500 cursor-pointer"
									/>
								</td>
								<td class="py-3 px-4 whitespace-nowrap">
									<a
										href="/audience/groups/{group.id}/edit"
										class="font-semibold text-gray-900 hover:text-teal-600 hover:underline"
									>
										{group.name}
									</a>
								</td>
								<td class="py-3 px-4 whitespace-nowrap">
									<span class="inline-flex items-center rounded-full bg-teal-50 px-2.5 py-0.5 text-xs font-semibold text-teal-700 border border-teal-200">
										{group.contacts_count || 0}
									</span>
								</td>
								<td class="py-3 px-4 text-gray-600 max-w-xs truncate">
									{group.description || '—'}
								</td>
								<td class="py-3 px-4 text-gray-600 whitespace-nowrap">
									{new Date(group.created_at).toLocaleDateString()}
								</td>
								<td class="py-3 px-4 text-gray-600 whitespace-nowrap">
									{new Date(group.updated_at || group.created_at).toLocaleDateString()}
								</td>
								<td class="py-3 px-4 text-right whitespace-nowrap">
									<div class="flex items-center justify-end gap-1">
										<button
											type="button"
											onclick={() => { viewGroup = group; viewModalOpen = true; }}
											class="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-700 cursor-pointer"
											title="View group"
										>
											<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
												<path stroke-linecap="round" stroke-linejoin="round" d="M2.036 12.322a1.012 1.012 0 0 1 0-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.963-7.178Z" />
												<path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z" />
											</svg>
										</button>
										<a
											href="/audience/groups/{group.id}/edit"
											class="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-teal-600 cursor-pointer"
											title="Edit group"
										>
											<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
												<path stroke-linecap="round" stroke-linejoin="round" d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L6.832 19.82a4.5 4.5 0 0 1-1.897 1.13l-2.685.8.8-2.685a4.5 4.5 0 0 1 1.13-1.897L16.863 4.487Zm0 0L19.5 7.125" />
											</svg>
										</a>
										<button
											type="button"
											onclick={() => { groupToDelete = group; deleteConfirmOpen = true; }}
											class="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-red-600 cursor-pointer"
											title="Delete group"
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
		<div class="flex flex-wrap items-center justify-between gap-2 border-t border-gray-200 px-4 py-3 bg-white text-xs text-gray-600">
			<div>
				Showing {filteredGroups.length > 0 ? (currentPage - 1) * perPage + 1 : 0} to {Math.min(currentPage * perPage, meta.total)} of {meta.total} results
			</div>
			<div class="flex items-center gap-3">
				<div class="flex items-center gap-2">
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
				{#if (meta.last_page ?? 1) > 1}
					<div class="flex items-center gap-1">
						<button
							type="button"
							disabled={currentPage <= 1}
							onclick={() => load(currentPage - 1)}
							class="rounded-lg border border-gray-300 bg-white px-2.5 py-1 text-xs font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-40 cursor-pointer"
						>
							Previous
						</button>
						<span class="px-2 text-xs text-gray-500">Page {currentPage} / {meta.last_page ?? 1}</span>
						<button
							type="button"
							disabled={currentPage >= (meta.last_page ?? 1)}
							onclick={() => load(currentPage + 1)}
							class="rounded-lg border border-gray-300 bg-white px-2.5 py-1 text-xs font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-40 cursor-pointer"
						>
							Next
						</button>
					</div>
				{/if}
			</div>
		</div>
	</div>
</div>

<!-- View Group Modal -->
{#if viewModalOpen && viewGroup}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 animate-fade-in backdrop-blur-xs">
		<div class="relative w-full max-w-md rounded-2xl bg-white p-6 shadow-xl border border-gray-100">
			<div class="flex items-center justify-between border-b border-gray-100 pb-3">
				<h3 class="text-base font-bold text-gray-900">Group Details</h3>
				<button
					type="button"
					onclick={() => (viewModalOpen = false)}
					class="text-gray-400 hover:text-gray-600 cursor-pointer"
				>
					<svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" />
					</svg>
				</button>
			</div>
			<div class="mt-4 grid grid-cols-2 gap-4 text-xs">
				<div>
					<p class="text-[11px] font-medium uppercase tracking-wider text-gray-400">Group Name</p>
					<p class="mt-1 font-bold text-gray-900">{viewGroup.name}</p>
				</div>
				<div>
					<p class="text-[11px] font-medium uppercase tracking-wider text-gray-400">Contacts</p>
					<p class="mt-1 font-semibold text-teal-700">{viewGroup.contacts_count || 0}</p>
				</div>
				<div class="col-span-2">
					<p class="text-[11px] font-medium uppercase tracking-wider text-gray-400">Description</p>
					<p class="mt-1 text-gray-700">{viewGroup.description || '—'}</p>
				</div>
				<div>
					<p class="text-[11px] font-medium uppercase tracking-wider text-gray-400">Created At</p>
					<p class="mt-1 text-gray-700">{new Date(viewGroup.created_at).toLocaleString()}</p>
				</div>
				<div>
					<p class="text-[11px] font-medium uppercase tracking-wider text-gray-400">Updated At</p>
					<p class="mt-1 text-gray-700">{new Date(viewGroup.updated_at || viewGroup.created_at).toLocaleString()}</p>
				</div>
			</div>
			<div class="mt-6 flex items-center justify-end gap-3 border-t border-gray-100 pt-3">
				<a
					href="/audience/groups/{viewGroup.id}/edit"
					class="rounded-lg bg-teal-600 px-3.5 py-1.5 text-xs font-semibold text-white hover:bg-teal-700"
				>
					Edit Group
				</a>
				<button
					type="button"
					onclick={() => (viewModalOpen = false)}
					class="rounded-lg border border-gray-300 bg-white px-3.5 py-1.5 text-xs font-semibold text-gray-700 hover:bg-gray-50 cursor-pointer"
				>
					Close
				</button>
			</div>
		</div>
	</div>
{/if}

<!-- Bulk Delete Groups Modal -->
{#if bulkDeleteOpen}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 animate-fade-in backdrop-blur-xs">
		<div class="relative w-full max-w-sm rounded-2xl bg-white p-6 shadow-xl border border-gray-100">
			<h3 class="text-base font-bold text-gray-900">Delete Selected Groups</h3>
			<p class="mt-2 text-xs text-gray-600">
				Are you sure you want to delete <span class="font-semibold text-gray-900">{selectedIds.length}</span> selected group{selectedIds.length === 1 ? '' : 's'}? Contacts inside will remain in your directory.
			</p>
			<div class="mt-6 flex items-center justify-end gap-3">
				<button
					type="button"
					onclick={() => (bulkDeleteOpen = false)}
					class="rounded-lg border border-gray-300 bg-white px-3.5 py-1.5 text-xs font-semibold text-gray-700 hover:bg-gray-50 cursor-pointer"
				>
					Cancel
				</button>
				<button
					type="button"
					disabled={bulkDeleting}
					onclick={handleBulkDeleteGroups}
					class="rounded-lg bg-red-600 px-3.5 py-1.5 text-xs font-semibold text-white shadow-xs hover:bg-red-700 disabled:opacity-50 cursor-pointer"
				>
					{bulkDeleting ? 'Deleting...' : 'Delete'}
				</button>
			</div>
		</div>
	</div>
{/if}

<!-- Delete Confirmation Modal -->
{#if deleteConfirmOpen && groupToDelete}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 animate-fade-in backdrop-blur-xs">
		<div class="relative w-full max-w-sm rounded-2xl bg-white p-6 shadow-xl border border-gray-100">
			<h3 class="text-base font-bold text-gray-900">Delete Group</h3>
			<p class="mt-2 text-xs text-gray-600">
				Are you sure you want to delete <span class="font-semibold text-gray-900">"{groupToDelete.name}"</span>? Any contacts in this group will remain in your contacts directory.
			</p>
			<div class="mt-6 flex items-center justify-end gap-3">
				<button
					type="button"
					onclick={() => { deleteConfirmOpen = false; groupToDelete = null; }}
					class="rounded-lg border border-gray-300 bg-white px-3.5 py-1.5 text-xs font-semibold text-gray-700 hover:bg-gray-50 cursor-pointer"
				>
					Cancel
				</button>
				<button
					type="button"
					onclick={handleDeleteGroup}
					class="rounded-lg bg-red-600 px-3.5 py-1.5 text-xs font-semibold text-white shadow-xs hover:bg-red-700 cursor-pointer"
				>
					Delete
				</button>
			</div>
		</div>
	</div>
{/if}
