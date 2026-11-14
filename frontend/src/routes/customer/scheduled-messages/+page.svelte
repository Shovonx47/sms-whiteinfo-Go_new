<script lang="ts">
	import { api, ApiError } from '$lib/api.js';
	import { toast } from '$lib/components/Toast.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import { onMount, onDestroy } from 'svelte';

	interface ScheduledItem {
		id: number;
		sender_db_id: number | null;
		sender_id: string;
		recipient_preview: string;
		recipients_list: string;
		total_recipients: number;
		body: string;
		source_type: string;
		status: string;
		scheduled_at: string | null;
		created_at: string;
	}

	interface SenderOption {
		id: number;
		sender_id: string;
	}

	interface Meta {
		total: number;
		per_page: number;
		current_page: number;
		counts?: {
			all: number;
			scheduled: number;
			paused: number;
		};
	}

	let items = $state<ScheduledItem[]>([]);
	let senders = $state<SenderOption[]>([]);
	let meta = $state<Meta>({
		total: 0,
		per_page: 15,
		current_page: 1,
		counts: { all: 0, scheduled: 0, paused: 0 }
	});
	let loading = $state(true);
	let activeTab = $state<'all' | 'scheduled' | 'paused'>('all');
	let search = $state('');
	let sortKey = $state<'scheduled_at' | 'total_recipients' | 'created_at'>('scheduled_at');
	let sortDir = $state<'asc' | 'desc'>('asc');

	// Edit Modal state
	let editOpen = $state(false);
	let editingItem = $state<ScheduledItem | null>(null);
	let editSenderId = $state<number>(0);
	let editScheduledAt = $state('');
	let editRecipientsList = $state('');
	let editBody = $state('');
	let savingEdit = $state(false);

	// Delete Modal state
	let deleteOpen = $state(false);
	let deletingItem = $state<ScheduledItem | null>(null);
	let deleting = $state(false);
	let selectedIds = $state<number[]>([]);
	let bulkDeleting = $state(false);

	let pollTimer: ReturnType<typeof setInterval> | null = null;

	let allSelected = $derived(
		items.length > 0 && items.every((item) => selectedIds.includes(item.id))
	);

	function toggleSelectAll() {
		if (allSelected) {
			selectedIds = [];
		} else {
			selectedIds = items.map((item) => item.id);
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
			await api.post('/api/v1/customer/scheduled-messages/bulk-delete', { ids: selectedIds });
			toast.success('Scheduled Messages Deleted', `Deleted ${selectedIds.length} scheduled message(s).`);
			selectedIds = [];
			await load(1);
		} catch (e) {
			const msg = e instanceof ApiError ? (e.data.message as string) : 'Could not delete selected schedules.';
			toast.error('Bulk Delete Failed', msg);
		} finally {
			bulkDeleting = false;
		}
	}

	async function load(page = 1, silent = false) {
		if (!silent) loading = true;
		try {
			const params = new URLSearchParams({
				page: String(page),
				per_page: String(meta.per_page),
				sort: sortKey,
				dir: sortDir
			});
			if (activeTab !== 'all') params.set('status', activeTab);
			if (search.trim()) params.set('search', search.trim());

			const res = await api.get<{ data: ScheduledItem[]; meta: Meta }>(
				`/api/v1/customer/scheduled-messages?${params}`
			);
			items = res.data;
			meta = res.meta;
		} catch {
			if (!silent) items = [];
		} finally {
			loading = false;
		}
	}

	async function loadSenders() {
		try {
			const res = await api.get<{ data: SenderOption[] }>('/api/v1/customer/senders');
			senders = res.data ?? [];
		} catch {
			senders = [];
		}
	}

	onMount(() => {
		load(1);
		loadSenders();
		pollTimer = setInterval(() => load(meta.current_page, true), 10000);
	});

	onDestroy(() => {
		if (pollTimer) clearInterval(pollTimer);
	});

	function selectTab(tab: 'all' | 'scheduled' | 'paused') {
		activeTab = tab;
		load(1);
	}

	function toggleSort(col: 'scheduled_at' | 'total_recipients' | 'created_at') {
		if (sortKey === col) {
			sortDir = sortDir === 'asc' ? 'desc' : 'asc';
		} else {
			sortKey = col;
			sortDir = 'asc';
		}
		load(1);
	}

	function formatDateTime(iso: string | null): string {
		if (!iso) return '—';
		return new Date(iso).toLocaleString('en-GB', {
			day: '2-digit',
			month: 'short',
			year: 'numeric',
			hour: '2-digit',
			minute: '2-digit',
			hour12: true
		});
	}

	function formatRelativeTime(iso: string | null): string {
		if (!iso) return '';
		const target = new Date(iso).getTime();
		const diffSec = Math.round((target - Date.now()) / 1000);
		if (Math.abs(diffSec) < 60) return diffSec >= 0 ? 'in a few seconds' : 'just now';
		const absMin = Math.round(Math.abs(diffSec) / 60);
		if (absMin < 60) return diffSec >= 0 ? `in ${absMin} min` : `${absMin} min ago`;
		const absHr = Math.round(absMin / 60);
		if (absHr < 24) return diffSec >= 0 ? `in ${absHr} hr` : `${absHr} hr ago`;
		const absDays = Math.round(absHr / 24);
		return diffSec >= 0 ? `in ${absDays} days` : `${absDays} days ago`;
	}

	function toLocalInputValue(iso: string | null): string {
		const d = iso ? new Date(iso) : new Date(Date.now() + 15 * 60000);
		const pad = (n: number) => String(n).padStart(2, '0');
		return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
	}

	function openEdit(item: ScheduledItem) {
		editingItem = item;
		editSenderId = item.sender_db_id ?? (senders[0]?.id ?? 0);
		editScheduledAt = toLocalInputValue(item.scheduled_at);
		editRecipientsList = item.recipients_list ?? '';
		editBody = item.body ?? '';
		editOpen = true;
	}

	async function submitEdit() {
		if (!editingItem || savingEdit) return;
		if (!editBody.trim()) {
			toast.error('Empty message', 'Message content is required.');
			return;
		}
		if (!editScheduledAt) {
			toast.error('Invalid time', 'Scheduled date & time is required.');
			return;
		}
		savingEdit = true;
		try {
			await api.put(`/api/v1/customer/scheduled-messages/${editingItem.id}`, {
				sender_id: Number(editSenderId),
				scheduled_at: editScheduledAt,
				recipients_list: editRecipientsList,
				body: editBody.trim()
			});
			toast.success(
				'Scheduled Message Updated',
				'The schedule and message details have been successfully updated.'
			);
			editOpen = false;
			await load(meta.current_page);
		} catch (e) {
			const msg = e instanceof ApiError ? (e.data.message as string) : 'Unable to update schedule.';
			toast.error('Update failed', msg || 'Unable to update schedule.');
		} finally {
			savingEdit = false;
		}
	}

	async function handlePause(item: ScheduledItem) {
		try {
			await api.post(`/api/v1/customer/scheduled-messages/${item.id}/pause`, {});
			toast.warning('Scheduled Message Paused', 'The scheduled message has been paused.');
			await load(meta.current_page);
		} catch (e) {
			const msg = e instanceof ApiError ? (e.data.message as string) : 'Could not pause schedule.';
			toast.error('Pause failed', msg);
		}
	}

	async function handleResume(item: ScheduledItem) {
		try {
			await api.post(`/api/v1/customer/scheduled-messages/${item.id}/resume`, {});
			toast.success(
				'Schedule Resumed',
				'The message schedule is now active and will send at the scheduled time.'
			);
			await load(meta.current_page);
		} catch (e) {
			const msg = e instanceof ApiError ? (e.data.message as string) : 'Could not resume schedule.';
			toast.error('Resume failed', msg);
		}
	}

	function confirmDelete(item: ScheduledItem) {
		deletingItem = item;
		deleteOpen = true;
	}

	async function executeDelete() {
		if (!deletingItem || deleting) return;
		deleting = true;
		try {
			await api.delete(`/api/v1/customer/scheduled-messages/${deletingItem.id}`);
			toast.success('Scheduled Message Deleted', 'The scheduled message has been removed.');
			deleteOpen = false;
			deletingItem = null;
			await load(1);
		} catch (e) {
			const msg = e instanceof ApiError ? (e.data.message as string) : 'Could not delete schedule.';
			toast.error('Delete failed', msg);
		} finally {
			deleting = false;
		}
	}

	async function copyRecipient(text: string) {
		if (!text || text === '—') return;
		try {
			await navigator.clipboard.writeText(text);
			toast.success('Copied', 'Recipient copied to clipboard.');
		} catch {
			// Ignore
		}
	}
</script>

<svelte:head>
	<title>Scheduled Messages - White INFO</title>
</svelte:head>

<div class="space-y-5">
	<!-- Breadcrumb -->
	<nav class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
		<a href="/messaging/scheduled-messages" class="hover:text-gray-800 dark:hover:text-gray-200 transition">Scheduled Messages</a>
		<svg class="h-3 w-3 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
			<path stroke-linecap="round" stroke-linejoin="round" d="M8.25 4.5l7.5 7.5-7.5 7.5" />
		</svg>
		<span class="text-gray-900 dark:text-gray-100 font-medium">List</span>
	</nav>

	<!-- Header -->
	<div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
		<div>
			<h1 class="text-2xl font-bold tracking-tight text-gray-900 dark:text-gray-100">
				Scheduled Messages
			</h1>
		</div>
		<a
			href="/customer/messages/quick?schedule=1"
			class="inline-flex items-center gap-2 self-start rounded-lg bg-teal-600 px-4 py-2 text-sm font-semibold text-white shadow-xs transition hover:bg-teal-500 sm:self-auto"
		>
			<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
			</svg>
			Schedule New SMS
		</a>
	</div>

	<!-- Status Tabs + Search -->
	<div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
		<div class="inline-flex items-center rounded-xl border border-gray-200 bg-white p-1 shadow-2xs dark:border-gray-800 dark:bg-gray-900">
			<button
				type="button"
				onclick={() => selectTab('all')}
				class="inline-flex items-center gap-2 rounded-lg px-3.5 py-1.5 text-xs font-semibold transition {activeTab === 'all'
					? 'bg-teal-50 text-teal-700 dark:bg-teal-950/60 dark:text-teal-300'
					: 'text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white'}"
			>
				All Scheduled
				<span class="rounded-md bg-gray-100 px-1.5 py-0.5 text-[11px] font-bold text-gray-700 dark:bg-gray-800 dark:text-gray-300">
					{meta.counts?.all ?? 0}
				</span>
			</button>
			<button
				type="button"
				onclick={() => selectTab('scheduled')}
				class="inline-flex items-center gap-2 rounded-lg px-3.5 py-1.5 text-xs font-semibold transition {activeTab === 'scheduled'
					? 'bg-teal-50 text-teal-700 dark:bg-teal-950/60 dark:text-teal-300'
					: 'text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white'}"
			>
				Active Schedule
				<span class="rounded-md bg-sky-50 px-1.5 py-0.5 text-[11px] font-bold text-sky-700 dark:bg-sky-950/60 dark:text-sky-300">
					{meta.counts?.scheduled ?? 0}
				</span>
			</button>
			<button
				type="button"
				onclick={() => selectTab('paused')}
				class="inline-flex items-center gap-2 rounded-lg px-3.5 py-1.5 text-xs font-semibold transition {activeTab === 'paused'
					? 'bg-teal-50 text-teal-700 dark:bg-teal-950/60 dark:text-teal-300'
					: 'text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white'}"
			>
				Paused
				<span class="rounded-md bg-amber-50 px-1.5 py-0.5 text-[11px] font-bold text-amber-700 dark:bg-amber-950/60 dark:text-amber-300">
					{meta.counts?.paused ?? 0}
				</span>
			</button>
		</div>

		<div class="flex flex-wrap items-center gap-2">
			{#if selectedIds.length > 0}
				<button
					type="button"
					disabled={bulkDeleting}
					onclick={handleBulkDelete}
					class="inline-flex items-center gap-1.5 rounded-xl bg-rose-600 px-3 py-2 text-xs font-semibold text-white shadow-xs hover:bg-rose-500 disabled:opacity-50 cursor-pointer"
				>
					{bulkDeleting ? 'Deleting…' : `Delete Selected (${selectedIds.length})`}
				</button>
			{/if}
			<div class="relative w-full sm:w-72">
				<input
					type="text"
					bind:value={search}
					oninput={() => load(1)}
					placeholder="Search recipient, sender, or content..."
					class="w-full rounded-xl border border-gray-200 bg-white py-2 pl-9 pr-3 text-sm text-gray-900 placeholder-gray-400 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600 dark:border-gray-800 dark:bg-gray-900 dark:text-white"
				/>
				<svg
					class="pointer-events-none absolute left-3 top-2.5 h-4 w-4 text-gray-400"
					fill="none"
					viewBox="0 0 24 24"
					stroke-width="2"
					stroke="currentColor"
				>
					<path stroke-linecap="round" stroke-linejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
				</svg>
			</div>
		</div>
	</div>

	<!-- Table Card -->
	<div class="overflow-hidden rounded-2xl border border-gray-200/80 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
		<div class="overflow-x-auto">
			<table class="w-full text-left text-sm">
				<thead>
					<tr class="border-b border-gray-100 bg-gray-50/50 text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:border-gray-800 dark:bg-gray-800/40 dark:text-gray-400">
						<th class="w-10 px-4 py-3.5">
							<input
								type="checkbox"
								checked={allSelected}
								onchange={toggleSelectAll}
								class="rounded border-gray-300 text-teal-600 focus:ring-teal-500 cursor-pointer"
								aria-label="Select all scheduled messages"
							/>
						</th>
						<th class="px-6 py-3.5">SENDER ID</th>
						<th class="px-6 py-3.5">RECIPIENT NUMBER</th>
						<th class="px-6 py-3.5">SOURCE</th>
						<th class="px-6 py-3.5 text-right">
							<button
								type="button"
								onclick={() => toggleSort('total_recipients')}
								class="inline-flex items-center gap-1 font-bold uppercase tracking-wider hover:text-gray-900 dark:hover:text-white"
							>
								RECIPIENTS
							</button>
						</th>
						<th class="px-6 py-3.5">MESSAGE CONTENT</th>
						<th class="px-6 py-3.5">
							<button
								type="button"
								onclick={() => toggleSort('scheduled_at')}
								class="inline-flex items-center gap-1 font-bold uppercase tracking-wider hover:text-gray-900 dark:hover:text-white"
							>
								SCHEDULED FOR
								<svg
									class="h-3.5 w-3.5 transition-transform {sortKey === 'scheduled_at' && sortDir === 'desc' ? 'rotate-180' : ''}"
									fill="none"
									viewBox="0 0 24 24"
									stroke-width="2"
									stroke="currentColor"
								>
									<path stroke-linecap="round" stroke-linejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" />
								</svg>
							</button>
						</th>
						<th class="px-6 py-3.5">STATUS</th>
						<th class="px-6 py-3.5 text-right">ACTIONS</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-100 dark:divide-gray-800">
					{#if loading}
						{#each { length: 3 } as _, i (i)}
							<tr>
								<td colspan="9" class="px-6 py-4">
									<div class="h-6 w-full animate-pulse rounded bg-gray-100 dark:bg-gray-800"></div>
								</td>
							</tr>
						{/each}
					{:else if items.length === 0}
						<tr>
							<td colspan="9" class="px-6 py-16 text-center">
								<div class="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-gray-100 text-gray-400 dark:bg-gray-800">
									<svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
										<path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
									</svg>
								</div>
								<p class="mt-3.5 text-sm font-bold text-gray-900 dark:text-white">
									No Scheduled Messages
								</p>
								<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
									You do not have any scheduled messages in queue.
								</p>
								<div class="mt-4">
									<a
										href="/customer/messages/quick?schedule=1"
										class="inline-flex items-center gap-1.5 rounded-lg border border-teal-600/80 bg-white px-3.5 py-2 text-xs font-semibold text-teal-700 shadow-2xs transition hover:bg-teal-50 dark:border-teal-500/60 dark:bg-gray-900 dark:text-teal-400"
									>
										<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor">
											<path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
										</svg>
										Schedule New SMS
									</a>
								</div>
							</td>
						</tr>
					{:else}
						{#each items as item (item.id)}
							<tr class="transition hover:bg-gray-50/70 dark:hover:bg-gray-800/40">
								<td class="px-4 py-4">
									<input
										type="checkbox"
										checked={selectedIds.includes(item.id)}
										onchange={() => toggleSelect(item.id)}
										class="rounded border-gray-300 text-teal-600 focus:ring-teal-500 cursor-pointer"
										aria-label="Select scheduled message {item.id}"
									/>
								</td>
								<td class="whitespace-nowrap px-6 py-4">
									<span class="inline-flex items-center rounded-md bg-teal-50 px-2.5 py-1 text-xs font-semibold text-teal-700 ring-1 ring-inset ring-teal-600/20 dark:bg-teal-950/60 dark:text-teal-300">
										{item.sender_id}
									</span>
								</td>
								<td class="whitespace-nowrap px-6 py-4">
									<button
										type="button"
										onclick={() => copyRecipient(item.recipient_preview)}
										class="inline-flex items-center rounded-md bg-emerald-50 px-2.5 py-1 text-xs font-semibold text-emerald-700 ring-1 ring-inset ring-emerald-600/20 transition hover:bg-emerald-100 dark:bg-emerald-950/60 dark:text-emerald-300"
									>
										{item.recipient_preview}
									</button>
								</td>
								<td class="whitespace-nowrap px-6 py-4">
									<span class="inline-flex items-center rounded-md bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-700 dark:bg-gray-800 dark:text-gray-300">
										{item.source_type || 'manual'}
									</span>
								</td>
								<td class="whitespace-nowrap px-6 py-4 text-right font-semibold text-gray-900 dark:text-white">
									{item.total_recipients}
								</td>
								<td class="max-w-xs px-6 py-4 text-sm text-gray-800 dark:text-gray-200" title={item.body}>
									<span class="line-clamp-1">
										{item.body && item.body.length > 55 ? item.body.slice(0, 55) + '...' : item.body}
									</span>
								</td>
								<td class="whitespace-nowrap px-6 py-4">
									<div class="text-xs font-semibold text-gray-900 dark:text-white">
										{formatDateTime(item.scheduled_at)}
									</div>
									<div class="text-[11px] text-gray-500 dark:text-gray-400">
										{formatRelativeTime(item.scheduled_at)}
									</div>
								</td>
								<td class="whitespace-nowrap px-6 py-4">
									{#if item.status === 'paused'}
										<span class="inline-flex items-center rounded-md bg-amber-50 px-2.5 py-1 text-xs font-semibold text-amber-700 ring-1 ring-inset ring-amber-600/20 dark:bg-amber-950/60 dark:text-amber-300">
											Paused
										</span>
									{:else}
										<span class="inline-flex items-center rounded-md bg-violet-50 px-2.5 py-1 text-xs font-semibold text-violet-700 ring-1 ring-inset ring-violet-600/20 dark:bg-violet-950/60 dark:text-violet-300">
											Scheduled
										</span>
									{/if}
								</td>
								<td class="whitespace-nowrap px-6 py-4 text-right">
									<div class="inline-flex items-center justify-end gap-2">
										<button
											type="button"
											onclick={() => openEdit(item)}
											class="rounded-lg border border-gray-200 bg-white px-2.5 py-1 text-xs font-semibold text-teal-700 transition hover:bg-teal-50 dark:border-gray-700 dark:bg-gray-800 dark:text-teal-400"
										>
											Edit
										</button>
										{#if item.status === 'scheduled'}
											<button
												type="button"
												onclick={() => handlePause(item)}
												class="rounded-lg border border-amber-200 bg-amber-50 px-2.5 py-1 text-xs font-semibold text-amber-700 transition hover:bg-amber-100 dark:border-amber-800 dark:bg-amber-950/50 dark:text-amber-300"
											>
												Pause
											</button>
										{:else if item.status === 'paused'}
											<button
												type="button"
												onclick={() => handleResume(item)}
												class="rounded-lg border border-sky-200 bg-sky-50 px-2.5 py-1 text-xs font-semibold text-sky-700 transition hover:bg-sky-100 dark:border-sky-800 dark:bg-sky-950/50 dark:text-sky-300"
											>
												Resume
											</button>
										{/if}
										<button
											type="button"
											onclick={() => confirmDelete(item)}
											class="rounded-lg border border-rose-200 bg-rose-50 px-2.5 py-1 text-xs font-semibold text-rose-700 transition hover:bg-rose-100 dark:border-rose-800 dark:bg-rose-950/50 dark:text-rose-300"
										>
											Delete
										</button>
									</div>
								</td>
							</tr>
						{/each}
					{/if}
				</tbody>
			</table>
		</div>
	</div>
</div>

<!-- Edit Scheduled Message Modal -->
<Modal bind:open={editOpen} title="Edit Scheduled Message">
	<div class="space-y-4">
		<div>
			<label for="sched-time" class="block text-xs font-semibold text-gray-700 dark:text-gray-300">
				Scheduled Date &amp; Time
			</label>
			<input
				id="sched-time"
				type="datetime-local"
				bind:value={editScheduledAt}
				class="mt-1 w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 focus:border-teal-600 focus:outline-none dark:border-gray-700 dark:bg-gray-800 dark:text-white"
			/>
		</div>

		<div>
			<label for="sched-sender" class="block text-xs font-semibold text-gray-700 dark:text-gray-300">
				Sender ID
			</label>
			<select
				id="sched-sender"
				bind:value={editSenderId}
				class="mt-1 w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 focus:border-teal-600 focus:outline-none dark:border-gray-700 dark:bg-gray-800 dark:text-white"
			>
				{#each senders as s (s.id)}
					<option value={s.id}>{s.sender_id}</option>
				{/each}
			</select>
		</div>

		<div>
			<label for="sched-recipients" class="block text-xs font-semibold text-gray-700 dark:text-gray-300">
				Recipient Phone Numbers
			</label>
			<textarea
				id="sched-recipients"
				rows="3"
				bind:value={editRecipientsList}
				placeholder="017xxxxxxxx&#10;018xxxxxxxx"
				class="mt-1 w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 focus:border-teal-600 focus:outline-none dark:border-gray-700 dark:bg-gray-800 dark:text-white"
			></textarea>
			<p class="mt-1 text-[11px] text-gray-500">Comma or new line separated phone numbers.</p>
		</div>

		<div>
			<label for="sched-body" class="block text-xs font-semibold text-gray-700 dark:text-gray-300">
				Message Content
			</label>
			<textarea
				id="sched-body"
				rows="4"
				bind:value={editBody}
				class="mt-1 w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 focus:border-teal-600 focus:outline-none dark:border-gray-700 dark:bg-gray-800 dark:text-white"
			></textarea>
		</div>

		<div class="flex justify-end gap-2 pt-2">
			<button
				type="button"
				onclick={() => (editOpen = false)}
				class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-xs font-semibold text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300"
			>
				Cancel
			</button>
			<button
				type="button"
				onclick={submitEdit}
				disabled={savingEdit}
				class="rounded-lg bg-teal-600 px-4 py-2 text-xs font-semibold text-white hover:bg-teal-500 disabled:opacity-50"
			>
				{savingEdit ? 'Saving...' : 'Save Changes'}
			</button>
		</div>
	</div>
</Modal>

<!-- Delete Confirmation Modal -->
<Modal bind:open={deleteOpen} title="Delete Scheduled Message">
	<div class="space-y-4">
		<p class="text-sm text-gray-600 dark:text-gray-300">
			Are you sure you want to permanently delete this scheduled message? This action cannot be undone and the scheduled message will be cancelled.
		</p>
		<div class="flex justify-end gap-2 pt-2">
			<button
				type="button"
				onclick={() => (deleteOpen = false)}
				class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-xs font-semibold text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300"
			>
				Cancel
			</button>
			<button
				type="button"
				onclick={executeDelete}
				disabled={deleting}
				class="rounded-lg bg-rose-600 px-4 py-2 text-xs font-semibold text-white hover:bg-rose-500 disabled:opacity-50"
			>
				{deleting ? 'Deleting...' : 'Yes, Delete Permanently'}
			</button>
		</div>
	</div>
</Modal>

