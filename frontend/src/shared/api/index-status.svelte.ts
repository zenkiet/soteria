import { Events } from '@wailsio/runtime';
import type { IndexStatus } from '#bindings/soteria/internal/domain/models.ts';
import { Indexed } from '#bindings/soteria/internal/infra/wails/app.ts';

// One "index" subscription for every view; current is replaced on each event.
export const indexStatus = new (class {
	current = $state.raw<IndexStatus | null>(null);
})();

Indexed().then((s) => (indexStatus.current = s));
Events.On('index', (e) => (indexStatus.current = e.data));
