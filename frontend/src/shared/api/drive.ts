import { InstallDriver, Mount } from '@bindings/soteria/internal/infra/wails/app';

// connectDrive mounts the drive, installing the WinFsp driver first when Windows reports it missing.
export async function connectDrive() {
	try {
		return await Mount();
	} catch (e) {
		if (!String(e).includes('WinFsp')) throw e;
		await InstallDriver();
		return Mount();
	}
}
