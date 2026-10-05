import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { encodeCertSource, decodeCertSource, encodeScalarChoice, decodeScalarChoice, encodeCoverage, decodeCoverage } from './js/model.js';

const ab = (u8) => u8.buffer.slice(u8.byteOffset, u8.byteOffset + u8.byteLength);
const roundTrip = (encode, decode, m) => decode(ab(encode(m)));
const fromHex = (h) => Uint8Array.from(Buffer.from(h, 'hex')).buffer;
const toHex = (u8) => Buffer.from(u8).toString('hex');

assert.throws(() => encodeCertSource({}), /CertSource.value: expected exactly one alternative, got none/);
assert.throws(() => encodeCertSource({ value: {} }), /CertSource.value: expected exactly one alternative, got 0/);
assert.throws(() => encodeCertSource({ value: { acme: { email: 'a', domains: [] }, secret: { name: 'n', secret: new Uint8Array(0) } } }), /got 2/);
assert.deepEqual(roundTrip(encodeCertSource, decodeCertSource, { value: { acme: { email: 'ops@example.com', domains: ['a', 'b'] } } }), { value: { acme: { email: 'ops@example.com', domains: ['a', 'b'] } } });
assert.deepEqual(decodeCertSource(new ArrayBuffer(0)), { value: {} });

assert.deepEqual(roundTrip(encodeScalarChoice, decodeScalarChoice, {}), { value: undefined });
assert.deepEqual(roundTrip(encodeScalarChoice, decodeScalarChoice, { value: undefined }), { value: undefined });
const alternatives = [
  { flag: true }, { flag: false },
  { text: 'hi' }, { text: '' },
  { data: new Uint8Array([1, 2]) }, { data: new Uint8Array(0) },
  { status: 1 }, { status: 0 },
  { updated: new Date(1700000000123) },
  { delay: 1500.5 }, { delay: 0 },
  { uuid: new Uint8Array(16).fill(7) },
];
for (const alt of alternatives) {
  const back = roundTrip(encodeScalarChoice, decodeScalarChoice, { value: alt });
  assert.deepEqual(Object.keys(back.value), Object.keys(alt), JSON.stringify(alt));
  const key = Object.keys(alt)[0];
  if (alt[key] instanceof Date) {
    assert.equal(back.value[key].getTime(), alt[key].getTime());
  } else {
    assert.deepEqual(back.value[key], alt[key]);
  }
}
assert.throws(() => encodeScalarChoice({ value: { flag: true, text: 'x' } }), /ScalarChoice.value: expected at most one alternative, got 2/);

const first = encodeScalarChoice({ value: { text: 'first' } });
const second = encodeScalarChoice({ value: { status: 1 } });
const joined = new Uint8Array(first.length + second.length);
joined.set(first);
joined.set(second, first.length);
assert.deepEqual(decodeScalarChoice(ab(joined)), { value: { status: 1 } });

const coverage = roundTrip(encodeCoverage, decodeCoverage, { items: [], byName: {}, choice: { secret: { name: 's', secret: new Uint8Array([9]) } } });
assert.deepEqual(coverage.choice, { secret: { name: 's', secret: new Uint8Array([9]) } });
assert.deepEqual(roundTrip(encodeCoverage, decodeCoverage, { choice: { number: 0 } }).choice, { number: 0 });

const wire = JSON.parse(readFileSync(process.argv[2], 'utf8'));
const goCases = {
  cert_acme: [decodeCertSource, encodeCertSource, { value: { acme: { email: 'ops@example.com', domains: ['a', 'b'] } } }],
  cert_secret: [decodeCertSource, encodeCertSource, { value: { secret: { name: 'n', secret: new Uint8Array([1, 2, 3]) } } }],
  scalar_flag: [decodeScalarChoice, encodeScalarChoice, { value: { flag: true } }],
  scalar_text: [decodeScalarChoice, encodeScalarChoice, { value: { text: 'hi' } }],
  scalar_data: [decodeScalarChoice, encodeScalarChoice, { value: { data: new Uint8Array(0) } }],
  scalar_status: [decodeScalarChoice, encodeScalarChoice, { value: { status: 1 } }],
  scalar_updated: [decodeScalarChoice, encodeScalarChoice, { value: { updated: new Date(1700000000123) } }],
  scalar_delay: [decodeScalarChoice, encodeScalarChoice, { value: { delay: 1500 } }],
  scalar_uuid: [decodeScalarChoice, encodeScalarChoice, { value: { uuid: new Uint8Array([1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16]) } }],
  scalar_none: [decodeScalarChoice, encodeScalarChoice, { value: undefined }],
};
for (const [name, [decode, encode, expected]] of Object.entries(goCases)) {
  assert.ok(name in wire, name);
  const got = decode(fromHex(wire[name]));
  assert.deepEqual(got, expected, name);
  assert.equal(toHex(encode(got)), wire[name], name + ' re-encode');
}
assert.deepEqual(decodeCoverage(fromHex(wire.coverage_number)).choice, { number: 0 });
console.log('ok');
