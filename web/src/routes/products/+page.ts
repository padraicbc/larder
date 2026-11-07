import { api } from '#lib/api.ts';
import type { Product } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => ({ products: await api<Product[]>('/products', { fetch }) });
