<script lang="ts">
	import { onMount } from 'svelte';
	import { supermanApi } from '$lib/superman-api.js';

	interface AlertStats {
		disabled_gateways: number;
		low_balance_users: number;
		failed_sms_today: number;
	}

	let stats = $state<AlertStats>({ disabled_gateways: 0, low_balance_users: 0, failed_sms_today: 0 });

	async function load() {
		try {
			const response = await supermanApi.get<{ data: AlertStats }>('/api/v1/superman/alerts/dashboard');
			stats = response.data ?? stats;
		} catch {
			// Keep the dashboard's zero state instead of an error banner.
		}
	}

	onMount(() => {
		void load();
	});
</script>

<svelte:head>
	<title>System Incident &amp; Operations Alerts — SMS Whiteinfo Administrator</title>
</svelte:head>

<div class="space-y-6">
	<h1 class="text-3xl font-bold tracking-tight text-gray-950 dark:text-white">System Incident & Operations Alerts</h1>

	<div class="grid grid-cols-1 gap-6 md:grid-cols-3">
		<div class="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-700 dark:bg-gray-800">
			<div class="flex items-center justify-between">
				<div>
					<p class="text-xs font-semibold uppercase tracking-wider text-gray-500">Gateway Alerts</p>
					<p class="mt-1 text-3xl font-extrabold text-gray-900 dark:text-white">{stats.disabled_gateways}</p>
					<p class="mt-1 text-xs text-gray-500">Inactive / Down Gateways</p>
				</div>
				<div class="rounded-xl bg-rose-50 p-3 dark:bg-rose-950/40">
					<svg class="h-8 w-8 text-rose-500" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M5.25 14.25h13.5m-13.5 0 2.25-7.5h9l2.25 7.5m-13.5 0v3h13.5v-3M6.75 20.25h10.5" />
					</svg>
				</div>
			</div>
			<div class="mt-4">
				<a href="/superman/alerts/provider-down" class="text-xs font-medium text-emerald-600 hover:underline dark:text-emerald-400">View Outage Details →</a>
			</div>
		</div>

		<div class="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-700 dark:bg-gray-800">
			<div class="flex items-center justify-between">
				<div>
					<p class="text-xs font-semibold uppercase tracking-wider text-gray-500">Low Balance Alerts</p>
					<p class="mt-1 text-3xl font-extrabold text-gray-900 dark:text-white">{stats.low_balance_users}</p>
					<p class="mt-1 text-xs text-gray-500">Customers below ৳50.00</p>
				</div>
				<div class="rounded-xl bg-amber-50 p-3 dark:bg-amber-950/40">
					<svg class="h-8 w-8 text-amber-500" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M21 12a2.25 2.25 0 0 0-2.25-2.25H15a3 3 0 1 1-6 0H5.25A2.25 2.25 0 0 0 3 12v6.75A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V12Z" />
					</svg>
				</div>
			</div>
			<div class="mt-4">
				<a href="/superman/profit/alerts" class="text-xs font-medium text-emerald-600 hover:underline dark:text-emerald-400">View Low Balance Users →</a>
			</div>
		</div>

		<div class="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-700 dark:bg-gray-800">
			<div class="flex items-center justify-between">
				<div>
					<p class="text-xs font-semibold uppercase tracking-wider text-gray-500">Daily Failed SMS</p>
					<p class="mt-1 text-3xl font-extrabold text-gray-900 dark:text-white">{stats.failed_sms_today}</p>
					<p class="mt-1 text-xs text-gray-500">SMS failures today</p>
				</div>
				<div class="rounded-xl bg-blue-50 p-3 dark:bg-blue-950/40">
					<svg class="h-8 w-8 text-blue-500" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M8.625 12a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0H8.25m4.125 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0H12m4.125 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0h-.375M21 12c0 4.556-4.03 8.25-9 8.25a9.764 9.764 0 0 1-2.555-.337A5.972 5.972 0 0 1 5.41 20.97a5.969 5.969 0 0 1-.474-.065 4.48 4.48 0 0 0 .978-2.025c.09-.457-.133-.901-.467-1.226C3.93 16.178 3 14.189 3 12c0-4.556 4.03-8.25 9-8.25s9 3.694 9 8.25Z" />
					</svg>
				</div>
			</div>
			<div class="mt-4">
				<a href="/superman/sms/failed" class="text-xs font-medium text-emerald-600 hover:underline dark:text-emerald-400">Inspect Failed Messages →</a>
			</div>
		</div>
	</div>
</div>
