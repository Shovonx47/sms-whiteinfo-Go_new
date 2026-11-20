<script lang="ts">
	import { onMount } from 'svelte';
	import { supermanApi } from '$lib/superman-api.js';

	interface SenderOption {
		id: number;
		sender_id: string;
		type: string;
	}

	let senders = $state<SenderOption[]>([]);
	let senderId = $state<number | null>(null);
	let to = $state('');
	let message = $state('');
	let submitting = $state(false);
	let notice = $state<string | null>(null);
	let failed = $state(false);

	const parts = $derived(Math.ceil(Math.max(message.length, 1) / 160));

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
		if (!to.trim() || !message.trim()) {
			failed = true;
			notice = 'The recipient and message fields are required.';
			return;
		}
		submitting = true;
		failed = false;
		notice = null;
		try {
			const response = await supermanApi.post<{ message: string }>('/api/v1/superman/sms/send', {
				sender_id: senderId ?? 0,
				to: to.trim(),
				message
			});
			notice = response.message ?? 'Message queued for delivery.';
			to = '';
			message = '';
		} catch {
			failed = true;
			notice = 'The message could not be queued. Please try again.';
		} finally {
			submitting = false;
		}
	}

	onMount(() => {
		void loadSenders();
	});
</script>

<svelte:head>
	<title>Admin SMS Dispatcher — SMS Whiteinfo Administrator</title>
</svelte:head>

<div class="space-y-4">
	<h1 class="text-3xl font-bold tracking-tight text-gray-950 dark:text-white">Admin SMS Dispatcher</h1>

	<div class="rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
		<div class="border-b border-gray-100 px-6 py-4 dark:border-gray-800">
			<h2 class="text-sm font-bold text-gray-900 dark:text-white">Admin Instant SMS Dispatch Console</h2>
			<p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
				Send high-priority test messages or system notifications directly to any mobile number.
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
				<label class="block text-sm font-medium text-gray-700 dark:text-gray-300" for="sms-sender">Sender ID / Header</label>
				<select
					id="sms-sender"
					bind:value={senderId}
					class="mt-1 w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-gray-700 dark:bg-gray-900 dark:text-white"
				>
					{#each senders as sender (sender.id)}
						<option value={sender.id}>{sender.sender_id} ({sender.type.replace('_', ' ').toUpperCase()})</option>
					{/each}
				</select>
			</div>

			<div>
				<label class="block text-sm font-medium text-gray-700 dark:text-gray-300" for="sms-to">Recipient Phone Number (MSISDN)</label>
				<input
					id="sms-to"
					type="text"
					bind:value={to}
					placeholder="e.g. 01712345678"
					required
					class="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2 font-mono text-sm dark:border-gray-700 dark:bg-gray-900 dark:text-white"
				/>
			</div>

			<div>
				<label class="block text-sm font-medium text-gray-700 dark:text-gray-300" for="sms-body">Message Body</label>
				<textarea
					id="sms-body"
					bind:value={message}
					rows="4"
					placeholder="Type your message text here..."
					required
					class="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-gray-700 dark:bg-gray-900 dark:text-white"
				></textarea>
				<div class="mt-1 flex items-center justify-between text-xs text-gray-500">
					<span>Characters: <strong class="text-gray-900 dark:text-white">{message.length}</strong></span>
					<span>Estimated Parts: <strong class="text-gray-900 dark:text-white">{parts} SMS</strong></span>
				</div>
			</div>

			{#if notice}
				<p class="rounded-lg border px-3 py-2 text-xs {failed ? 'border-rose-200 bg-rose-50 text-rose-700 dark:border-rose-900 dark:bg-rose-950/30 dark:text-rose-300' : 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-300'}">
					{notice}
				</p>
			{/if}

			<button
				type="submit"
				disabled={submitting}
				class="inline-flex items-center gap-2 rounded-lg bg-emerald-600 px-4 py-2 text-xs font-bold text-white hover:bg-emerald-700 disabled:opacity-60"
			>
				<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.8" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="M6 12 3.269 3.125A59.769 59.769 0 0 1 21.485 12 59.768 59.768 0 0 1 3.27 20.875L5.999 12Zm0 0h7.5" />
				</svg>
				{submitting ? 'Dispatching…' : 'Send Immediate SMS'}
			</button>
		</form>
	</div>
</div>
