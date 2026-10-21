/**
 * Pure TypeScript EXIF Metadata Parser for FileX Public Album Gallery
 * Extracts Camera, Lens, Exposure (Aperture, Shutter Speed, ISO), Focal Length,
 * Date Taken, and Image Dimensions without external dependencies.
 */

export interface ExifMetadata {
  make?: string;
  model?: string;
  camera?: string;
  lens?: string;
  shutter_speed?: string;
  aperture?: string;
  iso?: number;
  focal_length?: string;
  taken_at?: string;
  width?: number;
  height?: number;
}

/**
 * Parses raw EXIF markers and TIFF tags from an ArrayBuffer / Uint8Array.
 */
export function parseExifFromBuffer(buffer: ArrayBuffer | Uint8Array): ExifMetadata | null {
  const data = buffer instanceof Uint8Array ? buffer : new Uint8Array(buffer);
  if (data.length < 14) return null;

  // Verify SOI marker 0xFFD8
  if (data[0] !== 0xFF || data[1] !== 0xD8) return null;

  let offset = 2;
  let exifOffset = -1;
  let sofWidth = 0;
  let sofHeight = 0;

  while (offset < data.length - 4) {
    if (data[offset] !== 0xFF) {
      offset++;
      continue;
    }
    const marker = data[offset + 1];

    // SOF0 (0xC0), SOF1 (0xC1), SOF2 (0xC2) markers contain true image dimensions
    if ((marker === 0xC0 || marker === 0xC1 || marker === 0xC2) && offset + 8 < data.length) {
      sofHeight = (data[offset + 5] << 8) | data[offset + 6];
      sofWidth = (data[offset + 7] << 8) | data[offset + 8];
    }

    // Stop scan at Start of Scan (0xDA)
    if (marker === 0xDA) break;

    const len = (data[offset + 2] << 8) | data[offset + 3];

    // APP1 marker 0xFFE1
    if (marker === 0xE1 && offset + 10 <= data.length) {
      if (
        data[offset + 4] === 0x45 && // E
        data[offset + 5] === 0x78 && // x
        data[offset + 6] === 0x69 && // i
        data[offset + 7] === 0x66 && // f
        data[offset + 8] === 0x00 &&
        data[offset + 9] === 0x00
      ) {
        exifOffset = offset + 10;
      }
    }
    offset += 2 + len;
  }

  const result: ExifMetadata = {};
  if (sofWidth > 0 && sofHeight > 0) {
    result.width = sofWidth;
    result.height = sofHeight;
  }

  if (exifOffset === -1 || exifOffset + 8 > data.length) {
    return Object.keys(result).length > 0 ? result : null;
  }

  const view = new DataView(data.buffer, data.byteOffset + exifOffset, data.length - exifOffset);
  const endianBytes = (data[exifOffset] << 8) | data[exifOffset + 1];
  const isLE = endianBytes === 0x4949; // 'II' = Little Endian, 'MM' = Big Endian
  if (endianBytes !== 0x4949 && endianBytes !== 0x4D4D) return result;

  const magic = view.getUint16(2, isLE);
  if (magic !== 0x002A) return result;

  const ifd0Offset = view.getUint32(4, isLE);

  function readString(off: number, count: number): string {
    let s = '';
    for (let i = 0; i < count; i++) {
      if (off + i >= view.byteLength) break;
      const c = view.getUint8(off + i);
      if (c === 0) break;
      s += String.fromCharCode(c);
    }
    return s.trim();
  }

  function parseIFD(ifdOff: number): Record<number, any> {
    const entries: Record<number, any> = {};
    if (ifdOff + 2 > view.byteLength) return entries;
    const count = view.getUint16(ifdOff, isLE);
    let p = ifdOff + 2;

    for (let i = 0; i < count; i++) {
      if (p + 12 > view.byteLength) break;
      const tag = view.getUint16(p, isLE);
      const type = view.getUint16(p + 2, isLE);
      const cnt = view.getUint32(p + 4, isLE);
      const valOff = view.getUint32(p + 8, isLE);
      p += 12;

      let val: any = null;
      if (type === 2) {
        // ASCII string
        const off = cnt <= 4 ? p - 4 : valOff;
        if (off + cnt <= view.byteLength) {
          val = readString(off, cnt);
        }
      } else if (type === 3) {
        // Short (2 bytes)
        const off = cnt <= 2 ? p - 4 : valOff;
        if (off + 2 <= view.byteLength) {
          val = view.getUint16(off, isLE);
        }
      } else if (type === 4) {
        // Long (4 bytes)
        val = valOff;
      } else if (type === 5 || type === 10) {
        // Rational (8 bytes: 2 x 32-bit int)
        if (valOff + 8 <= view.byteLength) {
          const num = view.getUint32(valOff, isLE);
          const den = view.getUint32(valOff + 4, isLE);
          val = { num, den, value: den !== 0 ? num / den : 0 };
        }
      }
      entries[tag] = val;
    }
    return entries;
  }

  const ifd0 = parseIFD(ifd0Offset);

  if (ifd0[0x010F]) result.make = String(ifd0[0x010F]);
  if (ifd0[0x0110]) result.model = String(ifd0[0x0110]);
  if (result.make && result.model) {
    result.camera = result.model.toLowerCase().includes(result.make.toLowerCase())
      ? result.model
      : `${result.make} ${result.model}`;
  } else if (result.model) {
    result.camera = result.model;
  }

  // Exif SubIFD (tag 0x8769)
  const exifIFDOff = ifd0[0x8769];
  if (typeof exifIFDOff === 'number') {
    const subIFD = parseIFD(exifIFDOff);

    // ExposureTime (0x829A)
    if (subIFD[0x829A]) {
      const r = subIFD[0x829A];
      if (r && r.value > 0) {
        result.shutter_speed = r.value < 1 ? `1/${Math.round(1 / r.value)}s` : `${r.value}s`;
      }
    }

    // FNumber (0x829D)
    if (subIFD[0x829D]) {
      const r = subIFD[0x829D];
      if (r && r.value > 0) {
        result.aperture = `f/${parseFloat(r.value.toFixed(1))}`;
      }
    }

    // ISO (0x8827 or 0x8833)
    const isoVal = subIFD[0x8827] !== undefined ? subIFD[0x8827] : subIFD[0x8833];
    if (isoVal !== undefined && isoVal !== null) {
      const n = typeof isoVal === 'number' ? isoVal : isoVal?.value;
      if (n) result.iso = Number(n);
    }

    // DateTimeOriginal (0x9003) e.g. "2026:09:01 12:28:16"
    if (subIFD[0x9003]) {
      const rawDate = String(subIFD[0x9003]);
      if (rawDate.length >= 19) {
        const parts = rawDate.split(' ');
        if (parts.length === 2) {
          result.taken_at = parts[0].replace(/:/g, '-') + 'T' + parts[1];
        }
      }
    }

    // FocalLength (0x920A)
    if (subIFD[0x920A]) {
      const r = subIFD[0x920A];
      if (r && r.value > 0) {
        result.focal_length = `${Math.round(r.value)}mm`;
      }
    }

    // LensModel (0xA434)
    if (subIFD[0xA434]) {
      result.lens = String(subIFD[0xA434]);
    }

    // Dimensions in Exif (0xA002, 0xA003)
    if (subIFD[0xA002]) {
      const w = typeof subIFD[0xA002] === 'number' ? subIFD[0xA002] : subIFD[0xA002]?.value;
      if (w) result.width = Number(w);
    }
    if (subIFD[0xA003]) {
      const h = typeof subIFD[0xA003] === 'number' ? subIFD[0xA003] : subIFD[0xA003]?.value;
      if (h) result.height = Number(h);
    }
  }

  return Object.keys(result).length > 0 ? result : null;
}

/**
 * Extracts EXIF directly from a Blob or File. Reads the first 128KB.
 */
export async function parseExifFromBlob(blob: Blob): Promise<ExifMetadata | null> {
  try {
    const slice = blob.slice(0, 131072);
    const buf = await slice.arrayBuffer();
    return parseExifFromBuffer(buf);
  } catch {
    return null;
  }
}

/**
 * Fetches EXIF header from a URL using streaming or Range header (first 128KB).
 */
export async function fetchAndParseExif(url: string): Promise<ExifMetadata | null> {
  try {
    const res = await fetch(url, {
      headers: { Range: 'bytes=0-131071' },
    });
    if (!res.ok && res.status !== 206) {
      return null;
    }

    // Prefer stream reader to download only the initial 128KB chunk without reading the whole file
    if (res.body && typeof res.body.getReader === 'function') {
      const reader = res.body.getReader();
      const chunks: Uint8Array[] = [];
      let totalBytes = 0;
      const maxBytes = 131072;

      try {
        while (totalBytes < maxBytes) {
          const { done, value } = await reader.read();
          if (done || !value) break;
          chunks.push(value);
          totalBytes += value.length;
        }
      } finally {
        try {
          await reader.cancel();
        } catch {
          // ignore stream cancellation error
        }
      }

      if (chunks.length > 0) {
        const combined = new Uint8Array(totalBytes);
        let pos = 0;
        for (const chunk of chunks) {
          combined.set(chunk, pos);
          pos += chunk.length;
        }
        return parseExifFromBuffer(combined.buffer);
      }
    }

    const buf = await res.arrayBuffer();
    return parseExifFromBuffer(buf);
  } catch (e) {
    console.warn('fetchAndParseExif failed for url:', url, e);
    return null;
  }
}
