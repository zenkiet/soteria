import { Events } from '@wailsio/runtime';
import type { IndexStatus } from '#bindings/soteria/internal/domain/models.ts';
import { Indexed } from '#bindings/soteria/internal/infra/wails/app.ts';

// One "index" subscription for every view; current is replaced on each event.
let status = $state.raw<IndexStatus | null>(null);
export const indexStatus = {
	get current() {
		return status;
	}
};

Indexed().then((s) => (status = s));
Events.On('index', (e) => (status = e.data as IndexStatus));
