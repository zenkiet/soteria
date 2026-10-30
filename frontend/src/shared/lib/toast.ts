import { toast as sonner } from 'svelte-sonner';

export function toast(
	text: string,
	kind: 'ok' | 'error' = 'ok',
	action?: { label: string; run: () => void }
) {
	sonner[kind === 'ok' ? 'success' : 'error'](
		text,
		action && { action: { label: action.label, onClick: action.run }, duration: 6000 }
	);
}
