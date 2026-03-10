export type FlacMetadata = {
  title?: string;
  artist?: string;
  album?: string;
  albumArtist?: string;
  year?: number;
  trackNumber?: number;
  trackTotal?: number;
  discNumber?: number;
  lyrics?: string;
  coverData?: ArrayBuffer;
  coverMime?: string;
};

type FlacBlock = {
  type: number;
  data: Uint8Array;
};

const FLAC_SIGNATURE = new Uint8Array([0x66, 0x4c, 0x61, 0x43]); // fLaC

export function writeFlacMetadata(input: ArrayBuffer, metadata: FlacMetadata): ArrayBuffer {
  const source = new Uint8Array(input);
  if (!isFlac(source)) {
    throw new Error("not a valid FLAC file");
  }

  const { blocks, audioStart } = parseBlocks(source);
  const nextBlocks: FlacBlock[] = blocks.filter((item) => item.type !== 4 && item.type !== 6);
  nextBlocks.push({ type: 4, data: buildVorbisComment(metadata) });

  if (metadata.coverData && metadata.coverData.byteLength > 0) {
    nextBlocks.push({
      type: 6,
      data: buildPictureBlock(new Uint8Array(metadata.coverData), metadata.coverMime),
    });
  }

  return rebuildFlac(source, nextBlocks, audioStart);
}

function isFlac(data: Uint8Array): boolean {
  return (
    data.length >= 4 &&
    data[0] === FLAC_SIGNATURE[0] &&
    data[1] === FLAC_SIGNATURE[1] &&
    data[2] === FLAC_SIGNATURE[2] &&
    data[3] === FLAC_SIGNATURE[3]
  );
}

function parseBlocks(data: Uint8Array): { blocks: FlacBlock[]; audioStart: number } {
  const blocks: FlacBlock[] = [];
  let offset = 4;

  while (offset + 4 <= data.length) {
    const blockHeader = data[offset];
    const isLast = (blockHeader & 0x80) !== 0;
    const blockType = blockHeader & 0x7f;
    const blockSize = (data[offset + 1] << 16) | (data[offset + 2] << 8) | data[offset + 3];
    const blockDataStart = offset + 4;
    const blockDataEnd = blockDataStart + blockSize;

    if (blockDataEnd > data.length) {
      throw new Error("invalid FLAC metadata block size");
    }

    blocks.push({
      type: blockType,
      data: data.slice(blockDataStart, blockDataEnd),
    });

    offset = blockDataEnd;
    if (isLast) {
      return { blocks, audioStart: offset };
    }
  }

  throw new Error("invalid FLAC metadata layout");
}

function rebuildFlac(source: Uint8Array, blocks: FlacBlock[], audioStart: number): ArrayBuffer {
  if (blocks.length === 0) {
    throw new Error("cannot write FLAC without metadata blocks");
  }

  let metadataSize = 0;
  for (const block of blocks) {
    metadataSize += 4 + block.data.length;
  }

  const audioPayload = source.slice(audioStart);
  const result = new Uint8Array(4 + metadataSize + audioPayload.length);
  let offset = 0;

  result.set(FLAC_SIGNATURE, offset);
  offset += 4;

  for (let i = 0; i < blocks.length; i++) {
    const block = blocks[i];
    if (block.data.length > 0x00ffffff) {
      throw new Error("FLAC metadata block too large");
    }

    const isLast = i === blocks.length - 1;
    result[offset] = (isLast ? 0x80 : 0x00) | (block.type & 0x7f);
    result[offset + 1] = (block.data.length >> 16) & 0xff;
    result[offset + 2] = (block.data.length >> 8) & 0xff;
    result[offset + 3] = block.data.length & 0xff;
    offset += 4;

    result.set(block.data, offset);
    offset += block.data.length;
  }

  result.set(audioPayload, offset);
  return result.buffer.slice(result.byteOffset, result.byteOffset + result.byteLength);
}

function buildVorbisComment(metadata: FlacMetadata): Uint8Array {
  const encoder = new TextEncoder();
  const vendorBytes = encoder.encode("codex-browser-metadata-writer");
  const comments: Uint8Array[] = [];
  const yearText = metadata.year && metadata.year > 0 ? String(metadata.year) : undefined;
  const trackText = metadata.trackNumber && metadata.trackNumber > 0 ? String(metadata.trackNumber) : undefined;
  const trackTotalText = metadata.trackTotal && metadata.trackTotal > 0 ? String(metadata.trackTotal) : undefined;
  const discText = metadata.discNumber && metadata.discNumber > 0 ? String(metadata.discNumber) : undefined;

  const pairs: Array<[string, string | undefined]> = [
    ["TITLE", metadata.title],
    ["ARTIST", metadata.artist],
    ["ALBUM", metadata.album],
    ["ALBUMARTIST", metadata.albumArtist],
    ["DATE", yearText],
    ["TRACKNUMBER", trackText],
    ["TRACKTOTAL", trackTotalText],
    ["DISCNUMBER", discText],
    ["LYRICS", metadata.lyrics],
  ];

  for (const [key, value] of pairs) {
    const normalized = (value ?? "").trim();
    if (!normalized) {
      continue;
    }
    comments.push(encoder.encode(`${key}=${normalized}`));
  }

  let totalSize = 4 + vendorBytes.length + 4;
  for (const item of comments) {
    totalSize += 4 + item.length;
  }

  const out = new Uint8Array(totalSize);
  let offset = 0;
  writeUInt32LE(out, offset, vendorBytes.length);
  offset += 4;
  out.set(vendorBytes, offset);
  offset += vendorBytes.length;

  writeUInt32LE(out, offset, comments.length);
  offset += 4;

  for (const item of comments) {
    writeUInt32LE(out, offset, item.length);
    offset += 4;
    out.set(item, offset);
    offset += item.length;
  }

  return out;
}

function buildPictureBlock(coverData: Uint8Array, coverMime?: string): Uint8Array {
  const encoder = new TextEncoder();
  const mime = encoder.encode(coverMime || "image/jpeg");
  const description = encoder.encode("Cover");
  const totalSize = 32 + mime.length + description.length + coverData.length;
  const out = new Uint8Array(totalSize);
  const view = new DataView(out.buffer);
  let offset = 0;

  // Picture type: 3 (front cover)
  view.setUint32(offset, 3, false);
  offset += 4;

  view.setUint32(offset, mime.length, false);
  offset += 4;
  out.set(mime, offset);
  offset += mime.length;

  view.setUint32(offset, description.length, false);
  offset += 4;
  out.set(description, offset);
  offset += description.length;

  // width/height/depth/colors. Unknown width/height is allowed as 0.
  view.setUint32(offset, 0, false);
  offset += 4;
  view.setUint32(offset, 0, false);
  offset += 4;
  view.setUint32(offset, 24, false);
  offset += 4;
  view.setUint32(offset, 0, false);
  offset += 4;

  view.setUint32(offset, coverData.length, false);
  offset += 4;
  out.set(coverData, offset);

  return out;
}

function writeUInt32LE(target: Uint8Array, offset: number, value: number) {
  target[offset] = value & 0xff;
  target[offset + 1] = (value >> 8) & 0xff;
  target[offset + 2] = (value >> 16) & 0xff;
  target[offset + 3] = (value >> 24) & 0xff;
}
