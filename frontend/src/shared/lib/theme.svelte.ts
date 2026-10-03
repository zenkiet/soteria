export type Theme = 'system' | 'light' | 'dark';

function stored(): Theme {
	try {
		return (localStorage.getItem('theme') as Theme | null) ?? 'system';
	} catch {
		return 'system';
	}
}

export const theme = $state({ value: stored() });

// 'system' drops the attribute and the stylesheet's color-scheme follows the OS on its own.
export function setTheme(t: Theme) {
	theme.value = t;
	if (t === 'system') delete document.documentElement.dataset.theme;
	else document.documentElement.dataset.theme = t;
	try {
		localStorage.setItem('theme', t);
	} catch {
		// storage unavailable in this webview; the choice lasts for the session
	}
}
