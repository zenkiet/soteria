import { Events } from '@wailsio/runtime';
import type { Transfer } from '#bindings/soteria/internal/domain/models.ts';
import { Transfers } from '#bindings/soteria/internal/infra/wails/app.ts';

// A folder upload sends one event per file; they are applied once per frame onto a fresh array.
export const transfers = new (class {
	list = $state.raw<Transfer[]>([]);
})();

export const live = (t: Transfer) => t.status === 'running' || t.status === 'queued';

Transfers().then((l) => (transfers.list = l ?? []));

let pending: Record<string, Transfer> = {};
let frame = 0;

function flush() {
	frame = 0;
	const next = transfers.list.slice();
	const at = new Map(next.map((t, i) => [t.id, i]));
	for (const t of Object.values(pending)) {
		const i = at.get(t.id);
		if (i === undefined) next.push(t);
		else next[i] = t;
	}
	pending = {};
	transfers.list = next;
}

Events.On('transfer', (e) => {
	const t = e.data;
	pending[t.id] = t;
	frame ||= requestAnimationFrame(flush);
});
