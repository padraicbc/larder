<script lang="ts">
	import { goto, invalidateAll } from '$app/navigation';
	import { api, errorText } from '#lib/api.ts';
	import { round, sumMacros, todayISO } from '#lib/format.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import type { Recipe } from '#lib/types.ts';
	import Icon from '#lib/components/Icon.svelte';
	import MacroStripe from '#lib/components/MacroStripe.svelte';
	import MealBadge from '#lib/components/MealBadge.svelte';
	import Stars from '#lib/components/Stars.svelte';

	let { data } = $props();
	const targets = $derived(data.profile.targets);

	async function logToday(r: Recipe) {
		try {
			await api('/log', { body: { date: todayISO(), recipeId: r.id, servings: 1 } });
			toast(`Logged ${r.title} to today`);
		} catch (e) {
			toast(errorText(e), 'error');
		}
	}

	async function rate(r: Recipe, stars: number) {
		try {
			await api(`/recipes/${r.id}/ratings`, { body: { stars } });
			await invalidateAll();
			toast('Rating saved. Your next plan will use it.');
		} catch (e) {
			toast(errorText(e), 'error');
		}
	}

	let replaceDialog: HTMLDialogElement;
	let replacement = $state<{ day: number; index: number; recipe: Recipe } | null>(null);
	let replacementNotes = $state('');
	let replacing = $state(false);

	function chooseReplacement(day: number, index: number, recipe: Recipe) {
		replacement = { day, index, recipe };
		replacementNotes = '';
		replaceDialog.showModal();
	}

	async function replaceMeal() {
		if (!replacement || replacing) return;
		replacing = true;
		try {
			await api(`/plans/${data.plan.id}/replace-meal`, { body: {
				day: replacement.day, index: replacement.index,
				recipeId: replacement.recipe.id, notes: replacementNotes
			} });
			await invalidateAll();
			replaceDialog.close();
			toast('Meal replaced. Day totals and shopping list updated.');
		} catch (err) {
			toast(errorText(err), 'error');
		} finally {
			replacing = false;
		}
	}

	let confirmDelete: HTMLDialogElement;
	async function del() {
		try {
			await api(`/plans/${data.plan.id}`, { method: 'DELETE' });
			toast('Plan deleted', 'info');
			await goto('/plans');
		} catch (e) {
			toast(errorText(e), 'error');
		}
	}

	const dayTotal = (ids: string[]) => sumMacros(ids.map((id) => data.recipes[id]?.macros).filter(Boolean));
	// A recipe repeated on a later day is leftovers.
	const isRepeat = (id: string, day: number) => data.plan.days.some((d) => d.day < day && d.recipeIds.includes(id));
</script>

<svelte:head><title>{data.plan.title} · Larder</title></svelte:head>

<a href="/plans" class="btn btn-ghost btn-sm -ml-2"><Icon name="left" size={16} />All plans</a>

<div class="mt-2 flex flex-wrap items-end justify-between gap-3">
	<div>
		<h1 class="text-3xl font-extrabold lg:text-4xl">{data.plan.title}</h1>
		{#if data.plan.notes}<p class="text-base-content/70 mt-1 italic">“{data.plan.notes}”</p>{/if}
	</div>
	<div class="flex gap-2">
		<a href="/plans/{data.plan.id}/shopping" class="btn btn-primary"><Icon name="cart" size={18} />Shopping list</a>
		<button class="btn btn-ghost" aria-label="Delete plan" onclick={() => confirmDelete.showModal()}>
			<Icon name="trash" size={18} />
		</button>
	</div>
</div>

<!-- A menu board: days are columns on wide screens, stacked on phones. -->
<div
	class="mt-8 flex flex-col gap-12 lg:grid lg:auto-cols-[minmax(16rem,1fr)] lg:grid-flow-col lg:gap-8 lg:overflow-x-auto lg:pb-4"
>
	{#each data.plan.days as d (d.day)}
		{@const total = dayTotal(d.recipeIds)}
		<section aria-labelledby="day-{d.day}">
			<div class="border-base-content flex items-baseline justify-between border-b-2 pb-2">
				<h2 id="day-{d.day}" class="text-2xl font-extrabold">Day {d.day}</h2>
				<p class="tabular text-sm">
					<span class="font-semibold">{round(total.calories)}</span><span class="text-base-content/55">&nbsp;/ {targets.calories} kcal</span>
				</p>
			</div>
			<div class="mt-3"><MacroStripe m={total} height={8} legend /></div>

			<ol class="divide-base-content/10 mt-3 divide-y">
				{#each d.recipeIds as id, i (id + i)}
					{@const r = data.recipes[id]}
					{#if r}
						<li class="py-4">
							<div class="flex items-center justify-between gap-2">
								<MealBadge type={r.mealType} minutes={r.prepMinutes} />
								{#if isRepeat(id, d.day)}<span class="text-base-content/55 text-xs italic">leftovers</span>{/if}
							</div>
							<a href="/recipes/{r.id}" class="font-display link-hover mt-1 block text-xl leading-tight font-bold">{r.title}</a>
							{#if r.description}<p class="text-base-content/65 mt-1 text-sm">{r.description}</p>{/if}
							<div class="mt-3 flex items-center gap-3">
								<div class="flex-1"><MacroStripe m={r.macros} height={5} /></div>
								<span class="tabular text-sm font-semibold">{round(r.macros.calories)} kcal</span>
							</div>
							{#if r.warnings?.length}
								<p class="mt-2 flex gap-1.5 text-sm font-semibold">
									<Icon name="alert" size={16} class="text-warning mt-0.5 shrink-0" /><span>{r.warnings.join('. ')}</span>
								</p>
							{/if}
							<button class="btn btn-ghost btn-sm mt-2 -ml-3" disabled={replacing} onclick={() => chooseReplacement(d.day, i, r)} aria-label="Replace {r.title} on day {d.day}"><Icon name="refresh" size={16} />Replace meal</button>
							<div class="mt-2 flex items-center justify-between">
								<Stars size="sm" value={0} onrate={(n) => rate(r, n)} />
								<button class="btn btn-sm btn-ghost -mr-3" onclick={() => logToday(r)}>
									<Icon name="plus" size={16} />Log to today
								</button>
							</div>
						</li>
					{/if}
				{/each}
			</ol>
		</section>
	{/each}
</div>

<dialog bind:this={confirmDelete} class="modal">
	<div class="modal-box">
		<h3 class="text-xl font-bold">Delete this plan?</h3>
		<p class="mt-2">Its shopping list goes too. Recipes and ratings stay, so the app keeps learning from them.</p>
		<div class="modal-action">
			<form method="dialog"><button class="btn btn-ghost">Keep plan</button></form>
			<button class="btn btn-error" onclick={del}>Delete plan</button>
		</div>
	</div>
	<form method="dialog" class="modal-backdrop"><button>close</button></form>
</dialog>

<dialog bind:this={replaceDialog} class="modal modal-bottom sm:modal-middle" oncancel={(event) => { if (replacing) event.preventDefault(); }}>
	<div class="modal-box">
		<h3 class="text-xl font-bold">Replace {replacement?.recipe.mealType || 'meal'}?</h3>
		<p class="mt-2">Get a different idea for {replacement?.recipe.title} on day {replacement?.day}. The other meals stay as they are.</p>
		<p class="text-base-content/65 mt-2 text-sm">The replacement is checked against your dietary preferences, allergies and the day’s calorie and macro targets. The shopping list updates too.</p>
		<label class="mt-4 block">
			<span class="mb-1 block text-sm font-semibold">Anything you’d prefer? (optional)</span>
			<textarea class="textarea w-full" rows="2" bind:value={replacementNotes} disabled={replacing} placeholder="Something with fish, or a quicker dinner…"></textarea>
		</label>
		<div class="modal-action">
			<button class="btn btn-ghost" disabled={replacing} onclick={() => replaceDialog.close()}>Cancel</button>
			<button class="btn btn-primary" disabled={replacing} onclick={replaceMeal}>{#if replacing}<span class="loading loading-spinner loading-sm"></span>{/if}{replacing ? 'Finding another meal…' : 'Replace meal'}</button>
		</div>
	</div>
	<form method="dialog" class="modal-backdrop"><button disabled={replacing}>close</button></form>
</dialog>
