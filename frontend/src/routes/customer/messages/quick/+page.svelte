<script lang="ts">
	import { api, ApiError } from '$lib/api.js';
	import { toast } from '$lib/components/Toast.svelte';
	import { computeSmsStats } from '$lib/utils/sms.js';
	import { goto } from '$app/navigation';
	import Modal from '$lib/components/Modal.svelte';
	import { onMount } from 'svelte';
	import { page } from '$app/state';

	interface Sender {
		id: number;
		sender_id: string;
		type?: string;
	}

	interface Template {
		id: number;
		name: string;
		body: string;
		approval?: string;
	}

	interface PricingRate {
		operator_name?: string;
		name?: string;
		masking_rate: number | null;
		non_masking_rate: number | null;
	}

	let senders = $state<Sender[]>([]);
	let selectedSenderId = $state<number | null>(null);
	let phone = $state('');
	let recentRecipients = $state<string[]>([]);
	let templates = $state<Template[]>([]);
	let selectedTemplateId = $state<number | null>(null);
	let body = $state('');
	let submitting = $state(false);
	let scheduleOpen = $state(false);

	// Pricing rates
	let nonMaskingRates = $state<number[]>([]);
	let maskingRates = $state<number[]>([]);

	// Schedule form state
	let schedDate = $state(new Date().toISOString().slice(0, 10));
	let schedHour = $state(String(new Date().getHours() % 12 || 12).padStart(2, '0'));
	let schedMinute = $state(String((new Date().getMinutes() + 10) % 60).padStart(2, '0'));
	let schedPeriod = $state<'AM' | 'PM'>(new Date().getHours() >= 12 ? 'PM' : 'AM');

	let smsStats = $derived(computeSmsStats(body));

	function formatRates(rates: number[], fallback: string): string {
		if (!rates || rates.length === 0) return fallback;
		const min = Math.min(...rates);
		const max = Math.max(...rates);
		return min === max ? `৳${min.toFixed(2)}` : `৳${min.toFixed(2)}–৳${max.toFixed(2)}`;
	}

	async function loadSenders() {
		try {
			const res = await api.get<{ data: Sender[] }>('/api/v1/customer/senders');
			senders = res.data ?? [];
			if (senders.length === 1 && !selectedSenderId) {
				selectedSenderId = senders[0].id;
			}
		} catch {
			senders = [];
		}
	}

	async function loadRecentRecipients() {
		try {
			const res = await api.get<{ data: string[] }>('/api/v1/customer/recent-recipients');
			recentRecipients = res.data ?? [];
		} catch {
			recentRecipients = [];
		}
	}

	async function loadTemplates() {
		try {
			const res = await api.get<{ data: Template[] }>('/api/v1/customer/templates');
			templates = res.data ?? [];
		} catch {
			templates = [];
		}
	}

	async function loadPricing() {
		try {
			const res = await api.get<{ data: PricingRate[] }>('/api/v1/customer/pricing');
			if (res.data && res.data.length > 0) {
				nonMaskingRates = res.data.map((r) => r.non_masking_rate).filter((p): p is number => typeof p === 'number' && p > 0);
				maskingRates = res.data.map((r) => r.masking_rate).filter((p): p is number => typeof p === 'number' && p > 0);
			}
		} catch {
			// Fallbacks
		}
	}

	onMount(() => {
		loadSenders();
		loadRecentRecipients();
		loadTemplates();
		loadPricing();
		if (page.url.searchParams.get('schedule') === '1') {
			scheduleOpen = true;
		}
	});

	function handleTemplateChange(e: Event) {
		const target = e.target as HTMLSelectElement;
		const tid = Number(target.value);
		if (!tid) return;
		const tmpl = templates.find((t) => t.id === tid);
		if (tmpl) {
			body = tmpl.body;
		}
	}

	function selectRecipient(num: string) {
		phone = num.trim();
	}

	async function sendNow() {
		if (submitting) return;

		const cleanPhone = phone.trim();
		if (!selectedSenderId) {
			toast.error('Sender ID required', 'Please select a Sender ID.');
			return;
		}
		if (!cleanPhone) {
			toast.error('Recipient required', 'Please enter a recipient phone number.');
			return;
		}
		if (!body.trim()) {
			toast.error('Empty message', 'Please write your message.');
			return;
		}

		submitting = true;
		try {
			const res = await api.post<{ data: { id: number; requires_approval?: boolean } }>('/api/v1/customer/messages/send', {
				sender_id: selectedSenderId,
				template_id: selectedTemplateId,
				phones: [cleanPhone],
				body: body.trim(),
				source: 'quick'
			});
			if (res?.data?.requires_approval) {
				toast.warning('Awaiting approval', 'Your message batch is pending admin approval before sending.');
			} else {
				toast.success('Message sent successfully', 'Your quick message has been sent successfully.');
			}
			if (res?.data?.id) {
				goto(`/messaging/batches/${res.data.id}`);
			} else {
				goto('/messaging/batches');
			}
		} catch (e) {
			if (e instanceof ApiError) {
				toast.error('Send failed', (e.data.message as string) ?? 'Please try again.');
			} else {
				toast.error('Network error', 'Could not reach the server.');
			}
		} finally {
			submitting = false;
		}
	}

	async function scheduleMessage() {
		const cleanPhone = phone.trim();
		if (!selectedSenderId || !cleanPhone || !body.trim()) {
			toast.error('Required Field Warning', 'Please fill in all required fields (Sender ID, Phone Number, and Message) correctly before scheduling.');
			return;
		}

		submitting = true;
		try {
			const res = await api.post<{ data: { id: number } }>('/api/v1/customer/messages/schedule', {
				sender_id: selectedSenderId,
				template_id: selectedTemplateId,
				phones: [cleanPhone],
				body: body.trim(),
				scheduled_date: schedDate,
				scheduled_hour: schedHour,
				scheduled_minute: schedMinute,
				scheduled_period: schedPeriod,
				source: 'quick'
			});
			toast.success('Message scheduled successfully', `Your message has been scheduled for ${schedDate} at ${schedHour}:${schedMinute} ${schedPeriod}.`);
			scheduleOpen = false;
			if (res?.data?.id) {
				goto(`/messaging/batches/${res.data.id}`);
			} else {
				goto('/messaging/scheduled-messages');
			}
		} catch (e) {
			if (e instanceof ApiError) {
				toast.error('Invalid Schedule Time', (e.data.message as string) ?? 'Please try again.');
			}
		} finally {
			submitting = false;
		}
	}

	const hours = Array.from({ length: 12 }, (_, i) => String(i + 1).padStart(2, '0'));
	const minutes = Array.from({ length: 60 }, (_, i) => String(i).padStart(2, '0'));
</script>

<svelte:head>
	<title>Quick Message - White INFO</title>
</svelte:head>

<div class="mx-auto max-w-4xl space-y-6">
	<!-- Breadcrumb -->
	<nav class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
		<a href="/customer/messages" class="hover:text-gray-800 dark:hover:text-gray-200 transition">Sent Message Details</a>
		<svg class="h-3 w-3 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
			<path stroke-linecap="round" stroke-linejoin="round" d="M8.25 4.5l7.5 7.5-7.5 7.5" />
		</svg>
		<span class="text-gray-900 dark:text-gray-100 font-medium">Quick Message</span>
	</nav>

	<!-- Page Heading -->
	<div>
		<h1 class="text-2xl font-bold tracking-tight text-gray-900 dark:text-gray-100">Quick Message</h1>
	</div>

	<!-- Form Section Card -->
	<div class="rounded-xl border border-gray-200 bg-white p-6 shadow-xs dark:border-gray-800 dark:bg-gray-900 space-y-6">
		<!-- Sender ID -->
		<div>
			<label class="mb-1.5 block text-sm font-semibold text-gray-900 dark:text-gray-100" for="sender-select">
				Sender ID <span class="text-red-500">*</span>
			</label>
			<div class="relative">
				<div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5 text-gray-400">
					<!-- Heroicon: Identification -->
					<svg class="h-5 w-5 text-gray-400 dark:text-gray-500" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M15 9h3.75M15 12h3.75M15 15h3.75M4.5 19.5h15a2.25 2.25 0 0 0 2.25-2.25V6.75A2.25 2.25 0 0 0 19.5 4.5h-15a2.25 2.25 0 0 0-2.25 2.25v10.5A2.25 2.25 0 0 0 4.5 19.5Zm6-10.125a1.875 1.875 0 1 1-3.75 0 1.875 1.875 0 0 1 3.75 0Zm1.294 6.336a6.721 6.721 0 0 1-3.17.789 6.721 6.721 0 0 1-3.168-.789 3.376 3.376 0 0 1 6.338 0Z" />
					</svg>
				</div>
				<select
					id="sender-select"
					bind:value={selectedSenderId}
					class="w-full rounded-lg border border-gray-300 bg-white py-2.5 pl-11 pr-10 text-sm text-gray-900 transition focus:border-teal-600 focus:outline-hidden focus:ring-1 focus:ring-teal-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100 cursor-pointer"
				>
					<option value={null} disabled>Select a Sender ID</option>
					{#each senders as s (s.id)}
						<option value={s.id}>{s.sender_id}</option>
					{/each}
				</select>
			</div>
		</div>

		<!-- Recipient Phone Number -->
		<div>
			<label class="mb-1.5 block text-sm font-semibold text-gray-900 dark:text-gray-100" for="recipient-phone">
				Recipient Phone Number <span class="text-red-500">*</span>
			</label>
			<div class="relative">
				<!-- Bangladesh Flag Prefix -->
				<div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5">
					<svg class="h-4 w-5 rounded-xs shrink-0 overflow-hidden shadow-xs" viewBox="0 0 20 12">
						<rect width="20" height="12" fill="#006a4e" />
						<circle cx="9" cy="6" r="4" fill="#f42a41" />
					</svg>
				</div>
				<input
					id="recipient-phone"
					type="tel"
					bind:value={phone}
					placeholder="Enter one phone number"
					class="w-full rounded-lg border border-gray-300 bg-white py-2.5 pl-11 pr-4 text-sm text-gray-900 placeholder-gray-400 transition focus:border-teal-600 focus:outline-hidden focus:ring-1 focus:ring-teal-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100 dark:placeholder-gray-500"
				/>
			</div>

			<!-- Recent Recipients Chips -->
			{#if recentRecipients.length > 0}
				<div class="mt-2.5 flex flex-wrap items-center gap-2">
					<div class="flex items-center gap-1 text-xs text-gray-500 dark:text-gray-400 font-medium">
						<svg class="h-3.5 w-3.5 text-gray-400 dark:text-gray-500" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
						</svg>
						<span>Recent recipients:</span>
					</div>
					{#each recentRecipients as rec}
						<button
							type="button"
							onclick={() => selectRecipient(rec)}
							class="inline-flex items-center gap-1 rounded-full border border-gray-200 bg-white px-2.5 py-0.5 text-[11px] font-mono text-gray-700 transition hover:border-teal-500 hover:text-teal-600 cursor-pointer dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300 dark:hover:border-teal-400 dark:hover:text-teal-300"
						>
							<span class="font-bold text-teal-600 dark:text-teal-400">+</span>
							<span>{rec}</span>
						</button>
					{/each}
				</div>
			{/if}

			<p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
				Quick Message only supports a single recipient. Use Bulk SMS for multiple recipients.
			</p>
		</div>

		<!-- Use Saved Template -->
		<div>
			<label class="mb-1.5 block text-sm font-semibold text-gray-900 dark:text-gray-100" for="template-select">
				Use Saved Template
			</label>
			<div class="relative">
				<div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5 text-gray-400">
					<!-- Heroicon: DocumentText -->
					<svg class="h-5 w-5 text-gray-400 dark:text-gray-500" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 0 0-3.375-3.375h-1.5A1.125 1.125 0 0 1 13.5 7.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H8.25m0 12.75h7.5m-7.5 3H12M10.5 2.25H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 0 0-9-9Z" />
					</svg>
				</div>
				<select
					id="template-select"
					bind:value={selectedTemplateId}
					onchange={handleTemplateChange}
					class="w-full rounded-lg border border-gray-300 bg-white py-2.5 pl-11 pr-10 text-sm text-gray-900 transition focus:border-teal-600 focus:outline-hidden focus:ring-1 focus:ring-teal-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100 cursor-pointer"
				>
					<option value={null}>— Select a Template to Autofill Message (Optional) —</option>
					{#each templates as t (t.id)}
						<option value={t.id}>
							{t.name} — {t.approval === 'approved' ? '🟢 [Approved]' : t.approval === 'rejected' ? '🔴 [Rejected]' : '🟡 [Pending Review]'}
						</option>
					{/each}
				</select>
			</div>
		</div>

		<!-- Message Textarea -->
		<div>
			<label class="mb-1.5 block text-sm font-semibold text-gray-900 dark:text-gray-100" for="quick-message-body">
				Message <span class="text-red-500">*</span>
			</label>
			<textarea
				id="quick-message-body"
				bind:value={body}
				rows={5}
				placeholder="বাংলায় আপনার এসএমএস লিখুন..."
				class="w-full rounded-lg border border-gray-300 bg-white p-3 text-sm text-gray-900 placeholder-gray-400 transition focus:border-teal-600 focus:outline-hidden focus:ring-1 focus:ring-teal-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100 dark:placeholder-gray-500"
			></textarea>

			<!-- SMS Stats Bar -->
			<div class="mt-2.5 flex flex-wrap items-center gap-x-6 gap-y-1.5 rounded-lg border border-gray-200 bg-gray-50 px-3.5 py-2.5 text-xs text-gray-600 dark:border-gray-800 dark:bg-gray-900/60 dark:text-gray-400">
				<div class="flex items-center gap-1.5">
					<span class="font-medium text-gray-700 dark:text-gray-300">Remaining:</span>
					<span>{smsStats.remaining}/{smsStats.perPartTotal}</span>
				</div>
				<div class="flex items-center gap-1.5">
					<span class="font-medium text-gray-700 dark:text-gray-300">SMS Count:</span>
					<span class="font-bold text-teal-600 dark:text-teal-400">{smsStats.parts}</span>
				</div>
				<div class="flex items-center gap-1.5">
					<span class="font-medium text-gray-700 dark:text-gray-300">Encoding:</span>
					<span>{smsStats.encoding}</span>
				</div>
				<div class="flex items-center gap-1.5">
					<span class="font-medium text-gray-700 dark:text-gray-300">Non-masking:</span>
					<span>{formatRates(nonMaskingRates, '—')}/SMS</span>
				</div>
				<div class="flex items-center gap-1.5">
					<span class="font-medium text-gray-700 dark:text-gray-300">Masking:</span>
					<span>{formatRates(maskingRates, '—')}/SMS</span>
				</div>
			</div>

			<!-- BTRC Compliance Notice Banner -->
			<div class="mt-3 flex items-start gap-2.5 rounded-lg border border-amber-500/25 bg-amber-500/10 px-3.5 py-2.5 text-xs text-amber-800 dark:text-amber-300 leading-relaxed">
				<svg class="h-4 w-4 shrink-0 text-amber-600 dark:text-amber-400 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
				</svg>
				<span>
					<strong class="font-semibold text-amber-900 dark:text-amber-200">BTRC নির্দেশনা:</strong> বিটিআরসি (BTRC) নিয়মানুযায়ী সকল প্রকার প্রমোশনাল ও বাল্ক এসএমএস বাংলায় পাঠানো বাধ্যতামূলক।
				</span>
			</div>
		</div>

		<!-- Action Buttons Row (Teal outlined Send Now + Gray outlined Schedule) -->
		<div class="flex items-center gap-3 pt-2">
			<button
				type="button"
				id="btn-send-now"
				onclick={sendNow}
				disabled={submitting}
				class="inline-flex items-center gap-2 rounded-lg border border-teal-600 bg-white px-5 py-2.5 text-sm font-semibold text-teal-600 shadow-xs transition hover:bg-teal-50 dark:bg-gray-800 dark:hover:bg-gray-700 disabled:opacity-50 cursor-pointer"
			>
				{#if submitting}
					<svg class="h-4 w-4 animate-spin text-teal-600" fill="none" viewBox="0 0 24 24">
						<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
						<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
					</svg>
				{:else}
					<svg class="h-4 w-4 text-teal-600" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M6 12 3.269 3.125A59.769 59.769 0 0 1 21.485 12 59.768 59.768 0 0 1 3.27 20.875L5.999 12Zm0 0h7.5" />
					</svg>
				{/if}
				<span>Send Now</span>
			</button>

			<button
				type="button"
				id="btn-schedule-message"
				onclick={() => (scheduleOpen = true)}
				disabled={submitting}
				class="inline-flex items-center gap-2 rounded-lg border border-gray-300 bg-white px-5 py-2.5 text-sm font-medium text-gray-700 shadow-xs transition hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300 dark:hover:bg-gray-700 disabled:opacity-50 cursor-pointer"
			>
				<svg class="h-4 w-4 text-gray-500 dark:text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
				</svg>
				<span>Schedule Message</span>
			</button>
		</div>
	</div>
</div>

<!-- Schedule Modal -->
<Modal bind:open={scheduleOpen} title="Schedule Quick Message" description="Choose the exact day and timeline when your SMS message should be automatically sent." size="lg">
	<div class="space-y-4">
		<!-- Schedule Banner -->
		<div class="rounded-xl border border-sky-200/80 bg-sky-50/70 p-3.5 text-sky-950 dark:border-sky-800/60 dark:bg-sky-950/40 dark:text-sky-200">
			<div class="flex items-start gap-3">
				<div class="mt-0.5 shrink-0 rounded-lg bg-sky-500/10 p-1.5 text-sky-600 dark:text-sky-400">
					<svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
					</svg>
				</div>
				<div class="text-xs space-y-1">
					<p class="font-semibold text-sm text-sky-900 dark:text-sky-100">Scheduled Dispatch</p>
					<p class="text-sky-800/90 dark:text-sky-300">
						Your SMS will be safely held and will only appear in the live queue when dispatched at your selected date and timeline.
					</p>
					<div class="pt-1 flex items-center gap-2 text-[11px] text-sky-700 dark:text-sky-400 font-mono">
						<span class="inline-flex items-center gap-1 rounded bg-sky-100 dark:bg-sky-900/60 px-1.5 py-0.5 font-medium">
							🌐 Asia/Dhaka
						</span>
						<span>Current: <strong>{new Date().toLocaleDateString('en-GB', { day: '2-digit', month: 'short', year: 'numeric' })}, {new Date().toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', hour12: true })}</strong></span>
					</div>
				</div>
			</div>
		</div>

		<!-- Date Picker -->
		<div>
			<label class="mb-1.5 block text-sm font-semibold text-gray-900 dark:text-gray-100" for="sched-date">
				Select Day <span class="text-red-500">*</span>
			</label>
			<div class="relative">
				<div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5 text-gray-400">
					<svg class="h-5 w-5 text-gray-400 dark:text-gray-500" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M6.75 3v2.25M17.25 3v2.253 3.75m3 0a1.5 1.5 0 0 1-1.5-1.5h-15a1.5 1.5 0 0 1-1.5 1.5v12a2.25 2.25 0 0 0 2.25 2.25h15a2.25 2.25 0 0 0 2.25-2.25V7.5a1.5 1.5 0 0 1-1.5-1.5Z" />
					</svg>
				</div>
				<input
					id="sched-date"
					type="date"
					bind:value={schedDate}
					min={new Date().toISOString().slice(0, 10)}
					class="w-full rounded-lg border border-gray-300 bg-white py-2.5 pl-11 pr-4 text-sm text-gray-900 transition focus:border-teal-600 focus:outline-hidden focus:ring-1 focus:ring-teal-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
				/>
			</div>
		</div>

		<!-- Time Grid -->
		<div class="grid grid-cols-3 gap-3">
			<div>
				<label class="mb-1.5 block text-sm font-semibold text-gray-900 dark:text-gray-100" for="sched-hour">Hour <span class="text-red-500">*</span></label>
				<select
					id="sched-hour"
					bind:value={schedHour}
					class="w-full rounded-lg border border-gray-300 bg-white p-2.5 text-sm text-gray-900 focus:border-teal-600 focus:outline-hidden focus:ring-1 focus:ring-teal-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
				>
					{#each hours as h}
						<option value={h}>{h}</option>
					{/each}
				</select>
			</div>

			<div>
				<label class="mb-1.5 block text-sm font-semibold text-gray-900 dark:text-gray-100" for="sched-minute">Minute <span class="text-red-500">*</span></label>
				<select
					id="sched-minute"
					bind:value={schedMinute}
					class="w-full rounded-lg border border-gray-300 bg-white p-2.5 text-sm text-gray-900 focus:border-teal-600 focus:outline-hidden focus:ring-1 focus:ring-teal-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
				>
					{#each minutes as m}
						<option value={m}>{m}</option>
					{/each}
				</select>
			</div>

			<div>
				<span class="mb-1.5 block text-sm font-semibold text-gray-900 dark:text-gray-100">AM / PM <span class="text-red-500">*</span></span>
				<div class="flex items-center gap-1 rounded-lg border border-gray-300 p-1 dark:border-gray-700 bg-gray-50 dark:bg-gray-800">
					<button
						type="button"
						onclick={() => (schedPeriod = 'AM')}
						class="flex-1 rounded-md py-1.5 text-xs font-bold transition {schedPeriod === 'AM' ? 'bg-sky-600 text-white shadow-xs' : 'text-gray-600 hover:text-gray-900 dark:text-gray-300'}"
					>
						AM
					</button>
					<button
						type="button"
						onclick={() => (schedPeriod = 'PM')}
						class="flex-1 rounded-md py-1.5 text-xs font-bold transition {schedPeriod === 'PM' ? 'bg-amber-600 text-white shadow-xs' : 'text-gray-600 hover:text-gray-900 dark:text-gray-300'}"
					>
						PM
					</button>
				</div>
			</div>
		</div>

		<!-- Modal Actions -->
		<div class="flex items-center justify-end gap-2.5 pt-4 border-t border-gray-100 dark:border-gray-800">
			<button
				type="button"
				onclick={() => (scheduleOpen = false)}
				class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-xs font-medium text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300"
			>
				Cancel
			</button>
			<button
				type="button"
				onclick={scheduleMessage}
				disabled={submitting}
				class="inline-flex items-center gap-2 rounded-lg bg-teal-600 px-5 py-2 text-xs font-semibold text-white shadow-xs hover:bg-teal-500 disabled:opacity-50 cursor-pointer"
			>
				{#if submitting}
					<svg class="h-3.5 w-3.5 animate-spin" fill="none" viewBox="0 0 24 24">
						<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
						<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
					</svg>
				{/if}
				<span>Confirm & Schedule</span>
			</button>
		</div>
	</div>
</Modal>
