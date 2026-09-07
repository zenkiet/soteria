type Toast = {
	id: number;
	text: string;
	kind: 'ok' | 'error';
	action?: { label: string; run: () => void };
};

export const toasts = $state<{ list: Toast[] }>({ list: [] });

export const dismiss = (id: number) => (toasts.list = toasts.list.filter((t) => t.id !== id));

export function toast(text: string, kind: Toast['kind'] = 'ok', action?: Toast['action']) {
	const id = Date.now() + Math.random();
	toasts.list.push({ id, text, kind, action });
	setTimeout(() => dismiss(id), action ? 6000 : 4000);
}
