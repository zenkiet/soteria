import { redirect } from '@sveltejs/kit';
import { Current } from '#/shared/api/index.ts';
import { home } from '#/shared/lib/index.ts';

export async function load() {
	const s = await Current();
	if (s) redirect(307, home(s));
}
