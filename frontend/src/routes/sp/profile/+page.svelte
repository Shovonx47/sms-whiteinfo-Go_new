<script lang="ts">
	import { onMount } from 'svelte';
	import { spApi, SpApiError } from '$lib/sp-api.js';
	import { spAuth } from '$lib/stores/spAuth.svelte.js';

	let name = $state('');
	let phone = $state('');
	let currentPassword = $state('');
	let newPassword = $state('');
	let confirmPassword = $state('');
	let error = $state('');
	let success = $state('');
	let savingProfile = $state(false);
	let savingPassword = $state(false);

	onMount(() => {
		name = spAuth.user?.name ?? '';
		phone = spAuth.user?.phone ?? '';
	});

	async function saveProfile(e: SubmitEvent) {
		e.preventDefault();
		savingProfile = true; error = ''; success = '';
		try {
			await spApi.put('/api/v1/sp/auth/profile', { name, phone });
			await spAuth.refreshUser();
			success = 'Profile updated.';
			setTimeout(() => (success = ''), 5000);
		} catch (e) {
			error = e instanceof SpApiError ? e.message : 'Unable to update profile.';
		} finally {
			savingProfile = false;
		}
	}

	async function changePassword(e: SubmitEvent) {
		e.preventDefault();
		error = ''; success = '';
		if (newPassword !== confirmPassword) { error = 'New password confirmation does not match.'; return; }
		savingPassword = true;
		try {
			await spApi.post('/api/v1/sp/auth/change-password', { current_password: currentPassword, password: newPassword });
			currentPassword = ''; newPassword = ''; confirmPassword = '';
			success = 'Password changed.';
			setTimeout(() => (success = ''), 5000);
		} catch (e) {
			error = e instanceof SpApiError ? e.message : 'Unable to change password.';
		} finally {
			savingPassword = false;
		}
	}
</script>

<svelte:head><title>Profile — SMS Whiteinfo S P Reseller</title></svelte:head>

<div class="max-w-2xl space-y-4">
	<div>
		<h1 class="text-2xl font-bold tracking-tight text-gray-950 dark:text-white">Profile</h1>
		<p class="text-xs text-gray-500 dark:text-gray-400">Update your reseller account details and password.</p>
	</div>

	{#if error}<div class="rounded-xl border border-rose-200 bg-rose-50 p-3 text-xs text-rose-700 dark:border-rose-900 dark:bg-rose-950/40 dark:text-rose-300">{error}</div>{/if}
	{#if success}<div class="rounded-xl border border-emerald-200 bg-emerald-50 p-3 text-xs text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-300">{success}</div>{/if}

	<form onsubmit={saveProfile} class="space-y-3 rounded-xl border border-gray-200 bg-white p-5 text-xs shadow-xs dark:border-gray-800 dark:bg-gray-900">
		<h2 class="text-sm font-bold text-gray-900 dark:text-white">Account details</h2>
		<div><label for="profile-name" class="font-medium text-gray-700 dark:text-gray-300">Name</label><input id="profile-name" bind:value={name} required class="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2 dark:border-gray-700 dark:bg-gray-800 dark:text-white" /></div>
		<div><label for="profile-email" class="font-medium text-gray-700 dark:text-gray-300">Email</label><input id="profile-email" value={spAuth.user?.email ?? ''} disabled class="mt-1 w-full rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-gray-500 dark:border-gray-700 dark:bg-gray-800" /></div>
		<div><label for="profile-phone" class="font-medium text-gray-700 dark:text-gray-300">Phone</label><input id="profile-phone" bind:value={phone} class="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2 dark:border-gray-700 dark:bg-gray-800 dark:text-white" /></div>
		<button type="submit" disabled={savingProfile} class="rounded-lg bg-amber-500 px-4 py-1.5 font-semibold text-white disabled:opacity-50">{savingProfile ? 'Saving…' : 'Save Profile'}</button>
	</form>

	<form onsubmit={changePassword} class="space-y-3 rounded-xl border border-gray-200 bg-white p-5 text-xs shadow-xs dark:border-gray-800 dark:bg-gray-900">
		<h2 class="text-sm font-bold text-gray-900 dark:text-white">Change password</h2>
		<div><label for="current-password" class="font-medium text-gray-700 dark:text-gray-300">Current password</label><input id="current-password" type="password" bind:value={currentPassword} required class="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2 dark:border-gray-700 dark:bg-gray-800 dark:text-white" /></div>
		<div><label for="new-password" class="font-medium text-gray-700 dark:text-gray-300">New password</label><input id="new-password" type="password" bind:value={newPassword} required minlength="8" class="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2 dark:border-gray-700 dark:bg-gray-800 dark:text-white" /></div>
		<div><label for="confirm-password" class="font-medium text-gray-700 dark:text-gray-300">Confirm new password</label><input id="confirm-password" type="password" bind:value={confirmPassword} required class="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2 dark:border-gray-700 dark:bg-gray-800 dark:text-white" /></div>
		<button type="submit" disabled={savingPassword} class="rounded-lg bg-gray-900 px-4 py-1.5 font-semibold text-white disabled:opacity-50 dark:bg-white dark:text-gray-900">{savingPassword ? 'Changing…' : 'Change Password'}</button>
	</form>
</div>
