<script lang="ts">
	import LegacyTable from '$lib/components/admin/LegacyTable.svelte';

	const columns = [
		{ key: 'sent_at', label: 'Dispatched At', format: 'datetime' as const },
		{ key: 'recipient', label: 'Recipient', format: 'mono' as const },
		{ key: 'gateway_message_id', label: 'Gateway Reference', format: 'mono' as const },
		{ key: 'gateway', label: 'Route' },
		{ key: 'status', label: 'Status', format: 'badge' as const, align: 'center' as const }
	];
</script>

<svelte:head>
	<title>Pending DLR Polling Queue — SMS Whiteinfo Administrator</title>
</svelte:head>

<LegacyTable
	title="Pending DLR Polling Queue"
	endpoint="/api/v1/superman/dlr/queue"
	{columns}
	emptyText="All dispatched messages have received delivery reports."
/>
