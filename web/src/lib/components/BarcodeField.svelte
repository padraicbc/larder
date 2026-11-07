<script lang="ts">
	import { onDestroy } from 'svelte';
	import { errorText, upload } from '#lib/api.ts';
	import { shrinkImage } from '#lib/images.ts';
	import { canScanBarcodes, cleanBarcode } from '#lib/barcode.ts';
	import Icon from '#lib/components/Icon.svelte';

	let { onfound, busy = false, label = 'Look up', message = '' }: { onfound: (code: string) => void; busy?: boolean; label?: string; message?: string } = $props();

	let code = $state('');
	let scanning = $state(false);
	let error = $state('');
	let reading = $state(false);
	let liveFailed = $state(false);
	const liveOk = $derived(canScanBarcodes() && window.isSecureContext);
	const BARS = [2, 1, 3, 1, 2, 2, 1, 3, 1, 1, 2, 3, 1, 2, 1, 1, 3, 2, 1, 2];
	let photo = $state<HTMLInputElement>();
	let video = $state<HTMLVideoElement>();
	let stream: MediaStream | null = null;
	let timer: ReturnType<typeof setInterval> | undefined;

	const valid = $derived(/^\d{8}$|^\d{12,14}$/.test(cleanBarcode(code)));

	function submit() {
		if (valid && !busy) onfound(cleanBarcode(code));
	}

	function stop() {
		clearInterval(timer);
		stream?.getTracks().forEach((t) => t.stop());
		stream = null;
		scanning = false;
	}

	async function scan() {
		error = '';
		liveFailed = false;
		try {
			// eslint-disable-next-line @typescript-eslint/no-explicit-any
			const detector = new (window as any).BarcodeDetector({ formats: ['ean_13', 'ean_8', 'upc_a'] });
			stream = await navigator.mediaDevices.getUserMedia({ video: { facingMode: 'environment' } });
			scanning = true;
			await Promise.resolve(); // let the <video> mount
			if (!video) return stop();
			video.srcObject = stream;
			await video.play();
			let checking = false;
			timer = setInterval(async () => {
				if (checking || !video) return;
				checking = true;
				try {
					const found = await detector.detect(video);
					if (found[0]?.rawValue) {
						code = found[0].rawValue;
						stop();
						submit();
					}
				} catch {
					/* frame not ready; try again */
				} finally {
					checking = false;
				}
			}, 250);
		} catch (e) {
			stop();
			const name = (e as DOMException)?.name;
			liveFailed = true;
			error =
				name === 'NotAllowedError' || name === 'SecurityError'
					? 'Camera access was blocked. In the Android app: Settings → Apps → Larder → Permissions → Camera → Allow (or install the latest version, which asks for it). Meanwhile, Photo works without it.'
					: name === 'NotFoundError'
						? 'No camera was found on this device. Use Photo, or type the digits.'
						: 'The live camera couldn’t start. Use Photo instead, or type the digits.';
		}
	}

	/** Reads a barcode from a picture: on-device when the browser can, otherwise via the photo model. */
	async function fromPhoto(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		input.value = '';
		if (!file) return;
		error = '';
		reading = true;
		try {
			let found = '';
			if (canScanBarcodes()) {
				try {
					// eslint-disable-next-line @typescript-eslint/no-explicit-any
					const detector = new (window as any).BarcodeDetector({ formats: ['ean_13', 'ean_8', 'upc_a'] });
					const bitmap = await createImageBitmap(file);
					found = (await detector.detect(bitmap))[0]?.rawValue ?? '';
				} catch {
					/* fall through to the server */
				}
			}
			if (!found) {
				const form = new FormData();
				form.append('photos', await shrinkImage(file), 'barcode.jpg');
				found = (await upload<{ barcode: string }>('/products/barcode-photo', form)).barcode;
			}
			code = found;
			submit();
		} catch (e) {
			error = errorText(e);
		} finally {
			reading = false;
		}
	}

	onDestroy(stop);
</script>

<div>
	<input bind:this={photo} type="file" accept="image/*" capture="environment" class="hidden" onchange={fromPhoto} />

	<div class="grid grid-cols-2 gap-2">
		<!-- Live scan: the camera looks for the barcode itself, no picture is taken -->
		<button
			type="button"
			class="rounded-box border-base-300 hover:bg-base-200 flex flex-col items-center gap-1 border p-3 text-center disabled:opacity-60"
			disabled={!liveOk || busy || reading}
			onclick={scan}
		>
			<svg viewBox="0 0 96 64" class="text-base-content w-full max-w-28" aria-hidden="true">
				<rect x="1" y="1" width="94" height="62" rx="6" fill="currentColor" fill-opacity="0.06" />
				<g fill="currentColor" fill-opacity="0.75">
					{#each BARS as w, i (i)}<rect x={17 + BARS.slice(0, i).reduce((n, v) => n + v + 0.9, 0) * 1.15} y="20" width={w * 1.15} height="24" />{/each}
				</g>
				<!-- viewfinder corners + scan line -->
				<g fill="none" stroke="var(--color-primary)" stroke-width="3" stroke-linecap="round">
					<path d="M12 18v-6h6M84 18v-6h-6M12 46v6h6M84 46v6h-6" />
				</g>
				<rect x="12" y="30" width="72" height="2.5" rx="1.2" fill="var(--color-error)" />
			</svg>
			<span class="font-semibold">Scan barcode</span>
			<span class="text-base-content/60 text-xs leading-snug">
				{liveOk ? 'Hold the camera over the barcode. It reads by itself, no picture taken.' : 'Not available on this device or page. Use Photo.'}
			</span>
		</button>

		<!-- Photo: take (or choose) a picture; it's read afterwards -->
		<button
			type="button"
			class="rounded-box border-base-300 hover:bg-base-200 flex flex-col items-center gap-1 border p-3 text-center disabled:opacity-60"
			disabled={busy || reading}
			onclick={() => photo?.click()}
		>
			<svg viewBox="0 0 96 64" class="text-base-content w-full max-w-28" aria-hidden="true">
				<rect x="1" y="1" width="94" height="62" rx="6" fill="currentColor" fill-opacity="0.06" />
				<rect x="16" y="9" width="64" height="46" rx="3" fill="var(--color-base-100)" stroke="currentColor" stroke-opacity="0.4" />
				<g fill="currentColor" fill-opacity="0.75">
					{#each BARS.slice(0, 14) as w, i (i)}<rect x={24 + BARS.slice(0, 14).slice(0, i).reduce((n, v) => n + v + 0.9, 0) * 1.0} y="18" width={w * 1.0} height="20" />{/each}
				</g>
				<rect x="26" y="44" width="40" height="3" rx="1.5" fill="currentColor" fill-opacity="0.25" />
				<!-- shutter button -->
				<circle cx="76" cy="50" r="11" fill="var(--color-primary)" />
				<circle cx="76" cy="50" r="6" fill="none" stroke="var(--color-primary-content)" stroke-width="2" />
			</svg>
			<span class="font-semibold">{reading ? 'Reading…' : 'Photo'}</span>
			<span class="text-base-content/60 text-xs leading-snug">Take a picture of the barcode, or pick one from your gallery. Works on every device.</span>
		</button>
	</div>

	{#if message && !error}
		<div role="alert" class="alert alert-error mt-2 text-sm font-semibold shadow-md"><span>{message}</span></div>
	{/if}

	{#if error}
		<div role="alert" class="alert alert-warning mt-2 text-sm font-semibold shadow-md">
			<span>{error}</span>
			{#if liveFailed}<button type="button" class="btn btn-sm" onclick={() => photo?.click()}>Use Photo instead</button>{/if}
		</div>
	{/if}

	<p class="text-base-content/70 mt-3 text-sm">Or type the digits printed under the bars:</p>
	<div class="mt-1 flex gap-2">
		<input
			class="input w-full min-w-0 font-mono"
			inputmode="numeric"
			autocomplete="off"
			aria-label="Barcode digits"
			placeholder="e.g. 5449000000996"
			bind:value={code}
			onkeydown={(e) => e.key === 'Enter' && (e.preventDefault(), submit())}
		/>
		<button type="button" class="btn btn-secondary" disabled={!valid || busy} onclick={submit}>{label}</button>
	</div>
	{#if code && !valid}<p class="text-base-content/60 mt-1 text-xs">Barcodes are 8, 12 or 13 digits.</p>{/if}
</div>

{#if scanning}
	<div class="fixed inset-0 z-50 flex flex-col items-center justify-center gap-4 bg-black/80 p-4" role="dialog" aria-label="Scan barcode">
		<!-- svelte-ignore a11y_media_has_caption -->
		<video bind:this={video} class="rounded-box max-h-[60vh] w-full max-w-md bg-black" playsinline muted></video>
		<p class="text-sm text-white">Point the camera at the barcode</p>
		<button type="button" class="btn" onclick={stop}>Cancel</button>
	</div>
{/if}
