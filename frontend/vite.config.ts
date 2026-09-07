import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import wails from '@wailsio/runtime/plugins/vite';
import { readFileSync } from 'node:fs';
import { defineConfig } from 'vite';

const { version } = JSON.parse(readFileSync('package.json', 'utf8'));

export default defineConfig({
	define: { __APP_VERSION__: JSON.stringify(version) },
	server: {
		host: '127.0.0.1',
		port: Number(process.env.WAILS_VITE_PORT) || 9245,
		strictPort: true
	},
	plugins: [tailwindcss(), sveltekit(), wails('./bindings')]
});
