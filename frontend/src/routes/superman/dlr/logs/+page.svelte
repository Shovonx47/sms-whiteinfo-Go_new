<script lang="ts">
	import LegacyTable from '$lib/components/admin/LegacyTable.svelte';

	const columns = [
		{ key: 'created_at', label: 'Submitted', format: 'datetime' as const },
		{ key: 'sent_at', label: 'Sent to Gateway', format: 'datetime' as const },
		{ key: 'delivered_at', label: 'Handset Delivery', format: 'datetime' as const },
		{ key: 'recipient', label: 'Recipient', format: 'mono' as const },
		{ key: 'status', label: 'Status', format: 'badge' as const, align: 'center' as const },
		{ key: 'gateway_message_id', label: 'Gateway DLR ID', format: 'mono' as const }
	];

	const statusOptions = [
		{ value: 'delivered', label: 'Delivered' },
		{ value: 'sent', label: 'Sent (DLR Pending)' },
		{ value: 'dlr_failed', label: 'DLR Failed' }
	];
</script>

<svelte:head>
	<title>Historical DLR Logs &amp; Timestamps — SMS Whiteinfo Administrator</title>
</svelte:head>

<LegacyTable
	title="Historical DLR Logs & Timestamps"
	endpoint="/api/v1/superman/dlr/logs"
	{columns}
	{statusOptions}
	emptyText="No delivery logs found."
/>
