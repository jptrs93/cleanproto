import assert from 'node:assert/strict';
import { encodeWide, decodeWide } from './js/model.js';

const ab = (u8) => u8.buffer.slice(u8.byteOffset, u8.byteOffset + u8.byteLength);
const message = { num: Number.MAX_SAFE_INTEGER, big: 18446744073709551615n, nums: [1, 2 ** 40], fixed: 123456, signed: -77, plain: 2n ** 60n, sfixedBig: -9n, bigs: [1n, 2n], plains: [-(2n ** 62n), 7n], totals: { a: 2n ** 61n, b: -1n } };
assert.deepEqual(decodeWide(ab(encodeWide(message))), message);
assert.deepEqual(decodeWide(ab(encodeWide({}))), { num: 0, big: 0n, nums: [], fixed: 0, signed: 0, plain: 0n, sfixedBig: 0n, bigs: [], plains: [], totals: {} });
assert.throws(() => encodeWide({ num: Number.MAX_SAFE_INTEGER + 1 }), /outside the safe integer range/);
assert.throws(() => encodeWide({ num: Infinity }), /outside the safe integer range/);
assert.throws(() => encodeWide({ nums: [2 ** 60] }), /outside the safe integer range/);

const tooBig = new Uint8Array(encodeWide({ big: 2n ** 60n }));
tooBig[0] = (1 << 3) | 0;
assert.throws(() => decodeWide(tooBig.buffer), /uint64 value 1152921504606846976 is outside the safe integer range/);
console.log('ok');
