type View = 'card' | 'list';
export type SortKey = 'name' | 'kind' | 'size' | 'modified' | 'created';
type Prefs = {
	view: View;
	sort: SortKey;
	asc: boolean;
	hidden: boolean;
	lastDir: Record<string, string>;
	downloadDir: string;
	askDownloadDir: boolean;
	sidebarHidden: boolean;
	drive: Record<string, boolean>;
	intro: Record<string, boolean>;
	background: boolean;
	notify: boolean;
	autoUpdate: boolean;
	skipVersion: string;
};

const DEFAULTS: Prefs = {
	view: 'card',
	sort: 'name',
	asc: true,
	hidden: false,
	lastDir: {},
	downloadDir: '',
	askDownloadDir: false,
	sidebarHidden: false,
	drive: {},
	intro: {},
	background: true,
	notify: true,
	autoUpdate: true,
	skipVersion: ''
};

function stored(): Prefs {
	try {
		return { ...DEFAULTS, ...JSON.parse(localStorage.getItem('prefs') ?? '{}') };
	} catch {
		return DEFAULTS;
	}
}

export const prefs = $state<Prefs>(stored());

export function setPrefs(p: Partial<Prefs>) {
	Object.assign(prefs, p);
	try {
		localStorage.setItem('prefs', JSON.stringify($state.snapshot(prefs)));
	} catch {
		// storage unavailable in this webview; the choice lasts for the session
	}
}
