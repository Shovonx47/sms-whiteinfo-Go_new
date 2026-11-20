<script lang="ts">
	import LegacyTable from '$lib/components/admin/LegacyTable.svelte';
	import { supermanApi } from '$lib/superman-api.js';

	let refresh = $state(0);

	const columns = [
		{ key: 'name', label: 'Affected Provider' },
		{ key: 'driver', label: 'Protocol', format: 'badge' as const },
		{ key: 'health_status', label: 'Outage State', format: 'badge' as const, align: 'center' as const },
		{ key: 'last_health_response', label: 'Failure Reason / Response' },
		{ key: 'updated_at', label: 'Incident Time', format: 'datetime' as const }
	];

	const actions = [
		{
			label: 'Restore / Enable',
			tone: 'success' as const,
			run: async (row: Record<string, any>) => {
				if (!confirm(`Restore ${row.name} and mark it healthy?`)) return;
				await supermanApi.post(`/api/v1/superman/alerts/providers/${row.id}/restore`);
				refresh += 1;
			}
		}
	];
</script>

<svelte:head>
	<title>Gateway &amp; Upstream Provider Outages — SMS Whiteinfo Administrator</title>
</svelte:head>

{#key refresh}
	<LegacyTable
		title="Gateway & Upstream Provider Outages"
		endpoint="/api/v1/superman/alerts/providers"
		{columns}
		{actions}
		searchable={false}
		emptyText="All upstream SMS gateways and telco routes are connected and healthy."
	/>
{/key}
