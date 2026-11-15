<script lang="ts">
	import { api, ApiError } from '$lib/api.js';
	import { auth } from '$lib/stores/auth.svelte.js';
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

	interface Group {
		id: number;
		name: string;
		contacts_count?: number;
	}

	interface PricingRate {
		operator_name?: string;
		name?: string;
		masking_rate: number | null;
		non_masking_rate: number | null;
	}

	type SourceType = 'manual' | 'group' | 'upload';

	interface ManualRecipientRow {
		phone: string;
		values: Record<string, string>;
	}

	// State
	let senders = $state<Sender[]>([]);
	let selectedSenderId = $state<number | null>(null);
	let source = $state<SourceType>('manual');
	let templates = $state<Template[]>([]);
	let selectedTemplateId = $state<number | null>(null);
	let groups = $state<Group[]>([]);
	let selectedGroupIds = $state<number[]>([]);
	let recentRecipients = $state<string[]>([]);
	let body = $state('');
	let userBalance = $state(0);

	// Manual source: single/multiple phone tags (when no variables)
	let phones = $state<string[]>([]);
	let phoneInput = $state('');

	// Manual source: repeater table (when variables exist)
	let manualRows = $state<ManualRecipientRow[]>([{ phone: '', values: {} }]);

	// Upload source
	let uploadFile = $state<File | null>(null);
	let tempPath = $state('');
	let fileHeaders = $state<string[]>([]);
	let fileSample = $state<Record<string, string>[]>([]);
	let fileTotalRows = $state(0);
	let hasHeaderRow = $state(true);
	let uploadingPreview = $state(false);
	let dedup = $state(true);
	let phoneColumn = $state('');
	let variableMappings = $state<Record<string, string>>({});

	// Rates
	let nonMaskingRates = $state<number[]>([]);
	let maskingRates = $state<number[]>([]);

	// Modals & submittals
	let submitting = $state(false);
	let confirmModalOpen = $state(false);
	let scheduleOpen = $state(false);

	// Schedule form state
	let schedDate = $state(new Date().toISOString().slice(0, 10));
	let schedHour = $state(String(new Date().getHours() % 12 || 12).padStart(2, '0'));
	let schedMinute = $state(String((new Date().getMinutes() + 10) % 60).padStart(2, '0'));
	let schedPeriod = $state<'AM' | 'PM'>(new Date().getHours() >= 12 ? 'PM' : 'AM');

	// Derived: template variables like {name}, {code}
	let bodyVariables = $derived([...new Set([...body.matchAll(/\{(\w+)\}/g)].map((m) => m[1]))]);
	let smsStats = $derived(computeSmsStats(body));

	function messageMatchesTemplate(templateBody: string, messageBody: string): boolean {
		const normT = templateBody.trim().replace(/\r\n?/g, '\n');
		const normM = messageBody.trim().replace(/\r\n?/g, '\n');
		if (!normT || !normM) return false;
		const parts = normT.split(/(\{\w+\})/g);
		const pattern = parts
			.map((p) => (/^\{\w+\}$/.test(p) ? '[\\s\\S]*?' : p.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
			.join('');
		try {
			return new RegExp(`^${pattern}$`).test(normM);
		} catch {
			return normT === normM;
		}
	}

	let selectedTemplate = $derived(templates.find((t) => t.id === selectedTemplateId) ?? null);
	let approvedTemplateActive = $derived(
		Boolean(selectedTemplate && selectedTemplate.approval === 'approved' && messageMatchesTemplate(selectedTemplate.body, body))
	);
	let accountNeedsApproval = $derived(auth.user?.sms_auto_approved === false);

	let mappedSampleMessage = $derived.by(() => {
		if (!body.trim() || fileSample.length === 0) return '';
		const firstRow = fileSample[0];
		return body.replace(/\{(\w+)\}/g, (full, v) => {
			const col = variableMappings[v];
			return col && firstRow[col] !== undefined && firstRow[col] !== '' ? firstRow[col] : full;
		});
	});

	// Sender selected object
	let selectedSender = $derived(senders.find((s) => s.id === selectedSenderId));
	let isMasking = $derived.by(() => {
		const t = selectedSender?.type?.toLowerCase().trim() ?? '';
		return t !== '' && t !== 'numeric' && t !== 'non_masking' && t !== 'non-masking' && t !== 'number';
	});

	// Active rate per SMS
	let activeRate = $derived.by(() => {
		const list = isMasking ? maskingRates : nonMaskingRates;
		if (list.length > 0) {
			return Math.max(...list);
		}
		// No resolved rate means it is not safe to invent one; keep the estimate at zero
		// and let the server perform the authoritative affordability check.
		return 0;
	});

	// Recipient count estimation
	let estimatedRecipientCount = $derived.by(() => {
		if (source === 'manual') {
			if (bodyVariables.length === 0) {
				return phones.length;
			}
			return manualRows.filter((r) => r.phone.trim()).length;
		}
		if (source === 'group') {
			let total = 0;
			for (const gid of selectedGroupIds) {
				const g = groups.find((grp) => grp.id === gid);
				total += g?.contacts_count ?? 1; // estimate at least 1 per group
			}
			return total;
		}
		if (source === 'upload') {
			return fileTotalRows;
		}
		return 0;
	});

	// Cost calculations for confirmation dialog
	let estimatedCost = $derived(
		Math.round(activeRate * (smsStats.parts || 1) * estimatedRecipientCount * 10000) / 10000
	);
	let totalSmsCount = $derived(estimatedRecipientCount * (smsStats.parts || 1));
	let balanceAfter = $derived(Math.max(0, Math.round((userBalance - estimatedCost) * 10000) / 10000));

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

	async function loadTemplates() {
		try {
			const res = await api.get<{ data: Template[] }>('/api/v1/customer/templates');
			templates = res.data ?? [];
		} catch {
			templates = [];
		}
	}

	async function loadGroups() {
		try {
			const res = await api.get<{ data: Group[] }>('/api/v1/customer/groups');
			groups = res.data ?? [];
		} catch {
			groups = [];
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

	async function loadPricing() {
		try {
			const res = await api.get<{ data: PricingRate[] }>('/api/v1/customer/pricing');
			if (res.data && res.data.length > 0) {
				nonMaskingRates = res.data.map((r) => r.non_masking_rate).filter((p): p is number => typeof p === 'number' && p > 0);
				maskingRates = res.data.map((r) => r.masking_rate).filter((p): p is number => typeof p === 'number' && p > 0);
			}
		} catch {
			// Fallbacks used
		}
	}

	async function loadBalance() {
		try {
			const res = await api.get<{ data: { balance: number } }>('/api/v1/customer/dashboard/stats');
			userBalance = res.data?.balance ?? 0;
		} catch {
			userBalance = 0;
		}
	}

	onMount(() => {
		loadSenders();
		loadTemplates();
		loadGroups();
		loadRecentRecipients();
		loadPricing();
		loadBalance();
		if (page.url.searchParams.get('schedule') === '1') {
			scheduleOpen = true;
		}
	});

	// Phone tags helpers
	function addPhone() {
		const raw = phoneInput.trim();
		if (!raw) return;
		const nums = raw
			.split(/[\n,;\s]+/)
			.map((p) => p.trim())
			.filter(Boolean);
		const set = new Set(phones);
		for (const n of nums) {
			set.add(n);
		}
		phones = Array.from(set);
		phoneInput = '';
	}

	function removePhone(p: string) {
		phones = phones.filter((item) => item !== p);
	}

	function handlePhoneKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' || e.key === ',') {
			e.preventDefault();
			addPhone();
		}
	}

	function addRecentRecipient(num: string) {
		num = num.trim();
		if (!num) return;
		if (bodyVariables.length === 0) {
			if (!phones.includes(num)) {
				phones = [...phones, num];
			}
		} else {
			manualRows = [...manualRows, { phone: num, values: {} }];
		}
	}

	// Repeater helpers
	function addManualRow() {
		manualRows = [...manualRows, { phone: '', values: {} }];
	}

	function removeManualRow(index: number) {
		manualRows = manualRows.filter((_, i) => i !== index);
	}

	// Template select
	function handleTemplateChange(e: Event) {
		const target = e.target as HTMLSelectElement;
		const tid = Number(target.value);
		if (!tid) return;
		const tmpl = templates.find((t) => t.id === tid);
		if (tmpl) {
			body = tmpl.body;
		}
	}

	function syncAutoMappings(headers: string[]) {
		const detected = headers.find((h) => /phone|mobile|number|msisdn|contact/i.test(h));
		phoneColumn = detected ?? headers[0] ?? '';

		const nextMap: Record<string, string> = {};
		for (const v of bodyVariables) {
			if (variableMappings[v] && headers.includes(variableMappings[v])) {
				nextMap[v] = variableMappings[v];
			} else {
				const exact = headers.find((h) => h.toLowerCase() === v.toLowerCase());
				nextMap[v] = exact ?? '';
			}
		}
		variableMappings = nextMap;
	}

	$effect(() => {
		if (fileHeaders.length === 0) return;
		let changed = false;
		const nextMap = { ...variableMappings };
		for (const v of bodyVariables) {
			if (!(v in nextMap)) {
				const exact = fileHeaders.find((h) => h.toLowerCase() === v.toLowerCase());
				nextMap[v] = exact ?? '';
				changed = true;
			}
		}
		if (changed) variableMappings = nextMap;
	});

	// Upload Preview via backend parser (.csv, .txt, .xlsx)
	async function previewUploadedFile(file: File, withHeader = hasHeaderRow) {
		uploadingPreview = true;
		try {
			const fd = new FormData();
			fd.append('file', file);
			fd.append('has_header_row', withHeader ? '1' : '0');
			const res = await api.upload<{
				data: {
					temp_path: string;
					headers: string[];
					sample: Record<string, string>[];
					total_rows: number;
					has_header_row: boolean;
				};
			}>('/api/v1/customer/messages/upload-preview', fd);

			tempPath = res.data.temp_path;
			fileHeaders = res.data.headers ?? [];
			fileSample = res.data.sample ?? [];
			fileTotalRows = res.data.total_rows ?? 0;
			hasHeaderRow = res.data.has_header_row ?? withHeader;
			syncAutoMappings(fileHeaders);
			toast.success('File loaded', `Detected ${fileTotalRows} rows.`);
		} catch (e) {
			tempPath = '';
			fileHeaders = [];
			fileSample = [];
			fileTotalRows = 0;
			const msg = e instanceof ApiError ? String(e.data?.message ?? 'Could not parse uploaded file.') : 'Could not upload file preview.';
			toast.error('Upload error', msg);
		} finally {
			uploadingPreview = false;
		}
	}

	async function handleFileUpload(e: Event) {
		const file = (e.target as HTMLInputElement).files?.[0];
		if (!file) return;
		uploadFile = file;
		await previewUploadedFile(file, hasHeaderRow);
	}

	// Validation
	function validateForm(): boolean {
		if (!selectedSenderId) {
			toast.error('Sender ID required', 'Please select a Sender ID before sending.');
			return false;
		}
		if (!body.trim()) {
			toast.error('Message required', 'Message body cannot be empty.');
			return false;
		}

		if (source === 'manual') {
			if (bodyVariables.length === 0) {
				if (phones.length === 0) {
					toast.error('Recipients required', 'Please enter at least one recipient phone number.');
					return false;
				}
			} else {
				const valid = manualRows.filter((r) => r.phone.trim());
				if (valid.length === 0) {
					toast.error('Recipients required', 'Please enter at least one valid recipient in the table.');
					return false;
				}
			}
		} else if (source === 'group') {
			if (selectedGroupIds.length === 0) {
				toast.error('Groups required', 'Please select at least one contact group.');
				return false;
			}
		} else if (source === 'upload') {
			if (!uploadFile || !tempPath || fileTotalRows === 0) {
				toast.error('File required', 'Please upload a valid recipient file.');
				return false;
			}
			if (!phoneColumn) {
				toast.error('Mapping required', 'Please select which column contains phone numbers.');
				return false;
			}
			const missingVar = bodyVariables.find((v) => !variableMappings[v]);
			if (missingVar) {
				toast.error('Mapping required', `Please select a file column for template variable {${missingVar}}.`);
				return false;
			}
		}

		return true;
	}

	function handleSendNowClick() {
		if (!validateForm()) return;
		confirmModalOpen = true;
	}

	function handleScheduleClick() {
		if (!validateForm()) return;
		scheduleOpen = true;
	}

	function buildBulkPayload(): Record<string, unknown> {
		const payload: Record<string, unknown> = {
			sender_id: selectedSenderId,
			template_id: selectedTemplateId,
			body: body.trim(),
			source
		};

		if (source === 'manual') {
			if (bodyVariables.length === 0) {
				payload.phones = phones;
			} else {
				payload.recipients = manualRows
					.filter((r) => r.phone.trim())
					.map((r) => ({ phone: r.phone.trim(), values: r.values }));
			}
		} else if (source === 'group') {
			payload.groups = selectedGroupIds;
			payload.dedup = dedup;
		} else if (source === 'upload') {
			payload.temp_path = tempPath;
			payload.file_total_rows = fileTotalRows;
			payload.has_header_row = hasHeaderRow;
			payload.dedup = dedup;
			const vars: Record<string, string> = {};
			for (const v of bodyVariables) {
				if (variableMappings[v]) vars[v] = variableMappings[v];
			}
			payload.mapping = {
				phone: phoneColumn,
				variables: vars
			};
		}

		return payload;
	}

	async function submitSendNow() {
		submitting = true;
		try {
			const res = await api.post<{ data: { id: number; requires_approval?: boolean } }>(
				'/api/v1/customer/messages/bulk-send',
				buildBulkPayload()
			);
			if (res.data?.requires_approval) {
				toast.warning(
					'Batch queued for Admin Approval',
					'Your bulk SMS batch has been submitted and awaits admin approval before dispatch.'
				);
			} else {
				toast.success('Message sent successfully', 'Your bulk message batch has been dispatched.');
			}
			confirmModalOpen = false;
			goto(`/messaging/batches/${res.data.id}`);
		} catch (e) {
			if (e instanceof ApiError) {
				toast.error('Send failed', (e.data.message as string) ?? 'Please try again.');
			} else {
				toast.error('Network error', 'Could not dispatch message.');
			}
		} finally {
			submitting = false;
		}
	}

	async function submitSchedule() {
		submitting = true;
		try {
			const payload = {
				...buildBulkPayload(),
				scheduled_date: schedDate,
				scheduled_hour: schedHour,
				scheduled_minute: schedMinute,
				scheduled_period: schedPeriod
			};

			const res = await api.post<{ data: { id: number; requires_approval?: boolean } }>(
				'/api/v1/customer/messages/bulk-send',
				payload
			);
			if (res.data?.requires_approval) {
				toast.warning(
					'Scheduled & Awaiting Approval',
					`Scheduled for ${schedDate} at ${schedHour}:${schedMinute} ${schedPeriod}, pending admin approval.`
				);
			} else {
				toast.success('Batch scheduled successfully', `Your bulk message batch has been scheduled for ${schedDate} at ${schedHour}:${schedMinute} ${schedPeriod}.`);
			}
			scheduleOpen = false;
			if (res.data?.id) {
				goto(`/messaging/batches/${res.data.id}`);
			} else {
				goto('/messaging/scheduled-messages');
			}
		} catch (e) {
			if (e instanceof ApiError) {
				toast.error('Schedule failed', (e.data.message as string) ?? 'Please try again.');
			} else {
				toast.error('Network error', 'Could not schedule message.');
			}
		} finally {
			submitting = false;
		}
	}

	const hours = Array.from({ length: 12 }, (_, i) => String(i + 1).padStart(2, '0'));
	const minutes = Array.from({ length: 60 }, (_, i) => String(i).padStart(2, '0'));
</script>

<svelte:head>
	<title>Bulk Message - White INFO</title>
</svelte:head>

<div class="mx-auto max-w-5xl space-y-6">
	<!-- Breadcrumb -->
	<nav class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
		<a href="/messaging/messages" class="hover:text-gray-800 dark:hover:text-gray-200 transition">Sent Message Details</a>
		<svg class="h-3 w-3 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
			<path stroke-linecap="round" stroke-linejoin="round" d="M8.25 4.5l7.5 7.5-7.5 7.5" />
		</svg>
		<span class="text-gray-900 dark:text-gray-100 font-medium">Bulk Message</span>
	</nav>

	<!-- Page Heading -->
	<div>
		<h1 class="text-2xl font-bold tracking-tight text-gray-900 dark:text-gray-100">Bulk Message</h1>
	</div>

	{#if accountNeedsApproval}
		{#if approvedTemplateActive}
			<div class="flex items-start gap-3 rounded-xl border border-emerald-200 bg-emerald-50 p-4 text-xs text-emerald-900 dark:border-emerald-900/60 dark:bg-emerald-950/40 dark:text-emerald-200">
				<svg class="h-5 w-5 shrink-0 text-emerald-600 dark:text-emerald-400 mt-0.5" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
				</svg>
				<div>
					<p class="font-semibold text-sm">Approved Template Selected — Auto-Dispatch Enabled</p>
					<p class="mt-0.5 text-emerald-800/90 dark:text-emerald-300">
						This message matches an approved template and will be sent immediately without waiting for admin review.
					</p>
				</div>
			</div>
		{:else}
			<div class="flex items-start gap-3 rounded-xl border border-amber-200 bg-amber-50 p-4 text-xs text-amber-900 dark:border-amber-900/60 dark:bg-amber-950/40 dark:text-amber-200">
				<svg class="h-5 w-5 shrink-0 text-amber-600 dark:text-amber-400 mt-0.5" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m9-.75a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 3.75h.008v.008H12v-.008Z" />
				</svg>
				<div>
					<p class="font-semibold text-sm">Admin Approval Required for Bulk SMS</p>
					<p class="mt-0.5 text-amber-800/90 dark:text-amber-300">
						Your account requires manual admin approval for bulk campaigns unless you select an approved Bulk SMS template and keep its fixed text unchanged.
					</p>
				</div>
			</div>
		{/if}
	{/if}

	<!-- Main Form Card -->
	<div class="rounded-xl border border-gray-200 bg-white p-6 shadow-xs dark:border-gray-800 dark:bg-gray-900 space-y-6">
		<!-- Row 1: Sender ID & Source (2 Columns) -->
		<div class="grid grid-cols-1 md:grid-cols-2 gap-5">
			<!-- Sender ID (Left) -->
			<div>
				<label class="mb-1.5 block text-sm font-semibold text-gray-900 dark:text-gray-100" for="bulk-sender-id">
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
						id="bulk-sender-id"
						bind:value={selectedSenderId}
						class="w-full rounded-lg border border-gray-300 bg-white py-2.5 pl-11 pr-10 text-sm text-gray-900 transition focus:border-teal-600 focus:outline-hidden focus:ring-1 focus:ring-teal-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
					>
						<option value={null} disabled>Select a Sender ID</option>
						{#each senders as s (s.id)}
							<option value={s.id}>{s.sender_id}</option>
						{/each}
					</select>
				</div>
			</div>

			<!-- Source (Right) -->
			<div>
				<label class="mb-1.5 block text-sm font-semibold text-gray-900 dark:text-gray-100" for="bulk-source">
					Source <span class="text-red-500">*</span>
				</label>
				<div class="relative">
					<div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5 text-gray-400">
						<!-- Heroicon: Inbox -->
						<svg class="h-5 w-5 text-gray-400 dark:text-gray-500" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" d="M2.25 13.5h3.86a2.25 2.25 0 0 1 2.012 1.244l.256.512a2.25 2.25 0 0 0 2.013 1.244h3.218a2.25 2.25 0 0 0 2.013-1.244l.256-.512a2.25 2.25 0 0 1 2.013-1.244h3.859m-19.5.375V19.5a2.25 2.25 0 0 0 2.25 2.25h15a2.25 2.25 0 0 0 2.25-2.25v-5.625m-19.5 0v-6.75a2.25 2.25 0 0 1 2.25-2.25h15a2.25 2.25 0 0 1 2.25 2.25v6.75" />
						</svg>
					</div>
					<select
						id="bulk-source"
						bind:value={source}
						class="w-full rounded-lg border border-gray-300 bg-white py-2.5 pl-11 pr-10 text-sm text-gray-900 transition focus:border-teal-600 focus:outline-hidden focus:ring-1 focus:ring-teal-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
					>
						<option value="manual">Manual</option>
						<option value="group">Group</option>
						<option value="upload">Upload</option>
					</select>
				</div>
			</div>
		</div>

		<!-- Dynamic Recipients Area based on Source -->
		{#if source === 'manual'}
			{#if bodyVariables.length === 0}
				<!-- Manual: Phone Numbers Tag Input -->
				<div>
					<label class="mb-1.5 block text-sm font-semibold text-gray-900 dark:text-gray-100" for="bulk-phones-input">
						Phone Numbers <span class="text-red-500">*</span>
					</label>

					<!-- Tags Chips -->
					{#if phones.length > 0}
						<div class="mb-2.5 flex flex-wrap gap-1.5">
							{#each phones as p (p)}
								<span class="inline-flex items-center gap-1 rounded-md bg-teal-50 border border-teal-200 px-2.5 py-1 text-xs font-mono font-medium text-teal-800 dark:border-teal-800/40 dark:bg-teal-950/40 dark:text-teal-300">
									{p}
									<button
										type="button"
										onclick={() => removePhone(p)}
										class="text-teal-600 hover:text-red-500 transition dark:text-teal-400 cursor-pointer"
										aria-label="Remove {p}"
									>
										<svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
											<path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" />
										</svg>
									</button>
								</span>
							{/each}
						</div>
					{/if}

					<!-- Input row with Bangladesh flag -->
					<div class="relative flex items-center">
						<div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5">
							<svg class="h-4 w-5 rounded-xs shrink-0 overflow-hidden shadow-xs" viewBox="0 0 20 12">
								<rect width="20" height="12" fill="#006a4e" />
								<circle cx="9" cy="6" r="4" fill="#f42a41" />
							</svg>
						</div>
						<input
							id="bulk-phones-input"
							type="tel"
							bind:value={phoneInput}
							onkeydown={handlePhoneKeydown}
							onblur={addPhone}
							placeholder="Input Phone Numbers"
							class="w-full rounded-lg border border-gray-300 bg-white py-2.5 pl-11 pr-20 text-sm text-gray-900 placeholder-gray-400 transition focus:border-teal-600 focus:outline-hidden focus:ring-1 focus:ring-teal-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100 dark:placeholder-gray-500"
						/>
						<button
							type="button"
							onclick={addPhone}
							class="absolute right-2 rounded-md bg-gray-100 px-3 py-1.5 text-xs font-medium text-gray-700 hover:bg-gray-200 transition dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600 cursor-pointer"
						>
							Add
						</button>
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
									onclick={() => addRecentRecipient(rec)}
									class="inline-flex items-center gap-1 rounded-full border border-gray-200 bg-white px-2.5 py-0.5 text-[11px] font-mono text-gray-700 transition hover:border-teal-500 hover:text-teal-600 cursor-pointer dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300 dark:hover:border-teal-400 dark:hover:text-teal-300"
								>
									<span class="font-bold text-teal-600 dark:text-teal-400">+</span>
									<span>{rec}</span>
								</button>
							{/each}
						</div>
					{/if}
				</div>
			{:else}
				<!-- Manual: Repeater Table when body variables exist -->
				<div>
					<label class="mb-1.5 block text-sm font-semibold text-gray-900 dark:text-gray-100">
						Recipients <span class="text-red-500">*</span>
					</label>
					<div class="overflow-x-auto rounded-lg border border-gray-200 dark:border-gray-700">
						<table class="w-full text-left text-xs">
							<thead class="bg-gray-50 text-gray-600 uppercase font-semibold dark:bg-gray-800/80 dark:text-gray-300 border-b border-gray-200 dark:border-gray-700">
								<tr>
									<th class="px-4 py-2.5">Phone <span class="text-red-500">*</span></th>
									{#each bodyVariables as v (v)}
										<th class="px-4 py-2.5">{v} <span class="text-red-500">*</span></th>
									{/each}
									<th class="w-12 px-2 py-2.5 text-center"></th>
								</tr>
							</thead>
							<tbody class="divide-y divide-gray-100 dark:divide-gray-800 bg-white dark:bg-gray-900">
								{#each manualRows as row, i (i)}
									<tr>
										<td class="p-2">
											<input
												type="tel"
												bind:value={row.phone}
												placeholder="Type phone number"
												class="w-full rounded-md border border-gray-300 bg-white px-3 py-1.5 text-xs text-gray-900 focus:border-teal-600 focus:outline-hidden dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
											/>
										</td>
										{#each bodyVariables as v (v)}
											<td class="p-2">
												<input
													type="text"
													bind:value={row.values[v]}
													placeholder="Value for {v}"
													class="w-full rounded-md border border-gray-300 bg-white px-3 py-1.5 text-xs text-gray-900 focus:border-teal-600 focus:outline-hidden dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
												/>
											</td>
										{/each}
										<td class="p-2 text-center">
											<button
												type="button"
												onclick={() => removeManualRow(i)}
												class="text-gray-400 hover:text-red-500 transition cursor-pointer"
												aria-label="Remove row"
											>
												<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
													<path stroke-linecap="round" stroke-linejoin="round" d="m14.74 9-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0" />
												</svg>
											</button>
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
					<button
						type="button"
						onclick={addManualRow}
						class="mt-2 inline-flex items-center gap-1.5 text-xs font-semibold text-teal-600 hover:text-teal-700 dark:text-teal-400 transition cursor-pointer"
					>
						<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
						</svg>
						<span>Add to Recipients</span>
					</button>
				</div>
			{/if}
		{:else if source === 'group'}
			<!-- Group Selection -->
			<div>
				<label class="mb-1.5 block text-sm font-semibold text-gray-900 dark:text-gray-100" for="bulk-groups">
					Groups <span class="text-red-500">*</span>
				</label>
				<select
					id="bulk-groups"
					multiple
					bind:value={selectedGroupIds}
					class="w-full rounded-lg border border-gray-300 bg-white p-3 text-sm text-gray-900 transition focus:border-teal-600 focus:outline-hidden focus:ring-1 focus:ring-teal-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
				>
					{#each groups as g (g.id)}
						<option value={g.id}>{g.name} ({g.contacts_count ?? 0} contacts)</option>
					{/each}
				</select>
				<p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
					Hold Ctrl (Windows) or Cmd (Mac) to select multiple contact groups.
				</p>

				{#if bodyVariables.length > 0}
					<!-- Callout warning: variables not supported for groups -->
					<div class="mt-3 flex items-start gap-2.5 rounded-lg border border-amber-500/25 bg-amber-500/10 p-3 text-xs text-amber-800 dark:text-amber-300 leading-relaxed">
						<svg class="h-4 w-4 shrink-0 text-amber-600 dark:text-amber-400 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
						</svg>
						<div>
							<p class="font-semibold text-amber-900 dark:text-amber-200">Variables are not supported for group messages.</p>
							<p class="mt-0.5 text-amber-800 dark:text-amber-300">
								Your message contains placeholders like {bodyVariables.map((v) => '{' + v + '}').join(', ')} — these placeholders will be sent as-is to every contact. Switch to Upload or Manual if you need per-recipient values.
							</p>
						</div>
					</div>
				{/if}
			</div>
		{:else if source === 'upload'}
			<!-- File Upload -->
			<div>
				<label class="mb-1.5 block text-sm font-semibold text-gray-900 dark:text-gray-100" for="bulk-file-upload">
					File <span class="text-red-500">*</span>
				</label>
				<div class="rounded-xl border-2 border-dashed border-gray-300 p-6 text-center hover:border-teal-500 transition dark:border-gray-700 dark:bg-gray-800/40">
					<svg class="mx-auto h-10 w-10 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 0 0-3.375-3.375h-1.5A1.125 1.125 0 0 1 13.5 7.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H8.25m6.75 12H9m1.5-12H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 0 0-9-9Z" />
					</svg>
					<div class="mt-3">
						<label for="bulk-file-upload" class="font-semibold text-teal-600 hover:text-teal-500 cursor-pointer dark:text-teal-400">
							Upload a file
							<input id="bulk-file-upload" type="file" accept=".csv,.txt,.xlsx,.xls" onchange={handleFileUpload} class="sr-only" />
						</label>
						<span class="text-gray-500 dark:text-gray-400"> or drag and drop</span>
					</div>
					<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">CSV, TXT, or Excel (.xlsx) up to 100MB</p>
					{#if uploadingPreview}
						<p class="mt-2 text-xs font-medium text-gray-600 dark:text-gray-300">Parsing uploaded file...</p>
					{:else if uploadFile && tempPath}
						<p class="mt-2 text-xs font-semibold text-teal-700 dark:text-teal-300">
							✓ {uploadFile.name} ({fileTotalRows} rows detected)
						</p>
					{/if}
				</div>

				<!-- Helper text with Download Sample Link & Options -->
				<div class="mt-2 flex flex-wrap items-center justify-between gap-2 text-xs text-gray-500 dark:text-gray-400">
					<span>Upload a CSV or Excel file with a <strong>"phone"</strong> column and any variables used in your message body.</span>
					<a
						href="/bd_phone_numbers.xlsx"
						download
						class="inline-flex items-center gap-1 font-medium text-teal-600 hover:text-teal-500 dark:text-teal-400 underline underline-offset-2 transition"
					>
						⬇ Download Sample File
					</a>
				</div>

				<div class="mt-3 flex flex-wrap items-center gap-5 text-xs text-gray-700 dark:text-gray-300">
					<label class="inline-flex items-center gap-2 cursor-pointer">
						<input
							type="checkbox"
							bind:checked={hasHeaderRow}
							onchange={() => { if (uploadFile) previewUploadedFile(uploadFile, hasHeaderRow); }}
							class="rounded border-gray-300 text-teal-600 focus:ring-teal-500"
						/>
						<span>First row is column header</span>
					</label>
					<label class="inline-flex items-center gap-2 cursor-pointer">
						<input
							type="checkbox"
							bind:checked={dedup}
							class="rounded border-gray-300 text-teal-600 focus:ring-teal-500"
						/>
						<span>Remove duplicate phone numbers</span>
					</label>
				</div>

				<!-- File Preview & Column Mapping -->
				{#if fileHeaders.length > 0}
					<div class="mt-4 space-y-4 rounded-lg border border-gray-200 bg-gray-50 p-4 dark:border-gray-700 dark:bg-gray-800/60">
						<div class="flex items-center justify-between">
							<p class="text-xs font-semibold uppercase tracking-wider text-gray-700 dark:text-gray-300">Upload Preview & Variable Mapping</p>
							<span class="text-xs text-gray-500">{fileTotalRows} total rows</span>
						</div>

						<!-- Phone Column + Dynamic Variable Mappings -->
						<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
							<div>
								<label for="map-phone-col" class="mb-1 block text-xs font-medium text-gray-700 dark:text-gray-300">
									Phone Number Column <span class="text-red-500">*</span>
								</label>
								<select
									id="map-phone-col"
									bind:value={phoneColumn}
									class="w-full rounded-md border border-gray-300 bg-white py-1.5 px-2.5 text-xs text-gray-900 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
								>
									{#each fileHeaders as h (h)}
										<option value={h}>{h}</option>
									{/each}
								</select>
							</div>
							{#each bodyVariables as varName (varName)}
								<div>
									<label for={"map-var-" + varName} class="mb-1 block text-xs font-medium text-gray-700 dark:text-gray-300">
										Column for <span class="font-mono text-teal-700 dark:text-teal-400">{'{' + varName + '}'}</span> <span class="text-red-500">*</span>
									</label>
									<select
										id={"map-var-" + varName}
										bind:value={variableMappings[varName]}
										class="w-full rounded-md border border-gray-300 bg-white py-1.5 px-2.5 text-xs text-gray-900 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
									>
										<option value="">— Select Column —</option>
										{#each fileHeaders as h (h)}
											<option value={h}>{h}</option>
										{/each}
									</select>
								</div>
							{/each}
						</div>

						{#if mappedSampleMessage}
							<div class="rounded-md border border-teal-200 bg-white p-3 dark:border-teal-900/50 dark:bg-gray-900">
								<p class="mb-1 text-[11px] font-semibold uppercase tracking-wider text-teal-700 dark:text-teal-400">
									Sample Message Preview (Row 1)
								</p>
								<p class="font-mono text-xs text-gray-800 dark:text-gray-200 whitespace-pre-wrap">{mappedSampleMessage}</p>
							</div>
						{/if}

						<!-- Sample Rows Table -->
						<div class="overflow-x-auto rounded border border-gray-200 dark:border-gray-700">
							<table class="w-full text-left text-[11px]">
								<thead class="bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300">
									<tr>
										{#each fileHeaders as h (h)}
											<th class="px-3 py-1.5 font-semibold">{h}</th>
										{/each}
									</tr>
								</thead>
								<tbody class="divide-y divide-gray-200 dark:divide-gray-700 bg-white dark:bg-gray-900">
									{#each fileSample as row, idx (idx)}
										<tr>
											{#each fileHeaders as h (h)}
												<td class="px-3 py-1 font-mono text-gray-700 dark:text-gray-300">{row[h] ?? '—'}</td>
											{/each}
										</tr>
									{/each}
								</tbody>
							</table>
						</div>
					</div>
				{/if}
			</div>
		{/if}

		<!-- Use Bulk SMS Template -->
		<div>
			<label class="mb-1.5 block text-sm font-semibold text-gray-900 dark:text-gray-100" for="bulk-template-select">
				Use Bulk SMS Template
			</label>
			<div class="relative">
				<div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5 text-gray-400">
					<!-- Heroicon: DocumentText -->
					<svg class="h-5 w-5 text-gray-400 dark:text-gray-500" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 0 0-3.375-3.375h-1.5A1.125 1.125 0 0 1 13.5 7.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H8.25m0 12.75h7.5m-7.5 3H12M10.5 2.25H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 0 0-9-9Z" />
					</svg>
				</div>
				<select
					id="bulk-template-select"
					bind:value={selectedTemplateId}
					onchange={handleTemplateChange}
					class="w-full rounded-lg border border-gray-300 bg-white py-2.5 pl-11 pr-10 text-sm text-gray-900 transition focus:border-teal-600 focus:outline-hidden focus:ring-1 focus:ring-teal-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
				>
					<option value={null}>— Select a Bulk SMS Template to Autofill Message (Optional) —</option>
					{#each templates as t (t.id)}
						<option value={t.id}>
							{t.name} — {t.approval === 'approved' ? '🟢 [Approved — Bypasses Daily Admin Approval]' : t.approval === 'rejected' ? '🔴 [Rejected]' : '🟡 [Pending Review]'}
						</option>
					{/each}
				</select>
			</div>
			<p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
				Select any of your created Bulk SMS templates to automatically fill your message text without retyping.
			</p>
		</div>

		<!-- Message Textarea -->
		<div>
			<label class="mb-1.5 block text-sm font-semibold text-gray-900 dark:text-gray-100" for="bulk-message-body">
				Message <span class="text-red-500">*</span>
			</label>
			<textarea
				id="bulk-message-body"
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
					<strong class="font-semibold text-amber-900 dark:text-amber-200">BTRC নির্দেশনা:</strong> বিটিআরসি (BTRC) নিয়মানুযায়ী সকল প্রকার প্রমোশনাল ও বাল্ক এসএমএস বাংলায় পাঠানো বাধ্যতামূলক।
				</span>
			</div>
		</div>

		<!-- Action Buttons Row -->
		<div class="flex items-center gap-3 pt-2">
			<button
				type="button"
				id="btn-bulk-send-now"
				onclick={handleSendNowClick}
				disabled={submitting}
				class="inline-flex items-center gap-2 rounded-lg border border-teal-600 bg-white px-5 py-2.5 text-sm font-semibold text-teal-600 shadow-xs transition hover:bg-teal-50 dark:bg-gray-800 dark:hover:bg-gray-700 disabled:opacity-50 cursor-pointer"
			>
				<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="M6 12 3.269 3.125A59.769 59.769 0 0 1 21.485 12 59.768 59.768 0 0 1 3.27 20.875L5.999 12Zm0 0h7.5" />
				</svg>
				<span>Send Now</span>
			</button>

			<button
				type="button"
				id="btn-bulk-schedule"
				onclick={handleScheduleClick}
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

<!-- Double-Check & Confirm SMS Modal -->
<Modal bind:open={confirmModalOpen} title="Double-Check & Confirm SMS" description="Please verify your message content and review the cost breakdown below before sending." size="lg">
	<div class="space-y-4 text-sm text-left">
		<!-- BTRC Alert in Confirmation -->
		<div class="rounded-lg border border-amber-200 dark:border-amber-900 bg-amber-50 dark:bg-amber-950/50 p-3.5 text-amber-800 dark:text-amber-200">
			<div class="flex items-start gap-2.5">
				<svg class="w-5 h-5 shrink-0 text-amber-500 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
				</svg>
				<div>
					<p class="font-semibold text-sm">BTRC নির্দেশনা ও কনফার্মেশন</p>
					<p class="text-xs text-amber-700 dark:text-amber-300 mt-0.5">
						বিটিআরসি (BTRC) নিয়মানুযায়ী সকল প্রমোশনাল বা বাল্ক এসএমএস বাংলায় পাঠানো বাধ্যতামূলক। আপনার প্রেরিত মেসেজ, Sender ID (<strong>{selectedSender?.sender_id ?? 'Not Selected'}</strong>) ও প্রাপকের সংখ্যা যাচাই করে নিশ্চিত করুন।
					</p>
				</div>
			</div>
		</div>

		<!-- Message Preview -->
		<div class="rounded-lg border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900 p-3">
			<p class="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wider mb-1">Message Preview</p>
			<p class="text-xs text-gray-800 dark:text-gray-200 whitespace-pre-wrap font-mono bg-white dark:bg-gray-950 p-2.5 rounded border border-gray-200 dark:border-gray-800 max-h-24 overflow-y-auto">{body}</p>
		</div>

		<!-- 3-Col Stats Cards -->
		<div class="grid grid-cols-3 gap-2">
			<div class="rounded-lg border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900 p-2.5 text-center">
				<p class="text-xs text-gray-500 dark:text-gray-400">Recipients</p>
				<p class="mt-0.5 text-sm font-bold text-gray-900 dark:text-gray-100">{estimatedRecipientCount}</p>
			</div>
			<div class="rounded-lg border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900 p-2.5 text-center">
				<p class="text-xs text-gray-500 dark:text-gray-400">SMS Count</p>
				<p class="mt-0.5 text-sm font-bold text-teal-600 dark:text-teal-400">{smsStats.parts || 1}</p>
			</div>
			<div class="rounded-lg border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900 p-2.5 text-center">
				<p class="text-xs text-gray-500 dark:text-gray-400">Rate / SMS</p>
				<p class="mt-0.5 text-sm font-bold text-gray-900 dark:text-gray-100">৳{activeRate.toFixed(2)}</p>
			</div>
		</div>

		<!-- Cost Breakdown Table -->
		<div class="rounded-lg border border-gray-200 dark:border-gray-700 divide-y divide-gray-200 dark:divide-gray-700 overflow-hidden text-xs">
			<div class="flex items-center justify-between px-3.5 py-2.5 bg-white dark:bg-gray-950">
				<span class="text-gray-600 dark:text-gray-400">Previous balance:</span>
				<span class="font-semibold text-gray-900 dark:text-gray-100">BDT {userBalance.toFixed(2)}</span>
			</div>
			<div class="flex items-center justify-between px-3.5 py-2.5 bg-white dark:bg-gray-950">
				<span class="text-gray-600 dark:text-gray-400">Total SMS:</span>
				<span class="font-semibold text-teal-600 dark:text-teal-400">{totalSmsCount} SMS ({estimatedRecipientCount} recipients × {smsStats.parts || 1} SMS)</span>
			</div>
			<div class="flex items-center justify-between px-3.5 py-2.5 bg-white dark:bg-gray-950">
				<span class="text-gray-600 dark:text-gray-400">Total cost ({totalSmsCount} SMS × ৳{activeRate.toFixed(2)}):</span>
				<span class="font-bold text-red-600 dark:text-red-400">− BDT {estimatedCost.toFixed(2)}</span>
			</div>
			<div class="flex items-center justify-between px-3.5 py-2.5 bg-gray-50 dark:bg-gray-900 font-semibold">
				<span class="text-gray-900 dark:text-gray-100">Balance after sending:</span>
				<span class="text-emerald-700 dark:text-emerald-400 text-sm">BDT {balanceAfter.toFixed(2)}</span>
			</div>
		</div>

		<!-- Modal Action Buttons -->
		<div class="flex justify-end gap-3 pt-3 border-t border-gray-100 dark:border-gray-800">
			<button
				type="button"
				onclick={() => (confirmModalOpen = false)}
				class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 transition hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300 cursor-pointer"
			>
				No, let me double-check
			</button>
			<button
				type="button"
				id="btn-confirm-send-now"
				onclick={submitSendNow}
				disabled={submitting}
				class="inline-flex items-center gap-2 rounded-lg bg-teal-600 px-5 py-2 text-sm font-semibold text-white shadow-xs transition hover:bg-teal-500 disabled:opacity-50 cursor-pointer"
			>
				{#if submitting}
					<svg class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
						<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
						<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
					</svg>
				{/if}
				<span>Yes, Send Now</span>
			</button>
		</div>
	</div>
</Modal>

<!-- Schedule Bulk Message Modal -->
<Modal bind:open={scheduleOpen} title="Schedule Bulk Message" description="Choose the exact day and timeline when this bulk message batch should be automatically sent." size="lg">
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
						Your SMS batch will be safely held and will only appear in the live queue when dispatched at your selected date and timeline.
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
			<label class="mb-1.5 block text-sm font-semibold text-gray-900 dark:text-gray-100" for="bulk-sched-date">
				Select Day <span class="text-red-500">*</span>
			</label>
			<div class="relative">
				<div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5 text-gray-400">
					<svg class="h-5 w-5 text-gray-400 dark:text-gray-500" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M6.75 3v2.25M17.25 3v2.253 3.75m3 0a1.5 1.5 0 0 1-1.5-1.5h-15a1.5 1.5 0 0 1-1.5 1.5v12a2.25 2.25 0 0 0 2.25 2.25h15a2.25 2.25 0 0 0 2.25-2.25V7.5a1.5 1.5 0 0 1-1.5-1.5Z" />
					</svg>
				</div>
				<input
					id="bulk-sched-date"
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
				<label class="mb-1.5 block text-sm font-semibold text-gray-900 dark:text-gray-100" for="bulk-sched-hour">Hour</label>
				<div class="relative">
					<div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3 text-gray-400">
						<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
						</svg>
					</div>
					<select
						id="bulk-sched-hour"
						bind:value={schedHour}
						class="w-full rounded-lg border border-gray-300 bg-white py-2.5 pl-9 pr-8 text-sm text-gray-900 transition focus:border-teal-600 focus:outline-hidden dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
					>
						{#each hours as h (h)}<option value={h}>{h}</option>{/each}
					</select>
				</div>
			</div>
			<div>
				<label class="mb-1.5 block text-sm font-semibold text-gray-900 dark:text-gray-100" for="bulk-sched-min">Minute</label>
				<select
					id="bulk-sched-min"
					bind:value={schedMinute}
					class="w-full rounded-lg border border-gray-300 bg-white py-2.5 px-3 text-sm text-gray-900 transition focus:border-teal-600 focus:outline-hidden dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
				>
					{#each minutes as m (m)}<option value={m}>{m}</option>{/each}
				</select>
			</div>
			<div>
				<span class="mb-1.5 block text-sm font-semibold text-gray-900 dark:text-gray-100">AM / PM</span>
				<div class="flex gap-2">
					<button
						type="button"
						onclick={() => (schedPeriod = 'AM')}
						class="flex-1 rounded-lg py-2 text-sm font-semibold transition cursor-pointer {schedPeriod === 'AM'
							? 'bg-sky-600 text-white shadow-xs'
							: 'border border-gray-300 bg-white text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300'}"
					>
						AM
					</button>
					<button
						type="button"
						onclick={() => (schedPeriod = 'PM')}
						class="flex-1 rounded-lg py-2 text-sm font-semibold transition cursor-pointer {schedPeriod === 'PM'
							? 'bg-amber-600 text-white shadow-xs'
							: 'border border-gray-300 bg-white text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300'}"
					>
						PM
					</button>
				</div>
			</div>
		</div>

		<!-- Modal Actions -->
		<div class="flex justify-end gap-3 pt-3 border-t border-gray-100 dark:border-gray-800">
			<button
				type="button"
				onclick={() => (scheduleOpen = false)}
				class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 transition hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300 cursor-pointer"
			>
				Cancel
			</button>
			<button
				type="button"
				id="btn-confirm-bulk-schedule"
				onclick={submitSchedule}
				disabled={submitting}
				class="inline-flex items-center gap-2 rounded-lg bg-teal-600 px-5 py-2 text-sm font-semibold text-white shadow-xs transition hover:bg-teal-500 disabled:opacity-50 cursor-pointer"
			>
				{#if submitting}
					<svg class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
						<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
						<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
					</svg>
				{/if}
				<span>Confirm & Schedule</span>
			</button>
		</div>
	</div>
</Modal>
