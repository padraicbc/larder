<script lang="ts">
	let {
		value = $bindable(0),
		readonly = false,
		size = 'md',
		onrate
	}: { value?: number; readonly?: boolean; size?: 'sm' | 'md' | 'lg'; onrate?: (stars: number) => void } = $props();

	// daisyUI rating needs a unique radio group name per instance.
	const name = `rating-${Math.random().toString(36).slice(2)}`;
	const sizeClass = { sm: 'rating-sm', md: 'rating-md', lg: 'rating-lg' };
</script>

{#if readonly}
	<div class="rating {sizeClass[size]} rating-half pointer-events-none" aria-label="{value} out of 5 stars">
		{#each [1, 2, 3, 4, 5] as n (n)}
			<div class="mask mask-star-2 bg-accent {value >= n - 0.25 ? '' : 'opacity-20'}" aria-hidden="true"></div>
		{/each}
	</div>
{:else}
	<div class="rating {sizeClass[size]}" role="radiogroup" aria-label="Rate this recipe">
		<!-- Checked when unrated, so the stars show empty instead of full. -->
		<input type="radio" {name} class="rating-hidden" aria-label="No rating" checked={value === 0} />
		{#each [1, 2, 3, 4, 5] as n (n)}
			<input
				type="radio"
				{name}
				class="mask mask-star-2 bg-accent"
				aria-label="{n} star{n > 1 ? 's' : ''}"
				checked={value === n}
				onchange={() => {
					value = n;
					onrate?.(n);
				}}
			/>
		{/each}
	</div>
{/if}
