<script lang="ts">
	import LegacyTable from '$lib/components/admin/LegacyTable.svelte';

	const columns = [
		{ key: 'created_at', label: 'Timestamp', format: 'datetime' as const },
		{ key: 'customer', label: 'Customer' },
		{ key: 'recipient', label: 'Recipient', format: 'mono' as const },
		{ key: 'sender_id', label: 'Sender ID', format: 'badge' as const },
		{ key: 'parts', label: 'Parts', format: 'number' as const, align: 'center' as const },
		{ key: 'status', label: 'Status', format: 'badge' as const, align: 'center' as const },
		{ key: 'cost', label: 'Customer Cost', format: 'currency' as const, align: 'right' as const },
		{ key: 'gateway', label: 'Route / Gateway' }
	];

	const statusOptions = [
		{ value: 'pending', label: 'Pending' },
		{ value: 'sending', label: 'Sending' },
		{ value: 'sent', label: 'Sent' },
		{ value: 'delivered', label: 'Delivered' },
		{ value: 'failed', label: 'Failed' },
		{ value: 'dlr_failed', label: 'DLR Failed' }
	];
</script>

<svelte:head>
	<title>All SMS Messages — SMS Whiteinfo Administrator</title>
</svelte:head>

<LegacyTable
	title="All SMS Messages"
	endpoint="/api/v1/superman/messages"
	{columns}
	{statusOptions}
	emptyText="No SMS messages found."
/>
