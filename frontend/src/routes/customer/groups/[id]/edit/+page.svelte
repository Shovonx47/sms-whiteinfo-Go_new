<script lang="ts">
	import { api } from '$lib/api.js';
	import { toast } from '$lib/stores/toast.svelte.js';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';

	interface Contact {
		id: number;
		name: string;
		phone: string;
		created_at: string;
		updated_at?: string;
	}

	interface GroupDetail {
		id: number;
		name: string;
		description: string;
		contacts_count: number;
		created_at: string;
	}

	let groupId = $derived(page.params.id);

	let group = $state<GroupDetail | null>(null);
	let groupName = $state('');
	let groupDesc = $state('');
	let savingGroup = $state(false);

	let contacts = $state<Contact[]>([]);
	let loadingContacts = $state(true);
	let contactSearch = $state('');
	let selectedContactIds = $state<number[]>([]);
	let exporting = $state(false);

	// Modals
	let viewGroupModalOpen = $state(false);
	let bulkDetachModalOpen = $state(false);
	let bulkDetaching = $state(false);

	let attachModalOpen = $state(false);
	let attachSearch = $state('');
	let attachOptions = $state<Contact[]>([]);
	let attachLoading = $state(false);
	let attachSelectedIds = $state<number[]>([]);
	let attaching = $state(false);
	let attachSearchTimeout: ReturnType<typeof setTimeout> | null = null;

	let addNumbersModalOpen = $state(false);
	let rawNumbers = $state('');
	let newContactName = $state('');
	let addingNumbers = $state(false);

	let importModalOpen = $state(false);
	let importFile = $state<File | null>(null);
	let importFileName = $state('');
	let importing = $state(false);

	let createContactModalOpen = $state(false);
	let singleName = $state('');
	let singlePhone = $state('');
	let creatingContact = $state(false);

	let deleteGroupModalOpen = $state(false);

	async function loadGroup() {
		try {
			const res = await api.get<{ data: GroupDetail }>(`/api/v1/customer/groups/${groupId}`);
			group = res.data;
			groupName = res.data.name;
			groupDesc = res.data.description || '';
		} catch (err: unknown) {
			const msg = err instanceof Error ? err.message : 'Failed to load group.';
			toast.error('Group Error', msg);
		}
	}

	async function loadContacts() {
		loadingContacts = true;
		selectedContactIds = [];
		try {
			const res = await api.get<{ data: Contact[] }>(`/api/v1/customer/groups/${groupId}/contacts`);
			contacts = res.data || [];
		} catch {
			contacts = [];
		} finally {
			loadingContacts = false;
		}
	}

	onMount(() => {
		loadGroup();
		loadContacts();
	});

	async function handleSaveGroup() {
		if (!groupName.trim()) {
			toast.error('Validation error', 'Group name is required.');
			return;
		}
		savingGroup = true;
		try {
			await api.put(`/api/v1/customer/groups/${groupId}`, {
				name: groupName.trim(),
				description: groupDesc.trim()
			});
			toast.success('Group Saved', 'Group details updated successfully.');
		} catch (err: unknown) {
			const msg = err instanceof Error ? err.message : 'Failed to save group.';
			toast.error('Save Error', msg);
		} finally {
			savingGroup = false;
		}
	}

	async function handleDeleteGroup() {
		try {
			await api.delete(`/api/v1/customer/groups/${groupId}`);
			toast.success('Group Deleted', 'Group was deleted successfully.');
			goto('/audience/groups');
		} catch (err: unknown) {
			const msg = err instanceof Error ? err.message : 'Failed to delete group.';
			toast.error('Delete Error', msg);
		}
	}

	// 1. Add Numbers Handler
	async function handleAddNumbers() {
		if (!rawNumbers.trim()) {
			toast.error('Validation error', 'Please enter at least one phone number.');
			return;
		}
		addingNumbers = true;
		try {
			const res = await api.post<{ valid_count: number; invalid_count?: number; message: string }>(`/api/v1/customer/groups/${groupId}/contacts`, {
				phone_numbers: rawNumbers.trim(),
				name: newContactName.trim()
			});
			toast.success('Contacts Added', res.message || `Added contacts to group.`);
			if ((res.invalid_count ?? 0) > 0) {
				toast.warning('Notice', `${res.invalid_count} invalid number(s) were skipped.`);
			}
			addNumbersModalOpen = false;
			rawNumbers = '';
			newContactName = '';
			loadContacts();
			loadGroup();
		} catch (err: unknown) {
			const msg = err instanceof Error ? err.message : 'Failed to add phone numbers.';
			toast.error('Add Error', msg);
		} finally {
			addingNumbers = false;
		}
	}

	// 2. Import CSV / Excel Handler
	function handleFileSelected(e: Event) {
		const target = e.target as HTMLInputElement;
		const file = target.files?.[0];
		if (!file) return;
		importFile = file;
		importFileName = file.name;
	}

	async function handleImportSubmit() {
		if (!importFile) {
			toast.error('File Required', 'Please choose a CSV or Excel file to import.');
			return;
		}
		importing = true;
		try {
			const fd = new FormData();
			fd.append('file', importFile);
			const res = await api.upload<{ message: string; valid_count: number; invalid_count: number }>(
				`/api/v1/customer/groups/${groupId}/contacts/import`,
				fd
			);
			toast.success('Import Completed', res.message || `Imported ${res.valid_count} contact(s) into group.`);
			if ((res.invalid_count ?? 0) > 0) {
				toast.warning('Notice', `${res.invalid_count} row(s) without valid phone numbers were skipped.`);
			}
			importModalOpen = false;
			importFile = null;
			importFileName = '';
			loadContacts();
			loadGroup();
		} catch (err: unknown) {
			const msg = err instanceof Error ? err.message : 'Import failed.';
			toast.error('Import Error', msg);
		} finally {
			importing = false;
		}
	}

	// 3. Create Single Contact
	async function handleCreateSingleContact() {
		if (!singlePhone.trim()) {
			toast.error('Validation error', 'Phone number is required.');
			return;
		}
		creatingContact = true;
		try {
			await api.post(`/api/v1/customer/groups/${groupId}/contacts`, {
				phone_numbers: singlePhone.trim(),
				name: singleName.trim()
			});
			toast.success('Contact Created', `${singlePhone} created and attached to group.`);
			createContactModalOpen = false;
			singleName = '';
			singlePhone = '';
			loadContacts();
			loadGroup();
		} catch (err: unknown) {
			const msg = err instanceof Error ? err.message : 'Failed to create contact.';
			toast.error('Create Error', msg);
		} finally {
			creatingContact = false;
		}
	}

	// 4. Detach Contact
	async function handleDetachContact(contactId: number) {
		try {
			await api.delete(`/api/v1/customer/groups/${groupId}/contacts/${contactId}`);
			toast.success('Contact Detached', 'Contact removed from this group.');
			loadContacts();
			loadGroup();
		} catch (err: unknown) {
			const msg = err instanceof Error ? err.message : 'Failed to detach contact.';
			toast.error('Detach Error', msg);
		}
	}

	// 5. Export Contacts
	async function handleExportContacts() {
		if (contacts.length === 0) {
			toast.info('No Contacts', 'There are no contacts to export in this group.');
			return;
		}
		exporting = true;
		try {
			const params = new URLSearchParams();
			if (contactSearch.trim()) params.set('search', contactSearch.trim());
			const qs = params.toString();
			const blob = await api.download(`/api/v1/customer/groups/${groupId}/contacts/export${qs ? `?${qs}` : ''}`);
			const url = URL.createObjectURL(blob);
			const a = document.createElement('a');
			a.href = url;
			a.download = `${group?.name || 'group'}-contacts.csv`;
			a.click();
			URL.revokeObjectURL(url);
			toast.success('Export Complete', 'Downloaded group contacts CSV.');
		} catch (err: unknown) {
			const msg = err instanceof Error ? err.message : 'Failed to export contacts.';
			toast.error('Export Error', msg);
		} finally {
			exporting = false;
		}
	}

	// 6. Attach Existing Contacts
	async function loadAttachOptions() {
		attachLoading = true;
		try {
			const params = new URLSearchParams({ exclude_group_id: String(groupId), per_page: '50' });
			if (attachSearch.trim()) params.set('search', attachSearch.trim());
			const res = await api.get<{ data: Contact[] }>(`/api/v1/customer/contacts?${params}`);
			attachOptions = res.data || [];
		} catch {
			attachOptions = [];
		} finally {
			attachLoading = false;
		}
	}

	function handleAttachSearchInput() {
		if (attachSearchTimeout) clearTimeout(attachSearchTimeout);
		attachSearchTimeout = setTimeout(() => loadAttachOptions(), 250);
	}

	function openAttachModal() {
		attachSearch = '';
		attachSelectedIds = [];
		attachModalOpen = true;
		loadAttachOptions();
	}

	function toggleAttachSelect(id: number) {
		if (attachSelectedIds.includes(id)) {
			attachSelectedIds = attachSelectedIds.filter((item) => item !== id);
		} else {
			attachSelectedIds = [...attachSelectedIds, id];
		}
	}

	async function handleAttachExisting() {
		if (attachSelectedIds.length === 0) {
			toast.error('Selection Required', 'Select at least one contact to attach.');
			return;
		}
		attaching = true;
		try {
			const res = await api.post<{ message: string }>(`/api/v1/customer/groups/${groupId}/contacts`, {
				contact_ids: attachSelectedIds
			});
			toast.success('Contacts Attached', res.message || `Attached ${attachSelectedIds.length} contact(s).`);
			attachModalOpen = false;
			attachSelectedIds = [];
			loadContacts();
			loadGroup();
		} catch (err: unknown) {
			const msg = err instanceof Error ? err.message : 'Failed to attach contacts.';
			toast.error('Attach Error', msg);
		} finally {
			attaching = false;
		}
	}

	// 7. Bulk Remove from Group
	function toggleSelectAllContacts() {
		if (filteredContacts.length > 0 && selectedContactIds.length === filteredContacts.length) {
			selectedContactIds = [];
		} else {
			selectedContactIds = filteredContacts.map((c) => c.id);
		}
	}

	function toggleSelectContact(id: number) {
		if (selectedContactIds.includes(id)) {
			selectedContactIds = selectedContactIds.filter((item) => item !== id);
		} else {
			selectedContactIds = [...selectedContactIds, id];
		}
	}

	async function handleBulkDetach() {
		if (selectedContactIds.length === 0) return;
		bulkDetaching = true;
		try {
			const res = await api.post<{ message: string }>(`/api/v1/customer/groups/${groupId}/contacts/bulk-detach`, {
				contact_ids: selectedContactIds
			});
			toast.success('Contacts Removed', res.message || 'Selected contacts removed from group.');
			bulkDetachModalOpen = false;
			selectedContactIds = [];
			loadContacts();
			loadGroup();
		} catch (err: unknown) {
			const msg = err instanceof Error ? err.message : 'Failed to remove selected contacts.';
			toast.error('Remove Error', msg);
		} finally {
			bulkDetaching = false;
		}
	}

	let filteredContacts = $derived.by(() => {
		if (!contactSearch) return contacts;
		const q = contactSearch.toLowerCase();
		return contacts.filter(
			(c) =>
				c.name?.toLowerCase().includes(q) ||
				c.phone?.includes(q)
		);
	});
</script>

<svelte:head>
	<title>Edit Group - White INFO</title>
</svelte:head>

<div class="space-y-6 max-w-5xl">
	<!-- Breadcrumb -->
	<nav class="flex items-center gap-2 text-xs font-medium text-gray-500">
		<a href="/audience/groups" class="hover:text-gray-700">Groups</a>
		<svg class="h-3 w-3 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
			<path stroke-linecap="round" stroke-linejoin="round" d="m8.25 4.5 7.5 7.5-7.5 7.5" />
		</svg>
		<span class="text-gray-700">{group?.name || 'Group'}</span>
		<svg class="h-3 w-3 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
			<path stroke-linecap="round" stroke-linejoin="round" d="m8.25 4.5 7.5 7.5-7.5 7.5" />
		</svg>
		<span class="text-gray-900 font-semibold">Edit</span>
	</nav>

	<!-- Header with Top Actions: Import Contacts, View, Delete -->
	<div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
		<h1 class="text-2xl font-bold tracking-tight text-gray-950">
			Edit {group?.name || 'Group'}
		</h1>

		<div class="flex items-center gap-2">
			<!-- Import Contacts button -->
			<button
				type="button"
				onclick={() => (importModalOpen = true)}
				class="inline-flex items-center gap-1.5 rounded-lg border border-gray-300 bg-white px-3.5 py-1.5 text-xs font-semibold text-gray-700 shadow-2xs hover:bg-gray-50 cursor-pointer"
			>
				<svg class="h-4 w-4 text-gray-500" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5m-13.5-9L12 3m0 0 4.5 4.5M12 3v13.5" />
				</svg>
				Import Contacts
			</button>

			<!-- View (blue) -->
			<button
				type="button"
				onclick={() => (viewGroupModalOpen = true)}
				class="inline-flex items-center gap-1.5 rounded-lg bg-blue-600 px-3.5 py-1.5 text-xs font-semibold text-white shadow-2xs hover:bg-blue-700 cursor-pointer"
			>
				View
			</button>

			<!-- Delete (red) -->
			<button
				type="button"
				onclick={() => (deleteGroupModalOpen = true)}
				class="inline-flex items-center gap-1.5 rounded-lg bg-red-600 px-3.5 py-1.5 text-xs font-semibold text-white shadow-2xs hover:bg-red-700 cursor-pointer"
			>
				Delete
			</button>
		</div>
	</div>

	<!-- Card 1: Group Details Form -->
	<div class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm">
		<h2 class="text-base font-bold text-gray-900 border-b border-gray-100 pb-3">Group Details</h2>
		<form onsubmit={(e) => { e.preventDefault(); handleSaveGroup(); }} class="mt-4 space-y-4">
			<div>
				<label for="edit-group-name" class="block text-xs font-semibold text-gray-700">Group Name<span class="text-red-500">*</span></label>
				<input
					id="edit-group-name"
					type="text"
					bind:value={groupName}
					required
					class="mt-1.5 w-full rounded-lg border border-gray-300 bg-white py-2 px-3 text-xs text-gray-900 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600"
				/>
			</div>

			<div>
				<label for="edit-group-desc" class="block text-xs font-semibold text-gray-700">Description</label>
				<textarea
					id="edit-group-desc"
					bind:value={groupDesc}
					rows={3}
					class="mt-1.5 w-full rounded-lg border border-gray-300 bg-white py-2 px-3 text-xs text-gray-900 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600"
				></textarea>
			</div>

			<div class="flex items-center gap-3 pt-2">
				<button
					type="submit"
					disabled={savingGroup}
					class="rounded-lg bg-teal-600 px-4 py-2 text-xs font-semibold text-white shadow-sm hover:bg-teal-700 transition disabled:opacity-50 cursor-pointer"
				>
					{savingGroup ? 'Saving...' : 'Save changes'}
				</button>
				<a
					href="/audience/groups"
					class="text-xs font-semibold text-gray-600 hover:text-gray-900 px-3 py-2"
				>
					Cancel
				</a>
			</div>
		</form>
	</div>

	<!-- Card 2: Contacts Relation Manager -->
	<div class="rounded-xl border border-gray-200 bg-white shadow-sm overflow-hidden">
		<!-- Card Header with 5 Action Buttons -->
		<div class="flex flex-col sm:flex-row sm:items-center sm:justify-between p-4 border-b border-gray-200 gap-3 bg-white">
			<h2 class="text-base font-bold text-gray-900">Contacts</h2>

			<div class="flex flex-wrap items-center gap-2">
				<!-- 1. Add Phone Numbers -->
				<button
					type="button"
					onclick={() => { rawNumbers = ''; newContactName = ''; addNumbersModalOpen = true; }}
					class="inline-flex items-center gap-1.5 rounded-lg border border-teal-600 bg-white px-3 py-1.5 text-xs font-semibold text-teal-600 shadow-2xs hover:bg-teal-50 cursor-pointer"
				>
					<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M12 9v6m3-3H9m12 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
					</svg>
					Add Phone Numbers
				</button>

				<!-- 2. Import Excel / CSV -->
				<button
					type="button"
					onclick={() => { importFile = null; importFileName = ''; importModalOpen = true; }}
					class="inline-flex items-center gap-1.5 rounded-lg border border-gray-300 bg-white px-3 py-1.5 text-xs font-semibold text-gray-700 shadow-2xs hover:bg-gray-50 cursor-pointer"
				>
					<svg class="h-3.5 w-3.5 text-gray-500" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5m-13.5-9L12 3m0 0 4.5 4.5M12 3v13.5" />
					</svg>
					Import Excel / CSV
				</button>

				<!-- 3. Create Contact -->
				<button
					type="button"
					onclick={() => { singleName = ''; singlePhone = ''; createContactModalOpen = true; }}
					class="inline-flex items-center gap-1.5 rounded-lg border border-gray-300 bg-white px-3 py-1.5 text-xs font-semibold text-gray-700 shadow-2xs hover:bg-gray-50 cursor-pointer"
				>
					<svg class="h-3.5 w-3.5 text-gray-500" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
					</svg>
					Create Contact
				</button>

				<!-- 4. Attach Existing -->
				<button
					type="button"
					onclick={openAttachModal}
					class="inline-flex items-center gap-1.5 rounded-lg border border-gray-300 bg-white px-3 py-1.5 text-xs font-semibold text-gray-700 shadow-2xs hover:bg-gray-50 cursor-pointer"
				>
					<svg class="h-3.5 w-3.5 text-gray-500" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M13.19 8.688a4.5 4.5 0 0 1 1.242 7.244l-4.5 4.5a4.5 4.5 0 0 1-6.364-6.364l1.757-1.757m13.35-.622 1.757-1.757a4.5 4.5 0 0 0-6.364-6.364l-4.5 4.5a4.5 4.5 0 0 0 1.242 7.244" />
					</svg>
					Attach Existing
				</button>

				<!-- 5. Export Contacts -->
				<button
					type="button"
					onclick={handleExportContacts}
					disabled={exporting}
					class="inline-flex items-center gap-1.5 rounded-lg border border-gray-300 bg-white px-3 py-1.5 text-xs font-semibold text-gray-700 shadow-2xs hover:bg-gray-50 disabled:opacity-50 cursor-pointer"
				>
					<svg class="h-3.5 w-3.5 text-gray-500" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5M16.5 12 12 16.5m0 0L7.5 12m4.5 4.5V3" />
					</svg>
					{exporting ? 'Exporting...' : 'Export Contacts'}
				</button>
			</div>
		</div>

		<!-- Table Search & Bulk Actions -->
		<div class="flex flex-wrap items-center justify-between gap-2 p-3 border-b border-gray-100 bg-white">
			<div>
				{#if selectedContactIds.length > 0}
					<button
						type="button"
						onclick={() => (bulkDetachModalOpen = true)}
						class="inline-flex items-center gap-1.5 rounded-lg border border-red-200 bg-red-50 px-3 py-1.5 text-xs font-semibold text-red-700 hover:bg-red-100 cursor-pointer"
					>
						Remove from Group ({selectedContactIds.length})
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
					bind:value={contactSearch}
					placeholder="Search contacts"
					class="w-full rounded-lg border border-gray-300 bg-white py-1.5 pr-3 pl-8 text-xs text-gray-900 placeholder-gray-400 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600"
				/>
			</div>
		</div>

		<!-- Contacts Table -->
		<div class="overflow-x-auto">
			<table class="w-full text-left border-collapse text-xs">
				<thead>
					<tr class="border-b border-gray-200 bg-gray-50/70 text-gray-600 font-semibold tracking-wider uppercase text-[11px]">
						<th class="w-10 py-3 px-4">
							<input
								type="checkbox"
								checked={filteredContacts.length > 0 && selectedContactIds.length === filteredContacts.length}
								onchange={toggleSelectAllContacts}
								class="rounded border-gray-300 text-teal-600 focus:ring-teal-500 cursor-pointer"
							/>
						</th>
						<th class="py-3 px-4">NAME</th>
						<th class="py-3 px-4">PHONE</th>
						<th class="py-3 px-4">ADDED AT</th>
						<th class="py-3 px-4 text-right"></th>
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-100 bg-white">
					{#if loadingContacts}
						<tr>
							<td colspan="5" class="py-12 text-center text-gray-500">
								<div class="inline-block h-6 w-6 animate-spin rounded-full border-2 border-gray-300 border-t-teal-600"></div>
								<p class="mt-2 text-xs">Loading contacts...</p>
							</td>
						</tr>
					{:else if filteredContacts.length === 0}
						<tr>
							<td colspan="5" class="py-12 text-center text-gray-500">
								<p class="text-sm font-medium text-gray-700">No contacts in this group</p>
								<p class="mt-1 text-xs text-gray-400">Use "Add Phone Numbers" or "Import Excel / CSV" above to populate this group.</p>
							</td>
						</tr>
					{:else}
						{#each filteredContacts as contact (contact.id)}
							<tr class="hover:bg-gray-50/70 transition">
								<td class="py-3 px-4">
									<input
										type="checkbox"
										checked={selectedContactIds.includes(contact.id)}
										onchange={() => toggleSelectContact(contact.id)}
										class="rounded border-gray-300 text-teal-600 focus:ring-teal-500 cursor-pointer"
									/>
								</td>
								<td class="py-3 px-4 font-medium text-gray-900 whitespace-nowrap">
									{contact.name || '—'}
								</td>
								<td class="py-3 px-4 font-mono font-medium text-gray-800 whitespace-nowrap">
									{contact.phone}
								</td>
								<td class="py-3 px-4 text-gray-600 whitespace-nowrap">
									{new Date(contact.created_at).toLocaleDateString()}
								</td>
								<td class="py-3 px-4 text-right whitespace-nowrap">
									<button
										type="button"
										onclick={() => handleDetachContact(contact.id)}
										class="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-red-600 cursor-pointer"
										title="Remove from group"
									>
										<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
											<path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" />
										</svg>
									</button>
								</td>
							</tr>
						{/each}
					{/if}
				</tbody>
			</table>
		</div>
	</div>
</div>

<!-- Add Phone Numbers Modal -->
{#if addNumbersModalOpen}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 animate-fade-in backdrop-blur-xs">
		<div class="relative w-full max-w-lg rounded-2xl bg-white p-6 shadow-xl border border-gray-100">
			<h3 class="text-base font-bold text-gray-900">Add Phone Numbers to Group</h3>
			<p class="mt-1 text-xs text-gray-500">
				Type or paste phone numbers to add them to this group. You can enter single or multiple numbers separated by newlines, commas, or spaces.
			</p>

			<form onsubmit={(e) => { e.preventDefault(); handleAddNumbers(); }} class="mt-4 space-y-4">
				<div>
					<label for="add-numbers-textarea" class="block text-xs font-semibold text-gray-700">Phone Numbers<span class="text-red-500">*</span></label>
					<textarea
						id="add-numbers-textarea"
						bind:value={rawNumbers}
						rows={6}
						required
						placeholder="01712345678&#10;01812345678&#10;+8801912345678"
						class="mt-1.5 w-full font-mono rounded-lg border border-gray-300 bg-white py-2 px-3 text-xs text-gray-900 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600"
					></textarea>
					<p class="mt-1 text-[11px] text-gray-500">
						Accepts 11-digit Bangladeshi numbers (e.g. 017XXXXXXXX, +8801XXXXXXXX). Separated by lines, commas, or spaces.
					</p>
				</div>

				<div>
					<label for="add-numbers-name" class="block text-xs font-semibold text-gray-700">Contact Name (Optional)</label>
					<input
						id="add-numbers-name"
						type="text"
						bind:value={newContactName}
						placeholder="Optional name for newly created contact(s)"
						class="mt-1.5 w-full rounded-lg border border-gray-300 bg-white py-2 px-3 text-xs text-gray-900 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600"
					/>
				</div>

				<div class="mt-6 flex items-center justify-end gap-3 pt-2">
					<button
						type="button"
						onclick={() => (addNumbersModalOpen = false)}
						class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-xs font-semibold text-gray-700 hover:bg-gray-50 cursor-pointer"
					>
						Cancel
					</button>
					<button
						type="submit"
						disabled={addingNumbers}
						class="rounded-lg bg-teal-600 px-4 py-2 text-xs font-semibold text-white shadow-sm hover:bg-teal-700 transition disabled:opacity-50 cursor-pointer"
					>
						{addingNumbers ? 'Adding...' : 'Add Numbers'}
					</button>
				</div>
			</form>
		</div>
	</div>
{/if}

<!-- Import Excel / CSV Modal -->
{#if importModalOpen}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 animate-fade-in backdrop-blur-xs">
		<div class="relative w-full max-w-lg rounded-2xl bg-white p-6 shadow-xl border border-gray-100">
			<div class="flex items-center justify-between pb-3 border-b border-gray-100">
				<h3 class="text-base font-bold text-gray-900">Import Contacts</h3>
				<button
					type="button"
					onclick={() => (importModalOpen = false)}
					class="text-gray-400 hover:text-gray-600 cursor-pointer"
				>
					<svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" />
					</svg>
				</button>
			</div>

			<form onsubmit={(e) => { e.preventDefault(); handleImportSubmit(); }} class="mt-4 space-y-4">
				<div>
					<label class="flex flex-col items-center justify-center rounded-xl border-2 border-dashed border-gray-300 p-8 text-center hover:border-teal-500 hover:bg-teal-50/20 transition cursor-pointer">
						<input
							type="file"
							accept=".csv,.xlsx,.xls,.txt"
							onchange={handleFileSelected}
							class="hidden"
						/>
						<svg class="h-10 w-10 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 0 0-3.375-3.375h-1.5A1.125 1.125 0 0 1 13.5 7.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H8.25m6.75 12-3-3m0 0-3 3m3-3v6m-1.5-15H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 0 0-9-9Z" />
						</svg>
						<span class="mt-2 text-xs font-semibold text-gray-700">
							{importFileName ? importFileName : 'Drag and drop your files or browse'}
						</span>
						<span class="mt-1 text-[11px] text-gray-500">
							Upload a CSV or Excel file containing phone numbers.
						</span>
					</label>
				</div>

				<div class="flex items-center justify-between text-xs pt-1">
					<button
						type="button"
						onclick={() => {
							const sample = 'name,phone\nRahim Ahmed,01712345678\nKarim Hossain,01812345678\n';
							const blob = new Blob([sample], { type: 'text/csv' });
							const url = URL.createObjectURL(blob);
							const a = document.createElement('a');
							a.href = url;
							a.download = 'example-contacts.csv';
							a.click();
						}}
						class="text-teal-600 hover:text-teal-700 hover:underline font-medium cursor-pointer"
					>
						Download example CSV file
					</button>
				</div>

				<div class="mt-6 flex items-center justify-end gap-3 pt-2">
					<button
						type="button"
						onclick={() => (importModalOpen = false)}
						class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-xs font-semibold text-gray-700 hover:bg-gray-50 cursor-pointer"
					>
						Cancel
					</button>
					<button
						type="submit"
						disabled={importing}
						class="rounded-lg bg-teal-600 px-4 py-2 text-xs font-semibold text-white shadow-sm hover:bg-teal-700 transition disabled:opacity-50 cursor-pointer"
					>
						{importing ? 'Importing...' : 'Import'}
					</button>
				</div>
			</form>
		</div>
	</div>
{/if}

<!-- Create Contact Modal -->
{#if createContactModalOpen}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 animate-fade-in backdrop-blur-xs">
		<div class="relative w-full max-w-md rounded-2xl bg-white p-6 shadow-xl border border-gray-100">
			<h3 class="text-base font-bold text-gray-900">Create Contact</h3>
			<form onsubmit={(e) => { e.preventDefault(); handleCreateSingleContact(); }} class="mt-4 space-y-4">
				<div>
					<label for="create-contact-name" class="block text-xs font-semibold text-gray-700">Name</label>
					<input
						id="create-contact-name"
						type="text"
						bind:value={singleName}
						placeholder="Contact name"
						class="mt-1.5 w-full rounded-lg border border-gray-300 bg-white py-2 px-3 text-xs text-gray-900 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600"
					/>
				</div>
				<div>
					<label for="create-contact-phone" class="block text-xs font-semibold text-gray-700">Phone<span class="text-red-500">*</span></label>
					<input
						id="create-contact-phone"
						type="text"
						bind:value={singlePhone}
						required
						placeholder="e.g. 01712345678"
						class="mt-1.5 w-full font-mono rounded-lg border border-gray-300 bg-white py-2 px-3 text-xs text-gray-900 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600"
					/>
				</div>
				<div class="mt-6 flex items-center justify-end gap-3 pt-2">
					<button
						type="button"
						onclick={() => (createContactModalOpen = false)}
						class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-xs font-semibold text-gray-700 hover:bg-gray-50 cursor-pointer"
					>
						Cancel
					</button>
					<button
						type="submit"
						disabled={creatingContact}
						class="rounded-lg bg-teal-600 px-4 py-2 text-xs font-semibold text-white shadow-sm hover:bg-teal-700 transition disabled:opacity-50 cursor-pointer"
					>
						{creatingContact ? 'Creating...' : 'Create Contact'}
					</button>
				</div>
			</form>
		</div>
	</div>
{/if}

<!-- Attach Existing Contacts Modal -->
{#if attachModalOpen}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 animate-fade-in backdrop-blur-xs">
		<div class="relative w-full max-w-lg rounded-2xl bg-white p-6 shadow-xl border border-gray-100">
			<div class="flex items-center justify-between pb-3 border-b border-gray-100">
				<h3 class="text-base font-bold text-gray-900">Attach Existing Contacts</h3>
				<button
					type="button"
					onclick={() => (attachModalOpen = false)}
					class="text-gray-400 hover:text-gray-600 cursor-pointer"
				>
					<svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" />
					</svg>
				</button>
			</div>

			<div class="mt-4 space-y-3">
				<input
					type="text"
					bind:value={attachSearch}
					oninput={handleAttachSearchInput}
					placeholder="Search contacts by name or phone..."
					class="w-full rounded-lg border border-gray-300 bg-white py-2 px-3 text-xs text-gray-900 placeholder-gray-400 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600"
				/>

				<div class="max-h-64 overflow-y-auto rounded-lg border border-gray-200 divide-y divide-gray-100">
					{#if attachLoading}
						<div class="py-8 text-center text-xs text-gray-500">Searching contacts...</div>
					{:else if attachOptions.length === 0}
						<div class="py-8 text-center text-xs text-gray-500">
							No unattached contacts found{attachSearch.trim() ? ' matching your search' : ''}.
						</div>
					{:else}
						{#each attachOptions as opt (opt.id)}
							<label class="flex items-center justify-between px-3 py-2.5 hover:bg-gray-50 cursor-pointer text-xs">
								<div class="flex items-center gap-2.5">
									<input
										type="checkbox"
										checked={attachSelectedIds.includes(opt.id)}
										onchange={() => toggleAttachSelect(opt.id)}
										class="rounded border-gray-300 text-teal-600 focus:ring-teal-500 cursor-pointer"
									/>
									<div>
										<p class="font-semibold text-gray-900">{opt.name || 'Unnamed Contact'}</p>
										<p class="font-mono text-[11px] text-gray-500">{opt.phone}</p>
									</div>
								</div>
							</label>
						{/each}
					{/if}
				</div>
			</div>

			<div class="mt-6 flex items-center justify-between gap-3 pt-2">
				<span class="text-xs text-gray-500">{attachSelectedIds.length} selected</span>
				<div class="flex items-center gap-2">
					<button
						type="button"
						onclick={() => (attachModalOpen = false)}
						class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-xs font-semibold text-gray-700 hover:bg-gray-50 cursor-pointer"
					>
						Cancel
					</button>
					<button
						type="button"
						disabled={attaching || attachSelectedIds.length === 0}
						onclick={handleAttachExisting}
						class="rounded-lg bg-teal-600 px-4 py-2 text-xs font-semibold text-white shadow-sm hover:bg-teal-700 transition disabled:opacity-50 cursor-pointer"
					>
						{attaching ? 'Attaching...' : 'Attach Selected'}
					</button>
				</div>
			</div>
		</div>
	</div>
{/if}

<!-- View Group Details Modal -->
{#if viewGroupModalOpen && group}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 animate-fade-in backdrop-blur-xs">
		<div class="relative w-full max-w-md rounded-2xl bg-white p-6 shadow-xl border border-gray-100">
			<div class="flex items-center justify-between border-b border-gray-100 pb-3">
				<h3 class="text-base font-bold text-gray-900">Group Details</h3>
				<button
					type="button"
					onclick={() => (viewGroupModalOpen = false)}
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
					<p class="mt-1 font-bold text-gray-900">{group.name}</p>
				</div>
				<div>
					<p class="text-[11px] font-medium uppercase tracking-wider text-gray-400">Contacts</p>
					<p class="mt-1 font-semibold text-teal-700">{contacts.length}</p>
				</div>
				<div class="col-span-2">
					<p class="text-[11px] font-medium uppercase tracking-wider text-gray-400">Description</p>
					<p class="mt-1 text-gray-700">{group.description || '—'}</p>
				</div>
				<div class="col-span-2">
					<p class="text-[11px] font-medium uppercase tracking-wider text-gray-400">Created At</p>
					<p class="mt-1 text-gray-700">{new Date(group.created_at).toLocaleString()}</p>
				</div>
			</div>
			<div class="mt-6 flex items-center justify-end border-t border-gray-100 pt-3">
				<button
					type="button"
					onclick={() => (viewGroupModalOpen = false)}
					class="rounded-lg border border-gray-300 bg-white px-3.5 py-1.5 text-xs font-semibold text-gray-700 hover:bg-gray-50 cursor-pointer"
				>
					Close
				</button>
			</div>
		</div>
	</div>
{/if}

<!-- Bulk Remove from Group Confirmation Modal -->
{#if bulkDetachModalOpen}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 animate-fade-in backdrop-blur-xs">
		<div class="relative w-full max-w-sm rounded-2xl bg-white p-6 shadow-xl border border-gray-100">
			<h3 class="text-base font-bold text-gray-900">Remove Selected from Group</h3>
			<p class="mt-2 text-xs text-gray-600">
				Remove <span class="font-semibold text-gray-900">{selectedContactIds.length}</span> selected contact{selectedContactIds.length === 1 ? '' : 's'} from "{group?.name}"? Contacts remain in your directory.
			</p>
			<div class="mt-6 flex items-center justify-end gap-3">
				<button
					type="button"
					onclick={() => (bulkDetachModalOpen = false)}
					class="rounded-lg border border-gray-300 bg-white px-3.5 py-1.5 text-xs font-semibold text-gray-700 hover:bg-gray-50 cursor-pointer"
				>
					Cancel
				</button>
				<button
					type="button"
					disabled={bulkDetaching}
					onclick={handleBulkDetach}
					class="rounded-lg bg-red-600 px-3.5 py-1.5 text-xs font-semibold text-white shadow-xs hover:bg-red-700 disabled:opacity-50 cursor-pointer"
				>
					{bulkDetaching ? 'Removing...' : 'Remove'}
				</button>
			</div>
		</div>
	</div>
{/if}

<!-- Delete Confirmation Modal -->
{#if deleteGroupModalOpen}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 animate-fade-in backdrop-blur-xs">
		<div class="relative w-full max-w-sm rounded-2xl bg-white p-6 shadow-xl border border-gray-100">
			<h3 class="text-base font-bold text-gray-900">Delete Group</h3>
			<p class="mt-2 text-xs text-gray-600">
				Are you sure you want to delete <span class="font-semibold text-gray-900">"{group?.name}"</span>? This action is permanent and cannot be undone.
			</p>
			<div class="mt-6 flex items-center justify-end gap-3">
				<button
					type="button"
					onclick={() => (deleteGroupModalOpen = false)}
					class="rounded-lg border border-gray-300 bg-white px-3.5 py-1.5 text-xs font-semibold text-gray-700 hover:bg-gray-50 cursor-pointer"
				>
					Cancel
				</button>
				<button
					type="button"
					onclick={handleDeleteGroup}
					class="rounded-lg bg-red-600 px-3.5 py-1.5 text-xs font-semibold text-white shadow-xs hover:bg-red-700 cursor-pointer"
				>
					Delete Permanently
				</button>
			</div>
		</div>
	</div>
{/if}

