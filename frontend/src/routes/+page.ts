import { redirect } from '@sveltejs/kit';
import { Current } from '#/shared/api/index.ts';

export async function load() {
	if (await Current()) redirect(307, '/files');
}
