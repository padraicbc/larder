import type { PageLoad } from './$types';

// Everything on this page is fetched as you use it (a scan, a search), so there is nothing to preload.
export const load: PageLoad = async () => ({});
