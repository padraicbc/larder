<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { ApiError, api, errorText, upload } from '#lib/api.ts';
	import { round } from '#lib/format.ts';
	import { shrinkImage } from '#lib/images.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import type { LabelEstimate, Scan } from '#lib/types.ts';
	import BarcodeField from '#lib/components/BarcodeField.svelte';
	import RatingBadge from '#lib/components/RatingBadge.svelte';
	import Icon from '#lib/components/Icon.svelte';

	let { data } = $props();

	let result = $state<Scan | null>(null);
	let busy = $state(false);
	let message = $state(''); // barcode lookup failure, shown under the scanner
	let photoError = $state('');
	let photoInput = $state<HTMLInputElement>();
	let notFound = $state(false); // the barcode isn't in our database or Open Food Facts
	let pendingBarcode = $state(''); // a barcode that failed to find anything: kept for when the ingredients arrive
	const missingIng = $derived(!!result && !result.ingredients?.trim());
	const missingNut = $derived(!!result && !result.hasNutrition);
	let nutInput = $state<HTMLInputElement>();
	let showNutForm = $state(false);
	let nut = $state<{ calories: number | null; protein: number | null; carbs: number | null; fat: number | null; fiber: number | null; unit: 'g' | 'ml' }>({ calories: null, protein: null, carbs: null, fat: null, fiber: null, unit: 'g' });
	const nutValid = $derived([nut.calories, nut.protein, nut.carbs, nut.fat, nut.fiber].some((v) => (v ?? 0) > 0));
	let editText = $state(''); // the ingredients as you can correct them
	let recalculating = $state(false);
	$effect(() => {
		editText = result?.ingredients ?? '';
	});
	const edited = $derived(editText.trim() !== (result?.ingredients ?? '').trim());
	let completing = $state<Scan | null>(null); // a product we already have, whose ingredients we're adding from a photo
	let saving = $state(false);
	let dialog = $state<HTMLDialogElement>();
	$effect(() => {
		if (!dialog) return;
		if (result && !dialog.open) dialog.showModal();
		else if (!result && dialog.open) dialog.close();
	});
	// Look up something you've saved: a name or barcode, searched as you type.
	let q = $state('');
	let hits = $state<Scan[]>([]);
	let searching = $state(false);
	let searched = $state(false);
	let seq = 0;
	$effect(() => {
		const term = q.trim();
		if (term.length < 2) {
			seq++;
			hits = [];
			searched = false;
			searching = false;
			return;
		}
		const mine = ++seq;
		searching = true;
		const timer = setTimeout(async () => {
			try {
				const found = await api<Scan[]>(`/scan/search?q=${encodeURIComponent(term)}`);
				if (mine === seq) {
					hits = found;
					searched = true;
				}
			} catch {
				if (mine === seq) hits = [];
			} finally {
				if (mine === seq) searching = false;
			}
		}, 300); // wait for a pause in typing
		return () => clearTimeout(timer);
	});

	const SOURCE: Record<Scan['source'], string> = {
		saved: 'One of your products',
		openfoodfacts: 'From Open Food Facts',
		photo: 'Read from your photo'
	};
	const lowers = $derived(result?.rating?.factors.filter((f) => f.points < 0) ?? []);
	const raises = $derived(result?.rating?.factors.filter((f) => f.points > 0) ?? []);
	const isSaved = $derived(!!result?.id || !!result?.productId || result?.source === 'saved');
	const BAR: Record<string, string> = { excellent: 'progress-success', good: 'progress-success', fair: 'progress-warning', poor: 'progress-error', avoid: 'progress-error' };

	async function checkBarcode(code: string) {
		busy = true;
		message = '';
		photoError = '';
		notFound = false;
		pendingBarcode = '';
		try {
			result = await api<Scan>(`/scan/barcode/${encodeURIComponent(code)}`);
		} catch (e) {
			result = null;
			if (e instanceof ApiError && e.status === 404) {
				notFound = true; // not an error as such: it's a product we haven't seen
				pendingBarcode = code;
			} else {
				message = errorText(e);
			}
		} finally {
			busy = false;
		}
	}

	async function checkIngredients(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		input.value = '';
		const base = completing;
		completing = null;
		if (!file) return;
		busy = true;
		message = '';
		photoError = '';
		try {
			const form = new FormData();
			form.append('photos', await shrinkImage(file), 'ingredients.jpg');
			const photo = await upload<Scan>('/scan/ingredients', form);
			if (!base) {
				// Keep the barcode that couldn't be found, so scanning it finds this product next time.
				const scan: Scan = { ...photo, barcode: pendingBarcode || photo.barcode || '' };
				notFound = false;
				pendingBarcode = '';
				if (scan.barcode) {
					// Saved straight away: reading it is how this product gets into your database.
					result = await api<Scan>('/scans', { body: scan });
					await invalidateAll();
					toast(result?.rating ? 'Ingredients read and saved. Scanning this barcode will now bring it up.' : 'Ingredients read and saved. A rating also needs the nutrition.', 'info');
				} else {
					result = scan; // no barcode yet: the popup asks for it, then it can be saved
				}
				return;
			}
			// Keep what we already knew (name, barcode, nutrition) and add what the photo gave us.
			const merged: Scan = {
				...base,
				ingredients: photo.ingredients,
				warning: '',
				barcode: base.barcode || photo.barcode || '',
				...(base.hasNutrition || !photo.hasNutrition ? {} : { hasNutrition: true, basisQuantity: photo.basisQuantity, basisUnit: photo.basisUnit, macros: photo.macros })
			};
			if (base.productId) {
				// One of your products: fill in its ingredients (nothing else changes).
				await api(`/products/${base.productId}/ingredients`, { method: 'PUT', body: { ingredients: photo.ingredients } });
				result = await api<Scan>('/scan/rate', { body: merged });
			} else if (base.id) {
				result = await api<Scan>('/scans', { body: merged }); // already saved: keep it up to date
			} else if (merged.barcode) {
				result = await api<Scan>('/scans', { body: merged }); // keep it, so this barcode shows the ingredients next time
			} else {
				result = await api<Scan>('/scan/rate', { body: merged });
			}
			await invalidateAll();
			toast(result?.rating ? 'Ingredients saved. Here is the rating.' : 'Ingredients saved. A rating also needs the nutrition.', 'info');
		} catch (e) {
			if (base) toast(errorText(e), 'error');
			else photoError = errorText(e);
		} finally {
			busy = false;
		}
	}

	/** Save corrected ingredients (on the scan or product if it's saved); the rating follows from them. */
	async function recalculate() {
		if (!result || recalculating) return;
		const text = editText.trim();
		recalculating = true;
		try {
			const merged: Scan = { ...result, ingredients: text, warning: text ? '' : result.warning };
			if (result.productId && text) {
				await api(`/products/${result.productId}/ingredients`, { method: 'PUT', body: { ingredients: text } });
				result = await api<Scan>('/scan/rate', { body: merged });
			} else if (result.id) {
				result = await api<Scan>('/scans', { body: merged });
			} else {
				result = await api<Scan>('/scan/rate', { body: merged });
			}
			await invalidateAll();
			toast('Ingredients saved. The rating has been updated.', 'info');
		} catch (e) {
			toast(errorText(e), 'error');
		} finally {
			recalculating = false;
		}
	}

	/** Merge new facts into what we know and keep them (on the scan or product if it's saved). */
	async function applyUpdate(base: Scan, patch: Partial<Scan>, message: (s: Scan) => string) {
		const merged: Scan = { ...base, ...patch, warning: patch.ingredients ? '' : base.warning };
		if (base.productId && patch.ingredients) {
			await api(`/products/${base.productId}/ingredients`, { method: 'PUT', body: { ingredients: patch.ingredients } });
			result = await api<Scan>('/scan/rate', { body: merged });
		} else if (base.id || merged.barcode) {
			result = await api<Scan>('/scans', { body: merged });
		} else {
			result = await api<Scan>('/scan/rate', { body: merged });
		}
		await invalidateAll();
		toast(message(result), 'info');
	}

	/** Read the nutrition panel from a photo and add it. */
	async function readNutrition(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		input.value = '';
		const base = result;
		if (!file || !base) return;
		busy = true;
		try {
			const form = new FormData();
			form.append('photos', await shrinkImage(file), 'nutrition.jpg');
			const label = await upload<LabelEstimate>('/log/label', form);
			await applyUpdate(
				base,
				{
					hasNutrition: true,
					basisQuantity: label.basisQuantity,
					basisUnit: label.basisUnit,
					macros: label.macros,
					barcode: base.barcode || label.barcode || '',
					...(base.ingredients?.trim() || !label.ingredients ? {} : { ingredients: label.ingredients })
				},
				(s) => (s.rating ? 'Nutrition added. Here is the rating.' : 'Nutrition added. A rating also needs the ingredients.')
			);
		} catch (e) {
			toast(errorText(e), 'error');
		} finally {
			busy = false;
		}
	}

	async function saveNutrition() {
		if (!result || !nutValid || busy) return;
		busy = true;
		try {
			const macros = { calories: nut.calories ?? 0, protein: nut.protein ?? 0, carbs: nut.carbs ?? 0, fat: nut.fat ?? 0, fiber: nut.fiber ?? 0 };
			await applyUpdate(result, { hasNutrition: true, basisQuantity: 100, basisUnit: nut.unit, macros }, (s) =>
				s.rating ? 'Nutrition saved. Here is the rating.' : 'Nutrition saved. A rating also needs the ingredients.'
			);
			showNutForm = false;
		} catch (e) {
			toast(errorText(e), 'error');
		} finally {
			busy = false;
		}
	}

	function addIngredients(base: Scan | null) {
		completing = base;
		photoInput?.click();
	}

	/** A barcode for something we only know from its ingredients, so scanning it finds it next time. */
	async function attachBarcode(code: string) {
		if (!result) return;
		result = { ...result, barcode: code };
		if (!result.id) {
			toast('Barcode added. Save the scan so it’s found by barcode next time.', 'info');
			return;
		}
		try {
			result = await api<Scan>('/scans', { body: result }); // already saved: keep it up to date
			await invalidateAll();
			toast('Barcode added. Scanning it will now find this product.', 'info');
		} catch (e) {
			toast(errorText(e), 'error');
		}
	}

	async function save() {
		if (!result || saving) return;
		saving = true;
		try {
			result = await api<Scan>('/scans', { body: result });
			await invalidateAll();
			toast(result?.productId ? 'Saved to Products. Look it up here any time.' : 'Saved as a draft. Add the missing ingredients or nutrition to finish it.');
		} catch (e) {
			toast(errorText(e), 'error');
		} finally {
			saving = false;
		}
	}

	/** Throw away an unfinished scan (a finished one is a product: delete it from Products). */
	async function removeDraft() {
		const d = result;
		if (!d?.id || !confirm(`Delete “${d.name}”?`)) return;
		try {
			await api(`/scans/${d.id}`, { method: 'DELETE' });
			result = null;
			await invalidateAll();
			toast('Deleted');
		} catch (e) {
			toast(errorText(e), 'error');
		}
	}

	function open(s: Scan) {
		result = s;
		message = '';
		photoError = '';
		q = '';
	}
</script>

<svelte:head><title>Shop check · Larder</title></svelte:head>

<h1 class="text-3xl font-extrabold lg:text-4xl">Shop check</h1>
<p class="text-base-content/70 mt-1 max-w-2xl text-sm">
	In the shop, scan a barcode to see how good a food is: sugars, additives, how processed it is. A rating needs both the <strong>ingredients</strong> and the <strong>nutrition</strong>; if either is missing there is no rating. If it isn’t found, photograph them (and the barcode): one photo showing everything works, or take separate photos.
</p>

<div class="mt-5 max-w-2xl">
	<BarcodeField onfound={checkBarcode} {busy} label="Check" {message} />

	<input bind:this={photoInput} type="file" accept="image/*" capture="environment" class="hidden" onchange={checkIngredients} />
	{#if notFound}
		<div class="rounded-box border-primary bg-primary/10 mt-4 border-2 p-4" role="status">
			<p class="text-lg font-bold">We don’t have this one yet</p>
			<p class="text-base-content/70 text-sm">A rating needs <strong>both the ingredients and the nutrition panel</strong>. Add them with <strong>one photo showing the barcode, ingredients and nutrition</strong>, or with <strong>separate photos</strong>. After that, just scanning the barcode brings it up.</p>
			<ol class="mt-3 space-y-2 text-sm">
				<li class="flex items-start gap-2">
					<span class="badge {pendingBarcode ? 'badge-success' : 'badge-ghost'} mt-0.5">{pendingBarcode ? '✓' : '1'}</span>
					<span><strong>Barcode:</strong> {#if pendingBarcode}<span class="font-mono">{pendingBarcode}</span> from your scan, we’ll keep it.{:else}scan or photograph it above.{/if}</span>
				</li>
				<li class="flex items-start gap-2">
					<span class="badge badge-ghost mt-0.5">2</span>
					<span><strong>Ingredients and nutrition:</strong> photograph the ingredients list and the nutrition panel (the same photo is fine), then add the other afterwards if the first didn’t show both.</span>
				</li>
			</ol>
			<button class="btn btn-primary mt-3 w-full sm:w-auto" disabled={busy} onclick={() => addIngredients(null)}>
				{#if busy}<span class="loading loading-spinner loading-sm"></span>Reading…{:else}<Icon name="camera" size={18} />Photograph the ingredients{/if}
			</button>
		</div>
	{:else}
		<div class="rounded-box border-base-300 mt-4 flex flex-col gap-2 border p-4 sm:flex-row sm:items-center sm:justify-between">
			<div>
				<p class="font-semibold">No barcode, or can’t scan it? Photograph the ingredients</p>
				<p class="text-base-content/60 text-sm">One picture of the ingredients list is enough. If the barcode is in the same picture it’s read too, so next time you can just scan it.</p>
			</div>
			<button class="btn btn-primary shrink-0" disabled={busy} onclick={() => addIngredients(null)}>
				{#if busy}<span class="loading loading-spinner loading-sm"></span>Checking…{:else}<Icon name="camera" size={18} />Ingredients photo{/if}
			</button>
		</div>
	{/if}
	{#if photoError}<div role="alert" class="alert alert-error mt-2 text-sm font-semibold shadow-md"><span>{photoError}</span></div>{/if}

	<div class="mt-6">
		<label class="text-sm font-semibold" for="shop-search">Look up something you’ve saved</label>
		<div class="relative mt-1">
			<input id="shop-search" class="input w-full" type="search" autocomplete="off" placeholder="Type a name or barcode…" bind:value={q} />
			{#if searching}<span class="loading loading-spinner loading-sm absolute right-3 top-3"></span>{/if}
		</div>
		{#if hits.length}
			<ul class="border-base-300 rounded-box divide-base-content/10 mt-1 divide-y border" aria-label="Matches">
				{#each hits as h (h.productId ?? h.id ?? `${h.barcode}${h.name}`)}
					<li class="flex items-center gap-2 pr-3">
						<button class="min-w-0 flex-1 p-3 text-left" onclick={() => open(h)}>
							<span class="block truncate font-semibold">{h.name}</span>
							<span class="text-base-content/60 block truncate text-xs">{h.productId ? 'Your product' : 'Unfinished'}{h.barcode ? ` · ${h.barcode}` : ''}</span>
						</button>
						{#if h.rating}
							<RatingBadge rating={h.rating} name={h.name} />
						{:else}
							<span class="badge badge-warning badge-soft" title="A rating needs both the ingredients and the nutrition">Needs {!h.ingredients?.trim() && !h.hasNutrition ? 'both' : !h.ingredients?.trim() ? 'ingredients' : 'nutrition'}</span>
						{/if}
					</li>
				{/each}
			</ul>
		{:else if searched && !searching}
			<p class="text-base-content/60 mt-2 text-sm">Nothing saved matches “{q.trim()}”. Scan its barcode or photograph the ingredients to add it.</p>
		{/if}
	</div>
</div>

<!-- The result opens as a popup the moment a scan or photo comes back, so it can't be missed. -->
<dialog bind:this={dialog} class="modal modal-bottom sm:modal-middle" aria-label="Scan result" onclose={() => (result = null)}>
	<div class="modal-box max-h-[90dvh]">
		<div class="flex justify-end -mb-6"><form method="dialog"><button class="btn btn-ghost btn-sm btn-square" aria-label="Close">✕</button></form></div>
		{#if result}
		{#if isSaved && result.source !== 'saved'}
			<p class="text-base-content/60 text-xs">{SOURCE[result.source]} · saved</p>
		{:else}
			<p class="text-base-content/60 text-xs">{SOURCE[result.source]}</p>
		{/if}

		{#if isSaved}
			<h2 class="text-xl font-bold">{result.name}</h2>
		{:else}
			<input class="input input-ghost -ml-3 w-full text-xl font-bold" aria-label="Product name" bind:value={result.name} />
		{/if}
		{#if result.barcode}<p class="text-base-content/50 font-mono text-xs">Barcode {result.barcode}</p>{/if}

		{#if result.rating && (result.warning || result.rating.provisional)}
			<div role="alert" class="alert alert-warning mt-3 text-sm shadow-md">
				<div>
					<p class="font-bold">Provisional rating</p>
					<p>{result.warning || 'The ingredients aren’t known, so additives and sugars can’t be ruled out.'}</p>
					<button class="btn btn-sm mt-2" disabled={busy} onclick={() => addIngredients(result)}>
						{#if busy}<span class="loading loading-spinner loading-sm"></span>Reading…{:else}<Icon name="camera" size={16} />Photograph the ingredients{/if}
					</button>
				</div>
			</div>
		{/if}

		{#if result.rating}
			<div class="mt-4 flex items-center gap-4">
				<p class="font-display tabular text-5xl font-extrabold">{result.rating.score}<span class="text-2xl">%</span></p>
				<div class="min-w-0 flex-1">
					<p class="text-lg font-bold capitalize">{result.rating.label}</p>
					<progress class="progress {BAR[result.rating.label]} w-full" value={result.rating.score} max="100" aria-hidden="true"></progress>
				</div>
			</div>
			<div class="mt-3 text-sm">
				{#if lowers.length}
					<p><span class="font-semibold">Marked down for:</span> {lowers.slice(0, 4).map((f) => f.text).join('; ')}{lowers.length > 4 ? '…' : ''}</p>
				{:else}
					<p class="font-semibold">Nothing marks it down.</p>
				{/if}
				{#if raises.length}<p class="text-base-content/70 mt-1"><span class="font-semibold">In its favour:</span> {raises.map((f) => f.text).join('; ')}</p>{/if}
			</div>
			<div class="mt-2"><RatingBadge rating={result.rating} name={result.name} variant="link" /></div>
		{:else}
			<div role="alert" class="alert alert-warning mt-4 shadow-md">
				<div class="w-full">
					<p class="text-lg font-bold">No rating yet</p>
					<p class="text-sm">A rating needs <strong>both</strong> the ingredients and the nutrition. Still needed: <strong>{[missingIng ? 'ingredients' : '', missingNut ? 'nutrition' : ''].filter(Boolean).join(' and ')}</strong>.</p>
					{#if result.warning && missingIng}<p class="mt-1 text-sm">{result.warning}</p>{/if}
					{#if missingIng && missingNut}<p class="mt-1 text-sm">One photo showing the ingredients and the nutrition panel works, or take one of each.</p>{/if}
					<div class="mt-3 flex flex-wrap gap-2">
						{#if missingIng}
							<button class="btn btn-sm btn-primary" disabled={busy} onclick={() => addIngredients(result)}>
								{#if busy}<span class="loading loading-spinner loading-sm"></span>Reading…{:else}<Icon name="camera" size={16} />Photograph the ingredients{/if}
							</button>
						{/if}
						{#if missingNut && !result.productId}
							<input bind:this={nutInput} type="file" accept="image/*" capture="environment" class="hidden" onchange={readNutrition} />
							<button class="btn btn-sm btn-primary" disabled={busy} onclick={() => nutInput?.click()}><Icon name="camera" size={16} />Photograph the nutrition panel</button>
							<button class="btn btn-sm" disabled={busy} onclick={() => (showNutForm = !showNutForm)}>Type the nutrition</button>
						{/if}
					</div>
					{#if showNutForm && missingNut}
						<div class="bg-base-100 rounded-box mt-3 p-3">
							<p class="text-sm font-semibold">Nutrition per 100 <select class="select select-xs w-auto" aria-label="Unit" bind:value={nut.unit}><option value="g">g</option><option value="ml">ml</option></select></p>
							<div class="mt-2 grid grid-cols-3 gap-2">
								<label class="text-xs">Calories<input class="input input-sm tabular mt-1 w-full" type="number" min="0" step="any" inputmode="decimal" bind:value={nut.calories} /></label>
								<label class="text-xs">Protein (g)<input class="input input-sm tabular mt-1 w-full" type="number" min="0" step="any" inputmode="decimal" bind:value={nut.protein} /></label>
								<label class="text-xs">Carbs (g)<input class="input input-sm tabular mt-1 w-full" type="number" min="0" step="any" inputmode="decimal" bind:value={nut.carbs} /></label>
								<label class="text-xs">Fat (g)<input class="input input-sm tabular mt-1 w-full" type="number" min="0" step="any" inputmode="decimal" bind:value={nut.fat} /></label>
								<label class="text-xs">Fibre (g)<input class="input input-sm tabular mt-1 w-full" type="number" min="0" step="any" inputmode="decimal" bind:value={nut.fiber} /></label>
							</div>
							<button class="btn btn-sm btn-primary mt-3" disabled={!nutValid || busy} onclick={saveNutrition}>Save nutrition</button>
						</div>
					{/if}
				</div>
			</div>
		{/if}

		{#if result.hasNutrition && result.basisQuantity}
			{@const per = result.basisUnit === 'serving' ? 1 : 100 / result.basisQuantity}
			<p class="tabular text-base-content/70 mt-3 text-sm">
				Per {result.basisUnit === 'serving' ? 'serving' : `100 ${result.basisUnit}`}: {round(result.macros.calories * per)} kcal · P {round(result.macros.protein * per, 1)} · C {round(result.macros.carbs * per, 1)} · F {round(result.macros.fat * per, 1)} · Fibre {round(result.macros.fiber * per, 1)}
			</p>
		{/if}

		{#if !result.barcode && result.source !== 'saved'}
			<div class="bg-base-200 rounded-box mt-3 p-3">
				<p class="text-sm font-semibold">Add the barcode too</p>
				<p class="text-base-content/60 mb-2 text-xs">Photograph or scan it once and next time scanning it finds this product, with its rating, straight away.</p>
				<BarcodeField onfound={attachBarcode} label="Add" />
			</div>
		{/if}

		<details class="mt-3" open={!result.ingredients}>
			<summary class="cursor-pointer text-sm font-semibold">Ingredients{result.ingredients ? '' : ' (none yet)'}: check or edit</summary>
			<textarea class="textarea mt-2 w-full text-sm" rows="4" aria-label="Ingredients" placeholder="Type or paste the ingredients list" bind:value={editText}></textarea>
			<div class="mt-2 flex flex-wrap gap-2">
				{#if edited}
					<button class="btn btn-sm btn-primary" disabled={recalculating || busy} onclick={recalculate}>
						{#if recalculating}<span class="loading loading-spinner loading-sm"></span>{/if}Save ingredients
					</button>
				{/if}
				<button class="btn btn-sm" disabled={busy || recalculating} onclick={() => addIngredients(result)}><Icon name="camera" size={16} />Photograph again</button>
			</div>
		</details>
		{#if result.source === 'photo'}
			<p class="text-base-content/50 mt-2 text-xs">Read by a model from your photo, so check the ingredients above against the pack.</p>
		{/if}

		<div class="mt-4 flex flex-wrap gap-2">
			{#if result.productId}
				<span class="badge badge-success badge-soft self-center">In your products</span>
				<a class="btn btn-ghost btn-sm" href="/products">View in Products</a>
			{:else if result.id}
				<span class="badge badge-warning badge-soft self-center">Saved draft</span>
				<button class="btn btn-ghost btn-sm text-error" onclick={removeDraft}>Delete</button>
			{:else}
				<button class="btn btn-primary btn-sm" disabled={saving || !result.name.trim()} onclick={save}>Save</button>
			{/if}
			<button class="btn btn-ghost btn-sm" onclick={() => (result = null)}>Done</button>
		</div>
		{/if}
	</div>
	<form method="dialog" class="modal-backdrop"><button aria-label="Close">close</button></form>
</dialog>
