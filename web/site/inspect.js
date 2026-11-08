// "Inside the symbol": draws an Inspection (web/internal/bridge) as a module
// map with layers, the message's segments and bit budget, the codewords and
// blocks, and the mask candidates with their scores.

import { $, $$, el, hex } from './util.js';

const THEORY = 'https://github.com/snykk/qr-generator/blob/master/docs/theory/';

// Roles in qrgen.ModuleRole order, each with its map colour and an
// explanation. Data modules are coloured by codeword kind instead.
const ROLES = [
  { name: 'Data', color: '#2563eb', doc: '05-matrix-construction.md' },
  { name: 'Remainder bit', color: '#94a3b8', about: 'Left over after the last codeword; always light before masking.' },
  { name: 'Finder pattern', color: '#111827', about: 'Lets a reader find the symbol and its orientation.' },
  { name: 'Separator', color: '#a5b4c8', about: 'A light border between a finder and the data.' },
  { name: 'Timing pattern', color: '#d97706', about: 'Alternating modules that give the module grid spacing.' },
  { name: 'Alignment pattern', color: '#9333ea', about: 'Corrects for perspective and bending in larger symbols.' },
  { name: 'Format information', color: '#e11d48', about: 'Error-correction level and mask, protected by a BCH code.' },
  { name: 'Version information', color: '#0d9488', about: 'The version number, in QR versions 7 and up.' },
  { name: 'Dark module', color: '#b91c1c', about: 'A single module that is always dark.' },
  { name: 'Finder sub-pattern', color: '#4338ca', about: "rMQR's smaller finder in the opposite corner." },
  { name: 'Corner pattern', color: '#ea580c', about: "Marks the remaining corners of an rMQR symbol." },
];
const DATA_COLOR = '#2563eb';
const EC_COLOR = '#16a34a';
const MODE_COLORS = { Numeric: '#0284c7', Alphanumeric: '#7c3aed', Byte: '#d97706', Kanji: '#db2777', ECI: '#475569' };

// The eight QR mask conditions, row i and column j; Micro QR's masks 0..3 are
// QR's 1, 4, 6, and 7, and rMQR always uses QR's 4.
const QR_MASKS = [
  '(i + j) mod 2 = 0',
  'i mod 2 = 0',
  'j mod 3 = 0',
  '(i + j) mod 3 = 0',
  '(i/2 + j/3) mod 2 = 0',
  'ij mod 2 + ij mod 3 = 0',
  '(ij mod 2 + ij mod 3) mod 2 = 0',
  '((i + j) mod 2 + ij mod 3) mod 2 = 0',
];
const MICRO_TO_QR = [1, 4, 6, 7];

let view = null; // { ins, layer, colorRoles, highlight, cell, margin, cwKey }

/** Clears the view, showing message as an error. */
export function showInspectError(message) {
  $('#inspect-error').textContent = message;
  $('#inspect-error').hidden = !message;
  $('#inspect-body').hidden = Boolean(message);
}

/**
 * Renders an inspection. masks are the symbol under each candidate mask;
 * ctx.maskForced says whether the mask was chosen by the caller, and
 * ctx.onPickMask(k) forces mask k.
 */
export function showInspection(ins, masks, ctx) {
  showInspectError('');
  view = {
    ins,
    layer: $('input[name=layer]:checked').value,
    colorRoles: $('#color-roles').checked,
    highlight: -1,
    cwKey: new Map(),
  };
  ins.codewords.forEach((_, i) => {
    view.cwKey.set(`${ins.codewordBlock[i]}:${ins.codewordIsEC[i] ? 1 : 0}:${ins.codewordIndex[i]}`, i);
  });
  renderFacts(ins, ctx);
  drawMap();
  renderLegend(ins);
  renderMessage(ins);
  renderBlocks(ins);
  renderMasks(ins, masks, ctx);
}

/** Wires the controls that only redraw; called once. */
export function initInspect() {
  for (const input of $$('input[name=layer]')) {
    input.addEventListener('change', () => {
      if (!view) return;
      view.layer = input.value;
      drawMap();
      renderLayerHint();
    });
  }
  $('#color-roles').addEventListener('change', (e) => {
    if (!view) return;
    view.colorRoles = e.target.checked;
    drawMap();
  });
  const canvas = $('#map');
  canvas.addEventListener('mousemove', onHover);
  canvas.addEventListener('mouseleave', () => {
    $('#tooltip').hidden = true;
    setHighlight(-1);
  });
  window.addEventListener('resize', () => view && drawMap());
}

function renderFacts(ins, ctx) {
  const info = ins.info;
  const size = `${info.rows} × ${info.cols} modules`;
  const ecAbout = { L: '~7% recovery', M: '~15% recovery', Q: '~25% recovery', H: '~30% recovery' };
  let maskWhy = 'fixed: rMQR has one mask';
  if (!info.fixedMask) {
    maskWhy = ctx.maskForced ? 'forced in the settings' : info.symbology === 'QR Code' ? 'lowest penalty' : 'highest edge score';
  }
  const used = Math.min(100, (100 * info.dataBits) / info.capacityBits);
  $('#facts').replaceChildren(
    fact('Symbol', `${info.symbology === 'QR Code' ? 'Version ' : ''}${info.version}`, `${info.symbology}, ${size}`),
    fact('Error correction', info.detectionOnly ? 'None' : info.ecLevel, info.detectionOnly ? 'M1 only detects errors' : ecAbout[info.ecLevel]),
    fact('Mask', String(info.mask), maskWhy),
    el('div', { class: 'fact' },
      el('span', { text: 'Data' }),
      el('strong', { text: `${info.dataBits} of ${info.capacityBits} bits` }),
      el('small', { text: `${info.codewords} codewords in ${ins.blocks.length} block${ins.blocks.length === 1 ? '' : 's'}` }),
      el('div', { class: 'meter' }, el('i', { style: `width:${used.toFixed(1)}%` })),
    ),
  );
}

function fact(label, value, note) {
  return el('div', { class: 'fact' }, el('span', { text: label }), el('strong', { text: value }), el('small', { text: note }));
}

// Mixes a hex colour with white; t = 0 gives the colour, 1 gives white.
function tint(color, t) {
  const n = parseInt(color.slice(1), 16);
  const mix = (c) => Math.round(c + (255 - c) * t);
  return `rgb(${mix(n >> 16)}, ${mix((n >> 8) & 255)}, ${mix(n & 255)})`;
}

function moduleColor(i) {
  const ins = view.ins;
  const role = ins.roles[i];
  const isData = role <= 1;
  if (view.layer === 'mask') {
    if (!isData) return '#e7eaf0';
    return ins.maskPattern[i] === '1' ? '#334155' : '#ffffff';
  }
  const dark = (view.layer === 'unmasked' ? ins.unmasked : ins.modules)[i] === '1';
  if (!view.colorRoles) return dark ? '#111111' : '#ffffff';
  let color = ROLES[role].color;
  if (role === 0) color = ins.codewordIsEC[ins.moduleCodeword[i]] ? EC_COLOR : DATA_COLOR;
  return dark ? color : tint(color, 0.84);
}

function drawMap() {
  const { ins } = view;
  const { rows, cols } = ins.info;
  const canvas = $('#map');
  const wrap = canvas.parentElement;
  const margin = 1;
  const maxW = Math.max(240, wrap.clientWidth - 2);
  const maxH = 560;
  const cell = Math.max(3, Math.min(24, Math.floor(Math.min(maxW / (cols + 2 * margin), maxH / (rows + 2 * margin)))));
  view.cell = cell;
  view.margin = margin;
  const w = (cols + 2 * margin) * cell;
  const h = (rows + 2 * margin) * cell;
  const dpr = window.devicePixelRatio || 1;
  canvas.width = w * dpr;
  canvas.height = h * dpr;
  canvas.style.width = `${w}px`;
  canvas.style.height = `${h}px`;
  const g = canvas.getContext('2d');
  g.setTransform(dpr, 0, 0, dpr, 0, 0);
  g.fillStyle = '#ffffff';
  g.fillRect(0, 0, w, h);
  for (let r = 0; r < rows; r++) {
    for (let c = 0; c < cols; c++) {
      g.fillStyle = moduleColor(r * cols + c);
      g.fillRect((c + margin) * cell, (r + margin) * cell, cell, cell);
    }
  }
  if (cell >= 6) {
    // Faint grid lines make individual modules countable when they are big.
    g.strokeStyle = 'rgba(0, 0, 0, 0.07)';
    g.lineWidth = 1;
    g.beginPath();
    for (let r = 0; r <= rows; r++) {
      g.moveTo(margin * cell, (r + margin) * cell + 0.5);
      g.lineTo((cols + margin) * cell, (r + margin) * cell + 0.5);
    }
    for (let c = 0; c <= cols; c++) {
      g.moveTo((c + margin) * cell + 0.5, margin * cell);
      g.lineTo((c + margin) * cell + 0.5, (rows + margin) * cell);
    }
    g.stroke();
  }
  if (view.highlight >= 0) {
    g.fillStyle = 'rgba(255, 255, 255, 0.55)';
    g.fillRect(0, 0, w, h);
    g.lineWidth = Math.max(2, cell / 5);
    g.strokeStyle = '#f59e0b';
    for (let i = 0; i < rows * cols; i++) {
      if (ins.moduleCodeword[i] !== view.highlight) continue;
      const r = Math.floor(i / cols);
      const c = i % cols;
      g.fillStyle = moduleColor(i);
      g.fillRect((c + margin) * cell, (r + margin) * cell, cell, cell);
      g.strokeRect((c + margin) * cell + 1, (r + margin) * cell + 1, cell - 2, cell - 2);
      if (cell >= 12) {
        // The bit number, 7 for the most significant.
        g.fillStyle = (view.layer === 'unmasked' ? ins.unmasked : ins.modules)[i] === '1' ? '#fff' : '#111';
        g.font = `${Math.floor(cell * 0.55)}px ui-monospace, monospace`;
        g.textAlign = 'center';
        g.textBaseline = 'middle';
        g.fillText(String(ins.moduleBit[i]), (c + margin + 0.5) * cell, (r + margin + 0.55) * cell);
      }
    }
  }
  renderLayerHint();
}

function renderLayerHint() {
  const ins = view.ins;
  const qrMask = ins.info.fixedMask ? 4 : ins.info.symbology === 'Micro QR Code' ? MICRO_TO_QR[ins.info.mask] : ins.info.mask;
  const hints = {
    final: 'The finished symbol, as a scanner sees it. Hover a module to see its role and the codeword bit it carries.',
    unmasked: 'The data as placed, before masking. Without the mask, runs and blocks of one colour are common, and they confuse scanners.',
    mask: `Where the mask inverts data modules: ${QR_MASKS[qrMask]} (row i, column j). Function modules are never masked.`,
  };
  const link = view.layer === 'mask' ? ['06-masking.md', 'masking'] : ['05-matrix-construction.md', 'matrix construction'];
  $('#layer-hint').replaceChildren(hints[view.layer], ' See ', el('a', { href: THEORY + link[0], target: '_blank', rel: 'noopener', text: link[1] }), '.');
}

function renderLegend(ins) {
  const counts = new Map();
  for (const role of ins.roles) counts.set(role, (counts.get(role) || 0) + 1);
  const items = [];
  const dataModules = ins.roles.reduce((n, role, i) => n + (role === 0 && !ins.codewordIsEC[ins.moduleCodeword[i]] ? 1 : 0), 0);
  const ecModules = (counts.get(0) || 0) - dataModules;
  items.push(legendItem(DATA_COLOR, 'Data codewords', dataModules));
  items.push(legendItem(EC_COLOR, 'Error correction', ecModules));
  ROLES.forEach((role, i) => {
    if (i === 0 || !counts.get(i)) return;
    items.push(legendItem(role.color, role.name, counts.get(i), role.about));
  });
  $('#legend').replaceChildren(...items);
}

function legendItem(color, name, count, about) {
  return el('li', { title: about }, el('i', { style: `background:${color}` }), `${name} (${count})`);
}

function onHover(event) {
  const ins = view.ins;
  const { rows, cols } = ins.info;
  const rect = event.target.getBoundingClientRect();
  const c = Math.floor((event.clientX - rect.left) / view.cell) - view.margin;
  const r = Math.floor((event.clientY - rect.top) / view.cell) - view.margin;
  const tip = $('#tooltip');
  if (r < 0 || c < 0 || r >= rows || c >= cols) {
    tip.hidden = true;
    setHighlight(-1);
    return;
  }
  const i = r * cols + c;
  const role = ins.roles[i];
  const lines = [`Row ${r}, column ${c}`];
  if (role === 0) {
    const cw = ins.moduleCodeword[i];
    const part = ins.codewordIsEC[cw] ? 'error correction' : 'data';
    lines.push(`Codeword ${cw + 1} of ${ins.codewords.length} (${part}, 0x${hex(ins.codewords[cw])})`);
    lines.push(`Block ${ins.codewordBlock[cw] + 1}, ${part} codeword ${ins.codewordIndex[cw] + 1}`);
    lines.push(`Bit ${ins.moduleBit[i]}${ins.moduleBit[i] === 7 ? ' (most significant)' : ''}: ${ins.unmasked[i]} before the mask, ${ins.modules[i]} after`);
    setHighlight(cw);
  } else {
    lines.push(ROLES[role].name);
    if (role === 1) lines.push(`0 before the mask, ${ins.modules[i]} after`);
    else lines.push(ins.modules[i] === '1' ? 'Dark' : 'Light');
    setHighlight(-1);
  }
  tip.replaceChildren(...lines.flatMap((line, k) => (k ? [el('br'), line] : [el('strong', { text: line })])));
  tip.hidden = false;
  const wrap = event.target.parentElement.getBoundingClientRect();
  let x = event.clientX - wrap.left + 14;
  const y = event.clientY - wrap.top + 14;
  if (x + tip.offsetWidth > wrap.width) x = Math.max(0, event.clientX - wrap.left - tip.offsetWidth - 14);
  tip.style.left = `${x}px`;
  tip.style.top = `${y}px`;
}

function setHighlight(cw) {
  if (!view || view.highlight === cw) return;
  view.highlight = cw;
  drawMap();
  for (const chip of $$('.cw.on')) chip.classList.remove('on');
  if (cw >= 0) {
    const chip = $(`.cw[data-cw="${cw}"]`);
    if (chip) chip.classList.add('on');
  }
}

function renderMessage(ins) {
  const cap = ins.info.capacityBits;
  const term = ins.terminatorBits;
  const padding = cap - ins.info.dataBits - term;
  const bar = [];
  const rows = [];
  for (const s of ins.segments) {
    const bits = s.indicatorBits + s.countBits + s.payloadBits;
    const color = MODE_COLORS[s.mode] || '#475569';
    const label = s.mode === 'ECI' ? `ECI ${s.count}` : s.mode;
    bar.push(el('div', { style: `flex:${bits} 0 0;background:${color}`, title: `${label}: ${bits} bits` }, bits / cap > 0.08 ? label : ''));
    const content = s.mode === 'ECI' ? `ECI ${s.count} (${s.count === 26 ? 'UTF-8' : s.count === 3 ? 'ISO-8859-1' : 'charset'})` : s.text;
    rows.push(el('tr', {},
      el('td', {}, el('i', { class: 'mode-dot', style: `background:${color}` }), s.mode),
      el('td', { text: content }),
      el('td', { text: s.mode === 'ECI' ? '' : String(s.count) }),
      el('td', { text: `${s.indicatorBits} + ${s.countBits} + ${s.payloadBits} = ${bits}` }),
    ));
  }
  if (term) {
    bar.push(el('div', { style: `flex:${term} 0 0;background:#1f2937`, title: `Terminator: ${term} bits` }));
    rows.push(el('tr', {}, el('td', { text: 'Terminator' }), el('td', { text: '0'.repeat(term) }), el('td'), el('td', { text: String(term) })));
  }
  if (padding > 0) {
    bar.push(el('div', { style: `flex:${padding} 0 0;background:#cbd5e1;color:#334155`, title: `Padding: ${padding} bits` }, padding / cap > 0.12 ? 'padding' : ''));
    rows.push(el('tr', {}, el('td', { text: 'Padding' }), el('td', { class: 'note', text: 'Zero bits to the next codeword boundary, then the pad codewords EC and 11 in turn.' }), el('td'), el('td', { text: String(padding) })));
  }
  $('#bitbar').replaceChildren(...bar);
  $('#segments').replaceChildren(...rows);
}

const BLOCK_LIMIT = 12;

function renderBlocks(ins, showAll = false) {
  const blocks = showAll ? ins.blocks : ins.blocks.slice(0, BLOCK_LIMIT);
  const nodes = blocks.map((blk, b) => el('div', { class: 'blk' },
    el('div', { class: 'blk-title', text: `Block ${b + 1}: ${blk.data.length} data + ${blk.ec.length} error-correction codewords` }),
    el('div', { class: 'cws' },
      ...blk.data.map((v, i) => chip(v, view.cwKey.get(`${b}:0:${i}`), false)),
      ...blk.ec.map((v, i) => chip(v, view.cwKey.get(`${b}:1:${i}`), true)),
    ),
  ));
  if (!showAll && ins.blocks.length > BLOCK_LIMIT) {
    nodes.push(el('button', {
      type: 'button', class: 'ghost blocks-more', text: `Show all ${ins.blocks.length} blocks`,
      onclick: () => renderBlocks(ins, true),
    }));
  }
  $('#blocks').replaceChildren(...nodes);
}

function chip(value, cw, ec) {
  return el('span', {
    class: ec ? 'cw ec' : 'cw',
    dataset: { cw: String(cw) },
    title: `Codeword ${cw + 1}: 0x${hex(value)} = ${value.toString(2).padStart(8, '0')}`,
    onmouseenter: () => setHighlight(cw),
    onmouseleave: () => setHighlight(-1),
  }, hex(value));
}

function renderMasks(ins, masks, ctx) {
  const info = ins.info;
  const micro = info.symbology === 'Micro QR Code';
  if (info.fixedMask) {
    $('#mask-hint').replaceChildren(`rMQR has no mask choice: every symbol uses QR's mask 4, ${QR_MASKS[4]}. See `,
      el('a', { href: `${THEORY}28-rmqr.md`, target: '_blank', rel: 'noopener', text: 'rMQR Code' }), '.');
    $('#masks').replaceChildren();
    return;
  }
  $('#mask-hint').replaceChildren(
    micro
      ? 'Micro QR counts the dark modules along the right and bottom edges; the highest score wins. '
      : 'Each mask is scored with four penalty rules (N1 runs, N2 blocks, N3 finder-like patterns, N4 dark balance); the lowest total wins. ',
    'Click a mask to force it. See ', el('a', { href: `${THEORY}06-masking.md`, target: '_blank', rel: 'noopener', text: 'masking' }), '.');
  const best = micro ? Math.max(...ins.maskScores) : Math.min(...ins.maskScores);
  $('#masks').replaceChildren(...masks.map((m) => {
    const k = m.mask;
    const score = ins.maskScores[k];
    const thumb = el('canvas');
    drawThumb(thumb, m.modules, info.rows, info.cols);
    const rules = ins.maskPenalties ? ins.maskPenalties[k] : null;
    return el('div', {
      class: k === info.mask ? 'mask-card chosen' : 'mask-card',
      role: 'button', tabindex: '0',
      title: rules ? `N1 ${rules[0]}, N2 ${rules[1]}, N3 ${rules[2]}, N4 ${rules[3]}` : '',
      onclick: () => ctx.onPickMask(k),
      onkeydown: (e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); ctx.onPickMask(k); } },
    },
    thumb,
    el('div', { class: 'name', text: `Mask ${k}${k === info.mask ? ' (used)' : ''}` }),
    el('div', { class: 'score', text: `${micro ? 'score' : 'penalty'} ${score}${score === best ? (micro ? ', highest' : ', lowest') : ''}` }),
    el('div', { class: 'formula', text: QR_MASKS[micro ? MICRO_TO_QR[k] : k] }),
    );
  }));
}

function drawThumb(canvas, modules, rows, cols) {
  const cell = Math.max(1, Math.floor(110 / (cols + 2)));
  canvas.width = (cols + 2) * cell;
  canvas.height = (rows + 2) * cell;
  const g = canvas.getContext('2d');
  g.fillStyle = '#fff';
  g.fillRect(0, 0, canvas.width, canvas.height);
  g.fillStyle = '#111';
  for (let r = 0; r < rows; r++) {
    for (let c = 0; c < cols; c++) {
      if (modules[r * cols + c] === '1') g.fillRect((c + 1) * cell, (r + 1) * cell, cell, cell);
    }
  }
}
