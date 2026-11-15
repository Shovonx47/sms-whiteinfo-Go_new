<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte.js';
	import { api, ApiError } from '$lib/api.js';
	import { toast } from '$lib/stores/toast.svelte.js';
	import { goto } from '$app/navigation';
	import { onDestroy, onMount, tick } from 'svelte';

	interface KycUser {
		id: number;
		name: string;
		email: string;
		kyc_status: string;
		nid_number?: string | null;
		trade_license_number?: string | null;
		kyc_rejected_reason?: string | null;
	}

	interface KycData {
		user: KycUser;
		is_resubmission: boolean;
		is_locked: boolean;
		can_submit: boolean;
		has_nid_on_record: boolean;
		requested_fields: string[];
		title: string;
		heading: string;
		subheading: string;
	}

	let kycData = $state<KycData | null>(null);
	let loading = $state(true);
	let loadError = $state('');
	let submitting = $state(false);

	let nidNumber = $state('');
	let tradeLicenseNumber = $state('');
	let frontNidFile = $state<File | null>(null);
	let backNidFile = $state<File | null>(null);
	let tradeLicenseFile = $state<File | null>(null);
	let frontPreview = $state<string | null>(null);
	let backPreview = $state<string | null>(null);

	let cameraOpen = $state(false);
	let cameraError = $state('');
	let livePreview = $state<string | null>(null);
	let videoElement: HTMLVideoElement | undefined = $state();
	let mediaStream: MediaStream | null = null;

	const isResubmission = $derived(kycData?.is_resubmission ?? false);
	const canSubmit = $derived(kycData?.can_submit ?? false);
	const isLocked = $derived(kycData?.is_locked ?? false);

	function showField(field: string): boolean {
		if (!kycData || !kycData.is_resubmission) return true;
		const requested = kycData.requested_fields ?? [];
		if (requested.length === 0) return true;

		return requested.includes(field);
	}

	async function loadKyc() {
		loading = true;
		loadError = '';
		try {
			const res = await api.get<{ data: KycData }>('/api/v1/customer/kyc');
			kycData = res.data;
			nidNumber = res.data.user.nid_number ?? '';
			tradeLicenseNumber = res.data.user.trade_license_number ?? '';
		} catch (e) {
			loadError = e instanceof ApiError ? e.message : 'Unable to load your KYC information. Please try again.';
		} finally {
			loading = false;
		}
	}

	onMount(() => loadKyc());
	onDestroy(stopCamera);

	function handleFileChange(e: Event, type: 'front' | 'back' | 'trade') {
		const input = e.target as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;

		const reader = new FileReader();
		reader.onload = (event) => {
			const url = event.target?.result as string;
			if (type === 'front') {
				frontNidFile = file;
				frontPreview = url;
			} else if (type === 'back') {
				backNidFile = file;
				backPreview = url;
			} else {
				tradeLicenseFile = file;
			}
		};
		reader.readAsDataURL(file);
	}

	async function openCamera() {
		cameraError = '';
		cameraOpen = true;

		try {
			if (!navigator.mediaDevices || !navigator.mediaDevices.getUserMedia) {
				throw new Error('Camera access is not supported by your browser or requires HTTPS.');
			}
			const stream = await navigator.mediaDevices.getUserMedia({
				video: { facingMode: 'user', width: { ideal: 640 }, height: { ideal: 480 } },
				audio: false
			});
			mediaStream = stream;
			await tick();
			if (videoElement) {
				videoElement.srcObject = stream;
				await videoElement.play();
			}
		} catch (err: unknown) {
			cameraError = err instanceof Error ? err.message : 'Unable to access camera. Please allow camera permissions.';
		}
	}

	function stopCamera() {
		if (mediaStream) {
			mediaStream.getTracks().forEach((track) => track.stop());
			mediaStream = null;
		}
		cameraOpen = false;
	}

	function capturePhoto() {
		if (!videoElement) return;

		const canvas = document.createElement('canvas');
		canvas.width = videoElement.videoWidth || 640;
		canvas.height = videoElement.videoHeight || 480;
		const ctx = canvas.getContext('2d');
		if (!ctx) return;

		ctx.drawImage(videoElement, 0, 0, canvas.width, canvas.height);
		livePreview = canvas.toDataURL('image/jpeg', 0.9);
		stopCamera();
	}

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();

		if (!isResubmission) {
			if (!nidNumber.trim()) {
				toast.error('Validation Error', 'National ID Number is required.');
				return;
			}
			if (!frontNidFile) {
				toast.error('Validation Error', 'The front side of your NID is required.');
				return;
			}
			if (!backNidFile) {
				toast.error('Validation Error', 'The back side of your NID is required.');
				return;
			}
			if (!livePreview) {
				toast.error('Validation Error', 'Please capture a live photo before submitting.');
				return;
			}
		}

		submitting = true;
		try {
			const formData = new FormData();
			formData.append('nid_number', nidNumber.trim());
			if (frontNidFile) formData.append('front_nid', frontNidFile);
			if (backNidFile) formData.append('back_nid', backNidFile);
			if (livePreview) formData.append('live_photo', livePreview);
			if (tradeLicenseNumber.trim()) formData.append('trade_license_number', tradeLicenseNumber.trim());
			if (tradeLicenseFile) formData.append('trade_license_file', tradeLicenseFile);

			const token = api.getToken();
			const res = await fetch('/api/v1/customer/kyc', {
				method: 'POST',
				body: formData,
				credentials: 'include',
				headers: {
					Accept: 'application/json',
					'X-Requested-With': 'XMLHttpRequest',
					...(token ? { Authorization: `Bearer ${token}` } : {})
				}
			});

			if (!res.ok) {
				const errorData = await res.json().catch(() => ({}));
				let message = errorData.message ?? 'KYC submission failed';
				if (errorData.errors) {
					const firstKey = Object.keys(errorData.errors)[0];
					if (firstKey && errorData.errors[firstKey][0]) {
						message = errorData.errors[firstKey][0];
					}
				}
				throw new Error(message);
			}

			await auth.refresh();
			toast.success('KYC Submitted', 'Your documents have been submitted for review.');
			goto('/customer');
		} catch (err: unknown) {
			toast.error('Submission Failed', err instanceof Error ? err.message : 'KYC submission failed');
		} finally {
			submitting = false;
		}
	}

	function statusLabel(status: string): string {
		switch (status) {
			case 'verified':
				return 'KYC Verified';
			case 'under_review':
				return 'Under Review';
			case 'awaiting_sp_verification':
				return 'Awaiting Reseller Review';
			case 'resubmit_required':
				return 'Action Required';
			case 'rejected':
				return 'Rejected';
			default:
				return 'Unsubmitted';
		}
	}
</script>

<svelte:head>
	<title>KYC Documents - White INFO</title>
</svelte:head>

<div class="max-w-4xl space-y-6">
	{#if loading}
		<div class="flex items-center justify-center py-24">
			<div class="flex flex-col items-center gap-3 text-gray-500 dark:text-gray-400">
				<div class="h-8 w-8 animate-spin rounded-full border-2 border-gray-300 border-t-teal-600 dark:border-gray-700 dark:border-t-primary-500"></div>
				<p class="text-sm">Loading KYC information…</p>
			</div>
		</div>
	{:else if loadError}
		<div class="rounded-xl border border-red-200 bg-red-50 p-6 dark:border-red-500/30 dark:bg-red-500/10">
			<p class="text-sm font-semibold text-red-800 dark:text-red-300">KYC information unavailable</p>
			<p class="mt-1 text-xs text-red-700 dark:text-red-300">{loadError}</p>
			<button
				type="button"
				onclick={loadKyc}
				class="mt-4 rounded-lg bg-red-600 px-4 py-2 text-xs font-semibold text-white hover:bg-red-500"
			>
				Try again
			</button>
		</div>
	{:else if kycData}
		<div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
			<div>
				<h1 class="text-xl font-bold tracking-tight text-gray-950 dark:text-white">{kycData.title}</h1>
				<p class="mt-1 text-sm font-medium text-gray-700 dark:text-gray-300">{kycData.heading}</p>
				<p class="mt-1 max-w-2xl text-xs text-gray-500 dark:text-gray-400">{kycData.subheading}</p>
			</div>

			<span class="inline-flex items-center gap-1.5 self-start rounded-full border border-gray-200 bg-gray-100 px-3 py-1 text-xs font-semibold text-gray-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-400">
				<span class="h-2 w-2 rounded-full bg-teal-500"></span>
				{statusLabel(kycData.user.kyc_status)}
			</span>
		</div>

		{#if isResubmission && kycData.requested_fields.length > 0}
			<div class="rounded-xl border border-amber-200 bg-amber-50 p-5 text-xs text-amber-900 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200">
				<p class="text-sm font-bold text-amber-950 dark:text-amber-300">Resubmission Requested</p>
				<p class="mt-1">{kycData.subheading}</p>
				<p class="mt-2 font-medium">
					Fields needing update:
					<span class="font-mono">{kycData.requested_fields.join(', ')}</span>
				</p>
			</div>
		{/if}

		{#if isLocked || !canSubmit}
			<div class="space-y-4 rounded-xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900">
				<h2 class="text-sm font-bold text-gray-950 dark:text-white">Submitted KYC Documents</h2>
				{#if kycData.user.nid_number}
					<div>
						<p class="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">NID Number</p>
						<p class="mt-1 font-mono text-sm text-gray-900 dark:text-gray-100">{kycData.user.nid_number}</p>
					</div>
				{/if}
				{#if kycData.user.trade_license_number}
					<div>
						<p class="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">Trade License Number</p>
						<p class="mt-1 font-mono text-sm text-gray-900 dark:text-gray-100">{kycData.user.trade_license_number}</p>
					</div>
				{/if}
				<p class="text-xs text-gray-500 dark:text-gray-400">
					You cannot edit, change, or delete submitted KYC information. Please contact support if you need assistance.
				</p>
				<a href="/customer" class="inline-flex rounded-lg bg-teal-600 px-4 py-2 text-xs font-semibold text-white hover:bg-teal-500">
					Back to dashboard
				</a>
			</div>
		{:else}
			<form onsubmit={handleSubmit} class="space-y-6">
				<div class="space-y-5 rounded-xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900">
					<h2 class="text-sm font-bold text-gray-950 dark:text-white">
						{isResubmission ? 'Identity Documents — Correction Required' : 'Identity Documents'}
					</h2>

					{#if showField('nid_number')}
						<div>
							<label for="nid-number" class="mb-1.5 block text-xs font-semibold uppercase tracking-wider text-gray-700 dark:text-gray-300">
								NID Number {#if !isResubmission}<span class="text-red-500">*</span>{/if}
							</label>
							<input
								id="nid-number"
								type="text"
								bind:value={nidNumber}
								maxlength="30"
								placeholder="e.g. 1234567890"
								class="w-full rounded-lg border border-gray-300 bg-white px-3.5 py-2.5 font-mono text-sm text-gray-900 shadow-xs placeholder-gray-400 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100 dark:placeholder-gray-500"
							/>
						</div>
					{/if}

					<div class="grid grid-cols-1 gap-6 sm:grid-cols-2">
						{#if showField('nid_front')}
							<div>
								<span class="mb-1.5 block text-xs font-semibold uppercase tracking-wider text-gray-700 dark:text-gray-300">
									NID Front Side {#if !isResubmission}<span class="text-red-500">*</span>{/if}
								</span>
								<div class="relative flex flex-col items-center justify-center rounded-xl border-2 border-dashed border-gray-300 bg-gray-50/50 p-5 text-center transition hover:border-teal-600 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800/40 dark:hover:border-teal-500">
									{#if frontPreview}
										<img src={frontPreview} alt="NID front preview" class="max-h-36 rounded-lg object-contain" />
										<p class="mt-2 text-xs font-medium text-emerald-600 dark:text-emerald-400">Selected: {frontNidFile?.name}</p>
									{:else}
										<span class="text-xs font-semibold text-gray-700 dark:text-gray-300">Click to choose NID front</span>
										<span class="text-[10px] text-gray-500">JPG, PNG, WEBP up to 4MB</span>
									{/if}
									<input
										type="file"
										accept="image/*"
										onchange={(e) => handleFileChange(e, 'front')}
										class="absolute inset-0 cursor-pointer opacity-0"
									/>
								</div>
							</div>
						{/if}

						{#if showField('nid_back')}
							<div>
								<span class="mb-1.5 block text-xs font-semibold uppercase tracking-wider text-gray-700 dark:text-gray-300">
									NID Back Side {#if !isResubmission}<span class="text-red-500">*</span>{/if}
								</span>
								<div class="relative flex flex-col items-center justify-center rounded-xl border-2 border-dashed border-gray-300 bg-gray-50/50 p-5 text-center transition hover:border-teal-600 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800/40 dark:hover:border-teal-500">
									{#if backPreview}
										<img src={backPreview} alt="NID back preview" class="max-h-36 rounded-lg object-contain" />
										<p class="mt-2 text-xs font-medium text-emerald-600 dark:text-emerald-400">Selected: {backNidFile?.name}</p>
									{:else}
										<span class="text-xs font-semibold text-gray-700 dark:text-gray-300">Click to choose NID back</span>
										<span class="text-[10px] text-gray-500">JPG, PNG, WEBP up to 4MB</span>
									{/if}
									<input
										type="file"
										accept="image/*"
										onchange={(e) => handleFileChange(e, 'back')}
										class="absolute inset-0 cursor-pointer opacity-0"
									/>
								</div>
							</div>
						{/if}
					</div>
				</div>

				{#if showField('live_photo')}
					<div class="space-y-4 rounded-xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900">
						<div>
							<h2 class="text-sm font-bold text-gray-950 dark:text-white">Live Camera Selfie</h2>
							<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
								Take a live selfie using your device camera. Gallery uploads are not accepted.
								{#if !isResubmission}<span class="text-red-500">*</span>{/if}
							</p>
						</div>

						{#if livePreview}
							<div class="flex flex-col items-center gap-3">
								<img src={livePreview} alt="Live selfie preview" class="h-32 w-32 rounded-full border-2 border-teal-600 object-cover" />
								<button
									type="button"
									onclick={() => { livePreview = null; openCamera(); }}
									class="text-xs font-semibold text-teal-700 hover:underline dark:text-teal-400"
								>
									Retake photo
								</button>
							</div>
						{:else if cameraOpen}
							<div class="flex flex-col items-center gap-3">
								<video bind:this={videoElement} autoplay playsinline muted class="max-h-64 w-full max-w-md rounded-xl bg-black object-cover"></video>
								{#if cameraError}
									<p class="text-xs text-red-600 dark:text-red-400">{cameraError}</p>
								{/if}
								<div class="flex gap-3">
									<button
										type="button"
										onclick={capturePhoto}
										disabled={!!cameraError}
										class="rounded-lg bg-teal-600 px-4 py-2 text-xs font-semibold text-white hover:bg-teal-500 disabled:opacity-50"
									>
										Capture photo
									</button>
									<button
										type="button"
										onclick={stopCamera}
										class="rounded-lg border border-gray-300 px-4 py-2 text-xs font-semibold text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800"
									>
										Cancel
									</button>
								</div>
							</div>
						{:else}
							<button
								type="button"
								onclick={openCamera}
								class="flex w-full flex-col items-center justify-center gap-2 rounded-xl border-2 border-dashed border-gray-300 bg-gray-50/50 p-6 text-center transition hover:border-teal-600 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800/40 dark:hover:border-teal-500"
							>
								<span class="text-xs font-semibold text-gray-700 dark:text-gray-300">Open camera and capture selfie</span>
								<span class="text-[10px] text-gray-500">Camera permission is required</span>
							</button>
						{/if}
					</div>
				{/if}

				{#if showField('trade_license')}
					<div class="space-y-4 rounded-xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900">
						<div>
							<h2 class="text-sm font-bold text-gray-950 dark:text-white">Trade License (Optional)</h2>
							<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">Upload your trade license if you are a registered business.</p>
						</div>

						<div>
							<label for="trade-number" class="mb-1.5 block text-xs font-semibold uppercase tracking-wider text-gray-700 dark:text-gray-300">
								Trade License Number
							</label>
							<input
								id="trade-number"
								type="text"
								bind:value={tradeLicenseNumber}
								maxlength="100"
								placeholder="e.g. TL-2024-123456"
								class="w-full rounded-lg border border-gray-300 bg-white px-3.5 py-2.5 font-mono text-sm text-gray-900 shadow-xs placeholder-gray-400 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100 dark:placeholder-gray-500"
							/>
						</div>

						<div>
							<span class="mb-1.5 block text-xs font-semibold uppercase tracking-wider text-gray-700 dark:text-gray-300">Trade License Document</span>
							<input
								type="file"
								accept="image/*,.pdf,application/pdf"
								onchange={(e) => handleFileChange(e, 'trade')}
								class="w-full rounded-lg border border-gray-300 bg-white px-3.5 py-2 text-xs text-gray-700 shadow-xs file:mr-3 file:rounded-md file:border-0 file:bg-gray-100 file:px-3 file:py-1 file:text-xs file:font-semibold file:text-gray-700 hover:file:bg-gray-200 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300 dark:file:bg-gray-700 dark:file:text-gray-200 dark:hover:file:bg-gray-600"
							/>
							{#if tradeLicenseFile}
								<p class="mt-1 text-xs text-emerald-600 dark:text-emerald-400">Selected: {tradeLicenseFile.name}</p>
							{/if}
						</div>
					</div>
				{/if}

				<div class="flex items-center justify-end gap-3 pt-2">
					<button
						type="submit"
						disabled={submitting}
						class="rounded-lg bg-teal-600 px-6 py-2.5 text-sm font-semibold text-white shadow-xs transition hover:bg-teal-500 disabled:opacity-50"
					>
						{submitting ? 'Submitting Documents...' : isResubmission ? 'Resubmit KYC Documents' : 'Submit KYC'}
					</button>
				</div>
			</form>
		{/if}
	{/if}
</div>
