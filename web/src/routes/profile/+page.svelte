<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { untrack } from 'svelte';
	import { api, errorText } from '#lib/api.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import type { Macros, Profile } from '#lib/types.ts';
	import TagInput from '#lib/components/TagInput.svelte';
	import Icon from '#lib/components/Icon.svelte';

	let { data } = $props();

	// Deeply reactive local draft, reset whenever the page data reloads.
	let p = $state<Profile>(structuredClone(untrack(() => data.profile)));
	$effect(() => {
		p = structuredClone(data.profile);
	});
	let saving = $state(false);
	const dirty = $derived(JSON.stringify(p) !== JSON.stringify(data.profile));

	const DIETS = ['vegetarian', 'vegan', 'pescatarian', 'gluten-free', 'dairy-free', 'egg-free', 'keto', 'halal'];
	const ACTIVITY = [
		['sedentary', 'Mostly sitting'],
		['light', 'Light: 1–3 workouts a week'],
		['moderate', 'Moderate: 3–5 a week'],
		['active', 'Active: 6–7 a week'],
		['very_active', 'Very active: physical job or twice a day']
	];

	function toggleDiet(d: string) {
		p.diets = p.diets.includes(d) ? p.diets.filter((x) => x !== d) : [...p.diets, d];
	}

	async function calculate() {
		try {
			p.targets = await api<Macros>('/profile/targets', { body: p });
			toast('Targets updated. Save to keep them.', 'info');
		} catch (e) {
			toast(errorText(e), 'error');
		}
	}

	async function save() {
		saving = true;
		try {
			await api('/profile', { method: 'PUT', body: p });
			await invalidateAll();
			toast('Profile saved');
		} catch (e) {
			toast(errorText(e), 'error');
		} finally {
			saving = false;
		}
	}

	const TARGETS: { key: keyof Macros; label: string; unit: string; color: string }[] = [
		{ key: 'calories', label: 'Calories', unit: 'kcal', color: 'currentColor' },
		{ key: 'protein', label: 'Protein', unit: 'g', color: 'var(--c-protein)' },
		{ key: 'carbs', label: 'Carbs', unit: 'g', color: 'var(--c-carbs)' },
		{ key: 'fat', label: 'Fat', unit: 'g', color: 'var(--c-fat)' },
		{ key: 'fiber', label: 'Fiber', unit: 'g', color: 'var(--c-fiber)' }
	];
</script>

<svelte:head><title>Profile · Larder</title></svelte:head>

<div class="flex items-end justify-between gap-3">
	<h1 class="text-3xl font-extrabold lg:text-4xl">Profile</h1>
	<a href="/settings" class="btn btn-ghost btn-sm lg:hidden"><Icon name="settings" size={16} />Settings</a>
</div>

<nav class="mt-6 lg:hidden" aria-label="More">
	<ul class="grid grid-cols-2 gap-2">
		{#each [['/scan', 'Shop check', 'Scan food to check its quality'], ['/workouts', 'Workouts', 'Log sessions on a calendar'], ['/weight', 'Weight', 'Log and chart your weight'], ['/meals', 'Meals', 'Your saved meals'], ['/products', 'Products', 'Scanned products and ratings']] as [href, label, hint] (href)}
			<li><a {href} class="border-base-300 hover:bg-base-200 rounded-box block border p-3"><span class="block font-semibold">{label}</span><span class="text-base-content/60 text-xs">{hint}</span></a></li>
		{/each}
	</ul>
</nav>

<div class="mt-6 flex max-w-3xl flex-col gap-10">
	<section>
		<h2 class="text-xl font-bold">Daily targets</h2>
		<p class="text-base-content/70 mt-1 text-sm">Set them yourself, or fill in your stats below and calculate.</p>
		<div class="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-5">
			{#each TARGETS as t (t.key)}
				<label>
					<span class="mb-1 block text-sm font-semibold" style="color: {t.color}">{t.label}</span>
					<label class="input w-full">
						<input type="number" min="0" class="tabular" bind:value={p.targets[t.key]} />
						<span class="text-base-content/50 text-xs">{t.unit}</span>
					</label>
				</label>
			{/each}
		</div>
	</section>

	<section>
		<h2 class="text-xl font-bold">About you</h2>
		<p class="text-base-content/70 mt-1 text-sm">Used only to calculate targets (Mifflin-St Jeor). Metric units.</p>
		<div class="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-4">
			<label class="col-span-2 sm:col-span-1">
				<span class="mb-1 block text-sm font-semibold">Sex</span>
				<select class="select w-full" bind:value={p.sex}>
					<option value="">Prefer not to say</option><option value="female">Female</option><option value="male">Male</option>
				</select>
			</label>
			<label><span class="mb-1 block text-sm font-semibold">Age</span><input class="input w-full" type="number" min="0" bind:value={() => p.age || null, (v) => (p.age = v ?? 0)} /></label>
			<label><span class="mb-1 block text-sm font-semibold">Height (cm)</span><input class="input w-full" type="number" min="0" bind:value={() => p.heightCm || null, (v) => (p.heightCm = v ?? 0)} /></label>
			<label><span class="mb-1 block text-sm font-semibold">Weight (kg)</span><input class="input w-full" type="number" min="0" step="0.1" bind:value={() => p.weightKg || null, (v) => (p.weightKg = v ?? 0)} /></label>
			<label class="col-span-2">
				<span class="mb-1 block text-sm font-semibold">Activity</span>
				<select class="select w-full" bind:value={p.activity}>
					{#each ACTIVITY as [v, label] (v)}<option value={v}>{label}</option>{/each}
				</select>
			</label>
			<label class="col-span-2">
				<span class="mb-1 block text-sm font-semibold">Goal</span>
				<select class="select w-full" bind:value={p.goal}>
					<option value="lose">Lose weight</option><option value="maintain">Maintain</option><option value="gain">Build muscle</option>
				</select>
			</label>
		</div>
		<button class="btn btn-secondary btn-soft mt-4" onclick={calculate}>Calculate targets</button>
	</section>

	<section>
		<h2 class="text-xl font-bold">How you eat</h2>
		<p class="mb-2 mt-3 text-sm font-semibold">Diet</p>
		<div class="flex flex-wrap gap-2">
			{#each DIETS as d (d)}
				<button
					type="button"
					class="btn btn-sm rounded-full {p.diets.includes(d) ? 'btn-primary' : 'btn-outline border-base-300'}"
					aria-pressed={p.diets.includes(d)}
					onclick={() => toggleDiet(d)}>{d}</button
				>
			{/each}
		</div>

		<div class="mt-5 grid gap-5 sm:grid-cols-2">
			<div>
				<label for="allergies" class="mb-1 block text-sm font-semibold">Allergies</label>
				<TagInput id="allergies" bind:tags={p.allergies} tone="error" placeholder="e.g. peanuts" suggestions={['nuts', 'peanuts', 'shellfish', 'sesame', 'soy', 'eggs']} />
				<p class="text-base-content/60 mt-1 text-xs">Every recipe is checked for these and flagged if anything slips through.</p>
			</div>
			<div>
				<label for="excluded" class="mb-1 block text-sm font-semibold">Foods to leave out</label>
				<TagInput id="excluded" bind:tags={p.excluded} placeholder="e.g. mushrooms" suggestions={['mushrooms', 'olives', 'coriander', 'tofu']} />
			</div>
			<div class="sm:col-span-2">
				<label for="cuisines" class="mb-1 block text-sm font-semibold">Cuisines you enjoy</label>
				<TagInput id="cuisines" bind:tags={p.cuisines} placeholder="e.g. Thai" suggestions={['Mediterranean', 'Mexican', 'Indian', 'Japanese', 'Middle Eastern']} />
			</div>
		</div>

		<div class="mt-5 grid grid-cols-3 gap-3">
			<label>
				<span class="mb-1 block text-sm font-semibold">Meals a day</span>
				<input class="input w-full" type="number" min="1" max="6" bind:value={p.mealsPerDay} />
			</label>
			<label>
				<span class="mb-1 block text-sm font-semibold">Cooking for</span>
				<input class="input w-full" type="number" min="1" bind:value={p.householdSize} />
			</label>
			<label>
				<span class="mb-1 block text-sm font-semibold">Max prep (min)</span>
				<input class="input w-full" type="number" min="0" step="5" bind:value={() => p.maxPrepMins || null, (v) => (p.maxPrepMins = v ?? 0)} placeholder="No limit" />
			</label>
		</div>
	</section>
</div>

<!-- Sticky save bar appears only with unsaved changes -->
{#if dirty}
	<div class="bg-base-100/95 border-base-300 sticky bottom-20 z-10 mt-8 flex items-center justify-between gap-3 border-t py-3 backdrop-blur lg:bottom-0">
		<span class="text-sm">You have unsaved changes</span>
		<div class="flex gap-2">
			<button class="btn btn-ghost btn-sm" onclick={() => (p = structuredClone(data.profile))}>Discard</button>
			<button class="btn btn-primary btn-sm" disabled={saving} onclick={save}>Save profile</button>
		</div>
	</div>
{/if}
