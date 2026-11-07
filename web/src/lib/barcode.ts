import { api } from '#lib/api.ts';
import type { LabelEstimate, Product } from '#lib/types.ts';

export type BarcodeLookup =
	| { source: 'saved'; product: Product }
	| { source: 'openfoodfacts'; product: LabelEstimate };

/** Digits only, so pasted codes with spaces or dashes work. */
export const cleanBarcode = (s: string) => s.replace(/[\s-]/g, '');

/** Saved products first; Open Food Facts only if the admin enabled it in Settings. */
export const lookupBarcode = (code: string) => api<BarcodeLookup>(`/products/lookup/${encodeURIComponent(cleanBarcode(code))}`);

/** Live camera scanning needs the BarcodeDetector API (Chrome/Android). */
export const canScanBarcodes = () => typeof window !== 'undefined' && 'BarcodeDetector' in window && !!navigator.mediaDevices?.getUserMedia;
