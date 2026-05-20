// Mirrors theo-stacks/templates/node-worker. A worker process exposes
// no HTTP port — the test mode is justBuild + a structure check that
// confirms /app/src/worker.js is present.
console.log("worker booted");
// Simulate work then exit cleanly so E2E doesn't hang.
setTimeout(() => process.exit(0), 100);
