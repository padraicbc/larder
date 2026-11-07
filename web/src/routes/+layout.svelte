<script lang="ts">
	import '@fontsource-variable/atkinson-hyperlegible-next';
	import '@fontsource-variable/bricolage-grotesque/opsz.css';
	import '../app.css';
	import { goto, invalidateAll } from '$app/navigation';
	import { page } from '$app/state';
	import { api } from '#lib/api.ts';
	import Icon from '#lib/components/Icon.svelte';
	import { toasts } from '#lib/toast.svelte.ts';
	import '#lib/install.svelte.ts'; // start listening for the install prompt early

	let { children, data } = $props();

	// An open <dialog> lives in the browser's top layer and covers normal page content, so
	// toasts are a popover too and are re-shown on every new toast to land above any dialog.
	let toastBox = $state<HTMLElement>();
	$effect(() => {
		if (!toastBox) return;
		const count = toasts.length;
		try {
			toastBox.hidePopover();
			if (count) toastBox.showPopover();
		} catch {
			/* popover unsupported: toasts stay in the normal layer */
		}
	});

	// Desktop sidebar shows everything; the mobile dock has five slots, so
	// Settings lives behind Profile there.
	const NAV = [
		{ href: '/', label: 'Today', icon: 'today', dock: true },
		{ href: '/plans', label: 'Plans', icon: 'plans', dock: true },
		{ href: '/pantry', label: 'Pantry', icon: 'pantry', dock: true },
		{ href: '/recipes', label: 'Recipes', icon: 'recipes', dock: true },
		{ href: '/scan', label: 'Shop check', icon: 'camera', dock: false },
		{ href: '/workouts', label: 'Workouts', icon: 'plans', dock: false },
		{ href: '/weight', label: 'Weight', icon: 'profile', dock: false },
		{ href: '/meals', label: 'Meals', icon: 'pan', dock: false },
		{ href: '/products', label: 'Products', icon: 'cart', dock: false },
		{ href: '/profile', label: 'Profile', icon: 'profile', dock: true },
		{ href: '/settings', label: 'Settings', icon: 'settings', dock: false }
	] as const;

	const bare = $derived(page.url.pathname === '/login');
	const active = (href: string) =>
		href === '/' ? page.url.pathname === '/' : page.url.pathname.startsWith(href);
	const dockActive = (href: string) => active(href) || (href === '/profile' && active('/settings'));

	let online = $state(typeof navigator === 'undefined' ? true : navigator.onLine);

	async function signOut() {
		await api('/auth/logout', { method: 'POST' });
		await goto('/login');
		await invalidateAll();
	}
</script>

<svelte:window ononline={() => (online = true)} onoffline={() => (online = false)} />

{#if bare}
	{@render children()}
{:else}
	<div class="min-h-dvh lg:grid lg:grid-cols-[15rem_1fr]">
		<!-- Desktop sidebar -->
		<aside class="bg-base-200 sticky top-0 hidden h-dvh flex-col gap-6 px-4 py-6 lg:flex">
			<a href="/" class="flex items-center gap-2 px-2">
				<img src="/favicon.svg" alt="" class="size-8" />
				<span class="font-display text-2xl font-extrabold tracking-tight">Larder</span>
			</a>
			<nav aria-label="Main">
				<ul class="menu w-full gap-1 p-0">
					{#each NAV as item (item.href)}
						<li>
							<a href={item.href} class:menu-active={active(item.href)} aria-current={active(item.href) ? 'page' : undefined}>
								<Icon name={item.icon} />{item.label}
							</a>
						</li>
					{/each}
				</ul>
			</nav>
			{#if data.user && !data.user.local}
				<div class="border-base-300 mt-auto border-t px-2 pt-4">
					<p class="truncate text-sm font-semibold">{data.user.name || data.user.email}</p>
					{#if data.user.name}<p class="text-base-content/60 truncate text-xs">{data.user.email}</p>{/if}
					<button class="btn btn-ghost btn-sm -ml-3 mt-2" onclick={signOut}>Sign out</button>
				</div>
			{/if}
		</aside>

		<div class="flex min-w-0 flex-col pb-24 lg:pb-0">
			<!-- Mobile header -->
			<header
				class="bg-base-100/90 sticky top-0 z-20 flex items-center gap-2 px-4 py-3 backdrop-blur lg:hidden"
				style="padding-top: max(0.75rem, env(safe-area-inset-top))"
			>
				<img src="/favicon.svg" alt="" class="size-7" />
				<span class="font-display text-xl font-extrabold tracking-tight">Larder</span>
			</header>

			{#if !online}
				<div role="status" class="bg-warning text-warning-content px-4 py-2 text-center text-sm font-semibold">
					You’re offline. Changes won’t save until you reconnect.
				</div>
			{/if}
			<main class="mx-auto w-full max-w-5xl px-4 py-4 lg:px-10 lg:py-10">
				{@render children()}
			</main>
		</div>

		<!-- Mobile dock -->
		<nav class="dock dock-md bg-base-200 border-base-300 z-30 border-t lg:hidden" aria-label="Main">
			{#each NAV.filter((n) => n.dock) as item (item.href)}
				<a href={item.href} class:dock-active={dockActive(item.href)} aria-current={active(item.href) ? 'page' : undefined}>
					<Icon name={item.icon} />
					<span class="dock-label">{item.label}</span>
				</a>
			{/each}
		</nav>
	</div>
{/if}

<div bind:this={toastBox} popover="manual" class="toast z-50 m-0 overflow-visible border-0 bg-transparent p-0" style="inset: 1rem 1rem auto auto" aria-live="polite">
	{#each toasts as t (t.id)}
		<div class="alert {t.kind === 'error' ? 'alert-error' : t.kind === 'info' ? 'alert-info' : 'alert-success'} shadow-lg">
			<span>{t.text}</span>
		</div>
	{/each}
</div>
