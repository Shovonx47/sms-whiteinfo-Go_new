<script lang="ts">
	import LegacyTable from '$lib/components/admin/LegacyTable.svelte';

	const columns = [
		{ key: 'name', label: 'Campaign' },
		{ key: 'customer', label: 'Customer' },
		{ key: 'status', label: 'Status', format: 'badge' as const, align: 'center' as const },
		{ key: 'completed_at', label: 'Completed At', format: 'datetime' as const },
		{ key: 'created_at', label: 'Launched At', format: 'datetime' as const }
	];
</script>

<svelte:head>
	<title>Campaign Analytics &amp; Delivery Reports — SMS Whiteinfo Administrator</title>
</svelte:head>

<LegacyTable
	title="Campaign Analytics & Delivery Reports"
	endpoint="/api/v1/superman/campaigns"
	{columns}
	emptyText="No campaign reports found."
/>
