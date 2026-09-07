export type Theme = 'system' | 'light' | 'dark';

const mq = matchMedia('(prefers-color-scheme: dark)');

function stored(): Theme {
	try {
		return (localStorage.getItem('theme') as Theme | null) ?? 'system';
	} catch {
		return 'system';
	}
}

export const theme = $state({ value: stored() });

function apply() {
	document.documentElement.dataset.theme =
		theme.value === 'system' ? (mq.matches ? 'dark' : 'light') : theme.value;
}

export function setTheme(t: Theme) {
	theme.value = t;
	apply();
	try {
		localStorage.setItem('theme', t);
	} catch {
		// storage unavailable in this webview; the choice lasts for the session
	}
}

export function initTheme() {
	mq.addEventListener('change', apply);
	apply();
}
