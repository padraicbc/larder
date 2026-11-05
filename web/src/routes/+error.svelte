<script lang="ts">
	import { page } from '$app/state';

	let online = $state(typeof navigator === 'undefined' ? true : navigator.onLine);
	const notFound = $derived(page.status === 404);
</script>

<!-- Retry automatically when the connection comes back. -->
<svelte:window ononline={() => location.reload()} onoffline={() => (online = false)} />

<svelte:head><title>{!online ? 'Offline' : notFound ? 'Not found' : 'Error'} · Larder</title></svelte:head>

<div class="mx-auto max-w-md py-16 text-center">
	<img src="/favicon.svg" alt="" class="mx-auto size-14 {online ? '' : 'opacity-40 grayscale'}" />
	{#if !online}
		<h1 class="mt-6 text-3xl font-extrabold">You’re offline</h1>
		<p class="text-base-content/70 mt-2">Larder needs a connection to load your log and plans. This page reloads by itself when you’re back online.</p>
	{:else if notFound}
		<h1 class="mt-6 text-3xl font-extrabold">Nothing here</h1>
		<p class="text-base-content/70 mt-2">That plan or recipe may have been deleted.</p>
		<a href="/" class="btn btn-primary mt-6">Go to today</a>
	{:else}
		<h1 class="mt-6 text-3xl font-extrabold">That didn’t load</h1>
		<p class="text-base-content/70 mt-2">{page.error?.message ?? 'The server didn’t respond.'} Check that the Larder server is running.</p>
	{/if}
	{#if notFound === false || !online}
		<button class="btn mt-6" onclick={() => location.reload()}>Try again</button>
	{/if}
</div>
