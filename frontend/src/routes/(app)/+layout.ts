import { Current } from '@/shared/api';
import { redirect } from '@sveltejs/kit';

export async function load() {
	const server = await Current();
	if (!server) redirect(307, '/');
	return { server };
}
