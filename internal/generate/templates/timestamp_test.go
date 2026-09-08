package templates

import (
	"os/exec"
	"strings"
	"testing"
)

func TestJSTimestampPrecision(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node unavailable")
	}
	content, err := FS.ReadFile("js_file.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	helpers := string(content)
	helpers = helpers[strings.Index(helpers, "function writeTimestamp(value, writer)"):]
	helpers = helpers[:strings.Index(helpers, "{{- end}}")]
	script := `const assert = require('node:assert/strict');
 const WIRE={VARINT:0};const tag=(field,wire)=>field*8+wire;
 const readInt64=(reader)=>reader.int64();
 function decode(seconds,nanos){const values=[8,seconds,16,nanos];const r={pos:0,len:4,uint32(){return values[this.pos++]},int64(){return values[this.pos++]},int32(){return values[this.pos++]}};return decodeTimestampMessage(r);}
 function encode(value){const fields=[];const w={uint32(v){fields.push(v);return this},int64(v){fields.push(v);return this},int32(v){fields.push(v);return this}};writeTimestamp(value,w);return fields;}
 ` + helpers + `
 for(const [seconds,nanos] of [[1700000000,123456789],[-1,999999999],[0,1]]){
 const value=decode(seconds,nanos);
 assert.equal(value.epochNanoseconds,BigInt(seconds)*1000000000n+BigInt(nanos));
 assert.deepEqual(encode(value),[8,seconds,16,nanos]);
 assert.equal(JSON.stringify(value),JSON.stringify(new Date(value.getTime())));
 value.setTime(2000);assert.deepEqual(encode(value),[8,2]);
 }
 assert.deepEqual(encode(new Date(1234)),[8,1,16,234000000]);
 `
	output, err := exec.Command(node, "-e", script).CombinedOutput()
	if err != nil {
		t.Fatalf("timestamp runtime: %v\n%s", err, output)
	}
}
