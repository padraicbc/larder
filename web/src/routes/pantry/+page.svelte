<script lang="ts">
	import { untrack } from 'svelte';
	import { api, errorText, upload } from '#lib/api.ts';
	import { cap, groupBy, splitList } from '#lib/format.ts';
	import { shrinkImage } from '#lib/images.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import { MEAL_TYPES, PANTRY_CATEGORIES, type DetectedItem, type PantryItem, type Recipe } from '#lib/types.ts';
	import Icon from '#lib/components/Icon.svelte';
	import MacroStripe from '#lib/components/MacroStripe.svelte';
	import MealBadge from '#lib/components/MealBadge.svelte';

	let { data } = $props();

	let items = $state<PantryItem[]>(untrack(() => data.items));
	$effect(() => {
		items = data.items;
	});
	const groups = $derived(groupBy(items, (i) => i.category || 'other'));

	// ---------- add by typing ----------
	let quick = $state('');
	async function addTyped() {
		const names = splitList(quick);
		if (!names.length) return;
		await addItems(names.map((name) => ({ name })));
		quick = '';
	}

	async function addItems(list: Partial<PantryItem>[]) {
		try {
			const res = await api<{ items: PantryItem[]; added: number }>('/pantry', { body: { items: list } });
			items = res.items;
			const updated = list.length - res.added;
			toast(
				res.added && updated ? `Added ${res.added}, updated ${updated}` : res.added ? `Added ${res.added}` : `Updated ${updated}`
			);
		} catch (e) {
			toast(errorText(e), 'error');
		}
	}

	async function setQuantity(it: PantryItem, quantity: string) {
		if (quantity === it.quantity) return;
		try {
			items = await api<PantryItem[]>(`/pantry/${it.id}`, { method: 'PATCH', body: { quantity } });
		} catch (e) {
			toast(errorText(e), 'error');
		}
	}

	async function remove(it: PantryItem) {
		try {
			await api(`/pantry/${it.id}`, { method: 'DELETE' });
			items = items.filter((x) => x.id !== it.id);
		} catch (e) {
			toast(errorText(e), 'error');
		}
	}

	let confirmClear: HTMLDialogElement;
	async function clearAll() {
		try {
			await api('/pantry', { method: 'DELETE' });
			items = [];
			confirmClear.close();
			toast('Pantry cleared', 'info');
		} catch (e) {
			toast(errorText(e), 'error');
		}
	}

	// ---------- add from photos ----------
	let fileInput: HTMLInputElement;
	let reviewDialog: HTMLDialogElement;
	let scanning = $state(false);
	let previews = $state<string[]>([]);
	type Row = DetectedItem & { keep: boolean };
	let detected = $state<Row[]>([]);
	const keepCount = $derived(detected.filter((d) => d.keep).length);

	async function onPhotos(e: Event) {
		const files = Array.from((e.currentTarget as HTMLInputElement).files ?? []).slice(0, 4);
		(e.currentTarget as HTMLInputElement).value = '';
		if (!files.length) return;
		scanning = true;
		previews.forEach(URL.revokeObjectURL);
		previews = [];
		try {
			const form = new FormData();
			for (const f of files) {
				const blob = await shrinkImage(f).catch(() => f); // fall back to the original if the browser can't decode it
				form.append('photos', blob, f.name);
				previews.push(URL.createObjectURL(blob));
			}
			const res = await upload<{ items: DetectedItem[] }>('/pantry/scan', form);
			detected = res.items.map((d) => ({ ...d, keep: !d.have }));
			if (!detected.length) {
				toast('No food spotted. Try a closer, brighter photo.', 'info');
				return;
			}
			reviewDialog.showModal();
		} catch (err) {
			toast(errorText(err), 'error');
		} finally {
			scanning = false;
		}
	}

	async function addDetected() {
		const chosen = detected.filter((d) => d.keep && d.name.trim());
		reviewDialog.close();
		await addItems(chosen.map(({ name, quantity, category }) => ({ name, quantity, category, source: 'photo' })));
	}

	// ---------- cook ----------
	let meal = $state('any');
	let count = $state(2);
	let strict = $state(false);
	let notes = $state('');
	let cooking = $state(false);
	let ideas = $state<Recipe[]>([]);
	let cookError = $state('');

	async function cook() {
		cooking = true;
		cookError = '';
		try {
			const res = await api<{ recipes: Recipe[] }>('/pantry/cook', {
				body: { mealType: meal === 'any' ? '' : meal, count, strict, notes }
			});
			ideas = res.recipes;
		} catch (e) {
			cookError = errorText(e);
		} finally {
			cooking = false;
		}
	}
</script>

<svelte:head><title>Pantry · Larder</title></svelte:head>

<div class="flex flex-wrap items-end justify-between gap-3">
	<div>
		<h1 class="text-3xl font-extrabold lg:text-4xl">Pantry</h1>
		<p class="text-base-content/70 mt-1">What’s in your fridge and cupboards. {items.length} {items.length === 1 ? 'item' : 'items'}.</p>
	</div>
</div>

<!-- Add -->
<section class="mt-6 flex flex-col gap-3 sm:flex-row">
	<div class="join flex-1">
		<input
			class="input join-item w-full"
			placeholder="eggs, spinach, half a bag of rice"
			aria-label="Add ingredients, separated by commas"
			bind:value={quick}
			onkeydown={(e) => e.key === 'Enter' && addTyped()}
		/>
		<button class="btn btn-primary join-item" disabled={!quick.trim()} onclick={addTyped}>Add</button>
	</div>
	<input bind:this={fileInput} type="file" accept="image/*" multiple class="hidden" onchange={onPhotos} />
	<button class="btn btn-secondary btn-soft" disabled={scanning} onclick={() => fileInput.click()}>
		{#if scanning}
			<span class="loading loading-spinner loading-sm"></span>Looking at your photos…
		{:else}
			<Icon name="camera" size={18} />Add from photos
		{/if}
	</button>
</section>
<p class="text-base-content/60 mt-2 text-xs">Up to 4 photos of your fridge, cupboard, shopping bags or a receipt. You check the list before anything is added.</p>

<div class="mt-8 grid items-start gap-8 lg:grid-cols-[1fr_22rem]">
	<!-- Cook panel: first on mobile, right-hand column on desktop -->
	<section class="bg-base-200 rounded-box order-first p-5 lg:sticky lg:top-6 lg:order-last">
		<h2 class="flex items-center gap-2 text-xl font-bold"><Icon name="pan" />Cook with what I have</h2>

		<label class="mt-4 block">
			<span class="mb-1 block text-sm font-semibold">Meal</span>
			<select class="select w-full" bind:value={meal}>
				<option value="any">Whatever works</option>
				{#each MEAL_TYPES as t (t)}<option value={t}>{cap(t)}</option>{/each}
			</select>
		</label>

		<div class="mt-3">
			<span class="mb-1 block text-sm font-semibold">Ideas</span>
			<div class="join w-full">
				{#each [1, 2, 3] as n (n)}
					<input class="join-item btn btn-sm flex-1" type="radio" name="count" aria-label={String(n)} checked={count === n} onchange={() => (count = n)} />
				{/each}
			</div>
		</div>

		<label class="mt-4 flex cursor-pointer items-start gap-3">
			<input type="checkbox" class="toggle toggle-primary mt-0.5" bind:checked={strict} />
			<span>
				<span class="block text-sm font-semibold">Only use what I have</span>
				<span class="text-base-content/60 text-xs">Plus salt, pepper, oil and dried spices. Off allows up to 3 extras.</span>
			</span>
		</label>

		<label class="mt-4 block">
			<span class="mb-1 block text-sm font-semibold">Anything else?</span>
			<input class="input w-full" placeholder="Ready in 15 minutes" bind:value={notes} />
		</label>

		{#if cookError}<div role="alert" class="alert alert-error alert-soft mt-4 text-sm">{cookError}</div>{/if}

		<button class="btn btn-primary mt-4 w-full" disabled={cooking || !items.length} onclick={cook}>
			{#if cooking}<span class="loading loading-spinner loading-sm"></span>Thinking it through…{:else}<Icon name="sparkle" size={18} />Suggest recipes{/if}
		</button>
		{#if !items.length}<p class="text-base-content/60 mt-2 text-xs">Add a few ingredients first.</p>{/if}
	</section>

	<div class="flex min-w-0 flex-col gap-8">
		{#if ideas.length}
			<section aria-live="polite">
				<h2 class="mb-3 text-xl font-bold">Ideas from your pantry</h2>
				<ul class="divide-base-content/10 border-base-content divide-y border-t-2">
					{#each ideas as r (r.id)}
						<li class="py-4">
							<MealBadge type={r.mealType} minutes={r.prepMinutes} />
							<a href="/recipes/{r.id}" class="font-display link-hover mt-1 block text-xl leading-tight font-bold">{r.title}</a>
							{#if r.description}<p class="text-base-content/70 mt-1 text-sm">{r.description}</p>{/if}
							<div class="mt-3 flex items-center gap-3">
								<div class="flex-1"><MacroStripe m={r.macros} height={5} /></div>
								<span class="tabular text-sm font-semibold">{Math.round(r.macros.calories)} kcal</span>
							</div>
							{#if r.usesPantry?.length}
								<p class="mt-3 text-sm"><span class="font-semibold">Uses</span> {r.usesPantry.join(', ')}</p>
							{/if}
							{#if r.missing?.length}
								<p class="text-sm"><span class="font-semibold">You’d need</span> {r.missing.map((m) => m.name).join(', ')}</p>
							{:else}
								<p class="text-success text-sm font-semibold">Nothing to buy</p>
							{/if}
							{#if r.warnings?.length}
								<p class="mt-2 flex gap-1.5 text-sm font-semibold"><Icon name="alert" size={16} class="text-warning mt-0.5 shrink-0" /><span>{r.warnings.join('. ')}</span></p>
							{/if}
						</li>
					{/each}
				</ul>
			</section>
		{/if}

		<section>
			{#if items.length === 0}
				<div class="border-base-300 rounded-box border border-dashed p-6 text-center">
					<p class="font-semibold">Your pantry is empty</p>
					<p class="text-base-content/60 mt-1 text-sm">Type a few things above, or snap your fridge.</p>
				</div>
			{:else}
				<div class="grid gap-6 sm:grid-cols-2 lg:grid-cols-1 xl:grid-cols-2">
					{#each groups as [category, list] (category)}
						<div>
							<h2 class="mb-2 text-lg font-bold">{cap(category)}</h2>
							<ul class="bg-base-200 rounded-box divide-base-300 divide-y">
								{#each list as it (it.id)}
									<li class="flex items-center gap-2 py-1.5 pl-4 pr-1.5">
										<span class="min-w-0 flex-1 truncate">{cap(it.name)}</span>
										<input
											class="input input-ghost input-sm tabular w-24 shrink-0 text-right"
											placeholder="amount"
											aria-label="Amount of {it.name}"
											value={it.quantity}
											onblur={(e) => setQuantity(it, e.currentTarget.value.trim())}
											onkeydown={(e) => e.key === 'Enter' && e.currentTarget.blur()}
										/>
										<button class="btn btn-ghost btn-square btn-sm" aria-label="Remove {it.name}" onclick={() => remove(it)}>
											<Icon name="trash" size={16} />
										</button>
									</li>
								{/each}
							</ul>
						</div>
					{/each}
				</div>
				<button class="btn btn-ghost btn-sm mt-6" onclick={() => confirmClear.showModal()}>Clear pantry</button>
			{/if}
		</section>
	</div>
</div>

<dialog bind:this={reviewDialog} class="modal modal-bottom sm:modal-middle">
	<div class="modal-box max-w-xl">
		<h3 class="text-2xl font-bold">Here’s what I spotted</h3>
		<p class="text-base-content/70 mt-1 text-sm">Untick anything wrong and fix names or amounts before adding.</p>
		{#if previews.length}
			<div class="mt-3 flex gap-2 overflow-x-auto">
				{#each previews as src (src)}<img {src} alt="Your upload" class="rounded-field h-20 w-20 shrink-0 object-cover" />{/each}
			</div>
		{/if}
		<ul class="mt-4 flex flex-col gap-2">
			{#each detected as d, i (i)}
				<li class="flex items-center gap-2">
					<input type="checkbox" class="checkbox checkbox-primary" aria-label="Add {d.name}" bind:checked={d.keep} />
					<input class="input input-sm min-w-0 flex-1" aria-label="Name" bind:value={d.name} />
					<input class="input input-sm w-24 sm:w-28" aria-label="Amount" placeholder="amount" bind:value={d.quantity} />
					<select class="select select-sm w-28 max-sm:hidden" aria-label="Category" bind:value={d.category}>
						{#each PANTRY_CATEGORIES as c (c)}<option value={c}>{c}</option>{/each}
					</select>
					{#if d.have}
						<span class="badge badge-sm badge-ghost shrink-0">have</span>
					{:else if d.confidence === 'low'}
						<span class="badge badge-sm badge-warning badge-soft shrink-0">unsure</span>
					{/if}
				</li>
			{/each}
		</ul>
		<div class="modal-action">
			<form method="dialog"><button class="btn btn-ghost">Cancel</button></form>
			<button class="btn btn-primary" disabled={!keepCount} onclick={addDetected}>Add {keepCount} {keepCount === 1 ? 'item' : 'items'}</button>
		</div>
	</div>
	<form method="dialog" class="modal-backdrop"><button>close</button></form>
</dialog>

<dialog bind:this={confirmClear} class="modal">
	<div class="modal-box">
		<h3 class="text-xl font-bold">Clear your pantry?</h3>
		<p class="mt-2">Every item is removed. Recipes you’ve already made from it stay.</p>
		<div class="modal-action">
			<form method="dialog"><button class="btn btn-ghost">Keep items</button></form>
			<button class="btn btn-error" onclick={clearAll}>Clear pantry</button>
		</div>
	</div>
	<form method="dialog" class="modal-backdrop"><button>close</button></form>
</dialog>
