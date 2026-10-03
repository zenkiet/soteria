import { redirect } from '@sveltejs/kit';
import { Current } from '#/shared/api/index.ts';

export async function load() {
	const server = await Current();
	if (!server) redirect(307, '/');
	return { server };
}
