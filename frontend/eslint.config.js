import js from '@eslint/js';
import svelte from 'eslint-plugin-svelte';
import { defineConfig, includeIgnoreFile } from 'eslint/config';
import ts from 'typescript-eslint';

// Imports flow one way, app → pages → shared; page slices and shared segments stay independent.
const layer = (dir, banned) => ({
	files: [`src/${dir}/**`],
	rules: { 'no-restricted-imports': ['error', { patterns: [{ regex: `^#/(${banned})/` }] }] }
});

export default defineConfig(
	includeIgnoreFile(import.meta.dirname + '/.gitignore'),
	{ ignores: ['dist', 'bindings'] },
	js.configs.recommended,
	ts.configs.recommended,
	svelte.configs.recommended,
	{
		files: ['**/*.svelte', '**/*.svelte.ts', '**/*.svelte.js'],
		languageOptions: { parserOptions: { parser: ts.parser } }
	},
	{ rules: { 'no-undef': 'off', 'svelte/no-navigation-without-resolve': 'off' } },
	layer('pages', 'app|pages'),
	layer('shared/api', 'app|pages|shared/(lib|ui)'),
	layer('shared/lib', 'app|pages|shared/(api|ui)'),
	layer('shared/ui', 'app|pages|shared/(api|lib)')
);
