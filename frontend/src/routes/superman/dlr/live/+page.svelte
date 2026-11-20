<script lang="ts">
	import LegacyTable from '$lib/components/admin/LegacyTable.svelte';

	const columns = [
		{ key: 'delivered_at', label: 'Handset Delivered At', format: 'datetime' as const },
		{ key: 'recipient', label: 'Recipient MSISDN', format: 'mono' as const },
		{ key: 'sender_id', label: 'Sender' },
		{ key: 'gateway', label: 'Delivered via Gateway' },
		{ key: 'customer', label: 'Customer' },
		{ key: 'status', label: 'Status', format: 'badge' as const, align: 'center' as const }
	];
</script>

<svelte:head>
	<title>Real-Time Delivery Report (DLR) Stream — SMS Whiteinfo Administrator</title>
</svelte:head>

<LegacyTable
	title="Real-Time Delivery Report (DLR) Stream"
	endpoint="/api/v1/superman/dlr/live"
	{columns}
	emptyText="No delivered messages reported yet."
/>
