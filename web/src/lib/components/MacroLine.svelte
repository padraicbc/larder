<script lang="ts">
	import type { Macros } from '#lib/types.ts';
	import { round } from '#lib/format.ts';
	let { m, per = '' }: { m: Macros; per?: string } = $props();
</script>

<p class="tabular flex flex-wrap gap-x-3 gap-y-0.5 text-sm">
	<span class="font-semibold">{round(m.calories)} kcal{per}</span>
	<span style="color: var(--c-protein)">{round(m.protein)}g protein</span>
	<span style="color: var(--c-carbs)">{round(m.carbs)}g carbs</span>
	<span style="color: var(--c-fat)">{round(m.fat)}g fat</span>
</p>
