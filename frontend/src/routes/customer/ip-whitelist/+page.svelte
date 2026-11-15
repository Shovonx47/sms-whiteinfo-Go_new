<script lang="ts">
	import { api } from '$lib/api.js';
	import Modal from '$lib/components/Modal.svelte';
	import { toast } from '$lib/stores/toast.svelte.js';
	import { onMount } from 'svelte';

	interface IpRequest {
		id: number;
		ip_address: string;
		label: string;
		status: string;
		admin_note?: string | null;
		created_at: string;
		reviewed_at?: string | null;
	}

	interface Meta {
		total: number;
		per_page: number;
		current_page: number;
	}

	let ips = $state<IpRequest[]>([]);
	let meta = $state<Meta>({ total: 0, per_page: 10, current_page: 1 });
	let loading = $state(true);
	let search = $state('');

	let requestModalOpen = $state(false);
	let newIp = $state('');
	let newLabel = $state('');
	let formError = $state('');
	let submitting = $state(false);

	let editModalOpen = $state(false);
	let editing: IpRequest | null = $state(null);
	let editIp = $state('');
	let editLabel = $state('');
	let editError = $state('');
	let savingEdit = $state(false);

	let removeTarget = $state<IpRequest | null>(null);
	let removeOpen = $state(false);
	let removing = $state(false);

	function formatDateTime(value?: string | null) {
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

	async function load(p = 1) {
		loading = true;
		try {
			const params = new URLSearchParams({ page: String(p), per_page: String(meta.per_page) });
			if (search.trim()) params.set('search', search.trim());
			const data = await api.get<{ data: IpRequest[]; meta: Meta }>(
				`/api/v1/customer/ip-whitelists?${params}`
			);
			ips = data.data ?? [];
			meta = data.meta ?? { total: 0, per_page: meta.per_page, current_page: p };
		} catch {
			ips = [];
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

	async function handleRequestIp() {
		formError = '';
		const ip = newIp.trim();
		const label = newLabel.trim();
		if (!ip) {
			formError = 'The public IP address / CIDR block field is required.';
			return;
		}
		if (ip.length > 45) {
			formError = 'The public IP address / CIDR block must not be greater than 45 characters.';
			return;
		}
		if (label.length > 100) {
			formError = 'The label / description must not be greater than 100 characters.';
			return;
		}

		submitting = true;
		try {
			await api.post('/api/v1/customer/ip-whitelists', { ip_address: ip, label });
			toast.success(
				'Request Submitted',
				'Your IP whitelist request has been submitted for admin approval.'
			);
			requestModalOpen = false;
			newIp = '';
			newLabel = '';
			await load(1);
		} catch (err) {
			formError = err instanceof Error ? err.message : 'Failed to submit request.';
		} finally {
			submitting = false;
		}
	}

	function openEdit(row: IpRequest) {
		editing = row;
		editIp = row.ip_address;
		editLabel = row.label ?? '';
		editError = '';
		editModalOpen = true;
	}

	async function handleUpdateIp() {
		if (!editing) return;
		editError = '';
		const ip = editIp.trim();
		const label = editLabel.trim();
		if (!ip) {
			editError = 'The public IP address / CIDR block field is required.';
			return;
		}
		if (ip.length > 45) {
			editError = 'The public IP address / CIDR block must not be greater than 45 characters.';
			return;
		}
		if (label.length > 100) {
			editError = 'The label / description must not be greater than 100 characters.';
			return;
		}

		savingEdit = true;
		try {
			const res = await api.put<{ ip_changed?: boolean; message?: string }>(
				`/api/v1/customer/ip-whitelists/${editing.id}`,
				{ ip_address: ip, label }
			);
			if (res.ip_changed) {
				toast.warning(
					'IP Address Updated',
					'Because the IP address was modified, it has been resubmitted for admin review.'
				);
			} else {
				toast.success('IP Whitelist Updated', res.message ?? 'IP whitelist label updated.');
			}
			editModalOpen = false;
			editing = null;
			await load(meta.current_page);
		} catch (err) {
			editError = err instanceof Error ? err.message : 'Failed to update IP whitelist entry.';
		} finally {
			savingEdit = false;
		}
	}

	function confirmRemove(row: IpRequest) {
		removeTarget = row;
		removeOpen = true;
	}

	async function executeRemove() {
		if (!removeTarget || removing) return;
		removing = true;
		try {
			await api.delete(`/api/v1/customer/ip-whitelists/${removeTarget.id}`);
			toast.success('IP Whitelist Removed', 'The IP whitelist entry has been removed.');
			removeOpen = false;
			removeTarget = null;
			await load(1);
		} catch (err) {
			toast.error(
				'Remove Failed',
				err instanceof Error ? err.message : 'Failed to remove IP whitelist entry.'
			);
		} finally {
			removing = false;
		}
	}

	function statusLabel(status: string) {
		if (status === 'approved') return 'Approved';
		if (status === 'rejected') return 'Rejected';
		return 'Pending';
	}

	function statusClass(status: string) {
		if (status === 'approved')
			return 'bg-emerald-50 text-emerald-700 border-emerald-200 dark:bg-emerald-950/40 dark:text-emerald-300 dark:border-emerald-800';
		if (status === 'rejected')
			return 'bg-rose-50 text-rose-700 border-rose-200 dark:bg-rose-950/40 dark:text-rose-300 dark:border-rose-800';
		return 'bg-amber-50 text-amber-700 border-amber-200 dark:bg-amber-950/40 dark:text-amber-300 dark:border-amber-800';
	}

	let totalPages = $derived(Math.max(1, Math.ceil((meta.total || 0) / (meta.per_page || 10))));
</script>

<svelte:head>
	<title>IP Whitelist - White INFO</title>
</svelte:head>

<div class="space-y-6">
	<div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
		<h1 class="text-2xl font-bold tracking-tight text-gray-950 dark:text-white">IP Whitelist Requests</h1>
		<button
			type="button"
			onclick={() => {
				newIp = '';
				newLabel = '';
				formError = '';
				requestModalOpen = true;
			}}
			class="inline-flex items-center gap-2 rounded-lg bg-teal-600 px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:bg-teal-500"
		>
			<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
			</svg>
			Request IP Whitelist
		</button>
	</div>

	<div class="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-sm dark:border-gray-800 dark:bg-gray-900">
		<div class="flex items-center justify-end border-b border-gray-100 p-3 dark:border-gray-800">
			<div class="relative w-64">
				<input
					type="text"
					bind:value={search}
					oninput={handleSearchInput}
					placeholder="Search IP addresses or labels..."
					class="w-full rounded-lg border border-gray-300 bg-white py-1.5 pr-3 pl-8 text-xs text-gray-900 placeholder-gray-400 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200"
				/>
				<svg class="pointer-events-none absolute top-2.5 left-2.5 h-3.5 w-3.5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
				</svg>
			</div>
		</div>

		<div class="overflow-x-auto">
			<table class="w-full text-left text-xs">
				<thead>
					<tr class="border-b border-gray-200 bg-gray-50/70 font-semibold tracking-wider text-gray-600 uppercase dark:border-gray-700 dark:bg-gray-800/70 dark:text-gray-300">
						<th class="px-4 py-3">IP Address</th>
						<th class="px-4 py-3">Label</th>
						<th class="px-4 py-3">Status</th>
						<th class="px-4 py-3">Admin Remark</th>
						<th class="px-4 py-3">Requested At</th>
						<th class="px-4 py-3">Reviewed At</th>
						<th class="px-4 py-3 text-right"></th>
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-100 dark:divide-gray-800">
					{#if loading}
						<tr>
							<td colspan="7" class="px-4 py-12 text-center text-gray-500">
								<div class="inline-block h-6 w-6 animate-spin rounded-full border-2 border-gray-300 border-t-teal-600"></div>
								<p class="mt-2">Loading IP whitelist requests...</p>
							</td>
						</tr>
					{:else if ips.length === 0}
						<tr>
							<td colspan="7" class="px-4 py-12 text-center text-gray-500">No IP whitelist requests found.</td>
						</tr>
					{:else}
						{#each ips as row (row.id)}
							<tr class="transition hover:bg-gray-50/70 dark:hover:bg-gray-800/40">
								<td class="px-4 py-3 font-mono font-bold whitespace-nowrap text-gray-900 dark:text-gray-100">{row.ip_address}</td>
								<td class="px-4 py-3 text-gray-700 dark:text-gray-300">{row.label || '—'}</td>
								<td class="px-4 py-3">
									<span class="inline-flex items-center rounded-full border px-2.5 py-0.5 text-[11px] font-semibold {statusClass(row.status)}">
										{statusLabel(row.status)}
									</span>
								</td>
								<td class="max-w-xs truncate px-4 py-3 text-rose-700 dark:text-rose-300" title={row.admin_note || ''}>{row.admin_note || '—'}</td>
								<td class="px-4 py-3 whitespace-nowrap text-gray-600 dark:text-gray-400">{formatDateTime(row.created_at)}</td>
								<td class="px-4 py-3 whitespace-nowrap text-gray-600 dark:text-gray-400">{formatDateTime(row.reviewed_at)}</td>
								<td class="px-4 py-3 text-right whitespace-nowrap">
									<div class="flex items-center justify-end gap-1">
										<button
											type="button"
											onclick={() => openEdit(row)}
											class="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-teal-700 dark:hover:bg-gray-700"
											title="Edit"
										>
											<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
												<path stroke-linecap="round" stroke-linejoin="round" d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L10.582 16.07a4.5 4.5 0 0 1-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 0 1 1.13-1.897l8.932-8.931Z" />
											</svg>
										</button>
										<button
											type="button"
											onclick={() => confirmRemove(row)}
											class="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-rose-600 dark:hover:bg-gray-700"
											title="Remove"
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

		<div class="flex items-center justify-between border-t border-gray-200 px-4 py-3 text-xs text-gray-600 dark:border-gray-700 dark:text-gray-400">
			<div>
				{#if meta.total === 0}
					No results
				{:else}
					Showing {(meta.current_page - 1) * meta.per_page + 1} to {Math.min(meta.current_page * meta.per_page, meta.total)} of {meta.total} results
				{/if}
			</div>
			{#if totalPages > 1}
				<div class="flex items-center gap-1">
					<button type="button" disabled={meta.current_page <= 1} onclick={() => load(meta.current_page - 1)} class="rounded border border-gray-300 px-2 py-1 disabled:opacity-40 dark:border-gray-700">Previous</button>
					<span class="px-2">Page {meta.current_page} / {totalPages}</span>
					<button type="button" disabled={meta.current_page >= totalPages} onclick={() => load(meta.current_page + 1)} class="rounded border border-gray-300 px-2 py-1 disabled:opacity-40 dark:border-gray-700">Next</button>
				</div>
			{/if}
		</div>
	</div>
</div>

<Modal bind:open={requestModalOpen} title="Request IP Whitelist" description="Submit an IP address or CIDR range to be whitelisted for API access. Requests require admin approval before becoming active.">
	<form onsubmit={(e) => { e.preventDefault(); handleRequestIp(); }} class="space-y-4">
		{#if formError}
			<p class="rounded-lg border border-rose-200 bg-rose-50 p-3 text-xs text-rose-700">{formError}</p>
		{/if}
		<div>
			<label for="ip-address" class="mb-1.5 block text-xs font-semibold text-gray-700 dark:text-gray-300">Public IP Address / CIDR Block <span class="text-red-500">*</span></label>
			<input
				id="ip-address"
				type="text"
				bind:value={newIp}
				placeholder="e.g. 203.0.113.195 or 198.51.100.0/24"
				maxlength="45"
				required
				class="w-full rounded-lg border border-gray-300 bg-white px-3.5 py-2.5 font-mono text-sm text-gray-900 placeholder-gray-400 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
			/>
			<p class="mt-1 text-[11px] text-gray-500">Enter your public, internet-facing IP address (IPv4/IPv6) or CIDR block. Private/local IPs (e.g. 192.168.x.x, 10.x.x.x) are strictly rejected.</p>
		</div>
		<div>
			<label for="ip-label" class="mb-1.5 block text-xs font-semibold text-gray-700 dark:text-gray-300">Label / Description</label>
			<input
				id="ip-label"
				type="text"
				bind:value={newLabel}
				placeholder="e.g. Production API Server"
				maxlength="100"
				class="w-full rounded-lg border border-gray-300 bg-white px-3.5 py-2.5 text-sm text-gray-900 placeholder-gray-400 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
			/>
			<p class="mt-1 text-[11px] text-gray-500">An optional label to identify this IP or server.</p>
		</div>
		<div class="flex justify-end gap-3 pt-2">
			<button type="button" onclick={() => (requestModalOpen = false)} class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300">Cancel</button>
			<button type="submit" disabled={submitting} class="rounded-lg bg-teal-600 px-4 py-2 text-sm font-semibold text-white hover:bg-teal-500 disabled:opacity-50">{submitting ? 'Creating...' : 'Create'}</button>
		</div>
	</form>
</Modal>

<Modal bind:open={editModalOpen} title="Edit IP Whitelist Request" description="Update the IP address or label. Changing the IP address resubmits this entry for admin review.">
	<form onsubmit={(e) => { e.preventDefault(); handleUpdateIp(); }} class="space-y-4">
		{#if editError}
			<p class="rounded-lg border border-rose-200 bg-rose-50 p-3 text-xs text-rose-700">{editError}</p>
		{/if}
		<div>
			<label for="edit-ip-address" class="mb-1.5 block text-xs font-semibold text-gray-700 dark:text-gray-300">Public IP Address / CIDR Block <span class="text-red-500">*</span></label>
			<input id="edit-ip-address" type="text" bind:value={editIp} maxlength="45" required class="w-full rounded-lg border border-gray-300 bg-white px-3.5 py-2.5 font-mono text-sm text-gray-900 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100" />
			<p class="mt-1 text-[11px] text-gray-500">Enter your public, internet-facing IP address (IPv4/IPv6) or CIDR block. Private/local IPs are strictly rejected.</p>
		</div>
		<div>
			<label for="edit-ip-label" class="mb-1.5 block text-xs font-semibold text-gray-700 dark:text-gray-300">Label / Description</label>
			<input id="edit-ip-label" type="text" bind:value={editLabel} maxlength="100" class="w-full rounded-lg border border-gray-300 bg-white px-3.5 py-2.5 text-sm text-gray-900 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100" />
			<p class="mt-1 text-[11px] text-gray-500">An optional label to identify this IP or server.</p>
		</div>
		<div class="flex justify-end gap-3 pt-2">
			<button type="button" onclick={() => (editModalOpen = false)} class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300">Cancel</button>
			<button type="submit" disabled={savingEdit} class="rounded-lg bg-teal-600 px-4 py-2 text-sm font-semibold text-white hover:bg-teal-500 disabled:opacity-50">{savingEdit ? 'Saving...' : 'Save changes'}</button>
		</div>
	</form>
</Modal>

<Modal bind:open={removeOpen} title="Remove IP Whitelist Entry">
	<div class="space-y-4">
		<p class="text-sm text-gray-600 dark:text-gray-300">Are you sure you want to remove this IP address? If approved, API requests from this IP will no longer be authorized.</p>
		<div class="flex justify-end gap-2 pt-2">
			<button type="button" onclick={() => (removeOpen = false)} class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-xs font-semibold text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300">Cancel</button>
			<button type="button" onclick={executeRemove} disabled={removing} class="rounded-lg bg-rose-600 px-4 py-2 text-xs font-semibold text-white hover:bg-rose-500 disabled:opacity-50">{removing ? 'Removing...' : 'Remove'}</button>
		</div>
	</div>
</Modal>
