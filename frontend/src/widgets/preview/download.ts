import { Events } from '@wailsio/runtime';
import {
	Download,
	DownloadDir,
	Drive,
	live,
	Mount,
	Open,
	PickFolder,
	transfers,
	type Entry,
	type Transfer
} from '#/shared/api/index.ts';
import { fileManager, msg, prefs, toast, windows } from '#/shared/lib/index.ts';

export async function downloadAll(items: Entry[]) {
	try {
		const dest = prefs.askDownloadDir ? await PickFolder() : prefs.downloadDir;
		if (prefs.askDownloadDir && !dest) return;
		for (const e of items)
			await (e.dir ? DownloadDir(e.path, dest) : Download(e.path, e.size, dest));
		toast(
			items.length === 1 ? `Downloading ${items[0].name}` : `Downloading ${items.length} items`
		);
	} catch (e) {
		toast(msg(e), 'error');
	}
}

// Opened from the drive, the app saves straight back to the server; a downloaded copy is the fallback where there is none.
export async function openFile(e: Entry) {
	const now = await Drive().catch(() => null);
	const drive = now?.mounted ? now : await Mount().catch(() => null);
	if (drive?.mounted) {
		try {
			await Open(
				drive.path.replace(/[\\/]$/, '') + (windows ? e.path.replaceAll('/', '\\') : e.path)
			);
			if (!now?.mounted) toast(`Opened from the Soteria drive, now in ${fileManager}`);
		} catch (err) {
			toast(msg(err), 'error');
		}
		return;
	}
	try {
		const id = await Download(e.path, e.size, prefs.downloadDir);
		toast(`Downloading ${e.name}`);
		const settle = (t?: Transfer) => {
			if (!t || live(t)) return;
			off();
			if (t.status === 'done') Open(t.local).catch((err) => toast(msg(err), 'error'));
		};
		const off = Events.On('transfers', (ev) => settle(ev.data?.find((x) => x.id === id)));
		// a small file can finish before the listener above is in place
		settle(transfers.list.find((x) => x.id === id));
	} catch (err) {
		toast(msg(err), 'error');
	}
}
