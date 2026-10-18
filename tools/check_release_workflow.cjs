// Use ESLint's existing YAML dependency; no additional test dependencies.
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { spawnSync } = require('node:child_process');
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
for (const [jobName, job] of Object.entries(release.jobs)) {
  for (const step of job.steps) {
    if (!step.run) continue;
    const syntax = spawnSync('bash', ['-n'], { input: step.run, encoding: 'utf8' });
    assert.equal(syntax.status, 0, `${jobName}/${step.name}: ${syntax.stderr}`);
  }
}
assert.equal(release.on.workflow_dispatch.inputs.manifest_only.default, false);
assert.equal(release.jobs.release.steps.find(s => s.name === 'Run GoReleaser').if,
  "${{ github.event.inputs.manifest_only != 'true' }}");
const manifestStep = release.jobs.release.steps.find(s => s.name === 'Create and validate immutable release manifest');
assert.equal(manifestStep.if, undefined, 'manifest repair must retain image and contract validation');
assert(manifestStep.run.includes('org.opencontainers.image.revision'));
assert(manifestStep.run.includes('TestReleaseManifestFromEnvironment'));
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
