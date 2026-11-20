<script lang="ts">
	import { supermanApi } from '$lib/superman-api.js';
	import { INFOZILLION_DEFAULTS, type GatewayDetail } from '$lib/gateway-admin.js';

	interface Props {
		mode: 'create' | 'edit';
		gateway?: GatewayDetail | null;
		/** Called after a successful save. `another` is true for "Create & create another". */
		onSaved: (result: { id: number; another: boolean }) => void;
		onCancel: () => void;
		onApiTypeChange?: (apiType: string) => void;
	}

	let { mode, gateway = null, onSaved, onCancel, onApiTypeChange }: Props = $props();

	const HANDLED_CREDENTIALS = [
		'base_url',
		'api_type',
		'username',
		'password',
		'bill_msisdn',
		'api_key',
		'enabled_carriers',
		'transaction_type'
	];

	const stringOf = (value: unknown, fallback = ''): string =>
		value === null || value === undefined ? fallback : String(value);

	function carriersOf(value: unknown): string[] {
		if (Array.isArray(value)) return value.map((v) => String(v).trim()).filter(Boolean);
		if (typeof value === 'string') {
			return value
				.split(',')
				.map((v) => v.trim())
				.filter(Boolean);
		}
		return [];
	}

	// ----- form state -----
	let name = $state('');
	let driver = $state('infozillion');
	let requestsPerSecond = $state('5');
	let bulkChunkSize = $state('500');
	let timeoutSeconds = $state('2');
	let priority = $state('1');
	let isOtpEnabled = $state('1');
	let isEnabled = $state('1');
	let supportsBulk = $state('0');
	let dlrSupport = $state('1');
	let costMode = $state('manual');
	let dippingRequired = $state('1');
	let hasThirdPartyDipping = $state('0');

	let baseUrl = $state(INFOZILLION_DEFAULTS.base_url);
	let apiType = $state(INFOZILLION_DEFAULTS.api_type);
	let username = $state('');
	let password = $state('');
	let billMsisdn = $state('');
	let apiKey = $state(INFOZILLION_DEFAULTS.api_key);
	let carriers = $state<string[]>([]);
	let carrierInput = $state('');
	let transactionType = $state(INFOZILLION_DEFAULTS.transaction_type);

	let showPassword = $state(false);
	let showApiKey = $state(false);
	let saving = $state(false);
	let formError = $state<string | null>(null);
	let fieldErrors = $state<Record<string, string>>({});

	// Credentials / config keys the form has no input for are preserved on save.
	let extraCredentials: Record<string, unknown> = {};
	let extraConfig: Record<string, unknown> = {};
	let hydratedId: number | null = null;

	$effect(() => {
		if (mode !== 'edit' || !gateway || hydratedId === gateway.id) return;
		hydratedId = gateway.id;
		const creds = gateway.credentials ?? {};
		const config = gateway.config ?? {};

		name = gateway.name;
		driver = gateway.driver;
		requestsPerSecond = String(gateway.requests_per_second);
		timeoutSeconds = String(gateway.timeout_seconds);
		priority = String(gateway.priority);
		isOtpEnabled = gateway.is_otp_enabled ? '1' : '0';
		isEnabled = gateway.is_enabled ? '1' : '0';
		supportsBulk = gateway.supports_bulk ? '1' : '0';
		dlrSupport = gateway.dlr_support ? '1' : '0';
		costMode = gateway.cost_mode || 'manual';
		dippingRequired = gateway.dipping_required ? '1' : '0';
		hasThirdPartyDipping = gateway.has_third_party_dipping ? '1' : '0';
		bulkChunkSize = stringOf(config.bulk_chunk_size, '500');

		baseUrl = stringOf(creds.base_url);
		apiType = stringOf(creds.api_type, 'iptsp').toLowerCase() === 'mno' ? 'mno' : 'iptsp';
		username = stringOf(creds.username);
		password = stringOf(creds.password);
		billMsisdn = stringOf(creds.bill_msisdn);
		apiKey = stringOf(creds.api_key);
		carriers = carriersOf(creds.enabled_carriers);
		transactionType = stringOf(creds.transaction_type, 'T');

		extraCredentials = Object.fromEntries(Object.entries(creds).filter(([key]) => !HANDLED_CREDENTIALS.includes(key)));
		extraConfig = Object.fromEntries(Object.entries(config).filter(([key]) => key !== 'bulk_chunk_size'));
		onApiTypeChange?.(apiType);
	});

	function addCarrier() {
		const value = carrierInput.trim().replace(/,$/, '');
		carrierInput = '';
		if (!value || carriers.includes(value)) return;
		carriers = [...carriers, value];
	}

	function removeCarrier(value: string) {
		carriers = carriers.filter((c) => c !== value);
	}

	function validate(): boolean {
		const errors: Record<string, string> = {};
		if (!name.trim()) errors.name = 'The name field is required.';
		const rps = Number(requestsPerSecond);
		if (!requestsPerSecond.trim() || !Number.isFinite(rps)) errors.requests_per_second = 'The requests per second field is required.';
		else if (rps < 1) errors.requests_per_second = 'The requests per second field must be at least 1.';
		if (bulkChunkSize.trim()) {
			const chunk = Number(bulkChunkSize);
			if (!Number.isFinite(chunk)) errors.bulk_chunk_size = 'The bulk batch chunk size must be a number.';
			else if (chunk < 10 || chunk > 1000) errors.bulk_chunk_size = 'The bulk batch chunk size must be between 10 and 1000.';
		}
		const timeout = Number(timeoutSeconds);
		if (!timeoutSeconds.trim() || !Number.isFinite(timeout)) errors.timeout_seconds = 'The connection timeout field is required.';
		else if (timeout < 1 || timeout > 120) errors.timeout_seconds = 'The connection timeout must be between 1 and 120.';
		const prio = Number(priority);
		if (!priority.trim() || !Number.isFinite(prio)) errors.priority = 'The OTP gateway priority field is required.';
		else if (prio < 1) errors.priority = 'The OTP gateway priority field must be at least 1.';

		if (driver === 'infozillion') {
			if (!baseUrl.trim()) errors.base_url = 'The base URL field is required.';
			if (!username.trim()) errors.username = 'The username field is required.';
			if (!password.trim()) errors.password = 'The password field is required.';
			if (!billMsisdn.trim()) errors.bill_msisdn = 'The bill MSISDN field is required.';
			if (!apiKey.trim()) errors.api_key = 'The API key field is required.';
		}
		fieldErrors = errors;
		return Object.keys(errors).length === 0;
	}

	async function submit(another: boolean) {
		formError = null;
		addCarrierIfPending();
		if (!validate()) {
			formError = 'Please fix the highlighted fields.';
			return;
		}

		const credentials: Record<string, unknown> = { ...extraCredentials };
		if (driver === 'infozillion') {
			Object.assign(credentials, {
				base_url: baseUrl.trim(),
				api_type: apiType,
				username: username.trim(),
				password,
				bill_msisdn: billMsisdn.trim(),
				api_key: apiKey,
				enabled_carriers: carriers,
				transaction_type: transactionType
			});
		}

		const config: Record<string, unknown> = { ...extraConfig };
		if (bulkChunkSize.trim()) config.bulk_chunk_size = Number(bulkChunkSize);

		const payload: Record<string, unknown> = {
			name: name.trim(),
			requests_per_second: Number(requestsPerSecond),
			timeout_seconds: Number(timeoutSeconds),
			priority: Number(priority),
			is_otp_enabled: isOtpEnabled === '1',
			is_enabled: isEnabled === '1',
			supports_bulk: supportsBulk === '1',
			dlr_support: dlrSupport === '1',
			cost_mode: costMode,
			dipping_required: dippingRequired === '1',
			has_third_party_dipping: hasThirdPartyDipping === '1',
			credentials,
			config
		};

		saving = true;
		try {
			if (mode === 'create') {
				payload.driver = driver;
				const res = await supermanApi.post<{ id: number }>('/api/v1/superman/gateways', payload);
				onSaved({ id: res.id, another });
				if (another) resetForCreateAnother();
			} else if (gateway) {
				await supermanApi.put(`/api/v1/superman/gateways/${gateway.id}`, payload);
				onSaved({ id: gateway.id, another: false });
			}
		} catch (err: unknown) {
			formError = err instanceof Error ? err.message : 'Unable to save the gateway.';
		} finally {
			saving = false;
		}
	}

	function addCarrierIfPending() {
		if (carrierInput.trim()) addCarrier();
	}

	function resetForCreateAnother() {
		name = '';
		username = '';
		password = '';
		billMsisdn = '';
		carriers = [];
		fieldErrors = {};
	}

	const label = 'block text-sm font-medium text-gray-900 dark:text-gray-100';
	const input =
		'mt-1.5 w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-xs outline-none placeholder:text-gray-400 focus:border-emerald-600 focus:ring-1 focus:ring-emerald-600 disabled:cursor-not-allowed disabled:bg-gray-100 disabled:text-gray-500 dark:border-gray-700 dark:bg-gray-800 dark:text-white dark:disabled:bg-gray-800/60';
	const helper = 'mt-1.5 text-sm text-gray-500 dark:text-gray-400';
	const card = 'rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900';
	const cardHeader = 'border-b border-gray-200 px-6 py-4 dark:border-gray-800';
</script>

{#snippet req()}<span class="text-rose-600">*</span>{/snippet}

{#snippet err(key: string)}
	{#if fieldErrors[key]}<p class="mt-1.5 text-sm text-rose-600" role="alert">{fieldErrors[key]}</p>{/if}
{/snippet}

<form
	onsubmit={(e) => {
		e.preventDefault();
		void submit(false);
	}}
	class="grid items-start gap-6 lg:grid-cols-2"
	novalidate
>
	{#if formError}
		<div role="alert" data-testid="form-error" class="rounded-lg border border-rose-200 bg-rose-50 p-3 text-sm text-rose-700 lg:col-span-2 dark:border-rose-900 dark:bg-rose-950/30 dark:text-rose-300">{formError}</div>
	{/if}

	<div class="space-y-6">
		<!-- Gateway Details -->
		<section class={card}>
			<header class={cardHeader}><h2 class="text-base font-semibold text-gray-950 dark:text-white">Gateway Details</h2></header>
			<div class="space-y-5 p-6">
				<div>
					<label for="gw-name" class={label}>Name {@render req()}</label>
					<input id="gw-name" type="text" bind:value={name} maxlength="255" class={input} />
					{@render err('name')}
				</div>

				<div>
					<label for="gw-driver" class={label}>Driver</label>
					<select id="gw-driver" bind:value={driver} disabled={mode === 'edit'} class={input}>
						<option value="infozillion">InfoZillion (MNPSP)</option>
						{#if driver !== 'infozillion'}<option value={driver}>{driver}</option>{/if}
					</select>
				</div>

				<div>
					<label for="gw-rps" class={label}>Requests Per Second (Rate Limit) {@render req()}</label>
					<input id="gw-rps" type="number" min="1" bind:value={requestsPerSecond} class={input} />
					<p class={helper}>Maximum HTTP requests sent to gateway per second (e.g. 2, 5, 10). Remaining wait in queue.</p>
					{@render err('requests_per_second')}
				</div>

				<div>
					<label for="gw-chunk" class={label}>Bulk Batch Chunk Size</label>
					<input id="gw-chunk" type="number" min="10" max="1000" bind:value={bulkChunkSize} class={input} />
					<p class={helper}>How many numbers to send per bulk API call (e.g. 100, 200, 500, 999). Remaining stay in queue.</p>
					{@render err('bulk_chunk_size')}
				</div>

				<div>
					<label for="gw-timeout" class={label}>Connection Timeout (Seconds) {@render req()}</label>
					<input id="gw-timeout" type="number" min="1" max="120" bind:value={timeoutSeconds} class={input} />
					<p class={helper}>Maximum HTTP request timeout before failing over to the next provider.</p>
					{@render err('timeout_seconds')}
				</div>

				<div>
					<label for="gw-priority" class={label}>OTP Gateway Priority {@render req()}</label>
					<input id="gw-priority" type="number" min="1" bind:value={priority} class={input} />
					<p class={helper}>Lower number = higher priority during OTP failover (e.g. 1 is tried first).</p>
					{@render err('priority')}
				</div>

				<div>
					<label for="gw-otp" class={label}>Use for OTP Sending {@render req()}</label>
					<select id="gw-otp" bind:value={isOtpEnabled} class={input}>
						<option value="1">Yes</option>
						<option value="0">No</option>
					</select>
					<p class={helper}>Include this gateway in OTP priority delivery failover.</p>
				</div>

				<div>
					<label for="gw-enabled" class={label}>Gateway Enabled {@render req()}</label>
					<select id="gw-enabled" bind:value={isEnabled} class={input}>
						<option value="1">Yes</option>
						<option value="0">No</option>
					</select>
				</div>

				<div>
					<label for="gw-bulk" class={label}>Supports Promotional {@render req()}</label>
					<select id="gw-bulk" bind:value={supportsBulk} class={input}>
						<option value="1">Yes</option>
						<option value="0">No</option>
					</select>
				</div>
			</div>
		</section>

		<!-- Configuration -->
		<section class={card}>
			<header class={cardHeader}><h2 class="text-base font-semibold text-gray-950 dark:text-white">Configuration</h2></header>
			<div class="p-6">
				{#if driver === 'infozillion'}
					<fieldset class="rounded-xl border border-gray-200 px-5 pb-5 pt-2 dark:border-gray-800">
						<legend class="px-2 text-sm font-semibold text-gray-950 dark:text-white">Credentials</legend>
						<div class="space-y-5">
							<div>
								<label for="gw-base-url" class={label}>Base URL {@render req()}</label>
								<input id="gw-base-url" type="text" bind:value={baseUrl} placeholder="https://api.mnpspbd.com" class={input} />
								<p class={helper}>MNO URL: https://api.mnpspbd.com or IPTSP URL: https://api.mnpspbd.com</p>
								{@render err('base_url')}
							</div>

							<div>
								<label for="gw-api-type" class={label}>API Type {@render req()}</label>
								<select
									id="gw-api-type"
									bind:value={apiType}
									onchange={() => onApiTypeChange?.(apiType)}
									class={input}
								>
									<option value="iptsp">IPTSP</option>
									<option value="mno">MNO</option>
								</select>
								<p class={helper}>IPTSP: uses /a2p-sms-iptsp/ endpoints (ADN, Mirnet, Bracnet, etc.). MNO: uses /a2p-sms/ endpoints (GP, RB, TT, BL).</p>
							</div>

							<div>
								<label for="gw-username" class={label}>Username {@render req()}</label>
								<input id="gw-username" type="text" bind:value={username} placeholder="Enter your aggregator username at MNO" autocomplete="off" class={input} />
								{@render err('username')}
							</div>

							<div>
								<label for="gw-password" class={label}>Password {@render req()}</label>
								<div class="relative">
									<input id="gw-password" type={showPassword ? 'text' : 'password'} bind:value={password} placeholder="Enter your aggregator password at MNO" autocomplete="new-password" class="{input} pr-12" />
									<button type="button" onclick={() => (showPassword = !showPassword)} aria-label={showPassword ? 'Hide password' : 'Show password'} class="absolute inset-y-0 right-0 mt-1.5 flex w-11 items-center justify-center rounded-r-lg border-l border-gray-300 text-gray-500 hover:text-gray-800 dark:border-gray-700">
										<svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
											{#if showPassword}
												<path stroke-linecap="round" stroke-linejoin="round" d="M3.98 8.223A10.477 10.477 0 0 0 1.934 12C3.226 16.338 7.244 19.5 12 19.5c.993 0 1.953-.138 2.863-.395M6.228 6.228A10.451 10.451 0 0 1 12 4.5c4.756 0 8.773 3.162 10.065 7.498a10.522 10.522 0 0 1-4.293 5.774M6.228 6.228 3 3m3.228 3.228 3.65 3.65m7.894 7.894L21 21m-3.228-3.228-3.65-3.65m0 0a3 3 0 1 0-4.243-4.243m4.242 4.242L9.88 9.88" />
											{:else}
												<path stroke-linecap="round" stroke-linejoin="round" d="M2.036 12.322a1.012 1.012 0 0 1 0-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.963-7.178Z" /><path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z" />
											{/if}
										</svg>
									</button>
								</div>
								{@render err('password')}
							</div>

							<div>
								<label for="gw-bill" class={label}>Bill MSISDN {@render req()}</label>
								<input id="gw-bill" type="text" bind:value={billMsisdn} placeholder="8801XXXXXXXXX" class={input} />
								<p class={helper}>Your registered billing MSISDN at the MNO platform.</p>
								{@render err('bill_msisdn')}
							</div>

							<div>
								<label for="gw-api-key" class={label}>API Key {@render req()}</label>
								<div class="relative">
									<input id="gw-api-key" type={showApiKey ? 'text' : 'password'} bind:value={apiKey} placeholder="VQAzoXHxqsiHljWUfY5mk2lcIKIOFMmw" autocomplete="off" class="{input} pr-12" />
									<button type="button" onclick={() => (showApiKey = !showApiKey)} aria-label={showApiKey ? 'Hide API key' : 'Show API key'} class="absolute inset-y-0 right-0 mt-1.5 flex w-11 items-center justify-center rounded-r-lg border-l border-gray-300 text-gray-500 hover:text-gray-800 dark:border-gray-700">
										<svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
											{#if showApiKey}
												<path stroke-linecap="round" stroke-linejoin="round" d="M3.98 8.223A10.477 10.477 0 0 0 1.934 12C3.226 16.338 7.244 19.5 12 19.5c.993 0 1.953-.138 2.863-.395M6.228 6.228A10.451 10.451 0 0 1 12 4.5c4.756 0 8.773 3.162 10.065 7.498a10.522 10.522 0 0 1-4.293 5.774M6.228 6.228 3 3m3.228 3.228 3.65 3.65m7.894 7.894L21 21m-3.228-3.228-3.65-3.65m0 0a3 3 0 1 0-4.243-4.243m4.242 4.242L9.88 9.88" />
											{:else}
												<path stroke-linecap="round" stroke-linejoin="round" d="M2.036 12.322a1.012 1.012 0 0 1 0-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.963-7.178Z" /><path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z" />
											{/if}
										</svg>
									</button>
								</div>
								{@render err('api_key')}
							</div>

							<div>
								<label for="gw-carriers" class={label}>Enabled Carriers</label>
								<div class="mt-1.5 overflow-hidden rounded-lg border border-gray-300 bg-white focus-within:border-emerald-600 focus-within:ring-1 focus-within:ring-emerald-600 dark:border-gray-700 dark:bg-gray-800">
									<input
										id="gw-carriers"
										type="text"
										bind:value={carrierInput}
										placeholder="Type a short code and press Enter"
										onkeydown={(e) => {
											if (e.key === 'Enter' || e.key === ',') {
												e.preventDefault();
												addCarrier();
											} else if (e.key === 'Backspace' && !carrierInput && carriers.length) {
												carriers = carriers.slice(0, -1);
											}
										}}
										onblur={addCarrierIfPending}
										class="w-full border-0 bg-transparent px-3 py-2 text-sm outline-none placeholder:text-gray-400 dark:text-white"
									/>
									{#if carriers.length > 0}
										<div class="flex flex-wrap gap-1.5 border-t border-gray-200 px-3 py-2 dark:border-gray-700">
											{#each carriers as carrier (carrier)}
												<span class="inline-flex items-center gap-1 rounded-md border border-emerald-200 bg-emerald-50 px-2 py-0.5 text-xs font-medium text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-300">
													{carrier}
													<button type="button" onclick={() => removeCarrier(carrier)} aria-label="Remove {carrier}" class="leading-none opacity-70 hover:opacity-100">×</button>
												</span>
											{/each}
										</div>
									{/if}
								</div>
								<p class={helper}>Type carrier short codes (for example ADN or XX) and press Enter after each one. Whatever you type is saved and used as an enabled carrier. Leave empty to check all known carriers.</p>
							</div>

							<div>
								<label for="gw-txn" class={label}>Transaction Type</label>
								<select id="gw-txn" bind:value={transactionType} class={input}>
									<option value="T">Transactional (T) - Recommended for IPTSP &amp; Non-Masking</option>
									<option value="P">Promotional (P) - Requires approved template</option>
								</select>
								<p class={helper}>Transactional (T) bypasses promotional content restrictions on Infozillion/Telco. Strongly recommended for IPTSP &amp; Non-masking.</p>
							</div>
						</div>
					</fieldset>
				{:else}
					<p class="text-sm text-gray-500">Credentials for the "{driver}" driver are preserved as saved.</p>
				{/if}
			</div>
		</section>

		<div class="flex flex-wrap items-center gap-3">
			{#if mode === 'create'}
				<button type="submit" disabled={saving} data-testid="save-gateway" class="rounded-lg border border-emerald-600 bg-white px-4 py-2 text-sm font-semibold text-emerald-700 shadow-xs hover:bg-emerald-50 disabled:opacity-60 dark:bg-gray-900 dark:text-emerald-400">
					{saving ? 'Saving…' : 'Create'}
				</button>
				<button type="button" disabled={saving} onclick={() => void submit(true)} data-testid="save-another" class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-800 shadow-xs hover:bg-gray-50 disabled:opacity-60 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-200">
					Create &amp; create another
				</button>
			{:else}
				<button type="submit" disabled={saving} data-testid="save-gateway" class="rounded-lg border border-emerald-600 bg-white px-4 py-2 text-sm font-semibold text-emerald-700 shadow-xs hover:bg-emerald-50 disabled:opacity-60 dark:bg-gray-900 dark:text-emerald-400">
					{saving ? 'Saving…' : 'Save changes'}
				</button>
			{/if}
			<button type="button" onclick={onCancel} class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-800 shadow-xs hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-200">Cancel</button>
		</div>
	</div>

	<!-- Billing & Cost Configuration -->
	<section class={card}>
		<header class={cardHeader}>
			<h2 class="text-base font-semibold text-gray-950 dark:text-white">Billing &amp; Cost Configuration</h2>
			<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">Configure DLR tracking, dipping rules, and cost modes for this gateway.</p>
		</header>
		<div class="grid gap-5 p-6 sm:grid-cols-2">
			<div>
				<label for="gw-dlr" class={label}>DLR Support {@render req()}</label>
				<select id="gw-dlr" bind:value={dlrSupport} class={input}>
					<option value="1">Yes</option>
					<option value="0">No</option>
				</select>
				<p class={helper}>Determines whether delivery reports are tracked or explicitly marked N/A in analytics.</p>
			</div>
			<div>
				<label for="gw-cost-mode" class={label}>Cost Mode {@render req()}</label>
				<select id="gw-cost-mode" bind:value={costMode} class={input}>
					<option value="manual">Manual</option>
					<option value="api_assisted">Api assisted</option>
				</select>
				<p class={helper}>Manual is the authoritative billing source. API assisted allows cross-check.</p>
			</div>
			<div>
				<label for="gw-dipping" class={label}>Mandatory Dipping {@render req()}</label>
				<select id="gw-dipping" bind:value={dippingRequired} class={input}>
					<option value="1">Yes</option>
					<option value="0">No</option>
				</select>
				<p class={helper}>If enabled, profit calculation will require a configured dipping charge and refuse silent zeroing.</p>
			</div>
			<div>
				<label for="gw-tp-dipping" class={label}>Third-Party Dipping {@render req()}</label>
				<select id="gw-tp-dipping" bind:value={hasThirdPartyDipping} class={input}>
					<option value="1">Yes</option>
					<option value="0">No</option>
				</select>
				<p class={helper}>Enables third-party dipping volume and cost tracking on this gateway.</p>
			</div>
		</div>
	</section>
</form>
