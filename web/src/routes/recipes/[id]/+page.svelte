<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { api, errorText } from '#lib/api.ts';
	import { cap, fmtQty, todayISO } from '#lib/format.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import Icon from '#lib/components/Icon.svelte';
	import MacroStripe from '#lib/components/MacroStripe.svelte';
	import MealBadge from '#lib/components/MealBadge.svelte';
	import Stars from '#lib/components/Stars.svelte';

	let { data } = $props();
	const r = $derived(data.recipe);

	let servings = $derived(data.recipe.servings || 1);
	const scale = $derived(servings / (r.servings || 1));

	let stars = $state(0);
	let comment = $state('');
	let saving = $state(false);

	async function rate() {
		saving = true;
		try {
			await api(`/recipes/${r.id}/ratings`, { body: { stars, comment } });
			stars = 0;
			comment = '';
			await invalidateAll();
			toast('Rating saved. Your next plan will use it.');
		} catch (e) {
			toast(errorText(e), 'error');
		} finally {
			saving = false;
		}
	}

	async function logToday() {
		try {
			await api('/log', { body: { date: todayISO(), recipeId: r.id, servings: 1 } });
			toast(`Logged ${r.title} to today`);
		} catch (e) {
			toast(errorText(e), 'error');
		}
	}
</script>

<svelte:head><title>{r.title} · Larder</title></svelte:head>

<a href="/recipes" class="btn btn-ghost btn-sm -ml-2"><Icon name="left" size={16} />Recipes</a>

<header class="mt-2">
	<MealBadge type={r.mealType} minutes={r.prepMinutes} />
	<h1 class="mt-2 max-w-3xl text-4xl leading-[1.05] font-extrabold lg:text-6xl">{r.title}</h1>
	{#if r.description}<p class="text-base-content/75 mt-3 max-w-prose text-lg">{r.description}</p>{/if}
	{#if r.tags?.length}<p class="text-base-content/55 mt-2 text-sm">{r.tags.join(', ')}</p>{/if}

	<div class="mt-6 max-w-xl">
		<p class="flex items-baseline gap-2">
			<span class="font-display tabular text-3xl font-extrabold">{Math.round(r.macros.calories)}</span>
			<span class="text-base-content/60">kcal per serving</span>
		</p>
		<div class="mt-2"><MacroStripe m={r.macros} height={10} legend /></div>
	</div>
	<div class="mt-4 flex flex-wrap items-center gap-3">
		<button class="btn btn-primary" onclick={logToday}><Icon name="plus" size={18} />Log a serving today</button>
		{#if r.ratingCount}
			<span class="flex items-center gap-2 text-sm"><Stars readonly value={r.avgRating} size="sm" />{r.avgRating} from {r.ratingCount}</span>
		{/if}
	</div>
</header>

{#if r.source === 'pantry'}
	<div class="bg-base-200 rounded-box mt-6 max-w-2xl p-4 text-sm">
		<p><span class="font-semibold">Made from your pantry:</span> {r.usesPantry?.join(', ') || 'basics only'}</p>
		{#if r.missing?.length}<p class="mt-1"><span class="font-semibold">You’d need:</span> {r.missing.map((m) => m.name).join(', ')}</p>{/if}
	</div>
{/if}

{#if r.warnings?.length}
	<div role="alert" class="alert alert-warning alert-soft mt-6">
		<Icon name="alert" />
		<div>
			<p class="font-semibold">Check this recipe against your restrictions</p>
			<ul class="list-disc pl-5 text-sm">{#each r.warnings as w (w)}<li>{w}</li>{/each}</ul>
		</div>
	</div>
{/if}

<div class="mt-8 grid gap-8 lg:grid-cols-[minmax(16rem,22rem)_1fr]">
	<section>
		<div class="flex items-center justify-between">
			<h2 class="text-xl font-bold">Ingredients</h2>
			<div class="join" aria-label="Servings">
				<button class="btn btn-sm join-item" aria-label="Fewer servings" onclick={() => (servings = Math.max(1, servings - 1))}>−</button>
				<span class="btn btn-sm join-item pointer-events-none tabular">{servings} serving{servings === 1 ? '' : 's'}</span>
				<button class="btn btn-sm join-item" aria-label="More servings" onclick={() => servings++}>+</button>
			</div>
		</div>
		<ul class="mt-4 flex flex-col gap-2.5">
			{#each r.ingredients as ing (ing.name)}
				<li class="flex items-baseline gap-2">
					<span>{cap(ing.name)}</span>
					<span class="leader" aria-hidden="true"></span>
					<span class="tabular font-semibold">{fmtQty(ing.quantity * scale, ing.unit)}</span>
				</li>
			{/each}
		</ul>
	</section>

	<section>
		<h2 class="text-xl font-bold">Method</h2>
		<ol class="mt-3 flex max-w-prose flex-col gap-4">
			{#each r.steps as step, i (i)}
				<li class="flex gap-4">
					<span class="bg-primary text-primary-content font-display grid size-8 shrink-0 place-items-center rounded-full font-bold">{i + 1}</span>
					<p class="pt-1 leading-relaxed">{step}</p>
				</li>
			{/each}
		</ol>
	</section>
</div>

<section class="bg-base-200 rounded-box mt-10 max-w-2xl p-5">
	<h2 class="text-xl font-bold">How was it?</h2>
	<p class="text-base-content/70 mt-1 text-sm">A sentence on why helps more than stars alone: “too much chilli”, “great cold the next day”.</p>
	<div class="mt-3"><Stars bind:value={stars} size="lg" /></div>
	<textarea class="textarea mt-3 w-full" rows="2" placeholder="What would make it better?" bind:value={comment}></textarea>
	<button class="btn btn-primary mt-3" disabled={!stars || saving} onclick={rate}>Save rating</button>

	{#if data.ratings.length}
		<ul class="border-base-300 mt-5 flex flex-col gap-3 border-t pt-4">
			{#each [...data.ratings].reverse() as rt (rt.id)}
				<li>
					<div class="flex items-center gap-2">
						<Stars readonly value={rt.stars} size="sm" />
						<span class="text-base-content/50 text-xs">{new Date(rt.createdAt).toLocaleDateString()}</span>
					</div>
					{#if rt.comment}<p class="mt-0.5 text-sm">{rt.comment}</p>{/if}
				</li>
			{/each}
		</ul>
	{/if}
</section>
