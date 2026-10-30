import { Events } from '@wailsio/runtime';
import type { Transfer } from '#bindings/soteria/internal/domain/models.ts';
import { Transfers } from '#bindings/soteria/internal/infra/wails/app.ts';

// The backend batches changes; each batch lands as one fresh array.
export const transfers = new (class {
	list = $state.raw<Transfer[]>([]);
})();

export const live = (t: Transfer) => t.status === 'running' || t.status === 'queued';

Transfers().then((l) => (transfers.list = l ?? []));

Events.On('transfers', (e) => {
	const batch = new Map((e.data ?? []).map((t) => [t.id, t]));
	const known = new Set(transfers.list.map((t) => t.id));
	transfers.list = [
		...transfers.list.map((t) => batch.get(t.id) ?? t),
		...[...batch.values()].filter((t) => !known.has(t.id))
	];
});
