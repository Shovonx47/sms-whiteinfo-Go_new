<script lang="ts">
	let activeSnippet = $state<'curl' | 'php' | 'python' | 'node'>('curl');

	const snippetCurl = `curl -X POST "https://sms.whiteinfobd.com/api/v1/messages" \\
  -H "Authorization: Bearer YOUR_API_TOKEN" \\
  -H "Content-Type: application/json" \\
  -H "Accept: application/json" \\
  -d '{
    "sender_id": "WHITEINFO",
    "to": "+8801700000000",
    "message": "Your OTP code is 951234."
  }'`;

	const snippetPhp = `$response = Http::withToken('YOUR_API_TOKEN')
    ->post('https://sms.whiteinfobd.com/api/v1/messages', [
        'sender_id' => 'WHITEINFO',
        'to' => '+8801700000000',
        'message' => 'Your OTP code is 951234.',
    ]);

return $response->json();`;

	const snippetPython = `import requests

url = "https://sms.whiteinfobd.com/api/v1/messages"
headers = {
    "Authorization": "Bearer YOUR_API_TOKEN",
    "Content-Type": "application/json"
}
payload = {
    "sender_id": "WHITEINFO",
    "to": "+8801700000000",
    "message": "Your OTP code is 951234."
}

res = requests.post(url, json=payload, headers=headers)
print(res.json())`;

	const snippetNode = `const res = await fetch('https://sms.whiteinfobd.com/api/v1/messages', {
  method: 'POST',
  headers: {
    'Authorization': 'Bearer YOUR_API_TOKEN',
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    sender_id: 'WHITEINFO',
    to: '+8801700000000',
    message: 'Your OTP code is 951234.'
  })
});

const data = await res.json();
console.log(data);`;

	const snippets: Record<string, string> = {
		curl: snippetCurl,
		php: snippetPhp,
		python: snippetPython,
		node: snippetNode
	};
</script>

<svelte:head>
	<title>API Documentation - White INFO</title>
</svelte:head>

<!-- Breadcrumb -->
<div class="mb-2 flex items-center gap-1 text-xs text-gray-500">
	<span class="font-medium text-gray-700">API Documentation</span>
	<span class="text-gray-400">›</span>
	<span>Overview</span>
</div>

<div class="mb-6">
	<h1 class="text-2xl font-bold text-gray-900">API Documentation</h1>
</div>

<!-- HTTP API Quickstart -->
<div class="rounded-xl border border-gray-200 bg-white shadow-sm overflow-hidden">
	<div class="border-b border-gray-100 px-6 py-4 flex items-center justify-between">
		<div>
			<h2 class="text-sm font-bold text-gray-900">HTTP API Quickstart</h2>
			<p class="text-xs text-gray-500 mt-0.5">
				Endpoint:
				<code class="text-teal-600 font-mono">POST https://sms.whiteinfobd.com/api/v1/messages</code>
			</p>
		</div>
		<!-- Snippet Tabs -->
		<div class="flex items-center gap-1 rounded-lg bg-gray-100 p-1">
			{#each ['curl', 'php', 'python', 'node'] as lang (lang)}
				<button
					type="button"
					onclick={() => (activeSnippet = lang as typeof activeSnippet)}
					class="rounded-md px-2.5 py-1 text-xs font-semibold uppercase transition {activeSnippet === lang
						? 'bg-teal-600 text-white shadow-sm'
						: 'text-gray-600 hover:text-gray-900'}"
				>
					{lang}
				</button>
			{/each}
		</div>
	</div>

	<div class="p-6 bg-gray-950 font-mono text-xs text-gray-300 overflow-x-auto">
		<pre><code>{snippets[activeSnippet]}</code></pre>
	</div>
</div>
