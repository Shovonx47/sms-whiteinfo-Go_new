<script lang="ts">
	import { page } from '$app/stores';
	import SenderForm from '$lib/components/sender/SenderForm.svelte';

	let senderId = $derived(Number($page.params.id));
</script>

<svelte:head>
	<title>Edit Sender — SMS Whiteinfo Administrator</title>
</svelte:head>

<div class="space-y-5">
	<div>
		<nav class="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400" aria-label="Breadcrumb">
			<a href="/superman/sender-cli/add" class="hover:text-emerald-700 dark:hover:text-emerald-400">Senders</a>
			<span>&rsaquo;</span>
			<span>Edit</span>
		</nav>
		<h1 class="mt-1 text-3xl font-bold tracking-tight text-gray-950 dark:text-white">Edit Sender</h1>
	</div>

	{#key senderId}
		<SenderForm {senderId} />
	{/key}
</div>
