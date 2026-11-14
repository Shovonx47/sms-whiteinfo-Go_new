<script lang="ts">
	import { onMount } from 'svelte';
	import { supermanApi } from '$lib/superman-api.js';

	interface AdminNotification {
		id: string;
		title: string;
		body: string;
		status: string;
		url?: string;
		read_at: string | null;
		created_at: string;
	}

	let notifications = $state<AdminNotification[]>([]);
	let unreadCount = $state(0);
	let loading = $state(true);
	let error = $state('');

	async function load() {
		loading = true;
		error = '';
		try {
			const res = await supermanApi.get<{ data: AdminNotification[]; unread_count: number }>('/api/v1/superman/notifications');
			notifications = res.data ?? [];
			unreadCount = res.unread_count ?? 0;
		} catch (e: any) {
			error = e?.message || 'Unable to load notifications.';
		} finally {
			loading = false;
		}
	}

	async function markAllRead() {
		try {
			await supermanApi.post('/api/v1/superman/notifications/read-all');
			notifications = notifications.map((item) => ({ ...item, read_at: item.read_at ?? new Date().toISOString() }));
			unreadCount = 0;
		} catch (e: any) {
			error = e?.message || 'Unable to mark notifications as read.';
		}
	}

	async function markRead(item: AdminNotification) {
		if (item.read_at) return;
		try {
			await supermanApi.post(`/api/v1/superman/notifications/${item.id}/read`);
			item.read_at = new Date().toISOString();
			unreadCount = Math.max(0, unreadCount - 1);
		} catch (e: any) {
			error = e?.message || 'Unable to mark notification as read.';
		}
	}

	onMount(load);
</script>

<div class="max-w-3xl space-y-6">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-xl font-bold text-gray-900 dark:text-white">Notifications</h1>
			<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{unreadCount} unread notification{unreadCount === 1 ? '' : 's'}.</p>
		</div>
		<div class="flex gap-2">
			<button
				type="button"
				onclick={load}
				class="rounded-lg border border-gray-200 bg-white px-3 py-1.5 text-xs font-semibold text-gray-700 hover:bg-gray-50 dark:border-gray-800 dark:bg-gray-900 dark:text-gray-300"
			>
				Refresh
			</button>
			{#if unreadCount > 0}
				<button
					type="button"
					onclick={markAllRead}
					class="rounded-lg bg-emerald-600 px-3 py-1.5 text-xs font-semibold text-white hover:bg-emerald-700"
				>
					Mark all as read
				</button>
			{/if}
		</div>
	</div>

	{#if error}
		<div class="rounded-lg border border-red-200 bg-red-50 p-4 text-xs text-red-700 dark:border-red-800 dark:bg-red-950/40 dark:text-red-300">{error}</div>
	{/if}

	<div class="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
		{#if loading}
			<p class="px-5 py-10 text-center text-xs text-gray-400">Loading notifications…</p>
		{:else if notifications.length === 0}
			<p class="px-5 py-10 text-center text-xs text-gray-400">No notifications yet.</p>
		{:else}
			<div class="divide-y divide-gray-100 dark:divide-gray-800">
				{#each notifications as item (item.id)}
					<div class="flex items-start gap-3 px-5 py-4 {item.read_at ? '' : 'bg-emerald-50/40 dark:bg-emerald-950/10'}">
						<span class="mt-1.5 h-2 w-2 shrink-0 rounded-full {item.read_at ? 'bg-transparent' : item.status === 'danger' ? 'bg-red-500' : item.status === 'warning' ? 'bg-amber-500' : 'bg-emerald-500'}"></span>
						<div class="min-w-0 flex-1">
							<div class="flex flex-wrap items-center justify-between gap-2">
								<p class="text-sm font-semibold text-gray-900 dark:text-white">{item.title}</p>
								<p class="text-[11px] text-gray-400">{new Date(item.created_at).toLocaleString()}</p>
							</div>
							<p class="mt-1 text-xs leading-relaxed text-gray-600 dark:text-gray-400">{item.body}</p>
							<div class="mt-2 flex items-center gap-3">
								{#if item.url}
									<a href={item.url} class="text-[11px] font-semibold text-emerald-600 hover:underline dark:text-emerald-400">Open</a>
								{/if}
								{#if !item.read_at}
									<button type="button" onclick={() => markRead(item)} class="text-[11px] font-semibold text-gray-500 hover:underline dark:text-gray-400">
										Mark as read
									</button>
								{/if}
							</div>
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</div>
</div>
