<script lang="ts">
	import type { Macros } from '#lib/types.ts';
	import { ratio, round } from '#lib/format.ts';

	let { totals, targets }: { totals: Macros; targets: Macros } = $props();

	const RINGS = [
		{ key: 'protein', label: 'Protein', r: 86, color: 'var(--c-protein)' },
		{ key: 'carbs', label: 'Carbs', r: 68, color: 'var(--c-carbs)' },
		{ key: 'fat', label: 'Fat', r: 50, color: 'var(--c-fat)' }
	] as const;
	const circ = (r: number) => 2 * Math.PI * r;

	const left = $derived(round(targets.calories - totals.calories));
	const label = $derived(
		RINGS.map((r) => `${r.label} ${round(totals[r.key])} of ${round(targets[r.key])} grams`).join(', ') +
			`. ${round(totals.calories)} of ${round(targets.calories)} calories.`
	);
</script>

<svg viewBox="0 0 200 200" class="w-full max-w-64" role="img" aria-label={label}>
	{#each RINGS as ring (ring.key)}
		{@const frac = ratio(totals[ring.key], targets[ring.key])}
		<circle cx="100" cy="100" r={ring.r} fill="none" stroke={ring.color} stroke-opacity="0.15" stroke-width="13" />
		{#if frac > 0}
			<circle
				cx="100"
				cy="100"
				r={ring.r}
				fill="none"
				stroke={ring.color}
				stroke-width="13"
				stroke-linecap="round"
				stroke-dasharray={circ(ring.r)}
				stroke-dashoffset={circ(ring.r) * (1 - frac)}
				transform="rotate(-90 100 100)"
				style="transition: stroke-dashoffset 600ms cubic-bezier(.2,.8,.2,1)"
			/>
		{/if}
	{/each}
	<text x="100" y="100" text-anchor="middle" class="fill-base-content tabular" style="font: 800 30px var(--font-display)">
		{Math.abs(left)}
	</text>
	<text x="100" y="121" text-anchor="middle" class="fill-base-content/60" style="font: 600 11px var(--font-sans)">
		{left >= 0 ? 'kcal left' : 'kcal over'}
	</text>
</svg>
