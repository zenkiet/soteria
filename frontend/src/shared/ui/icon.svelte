<script lang="ts" module>
	const folder =
		'M3 7.5A1.5 1.5 0 0 1 4.5 6h4.3l1.8 2h8.9A1.5 1.5 0 0 1 21 9.5v8a1.5 1.5 0 0 1-1.5 1.5h-15A1.5 1.5 0 0 1 3 17.5z';
	const file =
		'<path d="M7 3h7l5 5v12a1 1 0 0 1-1 1H7a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1z"/><path d="M14 3v5h5"/>';

	const paths = {
		folder: `<path d="${folder}"/>`,
		folderFill: `<path d="${folder}" fill="var(--folder-a)" stroke="var(--folder-line)" stroke-width="1.3"/><path d="M3 11h18v6.5a1.5 1.5 0 0 1-1.5 1.5h-15A1.5 1.5 0 0 1 3 17.5z" fill="var(--folder-b)" stroke="var(--folder-line)" stroke-width="1.3"/>`,
		folderPlus: `<path d="${folder}"/><path d="M12 11v5M9.5 13.5h5"/>`,
		folderMove: `<path d="${folder}"/><path d="M9 13.5h6M13 11.5l2 2-2 2"/>`,
		file,
		doc: `${file}<path d="M9 13h6M9 16.5h6"/>`,
		image:
			'<rect x="4" y="5.5" width="16" height="13" rx="1.5"/><path d="M4 16l4.5-4.5 3.5 3.5 3-3 5 5"/><circle cx="15.5" cy="9.5" r="1.2" fill="currentColor" stroke="none"/>',
		video:
			'<rect x="3.5" y="6.5" width="12" height="11" rx="1.5"/><path d="M15.5 10.5l5-2.5v8l-5-2.5"/>',
		sheet:
			'<rect x="4" y="5" width="16" height="14" rx="1.5"/><path d="M4 10h16M4 14.5h16M10 5v14"/>',
		archive:
			'<rect x="4" y="5" width="16" height="4" rx="1"/><path d="M5 9v9a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1V9M10 12.5h4"/>',
		search: '<circle cx="11" cy="11" r="6.5"/><path d="M16 16l4.5 4.5"/>',
		upload: '<path d="M12 16V5M7 10l5-5 5 5M4 19.5h16"/>',
		download: '<path d="M12 4v11M7 10l5 5 5-5M4 19.5h16"/>',
		plus: '<path d="M12 5v14M5 12h14"/>',
		grid: '<rect x="4" y="4" width="6.5" height="6.5" rx="1"/><rect x="13.5" y="4" width="6.5" height="6.5" rx="1"/><rect x="4" y="13.5" width="6.5" height="6.5" rx="1"/><rect x="13.5" y="13.5" width="6.5" height="6.5" rx="1"/>',
		list: '<path d="M9 6h11M9 12h11M9 18h11"/><circle cx="4.5" cy="6" r="1" fill="currentColor" stroke="none"/><circle cx="4.5" cy="12" r="1" fill="currentColor" stroke="none"/><circle cx="4.5" cy="18" r="1" fill="currentColor" stroke="none"/>',
		chevronRight: '<path d="M9 6l6 6-6 6"/>',
		chevronDown: '<path d="M6 9l6 6 6-6"/>',
		chevronLeft: '<path d="M15 6l-6 6 6 6"/>',
		arrowLeft: '<path d="M19 12H5M11 6l-6 6 6 6"/>',
		arrowRight: '<path d="M5 12h14M13 6l6 6-6 6"/>',
		dots: '<circle cx="5" cy="12" r="1.3" fill="currentColor" stroke="none"/><circle cx="12" cy="12" r="1.3" fill="currentColor" stroke="none"/><circle cx="19" cy="12" r="1.3" fill="currentColor" stroke="none"/>',
		server:
			'<rect x="3.5" y="4.5" width="17" height="6" rx="1.5"/><rect x="3.5" y="13.5" width="17" height="6" rx="1.5"/><circle cx="7" cy="7.5" r="0.9" fill="currentColor" stroke="none"/><circle cx="7" cy="16.5" r="0.9" fill="currentColor" stroke="none"/>',
		signOut:
			'<path d="M9.5 4H5.5a1.5 1.5 0 0 0-1.5 1.5v13A1.5 1.5 0 0 0 5.5 20h4M15.5 8l4.5 4-4.5 4M20 12H9.5"/>',
		check: '<path d="M5 12.5l4.5 4.5L19 7"/>',
		x: '<path d="M6 6l12 12M18 6L6 18"/>',
		trash: '<path d="M4 7h16M9.5 7V4.5h5V7M6.5 7l.9 12h9.2l.9-12M10 11v5M14 11v5"/>',
		pencil: '<path d="M4 20l4.5-1L19 8.5 15.5 5 5 15.5 4 20z"/><path d="M13.5 7l3.5 3.5"/>',
		transfers: '<path d="M8 20V5M4 9l4-4 4 4M16 4v15M12 15l4 4 4-4"/>',
		link: '<path d="M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1.5 1.5"/><path d="M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1.5-1.5"/>',
		info: '<circle cx="12" cy="12" r="8.5"/><path d="M12 11v5M12 8h.01"/>',
		eye: '<path d="M2.5 12S6 5.5 12 5.5 21.5 12 21.5 12 18 18.5 12 18.5 2.5 12 2.5 12z"/><circle cx="12" cy="12" r="3"/>',
		open: '<path d="M14 4h6v6M20 4l-8 8"/><path d="M18 14v5a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1h5"/>',
		refresh: '<path d="M20 12a8 8 0 1 1-2.3-5.7"/><path d="M20 4v5h-5"/>',
		sidebar:
			'<rect x="3" y="4.5" width="18" height="15" rx="3"/><path d="M9.5 4.5v15"/><rect x="3" y="4.5" width="6.5" height="15" rx="3" fill="currentColor" stroke="none" opacity=".35"/>',
		sidebarOff: '<rect x="3" y="4.5" width="18" height="15" rx="3"/><path d="M9.5 4.5v15"/>',
		gear: '<path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/><circle cx="12" cy="12" r="3"/>',
		copy: '<rect x="9" y="9" width="11" height="11" rx="1.5"/><path d="M15 9V5.5A1.5 1.5 0 0 0 13.5 4h-8A1.5 1.5 0 0 0 4 5.5v8A1.5 1.5 0 0 0 5.5 15H9"/>',
		cut: '<circle cx="6.5" cy="6.5" r="2.5"/><circle cx="6.5" cy="17.5" r="2.5"/><path d="M8.5 8.3 20 20M8.5 15.7 20 4"/>',
		clock: '<circle cx="12" cy="12" r="8.5"/><path d="M12 7.5V12l3.5 2"/>'
	};

	export type IconName = keyof typeof paths;
</script>

<script lang="ts">
	let {
		name,
		size = 16,
		class: cls = ''
	}: { name: IconName; size?: number; class?: string } = $props();
</script>

<svg
	class="shrink-0 {cls}"
	width={size}
	height={size}
	viewBox="0 0 24 24"
	fill="none"
	stroke="currentColor"
	stroke-width="1.6"
	stroke-linecap="round"
	stroke-linejoin="round"
	aria-hidden="true"
>
	<!-- eslint-disable-next-line svelte/no-at-html-tags -->
	{@html paths[name]}
</svg>
