// The page's handle on the qrgen worker: call(method, request, blobs) sends one
// operation of web/internal/bridge and resolves with its result, or rejects
// with the library's error message.

let worker;
let nextId = 1;
const pending = new Map();

/** Starts the worker; resolves once the WebAssembly module is running. */
export function start() {
  worker = new Worker('worker.js');
  const ready = new Promise((resolve, reject) => {
    worker.addEventListener('error', (e) => reject(new Error(e.message || 'the worker failed to start')));
    worker.addEventListener('message', function onReady(e) {
      if (!('ready' in e.data)) return;
      worker.removeEventListener('message', onReady);
      if (e.data.ready) resolve();
      else reject(new Error(e.data.error));
    });
  });
  worker.addEventListener('message', (e) => {
    const { id, response } = e.data;
    const p = pending.get(id);
    if (!p) return;
    pending.delete(id);
    let parsed;
    try {
      parsed = JSON.parse(response);
    } catch {
      p.reject(new Error('unreadable response from the worker'));
      return;
    }
    if (parsed.error) p.reject(new Error(parsed.error));
    else p.resolve(parsed.result);
  });
  return ready;
}

/**
 * Runs one bridge operation. blobs are Uint8Arrays (image files or pixels);
 * they are copied to the worker, so the caller may keep using them.
 */
export function call(method, request = {}, blobs = []) {
  const id = nextId++;
  return new Promise((resolve, reject) => {
    pending.set(id, { resolve, reject });
    worker.postMessage({ id, method, request, blobs });
  });
}
