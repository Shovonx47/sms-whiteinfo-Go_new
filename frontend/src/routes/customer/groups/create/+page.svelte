<script lang="ts">
	import { api } from '$lib/api.js';
	import { toast } from '$lib/stores/toast.svelte.js';
	import { goto } from '$app/navigation';

	let name = $state('');
	let description = $state('');
	let phoneNumbers = $state('');
	let submitting = $state(false);

	let contactsSectionOpen = $state(true);

	// File upload parsing
	let importFile = $state<File | null>(null);
	let fileName = $state('');

	function handleFileUpload(e: Event) {
		const target = e.target as HTMLInputElement;
		const file = target.files?.[0];
		if (!file) return;
		importFile = file;
		fileName = file.name;
	}

	async function handleSubmit(createAnother = false) {
		if (!name.trim()) {
			toast.error('Validation error', 'Group name is required.');
			return;
		}

		submitting = true;
		try {
			interface CreateGroupResp {
				message?: string;
				valid_count?: number;
				invalid_count?: number;
				import_error?: string;
			}
			let res: CreateGroupResp;
			if (importFile) {
				const fd = new FormData();
				fd.append('name', name.trim());
				fd.append('description', description.trim());
				fd.append('_phone_numbers', phoneNumbers.trim());
				fd.append('file', importFile);
				res = await api.upload<CreateGroupResp>('/api/v1/customer/groups', fd);
			} else {
				res = await api.post<CreateGroupResp>('/api/v1/customer/groups', {
					name: name.trim(),
					description: description.trim(),
					_phone_numbers: phoneNumbers.trim()
				});
			}

			toast.success('Group Created', `Group "${name.trim()}" has been created.`);
			if (res.import_error) {
				toast.error('Import Error', res.import_error);
			}
			if ((res.valid_count ?? 0) > 0) {
				const n = res.valid_count ?? 0;
				toast.success('Contacts Added', `Successfully added ${n} contact${n === 1 ? '' : 's'} to group "${name.trim()}".`);
			}
			if ((res.invalid_count ?? 0) > 0) {
				const inv = res.invalid_count ?? 0;
				toast.warning('Some Entries Skipped', `${inv} invalid or unreadable ${inv === 1 ? 'entry' : 'entries'} could not be processed.`);
			}

			if (createAnother) {
				name = '';
				description = '';
				phoneNumbers = '';
				importFile = null;
				fileName = '';
			} else {
				goto('/audience/groups');
			}
		} catch (err: unknown) {
			const msg = err instanceof Error ? err.message : 'Failed to create group.';
			toast.error('Create Error', msg);
		} finally {
			submitting = false;
		}
	}
</script>

<svelte:head>
	<title>Create Group - White INFO</title>
</svelte:head>

<div class="space-y-6 max-w-4xl">
	<!-- Breadcrumb -->
	<nav class="flex items-center gap-2 text-xs font-medium text-gray-500">
		<a href="/audience/groups" class="hover:text-gray-700">Groups</a>
		<svg class="h-3 w-3 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
			<path stroke-linecap="round" stroke-linejoin="round" d="m8.25 4.5 7.5 7.5-7.5 7.5" />
		</svg>
		<span class="text-gray-900 font-semibold">Create</span>
	</nav>

	<h1 class="text-2xl font-bold tracking-tight text-gray-950">Create Group</h1>

	<form onsubmit={(e) => { e.preventDefault(); handleSubmit(false); }} class="space-y-6">
		<!-- Section 1: Group Details -->
		<div class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm">
			<h2 class="text-base font-bold text-gray-900 border-b border-gray-100 pb-3">Group Details</h2>
			<div class="mt-4 space-y-4">
				<div>
					<label for="group-name" class="block text-xs font-semibold text-gray-700">Group Name<span class="text-red-500">*</span></label>
					<input
						id="group-name"
						type="text"
						bind:value={name}
						placeholder="Enter group name"
						required
						class="mt-1.5 w-full rounded-lg border border-gray-300 bg-white py-2 px-3 text-xs text-gray-900 placeholder-gray-400 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600"
					/>
				</div>

				<div>
					<label for="group-desc" class="block text-xs font-semibold text-gray-700">Description</label>
					<textarea
						id="group-desc"
						bind:value={description}
						placeholder="Enter group description"
						rows={3}
						class="mt-1.5 w-full rounded-lg border border-gray-300 bg-white py-2 px-3 text-xs text-gray-900 placeholder-gray-400 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600"
					></textarea>
				</div>
			</div>
		</div>

		<!-- Section 2: Add Contacts (Optional) -->
		<div class="rounded-xl border border-gray-200 bg-white shadow-sm overflow-hidden">
			<button
				type="button"
				onclick={() => (contactsSectionOpen = !contactsSectionOpen)}
				class="w-full flex items-center justify-between p-6 text-left hover:bg-gray-50/50 transition cursor-pointer"
			>
				<div>
					<h2 class="text-base font-bold text-gray-900">Add Contacts (Optional)</h2>
					<p class="mt-0.5 text-xs text-gray-500">
						You can add contacts right now while creating the group. You can also add or import more contacts later from the Edit page.
					</p>
				</div>
				<svg class="h-5 w-5 text-gray-400 transition-transform duration-200 {contactsSectionOpen ? 'rotate-180' : ''}" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" />
				</svg>
			</button>

			{#if contactsSectionOpen}
				<div class="px-6 pb-6 pt-2 border-t border-gray-100 space-y-5">
					<!-- Phone Numbers textarea -->
					<div>
						<div class="flex items-center justify-between">
							<label for="manual-numbers" class="block text-xs font-semibold text-gray-700">Phone Numbers</label>
							<span class="text-[11px] font-medium text-gray-400 flex items-center gap-1">
								<svg class="h-3.5 w-3.5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
									<path stroke-linecap="round" stroke-linejoin="round" d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L6.832 19.82a4.5 4.5 0 0 1-1.897 1.13l-2.685.8.8-2.685a4.5 4.5 0 0 1 1.13-1.897L16.863 4.487Zm0 0L19.5 7.125" />
								</svg>
								Manual entry
							</span>
						</div>
						<textarea
							id="manual-numbers"
							bind:value={phoneNumbers}
							rows={5}
							placeholder="01712345678&#10;01812345678, 01912345678&#10;+8801312345678"
							class="mt-1.5 w-full font-mono rounded-lg border border-gray-300 bg-white py-2 px-3 text-xs text-gray-900 placeholder-gray-400 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600"
						></textarea>
						<p class="mt-1 text-[11px] text-gray-500">
							Enter one or multiple numbers separated by newlines, commas, or spaces. Accepts 11-digit Bangladeshi numbers (017XXXXXXXX, +8801XXXXXXXX).
						</p>
					</div>

					<!-- Or Import File -->
					<div>
						<div class="flex items-center justify-between mb-1.5">
							<span class="block text-xs font-semibold text-gray-700">Or Import from Excel / CSV</span>
							<span class="text-[11px] font-medium text-gray-400 flex items-center gap-1">
								<svg class="h-3.5 w-3.5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
									<path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5M16.5 12 12 16.5m0 0L7.5 12m4.5 4.5V3" />
								</svg>
								Bulk import
							</span>
						</div>

						<label class="flex flex-col items-center justify-center rounded-xl border-2 border-dashed border-gray-300 p-6 text-center hover:border-teal-500 hover:bg-teal-50/20 transition cursor-pointer">
							<input
								type="file"
								accept=".csv,.xlsx,.xls,.txt"
								onchange={handleFileUpload}
								class="hidden"
							/>
							<svg class="h-8 w-8 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
								<path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 0 0-3.375-3.375h-1.5A1.125 1.125 0 0 1 13.5 7.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H8.25m6.75 12-3-3m0 0-3 3m3-3v6m-1.5-15H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 0 0-9-9Z" />
							</svg>
							<span class="mt-2 text-xs font-semibold text-gray-700">
								{fileName ? fileName : 'Drag and drop your files or browse'}
							</span>
							<span class="mt-1 text-[11px] text-gray-500">
								Supports .csv, .xlsx, and .xls files. Columns like Phone, Mobile, Contact, Number, Name are auto-detected.
							</span>
						</label>
					</div>
				</div>
			{/if}
		</div>

		<!-- Actions -->
		<div class="flex items-center gap-3">
			<button
				type="submit"
				disabled={submitting}
				class="rounded-lg bg-teal-600 px-4 py-2 text-xs font-semibold text-white shadow-sm hover:bg-teal-700 transition disabled:opacity-50 cursor-pointer"
			>
				{submitting ? 'Creating...' : 'Create'}
			</button>
			<button
				type="button"
				disabled={submitting}
				onclick={() => handleSubmit(true)}
				class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-xs font-semibold text-gray-700 hover:bg-gray-50 transition disabled:opacity-50 cursor-pointer"
			>
				Create & create another
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

