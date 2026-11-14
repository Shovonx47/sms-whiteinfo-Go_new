<script lang="ts">
	import { onMount } from 'svelte';
	import { spApi, SpApiError } from '$lib/sp-api.js';
	import { spAuth } from '$lib/stores/spAuth.svelte.js';

	interface Role { id: number; name: string; description: string; permissions: string[]; users_count: number; }

	const permissionGroups: Record<string, { key: string; label: string }[]> = {
		Customers: [
			{ key: 'customers.view', label: 'View Customers' },
			{ key: 'customers.create', label: 'Create Customers' },
			{ key: 'customers.edit', label: 'Edit Customers' },
			{ key: 'customers.delete', label: 'Delete Customers' },
			{ key: 'customers.assign', label: 'Assign Customers' },
			{ key: 'customers.export', label: 'Export Customers' }
		],
		'Billing & Tariff': [
			{ key: 'billing.view', label: 'View Billing & Tariff' },
			{ key: 'billing.manage', label: 'Manage Tariff & Deposits' }
		],
		Messaging: [
			{ key: 'messaging.view', label: 'View Messaging' },
			{ key: 'messaging.manage', label: 'Manage Senders & Messages' }
		],
		Employees: [
			{ key: 'employees.view', label: 'View Employees' },
			{ key: 'employees.create', label: 'Create Employees' },
			{ key: 'employees.edit', label: 'Edit Employees' },
			{ key: 'employees.delete', label: 'Delete Employees' }
		],
		Roles: [
			{ key: 'roles.view', label: 'View Roles' },
			{ key: 'roles.manage', label: 'Manage Roles' }
		],
		Reports: [{ key: 'reports.view', label: 'View Reports' }]
	};

	let roles = $state<Role[]>([]);
	let loading = $state(true);
	let error = $state('');
	let success = $state('');
	let modalOpen = $state(false);
	let editing = $state<Role | null>(null);
	let name = $state('');
	let description = $state('');
	let selected = $state<string[]>([]);
	let saving = $state(false);

	async function load() {
		loading = true;
		error = '';
		try {
			if (!spAuth.can('roles.view')) {
				error = 'You do not have permission to view roles.';
				roles = [];
				return;
			}
			const res = await spApi.get<{ data: Role[] }>('/api/v1/sp/roles');
			roles = res.data ?? [];
		} catch (e) {
			error = e instanceof SpApiError ? e.message : 'Unable to load roles.';
		} finally {
			loading = false;
		}
	}

	function openCreate() {
		editing = null; name = ''; description = ''; selected = []; error = ''; modalOpen = true;
	}

	function openEdit(role: Role) {
		editing = role; name = role.name; description = role.description; selected = [...(role.permissions ?? [])]; error = ''; modalOpen = true;
	}

	function toggle(key: string) {
		selected = selected.includes(key) ? selected.filter((p) => p !== key) : [...selected, key];
	}

	async function save(e: SubmitEvent) {
		e.preventDefault();
		if (selected.length === 0) { error = 'Select at least one permission.'; return; }
		saving = true; error = '';
		try {
			const body = { name, description, permissions: selected };
			if (editing) await spApi.put(`/api/v1/sp/roles/${editing.id}`, body);
			else await spApi.post('/api/v1/sp/roles', body);
			success = editing ? 'Role updated.' : 'Role created.';
			modalOpen = false;
			await load();
			setTimeout(() => (success = ''), 5000);
		} catch (e) {
			error = e instanceof SpApiError ? e.message : 'Unable to save role.';
		} finally {
			saving = false;
		}
	}

	async function remove(role: Role) {
		if (!confirm(`Delete role "${role.name}"?`)) return;
		try {
			await spApi.delete(`/api/v1/sp/roles/${role.id}`);
			success = 'Role deleted.';
			await load();
			setTimeout(() => (success = ''), 5000);
		} catch (e) {
			error = e instanceof SpApiError ? e.message : 'Unable to delete role.';
		}
	}

	onMount(load);
</script>

<svelte:head><title>Roles — SMS Whiteinfo S P Reseller</title></svelte:head>

<div class="space-y-4">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-bold tracking-tight text-gray-950 dark:text-white">Roles</h1>
			<p class="text-xs text-gray-500 dark:text-gray-400">Granular permissions enforced by the SP API for every employee.</p>
		</div>
		{#if spAuth.can('roles.manage')}
			<button type="button" onclick={openCreate} class="rounded-lg bg-amber-500 px-3 py-1.5 text-xs font-semibold text-white hover:bg-amber-600">Add Role</button>
		{/if}
	</div>

	{#if error}<div class="rounded-xl border border-rose-200 bg-rose-50 p-3 text-xs text-rose-700 dark:border-rose-900 dark:bg-rose-950/40 dark:text-rose-300">{error}</div>{/if}
	{#if success}<div class="rounded-xl border border-emerald-200 bg-emerald-50 p-3 text-xs text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-300">{success}</div>{/if}

	<div class="grid gap-3 md:grid-cols-2">
		{#if loading}
			<p class="text-xs text-gray-400">Loading roles…</p>
		{:else if roles.length === 0}
			<p class="text-xs text-gray-400">No roles created yet.</p>
		{:else}
			{#each roles as role (role.id)}
				<div class="rounded-xl border border-gray-200 bg-white p-4 shadow-xs dark:border-gray-800 dark:bg-gray-900">
					<div class="flex items-start justify-between gap-2">
						<div>
							<h3 class="text-sm font-bold text-gray-900 dark:text-white">{role.name}</h3>
							<p class="mt-0.5 text-[11px] text-gray-400">{role.description || 'No description'}</p>
						</div>
						<span class="rounded-full bg-amber-100 px-2 py-0.5 text-[10px] font-semibold text-amber-800 dark:bg-amber-950 dark:text-amber-300">{role.users_count} assigned</span>
					</div>
					<p class="mt-3 text-[11px] text-gray-500 dark:text-gray-400">{role.permissions?.length ?? 0} permissions: {(role.permissions ?? []).join(', ') || '—'}</p>
					{#if spAuth.can('roles.manage')}
						<div class="mt-3 flex gap-2">
							<button type="button" onclick={() => openEdit(role)} class="rounded-lg border border-gray-200 px-2 py-1 text-[11px] font-semibold text-gray-600 dark:border-gray-700 dark:text-gray-300">Edit</button>
							<button type="button" onclick={() => remove(role)} class="rounded-lg border border-rose-200 px-2 py-1 text-[11px] font-semibold text-rose-600 dark:border-rose-900">Delete</button>
						</div>
					{/if}
				</div>
			{/each}
		{/if}
	</div>
</div>

{#if modalOpen}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-gray-950/50 p-4">
		<div class="max-h-[90vh] w-full max-w-2xl overflow-y-auto space-y-4 rounded-2xl bg-white p-6 shadow-2xl dark:bg-gray-900">
			<div class="flex items-center justify-between border-b border-gray-100 pb-3 dark:border-gray-800">
				<h3 class="text-sm font-bold text-gray-900 dark:text-white">{editing ? 'Edit Role' : 'Add Role'}</h3>
				<button type="button" onclick={() => (modalOpen = false)} class="text-gray-400">✕</button>
			</div>
			{#if error}<p class="rounded-lg bg-rose-50 p-3 text-xs text-rose-700 dark:bg-rose-950/40 dark:text-rose-300">{error}</p>{/if}
			<form onsubmit={save} class="space-y-4 text-xs">
				<div><label for="role-name" class="font-medium text-gray-700 dark:text-gray-300">Role name</label><input id="role-name" bind:value={name} required class="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2 dark:border-gray-700 dark:bg-gray-800 dark:text-white" /></div>
				<div><label for="role-desc" class="font-medium text-gray-700 dark:text-gray-300">Description</label><textarea id="role-desc" bind:value={description} rows="2" class="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2 dark:border-gray-700 dark:bg-gray-800 dark:text-white"></textarea></div>
				<div class="space-y-3">
					{#each Object.entries(permissionGroups) as [group, permissions]}
						<div>
							<p class="mb-1 text-[11px] font-bold uppercase text-gray-400">{group}</p>
							<div class="grid gap-1.5 sm:grid-cols-2">
								{#each permissions as permission}
									<label class="flex items-center gap-2 text-gray-600 dark:text-gray-300">
										<input type="checkbox" checked={selected.includes(permission.key)} onchange={() => toggle(permission.key)} />
										{permission.label}
									</label>
								{/each}
							</div>
						</div>
					{/each}
				</div>
				<div class="flex justify-end gap-2">
					<button type="button" onclick={() => (modalOpen = false)} class="rounded-lg border border-gray-200 px-3 py-1.5 font-semibold text-gray-600 dark:border-gray-700 dark:text-gray-300">Cancel</button>
					<button type="submit" disabled={saving} class="rounded-lg bg-amber-500 px-4 py-1.5 font-semibold text-white disabled:opacity-50">{saving ? 'Saving…' : 'Save'}</button>
				</div>
			</form>
		</div>
	</div>
{/if}
