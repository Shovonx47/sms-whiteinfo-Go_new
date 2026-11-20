<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { supermanApi } from '$lib/superman-api.js';

	interface SenderOption {
		id: number;
		sender_id: string;
		type: string;
	}

	let senders = $state<SenderOption[]>([]);
	let senderId = $state<number | null>(null);
	let name = $state('');
	let message = $state('');
	let submitting = $state(false);
	let notice = $state<string | null>(null);
	let failed = $state(false);

	async function loadSenders() {
		try {
			const response = await supermanApi.get<{ data: SenderOption[] }>('/api/v1/superman/sms/senders');
			senders = response.data ?? [];
			senderId = senders[0]?.id ?? null;
		} catch {
			senders = [];
		}
	}

	async function submit() {
		if (!name.trim()) {
			failed = true;
			notice = 'The campaign title field is required.';
			return;
		}
		submitting = true;
		failed = false;
		try {
			await supermanApi.post('/api/v1/superman/campaigns', {
				name: name.trim(),
				sender_id: senderId ?? 0,
				message
			});
			await goto('/superman/campaigns/list');
		} catch {
			failed = true;
			notice = 'The campaign could not be created. Please try again.';
		} finally {
			submitting = false;
		}
	}

	onMount(() => {
		void loadSenders();
	});
</script>

<svelte:head>
	<title>Launch New SMS Broadcast Campaign — SMS Whiteinfo Administrator</title>
</svelte:head>

<div class="space-y-4">
	<h1 class="text-3xl font-bold tracking-tight text-gray-950 dark:text-white">Launch New SMS Broadcast Campaign</h1>

	<div class="rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
		<div class="border-b border-gray-100 px-6 py-4 dark:border-gray-800">
			<h2 class="text-sm font-bold text-gray-900 dark:text-white">New SMS Marketing / Notification Campaign</h2>
			<p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
				Launch a targeted campaign across subscriber lists or upload ad-hoc contacts.
			</p>
		</div>

		<form
			class="max-w-xl space-y-5 px-6 py-5"
			onsubmit={(event) => {
				event.preventDefault();
				void submit();
			}}
		>
			<div>
				<label class="block text-sm font-medium text-gray-700 dark:text-gray-300" for="campaign-name">Campaign Title</label>
				<input
					id="campaign-name"
					type="text"
					bind:value={name}
					placeholder="e.g. Eid Mega Discount Offer 2026"
					required
					class="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-gray-700 dark:bg-gray-900 dark:text-white"
				/>
			</div>

			<div>
				<label class="block text-sm font-medium text-gray-700 dark:text-gray-300" for="campaign-sender">Sender ID</label>
				<select
					id="campaign-sender"
					bind:value={senderId}
					class="mt-1 w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-gray-700 dark:bg-gray-900 dark:text-white"
				>
					{#each senders as sender (sender.id)}
						<option value={sender.id}>{sender.sender_id} ({sender.type.replace('_', ' ').toUpperCase()})</option>
					{/each}
				</select>
			</div>

			<div>
				<label class="block text-sm font-medium text-gray-700 dark:text-gray-300" for="campaign-message">Target Broadcast Message</label>
				<textarea
					id="campaign-message"
					bind:value={message}
					rows="4"
					placeholder="Enter message text..."
					class="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-gray-700 dark:bg-gray-900 dark:text-white"
				></textarea>
			</div>

			{#if notice}
				<p class="rounded-lg border px-3 py-2 text-xs {failed ? 'border-rose-200 bg-rose-50 text-rose-700 dark:border-rose-900 dark:bg-rose-950/30 dark:text-rose-300' : 'border-emerald-200 bg-emerald-50 text-emerald-700'}">
					{notice}
				</p>
			{/if}

			<button
				type="submit"
				disabled={submitting}
				class="inline-flex items-center gap-1.5 rounded-lg bg-emerald-600 px-4 py-2 text-xs font-bold text-white hover:bg-emerald-700 disabled:opacity-60"
			>
				<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
				</svg>
				{submitting ? 'Saving…' : 'Save & Schedule Campaign'}
			</button>
		</form>
	</div>
</div>
