<script lang="ts">
	import { page } from '$app/stores';
	import { spAuth } from '$lib/stores/spAuth.svelte.js';

	let { collapsed = $bindable(false) }: { collapsed: boolean } = $props();

	let currentPath = $derived($page.url.pathname);

	const navItems = [
		{ label: 'Dashboard', href: '/sp', icon: 'home', permission: '' },
		{ label: 'Customers', href: '/sp/customers', icon: 'users', permission: 'customers.view' },
		{ label: 'KYC Verification', href: '/sp/kyc', icon: 'id', permission: 'customers.view' },
		{ label: 'Senders', href: '/sp/senders', icon: 'send', permission: 'messaging.view' },
		{ label: 'Messaging Logs', href: '/sp/messaging/messages', icon: 'chat-bubble', permission: 'messaging.view' },
		{ label: 'Live Queue', href: '/sp/messaging/queue', icon: 'queue', permission: 'messaging.view' },
		{ label: 'Billing & Ledger', href: '/sp/billing', icon: 'credit-card', permission: 'billing.view' },
		{ label: 'Deposits', href: '/sp/deposits', icon: 'deposit', permission: 'billing.view' },
		{ label: 'Tariff Plans', href: '/sp/tariff-plans', icon: 'plan', permission: 'billing.view' },
		{ label: 'Wallet & Distribution', href: '/sp/wallet', icon: 'banknotes', permission: 'billing.view' },
		{ label: 'Employees', href: '/sp/employees', icon: 'staff', permission: 'employees.view' },
		{ label: 'Roles', href: '/sp/roles', icon: 'shield', permission: 'roles.view' },
		{ label: 'Reports', href: '/sp/reports', icon: 'chart', permission: 'reports.view' }
	];

	let visibleItems = $derived(navItems.filter((item) => !item.permission || spAuth.can(item.permission)));

	function isPathActive(href: string): boolean {
		if (href === '/sp') {
			return currentPath === '/sp';
		}
		return currentPath.startsWith(href);
	}
</script>

<aside
	class="flex flex-col border-r border-gray-200/80 bg-white transition-all duration-300 dark:border-gray-800 dark:bg-gray-900 shrink-0 select-none {collapsed ? 'w-16' : 'w-[260px] lg:w-[280px]'}"
>
	<div class="flex-1 overflow-y-auto px-3 py-4 space-y-1">
		{#each visibleItems as item}
			<a
				href={item.href}
				class="group flex items-center gap-3 rounded-lg px-3 py-2 text-xs font-medium transition-colors {isPathActive(item.href)
					? 'bg-amber-50 text-amber-900 dark:bg-amber-950/40 dark:text-amber-300 font-semibold'
					: 'text-gray-700 hover:bg-gray-100 hover:text-gray-900 dark:text-gray-300 dark:hover:bg-gray-800 dark:hover:text-white'}"
			>
				<svg class="h-4 w-4 shrink-0 {isPathActive(item.href) ? 'text-amber-600 dark:text-amber-400' : 'text-gray-400 group-hover:text-gray-600 dark:text-gray-500'}" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor">
					{#if item.icon === 'home'}
						<path stroke-linecap="round" stroke-linejoin="round" d="m2.25 12 8.954-8.955c.44-.439 1.152-.439 1.591 0L21.75 12M4.5 9.75v10.125c0 .621.504 1.125 1.125 1.125H9.75v-4.875c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125V21h4.125c.621 0 1.125-.504 1.125-1.125V9.75M8.25 21h8.25" />
					{:else if item.icon === 'users'}
						<path stroke-linecap="round" stroke-linejoin="round" d="M15 19.128a9.38 9.38 0 0 0 2.625.372 9.337 9.337 0 0 0 4.121-.952 4.125 4.125 0 0 0-7.533-2.493M15 19.128v-.003c0-1.113-.285-2.16-.786-3.07M15 19.128v.106A12.318 12.318 0 0 1 8.624 21c-2.331 0-4.512-.645-6.374-1.766l-.001-.109a6.375 6.375 0 0 1 11.964-3.07M12 6.375a3.375 3.375 0 1 1-6.75 0 3.375 3.375 0 0 1 6.75 0Zm8.25 2.25a2.625 2.625 0 1 1-5.25 0 2.625 2.625 0 0 1 5.25 0Z" />
					{:else if item.icon === 'chat-bubble'}
						<path stroke-linecap="round" stroke-linejoin="round" d="M7.5 8.25h9m-9 3H12m-9.75 1.51c0 1.6 1.123 2.994 2.707 3.227 1.129.166 2.27.293 3.423.379.35.026.67.21.865.501L12 21l2.755-4.133a1.14 1.14 0 0 1 .865-.502 48.172 48.172 0 0 0 3.423-.379c1.584-.233 2.707-1.626 2.707-3.228V6.741c0-1.602-1.123-2.995-2.707-3.228A48.394 48.394 0 0 0 12 3c-2.392 0-4.744.175-7.043.513C3.373 3.746 2.25 5.14 2.25 6.741v6.018Z" />
					{:else if item.icon === 'banknotes'}
						<path stroke-linecap="round" stroke-linejoin="round" d="M2.25 18.75a60.07 60.07 0 0 1 15.797 2.101c.727.198 1.453-.342 1.453-1.096V18.75M3.75 4.5v.75A.75.75 0 0 1 3 6h-.75m0 0v-.375c0-.621.504-1.125 1.125-1.125H20.25M2.25 6v9m18-10.5v.75c0 .414.336.75.75.75h.75m-1.5-1.5h.375c.621 0 1.125.504 1.125 1.125v9.75c0 .621-.504 1.125-1.125 1.125h-.375m1.5-1.5H21a.75.75 0 0 0-.75.75v.75m0 0H3.75m0 0h-.375a1.125 1.125 0 0 1-1.125-1.125V15m1.5 1.5v-.75A.75.75 0 0 0 3 15h-.75M15 10.5a3 3 0 1 1-6 0 3 3 0 0 1 6 0Zm3 0h.008v.008H18V10.5Zm-12 0h.008v.008H6V10.5Z" />
					{:else if item.icon === 'credit-card'}
						<path stroke-linecap="round" stroke-linejoin="round" d="M2.25 8.25h19.5M2.25 9h19.5m-16.5 5.25h6m-6 2.25h3m-3.75 3h15a2.25 2.25 0 0 0 2.25-2.25V6.75A2.25 2.25 0 0 0 19.5 4.5h-15a2.25 2.25 0 0 0-2.25 2.25v10.5A2.25 2.25 0 0 0 4.5 19.5Z" />
					{:else if item.icon === 'queue'}
						<path stroke-linecap="round" stroke-linejoin="round" d="M3.75 12h16.5m-16.5 3.75h16.5M3.75 19.5h16.5M5.625 4.5h12.75a1.875 1.875 0 0 1 0 3.75H5.625a1.875 1.875 0 0 1 0-3.75Z" />
					{:else if item.icon === 'id'}
						<path stroke-linecap="round" stroke-linejoin="round" d="M15 9h3.75M15 12h3.75M15 15h3.75M4.5 19.5h15a2.25 2.25 0 0 0 2.25-2.25V6.75A2.25 2.25 0 0 0 19.5 4.5h-15a2.25 2.25 0 0 0-2.25 2.25v10.5A2.25 2.25 0 0 0 4.5 19.5Zm6-10.125a1.875 1.875 0 1 1-3.75 0 1.875 1.875 0 0 1 3.75 0Zm-3.75 5.625c0-1.036.84-1.875 1.875-1.875h.75c1.036 0 1.875.84 1.875 1.875v.375H6.75v-.375Z" />
					{:else if item.icon === 'send'}
						<path stroke-linecap="round" stroke-linejoin="round" d="M6 12 3.269 3.126A59.768 59.768 0 0 1 21.485 12 59.77 59.77 0 0 1 3.27 20.876L5.999 12Zm0 0h7.5" />
					{:else if item.icon === 'deposit'}
						<path stroke-linecap="round" stroke-linejoin="round" d="M2.25 8.25h19.5M2.25 9h19.5M3 18.75h18M4.5 6h15a1.5 1.5 0 0 1 1.5 1.5v9a1.5 1.5 0 0 1-1.5 1.5h-15A1.5 1.5 0 0 1 3 16.5v-9A1.5 1.5 0 0 1 4.5 6Z" />
					{:else if item.icon === 'plan'}
						<path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 0 0-3.375-3.375h-1.5A1.125 1.125 0 0 1 13.5 7.125v-1.5A3.375 3.375 0 0 0 10.125 2.25H8.25m0 12.75h7.5m-7.5 3H12M10.5 2.25H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 0 0-9-9Z" />
					{:else if item.icon === 'staff'}
						<path stroke-linecap="round" stroke-linejoin="round" d="M18 18.72a9.094 9.094 0 0 0 3.741-.479 3 3 0 0 0-4.682-2.72M18 18.72a3 3 0 1 0-3-5.196M18 18.72v.75a3 3 0 0 1-3 3H6a3 3 0 0 1-3-3v-.75m15 0a3 3 0 0 0-3-3H9a3 3 0 0 0-3 3M12 6.75a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z" />
					{:else if item.icon === 'shield'}
						<path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75 11.25 15 15 9.75m-3-7.036A11.959 11.959 0 0 1 3.598 6 11.99 11.99 0 0 0 3 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.286Zm0 13.036h.008v.008H12v-.008Z" />
					{:else if item.icon === 'chart'}
						<path stroke-linecap="round" stroke-linejoin="round" d="M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 0 1 3 19.875v-6.75ZM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 0 1-1.125-1.125V8.625ZM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 0 1-1.125-1.125V4.125Z" />
					{:else}
						<path stroke-linecap="round" stroke-linejoin="round" d="M2.25 8.25h19.5M2.25 9h19.5m-16.5 5.25h6m-6 2.25h3m-3.75 3h15a2.25 2.25 0 0 0 2.25-2.25V6.75A2.25 2.25 0 0 0 19.5 4.5h-15a2.25 2.25 0 0 0-2.25 2.25v10.5A2.25 2.25 0 0 0 4.5 19.5Z" />
					{/if}
				</svg>
				{#if !collapsed}
					<span class="truncate">{item.label}</span>
				{/if}
			</a>
		{/each}
	</div>
</aside>
