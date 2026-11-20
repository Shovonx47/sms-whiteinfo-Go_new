<script lang="ts">
	import { page } from '$app/stores';

	let { collapsed = $bindable(false) }: { collapsed: boolean } = $props();

	let openGroups = $state<Record<string, boolean>>({});

	function toggleGroup(key: string) {
		openGroups[key] = !openGroups[key];
	}

	$effect(() => {
		for (const section of navSections) {
			if (section.href || !section.items) {
				continue;
			}
			if (isGroupActive(section)) {
				openGroups[section.id] = true;
			}
		}
	});

	let currentPath = $derived($page.url.pathname);

	// Heroicons v2 (outline) paths matching the Filament Admin Superman panel
	const icons: Record<string, string> = {
		bolt: `<path stroke-linecap="round" stroke-linejoin="round" d="m3.75 13.5 10.5-11.25L12 10.5h8.25L9.75 21.75 12 13.5H3.75Z" />`,
		user: `<path stroke-linecap="round" stroke-linejoin="round" d="M15.75 6a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0ZM4.501 20.118a7.5 7.5 0 0 1 14.998 0A17.933 17.933 0 0 1 12 21.75c-2.676 0-5.216-.584-7.499-1.632Z" />`,
		identification: `<path stroke-linecap="round" stroke-linejoin="round" d="M15 9h3.75M15 12h3.75M15 15h3.75M4.5 19.5h15a2.25 2.25 0 0 0 2.25-2.25V6.75A2.25 2.25 0 0 0 19.5 4.5h-15a2.25 2.25 0 0 0-2.25 2.25v10.5A2.25 2.25 0 0 0 4.5 19.5Zm6-10.125a1.875 1.875 0 1 1-3.75 0 1.875 1.875 0 0 1 3.75 0Zm1.294 6.336a6.721 6.721 0 0 1-3.17.789 6.721 6.721 0 0 1-3.168-.789 3.376 3.376 0 0 1 6.338 0Z" />`,
		'credit-card': `<path stroke-linecap="round" stroke-linejoin="round" d="M2.25 8.25h19.5M2.25 9h19.5m-16.5 5.25h6m-6 2.25h3m-3.75 3h15a2.25 2.25 0 0 0 2.25-2.25V6.75A2.25 2.25 0 0 0 19.5 4.5h-15a2.25 2.25 0 0 0-2.25 2.25v10.5A2.25 2.25 0 0 0 4.5 19.5Z" />`,
		phone: `<path stroke-linecap="round" stroke-linejoin="round" d="M2.25 6.75c0 8.284 6.716 15 15 15h2.25a2.25 2.25 0 0 0 2.25-2.25v-1.372c0-.516-.351-.966-.852-1.091l-4.423-1.106c-.44-.11-.902.055-1.173.417l-.97 1.293c-.282.376-.769.542-1.21.38a12.035 12.035 0 0 1-7.143-7.143c-.162-.441.004-.928.38-1.21l1.293-.97c.363-.271.527-.734.417-1.173L6.963 3.102a1.125 1.125 0 0 0-1.091-.852H4.5A2.25 2.25 0 0 0 2.25 4.5v2.25Z" />`,
		server: `<path stroke-linecap="round" stroke-linejoin="round" d="M21.75 17.25v-.228a4.5 4.5 0 0 0-.12-1.03l-2.268-9.64a3.375 3.375 0 0 0-3.285-2.602H7.923a3.375 3.375 0 0 0-3.285 2.602l-2.268 9.64a4.5 4.5 0 0 0-.12 1.03v.228m19.5 0a3 3 0 0 1-3 3H5.25a3 3 0 0 1-3-3m19.5 0a3 3 0 0 0-3-3H5.25a3 3 0 0 0-3 3m16.5 0h.008v.008h-.008v-.008Zm-3 0h.008v.008h-.008v-.008Z" />`,
		'document-text': `<path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 0 0-3.375-3.375h-1.5A1.125 1.125 0 0 1 13.5 7.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H8.25m0 12.75h7.5m-7.5 3H12M10.5 2.25H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 0 0-9-9Z" />`,
		'chart-bar': `<path stroke-linecap="round" stroke-linejoin="round" d="M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 0 1 3 19.875v-6.75ZM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 0 1-1.125-1.125V8.625ZM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 0 1-1.125-1.125V4.125Z" />`,
		'chat-bubble': `<path stroke-linecap="round" stroke-linejoin="round" d="M20.25 8.511c.884.284 1.5 1.128 1.5 2.097v4.286c0 1.136-.847 2.1-1.98 2.193-.34.027-.68.052-1.02.072v3.091l-3-3c-1.354 0-2.694-.055-4.02-.163a2.115 2.115 0 0 1-.825-.242m9.345-8.334a2.126 2.126 0 0 0-.476-.095 48.64 48.64 0 0 0-8.048 0c-1.131.094-1.976 1.057-1.976 2.192v4.286c0 .837.46 1.58 1.155 1.951m9.345-8.334V6.637c0-1.621-1.152-3.026-2.76-3.235A48.455 48.455 0 0 0 11.25 3c-2.115 0-4.198.137-6.24.402-1.608.209-2.76 1.614-2.76 3.235v6.226c0 1.621 1.152 3.026 2.76 3.235.577.075 1.157.14 1.74.194V21l4.155-4.155" />`,
		'chat-bubble-oval': `<path stroke-linecap="round" stroke-linejoin="round" d="M12 20.25c4.97 0 9-3.694 9-8.25s-4.03-8.25-9-8.25S3 7.444 3 12c0 2.104.859 4.023 2.273 5.48.432.447.74 1.04.586 1.641a4.483 4.483 0 0 1-.923 1.785A5.969 5.969 0 0 0 6 21c1.282 0 2.47-.402 3.445-1.087.81.22 1.668.337 2.555.337Z" />`,
		users: `<path stroke-linecap="round" stroke-linejoin="round" d="M15 19.128a9.38 9.38 0 0 0 2.625.372 9.337 9.337 0 0 0 4.121-.952 4.125 4.125 0 0 0-7.533-2.493M15 19.128v-.003c0-1.113-.285-2.16-.786-3.07M15 19.128v.106A12.318 12.318 0 0 1 8.624 21c-2.331 0-4.512-.645-6.374-1.766l-.001-.109a6.375 6.375 0 0 1 11.964-3.07M12 6.375a3.375 3.375 0 1 1-6.75 0 3.375 3.375 0 0 1 6.75 0Zm8.25 2.25a2.625 2.625 0 1 1-5.25 0 2.625 2.625 0 0 1 5.25 0Z" />`,
		'user-plus': `<path stroke-linecap="round" stroke-linejoin="round" d="M18 7.5v3m0 0v3m0-3h3m-3 0h-3m-2.25-4.125a3.375 3.375 0 1 1-6.75 0 3.375 3.375 0 0 1 6.75 0ZM3 19.235v-.11a6.375 6.375 0 0 1 12.75 0v.109A12.318 12.318 0 0 1 9.374 21c-2.331 0-4.512-.645-6.374-1.765Z" />`,
		cog: `<path stroke-linecap="round" stroke-linejoin="round" d="M4.5 12a7.5 7.5 0 0 0 15 0m-15 0a7.5 7.5 0 1 1 15 0m-15 0H3m16.5 0H21m-1.5 0H12m-8.457 3.077 1.41-.513m14.095-5.13 1.41-.513M5.106 17.785l1.15-.964m11.49-9.642 1.149-.964M7.501 19.795l.75-1.3m7.5-12.99.75-1.3m-6.063 16.658.26-1.477m2.605-14.772.26-1.477m0 17.726-.26-1.477M10.698 4.614l-.26-1.477M16.5 19.794l-.75-1.299M7.5 4.205 12 12m6.894 5.785-1.149-.964M6.256 7.178l-1.15-.964m15.352 8.864-1.41-.513M4.954 9.435l-1.41-.514M12.002 12l-3.75 6.495" />`,
		'clipboard-check': `<path stroke-linecap="round" stroke-linejoin="round" d="M11.35 3.836c-.065.21-.1.433-.1.664 0 .414.336.75.75.75h4.5a.75.75 0 0 0 .75-.75 2.25 2.25 0 0 0-.1-.664m-5.8 0A2.251 2.251 0 0 1 13.5 2.25H15c1.012 0 1.867.668 2.15 1.586m-5.8 0c-.376.023-.75.05-1.124.08C9.095 4.01 8.25 4.973 8.25 6.108V8.25m8.9-4.414c.376.023.75.05 1.124.08 1.131.094 1.976 1.057 1.976 2.192V16.5A2.25 2.25 0 0 1 18 18.75h-2.25m-7.5-10.5H4.875c-.621 0-1.125.504-1.125 1.125v11.25c0 .621.504 1.125 1.125 1.125h9.75c.621 0 1.125-.504 1.125-1.125V18.75m-7.5-10.5h6.375c.621 0 1.125.504 1.125 1.125v9.375m-8.25-3 1.5 1.5 3-3.75" />`,
		'document-magnifying-glass': `<path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 0 0-3.375-3.375h-1.5A1.125 1.125 0 0 1 13.5 7.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H8.25m5.231 13.481L15 17.25m-4.5-15H5.625c-.621 0-1.125.504-1.125 1.125v16.5c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 0 0-9-9Zm3.75 11.625a2.625 2.625 0 1 1-5.25 0 2.625 2.625 0 0 1 5.25 0Z" />`,
		'cog-8-tooth': `<path stroke-linecap="round" stroke-linejoin="round" d="M10.343 3.94c.09-.542.56-.94 1.11-.94h1.093c.55 0 1.02.398 1.11.94l.149.894c.07.424.384.764.78.93.398.164.855.142 1.205-.108l.737-.527a1.125 1.125 0 0 1 1.45.12l.773.774c.39.389.44 1.002.12 1.45l-.527.737c-.25.35-.272.806-.107 1.204.165.397.505.71.93.78l.893.15c.543.09.94.559.94 1.109v1.094c0 .55-.397 1.02-.94 1.11l-.894.149c-.424.07-.764.383-.929.78-.165.398-.143.854.107 1.204l.527.738c.32.447.269 1.06-.12 1.45l-.774.773a1.125 1.125 0 0 1-1.449.12l-.738-.527c-.35-.25-.806-.272-1.203-.107-.398.165-.71.505-.781.929l-.149.894c-.09.542-.56.94-1.11.94h-1.094c-.55 0-1.019-.398-1.11-.94l-.148-.894c-.071-.424-.384-.764-.781-.93-.398-.164-.854-.142-1.204.108l-.738.527c-.447.32-1.06.269-1.45-.12l-.773-.774a1.125 1.125 0 0 1-.12-1.45l.527-.737c.25-.35.272-.806.108-1.204-.165-.397-.506-.71-.93-.78l-.894-.15c-.542-.09-.94-.56-.94-1.109v-1.094c0-.55.398-1.02.94-1.11l.894-.149c.424-.07.765-.383.93-.78.165-.398.143-.854-.108-1.204l-.526-.738a1.125 1.125 0 0 1 .12-1.45l.773-.773a1.125 1.125 0 0 1 1.45-.12l.737.527c.35.25.807.272 1.204.107.397-.165.71-.505.78-.929l.15-.894Z" /><path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z" />`,
		'building-office': `<path stroke-linecap="round" stroke-linejoin="round" d="M2.25 21h19.5m-18-18v18m10.5-18v18m6-13.5V21M6.75 6.75h.75m-.75 3h.75m-.75 3h.75m3-6h.75m-.75 3h.75m-.75 3h.75M6.75 21v-3.375c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125V21M3 3h12m-.75 4.5H21m-3.75 3.75h.008v.008h-.008v-.008Zm0 3h.008v.008h-.008v-.008Zm0 3h.008v.008h-.008v-.008Z" />`,
		'shield-check': `<path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75 11.25 15 15 9.75m-3-7.036A11.959 11.959 0 0 1 3.598 6 11.99 11.99 0 0 0 3 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285Z" />`,
		'computer-desktop': `<path stroke-linecap="round" stroke-linejoin="round" d="M9 17.25v1.007a3 3 0 0 1-.879 2.122L7.5 21h9l-.621-.621A3 3 0 0 1 15 18.257V17.25m6-12V15a2.25 2.25 0 0 1-2.25 2.25H5.25A2.25 2.25 0 0 1 3 15V5.25m18 0A2.25 2.25 0 0 0 18.75 3H5.25A2.25 2.25 0 0 0 3 5.25m18 0V12a2.25 2.25 0 0 1-2.25 2.25H5.25A2.25 2.25 0 0 1 3 12V5.25" />`,
		'arrows-right-left': `<path stroke-linecap="round" stroke-linejoin="round" d="M7.5 21 3 16.5m0 0L7.5 12M3 16.5h13.5m0-13.5L21 7.5m0 0L16.5 12M21 7.5H7.5" />`,
		'clipboard-document': `<path stroke-linecap="round" stroke-linejoin="round" d="M8.25 7.5V6.108c0-1.135.845-2.098 1.976-2.192.373-.03.748-.057 1.123-.08M15.75 18H18a2.25 2.25 0 0 0 2.25-2.25V6.108c0-1.135-.845-2.098-1.976-2.192a48.424 48.424 0 0 0-1.123-.08M15.75 18.75v-1.875a3.375 3.375 0 0 0-3.375-3.375h-1.5a1.125 1.125 0 0 1-1.125-1.125v-1.5A3.375 3.375 0 0 0 6.375 7.5H5.25m11.9-3.664A2.251 2.251 0 0 0 15 2.25h-1.5a2.251 2.251 0 0 0-2.15 1.586m5.8 0c.065.21.1.433.1.664v.75h-6V4.5c0-.231.035-.454.1-.664M6.75 7.5H4.875c-.621 0-1.125.504-1.125 1.125v12c0 .621.504 1.125 1.125 1.125h9.75c.621 0 1.125-.504 1.125-1.125V16.5a9 9 0 0 0-9-9Z" />`,
		'globe-alt': `<path stroke-linecap="round" stroke-linejoin="round" d="M12 21a9.004 9.004 0 0 0 8.716-6.747M12 21a9.004 9.004 0 0 1-8.716-6.747M12 21c2.485 0 4.5-4.03 4.5-9S14.485 3 12 3m0 18c-2.485 0-4.5-4.03-4.5-9S9.515 3 12 3m0 0a8.997 8.997 0 0 1 7.843 4.582M12 3a8.997 8.997 0 0 0-7.843 4.582m15.686 0A11.953 11.953 0 0 1 12 10.5c-2.998 0-5.74-1.1-7.843-2.918m15.686 0A8.959 8.959 0 0 1 21 12c0 .778-.099 1.533-.284 2.253m0 0A17.919 17.919 0 0 1 12 16.5c-3.162 0-6.133-.815-8.716-2.247m0 0A9.015 9.015 0 0 1 3 12c0-1.605.42-3.113 1.157-4.418" />`,
		megaphone: `<path stroke-linecap="round" stroke-linejoin="round" d="M19.114 5.636a9 9 0 0 1 0 12.728M16.463 8.288a5.25 5.25 0 0 1 0 7.424M6.75 8.25l4.72-4.72a.75.75 0 0 1 1.28.53v15.88a.75.75 0 0 1-1.28.53l-4.72-4.72H4.51c-.88 0-1.704-.507-1.938-1.354A9.009 9.009 0 0 1 2.25 12c0-.83.112-1.633.322-2.396C2.806 8.756 3.63 8.25 4.51 8.25H6.75Z" />`,
		'currency-dollar': `<path stroke-linecap="round" stroke-linejoin="round" d="M12 6v12m-3-2.818.879.659c1.171.879 3.07.879 4.242 0 1.172-.879 1.172-2.303 0-3.182C13.536 12.219 12.768 12 12 12c-.725 0-1.45-.22-2.003-.659-1.106-.879-1.106-2.303 0-3.182s2.9-.879 4.006 0l.415.33M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />`,
		'check-circle': `<path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />`,
		inbox: `<path stroke-linecap="round" stroke-linejoin="round" d="M2.25 13.5h3.86a2.25 2.25 0 0 1 2.012 1.244l.256.512a2.25 2.25 0 0 0 2.013 1.244h3.238a2.25 2.25 0 0 0 2.013-1.244l.256-.512a2.25 2.25 0 0 1 2.013-1.244h3.859m-19.5.338V18a2.25 2.25 0 0 0 2.25 2.25h15A2.25 2.25 0 0 0 21.75 18v-4.162c0-.224-.034-.447-.1-.661L19.24 5.338a2.25 2.25 0 0 0-2.15-1.588H6.911a2.25 2.25 0 0 0-2.15 1.588L2.35 13.177a2.25 2.25 0 0 0-.1.661Z" />`,
		bell: `<path stroke-linecap="round" stroke-linejoin="round" d="M14.857 17.082a23.848 23.848 0 0 0 5.454-1.31A8.967 8.967 0 0 1 18 9.75V9A6 6 0 0 0 6 9v.75a8.967 8.967 0 0 1-2.312 6.022c1.733.64 3.56 1.085 5.455 1.31m5.714 0a24.255 24.255 0 0 1-5.714 0m5.714 0a3 3 0 1 1-5.714 0" />`,
		banknotes: `<path stroke-linecap="round" stroke-linejoin="round" d="M2.25 18.75a60.07 60.07 0 0 1 15.797 2.101c.727.198 1.453-.342 1.453-1.096V18.75M3.75 4.5v.75A.75.75 0 0 1 3 6h-.75m0 0v-.375c0-.621.504-1.125 1.125-1.125H20.25M2.25 6v9m18-10.5v.75c0 .414.336.75.75.75h.75m-1.5-1.5h.375c.621 0 1.125.504 1.125 1.125v9.75c0 .621-.504 1.125-1.125 1.125h-.375m1.5-1.5H21a.75.75 0 0 0-.75.75v.75m0 0H3.75m0 0h-.375a1.125 1.125 0 0 1-1.125-1.125V15m1.5 1.5v-.75A.75.75 0 0 0 3 15h-.75M15 10.5a3 3 0 1 1-6 0 3 3 0 0 1 6 0Zm3 0h.008v.008H18V10.5Zm-12 0h.008v.008H6V10.5Z" />`,
		briefcase: `<path stroke-linecap="round" stroke-linejoin="round" d="M20.25 14.15v4.25c0 1.094-.787 2.036-1.872 2.18-2.087.277-4.216.42-6.378.42s-4.291-.143-6.378-.42c-1.085-.144-1.872-1.086-1.872-2.18v-4.25m16.5 0a2.18 2.18 0 0 0 .75-1.661V8.706c0-1.081-.768-2.015-1.837-2.175a48.114 48.114 0 0 0-3.413-.387m4.5 8.006c-.194.165-.42.295-.673.38A23.978 23.978 0 0 1 12 15.75c-2.648 0-5.195-.429-7.577-1.22a2.016 2.016 0 0 1-.673-.38m0 0A2.18 2.18 0 0 1 3 12.489V8.706c0-1.081.768-2.015 1.837-2.175a48.111 48.111 0 0 1 3.413-.387m7.5 0V5.25A2.25 2.25 0 0 0 13.5 3h-3a2.25 2.25 0 0 0-2.25 2.25v1.844m7.5 0a48.667 48.667 0 0 0-7.5 0" />`,
		'code-bracket': `<path stroke-linecap="round" stroke-linejoin="round" d="M17.25 6.75 22.5 12l-5.25 5.25m-10.5 0L1.5 12l5.25-5.25m7.5-3-4.5 16.5" />`,
		'book-open': `<path stroke-linecap="round" stroke-linejoin="round" d="M12 6.042A8.967 8.967 0 0 0 6 3.75c-1.052 0-2.062.18-3 .512v14.25A8.987 8.987 0 0 1 6 18c2.305 0 4.408.867 6 2.292m0-14.25a8.966 8.966 0 0 1 6-2.292c1.052 0 2.062.18 3 .512v14.25A8.987 8.987 0 0 0 18 18a8.967 8.967 0 0 0-6 2.292m0-14.25v14.25" />`
	};

	function iconFor(icon: string): string {
		return icons[icon] ?? icons.user;
	}

	interface NavigationItem {
		label: string;
		href: string;
		badge?: number;
	}

	interface NavigationSection {
		id: string;
		label: string;
		icon: string;
		href?: string;
		items?: NavigationItem[];
	}

	// All 32 Filament Navigation Groups + Live Dashboard matching sms.whiteinfobd.com exactly
	const navSections: NavigationSection[] = [
		{
			id: 'live_dashboard',
			label: 'Live Dashboard',
			href: '/superman',
			icon: 'bolt'
		},
		{
			id: 'user',
			label: 'User Management',
			icon: 'user',
			items: [
				{ label: 'Customers', href: '/superman/users/customers' },
				{ label: 'S P Resellers', href: '/superman/users/sps' }
			]
		},
		{
			id: 'kyc',
			label: 'KYC / Verification',
			icon: 'identification',
			items: [
				{ label: 'KYC Dashboard', href: '/superman/kyc/dashboard' },
				{ label: 'Verification Requests', href: '/superman/kyc/requests' },
				{ label: 'KYC Audit', href: '/superman/kyc/audit' },
				{ label: 'KYC Permissions', href: '/superman/kyc/permissions' }
			]
		},
		{
			id: 'billing',
			label: 'Billing',
			icon: 'credit-card',
			items: [
				{ label: 'Dashboard', href: '/superman/billing' },
				{ label: 'Tariff Plans', href: '/superman/billing/tariff-plans' },
				{ label: 'User-wise Assignment', href: '/superman/billing/assignments' },
				{ label: 'Rate History', href: '/superman/billing/rate-history' },
				{ label: 'Deposits', href: '/superman/billing/deposits' },
				{ label: 'Transactions', href: '/superman/billing/transactions' },
				{ label: 'Invoices', href: '/superman/billing/invoices' },
				{ label: 'Payments', href: '/superman/billing/payments' },
				{ label: 'Usage / CDR', href: '/superman/billing/usage-cdr' },
				{ label: 'Reports', href: '/superman/billing/reports' }
			]
		},
		{
			id: 'sender_cli',
			label: 'Sender / CLI Management',
			icon: 'phone',
			items: [
				{ label: 'Masking Sender', href: '/superman/sender-cli/masking' },
				{ label: 'Add Sender', href: '/superman/sender-cli/add' },
				{ label: 'Non-Masking Sender', href: '/superman/sender-cli/non-masking' },
				{ label: 'Sender Policy', href: '/superman/sender-cli/policy' },
				{ label: 'Sender Resolution', href: '/superman/sender-cli/resolution' },
				{ label: 'Sender Audit', href: '/superman/sender-cli/audit' }
			]
		},
		{
			id: 'providers',
			label: 'Provider Settings',
			icon: 'server',
			items: [
				{ label: 'Gateways / Providers', href: '/superman/gateways' },
				{ label: 'Provider Health', href: '/superman/providers/health' },
				{ label: 'Provider API Logs', href: '/superman/providers/api-logs' },
				{ label: 'DLR Mapping', href: '/superman/providers/dlr-mapping' },
				{ label: 'Error Mapping', href: '/superman/providers/error-mapping' }
			]
		},
		{
			id: 'reports',
			label: 'Reports',
			icon: 'document-text',
			items: [
				{ label: 'Executive Report', href: '/superman/reports/executive' },
				{ label: 'SMS Reports', href: '/superman/reports/sms' },
				{ label: 'Masking Report', href: '/superman/reports/masking' },
				{ label: 'Non-Masking Report', href: '/superman/reports/non-masking' },
				{ label: 'Reseller Reports', href: '/superman/reports/reseller' },
				{ label: 'Customer Reports', href: '/superman/reports/customer' },
				{ label: 'Provider Reports', href: '/superman/reports/provider' },
				{ label: 'Financial Reports', href: '/superman/reports/financial' }
			]
		},
		{
			id: 'analytics',
			label: 'Analytics',
			icon: 'chart-bar',
			items: [
				{ label: 'Cost & Profit', href: '/superman/analytics' },
				{ label: 'Messaging', href: '/superman/analytics/messaging' },
				{ label: 'Financial', href: '/superman/analytics/financial' }
			]
		},
		{
			id: 'messaging',
			label: 'Messaging',
			icon: 'chat-bubble',
			items: [
				{ label: 'Messages', href: '/superman/messages' },
				{ label: 'Batches', href: '/superman/batches' },
				{ label: 'Bulk SMS Templates', href: '/superman/templates' },
				{ label: 'Approvals', href: '/superman/messaging/approvals' }
			]
		},
		{
			id: 'audience',
			label: 'Audience',
			icon: 'users',
			items: [
				{ label: 'Contacts', href: '/superman/audience/contacts' },
				{ label: 'Contact Groups', href: '/superman/audience/groups' }
			]
		},
		{
			id: 'settings',
			label: 'Settings',
			icon: 'cog',
			items: [
				{ label: 'Company Info', href: '/superman/settings/general' },
				{ label: 'Countries', href: '/superman/countries' },
				{ label: 'Operators', href: '/superman/operators' },
				{ label: 'Email Gateways', href: '/superman/email-gateways' },
				{ label: 'Signup Register Gateways', href: '/superman/signup-gateways' }
			]
		},
		{
			id: 'resellers',
			label: 'S P',
			icon: 'building-office',
			items: [
				{ label: 'All Resellers', href: '/superman/users/sps' },
				{ label: 'Sub-Customers', href: '/superman/sps/customers' },
				{ label: 'Tariff & Rates', href: '/superman/sps/rates' },
				{ label: 'Reseller Wallets', href: '/superman/sps/wallet' },
				{ label: 'White-Label Branding', href: '/superman/sps/white-label' },
				{ label: 'Custom Domains', href: '/superman/sps/custom-domain' },
				{ label: 'S P Profit', href: '/superman/sps/profit' },
				{ label: 'Reseller Senders', href: '/superman/sps/senders' },
				{ label: 'S P Billing', href: '/superman/sps/billing' },
				{ label: 'Reseller KYC', href: '/superman/sps/kyc' },
				{ label: 'S P Statements', href: '/superman/sps/statements' },
				{ label: 'Reseller APIs', href: '/superman/sps/api' },
				{ label: 'Reseller Audit', href: '/superman/sps/audit' },
				{ label: 'Reseller Routing', href: '/superman/sps/routing' },
				{ label: 'Reseller Webhooks', href: '/superman/sps/webhooks' },
				{ label: 'S P Reports', href: '/superman/sps/reports' },
				{ label: 'S P Employees', href: '/superman/sps/employees' }
			]
		},
		{
			id: 'affiliates',
			label: 'Affiliates',
			icon: 'user-plus',
			items: [
				{ label: 'Affiliate Hub', href: '/superman/affiliates/dashboard' },
				{ label: 'Affiliate Partners', href: '/superman/affiliates/list' },
				{ label: 'Commission Rules', href: '/superman/affiliates/commission' },
				{ label: 'Withdrawals', href: '/superman/affiliates/withdrawals' },
				{ label: 'Affiliate Reports', href: '/superman/affiliates/reports' },
				{ label: 'Affiliate Audit', href: '/superman/affiliates/audit' }
			]
		},
		{
			id: 'sms_management',
			label: 'SMS Management',
			icon: 'chat-bubble-oval',
			items: [
				{ label: 'All SMS', href: '/superman/sms/all' },
				{ label: 'SMS Logs', href: '/superman/sms/logs' },
				{ label: 'Send SMS', href: '/superman/sms/send' },
				{ label: 'Failed SMS', href: '/superman/sms/failed' },
				{ label: 'Pending SMS', href: '/superman/sms/pending' },
				{ label: 'Scheduled SMS', href: '/superman/sms/scheduled' },
				{ label: 'Bulk SMS Batches', href: '/superman/sms/bulk' },
				{ label: 'Message Search', href: '/superman/sms/message-search' },
				{ label: 'Retry Queue', href: '/superman/sms/retry-queue' },
				{ label: 'SMS Inquiry', href: '/superman/sms/inquiry' }
			]
		},
		{
			id: 'routing',
			label: 'Routing',
			icon: 'arrows-right-left',
			items: [
				{ label: 'Routing Control', href: '/superman/routing/dashboard' },
				{ label: 'Routing Rules', href: '/superman/routing/rules' },
				{ label: 'Route Plans', href: '/superman/routing/routes' },
				{ label: 'Route Health', href: '/superman/routing/health' },
				{ label: 'Failover Audit', href: '/superman/routing/failover' },
				{ label: 'Route Simulator', href: '/superman/routing/testing' },
				{ label: 'Load Balancing', href: '/superman/routing/load-balancing' }
			]
		},
		{
			id: 'bangladesh_network',
			label: 'Bangladesh Network',
			icon: 'globe-alt',
			items: [
				{ label: 'MNO Management', href: '/superman/bd-network/mno' },
				{ label: 'Prefix Management', href: '/superman/bd-network/prefix' },
				{ label: 'Number Validation', href: '/superman/bd-network/number-validation' },
				{ label: 'MNP Lookup', href: '/superman/bd-network/mnp' },
				{ label: 'DND Registry', href: '/superman/bd-network/dnd' }
			]
		},
		{
			id: 'campaigns',
			label: 'Campaigns',
			icon: 'megaphone',
			items: [
				{ label: 'Campaigns List', href: '/superman/campaigns/list' },
				{ label: 'Create Campaign', href: '/superman/campaigns/create' },
				{ label: 'Campaign Reports', href: '/superman/campaigns/reports' }
			]
		},
		{
			id: 'profit_management',
			label: 'Profit Management',
			icon: 'currency-dollar',
			items: [
				{ label: 'Global Margin', href: '/superman/profit/global' },
				{ label: 'Reseller Profit', href: '/superman/profit/reseller' },
				{ label: 'Customer Profit', href: '/superman/profit/customer' },
				{ label: 'Trend Analysis', href: '/superman/profit/trend' },
				{ label: 'Alert Settings', href: '/superman/profit/alerts' }
			]
		},
		{
			id: 'dlr',
			label: 'DLR / Delivery',
			icon: 'check-circle',
			items: [
				{ label: 'Live DLR Stream', href: '/superman/dlr/live' },
				{ label: 'Delivery Logs', href: '/superman/dlr/logs' },
				{ label: 'DLR Queue', href: '/superman/dlr/queue' },
				{ label: 'DLR Retry', href: '/superman/dlr/retry' },
				{ label: 'Delivery Check', href: '/superman/dlr/check' }
			]
		},
		{
			id: 'mo_inbound',
			label: 'MO / Inbound',
			icon: 'inbox',
			items: [
				{ label: 'Live Inbound Feed', href: '/superman/mo/messages' },
				{ label: 'Inbound Logs', href: '/superman/mo/logs' },
				{ label: 'MO Routing', href: '/superman/mo/routing' },
				{ label: 'Inbound Analytics', href: '/superman/mo/reports' }
			]
		},
		{
			id: 'webhooks',
			label: 'Webhooks',
			icon: 'bolt',
			items: [
				{ label: 'Webhook Inbound Logs', href: '/superman/webhooks/logs' },
				{ label: 'Failed Webhooks', href: '/superman/webhooks/failed' },
				{ label: 'Webhook Settings', href: '/superman/webhooks/configuration' },
				{ label: 'Customer Webhooks', href: '/superman/webhooks/customer' },
				{ label: 'Reseller Webhooks', href: '/superman/webhooks/reseller' }
			]
		},
		{
			id: 'monitoring',
			label: 'Monitoring / NOC',
			icon: 'computer-desktop',
			items: [
				{ label: 'System Health', href: '/superman/monitoring/system-health' },
				{ label: 'Live Monitoring', href: '/superman/monitoring/live' },
				{ label: 'Provider Health', href: '/superman/monitoring/providers' },
				{ label: 'Queue Health', href: '/superman/monitoring/queue-health' },
				{ label: 'API Health', href: '/superman/monitoring/api-health' },
				{ label: 'TPS Monitor', href: '/superman/monitoring/tps' }
			]
		},
		{
			id: 'alerts',
			label: 'Alerts & Notifications',
			icon: 'bell',
			items: [
				{ label: 'Alerts Overview', href: '/superman/alerts/dashboard' },
				{ label: 'Alert Settings', href: '/superman/alerts/configuration' },
				{ label: 'Provider Outages', href: '/superman/alerts/provider-down' }
			]
		},
		{
			id: 'payment_gateway',
			label: 'Payment Gateway',
			icon: 'banknotes',
			items: [
				{ label: 'Global Payment Gateways', href: '/superman/payment-gateways/gateways' },
				{ label: 'Payment Transactions', href: '/superman/payment-gateways/transactions' },
				{ label: 'Payment Reports', href: '/superman/payment-gateways/reports' },
				{ label: 'Gateway Reconciliation', href: '/superman/payment-gateways/reconciliation' }
			]
		},
		{
			id: 'employees',
			label: 'Employees',
			icon: 'briefcase',
			items: [
				{ label: 'Staff & Employees', href: '/superman/employees/list' },
				{ label: 'Staff Sessions', href: '/superman/employees/sessions' }
			]
		},
		{
			id: 'roles_permissions',
			label: 'Roles & Permissions',
			icon: 'shield-check',
			items: [
				{ label: 'Roles', href: '/superman/roles-permissions/roles' },
				{ label: 'Permissions', href: '/superman/roles-permissions/permissions' }
			]
		},
		{
			id: 'approval_center',
			label: 'Approval Center',
			icon: 'clipboard-check',
			items: [
				{ label: 'Overview', href: '/superman/approvals/overview' },
				{ label: 'Pending Customers', href: '/superman/approvals/customers' },
				{ label: 'Pending Resellers', href: '/superman/approvals/resellers' },
				{ label: 'Pending Senders', href: '/superman/approvals/senders' },
				{ label: 'IP Whitelist Requests', href: '/superman/approval-center/ip-whitelists' }
			]
		},
		{
			id: 'audit_logs',
			label: 'Audit & Activity Logs',
			icon: 'clipboard-document',
			items: [
				{ label: 'Global Audit Log', href: '/superman/audit-logs/global' },
				{ label: 'IP Whitelist Audit', href: '/superman/audit-logs/ip-whitelist' },
				{ label: 'Employee Activity', href: '/superman/audit-logs/employees' },
				{ label: 'Security Activity', href: '/superman/audit-logs/security' }
			]
		},
		{
			id: 'api_management',
			label: 'API Management',
			icon: 'code-bracket',
			items: [
				{ label: 'API Accounts', href: '/superman/api-management/accounts' },
				{ label: 'API Tokens', href: '/superman/api-management/keys' },
				{ label: 'API Traffic Logs', href: '/superman/api-management/logs' },
				{ label: 'API Security Policy', href: '/superman/api-management/security' }
			]
		},
		{
			id: 'security',
			label: 'Security & Fraud Control',
			icon: 'shield-check',
			items: [
				{ label: 'Security Dashboard', href: '/superman/security/dashboard' },
				{ label: 'IP Whitelist Policy', href: '/superman/security/ip-whitelist' },
				{ label: 'Fraud Rules', href: '/superman/security/fraud-rules' },
				{ label: 'Security Logs', href: '/superman/security/logs' }
			]
		},
		{
			id: 'system_logs',
			label: 'System Logs',
			icon: 'document-magnifying-glass',
			items: [
				{ label: 'Logs Center', href: '/superman/system-logs/overview' },
				{ label: 'SMS Logs', href: '/superman/system-logs/sms-logs' },
				{ label: 'Error Logs', href: '/superman/system-logs/errors' },
				{ label: 'API Logs', href: '/superman/system-logs/api' },
				{ label: 'Email Logs', href: '/superman/system-logs/email' }
			]
		},
		{
			id: 'system_settings',
			label: 'System Settings',
			icon: 'cog-8-tooth',
			items: [
				{ label: 'General Settings', href: '/superman/system-settings' },
				{ label: 'SMS Settings', href: '/superman/system-settings/sms' },
				{ label: 'Provider Overview', href: '/superman/system-settings/providers' },
				{ label: 'Routing Overview', href: '/superman/system-settings/routing' },
				{ label: 'Queue Settings', href: '/superman/system-settings/queue' },
				{ label: 'Error Codes', href: '/superman/system-settings/error-codes' },
				{ label: 'DLR Settings', href: '/superman/system-settings/dlr' },
				{ label: 'Maintenance Mode', href: '/superman/system-settings/maintenance' },
				{ label: 'Database Backup', href: '/superman/system-settings/backups' }
			]
		},
		{
			id: 'api_documentation',
			label: 'API Documentation',
			href: '/superman/api-docs',
			icon: 'book-open'
		}
	];

	function isPathActive(href: string): boolean {
		if (href === '/superman') {
			return currentPath === '/superman';
		}
		return currentPath.startsWith(href);
	}

	function isGroupActive(section: NavigationSection): boolean {
		if (section.id === 'sender_cli' && (currentPath.startsWith('/superman/senders') || currentPath.startsWith('/superman/sender-cli'))) {
			return true;
		}
		return (section.items ?? []).some((item) => isPathActive(item.href));
	}
</script>

<aside
	class="flex flex-col border-r border-gray-200/80 bg-white transition-all duration-300 dark:border-gray-800 dark:bg-gray-900 shrink-0 select-none h-full {collapsed ? 'w-16' : 'w-[280px] lg:w-[300px]'}"
>
	<!-- Logo at the very top of the sidebar -->
	<div class="flex h-16 shrink-0 items-center border-b border-gray-200/80 px-3 dark:border-gray-800 {collapsed ? 'justify-center' : 'justify-between'}">
		{#if !collapsed}
			<a href="/superman" class="flex items-center overflow-hidden py-1">
				<img
					src="/logo.png"
					alt="WHITE INFO"
					class="h-9 max-h-9 w-auto max-w-[200px] object-contain"
					onerror={(e) => {
						(e.currentTarget as HTMLImageElement).style.display = 'none';
						const fb = document.getElementById('admin-sidebar-logo-fallback');
						if (fb) fb.style.display = 'flex';
					}}
				/>
				<div id="admin-sidebar-logo-fallback" class="hidden items-center gap-1">
					<span class="text-sm font-extrabold text-emerald-600 tracking-tight">WHITE INFO</span>
				</div>
			</a>
		{:else}
			<a href="/superman" class="flex items-center justify-center py-1">
				<img
					src="/icon.png"
					alt="WHITE INFO"
					class="h-8 w-8 object-contain rounded"
					onerror={(e) => {
						(e.currentTarget as HTMLImageElement).style.display = 'none';
						const fb = document.getElementById('admin-sidebar-icon-fallback');
						if (fb) fb.style.display = 'flex';
					}}
				/>
				<div id="admin-sidebar-icon-fallback" class="hidden h-7 w-7 items-center justify-center rounded-lg bg-emerald-600 text-white">
					<span class="text-xs font-bold">W</span>
				</div>
			</a>
		{/if}
	</div>

	<!-- Navigation list -->
	<div class="flex-1 overflow-y-auto px-3 py-3 space-y-1">
		{#each navSections as section}
			{#if section.href}
				<!-- Direct link (like Live Dashboard or API Docs) -->
				<a
					href={section.href}
					class="group flex items-center gap-3 rounded-lg px-3 py-2 text-xs font-medium transition-colors {isPathActive(section.href)
						? 'bg-emerald-50 text-emerald-800 dark:bg-emerald-950/50 dark:text-emerald-400 font-semibold'
						: 'text-gray-700 hover:bg-gray-100 hover:text-gray-900 dark:text-gray-300 dark:hover:bg-gray-800 dark:hover:text-white'}"
				>
					<svg class="h-4 w-4 shrink-0 {isPathActive(section.href) ? 'text-emerald-600 dark:text-emerald-400' : 'text-gray-400 group-hover:text-gray-600 dark:text-gray-500'}" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor">
						{@html iconFor(section.icon)}
					</svg>
					{#if !collapsed}
						<span class="truncate">{section.label}</span>
					{/if}
				</a>
			{:else}
				<!-- Collapsible Group -->
				<div>
					<button
						type="button"
						onclick={() => toggleGroup(section.id)}
						class="flex w-full items-center justify-between rounded-lg px-3 py-2 text-xs font-medium text-gray-600 hover:bg-gray-50 hover:text-gray-900 dark:text-gray-400 dark:hover:bg-gray-800/60 dark:hover:text-gray-200 transition-colors"
					>
						<div class="flex items-center gap-3 truncate">
							<!-- Group Icon -->
							<svg class="h-4 w-4 shrink-0 {isGroupActive(section) ? 'text-emerald-600 dark:text-emerald-400' : 'text-gray-400 dark:text-gray-500'}" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor">
								{@html iconFor(section.icon)}
							</svg>
							{#if !collapsed}
								<span class="truncate {isGroupActive(section) ? 'font-semibold text-gray-900 dark:text-white' : ''}">{section.label}</span>
							{/if}
						</div>
						{#if !collapsed}
							<svg
								class="h-3.5 w-3.5 text-gray-400 transition-transform duration-200 {openGroups[section.id] ? 'rotate-180' : ''}"
								fill="none"
								viewBox="0 0 24 24"
								stroke-width="2"
								stroke="currentColor"
							>
								<path stroke-linecap="round" stroke-linejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" />
							</svg>
						{/if}
					</button>

					<!-- Sub-items -->
					{#if !collapsed && openGroups[section.id] && section.items}
						<div class="ml-4 pl-3 border-l border-gray-100 dark:border-gray-800 mt-1 space-y-0.5">
							{#each section.items as item}
								<a
									href={item.href}
									class="flex items-center justify-between rounded-md px-2.5 py-1.5 text-xs transition-colors {isPathActive(item.href)
										? 'bg-emerald-50 text-emerald-800 font-semibold dark:bg-emerald-950/40 dark:text-emerald-400'
										: 'text-gray-600 hover:bg-gray-50 hover:text-gray-900 dark:text-gray-400 dark:hover:bg-gray-800/50 dark:hover:text-white'}"
								>
									<span class="truncate">{item.label}</span>
									{#if item.badge}
										<span class="rounded-full bg-emerald-100 px-1.5 py-0.2 text-[10px] font-bold text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300">
											{item.badge}
										</span>
									{/if}
								</a>
							{/each}
						</div>
					{/if}
				</div>
			{/if}
		{/each}
	</div>
</aside>
