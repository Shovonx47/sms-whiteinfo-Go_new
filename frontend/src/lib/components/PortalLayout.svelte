<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte.js';
	import { goto } from '$app/navigation';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import Navbar from '$lib/components/Navbar.svelte';
	import { page } from '$app/stores';
	import { browser } from '$app/environment';
	import { onMount } from 'svelte';
	import type { User } from '$lib/stores/auth.svelte.js';

	let { children } = $props();
	let sidebarCollapsed = $state(false);

	// Page title derived from route (supports both new /customer/* and original Filament routes)
	const pageTitles: Record<string, string> = {
		'/': 'Dashboard',
		'/customer': 'Dashboard',
		'/messaging/messages/quick': 'Quick Message',
		'/customer/messages/quick': 'Quick Message',
		'/messaging/messages/bulk': 'Bulk Message',
		'/messaging/messages/smart': 'Smart Message',
		'/customer/messages/bulk': 'Bulk Message',
		'/messaging/messages': 'SMS Logs',
		'/customer/messages': 'SMS Logs',
		'/messaging/scheduled-messages': 'Scheduled Messages',
		'/customer/scheduled-messages': 'Scheduled Messages',
		'/messaging/batches': 'Batches',
		'/customer/batches': 'Batches',
		'/messaging/templates': 'Templates',
		'/customer/templates': 'Templates',
		'/messaging/senders': 'Sender IDs',
		'/customer/senders': 'Sender IDs',
		'/messaging/live-queue': 'Live Message Queue',
		'/customer/live-queue': 'Live Message Queue',
		'/audience/contacts': 'Contacts',
		'/customer/contacts': 'Contacts',
		'/audience/groups': 'Contact Groups',
		'/customer/groups': 'Contact Groups',
		'/billing/deposits': 'Deposits',
		'/customer/deposits': 'Deposits',
		'/billing/transactions': 'Transactions',
		'/customer/transactions': 'Transactions',
		'/pricing': 'Pricing',
		'/customer/pricing': 'Pricing',
		'/developer/access-tokens': 'Developer API',
		'/developer': 'Developer API',
		'/customer/developer': 'Developer API',
		'/developer/ip-whitelists': 'IP Whitelist',
		'/customer/ip-whitelist': 'IP Whitelist',
		'/analytics/messaging': 'Messaging Analytics',
		'/analytics/billing': 'Billing Analytics',
		'/analytics': 'Analytics & Reports',
		'/customer/analytics': 'Analytics & Reports',
		'/account/kyc': 'KYC Verification',
		'/customer/kyc': 'KYC Verification',
		'/profile': 'My Profile',
		'/customer/profile': 'My Profile'
	};

	let currentTitle = $derived(pageTitles[$page.url.pathname] ?? 'SMS Whiteinfo');

	// Mirrors HandleCustomerApproval + HandleCustomerKyc: restricted accounts can
	// only see their status/KYC page, and approved accounts leave status pages.
	const KYC_PATHS = ['/account/kyc', '/customer/kyc'];
	const DASHBOARD_PATHS = ['/', '/customer'];
	const STATUS_PATHS = ['/account/pending', '/account/rejected', '/account/banned'];

	function guardTarget(user: User, path: string): string | null {
		const isKycPath = KYC_PATHS.includes(path);

		// Admin-created pending customers are never trapped on the pending page.
		if (user.status === 'pending' && user.created_by_admin) {
			return null;
		}

		const statusPage = user.status === 'pending'
			? '/account/pending'
			: user.status === 'rejected'
				? '/account/rejected'
				: user.status === 'banned'
					? '/account/banned'
					: null;

		// Pending/rejected/banned customers may still open the dashboard and KYC
		// page; every other portal page sends them to their status page.
		if (statusPage) {
			const allowed = [...DASHBOARD_PATHS, ...KYC_PATHS, statusPage];
			if (!allowed.includes(path)) {
				return statusPage;
			}
		}

		// KYC documents are required until the first submission.
		if (!user.created_by_admin && !user.front_nid && !isKycPath) {
			return '/account/kyc';
		}

		// SP-created customers flagged for resubmission must fix their documents.
		if (user.created_by_sp && user.kyc_status === 'resubmit_required' && !isKycPath) {
			return '/account/kyc';
		}

		// Approved customers do not stay on status pages.
		if (!statusPage && STATUS_PATHS.includes(path)) {
			return '/customer';
		}

		return null;
	}

	onMount(async () => {
		await auth.init();
		if (!auth.isLoggedIn) {
			goto('/login');
		}
	});

	$effect(() => {
		if (!browser) return;

		const path = $page.url.pathname;
		const user = auth.user;
		if (!auth.initialized || !user) return;

		const target = guardTarget(user, path);
		if (target && target !== path) {
			goto(target, { replaceState: true });
		}
	});
</script>

{#if auth.initialized && auth.isLoggedIn}
	<div class="flex h-screen overflow-hidden bg-gray-50 text-gray-900 transition-colors dark:bg-gray-950 dark:text-gray-100">
		<Sidebar bind:collapsed={sidebarCollapsed} />

		<div class="flex min-w-0 flex-1 flex-col overflow-hidden">
			<Navbar
				title={currentTitle}
				onDepositClick={() => goto('/billing/deposits')}
				onToggleSidebar={() => (sidebarCollapsed = !sidebarCollapsed)}
			/>

			<main class="flex-1 overflow-y-auto p-4 sm:p-6">
				<div class="animate-fade-in">
					{@render children()}
				</div>
			</main>
		</div>
	</div>
{:else if !auth.initialized}
	<!-- Loading skeleton -->
	<div class="flex h-screen items-center justify-center bg-gray-50 text-gray-900 dark:bg-gray-950 dark:text-gray-100">
		<div class="flex flex-col items-center gap-4">
			<img src="/logo.png" alt="SMS Whiteinfo" class="h-10 w-auto object-contain animate-pulse" />
			<div
				class="h-8 w-8 animate-spin rounded-full border-2 border-gray-300 border-t-teal-600 dark:border-gray-700 dark:border-t-primary-500"
			></div>
			<p class="text-sm text-gray-500">Loading portal…</p>
		</div>
	</div>
{/if}
