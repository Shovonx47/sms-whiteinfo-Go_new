<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte.js';
	import { api } from '$lib/api.js';
	import { toast } from '$lib/stores/toast.svelte.js';
	import { onMount } from 'svelte';

	let name = $state(auth.user?.name ?? '');
	let email = $state(auth.user?.email ?? '');
	let phone = $state(auth.user?.phone ?? '');

	// Password update
	let currentPassword = $state('');
	let newPassword = $state('');
	let confirmPassword = $state('');
	let updatingPassword = $state(false);

	// Profile update
	let updatingProfile = $state(false);

	// Two-factor authentication
	let twoFactorEnabled = $state(false);
	let twoFactorPassword = $state('');
	let twoFactorSecret = $state('');
	let twoFactorQr = $state('');
	let twoFactorCode = $state('');
	let twoFactorRecoveryCodes = $state<string[]>([]);
	let twoFactorBusy = $state(false);

	// Email change verification
	let newEmail = $state('');
	let emailPassword = $state('');
	let emailChangeBusy = $state(false);

	async function requestEmailChange(e: SubmitEvent) {
		e.preventDefault();
		emailChangeBusy = true;
		try {
			const res = await api.post<{ message?: string }>('/api/v1/customer/profile/email', {
				email: newEmail,
				password: emailPassword
			});
			toast.success('Verification sent', res.message ?? 'Check your new inbox for the confirmation link.');
			newEmail = '';
			emailPassword = '';
		} catch (err) {
			toast.error('Email change failed', err instanceof Error ? err.message : 'Unable to start the email change.');
		} finally {
			emailChangeBusy = false;
		}
	}

	async function loadTwoFactor() {
		try {
			const res = await api.get<{ data: { enabled: boolean } }>('/api/v1/customer/2fa');
			twoFactorEnabled = res.data?.enabled ?? false;
		} catch {
			twoFactorEnabled = false;
		}
	}

	async function startTwoFactorSetup() {
		if (!twoFactorPassword) {
			toast.error('Password required', 'Confirm your password to set up two-factor authentication.');
			return;
		}
		twoFactorBusy = true;
		try {
			const res = await api.post<{ data: { secret: string; qr_data_uri: string } }>('/api/v1/customer/2fa/setup', { password: twoFactorPassword });
			twoFactorSecret = res.data.secret;
			twoFactorQr = res.data.qr_data_uri;
			twoFactorPassword = '';
		} catch (err) {
			toast.error('Setup failed', err instanceof Error ? err.message : 'Unable to start 2FA setup.');
		} finally {
			twoFactorBusy = false;
		}
	}

	async function confirmTwoFactor() {
		twoFactorBusy = true;
		try {
			const res = await api.post<{ recovery_codes: string[] }>('/api/v1/customer/2fa/enable', { secret: twoFactorSecret, code: twoFactorCode });
			twoFactorRecoveryCodes = res.recovery_codes ?? [];
			twoFactorEnabled = true;
			twoFactorSecret = '';
			twoFactorQr = '';
			twoFactorCode = '';
			toast.success('Two-factor enabled', 'Store your recovery codes somewhere safe.');
		} catch (err) {
			toast.error('Invalid code', err instanceof Error ? err.message : 'Unable to enable two-factor authentication.');
		} finally {
			twoFactorBusy = false;
		}
	}

	async function disableTwoFactor() {
		twoFactorBusy = true;
		try {
			await api.post('/api/v1/customer/2fa/disable', { password: twoFactorPassword, code: twoFactorCode });
			twoFactorEnabled = false;
			twoFactorPassword = '';
			twoFactorCode = '';
			twoFactorRecoveryCodes = [];
			toast.success('Two-factor disabled', 'Authenticator codes are no longer required.');
		} catch (err) {
			toast.error('Unable to disable', err instanceof Error ? err.message : 'Check your password and authenticator code.');
		} finally {
			twoFactorBusy = false;
		}
	}

	onMount(loadTwoFactor);

	async function handleUpdateProfile(e: SubmitEvent) {
		e.preventDefault();
		updatingProfile = true;
		try {
			await api.put('/api/v1/customer/profile', { name, phone });
			await auth.refresh();
			toast.success('Profile Updated', 'Your contact details have been updated.');
		} catch (err) {
			toast.error('Update Failed', err instanceof Error ? err.message : 'Unable to update your profile.');
		} finally {
			updatingProfile = false;
		}
	}

	async function handleUpdatePassword(e: SubmitEvent) {
		e.preventDefault();
		if (!newPassword || newPassword !== confirmPassword) {
			toast.error('Mismatch', 'New passwords do not match.');
			return;
		}
		if (newPassword.length < 8) {
			toast.error('Weak Password', 'Password must be at least 8 characters.');
			return;
		}

		updatingPassword = true;
		try {
			await api.put('/api/v1/customer/password', {
				current_password: currentPassword,
				password: newPassword,
				password_confirmation: confirmPassword
			});
			toast.success('Password Changed', 'Your account security credentials were updated.');
			currentPassword = '';
			newPassword = '';
			confirmPassword = '';
		} catch (err) {
			toast.error('Password Update Failed', err instanceof Error ? err.message : 'Unable to update your password.');
		} finally {
			updatingPassword = false;
		}
	}
</script>

<svelte:head>
	<title>Profile - White INFO</title>
</svelte:head>

<div class="space-y-6 max-w-4xl">
	<!-- Page Header -->
	<div>
		<h1 class="text-xl font-bold tracking-tight text-gray-950 dark:text-white">Account Settings & Profile</h1>
		<p class="text-xs text-gray-500 dark:text-gray-400">Manage your SMS Whiteinfo contact details, security credentials, and KYC status.</p>
	</div>

	<!-- KYC & Status Badges -->
	<div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
		<div class="rounded-xl border border-gray-200 bg-white p-4 shadow-sm dark:border-gray-800 dark:bg-gray-900">
			<span class="text-xs text-gray-500 font-medium uppercase tracking-wider">Account Status</span>
			<div class="mt-2 flex items-center gap-2">
				<span class="h-2.5 w-2.5 rounded-full bg-emerald-500"></span>
				<span class="text-sm font-bold text-gray-950 dark:text-white capitalize">{auth.user?.status ?? 'Active'}</span>
			</div>
		</div>

		<div class="rounded-xl border border-gray-200 bg-white p-4 shadow-sm dark:border-gray-800 dark:bg-gray-900">
			<span class="text-xs text-gray-500 font-medium uppercase tracking-wider">KYC Verification</span>
			<div class="mt-2 flex items-center gap-2">
				<span class="h-2.5 w-2.5 rounded-full bg-emerald-500"></span>
				<span class="text-sm font-bold text-emerald-600 dark:text-emerald-400 capitalize">{auth.user?.kyc_status ?? 'Approved'}</span>
			</div>
		</div>

		<div class="rounded-xl border border-gray-200 bg-white p-4 shadow-sm dark:border-gray-800 dark:bg-gray-900">
			<span class="text-xs text-gray-500 font-medium uppercase tracking-wider">Current Balance</span>
			<div class="mt-2">
				<span class="font-mono text-sm font-bold text-teal-600 dark:text-teal-400">৳ {auth.balance.toFixed(2)} BDT</span>
			</div>
		</div>
	</div>

	<!-- Personal Details Card -->
	<div class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900">
		<h2 class="text-sm font-bold text-gray-950 dark:text-white mb-1">Personal Details</h2>
		<p class="text-xs text-gray-500 dark:text-gray-400 mb-6">Update your account name and secondary notification phone number.</p>

		<form onsubmit={handleUpdateProfile} class="space-y-4">
			<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
				<div>
					<label for="profile-name" class="mb-1.5 block text-xs font-semibold uppercase tracking-wider text-gray-700 dark:text-gray-300">Full Name</label>
					<input
						id="profile-name"
						type="text"
						bind:value={name}
						class="w-full rounded-lg border border-gray-300 bg-white px-3.5 py-2.5 text-sm text-gray-900 shadow-xs placeholder-gray-400 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
					/>
				</div>
				<div>
					<label for="profile-email" class="mb-1.5 block text-xs font-semibold uppercase tracking-wider text-gray-700 dark:text-gray-300">Email Address (Login)</label>
					<input
						id="profile-email"
						type="email"
						disabled
						value={email}
						class="w-full rounded-lg border border-gray-200 bg-gray-50 px-3.5 py-2.5 text-sm text-gray-500 shadow-xs cursor-not-allowed dark:border-gray-800 dark:bg-gray-800/40 dark:text-gray-400"
					/>
				</div>
			</div>

			<div>
				<label for="profile-phone" class="mb-1.5 block text-xs font-semibold uppercase tracking-wider text-gray-700 dark:text-gray-300">Phone Number</label>
				<input
					id="profile-phone"
					type="tel"
					bind:value={phone}
					placeholder="+8801700000000"
					class="w-full font-mono rounded-lg border border-gray-300 bg-white px-3.5 py-2.5 text-sm text-gray-900 shadow-xs placeholder-gray-400 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
				/>
			</div>

			<div class="flex justify-end pt-2">
				<button
					type="submit"
					disabled={updatingProfile}
					class="rounded-lg bg-teal-600 px-5 py-2.5 text-sm font-semibold text-white shadow-xs transition hover:bg-teal-500 disabled:opacity-50"
				>
					{updatingProfile ? 'Saving...' : 'Save Profile'}
				</button>
			</div>
		</form>
	</div>

	<!-- Change Email Card -->
	<div class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900">
		<h2 class="text-sm font-bold text-gray-950 dark:text-white mb-1">Change Email Address</h2>
		<p class="text-xs text-gray-500 dark:text-gray-400 mb-4">Your login email changes only after you confirm the link sent to the new address. Current email: <strong class="text-gray-800 dark:text-gray-200">{email}</strong></p>
		<form onsubmit={requestEmailChange} class="grid gap-4 sm:grid-cols-2">
			<div>
				<label for="new-email" class="mb-1.5 block text-xs font-semibold uppercase tracking-wider text-gray-700 dark:text-gray-300">New Email Address</label>
				<input id="new-email" type="email" required bind:value={newEmail} class="w-full rounded-lg border border-gray-300 bg-white px-3.5 py-2.5 text-sm dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100" />
			</div>
			<div>
				<label for="email-password" class="mb-1.5 block text-xs font-semibold uppercase tracking-wider text-gray-700 dark:text-gray-300">Current Password</label>
				<input id="email-password" type="password" required bind:value={emailPassword} class="w-full rounded-lg border border-gray-300 bg-white px-3.5 py-2.5 text-sm dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100" />
			</div>
			<div class="sm:col-span-2 flex justify-end">
				<button type="submit" disabled={emailChangeBusy} class="rounded-lg bg-teal-600 px-5 py-2.5 text-sm font-semibold text-white hover:bg-teal-500 disabled:opacity-50">{emailChangeBusy ? 'Sending…' : 'Send Verification Link'}</button>
			</div>
		</form>
	</div>

	<!-- Change Password Card -->
	<div class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900">
		<h2 class="text-sm font-bold text-gray-950 dark:text-white mb-1">Security & Password</h2>
		<p class="text-xs text-gray-500 dark:text-gray-400 mb-6">Ensure your customer portal account is protected with a strong passphrase.</p>

		<form onsubmit={handleUpdatePassword} class="space-y-4">
			<div>
				<label for="current-password" class="mb-1.5 block text-xs font-semibold uppercase tracking-wider text-gray-700 dark:text-gray-300">Current Password</label>
				<input
					id="current-password"
					type="password"
					bind:value={currentPassword}
					placeholder="••••••••"
					class="w-full rounded-lg border border-gray-300 bg-white px-3.5 py-2.5 text-sm text-gray-900 shadow-xs placeholder-gray-400 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
				/>
			</div>

			<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
				<div>
					<label for="new-password" class="mb-1.5 block text-xs font-semibold uppercase tracking-wider text-gray-700 dark:text-gray-300">New Password</label>
					<input
						id="new-password"
						type="password"
						bind:value={newPassword}
						placeholder="Minimum 8 characters"
						class="w-full rounded-lg border border-gray-300 bg-white px-3.5 py-2.5 text-sm text-gray-900 shadow-xs placeholder-gray-400 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
					/>
				</div>
				<div>
					<label for="confirm-password" class="mb-1.5 block text-xs font-semibold uppercase tracking-wider text-gray-700 dark:text-gray-300">Confirm New Password</label>
					<input
						id="confirm-password"
						type="password"
						bind:value={confirmPassword}
						placeholder="Re-enter new password"
						class="w-full rounded-lg border border-gray-300 bg-white px-3.5 py-2.5 text-sm text-gray-900 shadow-xs placeholder-gray-400 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
					/>
				</div>
			</div>

			<div class="flex justify-end pt-2">
				<button
					type="submit"
					disabled={updatingPassword}
					class="rounded-lg bg-teal-600 px-5 py-2.5 text-sm font-semibold text-white shadow-xs transition hover:bg-teal-500 disabled:opacity-50"
				>
					{updatingPassword ? 'Updating...' : 'Update Password'}
				</button>
			</div>
		</form>
	</div>

	<!-- Two-Factor Authentication Card -->
	<div class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900">
		<div class="flex items-start justify-between gap-4">
			<div>
				<h2 class="text-sm font-bold text-gray-950 dark:text-white mb-1">Two-Factor Authentication</h2>
				<p class="text-xs text-gray-500 dark:text-gray-400">Protect your account with an authenticator app (Google Authenticator, Authy, 1Password).</p>
			</div>
			<span class="rounded-full px-2.5 py-1 text-[10px] font-bold uppercase {twoFactorEnabled ? 'bg-emerald-50 text-emerald-700' : 'bg-gray-100 text-gray-600'}">{twoFactorEnabled ? 'Enabled' : 'Disabled'}</span>
		</div>

		{#if twoFactorRecoveryCodes.length > 0}
			<div class="mt-4 rounded-lg border border-amber-200 bg-amber-50 p-4">
				<p class="text-xs font-semibold text-amber-800">Recovery codes (shown once)</p>
				<div class="mt-2 grid grid-cols-2 gap-1 font-mono text-xs text-amber-900 sm:grid-cols-4">
					{#each twoFactorRecoveryCodes as code}<span>{code}</span>{/each}
				</div>
			</div>
		{/if}

		{#if twoFactorQr}
			<div class="mt-4 flex flex-col items-center gap-3 rounded-lg border border-gray-200 p-4 sm:flex-row dark:border-gray-700">
				<img src={twoFactorQr} alt="Authenticator QR code" class="h-40 w-40" />
				<div class="text-xs text-gray-600 dark:text-gray-300">
					<p class="font-semibold text-gray-900 dark:text-white">Scan this QR code</p>
					<p class="mt-1">Or enter this setup key manually:</p>
					<code class="mt-1 block break-all rounded bg-gray-100 p-2 font-mono text-[11px] dark:bg-gray-800">{twoFactorSecret}</code>
					<input type="text" bind:value={twoFactorCode} inputmode="numeric" maxlength="6" placeholder="6-digit code" class="mt-3 w-40 rounded-lg border border-gray-300 px-3 py-2 font-mono text-sm dark:border-gray-700 dark:bg-gray-800 dark:text-white" />
					<button type="button" onclick={confirmTwoFactor} disabled={twoFactorBusy || twoFactorCode.length !== 6} class="mt-2 rounded-lg bg-teal-600 px-4 py-2 text-xs font-semibold text-white hover:bg-teal-500 disabled:opacity-50">Verify & Enable</button>
				</div>
			</div>
		{:else}
			<div class="mt-4 flex flex-col gap-2 sm:flex-row sm:items-end">
				<div class="flex-1">
					<label for="twofactor-password" class="mb-1 block text-[11px] font-semibold uppercase tracking-wider text-gray-600 dark:text-gray-400">{twoFactorEnabled ? 'Password + authenticator code to disable' : 'Confirm your password'}</label>
					<input id="twofactor-password" type="password" bind:value={twoFactorPassword} placeholder="Current password" class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-gray-700 dark:bg-gray-800 dark:text-white" />
					{#if twoFactorEnabled}
						<input type="text" bind:value={twoFactorCode} inputmode="numeric" placeholder="Authenticator or recovery code" class="mt-2 w-full rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-gray-700 dark:bg-gray-800 dark:text-white" />
					{/if}
				</div>
				{#if twoFactorEnabled}
					<button type="button" onclick={disableTwoFactor} disabled={twoFactorBusy || !twoFactorPassword || !twoFactorCode} class="rounded-lg border border-rose-300 px-4 py-2 text-xs font-semibold text-rose-600 hover:bg-rose-50 disabled:opacity-50">Disable 2FA</button>
				{:else}
					<button type="button" onclick={startTwoFactorSetup} disabled={twoFactorBusy || !twoFactorPassword} class="rounded-lg bg-teal-600 px-4 py-2 text-xs font-semibold text-white hover:bg-teal-500 disabled:opacity-50">Set up 2FA</button>
				{/if}
			</div>
		{/if}
	</div>
</div>
