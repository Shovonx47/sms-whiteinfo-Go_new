<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte.js';
	import { api } from '$lib/api.js';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { onMount } from 'svelte';

	interface NavChild {
		label: string;
		href: string;
	}

	interface NavSection {
		section: string;
		icon: string;
		items: NavChild[];
	}

	// All collapsible sections matching the original screenshots exactly
	const navSections: NavSection[] = [
		{
			section: 'Messaging Management',
			icon: 'chat-bubbles',
			items: [
				{ label: 'Live Queue', href: '/messaging/live-queue' },
				{ label: 'Sender ID List', href: '/messaging/senders' },
				{ label: 'Bulk SMS Templates', href: '/messaging/templates' },
				{ label: 'Scheduled Messages', href: '/messaging/scheduled-messages' }
			]
		},
		{
			section: 'Audience',
			icon: 'users',
			items: [
				{ label: 'Groups', href: '/audience/groups' },
				{ label: 'Contacts', href: '/audience/contacts' }
			]
		},
		{
			section: 'Reports',
			icon: 'card',
			items: [
				{ label: 'Sent Message Logs', href: '/messaging/batches' },
				{ label: 'Sent Message Details', href: '/customer/messages' },
				{ label: 'Payment History', href: '/billing/deposits' },
				{ label: 'Transactional Report', href: '/billing/transactions' },
				{ label: 'Pricing Reports', href: '/pricing' }
			]
		},
		{
			section: 'Developer',
			icon: 'code-bracket',
			items: [
				{ label: 'Access Tokens', href: '/developer/access-tokens' },
				{ label: 'IP Whitelist', href: '/developer/ip-whitelists' },
				{ label: 'API Documentation', href: '/docs' }
			]
		},
		{
			section: 'Analytics',
			icon: 'chart-bar',
			items: [
				{ label: 'Messaging', href: '/analytics/messaging' },
				{ label: 'Billing', href: '/analytics/billing' }
			]
		}
	];

	let visibleSections = $derived(
		navSections.filter((section) => section.section !== 'Developer' || auth.user?.api_access)
	);

	let { collapsed = $bindable(false) }: { collapsed: boolean } = $props();

	let currentPath = $derived($page.url.pathname);

	// All sections open by default
	let openSections = $state<Record<string, boolean>>({
		'Messaging Management': true,
		'Audience': true,
		'Reports': true,
		'Developer': true,
		'Analytics': true,
		'Account & Security': true
	});

	function toggleSection(sectionName: string) {
		openSections[sectionName] = !openSections[sectionName];
	}

	let isImpersonating = $state(false);
	let scheduledCount = $state(0);

	async function loadScheduledCount() {
		try {
			const res = await api.get<{ meta?: { total?: number } }>('/api/v1/customer/scheduled-messages?per_page=1');
			scheduledCount = res.meta?.total ?? 0;
		} catch {
			scheduledCount = 0;
		}
	}

	onMount(() => {
		loadScheduledCount();
		const timer = setInterval(loadScheduledCount, 30000);
		return () => clearInterval(timer);
	});

	onMount(() => {
		try {
			isImpersonating = sessionStorage.getItem('sms_impersonation') !== null;
		} catch {
			isImpersonating = false;
		}
	});

	function handleLeaveImpersonation() {
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

	function isActive(href: string): boolean {
		const cleanHref = href.split('#')[0];
		if (cleanHref === '/customer') {
			return currentPath === '/' || currentPath === '/customer';
		}
		if (currentPath === cleanHref) return true;
		if (cleanHref.includes('quick') && currentPath.includes('quick')) return true;
		if (cleanHref.includes('bulk') && currentPath.includes('bulk')) return true;
		if (cleanHref.includes('live-queue') && currentPath.includes('live-queue')) return true;
		if (cleanHref.includes('senders') && currentPath.includes('senders')) return true;
		if (cleanHref.includes('templates') && currentPath.includes('templates')) return true;
		if (cleanHref.includes('scheduled-messages') && currentPath.includes('scheduled-messages')) return true;
		if (cleanHref.includes('groups') && currentPath.includes('groups')) return true;
		if (cleanHref.includes('contacts') && currentPath.includes('contacts')) return true;
		if (cleanHref.includes('batches') && (currentPath.includes('batches') || currentPath.includes('/customer/batches'))) return true;
		if (cleanHref === '/customer/messages' && currentPath === '/customer/messages') return true;
		if (cleanHref.includes('deposits') && currentPath.includes('deposits')) return true;
		if (cleanHref.includes('transactions') && currentPath.includes('transactions')) return true;
		if (cleanHref.includes('pricing') && currentPath.includes('pricing')) return true;
		if (cleanHref.includes('developer') && currentPath.includes('developer')) return true;
		if (cleanHref.includes('ip-whitelist') && currentPath.includes('ip-whitelist')) return true;
		if (cleanHref.includes('analytics') && currentPath.includes('analytics')) return true;
		if (cleanHref.includes('kyc') && currentPath.includes('kyc')) return true;
		if (cleanHref.includes('profile') && currentPath.includes('profile')) return true;
		return currentPath.startsWith(cleanHref + '/');
	}

	async function handleLogout() {
		await auth.logout();
		goto('/login');
	}

	const icons: Record<string, string> = {
		home: `<path stroke-linecap="round" stroke-linejoin="round" d="m2.25 12 8.954-8.955c.44-.439 1.152-.439 1.591 0L21.75 12M4.5 9.75v10.125c0 .621.504 1.125 1.125 1.125H9.75v-4.875c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125V21h4.125c.621 0 1.125-.504 1.125-1.125V9.75M8.25 21h8.25" />`,
		bolt: `<path stroke-linecap="round" stroke-linejoin="round" d="m3.75 13.5 10.5-11.25L12 10.5h8.25L9.75 21.75 12 13.5H3.75Z" />`,
		sparkles: `<path stroke-linecap="round" stroke-linejoin="round" d="M9.813 15.904 9 18.75l-.813-2.846a4.5 4.5 0 0 0-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 0 0 3.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 0 0 3.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 0 0-3.09 3.09ZM18.259 8.715 18 9.75l-.259-1.035a3.375 3.375 0 0 0-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 0 0 2.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 0 0 2.456 2.456L21.75 6l-1.035.259a3.375 3.375 0 0 0-2.456 2.456ZM16.894 20.567 16.5 21.75l-.394-1.183a2.25 2.25 0 0 0-1.423-1.423L13.5 18.75l1.183-.394a2.25 2.25 0 0 0 1.423-1.423l.394-1.183.394 1.183a2.25 2.25 0 0 0 1.423 1.423l1.183.394-1.183.394a2.25 2.25 0 0 0-1.423 1.423Z" />`,
		'chat-bubbles': `<path stroke-linecap="round" stroke-linejoin="round" d="M7.5 8.25h9m-9 3H12m-9.75 1.51c0 1.6 1.123 2.994 2.707 3.227 1.129.166 2.27.293 3.423.379.35.026.67.21.865.501L12 21l2.755-4.133a1.14 1.14 0 0 1 .865-.501 48.172 48.172 0 0 0 3.423-.379c1.584-.233 2.707-1.626 2.707-3.228V6.741c0-1.602-1.123-2.995-2.707-3.228A48.394 48.394 0 0 0 12 3c-2.392 0-4.744.175-7.043.513C3.373 3.746 2.25 5.14 2.25 6.741v6.018Z" />`,
		users: `<path stroke-linecap="round" stroke-linejoin="round" d="M15 19.128a9.38 9.38 0 0 0 2.625.372 9.337 9.337 0 0 0 4.121-.952 4.125 4.125 0 0 0-7.533-2.493M15 19.128v-.003c0-1.113-.285-2.16-.786-3.07M15 19.128v.106A12.318 12.318 0 0 1 8.624 21c-2.331 0-4.512-.645-6.374-1.766l-.001-.109a6.375 6.375 0 0 1 11.964-3.07M12 6.375a3.375 3.375 0 1 1-6.75 0 3.375 3.375 0 0 1 6.75 0Zm8.25 2.25a2.625 2.625 0 1 1-5.25 0 2.625 2.625 0 0 1 5.25 0Z" />`,
		card: `<path stroke-linecap="round" stroke-linejoin="round" d="M2.25 8.25h19.5M2.25 9h19.5m-16.5 5.25h6m-6 2.25h3m-6.75 5.25h16.5a2.25 2.25 0 0 0 2.25-2.25V6.75A2.25 2.25 0 0 0 19.5 4.5h-16.5a2.25 2.25 0 0 0-2.25 2.25v10.5A2.25 2.25 0 0 0 3 20.25Z" />`,
		'code-bracket': `<path stroke-linecap="round" stroke-linejoin="round" d="M17.25 6.75 22.5 12l-5.25 5.25m-10.5 0L1.5 12l5.25-5.25m7.5-3-4.5 16.5" />`,
		'chart-bar': `<path stroke-linecap="round" stroke-linejoin="round" d="M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 0 1 3 19.875v-6.75ZM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 0 1-1.125-1.125V8.625ZM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 0 1-1.125-1.125V4.125Z" />`,
		shield: `<path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75 11.25 15 15 9.75m-3-7.036A11.959 11.959 0 0 1 3.598 6 11.99 11.99 0 0 0 3 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285Z" />`,
		door: `<path stroke-linecap="round" stroke-linejoin="round" d="M15.75 9V5.25A2.25 2.25 0 0 0 13.5 3h-6a2.25 2.25 0 0 0-2.25 2.25v13.5A2.25 2.25 0 0 0 7.5 21h6a2.25 2.25 0 0 0 2.25-2.25V15M12 9l-3 3m0 0 3 3m-3-3h12.75" />`
	};
</script>

<!-- Sidebar -->
<aside
	class="flex h-full flex-col border-r border-gray-200 bg-white transition-all duration-300 dark:border-gray-800 dark:bg-gray-900 {collapsed
		? 'w-16'
		: 'w-64'}"
>
	<!-- Navigation -->
	<nav class="flex-1 overflow-y-auto px-3 py-3 space-y-1">
		<!-- Dashboard -->
		<a
			href="/customer"
			class="group flex items-center gap-2.5 rounded-lg px-2.5 py-2 text-sm font-medium transition {isActive('/customer')
				? 'bg-gray-100/80 text-teal-700 font-semibold dark:bg-gray-800 dark:text-teal-400'
				: 'text-gray-700 hover:bg-gray-100/70 hover:text-gray-950 dark:text-gray-300 dark:hover:bg-gray-800/60 dark:hover:text-white'} {collapsed
				? 'justify-center px-0'
				: ''}"
			title={collapsed ? 'Dashboard' : ''}
		>
			<svg
				class="h-4.5 w-4.5 shrink-0 {isActive('/customer') ? 'text-teal-600 dark:text-teal-400' : 'text-gray-500 group-hover:text-gray-700'}"
				fill="none"
				viewBox="0 0 24 24"
				stroke-width="1.5"
				stroke="currentColor"
			>
				{@html icons.home}
			</svg>
			{#if !collapsed}
				<span class="truncate">Dashboard</span>
			{/if}
		</a>

		<!-- Quick Message -->
		<a
			href="/messaging/messages/quick"
			class="group flex items-center gap-2.5 rounded-lg px-2.5 py-2 text-sm font-medium transition {isActive('/messaging/messages/quick')
				? 'bg-teal-50/70 text-teal-700 font-semibold dark:bg-teal-950/40 dark:text-teal-400'
				: 'text-gray-700 hover:bg-gray-100/70 hover:text-gray-950 dark:text-gray-300 dark:hover:bg-gray-800/60 dark:hover:text-white'} {collapsed
				? 'justify-center px-0'
				: ''}"
			title={collapsed ? 'Quick Message' : ''}
		>
			<div
				class="flex h-6 w-6 shrink-0 items-center justify-center rounded-md {isActive('/messaging/messages/quick')
					? 'bg-teal-100 text-teal-700 dark:bg-teal-900/50 dark:text-teal-300'
					: 'text-teal-600'}"
			>
				<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
					{@html icons.bolt}
				</svg>
			</div>
			{#if !collapsed}
				<span class="truncate">Quick Message</span>
			{/if}
		</a>

		<!-- Bulk Message -->
		<a
			href="/messaging/messages/bulk"
			class="group flex items-center gap-2.5 rounded-lg px-2.5 py-2 text-sm font-medium transition {isActive('/messaging/messages/bulk')
				? 'bg-teal-50/70 text-teal-700 font-semibold dark:bg-teal-950/40 dark:text-teal-400'
				: 'text-gray-700 hover:bg-gray-100/70 hover:text-gray-950 dark:text-gray-300 dark:hover:bg-gray-800/60 dark:hover:text-white'} {collapsed
				? 'justify-center px-0'
				: ''}"
			title={collapsed ? 'Bulk Message' : ''}
		>
			<div
				class="flex h-6 w-6 shrink-0 items-center justify-center rounded-md {isActive('/messaging/messages/bulk')
					? 'bg-teal-100 text-teal-700 dark:bg-teal-900/50 dark:text-teal-300'
					: 'text-teal-600'}"
			>
				<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
					{@html icons.sparkles}
				</svg>
			</div>
			{#if !collapsed}
				<span class="truncate">Bulk Message</span>
			{/if}
		</a>

		<!-- Collapsible Sections -->
		{#each visibleSections as section (section.section)}
			{@const isOpen = openSections[section.section] ?? true}
			<div class="pt-2">
				{#if !collapsed}
					<!-- Section Header with toggle -->
					<button
						type="button"
						onclick={() => toggleSection(section.section)}
						class="flex w-full items-center justify-between px-2.5 py-1.5 text-xs font-semibold text-gray-700 transition hover:text-gray-950 dark:text-gray-300 dark:hover:text-white"
					>
						<div class="flex items-center gap-2 min-w-0">
							<svg class="h-4 w-4 shrink-0 text-gray-500" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
								{@html icons[section.icon] ?? ''}
							</svg>
							<span class="truncate">{section.section}</span>
						</div>
						<svg
							class="h-3.5 w-3.5 shrink-0 text-gray-400 transition-transform duration-200 {isOpen ? 'rotate-180' : ''}"
							fill="none"
							viewBox="0 0 24 24"
							stroke-width="2"
							stroke="currentColor"
						>
							<path stroke-linecap="round" stroke-linejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" />
						</svg>
					</button>

					<!-- Child items with tree guide line -->
					{#if isOpen}
						<div class="relative ml-4 pl-3 border-l border-gray-200 dark:border-gray-800 space-y-0.5 mt-0.5">
							{#each section.items as item (item.href)}
								{@const active = isActive(item.href)}
								<a
									href={item.href}
									target={item.href === '/docs' ? '_blank' : undefined}
									rel={item.href === '/docs' ? 'noopener' : undefined}
									class="group flex items-center gap-2 rounded-lg px-2 py-1.5 text-xs font-medium transition {active
										? 'bg-gray-100 text-teal-700 font-semibold dark:bg-gray-800 dark:text-teal-400'
										: 'text-gray-600 hover:bg-gray-50 hover:text-gray-900 dark:text-gray-400 dark:hover:bg-gray-800/50 dark:hover:text-gray-200'}"
								>
									<span
										class="h-1.5 w-1.5 shrink-0 rounded-full transition-colors {active
											? 'bg-teal-600'
											: 'bg-gray-300 group-hover:bg-gray-500 dark:bg-gray-600 dark:group-hover:bg-gray-400'}"
									></span>
									<span class="truncate">{item.label}</span>
									{#if item.href.includes('scheduled-messages') && scheduledCount > 0}
										<span class="ml-auto rounded-full bg-teal-100 px-1.5 py-0.5 text-[10px] font-bold text-teal-700 dark:bg-teal-900/60 dark:text-teal-300">{scheduledCount > 99 ? '99+' : scheduledCount}</span>
									{/if}
								</a>
							{/each}
						</div>
					{/if}
				{:else}
					<!-- Collapsed view: icons only -->
					<div class="space-y-1">
						{#each section.items as item (item.href)}
							{@const active = isActive(item.href)}
							<a
								href={item.href}
								class="flex items-center justify-center rounded-lg p-2 text-xs font-medium transition {active
									? 'bg-gray-100 text-teal-700 dark:bg-gray-800 dark:text-teal-400'
									: 'text-gray-600 hover:bg-gray-50 hover:text-gray-900 dark:text-gray-400 dark:hover:bg-gray-800'}"
								title={item.label}
							>
								<span class="h-2 w-2 rounded-full {active ? 'bg-teal-600' : 'bg-gray-400'}"></span>
							</a>
						{/each}
					</div>
				{/if}
			</div>
			{#if section.section === 'Messaging Management'}
				<a
					href="/account/kyc"
					class="group flex items-center gap-2.5 rounded-lg px-2.5 py-2 text-sm font-medium transition {isActive('/account/kyc')
						? 'bg-gray-100/80 text-teal-700 font-semibold dark:bg-gray-800 dark:text-teal-400'
						: 'text-gray-700 hover:bg-gray-100/70 hover:text-gray-950 dark:text-gray-300 dark:hover:bg-gray-800/60 dark:hover:text-white'} {collapsed ? 'justify-center px-0' : ''}"
					title={collapsed ? 'KYC Documents' : ''}
				>
					<svg class="h-4.5 w-4.5 shrink-0 text-gray-500" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
						{@html icons.shield}
					</svg>
					{#if !collapsed}<span class="truncate">KYC Documents</span>{/if}
				</a>
			{/if}
		{/each}
	</nav>

	<!-- Footer with User info & Signout -->
	<div class="border-t border-gray-200 bg-gray-50/50 p-3 dark:border-gray-800 dark:bg-gray-900 shrink-0">
		{#if auth.user}
			<div class="flex items-center gap-2.5 {collapsed ? 'justify-center' : ''}">
				<div
					class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-teal-600 text-xs font-semibold text-white shadow-sm"
				>
					{auth.user.name[0]?.toUpperCase()}
				</div>
				{#if !collapsed}
					<div class="min-w-0 flex-1">
						<p class="truncate text-xs font-semibold text-gray-900 dark:text-gray-200">{auth.user.name}</p>
						<p class="truncate text-[11px] text-gray-500">{auth.user.email}</p>
					</div>
					<button
						type="button"
						onclick={handleLogout}
						class="rounded-lg p-1.5 text-gray-400 transition hover:bg-gray-200 hover:text-red-600 dark:text-gray-500 dark:hover:bg-gray-800 dark:hover:text-red-400 cursor-pointer"
						title="Sign out"
					>
						<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" d="M15.75 9V5.25A2.25 2.25 0 0 0 13.5 3h-6a2.25 2.25 0 0 0-2.25 2.25v13.5A2.25 2.25 0 0 0 7.5 21h6a2.25 2.25 0 0 0 2.25-2.25V15M12 9l-3 3m0 0 3 3m-3-3h12.75" />
						</svg>
					</button>
				{/if}
			</div>
		{/if}
	</div>
</aside>
