<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte.js';
	import { api } from '$lib/api.js';
	import { goto } from '$app/navigation';
	import { onMount, onDestroy } from 'svelte';
	import { browser } from '$app/environment';

	interface NotificationItem {
		id: string;
		title: string;
		body: string;
		status: string;
		url?: string;
		read_at: string | null;
		created_at: string;
	}

	interface Props {
		title?: string;
		onDepositClick?: () => void;
		onToggleSidebar?: () => void;
	}

	let { title = '', onDepositClick, onToggleSidebar }: Props = $props();

	let userMenuOpen = $state(false);
	let notificationsOpen = $state(false);
	let notifications = $state<NotificationItem[]>([]);
	let unreadCount = $state(0);
	let loadingNotifications = $state(false);

	interface SearchItem { id: number; title: string; subtitle: string; url: string; }
	interface SearchCategory { category: string; items: SearchItem[]; }
	let searchOpen = $state(false);
	let searchQuery = $state('');
	let searchLoading = $state(false);
	let searchCategories = $state<SearchCategory[]>([]);
	let searchTimer: number | null = null;
	let isImpersonating = $state(false);

	function runSearch(value: string) {
		searchQuery = value;
		if (searchTimer) clearTimeout(searchTimer);
		if (!value.trim()) {
			searchCategories = [];
			searchLoading = false;
			return;
		}
		searchLoading = true;
		searchTimer = window.setTimeout(async () => {
			try {
				const res = await api.get<{ categories: SearchCategory[] }>(`/api/v1/customer/global-search?q=${encodeURIComponent(value.trim())}`);
				searchCategories = res.categories ?? [];
			} catch {
				searchCategories = [];
			} finally {
				searchLoading = false;
			}
		}, 250);
	}

	function openSearch() {
		searchOpen = true;
		searchQuery = '';
		searchCategories = [];
	}

	function closeSearch() {
		searchOpen = false;
		if (searchTimer) clearTimeout(searchTimer);
	}

	function goToSearchResult(item: SearchItem) {
		closeSearch();
		goto(item.url);
	}

	function handleGlobalKeys(e: KeyboardEvent) {
		if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
			e.preventDefault();
			searchOpen ? closeSearch() : openSearch();
		}
		if (e.key === 'Escape' && searchOpen) {
			closeSearch();
		}
	}

	function leaveImpersonation() {
		try {
			const raw = sessionStorage.getItem('sms_impersonation');
			const info = raw ? JSON.parse(raw) : null;
			sessionStorage.removeItem('sms_impersonation');
			sessionStorage.removeItem('sms_customer_session_token');
			localStorage.removeItem('sms_customer_access_token');
			window.location.assign(info?.returnTo || '/superman');
		} catch {
			window.location.assign('/superman');
		}
	}

	async function loadNotifications() {
		if (!auth.isLoggedIn) return;
		loadingNotifications = true;
		try {
			const res = await api.get<{ data: NotificationItem[]; unread_count: number }>('/api/v1/customer/notifications');
			notifications = res.data ?? [];
			unreadCount = res.unread_count ?? 0;
		} catch {
			notifications = [];
			unreadCount = 0;
		} finally {
			loadingNotifications = false;
		}
	}

	async function openNotification(item: NotificationItem) {
		try {
			if (!item.read_at) {
				await api.post(`/api/v1/customer/notifications/${item.id}/read`);
				item.read_at = new Date().toISOString();
				unreadCount = Math.max(0, unreadCount - 1);
			}
		} catch {
			// Keep the notification visible even if the read marker fails.
		}

		notificationsOpen = false;
		if (item.url) {
			goto(item.url);
		}
	}

	async function markAllRead() {
		try {
			await api.post('/api/v1/customer/notifications/read-all');
			notifications = notifications.map((item) => ({ ...item, read_at: item.read_at ?? new Date().toISOString() }));
			unreadCount = 0;
		} catch {
			// Leave the current list intact.
		}
	}

	onMount(() => {
		loadNotifications();
		const timer = setInterval(loadNotifications, 30000);
		try {
			isImpersonating = sessionStorage.getItem('sms_impersonation') !== null;
		} catch {
			isImpersonating = false;
		}
		if (browser) {
			window.addEventListener('keydown', handleGlobalKeys);
		}

		return () => {
			clearInterval(timer);
			if (browser) window.removeEventListener('keydown', handleGlobalKeys);
		};
	});

	function formatBalance(balance: number): string {
		return new Intl.NumberFormat('en-US', {
			minimumFractionDigits: 2,
			maximumFractionDigits: 2
		}).format(balance);
	}

	function getInitials(name?: string): string {
		if (!name) return 'MH';
		const parts = name.trim().split(/\s+/);
		if (parts.length >= 2) {
			return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
		}
		return name.slice(0, 2).toUpperCase();
	}

	function handleClickOutside(e: MouseEvent) {
		const target = e.target as HTMLElement;
		if (!target.closest('[data-user-menu]')) {
			userMenuOpen = false;
		}
		if (!target.closest('[data-notification-menu]')) {
			notificationsOpen = false;
		}
	}

	async function handleLogout() {
		await auth.logout();
		goto('/login');
	}
</script>

<svelte:window onclick={handleClickOutside} />

<header
	class="flex h-14 shrink-0 items-center justify-between border-b border-gray-200 bg-white px-4 dark:border-gray-800 dark:bg-gray-900 z-20"
>
	<!-- Left: collapse button + White INFO logo -->
	<div class="flex items-center gap-3">
		<button
			type="button"
			onclick={onToggleSidebar}
			class="rounded-lg p-1.5 text-gray-500 hover:bg-gray-100 hover:text-gray-900 transition dark:text-gray-400 dark:hover:bg-gray-800 cursor-pointer"
			title="Toggle sidebar"
		>
			<svg class="h-4.5 w-4.5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" d="M15.75 19.5 8.25 12l7.5-7.5" />
			</svg>
		</button>

		<a href="/customer" class="flex items-center gap-2">
			<img src="/logo.png" alt="White INFO" class="h-7 w-auto object-contain" onerror={(e) => {
				(e.currentTarget as HTMLElement).style.display = 'none';
				const fb = document.getElementById('navbar-brand-text');
				if (fb) fb.style.display = 'flex';
			}} />
			<div id="navbar-brand-text" class="hidden items-center gap-1.5 font-bold tracking-tight text-sm">
				<span class="text-teal-700">WHITE</span>
				<span class="text-rose-600">INFO</span>
			</div>
		</a>
	</div>

	<!-- Center: Search input -->
	<div class="hidden md:flex flex-1 max-w-md mx-6">
		<div class="relative w-full">
			<div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3 text-gray-400">
				<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
				</svg>
			</div>
			<button
				type="button"
				onclick={openSearch}
				class="w-full rounded-lg border border-gray-200 bg-white py-1.5 pl-9 pr-16 text-left text-xs text-gray-400 focus:border-teal-500 focus:outline-hidden focus:ring-1 focus:ring-teal-500 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-400"
			>
				Search
			</button>
			<kbd class="pointer-events-none absolute inset-y-0 right-2 my-auto flex h-5 items-center rounded border border-gray-200 bg-gray-50 px-1.5 text-[10px] font-medium text-gray-400 dark:border-gray-600 dark:bg-gray-700">Ctrl K</kbd>
		</div>
	</div>

	<!-- Right: Balance pill + Bell + Apps + User avatar -->
	<div class="flex items-center gap-3">
		<!-- Balance pill (green outline with ৳ symbol) -->
		<a
			href="/billing/deposits"
			class="inline-flex items-center gap-1.5 rounded-full border border-teal-500/60 bg-white px-3 py-1 text-xs font-semibold text-gray-900 transition hover:bg-teal-50 hover:border-teal-600 dark:border-teal-500/40 dark:bg-gray-800 dark:text-white"
			title="View deposits"
		>
			<svg class="h-3.5 w-3.5 text-teal-600" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" d="M2.25 18.75a60.07 60.07 0 0 1 15.797 2.101c.727.198 1.453-.342 1.453-1.096V18.75M3.75 4.5v.75A.75.75 0 0 1 3 6h-.75m0 0v-.375c0-.621.504-1.125 1.125-1.125H20.25M2.25 6v9m18-10.5v.75c0 .414.336.75.75.75h.75m-1.5-1.5h.375c.621 0 1.125.504 1.125 1.125v9.75c0 .621-.504 1.125-1.125 1.125h-.375m1.5-1.5H21a.75.75 0 0 0-.75.75v.75m0 0H3.75m0 0h-.375a1.125 1.125 0 0 1-1.125-1.125V15m1.5 1.5v-.75A.75.75 0 0 0 3 15h-.75M15 10.5a3 3 0 1 1-6 0 3 3 0 0 1 6 0Zm3 0h.008v.008H18V10.5Zm-12 0h.008v.008H6V10.5Z" />
			</svg>
			<span>৳ {formatBalance(auth.balance)}</span>
		</a>

		<!-- Notification Bell -->
		<div class="relative" data-notification-menu>
			<button
				type="button"
				onclick={() => (notificationsOpen = !notificationsOpen)}
				class="rounded-lg p-1.5 text-gray-500 hover:bg-gray-100 hover:text-gray-900 transition dark:text-gray-400 dark:hover:bg-gray-800 cursor-pointer"
				title="Notifications"
			>
				<svg class="h-4.5 w-4.5" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="M14.857 17.082a23.848 23.848 0 0 0 5.454-1.31A8.967 8.967 0 0 1 18 9.75V9A6 6 0 0 0 6 9v.75a8.967 8.967 0 0 1-2.312 6.022c1.733.64 3.56 1.085 5.455 1.31m5.714 0a24.255 24.255 0 0 1-5.714 0m5.714 0a3 3 0 1 1-5.714 0" />
				</svg>
				{#if unreadCount > 0}
					<span class="absolute -top-0.5 -right-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-teal-100 px-1 text-[9px] font-bold text-teal-700 dark:bg-teal-900 dark:text-teal-300">
						{unreadCount > 99 ? '99+' : unreadCount}
					</span>
				{/if}
			</button>

			{#if notificationsOpen}
				<div class="absolute right-0 top-full z-50 mt-2 w-80 overflow-hidden rounded-xl border border-gray-200 bg-white shadow-xl dark:border-gray-700 dark:bg-gray-800">
					<div class="flex items-center justify-between border-b border-gray-100 px-4 py-2.5 dark:border-gray-700">
						<p class="text-xs font-semibold text-gray-900 dark:text-gray-100">Notifications</p>
						{#if unreadCount > 0}
							<button type="button" onclick={markAllRead} class="text-[11px] font-medium text-teal-700 hover:underline dark:text-teal-400">
								Mark all read
							</button>
						{/if}
					</div>

					<div class="max-h-80 overflow-y-auto">
						{#if loadingNotifications && notifications.length === 0}
							<p class="px-4 py-6 text-center text-xs text-gray-400">Loading notifications…</p>
						{:else if notifications.length === 0}
							<p class="px-4 py-6 text-center text-xs text-gray-400">No notifications yet.</p>
						{:else}
							{#each notifications as item (item.id)}
								<button
									type="button"
									onclick={() => openNotification(item)}
									class="block w-full border-b border-gray-50 px-4 py-3 text-left transition last:border-0 hover:bg-gray-50 dark:border-gray-700/50 dark:hover:bg-gray-700/40 {item.read_at ? 'opacity-70' : ''}"
								>
									<div class="flex items-start gap-2">
										<span class="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full {item.read_at ? 'bg-transparent' : item.status === 'danger' ? 'bg-red-500' : item.status === 'warning' ? 'bg-amber-500' : 'bg-teal-500'}"></span>
										<div class="min-w-0">
											<p class="text-xs font-semibold text-gray-900 dark:text-gray-100">{item.title}</p>
											<p class="mt-0.5 text-[11px] leading-snug text-gray-500 dark:text-gray-400">{item.body}</p>
											<p class="mt-1 text-[10px] text-gray-400">{new Date(item.created_at).toLocaleString()}</p>
										</div>
									</div>
								</button>
							{/each}
						{/if}
					</div>
				</div>
			{/if}
		</div>

		{#if isImpersonating}
			<button
				type="button"
				onclick={leaveImpersonation}
				class="inline-flex items-center gap-1 rounded-full bg-gradient-to-r from-rose-600 to-red-500 px-3 py-1.5 text-[11px] font-semibold text-white shadow-sm hover:from-rose-700 hover:to-red-600"
			>
				Back to Admin
			</button>
		{/if}

		<!-- Round User Initials Avatar with Green Online Dot -->
		<div class="relative" data-user-menu>
			<button
				type="button"
				onclick={() => (userMenuOpen = !userMenuOpen)}
				class="relative flex h-8 w-8 items-center justify-center rounded-full bg-black text-xs font-semibold text-white shadow-xs hover:ring-2 hover:ring-teal-500/50 transition cursor-pointer"
				title={auth.user?.name ?? 'User menu'}
			>
				<span>{getInitials(auth.user?.name)}</span>
				<!-- Bright green online indicator dot -->
				<span class="absolute bottom-0 right-0 h-2.5 w-2.5 rounded-full border-2 border-white bg-emerald-500 dark:border-gray-900"></span>
			</button>

			{#if userMenuOpen}
				<div
					class="absolute right-0 top-full z-50 mt-2 w-60 rounded-xl border border-gray-200 bg-white py-2 shadow-xl dark:border-gray-700 dark:bg-gray-800"
				>
					<div class="border-b border-gray-100 px-4 py-2.5 dark:border-gray-700">
						<p class="text-xs font-semibold text-gray-900 dark:text-gray-100">{auth.user?.name}</p>
						<p class="truncate text-[11px] text-gray-500">{auth.user?.email}</p>
						<div class="mt-1 flex items-center justify-between text-[11px]">
							<span class="text-gray-500">Balance:</span>
							<span class="font-bold text-teal-600">৳{formatBalance(auth.balance)}</span>
						</div>
					</div>

					<div class="py-1">
						<a
							href="/customer/profile"
							onclick={() => (userMenuOpen = false)}
							class="flex items-center gap-2 px-4 py-2 text-xs text-gray-700 hover:bg-gray-50 hover:text-gray-900 dark:text-gray-300 dark:hover:bg-gray-700"
						>
							<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
								<path stroke-linecap="round" stroke-linejoin="round" d="M15.75 6a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0ZM4.501 20.118a7.5 7.5 0 0 1 14.998 0A17.933 17.933 0 0 1 12 21.75c-2.676 0-5.216-.584-7.499-1.632Z" />
							</svg>
							<span>My Profile</span>
						</a>
						<a
							href="/billing/deposits"
							onclick={() => (userMenuOpen = false)}
							class="flex items-center gap-2 px-4 py-2 text-xs text-gray-700 hover:bg-gray-50 hover:text-gray-900 dark:text-gray-300 dark:hover:bg-gray-700"
						>
							<svg class="h-4 w-4 text-teal-600" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
								<path stroke-linecap="round" stroke-linejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
							</svg>
							<span>Make Deposit</span>
						</a>
						<button
							type="button"
							onclick={handleLogout}
							class="flex w-full items-center gap-2 px-4 py-2 text-xs text-red-600 hover:bg-red-50 dark:hover:bg-red-950/30 cursor-pointer"
						>
							<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
								<path stroke-linecap="round" stroke-linejoin="round" d="M15.75 9V5.25A2.25 2.25 0 0 0 13.5 3h-6a2.25 2.25 0 0 0-2.25 2.25v13.5A2.25 2.25 0 0 0 7.5 21h6a2.25 2.25 0 0 0 2.25-2.25V15M12 9l-3 3m0 0 3 3m-3-3h12.75" />
							</svg>
							<span>Sign out</span>
						</button>
					</div>
				</div>
			{/if}
		</div>
	</div>
</header>

{#if searchOpen}
	<div class="fixed inset-0 z-[80] flex items-start justify-center bg-black/40 px-4 pt-24" onclick={(e) => { if (e.target === e.currentTarget) closeSearch(); }} onkeydown={(e) => { if (e.key === 'Escape') closeSearch(); }} role="dialog" tabindex="-1" aria-modal="true" aria-label="Global search">
		<div class="w-full max-w-xl overflow-hidden rounded-xl border border-gray-200 bg-white shadow-2xl dark:border-gray-700 dark:bg-gray-900">
			<div class="flex items-center gap-2 border-b border-gray-100 px-4 dark:border-gray-800">
				<svg class="h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" /></svg>
				<input
					type="text"
					bind:value={searchQuery}
					oninput={(e) => runSearch((e.currentTarget as HTMLInputElement).value)}
					placeholder="Search messages, contacts, templates, deposits, transactions…"
					class="w-full border-0 bg-transparent py-3 text-sm text-gray-900 placeholder-gray-400 focus:outline-none dark:text-gray-100"
					autofocus
				/>
				<button type="button" onclick={closeSearch} class="text-xs text-gray-400 hover:text-gray-600">Esc</button>
			</div>
			<div class="max-h-96 overflow-y-auto p-2">
				{#if searchLoading}
					<p class="px-4 py-8 text-center text-xs text-gray-400">Searching…</p>
				{:else if !searchQuery.trim()}
					<p class="px-4 py-8 text-center text-xs text-gray-400">Type to search your account.</p>
				{:else if searchCategories.length === 0}
					<p class="px-4 py-8 text-center text-xs text-gray-400">No results found.</p>
				{:else}
					{#each searchCategories as group (group.category)}
						<div class="mb-1">
							<p class="px-3 py-1.5 text-[10px] font-bold uppercase tracking-wider text-gray-400">{group.category}</p>
							{#each group.items as item (group.category + '-' + item.id)}
								<button type="button" onclick={() => goToSearchResult(item)} class="block w-full rounded-lg px-3 py-2 text-left hover:bg-gray-50 dark:hover:bg-gray-800">
									<p class="text-xs font-semibold text-gray-900 dark:text-gray-100">{item.title}</p>
									<p class="truncate text-[11px] text-gray-500">{item.subtitle}</p>
								</button>
							{/each}
						</div>
					{/each}
				{/if}
			</div>
		</div>
	</div>
{/if}
