<script lang="ts">
	import { page } from '$app/state';
	import { api } from '$lib/api.js';
	import { onMount } from 'svelte';

	let status = $state<'working' | 'success' | 'error'>('working');
	let message = $state('Confirming your new email address…');
	let attempted = false;

	onMount(async () => {
		if (attempted) return;
		attempted = true;
		const token = page.url.searchParams.get('token') ?? '';
		if (!token) {
			status = 'error';
			message = 'This email verification link is invalid or has expired.';
			return;
		}
		try {
			const res = await api.post<{ message?: string }>('/api/v1/customer/email-change/verify', { token });
			status = 'success';
			message = res.message ?? 'Your email address has been updated.';
		} catch (err) {
			status = 'error';
			message = err instanceof Error ? err.message : 'This email verification link is invalid or has expired.';
		}
	});
</script>

<svelte:head>
	<title>Confirm email change – SMS Whiteinfo</title>
</svelte:head>

<div class="flex min-h-screen items-center justify-center bg-gray-50 px-4 py-12 dark:bg-gray-950">
	<div class="w-full max-w-lg rounded-2xl border border-gray-200/80 bg-white p-8 text-center shadow-sm sm:p-10 dark:border-gray-800 dark:bg-gray-900">
		<h1 class="text-2xl font-bold tracking-tight text-gray-950 dark:text-white">Email address change</h1>
		{#if status === 'working'}
			<div class="mx-auto mt-6 h-6 w-6 animate-spin rounded-full border-2 border-teal-600 border-t-transparent"></div>
			<p class="mt-4 text-sm text-gray-500 dark:text-gray-400">{message}</p>
		{:else if status === 'success'}
			<p class="mt-4 text-sm text-emerald-600 dark:text-emerald-400">{message}</p>
			<a href="/login" class="mt-6 inline-block rounded-lg bg-teal-600 px-5 py-2.5 text-sm font-semibold text-white hover:bg-teal-500">Sign in</a>
		{:else}
			<p class="mt-4 text-sm text-red-600 dark:text-red-400">{message}</p>
			<a href="/login" class="mt-6 inline-block rounded-lg border border-gray-300 px-5 py-2.5 text-sm font-semibold text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-200 dark:hover:bg-gray-800">Back to sign in</a>
		{/if}
	</div>
</div>
