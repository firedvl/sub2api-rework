// Validate documentation and real serialized fixtures without adding a backend
// dependency. Uses the AJV version already installed with frontend ESLint.
const fs = require('node:fs');
const path = require('node:path');
const { execFileSync } = require('node:child_process');
const assert = require('node:assert/strict');
const root = path.resolve(__dirname, '..');
const eslint = require.resolve('eslint', { paths: [path.join(root, 'frontend')] });
const Ajv = require(require.resolve('ajv', { paths: [path.dirname(eslint)] }));
const ajv = new Ajv({ allErrors: true });
function compile(filename) {
  const schema = JSON.parse(fs.readFileSync(path.join(root, 'docs', filename), 'utf8'));
  // Current schemas use only draft-07-compatible validation keywords. AJV6
  // resolves $defs JSON pointers, but cannot load the 2020-12 meta-schema.
  delete schema.$schema;
  return ajv.compile(schema);
}
const capabilities = compile('integration-contract-v2.schema.json');
const preflight = compile('gateway-preflight-v2.schema.json');
function validate(value, validator) {
  assert(validator(value), JSON.stringify(validator.errors));
}
const docs = fs.readFileSync(path.join(root, 'docs/integration-contract-v2.md'), 'utf8');
let examples = 0;
for (const match of docs.matchAll(/```json\n([\s\S]*?)\n```/g)) {
  const value = JSON.parse(match[1]);
  if (value.schema_version === 2) {
    validate(value, value.models ? capabilities : preflight);
    examples++;
  } else if (value.features) {
    const schema = JSON.parse(fs.readFileSync(path.join(root, 'docs/integration-contract-v2.schema.json'), 'utf8'));
    validate(value, ajv.compile({ $ref: '#/$defs/capabilities', $defs: schema.$defs }));
    examples++;
  } else {
    throw new Error('Unclassified documentation example');
  }
}
const output = execFileSync('go', ['test', './internal/service', '-run', '^TestGatewayEffectivePreflightCasesArePassive$', '-count=1', '-v'], {
  cwd: path.join(root, 'backend'), encoding: 'utf8', maxBuffer: 8 * 1024 * 1024,
});
let responses = 0;
for (const line of output.split('\n')) {
  const match = line.match(/GATEWAY_SCHEMA_(CAPABILITIES|RESPONSE) (\{.*\})/);
  if (match) {
    validate(JSON.parse(match[2]), match[1] === 'CAPABILITIES' ? capabilities : preflight);
    responses++;
  }
}
assert.equal(responses, 4, 'Missing serialized fixture coverage');
assert.equal(examples, 4, 'Documentation example coverage changed');
console.log(`PASS: ${examples} documentation examples and ${responses} serialized service responses`);
