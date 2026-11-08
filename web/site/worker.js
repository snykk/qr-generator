// Hosts the qrgen WebAssembly module off the main thread, so that encoding a
// large symbol or scanning a camera frame never freezes the page. The Go side
// (web/wasm) registers one function, qrgenCall(method, requestJSON, ...blobs),
// that returns a JSON response; this worker relays calls to it in order.
/* global Go */
importScripts('wasm_exec.js');

const ready = (async () => {
  const go = new Go();
  let instance;
  try {
    ({ instance } = await WebAssembly.instantiateStreaming(fetch('qrgen.wasm'), go.importObject));
  } catch {
    // Some static servers send .wasm without the application/wasm type,
    // which instantiateStreaming refuses; compiling from bytes always works.
    const response = await fetch('qrgen.wasm');
    if (!response.ok) throw new Error(`qrgen.wasm: HTTP ${response.status}`);
    ({ instance } = await WebAssembly.instantiate(await response.arrayBuffer(), go.importObject));
  }
  // Go's main registers qrgenCall and then blocks, which is when run returns.
  go.run(instance);
  if (typeof self.qrgenCall !== 'function') throw new Error('the qrgen module did not start');
})();

ready.then(
  () => postMessage({ ready: true }),
  (err) => postMessage({ ready: false, error: String((err && err.message) || err) }),
);

self.onmessage = async (event) => {
  const { id, method, request, blobs } = event.data;
  let response;
  try {
    await ready;
    response = self.qrgenCall(method, JSON.stringify(request || {}), ...(blobs || []));
  } catch (err) {
    response = JSON.stringify({ error: String((err && err.message) || err) });
  }
  postMessage({ id, response });
};
