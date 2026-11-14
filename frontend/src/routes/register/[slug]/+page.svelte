<script lang="ts">
	import { page } from '$app/stores';
	import { api } from '$lib/api.js';
	import { onMount } from 'svelte';
	import RegisterPage from '../+page.svelte';

	let slug = $derived($page.params.slug);
	let planName = $state('');
	let loading = $state(true);
	let planError = $state(false);

	onMount(async () => {
		try {
			const res = await api.get<{ data: { name: string; slug: string } }>(`/api/v1/customer/register/plan/${slug}`);
			planName = res.data.name;
		} catch {
			planError = true;
		} finally {
			loading = false;
		}
	});
</script>

{#if loading}
	<div class="flex min-h-screen items-center justify-center bg-gray-950 text-gray-400">
		<div class="flex flex-col items-center gap-3">
			<div class="h-8 w-8 animate-spin rounded-full border-2 border-gray-700 border-t-primary-500"></div>
			<p class="text-xs">Loading tariff plan...</p>
		</div>
	</div>
{:else if planError}
	<div class="flex min-h-screen items-center justify-center bg-gray-950 px-4">
		<div class="w-full max-w-md rounded-xl border border-gray-800 bg-gray-900 p-8 text-center">
			<h1 class="text-lg font-semibold text-white">This signup link is no longer active.</h1>
			<p class="mt-2 text-sm text-gray-400">Please use the current invitation link or create a regular account.</p>
			<a href="/register" class="mt-6 inline-flex items-center justify-center rounded-lg bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-700">
				Register without a plan
			</a>
		</div>
	</div>
{:else}
	<RegisterPage planSlug={slug} {planName} />
{/if}
