<script lang="ts">
	// Meal type as a quiet label with a coloured dot (no pill), so titles stay the loudest thing.
	let { type, minutes }: { type: string; minutes?: number } = $props();
	const dot: Record<string, string> = {
		breakfast: 'var(--c-carbs)',
		lunch: 'var(--c-fiber)',
		dinner: 'var(--c-protein)',
		snack: 'var(--c-fat)'
	};
</script>

<span class="text-base-content/65 inline-flex items-center gap-1.5 text-sm font-semibold">
	<span class="size-2 rounded-full" style="background: {dot[type] ?? 'currentColor'}"></span>
	<span><span class="capitalize">{type || 'meal'}</span>{#if minutes}<span class="font-normal">, {minutes}&nbsp;min</span>{/if}</span>
</span>
