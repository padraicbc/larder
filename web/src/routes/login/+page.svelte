<script lang="ts">
	import { goto, invalidateAll } from '$app/navigation';
	import { api, errorText } from '#lib/api.ts';

	let { data } = $props();

	let mode = $state<'signin' | 'register'>('signin');
	$effect(() => {
		if (data.status.firstUser) mode = 'register';
	});
	let email = $state('');
	let password = $state('');
	let name = $state('');
	let busy = $state(false);
	let error = $state('');

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		try {
			await api(mode === 'signin' ? '/auth/login' : '/auth/register', { body: { email, password, name } });
			await invalidateAll();
			await goto(data.next);
		} catch (err) {
			error = errorText(err);
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head><title>{mode === 'signin' ? 'Sign in' : 'Create account'} · Larder</title></svelte:head>

<div class="grid min-h-dvh place-items-center px-4 py-10">
	<div class="w-full max-w-sm">
		<img src="/favicon.svg" alt="" class="size-14" />
		<h1 class="mt-5 text-4xl font-extrabold">
			{#if data.status.firstUser}Set up Larder{:else if mode === 'signin'}Welcome back{:else}Create your account{/if}
		</h1>
		<p class="text-base-content/70 mt-2">
			{#if data.status.firstUser}
				This first account can also change the model settings.
			{:else if mode === 'signin'}
				Sign in to see today’s log and your plans.
			{:else}
				Your log, plans, pantry and ratings stay private to you.
			{/if}
		</p>

		<form class="mt-8 flex flex-col gap-4" onsubmit={submit}>
			{#if mode === 'register'}
				<label>
					<span class="mb-1 block text-sm font-semibold">Name <span class="text-base-content/50 font-normal">(optional)</span></span>
					<input class="input w-full" autocomplete="name" bind:value={name} />
				</label>
			{/if}
			<label>
				<span class="mb-1 block text-sm font-semibold">Email</span>
				<input class="input w-full" type="email" autocomplete="email" required bind:value={email} />
			</label>
			<label>
				<span class="mb-1 block text-sm font-semibold">Password</span>
				<input
					class="input w-full"
					type="password"
					minlength="8"
					required
					autocomplete={mode === 'signin' ? 'current-password' : 'new-password'}
					bind:value={password}
				/>
				{#if mode === 'register'}<span class="text-base-content/60 mt-1 block text-xs">At least 8 characters.</span>{/if}
			</label>

			{#if error}
				<div role="alert" class="alert alert-error alert-soft text-sm">{error}</div>
			{/if}

			<button class="btn btn-primary mt-2" disabled={busy}>
				{#if busy}<span class="loading loading-spinner loading-sm"></span>{/if}
				{mode === 'signin' ? 'Sign in' : 'Create account'}
			</button>
		</form>

		{#if !data.status.firstUser}
			<p class="text-base-content/70 mt-6 text-sm">
				{#if mode === 'signin'}
					{#if data.status.signupOpen}
						New here? <button class="link link-primary" onclick={() => ((mode = 'register'), (error = ''))}>Create an account</button>
					{:else}
						New accounts are turned off on this server. Ask whoever runs it.
					{/if}
				{:else}
					Already have an account? <button class="link link-primary" onclick={() => ((mode = 'signin'), (error = ''))}>Sign in</button>
				{/if}
			</p>
		{/if}
	</div>
</div>
