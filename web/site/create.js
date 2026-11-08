// "Create": the content and symbol settings, the live preview, downloads,
// structured append, and the "Inside the symbol" tab fed by the same settings.

import { call } from './client.js';
import { $, $$, el, debounce, downloadBlob, base64ToBytes, copyText, flash } from './util.js';
import { initInspect, showInspection, showInspectError } from './inspect.js';

const RMQR_WIDTHS = {
  7: [43, 59, 77, 99, 139],
  9: [43, 59, 77, 99, 139],
  11: [27, 43, 59, 77, 99, 139],
  13: [27, 43, 59, 77, 99, 139],
  15: [43, 59, 77, 99, 139],
  17: [43, 59, 77, 99, 139],
};

const SYMBOLOGY = {
  qr: {
    hint: 'ISO/IEC 18004. Square symbols from version 1 (21 × 21) to version 40 (177 × 177), with four error-correction levels.',
    ec: [['L', 'L, ~7% recovery'], ['M', 'M, ~15% recovery'], ['Q', 'Q, ~25% recovery'], ['H', 'H, ~30% recovery']],
    ecDefault: 'M',
    versions: () => [['0', 'Automatic'], ...Array.from({ length: 40 }, (_, i) => [String(i + 1), `${i + 1} (${21 + 4 * i} × ${21 + 4 * i})`])],
    masks: 8,
    quietZone: 4,
  },
  micro: {
    hint: 'Micro QR: one finder pattern and 11 to 17 modules, for very short data. M1 holds digits only and detects errors without correcting them.',
    ec: [['', 'Automatic'], ['L', 'L'], ['M', 'M'], ['Q', 'Q (M4 only)']],
    ecDefault: '',
    versions: () => [['', 'Automatic'], ['M1', 'M1 (11 × 11)'], ['M2', 'M2 (13 × 13)'], ['M3', 'M3 (15 × 15)'], ['M4', 'M4 (17 × 17)']],
    masks: 4,
    quietZone: 2,
  },
  rmqr: {
    hint: 'rMQR (ISO/IEC 23941): 32 rectangular sizes, 7 to 17 modules tall and 27 to 139 wide, for long narrow spaces. Levels M and H, one fixed mask.',
    ec: [['', 'Automatic'], ['M', 'M, ~15% recovery'], ['H', 'H, ~30% recovery']],
    ecDefault: '',
    versions: (height) => [['', 'Automatic'], ...Object.entries(RMQR_WIDTHS)
      .filter(([h]) => !height || Number(h) === height)
      .flatMap(([h, widths]) => widths.map((w) => [`R${h}x${w}`, `R${h}x${w} (${h} × ${w})`]))],
    masks: 0,
    quietZone: 2,
  },
};

const state = {
  logo: null, // Uint8Array of the logo file, or null
  text: '',
  svgURL: '',
  terminal: '',
  info: null,
  out: 'symbol',
  inspectStale: true,
  seq: 0,
};

const symbology = () => $('input[name=symbology]:checked').value;
const kind = () => $('#content-kind').value;
const appendOn = () => symbology() === 'qr' && $('#append-on').checked;

// Replaces a select's options, keeping its value when it is still offered
// (and keep is set), else selecting fallback.
function fillSelect(select, options, fallback, keep = true) {
  const prev = select.value;
  select.replaceChildren(...options.map(([value, label]) => el('option', { value, text: label })));
  select.value = keep && options.some(([v]) => v === prev) ? prev : fallback;
}

/**
 * Adjusts the controls to the chosen symbology. A new symbology starts from
 * its own defaults: QR's level M, say, would otherwise carry over to Micro QR
 * as an explicit choice and rule out M1.
 */
function configureSymbology(fresh = false) {
  const s = symbology();
  const conf = SYMBOLOGY[s];
  $('#symbology-hint').textContent = conf.hint;
  if (fresh) $('#rmqr-height').value = '0';
  fillSelect($('#ec-level'), conf.ec, conf.ecDefault, !fresh);
  const height = s === 'rmqr' ? Number($('#rmqr-height').value) || 0 : 0;
  fillSelect($('#version'), conf.versions(height), conf.versions(height)[0][0], !fresh);
  $('#height-field').hidden = s !== 'rmqr';
  $('#mask-field').hidden = conf.masks === 0;
  fillSelect($('#mask'), [['-1', 'Automatic'], ...Array.from({ length: conf.masks }, (_, k) => [String(k), String(k)])], '-1', !fresh);
  $('#eci-field').hidden = s === 'micro';
  $('#logo-group').hidden = s !== 'qr';
  $('#append-group').hidden = s !== 'qr';
  $('#quiet-zone').placeholder = `default (${conf.quietZone})`;
}

function settings() {
  const s = symbology();
  return {
    symbology: s,
    ecLevel: $('#ec-level').value,
    version: s === 'qr' ? Number($('#version').value) : 0,
    microVersion: s === 'micro' ? $('#version').value : '',
    rmqrVersion: s === 'rmqr' ? $('#version').value : '',
    rmqrHeight: s === 'rmqr' ? Number($('#rmqr-height').value) : 0,
    mask: s === 'rmqr' ? -1 : Number($('#mask').value),
    eci: s === 'micro' ? '' : $('#eci').value,
    quietZone: $('#quiet-zone').value === '' ? -1 : Number($('#quiet-zone').value),
    moduleSize: Number($('#module-size').value) || 0,
    foreground: $('#foreground').value,
    background: $('#background').value,
    logo: s === 'qr' && state.logo !== null,
    logoScale: Number($('#logo-scale').value),
  };
}

const blobs = () => (symbology() === 'qr' && state.logo ? [state.logo] : []);

/** The text to encode: typed, or built by a payload builder. */
async function resolveText() {
  const k = kind();
  $('#payload-preview').hidden = k === 'text';
  if (k === 'text') return $('#text').value;
  const req = { kind: k };
  for (const input of $$(`.kind[data-kind="${k}"] [data-payload]`)) {
    const name = input.dataset.payload;
    if (input.type === 'checkbox') req[name] = input.checked;
    else if (input.type === 'number') req[name] = Number(input.value);
    else if (name === 'phones' || name === 'emails') req[name] = [input.value];
    else req[name] = input.value;
  }
  const { text } = await call('payload', req);
  $('#payload-text').textContent = text;
  return text;
}

function showError(message) {
  $('#error').textContent = message;
  $('#error').hidden = !message;
  $('#preview').classList.toggle('stale', Boolean(message));
}

function setBusyButtons(disabled) {
  for (const id of ['#download-svg', '#copy-terminal', '#open-inspect']) $(id).disabled = disabled;
}

function statusLine(info) {
  const parts = [];
  const name = info.symbology === 'QR Code' ? `QR Code version ${info.version}` : `${info.symbology} ${info.version}`;
  parts.push(el('strong', { text: name }), ` (${info.cols} × ${info.rows} modules), `);
  parts.push(info.detectionOnly ? 'error detection only, ' : `level ${info.ecLevel}, `);
  parts.push(info.fixedMask ? 'fixed mask' : `mask ${info.mask}`);
  parts.push(`. ${info.dataBits} of ${info.capacityBits} data bits used.`);
  return parts;
}

const update = debounce(async () => {
  const seq = ++state.seq;
  try {
    const text = await resolveText();
    if (seq !== state.seq) return;
    state.text = text;
    if (appendOn()) {
      await renderAppend(seq);
      return;
    }
    const res = await call('encode', { text, settings: settings(), formats: ['svg', 'terminal'] }, blobs());
    if (seq !== state.seq) return;
    showError('');
    setBusyButtons(false);
    $('#preview').hidden = false;
    $('#append-preview').hidden = true;
    $('#download-png').textContent = 'Download PNG';
    state.info = res.info;
    state.terminal = res.terminal;
    if (state.svgURL) URL.revokeObjectURL(state.svgURL);
    state.svgURL = URL.createObjectURL(new Blob([res.svg], { type: 'image/svg+xml' }));
    $('#preview').replaceChildren(el('img', { src: state.svgURL, alt: `${res.info.symbology} encoding the text`, class: 'symbol' }));
    $('#status').replaceChildren(...statusLine(res.info));
    $('#terminal').textContent = res.terminal;
    state.inspectStale = true;
    if (state.out === 'inspect') refreshInspect();
  } catch (err) {
    if (seq !== state.seq) return;
    showError(err.message);
    if (state.out === 'inspect') showInspectError(err.message);
  }
}, 120);

async function renderAppend(seq) {
  const parts = Number($('#append-parts').value) || 2;
  const res = await call('structuredAppend', { text: state.text, parts, settings: settings() }, []);
  if (seq !== state.seq) return;
  showError('');
  setBusyButtons(true);
  $('#preview').hidden = true;
  $('#append-preview').hidden = false;
  $('#download-png').textContent = `Download ${parts} PNGs`;
  state.appendPNGs = res.pngs;
  $('#append-preview').replaceChildren(...res.pngs.map((b64, i) => el('figure', {},
    el('img', { src: `data:image/png;base64,${b64}`, alt: `Symbol ${i + 1} of ${parts}` }),
    el('figcaption', { text: `Symbol ${i + 1} of ${parts}` }),
  )));
  $('#status').replaceChildren(el('strong', { text: `Structured append: ${parts} QR symbols` }),
    '. A reader that scans all of them, in any order, joins the text back together.');
  $('#terminal').textContent = '';
  if (state.out === 'inspect') showInspectError('Turn off structured append to look inside a single symbol.');
}

async function refreshInspect() {
  if (appendOn()) {
    showInspectError('Turn off structured append to look inside a single symbol.');
    return;
  }
  state.inspectStale = false;
  const seq = state.seq;
  const req = { text: state.text, settings: settings() };
  try {
    const [ins, masks] = await Promise.all([call('inspect', req, blobs()), call('masks', req, blobs())]);
    if (seq !== state.seq) return;
    showInspection(ins, masks, {
      maskForced: req.settings.mask !== -1,
      onPickMask: (k) => {
        $('#mask').value = String(k);
        update();
      },
    });
  } catch (err) {
    if (seq === state.seq) showInspectError(err.message);
  }
}

function fileName(ext) {
  const info = state.info;
  if (!info) return `qrgen.${ext}`;
  const v = info.symbology === 'QR Code' ? `qr-v${info.version}` : `${info.symbology === 'rMQR Code' ? 'rmqr' : 'micro'}-${info.version.toLowerCase()}`;
  return `qrgen-${v}.${ext}`;
}

async function downloadPNG() {
  if (appendOn()) {
    (state.appendPNGs || []).forEach((b64, i) => {
      downloadBlob(new Blob([base64ToBytes(b64)], { type: 'image/png' }), `qrgen-part-${i + 1}.png`);
    });
    return;
  }
  try {
    const res = await call('encode', { text: state.text, settings: settings(), formats: ['png'] }, blobs());
    downloadBlob(new Blob([base64ToBytes(res.png)], { type: 'image/png' }), fileName('png'));
  } catch (err) {
    showError(err.message);
  }
}

async function downloadSVG() {
  if (!state.svgURL) return;
  const svg = await (await fetch(state.svgURL)).blob();
  downloadBlob(svg, fileName('svg'));
}

function selectOutput(out) {
  state.out = out;
  for (const tab of $$('.subtabs [data-out]')) tab.setAttribute('aria-selected', String(tab.dataset.out === out));
  $('#out-symbol').hidden = out !== 'symbol';
  $('#out-inspect').hidden = out !== 'inspect';
  if (out === 'inspect' && state.inspectStale) refreshInspect();
}

async function loadLogo(file) {
  if (!file) return;
  state.logo = new Uint8Array(await file.arrayBuffer());
  $('#logo-controls').hidden = false;
  update();
}

export function initCreate() {
  initInspect();
  configureSymbology();

  $('#content-kind').addEventListener('change', () => {
    for (const panel of $$('.kind')) panel.hidden = panel.dataset.kind !== kind();
    update();
  });
  for (const input of $$('input[name=symbology]')) {
    input.addEventListener('change', () => {
      configureSymbology(true);
      update();
    });
  }
  $('#rmqr-height').replaceChildren(...[['0', 'Any'], ...Object.keys(RMQR_WIDTHS).map((h) => [h, `${h} modules`])]
    .map(([value, label]) => el('option', { value, text: label })));
  $('#rmqr-height').addEventListener('change', () => {
    configureSymbology();
    update();
  });
  for (const button of $$('#examples [data-example]')) {
    button.addEventListener('click', () => {
      $('#text').value = button.dataset.example;
      update();
    });
  }
  $('#settings').addEventListener('input', (e) => {
    if (e.target.id === 'logo-file') return;
    if (e.target.id === 'logo-scale') $('#logo-scale-out').textContent = `${Math.round(e.target.value * 100)}%`;
    if (e.target.id === 'append-on') $('#append-parts-field').hidden = !e.target.checked;
    update();
  });
  $('#settings').addEventListener('submit', (e) => e.preventDefault());
  $('#logo-file').addEventListener('change', (e) => loadLogo(e.target.files[0]));
  $('#logo-clear').addEventListener('click', () => {
    state.logo = null;
    $('#logo-file').value = '';
    $('#logo-controls').hidden = true;
    update();
  });

  for (const tab of $$('.subtabs [data-out]')) tab.addEventListener('click', () => selectOutput(tab.dataset.out));
  $('#open-inspect').addEventListener('click', () => selectOutput('inspect'));
  $('#download-png').addEventListener('click', downloadPNG);
  $('#download-svg').addEventListener('click', downloadSVG);
  $('#copy-terminal').addEventListener('click', async (e) => {
    if (await copyText(state.terminal)) flash(e.target, 'Copied');
  });

  update();
}
