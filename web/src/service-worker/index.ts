// Makes Larder installable and quick to open. The app shell (JS, CSS, icons)
// is cached; data always comes from the network because it's personal and
// changes constantly. Offline, the shell still opens and shows a banner.
import { self } from '$app/service-worker';
import { version } from '$app/env';
import { assets, immutable } from '$app/manifest';

const CACHE = `larder-${version}`;
const SHELL = '/200.html';
const toURL = (p: string) => new URL(p, self.location.origin).pathname;
const PRECACHE = [...immutable.map((f) => toURL(f.path)), ...assets.map((f) => toURL(f.path)), SHELL];

self.addEventListener('install', (event) => {
	event.waitUntil(
		caches
			.open(CACHE)
			.then((c) => c.addAll(PRECACHE))
			.then(() => self.skipWaiting())
	);
});

self.addEventListener('activate', (event) => {
	event.waitUntil(
		caches
			.keys()
			.then((keys) => Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k))))
			.then(() => self.clients.claim())
	);
});

self.addEventListener('fetch', (event) => {
	const req = event.request;
	const url = new URL(req.url);
	// Never touch API calls, other origins or non-GETs.
	if (req.method !== 'GET' || url.origin !== self.location.origin || url.pathname.startsWith('/api/')) return;

	// Pages: network first so updates show up, cached shell when offline.
	if (req.mode === 'navigate') {
		event.respondWith(
			fetch(req).catch(async () => (await caches.match(SHELL)) ?? Response.error())
		);
		return;
	}

	// Build files and icons: cache first (filenames change when content does).
	if (PRECACHE.includes(url.pathname)) {
		event.respondWith(caches.match(url.pathname).then((hit) => hit ?? fetch(req)));
	}
});
