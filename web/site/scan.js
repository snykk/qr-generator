// "Scan": decodes symbols from image files (picked, dropped, or pasted) and
// from the camera. Every image is read by qrgen's DecodeAll in the worker; the
// outline and finder centres it reports are drawn over the image.

import { call } from './client.js';
import { $, el, copyText, flash, isWebURL } from './util.js';

const files = []; // the bytes of every scanned image, for structured append

function resultItem(found, n) {
  const kind = found.symbology ? `${found.symbology}${found.version ? ` ${found.version}` : ''}` : 'Symbol';
  const copy = el('button', { type: 'button', class: 'copy', text: 'Copy' });
  copy.addEventListener('click', async () => {
    if (await copyText(found.text)) flash(copy, 'Copied');
  });
  const text = isWebURL(found.text)
    ? el('a', { href: found.text, target: '_blank', rel: 'noopener noreferrer', text: found.text })
    : found.text || '(empty)';
  return el('div', { class: 'result' },
    el('div', { class: 'meta' }, n ? el('span', { class: 'num', text: String(n) }) : null, kind, copy),
    el('div', { class: 'text' }, text),
  );
}

// Draws each symbol's outline, finder centres, and number; corners and
// finders are in canvas pixels, and size scales the strokes and labels.
function drawOutlines(g, symbols, size = 1) {
  const lw = Math.max(2, 3 * size);
  symbols.forEach((s, i) => {
    if (!s.corners) return;
    const pts = s.corners;
    g.lineWidth = lw;
    g.strokeStyle = '#22c55e';
    g.fillStyle = 'rgba(34, 197, 94, 0.18)';
    g.beginPath();
    pts.forEach(([x, y], k) => (k ? g.lineTo(x, y) : g.moveTo(x, y)));
    g.closePath();
    g.fill();
    g.stroke();
    g.fillStyle = '#f97316';
    for (const [x, y] of s.finders || []) {
      g.beginPath();
      g.arc(x, y, Math.max(3, 4 * size), 0, 2 * Math.PI);
      g.fill();
    }
    // The symbol's number, at its top-left corner.
    const [x0, y0] = pts[0];
    const r = Math.max(9, 11 * size);
    g.fillStyle = '#2557d6';
    g.beginPath();
    g.arc(x0, y0, r, 0, 2 * Math.PI);
    g.fill();
    g.fillStyle = '#fff';
    g.font = `bold ${Math.round(r * 1.1)}px system-ui, sans-serif`;
    g.textAlign = 'center';
    g.textBaseline = 'middle';
    g.fillText(String(i + 1), x0, y0 + 1);
  });
}

function loadImage(blob) {
  return new Promise((resolve, reject) => {
    const url = URL.createObjectURL(blob);
    const img = new Image();
    img.onload = () => resolve(img);
    img.onerror = () => reject(new Error('the browser cannot display this image'));
    img.src = url;
  });
}

async function scanFile(file) {
  const bytes = new Uint8Array(await file.arrayBuffer());
  const card = el('div', { class: 'scan' }, el('div', { class: 'scan-name', text: file.name || 'pasted image' }));
  $('#scans').prepend(card);
  $('#clear-scans').hidden = false;
  const canvas = el('canvas');
  try {
    const img = await loadImage(file);
    canvas.width = img.naturalWidth;
    canvas.height = img.naturalHeight;
    canvas.getContext('2d').drawImage(img, 0, 0);
    card.append(canvas);
  } catch {
    // The decoder may still read formats the browser cannot show.
  }
  try {
    const res = await call('decode', {}, [bytes]);
    if (res.symbols.length === 0) {
      card.append(el('div', { class: 'result fail', text: `No symbol found: ${res.failure || 'nothing recognisable'}.` }));
      return;
    }
    files.push(bytes);
    $('#reassemble').hidden = files.length < 2;
    if (canvas.width) drawOutlines(canvas.getContext('2d'), res.symbols, Math.max(1, canvas.width / 600));
    res.symbols.forEach((s, i) => card.append(resultItem(s, res.symbols.length > 1 || s.corners ? i + 1 : 0)));
  } catch (err) {
    card.append(el('div', { class: 'result fail', text: err.message }));
  }
}

async function scanFiles(list) {
  for (const file of list) {
    if (file.type && !file.type.startsWith('image/')) continue;
    await scanFile(file);
  }
}

/** Builds a sample photo-like scene: a tilted QR symbol, a Micro QR symbol, and an rMQR strip. */
async function sampleScene() {
  const pngImage = async (text, settings) => {
    const res = await call('encode', { text, settings: { mask: -1, quietZone: -1, ...settings }, formats: ['png'] });
    return loadImage(new Blob([Uint8Array.from(atob(res.png), (c) => c.charCodeAt(0))], { type: 'image/png' }));
  };
  const [qrImg, microImg, rmqrImg] = await Promise.all([
    pngImage('https://github.com/snykk/qr-generator', { moduleSize: 6 }),
    pngImage('12345', { symbology: 'micro', moduleSize: 8 }),
    pngImage('RECTANGULAR MICRO QR', { symbology: 'rmqr', rmqrHeight: 7, moduleSize: 4 }),
  ]);
  const canvas = el('canvas');
  canvas.width = 720;
  canvas.height = 520;
  const g = canvas.getContext('2d');
  const bg = g.createLinearGradient(0, 0, 720, 520);
  bg.addColorStop(0, '#d9d4cb');
  bg.addColorStop(1, '#a9a197');
  g.fillStyle = bg;
  g.fillRect(0, 0, 720, 520);
  const place = (img, x, y, angle) => {
    g.save();
    g.translate(x, y);
    g.rotate((angle * Math.PI) / 180);
    g.drawImage(img, -img.width / 2, -img.height / 2);
    g.restore();
  };
  place(qrImg, 230, 200, -14);
  place(microImg, 560, 190, 9);
  place(rmqrImg, 380, 445, 4);
  const blob = await new Promise((resolve) => canvas.toBlob(resolve, 'image/png'));
  return new File([blob], 'sample: a tilted QR Code, a Micro QR Code, and an rMQR Code.png', { type: 'image/png' });
}

// Camera scanning: a centred square of each frame, at most 480 pixels wide, is
// sent to the worker; a new frame goes only when the last one is done.
const camera = { stream: null, running: false, busy: false, last: [], lastAt: 0, seen: new Map() };
const grab = document.createElement('canvas');

async function startCamera(deviceId) {
  stopCamera();
  if (!navigator.mediaDevices || !navigator.mediaDevices.getUserMedia) {
    $('#camera-status').textContent = 'This browser cannot use a camera here (a camera needs HTTPS or localhost).';
    return;
  }
  try {
    camera.stream = await navigator.mediaDevices.getUserMedia({
      audio: false,
      video: deviceId ? { deviceId: { exact: deviceId } } : { facingMode: 'environment', width: { ideal: 1280 }, height: { ideal: 720 } },
    });
  } catch (err) {
    $('#camera-status').textContent = `Cannot open the camera: ${err.message}`;
    return;
  }
  const video = $('#video');
  video.srcObject = camera.stream;
  await video.play();
  $('#camera').hidden = false;
  $('#camera-start').hidden = true;
  $('#camera-stop').hidden = false;
  $('#camera-status').textContent = 'Point the camera at a QR Code or Micro QR Code inside the square.';
  const devices = (await navigator.mediaDevices.enumerateDevices()).filter((d) => d.kind === 'videoinput');
  const select = $('#camera-device');
  if (devices.length > 1) {
    const current = camera.stream.getVideoTracks()[0].getSettings().deviceId;
    select.replaceChildren(...devices.map((d, i) => el('option', { value: d.deviceId, text: d.label || `Camera ${i + 1}` })));
    select.value = current;
    select.hidden = false;
  }
  camera.running = true;
  requestAnimationFrame(cameraLoop);
}

function stopCamera() {
  camera.running = false;
  if (camera.stream) camera.stream.getTracks().forEach((t) => t.stop());
  camera.stream = null;
  $('#camera').hidden = true;
  $('#camera-start').hidden = false;
  $('#camera-stop').hidden = true;
}

function scanBox(video) {
  const side = Math.floor(Math.min(video.videoWidth, video.videoHeight) * 0.8);
  return { x: Math.floor((video.videoWidth - side) / 2), y: Math.floor((video.videoHeight - side) / 2), side };
}

function cameraLoop() {
  if (!camera.running) return;
  const video = $('#video');
  if (video.readyState >= 2 && video.videoWidth) {
    drawCameraOverlay(video);
    if (!camera.busy) scanFrame(video);
  }
  requestAnimationFrame(cameraLoop);
}

function drawCameraOverlay(video) {
  const overlay = $('#camera-overlay');
  if (overlay.width !== video.videoWidth) {
    overlay.width = video.videoWidth;
    overlay.height = video.videoHeight;
  }
  const g = overlay.getContext('2d');
  const { x, y, side } = scanBox(video);
  g.clearRect(0, 0, overlay.width, overlay.height);
  g.fillStyle = 'rgba(0, 0, 0, 0.35)';
  g.fillRect(0, 0, overlay.width, overlay.height);
  g.clearRect(x, y, side, side);
  g.strokeStyle = 'rgba(255, 255, 255, 0.9)';
  g.lineWidth = Math.max(2, overlay.width / 320);
  g.strokeRect(x, y, side, side);
  if (performance.now() - camera.lastAt < 700) drawOutlines(g, camera.last, overlay.width / 640);
}

async function scanFrame(video) {
  camera.busy = true;
  const { x, y, side } = scanBox(video);
  const size = Math.min(side, 480);
  const scale = side / size;
  grab.width = size;
  grab.height = size;
  const g = grab.getContext('2d', { willReadFrequently: true });
  g.drawImage(video, x, y, side, side, 0, 0, size, size);
  const pixels = new Uint8Array(g.getImageData(0, 0, size, size).data.buffer);
  const started = performance.now();
  try {
    const res = await call('decodePixels', { width: size, height: size }, [pixels]);
    const ms = Math.round(performance.now() - started);
    if (res.symbols.length) {
      // Back to video coordinates for the overlay.
      camera.last = res.symbols.map((s) => ({
        ...s,
        corners: s.corners && s.corners.map(([cx, cy]) => [x + cx * scale, y + cy * scale]),
        finders: s.finders && s.finders.map(([fx, fy]) => [x + fx * scale, y + fy * scale]),
      }));
      camera.lastAt = performance.now();
      for (const s of res.symbols) addCameraResult(s);
    }
    if (camera.running) $('#camera-status').textContent = `Scanning, ${ms} ms per frame. The camera image stays in your browser.`;
  } catch (err) {
    $('#camera-status').textContent = err.message;
  } finally {
    camera.busy = false;
  }
}

function addCameraResult(found) {
  const list = $('#camera-results');
  const key = `${found.symbology}|${found.text}`;
  const existing = camera.seen.get(key);
  if (existing) {
    list.prepend(existing);
    return;
  }
  const item = el('li', {}, resultItem(found, 0));
  camera.seen.set(key, item);
  list.prepend(item);
  if (navigator.vibrate) navigator.vibrate(40);
}

export function initScan(isActive) {
  const drop = $('#drop');
  $('#scan-files').addEventListener('change', (e) => {
    scanFiles([...e.target.files]);
    e.target.value = '';
  });
  drop.addEventListener('dragover', (e) => {
    e.preventDefault();
    drop.classList.add('over');
  });
  drop.addEventListener('dragleave', () => drop.classList.remove('over'));
  drop.addEventListener('drop', (e) => {
    e.preventDefault();
    drop.classList.remove('over');
    scanFiles([...e.dataTransfer.files]);
  });
  drop.addEventListener('keydown', (e) => {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      $('#scan-files').click();
    }
  });
  document.addEventListener('paste', (e) => {
    if (!isActive()) return;
    const images = [...e.clipboardData.files].filter((f) => f.type.startsWith('image/'));
    if (images.length) scanFiles(images);
  });
  $('#sample').addEventListener('click', async (e) => {
    e.target.disabled = true;
    try {
      await scanFile(await sampleScene());
    } finally {
      e.target.disabled = false;
    }
  });
  $('#reassemble').addEventListener('click', async () => {
    const out = $('#reassembled');
    out.hidden = false;
    out.classList.remove('fail');
    try {
      const { text } = await call('decodeStructuredAppend', {}, files);
      out.replaceChildren(resultItem({ text, symbology: `Structured append of ${files.length} symbols` }, 0));
    } catch (err) {
      out.classList.add('fail');
      out.textContent = `These images are not one complete structured-append set: ${err.message}`;
    }
  });
  $('#clear-scans').addEventListener('click', () => {
    files.length = 0;
    $('#scans').replaceChildren();
    $('#reassembled').hidden = true;
    $('#reassemble').hidden = true;
    $('#clear-scans').hidden = true;
  });
  $('#camera-start').addEventListener('click', () => startCamera());
  $('#camera-stop').addEventListener('click', stopCamera);
  $('#camera-device').addEventListener('change', (e) => startCamera(e.target.value));
}

/** Stops the camera when the page leaves the Scan view. */
export function leaveScan() {
  stopCamera();
}
