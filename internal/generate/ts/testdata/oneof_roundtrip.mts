import assert from 'node:assert/strict';
import { encodeCertSource, decodeCertSource, encodeScalarChoice, decodeScalarChoice, encodeCoverage, decodeCoverage } from './model.ts';
import type { CertSource, Coverage, ScalarChoice, ScalarChoiceValueOneof } from './model.ts';

const ab = (u8: Uint8Array): ArrayBuffer => u8.buffer.slice(u8.byteOffset, u8.byteOffset + u8.byteLength) as ArrayBuffer;

const cert: CertSource = { value: { acme: { email: 'ops@example.com', domains: ['a', 'b'] } } };
assert.deepEqual(decodeCertSource(ab(encodeCertSource(cert))), cert);
assert.throws(() => encodeCertSource({ value: {} }), /CertSource.value: expected exactly one alternative, got 0/);
assert.throws(() => encodeCertSource({ value: { acme: { email: '', domains: [] }, secret: { name: '', secret: new Uint8Array(0) } } }), /got 2/);
assert.deepEqual(decodeCertSource(new ArrayBuffer(0)), { value: {} });

const alternatives: ScalarChoiceValueOneof[] = [
  { flag: true }, { flag: false },
  { text: 't' }, { text: '' },
  { data: new Uint8Array([1]) }, { data: new Uint8Array(0) },
  { status: 1 }, { status: 0 },
  { updated: new Date(1700000000000) },
  { delay: 2 }, { delay: 0 },
  { uuid: new Uint8Array(16).fill(3) },
];
for (const alt of alternatives) {
  const m: ScalarChoice = { value: alt };
  const back = decodeScalarChoice(ab(encodeScalarChoice(m)));
  assert.deepEqual(Object.keys(back.value!), Object.keys(alt));
  const key = Object.keys(alt)[0] as keyof ScalarChoiceValueOneof;
  const expected = alt[key];
  if (expected instanceof Date) {
    assert.equal((back.value![key] as Date).getTime(), expected.getTime());
  } else {
    assert.deepEqual(back.value![key], expected);
  }
}
assert.deepEqual(decodeScalarChoice(ab(encodeScalarChoice({}))), { value: undefined });
assert.throws(() => encodeScalarChoice({ value: { flag: true, text: 'x' } }), /ScalarChoice.value: expected at most one alternative, got 2/);

const first = encodeScalarChoice({ value: { text: 'first' } });
const second = encodeScalarChoice({ value: { status: 1 } });
const joined = new Uint8Array(first.length + second.length);
joined.set(first);
joined.set(second, first.length);
assert.deepEqual(decodeScalarChoice(ab(joined)), { value: { status: 1 } });

const coverage: Coverage = { items: [], byName: {}, choice: { number: 0 } };
assert.deepEqual(decodeCoverage(ab(encodeCoverage(coverage))).choice, { number: 0 });
console.log('ok');
