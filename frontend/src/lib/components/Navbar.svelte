<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte.js';
	import { goto } from '$app/navigation';

	interface Props {
		title?: string;
		onDepositClick?: () => void;
		onToggleSidebar?: () => void;
	}

	let { title = '', onDepositClick, onToggleSidebar }: Props = $props();

	let userMenuOpen = $state(false);

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
			<input
				type="text"
				placeholder="Search"
				class="w-full rounded-lg border border-gray-200 bg-white py-1.5 pl-9 pr-3 text-xs text-gray-800 placeholder-gray-400 focus:border-teal-500 focus:outline-hidden focus:ring-1 focus:ring-teal-500 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200"
			/>
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
		<div class="relative">
			<button
				type="button"
				class="rounded-lg p-1.5 text-gray-500 hover:bg-gray-100 hover:text-gray-900 transition dark:text-gray-400 dark:hover:bg-gray-800 cursor-pointer"
				title="Notifications"
			>
				<svg class="h-4.5 w-4.5" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="M14.857 17.082a23.848 23.848 0 0 0 5.454-1.31A8.967 8.967 0 0 1 18 9.75V9A6 6 0 0 0 6 9v.75a8.967 8.967 0 0 1-2.312 6.022c1.733.64 3.56 1.085 5.455 1.31m5.714 0a24.255 24.255 0 0 1-5.714 0m5.714 0a3 3 0 1 1-5.714 0" />
				</svg>
				<span class="absolute -top-0.5 -right-0.5 flex h-4 w-4 items-center justify-center rounded-full bg-teal-100 text-[9px] font-bold text-teal-700 dark:bg-teal-900 dark:text-teal-300">
					3
				</span>
			</button>
		</div>

		<!-- Apps Square -->
		<button
			type="button"
			class="rounded-lg p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-700 transition dark:hover:bg-gray-800"
			title="Apps"
		>
			<svg class="h-4.5 w-4.5" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" d="M3.75 6A2.25 2.25 0 0 1 6 3.75h2.25A2.25 2.25 0 0 1 10.5 6v2.25a2.25 2.25 0 0 1-2.25 2.25H6a2.25 2.25 0 0 1-2.25-2.25V6ZM3.75 15.75A2.25 2.25 0 0 1 6 13.5h2.25a2.25 2.25 0 0 1 2.25 2.25V18a2.25 2.25 0 0 1-2.25 2.25H6A2.25 2.25 0 0 1 3.75 18v-2.25ZM13.5 6a2.25 2.25 0 0 1 2.25-2.25H18A2.25 2.25 0 0 1 20.25 6v2.25A2.25 2.25 0 0 1 18 10.5h-2.25a2.25 2.25 0 0 1-2.25-2.25V6ZM13.5 15.75a2.25 2.25 0 0 1 2.25-2.25H18a2.25 2.25 0 0 1 2.25 2.25V18A2.25 2.25 0 0 1 18 20.25h-2.25A2.25 2.25 0 0 1 13.5 18v-2.25Z" />
			</svg>
		</button>

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
