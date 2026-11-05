<script lang="ts">
	import { goto, invalidateAll } from '$app/navigation';
	import { untrack } from 'svelte';
	import { page } from '$app/state';
	import { api, errorText } from '#lib/api.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import { install, promptInstall } from '#lib/install.svelte.ts';
	import type { LLMConfig } from '#lib/types.ts';

	let { data } = $props();

	// Deeply reactive local draft, reset whenever the page data reloads.
	let cfg = $state<LLMConfig>(structuredClone(untrack(() => data.settings.llm)));
	$effect(() => {
		cfg = structuredClone(data.settings.llm);
	});

	// Presets just fill in provider + base URL; everything stays editable.
	const PRESETS = [
		{ id: 'mock', label: 'Demo mode', hint: 'Canned recipes, no model needed', provider: 'mock', baseUrl: '', model: '' },
		{ id: 'ollama', label: 'Ollama', hint: 'Runs on your machine', provider: 'openai', baseUrl: 'http://localhost:11434/v1', model: 'llama3.1' },
		{ id: 'lmstudio', label: 'LM Studio', hint: 'Runs on your machine', provider: 'openai', baseUrl: 'http://localhost:1234/v1', model: '' },
		{ id: 'openai', label: 'OpenAI', hint: 'Needs an API key', provider: 'openai', baseUrl: 'https://api.openai.com/v1', model: '' },
		{ id: 'openrouter', label: 'OpenRouter', hint: 'Many models, one key', provider: 'openai', baseUrl: 'https://openrouter.ai/api/v1', model: '' },
		{ id: 'anthropic', label: 'Anthropic', hint: 'Claude models, needs a key', provider: 'anthropic', baseUrl: 'https://api.anthropic.com', model: 'claude-sonnet-5-5' }
	] as const;

	const activePreset = $derived(
		PRESETS.find((p) => p.provider === cfg.provider && (p.provider === 'mock' || p.baseUrl === cfg.baseUrl))?.id ?? 'custom'
	);

	function choose(p: (typeof PRESETS)[number]) {
		cfg.provider = p.provider;
		cfg.baseUrl = p.baseUrl;
		if (p.model || cfg.provider !== data.settings.llm.provider) cfg.model = p.model;
	}

	let testing = $state(false);
	let result = $state<{ ok: boolean; text: string } | null>(null);
	let saving = $state(false);

	async function test() {
		testing = true;
		result = null;
		try {
			const r = await api<{ latencyMs: number }>('/settings/test', { body: { llm: cfg } });
			result = { ok: true, text: `Connected. The model replied in ${(r.latencyMs / 1000).toFixed(1)} s.` };
		} catch (e) {
			result = { ok: false, text: errorText(e) };
		} finally {
			testing = false;
		}
	}

	async function save() {
		saving = true;
		try {
			await api('/settings', { method: 'PUT', body: { llm: cfg } });
			await invalidateAll();
			toast('Settings saved');
		} catch (e) {
			toast(errorText(e), 'error');
		} finally {
			saving = false;
		}
	}

	async function signOut() {
		await api('/auth/logout', { method: 'POST' });
		await goto('/login');
		await invalidateAll();
	}

	type BackupSummary = { exportedAt: string; logs: number; products: number; recipes: number; plans: number; pantry: number; ratings: number; dayAdjustments: number };
	let backupFile = $state<HTMLInputElement>();
	let restoreDialog: HTMLDialogElement;
	let backupPayload = $state<unknown>(null);
	let backupSummary = $state<BackupSummary | null>(null);
	let backupName = $state('');
	let checkingBackup = $state(false);
	let restoring = $state(false);

	async function chooseBackup(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		input.value = '';
		if (!file) return;
		checkingBackup = true;
		backupPayload = null;
		backupSummary = null;
		try {
			if (file.size > 32 * 1024 * 1024) throw new Error('Backup is over 32 MB.');
			let payload: unknown;
			try { payload = JSON.parse(await file.text()); } catch { throw new Error('Choose a valid Larder backup JSON file.'); }
			const summary = await api<BackupSummary>('/backup/preview', { body: payload });
			backupPayload = payload;
			backupSummary = summary;
			backupName = file.name;
			restoreDialog.showModal();
		} catch (err) {
			toast(errorText(err), 'error');
		} finally {
			checkingBackup = false;
		}
	}

	async function restoreBackup() {
		if (!backupPayload || restoring) return;
		restoring = true;
		try {
			await api('/backup/restore', { body: backupPayload });
			await invalidateAll();
			restoreDialog.close();
			backupPayload = null;
			toast('Backup restored. Your data has been replaced.');
		} catch (err) {
			toast(errorText(err), 'error');
		} finally {
			restoring = false;
		}
	}

	// ---- appearance (stored in this browser only) ----
	let theme = $state(typeof localStorage !== 'undefined' ? (localStorage.getItem('theme') ?? 'system') : 'system');
	function setTheme(t: string) {
		theme = t;
		try {
			localStorage.setItem('theme', t);
		} catch {}
		const dark = t === 'dark' || (t === 'system' && matchMedia('(prefers-color-scheme: dark)').matches);
		document.documentElement.dataset.theme = dark ? 'larder-night' : 'larder';
	}
</script>

<svelte:head><title>Settings · Larder</title></svelte:head>

<h1 class="text-3xl font-extrabold lg:text-4xl">Settings</h1>

<section class="mt-6 max-w-3xl">
	<h2 class="text-xl font-bold">Model</h2>
	<p class="text-base-content/70 mt-1 text-sm">The model writes meal plans, suggests pantry recipes, reads photos and estimates macros.</p>
	{#if !data.settings.canEdit}
		<div role="status" class="alert alert-info alert-soft mt-4 text-sm">
			Only the server’s admin can change the model. You’re using {data.settings.llm.provider === 'mock' ? 'demo mode' : data.settings.llm.model}.
		</div>
	{/if}
	<fieldset disabled={!data.settings.canEdit} class="contents">

	<div class="mt-4 grid grid-cols-2 gap-2 sm:grid-cols-3">
		{#each PRESETS as p (p.id)}
			<button
				type="button"
				class="rounded-box border p-3 text-left transition-colors {activePreset === p.id
					? 'border-primary bg-primary/10'
					: 'border-base-300 hover:bg-base-200'}"
				aria-pressed={activePreset === p.id}
				onclick={() => choose(p)}
			>
				<span class="block font-semibold">{p.label}</span>
				<span class="text-base-content/60 text-xs">{p.hint}</span>
			</button>
		{/each}
	</div>

	{#if cfg.provider !== 'mock'}
		<div class="mt-6 grid gap-4 sm:grid-cols-2">
			<label class="sm:col-span-2">
				<span class="mb-1 block text-sm font-semibold">API format</span>
				<select class="select w-full" bind:value={cfg.provider}>
					<option value="openai">OpenAI-compatible (/chat/completions)</option>
					<option value="anthropic">Anthropic (/v1/messages)</option>
				</select>
			</label>
			<label class="sm:col-span-2">
				<span class="mb-1 block text-sm font-semibold">Base URL</span>
				<input class="input w-full font-mono text-sm" bind:value={cfg.baseUrl} placeholder="http://localhost:11434/v1" />
			</label>
			<label>
				<span class="mb-1 block text-sm font-semibold">Model</span>
				<input class="input w-full font-mono text-sm" bind:value={cfg.model} placeholder="Model name" />
			</label>
			<label>
				<span class="mb-1 block text-sm font-semibold">API key</span>
				<input class="input w-full font-mono text-sm" type="password" autocomplete="off" bind:value={cfg.apiKey} placeholder={data.settings.envKeyLoaded ? 'Using NUTRI_LLM_API_KEY' : 'Not needed for local models'} />
			</label>
			<label>
				<span class="mb-1 flex justify-between text-sm font-semibold">Creativity <span class="tabular text-base-content/60">{cfg.temperature.toFixed(1)}</span></span>
				<input class="range range-sm range-primary w-full" type="range" min="0" max="1.2" step="0.1" bind:value={cfg.temperature} />
				<span class="text-base-content/60 flex justify-between text-xs"><span>Predictable</span><span>Adventurous</span></span>
			</label>
			<div class="grid grid-cols-2 gap-3">
				<label>
					<span class="mb-1 block text-sm font-semibold">Max tokens</span>
					<input class="input w-full" type="number" min="256" step="256" bind:value={cfg.maxTokens} />
				</label>
				<label>
					<span class="mb-1 block text-sm font-semibold">Timeout (s)</span>
					<input class="input w-full" type="number" min="10" step="10" bind:value={cfg.timeoutSeconds} />
				</label>
			</div>
			<label class="sm:col-span-2">
				<span class="mb-1 block text-sm font-semibold">Model for photos <span class="text-base-content/50 font-normal">(optional)</span></span>
				<input class="input w-full font-mono text-sm" bind:value={cfg.visionModel} placeholder="Leave empty to use the model above" />
				<span class="text-base-content/60 mt-1 block text-xs">Reading fridge photos needs a model that accepts images, such as llava or qwen2.5vl on Ollama.</span>
			</label>
			{#if cfg.provider === 'openai'}
				<label class="flex cursor-pointer items-start gap-3 sm:col-span-2">
					<input type="checkbox" class="toggle toggle-primary mt-0.5" bind:checked={cfg.jsonMode} />
					<span>
						<span class="block text-sm font-semibold">Force JSON replies</span>
						<span class="text-base-content/60 text-xs">Sends response_format. Turn off if your server rejects it.</span>
					</span>
				</label>
			{/if}
		</div>
	{/if}

	{#if result}
		<div role="status" class="alert alert-soft mt-5 {result.ok ? 'alert-success' : 'alert-error'}">
			<span class="break-all">{result.text}</span>
		</div>
	{/if}

	<div class="mt-5 flex gap-2">
		<button class="btn" disabled={testing} onclick={test}>
			{#if testing}<span class="loading loading-spinner loading-sm"></span>{/if}Test connection
		</button>
		<button class="btn btn-primary" disabled={saving} onclick={save}>Save settings</button>
	</div>
	</fieldset>
</section>

<section class="mt-12 max-w-3xl">
	<h2 class="text-xl font-bold">Backup and restore</h2>
	<p class="text-base-content/70 mt-2 text-sm">Save your profile, food logs, products and nutrition, calorie adjustments, recipes, plans, shopping lists, pantry and ratings to a JSON file. Keep it somewhere you can access if you change phones.</p>
	<p class="text-base-content/60 mt-2 text-sm">This backs up your own data. Account details, model settings, API keys and appearance are excluded; restoring keeps your current sign-in and model settings.</p>
	<div class="mt-4 flex flex-wrap gap-2">
		<a class="btn btn-primary" href="/api/backup" download>Download backup</a>
		<input bind:this={backupFile} type="file" accept="application/json,.json" class="hidden" onchange={chooseBackup} />
		<button class="btn" disabled={checkingBackup || restoring} onclick={() => backupFile?.click()}>{#if checkingBackup}<span class="loading loading-spinner loading-sm"></span>{/if}Restore from file</button>
	</div>
	<p class="text-base-content/60 mt-2 text-xs">You’ll review the file before restoring. Restore replaces your current data rather than merging it; download a backup first if you want to keep both versions.</p>
</section>

<dialog bind:this={restoreDialog} class="modal modal-bottom sm:modal-middle" oncancel={(event) => { if (restoring) event.preventDefault(); }}>
	<div class="modal-box">
		<h3 class="text-xl font-bold">Replace your data with this backup?</h3>
		<p class="mt-2 break-all font-semibold">{backupName}</p>
		{#if backupSummary}
			<p class="text-base-content/60 mt-1 text-sm">Exported {new Date(backupSummary.exportedAt).toLocaleString()}</p>
			<dl class="mt-4 grid grid-cols-2 gap-2 text-sm">
				{#each [['Food logs', backupSummary.logs], ['Saved products', backupSummary.products], ['Recipes', backupSummary.recipes], ['Plans', backupSummary.plans], ['Pantry items', backupSummary.pantry], ['Ratings', backupSummary.ratings], ['Day adjustments', backupSummary.dayAdjustments]] as [label, count]}
					<div class="flex justify-between gap-2 rounded-box bg-base-200 p-2"><dt>{label}</dt><dd class="font-semibold">{count}</dd></div>
				{/each}
			</dl>
		{/if}
		<p class="mt-4 text-sm">Your existing profile and saved data will be replaced. Your account and model settings stay as they are.</p>
		<div class="modal-action">
			<button class="btn btn-ghost" disabled={restoring} onclick={() => { restoreDialog.close(); backupPayload = null; }}>Cancel</button>
			<button class="btn btn-error" disabled={restoring || !backupSummary} onclick={restoreBackup}>{#if restoring}<span class="loading loading-spinner loading-sm"></span>{/if}{restoring ? 'Restoring…' : 'Replace data and restore'}</button>
		</div>
	</div>
	<form method="dialog" class="modal-backdrop"><button disabled={restoring}>close</button></form>
</dialog>

<section class="mt-12 max-w-3xl">
	<h2 class="text-xl font-bold">Account</h2>
	{#if page.data.user?.local}
		<p class="mt-2">Everything you add is stored privately on this phone. Uninstalling the app deletes it.</p>
	{:else if page.data.user}
		<p class="mt-2">Signed in as <span class="font-semibold">{page.data.user.email}</span>{page.data.user.admin ? ' (admin)' : ''}.</p>
		<button class="btn btn-sm mt-3" onclick={signOut}>Sign out</button>
	{/if}
	{#if install.event}
		<div class="mt-4">
			<button class="btn btn-primary btn-sm" onclick={promptInstall}>Install app</button>
			<p class="text-base-content/60 mt-1 text-xs">Adds Larder to your home screen and opens it full screen.</p>
		</div>
	{/if}
</section>

<section class="mt-12 max-w-3xl">
	<h2 class="text-xl font-bold">Appearance</h2>
	<div class="join mt-3">
		{#each [['system', 'Match device'], ['light', 'Light'], ['dark', 'Dark']] as [v, label] (v)}
			<input class="join-item btn btn-sm" type="radio" name="theme" aria-label={label} checked={theme === v} onchange={() => setTheme(v)} />
		{/each}
	</div>
</section>
