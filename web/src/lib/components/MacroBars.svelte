<script lang="ts">
	import type { Macros } from '#lib/types.ts';
	import { ratio, round } from '#lib/format.ts';

	let { totals, targets, compact = false }: { totals: Macros; targets: Macros; compact?: boolean } = $props();

	const ROWS = [
		{ key: 'protein', label: 'Protein', color: 'var(--c-protein)' },
		{ key: 'carbs', label: 'Carbs', color: 'var(--c-carbs)' },
		{ key: 'fat', label: 'Fat', color: 'var(--c-fat)' },
		{ key: 'fiber', label: 'Fiber', color: 'var(--c-fiber)' }
	] as const;
</script>

<ul class="flex flex-col {compact ? 'gap-1.5' : 'gap-3'}">
	{#each ROWS as row (row.key)}
		{@const v = totals[row.key]}
		{@const t = targets[row.key]}
		<li>
			<div class="flex items-baseline justify-between {compact ? 'text-xs' : 'text-sm'}">
				<span class="font-semibold">{row.label}</span>
				<span class="tabular text-base-content/70">
					{round(v)}<span class="text-base-content/45">&nbsp;/ {round(t)} g</span>
				</span>
			</div>
			<div class="mt-1 h-2 overflow-hidden rounded-full" style="background: color-mix(in oklab, {row.color} 15%, transparent)">
				<div
					class="h-full rounded-full transition-[width] duration-500"
					style="width: {ratio(v, t) * 100}%; background: {row.color}"
				></div>
			</div>
		</li>
	{/each}
</ul>
