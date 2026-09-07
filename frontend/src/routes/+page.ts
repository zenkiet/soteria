import { Current } from '@/shared/api';
import { redirect } from '@sveltejs/kit';

export async function load() {
	if (await Current()) redirect(307, '/files');
}
