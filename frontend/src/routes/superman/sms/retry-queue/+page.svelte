<script lang="ts">
	import LegacyTable from '$lib/components/admin/LegacyTable.svelte';

	const columns = [
		{ key: 'created_at', label: 'Failed At', format: 'datetime' as const },
		{ key: 'recipient', label: 'Recipient', format: 'mono' as const },
		{ key: 'customer', label: 'Customer' },
		{ key: 'sender_id', label: 'Sender' },
		{ key: 'status', label: 'Status', format: 'badge' as const }
	];
</script>

<svelte:head>
	<title>Automatic Retry Queue — SMS Whiteinfo Administrator</title>
</svelte:head>

<LegacyTable
	title="Automatic Retry Queue"
	endpoint="/api/v1/superman/messages?statuses=failed"
	{columns}
	emptyText="No messages currently awaiting retry dispatch."
/>
