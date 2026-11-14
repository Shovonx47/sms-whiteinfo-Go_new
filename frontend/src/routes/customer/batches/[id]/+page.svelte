<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { api, ApiError } from '$lib/api.js';
	import { toast } from '$lib/components/Toast.svelte';
	import DataTable from '$lib/components/DataTable.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import { onMount, onDestroy } from 'svelte';

	interface BatchDetail {
		id: number;
		sender_id: string;
		template_id?: number | null;
		status: string;
		approval: string | null;
		source_type: string;
		body: string;
		total_recipients: number;
		valid_count?: number;
		invalid_count?: number;
		failed_report_path?: string | null;
		sent_count: number;
		delivered_count: number;
		failed_count: number;
		cost: string;
		scheduled_at: string | null;
		started_at?: string | null;
		completed_at?: string | null;
		created_at: string;
	}

	interface BatchMessage {
		id: number;
		to: string;
		body: string;
		status: string;
		cost: number;
		error_message?: string | null;
		sent_at?: string | null;
		delivered_at?: string | null;
	}

	let batchId = $derived($page.params.id);
	let batch = $state<BatchDetail | null>(null);
	let messages = $state<BatchMessage[]>([]);
	let loading = $state(true);
	let loadError = $state('');
	let acting = $state(false);
	let downloadingFailed = $state(false);
	let deleteConfirmOpen = $state(false);
	let pollInterval: ReturnType<typeof setInterval> | null = null;

	async function loadBatch() {
		try {
			const res = await api.get<{ data: { batch: BatchDetail; messages: BatchMessage[] } }>(
				`/api/v1/customer/batches/${batchId}`
			);
			batch = res.data.batch;
			messages = res.data.messages ?? [];
			loadError = '';
		} catch (err) {
			batch = null;
			messages = [];
			loadError = err instanceof Error ? err.message : 'Unable to load batch details.';
		} finally {
			loading = false;
		}
	}

	async function handlePause() {
		if (!batch || acting) return;
		acting = true;
		try {
			await api.post(`/api/v1/customer/batches/${batch.id}/pause`, {});
			toast.warning('Batch paused', 'This batch will not send further chunks until resumed.');
			await loadBatch();
		} catch (err) {
			const msg = err instanceof ApiError ? String(err.data?.message ?? 'Failed to pause batch.') : 'Failed to pause batch.';
			toast.error('Pause failed', msg);
		} finally {
			acting = false;
		}
	}

	async function handleResume() {
		if (!batch || acting) return;
		acting = true;
		try {
			await api.post(`/api/v1/customer/batches/${batch.id}/resume`, {});
			toast.success('Batch resumed', 'Dispatching has been resumed.');
			await loadBatch();
		} catch (err) {
			const msg = err instanceof ApiError ? String(err.data?.message ?? 'Failed to resume batch.') : 'Failed to resume batch.';
			toast.error('Resume failed', msg);
		} finally {
			acting = false;
		}
	}

	async function handleDelete() {
		if (!batch || acting) return;
		acting = true;
		try {
			await api.delete(`/api/v1/customer/batches/${batch.id}`);
			toast.success('Batch deleted', 'The batch has been removed.');
			deleteConfirmOpen = false;
			goto('/messaging/batches');
		} catch (err) {
			const msg = err instanceof ApiError ? String(err.data?.message ?? 'Failed to delete batch.') : 'Failed to delete batch.';
			toast.error('Delete failed', msg);
		} finally {
			acting = false;
		}
	}

	async function downloadFailedReport() {
		if (!batch || downloadingFailed) return;
		downloadingFailed = true;
		try {
			const blob = await api.download(`/api/v1/customer/batches/${batch.id}/failed-report`);
			const url = URL.createObjectURL(blob);
			const link = document.createElement('a');
			link.href = url;
			link.download = `batch-${batch.id}-failed-recipients.csv`;
			link.click();
			URL.revokeObjectURL(url);
			toast.success('Report downloaded', 'Failed/invalid recipients CSV downloaded.');
		} catch (err) {
			toast.error('Download failed', err instanceof Error ? err.message : 'Unable to download failed report.');
		} finally {
			downloadingFailed = false;
		}
	}

	onMount(() => {
		loadBatch();
		pollInterval = setInterval(() => {
			if (batch && (batch.status === 'processing' || batch.status === 'pending')) {
				loadBatch();
			}
		}, 3000);
	});

	onDestroy(() => {
		if (pollInterval) clearInterval(pollInterval);
	});

	function statusBadgeClass(status: string): string {
		const map: Record<string, string> = {
			pending: 'badge-pending',
			processing: 'badge-processing',
			paused: 'badge-pending',
			completed: 'badge-delivered',
			delivered: 'badge-delivered',
			sent: 'badge-sent',
			failed: 'badge-failed',
			scheduled: 'badge-scheduled'
		};
		return map[status] ?? 'badge-processing';
	}

	let progress = $derived(() => {
		if (!batch || !batch.total_recipients) return 0;
		const done = (batch.sent_count ?? 0) + (batch.failed_count ?? 0);
		return Math.min(100, Math.round((done / batch.total_recipients) * 100));
	});

	const columns = [
		{ key: 'to', label: 'Recipient Phone', class: 'font-mono text-xs text-gray-200' },
		{
			key: 'status',
			label: 'Status',
			badge: true,
			badgeClass: (row: BatchMessage) => statusBadgeClass(row.status)
		},
		{
			key: 'cost',
			label: 'Charge',
			render: (row: BatchMessage) => `৳ ${Number(row.cost || 0).toFixed(2)}`,
			class: 'font-mono text-xs text-cyan-400'
		},
		{
			key: 'error_message',
			label: 'Note / Failure',
			render: (row: BatchMessage) => row.error_message || '—',
			class: 'text-xs text-gray-400'
		}
	];
</script>

<svelte:head>
	<title>Batch #{batchId} – SMS Whiteinfo</title>
</svelte:head>

<div class="space-y-6">
	<!-- Top Navigation -->
	<div class="flex items-center gap-3">
		<a
			href="/messaging/batches"
			class="flex items-center gap-1.5 rounded-lg border border-gray-300 bg-white px-3 py-1.5 text-xs font-semibold text-gray-700 shadow-sm transition hover:bg-gray-50 dark:border-gray-800 dark:bg-gray-900/60 dark:text-gray-300 dark:hover:bg-gray-800 dark:hover:text-white"
		>
			<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" d="M10.5 19.5 3 12m0 0 7.5-7.5M3 12h18" />
			</svg>
			Back to Batches
		</a>
		<span class="text-xs text-gray-400">/</span>
		<span class="text-xs font-mono font-medium text-gray-500 dark:text-gray-400">Batch #{batchId}</span>
	</div>

	{#if loadError}
		<div class="rounded-xl border border-rose-200 bg-rose-50 p-6 text-center dark:border-rose-900/50 dark:bg-rose-950/30">
			<p class="text-sm font-semibold text-rose-800 dark:text-rose-200">Unable to load batch #{batchId}</p>
			<p class="mt-1 text-xs text-rose-600 dark:text-rose-300">{loadError}</p>
			<button
				type="button"
				onclick={loadBatch}
				class="mt-4 rounded-lg bg-rose-600 px-4 py-1.5 text-xs font-semibold text-white hover:bg-rose-500 cursor-pointer"
			>
				Retry
			</button>
		</div>
	{:else}
		<!-- Batch Overview Header Card -->
		<div class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900">
			<div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
				<div>
					<div class="flex flex-wrap items-center gap-2.5">
						<h1 class="text-xl font-bold text-gray-950 dark:text-white">Batch #{batchId}</h1>
						{#if batch}
							<span class="rounded-full px-2.5 py-0.5 text-xs font-semibold {statusBadgeClass(batch.status)}">
								{batch.status}
							</span>
							{#if batch.approval}
								<span class="rounded-full bg-amber-50 px-2.5 py-0.5 text-xs font-semibold text-amber-700 border border-amber-200 dark:bg-amber-950/40 dark:text-amber-300 dark:border-amber-800">
									Approval: {batch.approval}
								</span>
							{/if}
						{/if}
					</div>
					<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
						Sender: <strong class="text-gray-900 dark:text-gray-200">{batch?.sender_id ?? '—'}</strong> •
						Source: <span class="capitalize text-gray-700 dark:text-gray-300">{batch?.source_type ?? '—'}</span> •
						Created: <span class="text-gray-700 dark:text-gray-300">{batch?.created_at ? new Date(batch.created_at).toLocaleString() : '—'}</span>
					</p>
				</div>

				{#if batch}
					<div class="flex flex-wrap items-center gap-2">
						{#if batch.status === 'processing' || batch.status === 'scheduled'}
							<button
								type="button"
								disabled={acting}
								onclick={handlePause}
								class="rounded-lg border border-amber-300 bg-amber-50 px-3 py-1.5 text-xs font-semibold text-amber-800 transition hover:bg-amber-100 disabled:opacity-50 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-300 cursor-pointer"
							>
								Pause
							</button>
						{/if}
						{#if batch.status === 'paused'}
							<button
								type="button"
								disabled={acting}
								onclick={handleResume}
								class="rounded-lg border border-sky-300 bg-sky-50 px-3 py-1.5 text-xs font-semibold text-sky-800 transition hover:bg-sky-100 disabled:opacity-50 dark:border-sky-800 dark:bg-sky-950/40 dark:text-sky-300 cursor-pointer"
							>
								Resume
							</button>
						{/if}
						{#if ['pending', 'processing', 'paused', 'scheduled'].includes(batch.status)}
							<button
								type="button"
								disabled={acting}
								onclick={() => (deleteConfirmOpen = true)}
								class="rounded-lg border border-rose-300 bg-rose-50 px-3 py-1.5 text-xs font-semibold text-rose-700 transition hover:bg-rose-100 disabled:opacity-50 dark:border-rose-800 dark:bg-rose-950/40 dark:text-rose-300 cursor-pointer"
							>
								Delete
							</button>
						{/if}
						{#if batch.failed_report_path && ((batch.invalid_count ?? 0) > 0 || batch.failed_count > 0)}
							<button
								type="button"
								disabled={downloadingFailed}
								onclick={downloadFailedReport}
								class="flex items-center gap-2 rounded-lg border border-amber-500/30 bg-amber-500/10 px-4 py-1.5 text-xs font-semibold text-amber-700 dark:text-amber-300 transition hover:bg-amber-500/20 disabled:opacity-50 cursor-pointer"
							>
								<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
									<path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5M16.5 12 12 16.5m0 0L7.5 12m4.5 4.5V3" />
								</svg>
								{downloadingFailed ? 'Downloading…' : `Download invalid list (${batch.invalid_count || batch.failed_count})`}
							</button>
						{/if}
					</div>
				{/if}
			</div>

			<!-- Progress Bar -->
			<div class="mt-6 space-y-2">
				<div class="flex items-center justify-between text-xs">
					<span class="text-gray-500 dark:text-gray-400">Dispatch Progress</span>
					<span class="font-mono font-bold text-gray-900 dark:text-white">{progress()}%</span>
				</div>
				<div class="h-2 w-full overflow-hidden rounded-full bg-gray-100 dark:bg-gray-800">
					<div
						class="h-full bg-gradient-to-r from-teal-600 to-emerald-500 transition-all duration-500"
						style="width: {progress()}%;"
					></div>
				</div>
			</div>

			<!-- KPI Counter Strip -->
			<div class="mt-6 grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6 border-t border-gray-100 pt-6 dark:border-gray-800">
				<div class="rounded-lg border border-gray-200 bg-gray-50 p-3 text-center dark:border-gray-800 dark:bg-gray-950/40">
					<span class="text-[11px] text-gray-500 dark:text-gray-400">Total Recipients</span>
					<p class="mt-1 font-mono text-xl font-bold text-gray-950 dark:text-white">{batch?.total_recipients ?? 0}</p>
				</div>
				<div class="rounded-lg border border-gray-200 bg-gray-50 p-3 text-center dark:border-gray-800 dark:bg-gray-950/40">
					<span class="text-[11px] text-gray-500 dark:text-gray-400">Valid</span>
					<p class="mt-1 font-mono text-xl font-bold text-emerald-600 dark:text-emerald-400">{batch?.valid_count ?? 0}</p>
				</div>
				<div class="rounded-lg border border-gray-200 bg-gray-50 p-3 text-center dark:border-gray-800 dark:bg-gray-950/40">
					<span class="text-[11px] text-gray-500 dark:text-gray-400">Invalid</span>
					<p class="mt-1 font-mono text-xl font-bold text-rose-600 dark:text-rose-400">{batch?.invalid_count ?? 0}</p>
				</div>
				<div class="rounded-lg border border-gray-200 bg-gray-50 p-3 text-center dark:border-gray-800 dark:bg-gray-950/40">
					<span class="text-[11px] text-gray-500 dark:text-gray-400">Delivered</span>
					<p class="mt-1 font-mono text-xl font-bold text-emerald-600 dark:text-emerald-400">{batch?.delivered_count ?? batch?.sent_count ?? 0}</p>
				</div>
				<div class="rounded-lg border border-gray-200 bg-gray-50 p-3 text-center dark:border-gray-800 dark:bg-gray-950/40">
					<span class="text-[11px] text-gray-500 dark:text-gray-400">Failed / Rejected</span>
					<p class="mt-1 font-mono text-xl font-bold text-red-600 dark:text-red-400">{batch?.failed_count ?? 0}</p>
				</div>
				<div class="rounded-lg border border-gray-200 bg-gray-50 p-3 text-center dark:border-gray-800 dark:bg-gray-950/40">
					<span class="text-[11px] text-gray-500 dark:text-gray-400">Total Billed</span>
					<p class="mt-1 font-mono text-xl font-bold text-teal-600 dark:text-cyan-400">৳ {batch?.cost ?? '0.00'}</p>
				</div>
			</div>

			{#if batch}
				<div class="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-4 border-t border-gray-100 pt-4 text-xs dark:border-gray-800">
					<div>
						<span class="text-gray-500 dark:text-gray-400">Created</span>
						<p class="mt-0.5 font-medium text-gray-900 dark:text-gray-200">{new Date(batch.created_at).toLocaleString()}</p>
					</div>
					<div>
						<span class="text-gray-500 dark:text-gray-400">Scheduled For</span>
						<p class="mt-0.5 font-medium text-gray-900 dark:text-gray-200">{batch.scheduled_at ? new Date(batch.scheduled_at).toLocaleString() : '—'}</p>
					</div>
					<div>
						<span class="text-gray-500 dark:text-gray-400">Started</span>
						<p class="mt-0.5 font-medium text-gray-900 dark:text-gray-200">{batch.started_at ? new Date(batch.started_at).toLocaleString() : '—'}</p>
					</div>
					<div>
						<span class="text-gray-500 dark:text-gray-400">Completed</span>
						<p class="mt-0.5 font-medium text-gray-900 dark:text-gray-200">{batch.completed_at ? new Date(batch.completed_at).toLocaleString() : '—'}</p>
					</div>
				</div>
			{/if}

			<!-- Message Text Preview -->
			{#if batch?.body}
				<div class="mt-6 rounded-lg border border-gray-200 bg-gray-50 p-3.5 dark:border-gray-800 dark:bg-gray-950/50">
					<span class="text-[11px] font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">Message Content</span>
					<p class="mt-1 font-mono text-xs text-gray-800 dark:text-gray-200 whitespace-pre-wrap">{batch.body}</p>
				</div>
			{/if}
		</div>

		<!-- Recipient Messages Table -->
		<div class="space-y-4">
			<h2 class="text-sm font-bold text-gray-900 dark:text-white">Dispatched Recipient Messages</h2>
			<DataTable
				rows={messages}
				{columns}
				{loading}
			/>
		</div>
	{/if}
</div>

<Modal bind:open={deleteConfirmOpen} title="Delete Batch" size="sm">
	<div class="space-y-4 text-sm">
		<p class="text-gray-600 dark:text-gray-300">
			Are you sure you want to delete batch <strong class="text-gray-900 dark:text-white">#{batchId}</strong>? Pending recipients in this batch will be stopped.
		</p>
		<div class="flex justify-end gap-2">
			<button
				type="button"
				onclick={() => (deleteConfirmOpen = false)}
				class="rounded-lg border border-gray-300 bg-white px-4 py-1.5 text-xs font-medium text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200"
			>
				Cancel
			</button>
			<button
				type="button"
				disabled={acting}
				onclick={handleDelete}
				class="rounded-lg bg-rose-600 px-4 py-1.5 text-xs font-semibold text-white hover:bg-rose-500 disabled:opacity-50"
			>
				{acting ? 'Deleting…' : 'Delete Batch'}
			</button>
		</div>
	</div>
</Modal>
