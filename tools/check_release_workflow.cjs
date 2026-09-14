// Use ESLint's existing YAML dependency; no additional test dependencies.
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const root = path.resolve(__dirname, '..');
const eslint = require.resolve('eslint', { paths: [path.join(root, 'frontend')] });
const yaml = require(require.resolve('js-yaml', { paths: [path.dirname(eslint)] }));
const release = yaml.load(fs.readFileSync(path.join(root, '.github/workflows/release.yml'), 'utf8'));
const ci = yaml.load(fs.readFileSync(path.join(root, '.github/workflows/backend-ci.yml'), 'utf8'));
function check(workflow) {
  const jobs = workflow.jobs;
  const needs = job => [jobs[job].needs].flat();
  assert(needs('build-frontend').includes('qualify'));
  assert(needs('release-integrity').includes('build-frontend'));
  assert(needs('release-integrity').includes('qualify'));
  assert(needs('release').includes('release-integrity'));
  for (const name of ['qualify', 'build-frontend', 'release-integrity', 'release']) {
    assert.equal(jobs[name].if, undefined, `${name} must not bypass failed dependencies`);
  }
  assert(!JSON.stringify(jobs.qualify).includes('verify_release_integrity.sh'));
  const upload = jobs['build-frontend'].steps.find(s => s.uses?.startsWith('actions/upload-artifact@'));
  assert.equal(upload.with.path, 'backend/internal/web/dist/');
  for (const name of ['build-frontend', 'release-integrity', 'release']) {
    const checkout = jobs[name].steps.find(s => s.uses?.startsWith('actions/checkout@'));
    assert.equal(checkout.with.ref, '${{ needs.qualify.outputs.qualified_sha }}');
  }
  for (const name of ['release-integrity', 'release']) {
    const steps = jobs[name].steps;
    const download = steps.findIndex(s => s.uses?.startsWith('actions/download-artifact@'));
    assert(download >= 0);
    assert.equal(steps[download].with.name, upload.with.name);
    assert.equal(steps[download].with.path, upload.with.path);
    assert.equal(steps[download].with['run-id'], undefined);
    const gate = steps.findIndex(s => s.run?.includes('verify_release_integrity.sh') || s.uses?.startsWith('goreleaser/goreleaser-action@'));
    assert(gate > download);
  }
}
check(release);
// Prove the guard rejects both edges whose removal could recreate the incident.
for (const [job, dependency] of [['release-integrity', 'build-frontend'], ['release', 'release-integrity']]) {
  const broken = structuredClone(release);
  broken.jobs[job].needs = broken.jobs[job].needs.filter(n => n !== dependency);
  assert.throws(() => check(broken));
}
const steps = ci.jobs.frontend.steps;
const clean = steps.findIndex(s => s.run?.includes('test ! -e backend/internal/web/dist'));
const build = steps.findIndex(s => s.run?.includes('pnpm run build'));
const integrity = steps.findIndex(s => s.run?.includes('verify_release_integrity.sh'));
assert(clean >= 0 && build > clean && integrity > build);
assert(steps.some(s => s.run?.includes('check_release_workflow.cjs')));
console.log('PASS: release DAG, exact source/artifact, negative ordering guards, fresh-checkout CI');
