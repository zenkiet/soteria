import prettier from 'eslint-config-prettier';
import path from 'node:path';
import js from '@eslint/js';
import boundaries from 'eslint-plugin-boundaries';
import svelte from 'eslint-plugin-svelte';
import { defineConfig, includeIgnoreFile } from 'eslint/config';
import globals from 'globals';
import ts from 'typescript-eslint';

const gitignorePath = path.resolve(import.meta.dirname, '.gitignore');

// Outermost first; imports flow only downward.
const LAYERS = ['app', 'pages', 'widgets', 'features', 'entities', 'shared'];
const SLICED = ['pages', 'widgets', 'features', 'entities'];

const PUBLIC_ENTRY = {
	pages: '*-page.svelte',
	widgets: 'index.ts',
	features: 'index.ts',
	entities: 'index.ts'
};

const below = (layer) => LAYERS.slice(LAYERS.indexOf(layer) + 1);

export default defineConfig(
	includeIgnoreFile(gitignorePath),
	{ ignores: ['dist', 'bindings'] },
	js.configs.recommended,
	ts.configs.recommended,
	svelte.configs.recommended,
	prettier,
	svelte.configs.prettier,
	{
		languageOptions: { globals: { ...globals.browser, ...globals.node } },
		rules: {
			'no-undef': 'off'
		}
	},
	{
		files: ['**/*.svelte', '**/*.svelte.ts', '**/*.svelte.js'],
		languageOptions: {
			parserOptions: {
				projectService: true,
				extraFileExtensions: ['.svelte'],
				parser: ts.parser
			}
		}
	},
	{
		// Override or add rule settings here, such as:
		// 'svelte/button-has-type': 'error'
		rules: { 'svelte/no-navigation-without-resolve': 'off' }
	},
	{
		files: ['src/**/*.{js,ts,svelte}'],
		plugins: { boundaries },
		settings: {
			'boundaries/dependency-nodes': ['import', 'dynamic-import'],
			'boundaries/elements': [
				...SLICED.map((layer) => ({
					type: layer,
					pattern: `src/${layer}/*`,
					capture: ['slice']
				})),
				{ type: 'shared', pattern: 'src/shared/*', capture: ['segment'] },
				{ type: 'app', pattern: 'src/app/**', partialMatch: false }
			]
		},
		rules: {
			'boundaries/no-unknown': 'off',
			'boundaries/no-unknown-files': 'off',
			'boundaries/dependencies': [
				'error',
				{
					default: 'disallow',
					policies: [
						...LAYERS.flatMap((layer) => {
							const targets = below(layer);
							const sliced = targets.filter((target) => SLICED.includes(target));
							const flat = targets.filter((target) => !SLICED.includes(target));

							return [
								...sliced.map((target) => ({
									from: { element: { type: layer } },
									allow: {
										to: {
											element: {
												type: target,
												fileInternalPath: PUBLIC_ENTRY[target]
											}
										}
									}
								})),
								...(flat.length
									? [
											{
												from: { element: { type: layer } },
												allow: { to: { element: { types: { anyOf: flat } } } }
											}
										]
									: [])
							];
						}),
						...SLICED.map((layer) => ({
							from: { element: { type: layer, captured: { slice: '{{slice}}' } } },
							allow: {
								to: {
									element: {
										type: layer,
										captured: { slice: '{{from.captured.slice}}' }
									}
								}
							}
						})),
						{
							from: { element: { type: 'shared', captured: { segment: '{{segment}}' } } },
							allow: {
								to: {
									element: {
										type: 'shared',
										captured: { segment: '{{from.captured.segment}}' }
									}
								}
							}
						},
						{ from: { element: { type: 'app' } }, allow: { to: { element: { type: 'app' } } } }
					]
				}
			]
		}
	}
);
