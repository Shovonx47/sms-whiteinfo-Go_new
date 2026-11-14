<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';

	const MARKER = 'sms_impersonation';

	let info = $state<{ portal: string; name?: string; email?: string; returnTo?: string } | null>(null);
	let leaving = $state(false);
	let visible = $derived(info !== null && !page.url.pathname.startsWith('/superman'));

	onMount(() => {
		try {
			const raw = sessionStorage.getItem(MARKER);
			info = raw ? JSON.parse(raw) : null;
		} catch {
			info = null;
		}
	});

	async function leave() {
		if (!info || leaving) return;
		leaving = true;
		const returnTo = info.returnTo || '/superman';
		try {
			if (info.portal === 'sp') {
				const { spApi } = await import('$lib/sp-api.js');
				try {
					await spApi.post('/api/v1/sp/auth/logout');
				} catch {
					// best effort
				}
				spApi.clearToken();
			} else if (info.portal === 'customer') {
				const token = sessionStorage.getItem('sms_customer_session_token');
				if (token) {
					try {
						await fetch('/api/v1/customer/logout', {
							method: 'POST',
							headers: { Accept: 'application/json', Authorization: `Bearer ${token}` }
						});
					} catch {
						// best effort
					}
				}
				sessionStorage.removeItem('sms_customer_session_token');
				localStorage.removeItem('sms_customer_access_token');
			}
		} finally {
			sessionStorage.removeItem(MARKER);
			window.location.assign(returnTo);
		}
	}
</script>

{#if visible && info}
	<header
		class="w-full shrink-0 z-50 flex items-center justify-between bg-[#161a29] px-4 py-2 text-white border-b border-gray-900 shadow-sm"
	>
		<div class="flex items-center gap-2.5 min-w-0">
			<span
				class="inline-flex items-center gap-1.5 rounded-full border border-red-500/40 bg-red-950/40 px-2.5 py-0.5 text-[11px] font-semibold text-red-400 shrink-0"
			>
				<svg class="h-3.5 w-3.5 text-red-400 shrink-0" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z" />
				</svg>
				IMPERSONATION MODE
			</span>
			<span class="truncate text-xs font-medium text-white">Logged in as {info.name ?? 'User'}</span>
		</div>
		<button
			type="button"
			onclick={leave}
			disabled={leaving}
			class="inline-flex items-center gap-1.5 rounded-md bg-[#e11d48] hover:bg-rose-700 px-3 py-1 text-xs font-semibold text-white transition shadow-xs disabled:opacity-60 shrink-0 cursor-pointer"
		>
			<svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke-width="2.5" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" d="M10.5 19.5 3 12m0 0 7.5-7.5M3 12h18" />
			</svg>
			<span>Back to Admin</span>
		</button>
	</header>
{/if}
