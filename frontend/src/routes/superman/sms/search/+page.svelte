<script lang="ts">
	import LegacyTable from '$lib/components/admin/LegacyTable.svelte';

	const columns = [
		{ key: 'created_at', label: 'Date', format: 'datetime' as const },
		{ key: 'recipient', label: 'Recipient', format: 'mono' as const },
		{ key: 'customer', label: 'Customer' },
		{ key: 'sender_id', label: 'Sender' },
		{ key: 'status', label: 'Status', format: 'badge' as const, align: 'center' as const },
		{ key: 'cost', label: 'Cost', format: 'currency' as const, align: 'right' as const }
	];
</script>

<svelte:head>
	<title>Search Messages by Recipient or Text — SMS Whiteinfo Administrator</title>
</svelte:head>

<LegacyTable
	title="Search Messages by Recipient or Text"
	endpoint="/api/v1/superman/messages"
	{columns}
	emptyText="No messages matched your search."
/>
