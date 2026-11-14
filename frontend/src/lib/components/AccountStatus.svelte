<script lang="ts">
	interface Props {
		tone?: 'amber' | 'red' | 'gray';
		icon?: string;
		title: string;
		heading: string;
		body: string;
	}

	let { tone = 'amber', icon = '!', title, heading, body }: Props = $props();

	const tones = {
		amber: 'bg-amber-100 text-amber-600 dark:bg-amber-500/15 dark:text-amber-400',
		red: 'bg-red-100 text-red-600 dark:bg-red-500/15 dark:text-red-400',
		gray: 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-300'
	};
</script>

<div class="mx-auto flex max-w-2xl flex-col items-center px-4 py-16 text-center">
	<div class="flex h-16 w-16 items-center justify-center rounded-full text-2xl font-semibold {tones[tone]}">
		{icon}
	</div>
	<p class="mt-6 text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">{title}</p>
	<h1 class="mt-2 text-2xl font-bold text-gray-900 dark:text-white">{heading}</h1>
	<p class="mt-3 max-w-xl text-sm leading-relaxed text-gray-600 dark:text-gray-400">{body}</p>
</div>
