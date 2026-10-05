import assert from 'node:assert/strict';
import { encodeWide, decodeWide } from './model.ts';
import type { Wide } from './model.ts';

const ab = (u8: Uint8Array): ArrayBuffer => u8.buffer.slice(u8.byteOffset, u8.byteOffset + u8.byteLength) as ArrayBuffer;
const message: Wide = { num: Number.MAX_SAFE_INTEGER, big: 18446744073709551615n, nums: [1, 2 ** 40], fixed: 123456, signed: -77, plain: 5n, sfixedBig: -9n, bigs: [1n, 2n] };
assert.deepEqual(decodeWide(ab(encodeWide(message))), message);
assert.throws(() => encodeWide({ ...message, num: Number.MAX_SAFE_INTEGER + 1 }), /outside the safe integer range/);
assert.throws(() => encodeWide({ ...message, nums: [2 ** 60] }), /outside the safe integer range/);

const tooBig = new Uint8Array(encodeWide({ ...message, big: 2n ** 60n, num: 0 }));
tooBig[0] = (1 << 3) | 0;
assert.throws(() => decodeWide(tooBig.buffer), /uint64 value 1152921504606846976 is outside the safe integer range/);
console.log('ok');
