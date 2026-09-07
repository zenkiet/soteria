import { CheckUpdate, UpdateStatus } from '@bindings/soteria/internal/infra/wails/app';
import type { Update } from '@bindings/soteria/internal/domain/models';
import { Events, Updater } from '@wailsio/runtime';

type Progress = { written: number; total: number; rate: number };

export const update = $state({
	s: { state: 'unconfigured' } as Update,
	progress: null as Progress | null,
	error: '',
	checkedAt: 0,
	open: false
});

const sync = () => UpdateStatus().then((s) => (update.s = s));
sync();

for (const n of [
	Updater.Events.DownloadStarted,
	Updater.Events.Verifying,
	Updater.Events.Installing
])
	Events.On(n, sync);
Events.On(Updater.Events.DownloadProgress, (e) => (update.progress = e.data));
Events.On(Updater.Events.UpdateReady, () => {
	update.progress = null;
	sync().then(() => (update.open = true));
});
Events.On(Updater.Events.Error, (e) => {
	update.error = e.data.message;
	update.progress = null;
	sync();
});

export const check = () => {
	update.error = '';
	return CheckUpdate()
		.catch((e) => (update.error = String(e)))
		.then(sync)
		.then(() => (update.checkedAt = Date.now()));
};
