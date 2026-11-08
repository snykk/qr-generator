// Entry point: starts the qrgen worker, switches between the Create and Scan
// views, and initialises both.

import { start } from './client.js';
import { $, $$ } from './util.js';
import { initCreate } from './create.js';
import { initScan, leaveScan } from './scan.js';

let current = 'create';

function selectView(name) {
  if (name === current) return;
  if (current === 'scan') leaveScan();
  current = name;
  for (const tab of $$('.views [data-view]')) tab.setAttribute('aria-selected', String(tab.dataset.view === name));
  for (const view of $$('.view')) view.hidden = view.id !== `view-${name}`;
  history.replaceState(null, '', name === 'create' ? location.pathname : `#${name}`);
}

for (const tab of $$('.views [data-view]')) tab.addEventListener('click', () => selectView(tab.dataset.view));

const engine = $('#engine');
start().then(
  () => {
    engine.dataset.state = 'ready';
    engine.textContent = 'Go engine ready';
    engine.title = 'The qrgen library, compiled to WebAssembly, runs in a Web Worker.';
  },
  (err) => {
    engine.dataset.state = 'error';
    engine.textContent = 'Engine failed';
    engine.title = err.message;
    $('#error').hidden = false;
    $('#error').textContent = `The WebAssembly engine did not start: ${err.message}. Build it with web/build.sh and serve web/site over HTTP.`;
  },
);

initCreate();
initScan(() => current === 'scan');
if (location.hash === '#scan') selectView('scan');
