/**
 * Shrink a photo before upload: phone pictures are 3–12 MB, and vision models
 * don't benefit beyond ~1600px. Also normalises HEIC/odd formats to JPEG where
 * the browser can decode them, and drops EXIF (including GPS location).
 */
export async function shrinkImage(file: File, maxSide = 1600, quality = 0.82): Promise<Blob> {
	const bitmap = await createImageBitmap(file, { imageOrientation: 'from-image' });
	const scale = Math.min(1, maxSide / Math.max(bitmap.width, bitmap.height));
	const w = Math.round(bitmap.width * scale);
	const h = Math.round(bitmap.height * scale);
	const canvas = new OffscreenCanvas(w, h);
	canvas.getContext('2d')!.drawImage(bitmap, 0, 0, w, h);
	bitmap.close();
	return canvas.convertToBlob({ type: 'image/jpeg', quality });
}
