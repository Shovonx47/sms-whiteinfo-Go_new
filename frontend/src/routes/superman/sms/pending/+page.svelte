<script lang="ts">
	import LegacyTable from '$lib/components/admin/LegacyTable.svelte';

	const columns = [
		{ key: 'created_at', label: 'Queued At', format: 'datetime' as const },
		{ key: 'customer', label: 'Customer' },
		{ key: 'recipient', label: 'Recipient', format: 'mono' as const },
		{ key: 'sender_id', label: 'Sender' },
		{ key: 'status', label: 'Status', format: 'badge' as const, align: 'center' as const },
		{ key: 'cost', label: 'Estimated Cost', format: 'currency' as const, align: 'right' as const }
	];
</script>

<svelte:head>
	<title>Pending &amp; Queueing SMS — SMS Whiteinfo Administrator</title>
</svelte:head>

<LegacyTable
	title="Pending & Queueing SMS"
	endpoint="/api/v1/superman/messages?statuses=pending,sending"
	{columns}
	emptyText="The outbound queue is completely clear and idle."
/>
