import { Transfers } from '@bindings/soteria/internal/infra/wails/app';
import type { Transfer } from '@bindings/soteria/internal/domain/models';
import { Events } from '@wailsio/runtime';

export const transfers = $state<{ list: Transfer[] }>({ list: [] });

Transfers().then((l) => (transfers.list = l ?? []));

Events.On('transfer', (e) => {
	const t = e.data as Transfer;
	const i = transfers.list.findIndex((x) => x.id === t.id);
	if (i < 0) transfers.list.push(t);
	else transfers.list[i] = t;
});
