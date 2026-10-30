<script lang="ts" module>
	import file from '~icons/vscode-icons/default-file';
	import ai from '~icons/vscode-icons/file-type-ai';
	import audio from '~icons/vscode-icons/file-type-audio';
	import css from '~icons/vscode-icons/file-type-css';
	import csv from '~icons/vscode-icons/file-type-csv';
	import epub from '~icons/vscode-icons/file-type-epub';
	import excel from '~icons/vscode-icons/file-type-excel';
	import font from '~icons/vscode-icons/file-type-font';
	import go from '~icons/vscode-icons/file-type-go';
	import html from '~icons/vscode-icons/file-type-html';
	import image from '~icons/vscode-icons/file-type-image';
	import ini from '~icons/vscode-icons/file-type-ini';
	import js from '~icons/vscode-icons/file-type-js-official';
	import json from '~icons/vscode-icons/file-type-json';
	import log from '~icons/vscode-icons/file-type-log';
	import markdown from '~icons/vscode-icons/file-type-markdown';
	import pdf from '~icons/vscode-icons/file-type-pdf2';
	import photoshop from '~icons/vscode-icons/file-type-photoshop';
	import powerpoint from '~icons/vscode-icons/file-type-powerpoint';
	import python from '~icons/vscode-icons/file-type-python';
	import shell from '~icons/vscode-icons/file-type-shell';
	import sql from '~icons/vscode-icons/file-type-sql';
	import svg from '~icons/vscode-icons/file-type-svg';
	import text from '~icons/vscode-icons/file-type-text';
	import toml from '~icons/vscode-icons/file-type-toml';
	import ts from '~icons/vscode-icons/file-type-typescript-official';
	import video from '~icons/vscode-icons/file-type-video';
	import word from '~icons/vscode-icons/file-type-word';
	import xml from '~icons/vscode-icons/file-type-xml';
	import yaml from '~icons/vscode-icons/file-type-yaml';
	import zip from '~icons/vscode-icons/file-type-zip';

	const types = {
		pdf,
		'doc docx rtf': word,
		'xls xlsx': excel,
		csv,
		'ppt pptx': powerpoint,
		psd: photoshop,
		ai,
		'zip gz tgz tar rar 7z': zip,
		'mp4 mov m4v mkv webm avi': video,
		'mp3 m4a wav aac flac ogg': audio,
		'jpg jpeg png gif webp heic bmp avif tif tiff': image,
		svg,
		txt: text,
		md: markdown,
		json,
		'yaml yml': yaml,
		toml,
		'ini conf': ini,
		'js mjs': js,
		ts,
		go,
		py: python,
		'html htm': html,
		css,
		'sh zsh bash': shell,
		sql,
		xml,
		log,
		'ttf otf woff woff2': font,
		epub
	};
	const byExt: Record<string, typeof file> = Object.fromEntries(
		Object.entries(types).flatMap(([exts, icon]) => exts.split(' ').map((x) => [x, icon]))
	);
</script>

<script lang="ts">
	import Icon from './icon.svelte';

	let { name, dir = false, size = 16 }: { name: string; dir?: boolean; size?: number } = $props();
	const Type = $derived(byExt[name.slice(name.lastIndexOf('.') + 1).toLowerCase()] ?? file);
</script>

{#if dir}
	<Icon name="folderFill" {size} />
{:else}
	<Type width={size} height={size} class="shrink-0" />
{/if}
