<script lang="ts">
	import '../app.css';
	import Toast from '$lib/components/Toast.svelte';
	import ImpersonationBanner from '$lib/components/ImpersonationBanner.svelte';

	let { children } = $props();
</script>

<div class="flex min-h-screen flex-col">
	<ImpersonationBanner />
	<div class="flex min-h-0 flex-1 flex-col">
		{@render children()}
	</div>
</div>
<Toast />
