import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

const config = {
	preprocess: vitePreprocess(),
	compilerOptions: {
		runes: ({ filename }) => (filename.split(/[/\\]/).includes('node_modules') ? undefined : true)
	},
	kit: {
		adapter: adapter({ pages: 'dist', assets: 'dist', fallback: 'index.html' }),
		alias: { '@': 'src', '@bindings': 'bindings' },
		paths: { relative: false }
	}
};

export default config;
