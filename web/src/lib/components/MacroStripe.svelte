<script lang="ts">
	// Where a meal's calories come from, like the colour band on a food label:
	// protein and carbs give 4 kcal per gram, fat 9.
	import type { Macros } from '#lib/types.ts';
	import { round } from '#lib/format.ts';

	let { m, height = 6, legend = false }: { m: Macros; height?: number; legend?: boolean } = $props();

	const parts = $derived.by(() => {
		const kcal = { protein: m.protein * 4, carbs: m.carbs * 4, fat: m.fat * 9 };
		const total = kcal.protein + kcal.carbs + kcal.fat || 1;
		return [
			{ key: 'protein', label: 'protein', grams: m.protein, share: kcal.protein / total, color: 'var(--c-protein)' },
			{ key: 'carbs', label: 'carbs', grams: m.carbs, share: kcal.carbs / total, color: 'var(--c-carbs)' },
			{ key: 'fat', label: 'fat', grams: m.fat, share: kcal.fat / total, color: 'var(--c-fat)' }
		];
	});
	const label = $derived(parts.map((p) => `${Math.round(p.share * 100)}% of calories from ${p.label}`).join(', '));
</script>

<div role="img" aria-label={label} class="flex w-full gap-[2px] overflow-hidden rounded-full" style="height: {height}px">
	{#each parts as p (p.key)}
		{#if p.share > 0}<span style="flex: {p.share}; background: {p.color}"></span>{/if}
	{/each}
</div>
{#if legend}
	<div class="mt-1.5 flex flex-wrap gap-x-4 gap-y-0.5 text-sm" aria-hidden="true">
		{#each parts as p (p.key)}
			<span style="color: {p.color}" class="font-semibold whitespace-nowrap">{round(p.grams)}g {p.label}</span>
		{/each}
	</div>
{/if}
