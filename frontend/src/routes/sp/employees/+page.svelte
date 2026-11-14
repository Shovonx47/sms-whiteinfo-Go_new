<script lang="ts">
	import { onMount } from 'svelte';
	import { spApi, SpApiError } from '$lib/sp-api.js';
	import { spAuth } from '$lib/stores/spAuth.svelte.js';

	interface Employee { id: number; name: string; email: string; phone: string; sp_role_id: number; role: string; status: string; }
	interface Role { id: number; name: string; }

	let employees = $state<Employee[]>([]);
	let roles = $state<Role[]>([]);
	let loading = $state(true);
	let error = $state('');
	let success = $state('');
	let modalOpen = $state(false);
	let editing = $state<Employee | null>(null);
	let name = $state('');
	let email = $state('');
	let phone = $state('');
	let password = $state('');
	let status = $state('approved');
	let roleId = $state('');
	let saving = $state(false);

	async function load() {
		loading = true;
		error = '';
		try {
			const [employeeRes, roleRes] = await Promise.all([
				spApi.get<{ data: Employee[] }>('/api/v1/sp/employees'),
				spApi.get<{ data: Role[] }>('/api/v1/sp/roles')
			]);
			employees = employeeRes.data ?? [];
			roles = roleRes.data ?? [];
		} catch (e) {
			error = e instanceof SpApiError ? e.message : 'Unable to load employees.';
		} finally {
			loading = false;
		}
	}

	function openCreate() {
		editing = null; name = ''; email = ''; phone = ''; password = ''; status = 'approved'; roleId = ''; error = ''; modalOpen = true;
	}

	function openEdit(employee: Employee) {
		editing = employee; name = employee.name; email = employee.email; phone = employee.phone;
		password = ''; status = employee.status; roleId = employee.sp_role_id ? String(employee.sp_role_id) : ''; error = ''; modalOpen = true;
	}

	async function save(e: SubmitEvent) {
		e.preventDefault();
		saving = true; error = '';
		try {
			const body: Record<string, unknown> = { name, email, phone, status, sp_role_id: roleId ? Number(roleId) : null };
			if (password) body.password = password;
			if (editing) await spApi.put(`/api/v1/sp/employees/${editing.id}`, body);
			else await spApi.post('/api/v1/sp/employees', body);
			success = editing ? 'Employee updated.' : 'Employee created.';
			modalOpen = false;
			await load();
			setTimeout(() => (success = ''), 5000);
		} catch (e) {
			error = e instanceof SpApiError ? e.message : 'Unable to save employee.';
		} finally {
			saving = false;
		}
	}

	async function remove(employee: Employee) {
		if (!confirm(`Delete ${employee.name}?`)) return;
		try {
			await spApi.delete(`/api/v1/sp/employees/${employee.id}`);
			success = 'Employee deleted.';
			await load();
			setTimeout(() => (success = ''), 5000);
		} catch (e) {
			error = e instanceof SpApiError ? e.message : 'Unable to delete employee.';
		}
	}

	onMount(load);
</script>

<svelte:head><title>Employees — SMS Whiteinfo S P Reseller</title></svelte:head>

<div class="space-y-4">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-bold tracking-tight text-gray-950 dark:text-white">Employees</h1>
			<p class="text-xs text-gray-500 dark:text-gray-400">Staff accounts that can sign in to this reseller panel.</p>
		</div>
		{#if spAuth.can('employees.create')}
			<button type="button" onclick={openCreate} class="rounded-lg bg-amber-500 px-3 py-1.5 text-xs font-semibold text-white hover:bg-amber-600">Add Employee</button>
		{/if}
	</div>

	{#if error}<div class="rounded-xl border border-rose-200 bg-rose-50 p-3 text-xs text-rose-700 dark:border-rose-900 dark:bg-rose-950/40 dark:text-rose-300">{error}</div>{/if}
	{#if success}<div class="rounded-xl border border-emerald-200 bg-emerald-50 p-3 text-xs text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-300">{success}</div>{/if}

	<div class="overflow-x-auto rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
		<table class="w-full text-left text-xs">
			<thead><tr class="border-b border-gray-100 text-[10px] uppercase text-gray-400 dark:border-gray-800">
				<th class="px-4 py-3">Name</th><th class="px-4 py-3">Email</th><th class="px-4 py-3">Phone</th><th class="px-4 py-3">Role</th><th class="px-4 py-3">Status</th><th class="px-4 py-3 text-right">Actions</th>
			</tr></thead>
			<tbody class="divide-y divide-gray-100 dark:divide-gray-800">
				{#if loading}
					<tr><td colspan="6" class="px-4 py-10 text-center text-gray-400">Loading employees…</td></tr>
				{:else if employees.length === 0}
					<tr><td colspan="6" class="px-4 py-10 text-center text-gray-400">No employees yet.</td></tr>
				{:else}
					{#each employees as employee (employee.id)}
						<tr>
							<td class="px-4 py-3 font-semibold text-gray-900 dark:text-white">{employee.name}</td>
							<td class="px-4 py-3 text-gray-600 dark:text-gray-300">{employee.email}</td>
							<td class="px-4 py-3 font-mono text-gray-600 dark:text-gray-300">{employee.phone || '—'}</td>
							<td class="px-4 py-3">{employee.role || 'No role'}</td>
							<td class="px-4 py-3">{employee.status}</td>
							<td class="px-4 py-3 text-right whitespace-nowrap">
								{#if spAuth.can('employees.edit')}<button type="button" onclick={() => openEdit(employee)} class="rounded-lg border border-gray-200 px-2 py-1 font-semibold text-gray-600 dark:border-gray-700 dark:text-gray-300">Edit</button>{/if}
								{#if spAuth.can('employees.delete')}<button type="button" onclick={() => remove(employee)} class="ml-1 rounded-lg border border-rose-200 px-2 py-1 font-semibold text-rose-600 dark:border-rose-900">Delete</button>{/if}
							</td>
						</tr>
					{/each}
				{/if}
			</tbody>
		</table>
	</div>
</div>

{#if modalOpen}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-gray-950/50 p-4">
		<div class="w-full max-w-md space-y-4 rounded-2xl bg-white p-6 shadow-2xl dark:bg-gray-900">
			<div class="flex items-center justify-between border-b border-gray-100 pb-3 dark:border-gray-800">
				<h3 class="text-sm font-bold text-gray-900 dark:text-white">{editing ? 'Edit Employee' : 'Add Employee'}</h3>
				<button type="button" onclick={() => (modalOpen = false)} class="text-gray-400">✕</button>
			</div>
			{#if error}<p class="rounded-lg bg-rose-50 p-3 text-xs text-rose-700 dark:bg-rose-950/40 dark:text-rose-300">{error}</p>{/if}
			<form onsubmit={save} class="space-y-3 text-xs">
				<div><label for="emp-name" class="font-medium text-gray-700 dark:text-gray-300">Name</label><input id="emp-name" bind:value={name} required class="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2 dark:border-gray-700 dark:bg-gray-800 dark:text-white" /></div>
				<div><label for="emp-email" class="font-medium text-gray-700 dark:text-gray-300">Email</label><input id="emp-email" type="email" bind:value={email} required class="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2 dark:border-gray-700 dark:bg-gray-800 dark:text-white" /></div>
				<div><label for="emp-phone" class="font-medium text-gray-700 dark:text-gray-300">Phone</label><input id="emp-phone" bind:value={phone} class="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2 dark:border-gray-700 dark:bg-gray-800 dark:text-white" /></div>
				<div><label for="emp-role" class="font-medium text-gray-700 dark:text-gray-300">Role</label><select id="emp-role" bind:value={roleId} class="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2 dark:border-gray-700 dark:bg-gray-800 dark:text-white"><option value="">No role</option>{#each roles as role (role.id)}<option value={role.id}>{role.name}</option>{/each}</select></div>
				<div><label for="emp-status" class="font-medium text-gray-700 dark:text-gray-300">Status</label><select id="emp-status" bind:value={status} class="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2 dark:border-gray-700 dark:bg-gray-800 dark:text-white"><option value="approved">Approved</option><option value="pending">Pending</option><option value="banned">Banned</option></select></div>
				<div><label for="emp-password" class="font-medium text-gray-700 dark:text-gray-300">Password {editing ? '(leave blank to keep)' : ''}</label><input id="emp-password" type="password" bind:value={password} required={!editing} class="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2 dark:border-gray-700 dark:bg-gray-800 dark:text-white" /></div>
				<div class="flex justify-end gap-2 pt-2">
					<button type="button" onclick={() => (modalOpen = false)} class="rounded-lg border border-gray-200 px-3 py-1.5 font-semibold text-gray-600 dark:border-gray-700 dark:text-gray-300">Cancel</button>
					<button type="submit" disabled={saving} class="rounded-lg bg-amber-500 px-4 py-1.5 font-semibold text-white disabled:opacity-50">{saving ? 'Saving…' : 'Save'}</button>
				</div>
			</form>
		</div>
	</div>
{/if}
