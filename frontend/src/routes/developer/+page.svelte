<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte.js';

	type Lang = 'curl' | 'php' | 'python' | 'node';
	let activeSnippet = $state<Lang>('curl');

	const baseUrl = typeof window !== 'undefined' ? window.location.origin : 'https://sms.whiteinfobd.com';

	const snippets: Record<Lang, string> = {
		curl: `curl -X POST "${baseUrl}/api/v1/messages" \\
  -H "Authorization: Bearer 1|your_access_token_here" \\
  -H "Content-Type: application/json" \\
  -d '{
    "sender_id": "WhiteINFO",
    "numbers": "01711000000,01811000000",
    "body": "Your OTP code is 482910. Valid for 5 minutes."
  }'`,
		php: `<?php
$ch = curl_init("${baseUrl}/api/v1/messages");
curl_setopt_array($ch, [
    CURLOPT_POST => true,
    CURLOPT_RETURNTRANSFER => true,
    CURLOPT_HTTPHEADER => [
        "Authorization: Bearer 1|your_access_token_here",
        "Content-Type: application/json",
    ],
    CURLOPT_POSTFIELDS => json_encode([
        "sender_id" => "WhiteINFO",
        "numbers"   => "01711000000",
        "body"      => "Hello from PHP",
    ]),
]);
$response = curl_exec($ch);`,
		python: `import requests

url = "${baseUrl}/api/v1/messages"
headers = {
    "Authorization": "Bearer 1|your_access_token_here",
    "Content-Type": "application/json",
}
payload = {
    "sender_id": "WhiteINFO",
    "numbers": ["01711000000", "01811000000"],
    "body": "Hello from Python!",
}
res = requests.post(url, json=payload, headers=headers)
print(res.json())`,
		node: `const res = await fetch('${baseUrl}/api/v1/messages', {
  method: 'POST',
  headers: {
    'Authorization': 'Bearer 1|your_access_token_here',
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    sender_id: 'WhiteINFO',
    numbers: '01711000000',
    body: 'Hello from Node.js!'
  })
});
const data = await res.json();
console.log(data);`
	};

	const endpoints = [
		{
			method: 'GET',
			path: '/api/v1/me',
			summary: 'Get authenticated account details (name, email, status, balance, api_access).'
		},
		{
			method: 'GET',
			path: '/api/v1/balance',
			summary: 'Retrieve current BDT wallet balance and currency.'
		},
		{
			method: 'GET',
			path: '/api/v1/senders',
			summary: 'List all enabled and approved Sender IDs assigned to your account.'
		},
		{
			method: 'GET / POST',
			path: '/api/v1/messages',
			summary: 'Dispatch single or bulk SMS (immediate or scheduled). Supports GET query string, JSON, or form-data.'
		},
		{
			method: 'GET',
			path: '/api/v1/batches/{batch}',
			summary: 'Check batch status, message delivery breakdown, parts, and billed cost.'
		},
		{
			method: 'GET',
			path: '/api/v1/ip-whitelists',
			summary: 'List configured IP whitelist rules (accessible even from non-whitelisted IPs).'
		},
		{
			method: 'POST',
			path: '/api/v1/ip-whitelists',
			summary: 'Register a public IPv4/IPv6 address or CIDR block (/16-/32 IPv4, /48-/128 IPv6).'
		},
		{
			method: 'DELETE',
			path: '/api/v1/ip-whitelists/{id}',
			summary: 'Remove an IP whitelist rule from your account.'
		}
	];
</script>

<svelte:head>
	<title>API Documentation - White INFO</title>
</svelte:head>

<div class="space-y-6">
	<div>
		<h1 class="text-2xl font-bold tracking-tight text-gray-900">Developer REST API Documentation</h1>
		<p class="mt-1 text-sm text-gray-600">
			Integrate programmatic SMS dispatch, balance lookups, and IP allowlist management into your applications.
		</p>
	</div>

	{#if auth.user && auth.user.api_access === false}
		<div class="rounded-xl border border-amber-200 bg-amber-50 p-5 text-sm text-amber-900">
			<p class="font-semibold">API Access Is Currently Disabled</p>
			<p class="mt-1 text-xs text-amber-800">
				Your account does not have API access enabled. Please contact your administrator or service provider to enable API access before generating keys or sending API requests.
			</p>
		</div>
	{:else}
		<div class="flex flex-wrap gap-3">
			<a
				href="/developer/access-tokens"
				class="inline-flex items-center gap-2 rounded-lg bg-teal-600 px-4 py-2 text-xs font-semibold text-white shadow-xs transition hover:bg-teal-500"
			>
				Manage API Tokens
			</a>
			<a
				href="/developer/ip-whitelists"
				class="inline-flex items-center gap-2 rounded-lg border border-gray-300 bg-white px-4 py-2 text-xs font-semibold text-gray-700 shadow-xs transition hover:bg-gray-50"
			>
				Manage IP Whitelist
			</a>
		</div>
	{/if}

	<!-- Authentication & Response Format -->
	<div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
		<div class="rounded-xl border border-gray-200 bg-white p-5 shadow-xs space-y-3">
			<h2 class="text-sm font-bold text-gray-900">Authentication &amp; Security</h2>
			<p class="text-xs text-gray-600 leading-relaxed">
				Authenticate using an API token formatted as <code class="font-mono text-teal-700">&lt;id&gt;|&lt;token&gt;</code> in the
				<code class="font-mono bg-gray-100 px-1 py-0.5 rounded">Authorization: Bearer ...</code> header (or
				<code class="font-mono bg-gray-100 px-1 py-0.5 rounded">api_token</code> /
				<code class="font-mono bg-gray-100 px-1 py-0.5 rounded">api_key</code> query/form parameter).
			</p>
			<p class="text-xs text-gray-600 leading-relaxed">
				When IP whitelisting is enforced, requests must originate from an <strong>approved</strong> public IP/CIDR entry. The
				<code class="font-mono text-teal-700">/api/v1/ip-whitelists</code> endpoints bypass IP checks so servers can bootstrap access.
			</p>
		</div>

		<div class="rounded-xl border border-gray-200 bg-white p-5 shadow-xs space-y-2">
			<h2 class="text-sm font-bold text-gray-900">Standard JSON Envelope</h2>
			<pre class="rounded-lg bg-gray-950 p-3 font-mono text-[11px] text-emerald-300 overflow-x-auto">{`{
  "status": "success",
  "code": 200,
  "message": "Message accepted for delivery",
  "data": {
    "message_id": 10421,
    "batch_id": 882,
    "recipients_count": 2,
    "status": "pending",
    "estimated_cost": 0.70
  }
}`}</pre>
		</div>
	</div>

	<!-- Endpoints Reference -->
	<div class="rounded-xl border border-gray-200 bg-white shadow-xs overflow-hidden">
		<div class="border-b border-gray-200 px-5 py-3.5 bg-gray-50/70">
			<h2 class="text-sm font-bold text-gray-900">API v1 Endpoints</h2>
		</div>
		<div class="divide-y divide-gray-100">
			{#each endpoints as ep (ep.path)}
				<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2 px-5 py-3.5 text-xs">
					<div class="flex items-center gap-3">
						<span class="inline-flex min-w-24 justify-center rounded-md bg-teal-50 px-2 py-0.5 font-mono font-bold text-teal-700 ring-1 ring-inset ring-teal-600/20">
							{ep.method}
						</span>
						<code class="font-mono font-semibold text-gray-900">{ep.path}</code>
					</div>
					<p class="text-gray-600 sm:text-right">{ep.summary}</p>
				</div>
			{/each}
		</div>
	</div>

	<!-- Send Messages Parameters -->
	<div class="rounded-xl border border-gray-200 bg-white shadow-xs overflow-hidden">
		<div class="border-b border-gray-200 px-5 py-3.5 bg-gray-50/70">
			<h2 class="text-sm font-bold text-gray-900">Send SMS Parameters (<code class="font-mono text-teal-700">/api/v1/messages</code>)</h2>
		</div>
		<div class="overflow-x-auto">
			<table class="w-full text-left text-xs border-collapse">
				<thead class="border-b border-gray-100 bg-gray-50/40 text-gray-600 font-semibold uppercase">
					<tr>
						<th class="px-5 py-2.5">Parameter</th>
						<th class="px-5 py-2.5">Aliases</th>
						<th class="px-5 py-2.5">Required</th>
						<th class="px-5 py-2.5">Description</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-100">
					<tr>
						<td class="px-5 py-2.5 font-mono font-semibold text-gray-900">sender_id</td>
						<td class="px-5 py-2.5 font-mono text-gray-500">—</td>
						<td class="px-5 py-2.5">Yes</td>
						<td class="px-5 py-2.5 text-gray-600">Approved Sender ID string (masking name or non-masking CLI number).</td>
					</tr>
					<tr>
						<td class="px-5 py-2.5 font-mono font-semibold text-gray-900">numbers</td>
						<td class="px-5 py-2.5 font-mono text-gray-500">phone, to</td>
						<td class="px-5 py-2.5">Yes</td>
						<td class="px-5 py-2.5 text-gray-600">Comma-separated string or JSON array of recipient numbers (e.g. 01711000000).</td>
					</tr>
					<tr>
						<td class="px-5 py-2.5 font-mono font-semibold text-gray-900">body</td>
						<td class="px-5 py-2.5 font-mono text-gray-500">message</td>
						<td class="px-5 py-2.5">Yes</td>
						<td class="px-5 py-2.5 text-gray-600">SMS message text (GSM-7 or Unicode).</td>
					</tr>
					<tr>
						<td class="px-5 py-2.5 font-mono font-semibold text-gray-900">schedule</td>
						<td class="px-5 py-2.5 font-mono text-gray-500">scheduled_at</td>
						<td class="px-5 py-2.5">No</td>
						<td class="px-5 py-2.5 text-gray-600">Future dispatch timestamp (YYYY-MM-DD HH:MM:SS or ISO-8601).</td>
					</tr>
					<tr>
						<td class="px-5 py-2.5 font-mono font-semibold text-gray-900">template_id</td>
						<td class="px-5 py-2.5 font-mono text-gray-500">—</td>
						<td class="px-5 py-2.5">No</td>
						<td class="px-5 py-2.5 text-gray-600">Approved template ID owned by your account to bypass manual SMS moderation queue.</td>
					</tr>
				</tbody>
			</table>
		</div>
	</div>

	<!-- Code Snippets -->
	<div class="rounded-xl border border-gray-200 bg-white shadow-xs overflow-hidden">
		<div class="flex items-center justify-between border-b border-gray-200 px-5 py-3 bg-gray-50/70">
			<span class="text-sm font-bold text-gray-900">Quickstart Examples</span>
			<div class="flex items-center gap-1 rounded-lg bg-gray-200/70 p-0.5 text-xs">
				{#each [['curl', 'cURL'], ['php', 'PHP'], ['python', 'Python'], ['node', 'Node.js']] as [id, label]}
					<button
						type="button"
						onclick={() => (activeSnippet = id as Lang)}
						class="rounded-md px-2.5 py-1 font-medium transition cursor-pointer {activeSnippet === id ? 'bg-white text-gray-900 shadow-xs' : 'text-gray-600 hover:text-gray-900'}"
					>
						{label}
					</button>
				{/each}
			</div>
		</div>
		<div class="p-5 bg-gray-950 font-mono text-xs text-gray-200 overflow-x-auto">
			<pre><code>{snippets[activeSnippet]}</code></pre>
		</div>
	</div>
</div>
