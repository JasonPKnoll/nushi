// The orrery: Nushi's mark as a small solar system behind the page. The amber sun, the two broken rings (four
// arcs each, as in the mark) and the agents as planets on them, in real 3D (perspective, lit from the sun).
// Each [data-cam] element is a camera stop: scrolling moves between stops (orbit, tilt). Focus stays on the sun's
// plane, so what's nearer or farther softens a little, never more. Below a screenshot the sun sits just behind its top edge, so the screenshot eclipses it.
// At the last stop the camera looks straight down and the system settles into the flat mark above Install.
//
// Depth of field: canvases blurred by fixed CSS amounts under two pixels apart, so a step between them can't be
// seen. Every piece is drawn whole and opaque onto the one nearest its own blur (crossfading two half-transparent
// copies showed seams). A ring is cut where a planet sits on it, so no line ever crosses a planet.
// The cursor tilts the camera at once, and planets lean toward a nearby cursor and spring back. Scrolling fast
// winds the rings a little. Nothing turns on its own: the loop runs only while something is still moving, then
// stops, so an idle page costs nothing. One still frame under Reduce Motion. Plain canvas, no library.
(() => {
  const sky = document.querySelector('.sky');
  if (!sky) return;
  const TAU = Math.PI * 2, D2R = Math.PI / 180;
  const AMBER = '245,165,36';

  // Signed blur levels in CSS px: positive is behind the focal plane, negative in front; stacked far to near.
  // Blurred canvases are drawn at lower resolution, which the blur hides.
  const LEVELS = [3, 1.5, 0, -1.8, -3.6];
  const layers = LEVELS.map((lv) => {
    const c = document.createElement('canvas');
    sky.append(c);
    const ctx = c.getContext('2d');
    if (lv) c.style.filter = `blur(${Math.abs(lv)}px)`;
    return { c, ctx, lv, res: lv === 0 ? 1 : Math.abs(lv) < 2 ? 0.6 : 0.45, used: false };
  });
  if (!layers[0].ctx) return;

  // The mark, normalised so the outer ring's centre line has radius 1. Angles are in the mark's own y-down
  // screen space, so seen from straight above with no turn it is the mark exactly.
  const W = 2.8 / 31;
  const RINGS = [
    { r: 1, from: 0, span: 68.7, wind: 1, turn: 0 },
    { r: 19 / 31, from: 30, span: 64, wind: 1.6, turn: 0 },
  ];
  const PLANETS = [
    { name: 'Claude', ring: 0, a: 0, size: 3.8 / 31, mark: true },
    { name: 'OpenCode', ring: 0, a: 135, size: 3.8 / 31, mark: true },
    { name: 'Ante', ring: 0, a: 222, size: 0.085 },
    { name: 'oh-my-pi', ring: 0, a: 296, size: 0.074 },
    { name: 'Codex', ring: 1, a: 270, size: 3.2 / 31, mark: true },
    { name: 'Pi', ring: 1, a: 30, size: 3.2 / 31, mark: true },
    { name: 'PiG', ring: 1, a: 152, size: 0.078 },
  ].map((p) => ({ ...p, ox: 0, oy: 0, vx: 0, vy: 0, label: 0, X: 0, Y: 0, s: 0, rad: 0, depth: 0, el: null }));
  const SUN = 5.6 / 31;

  // Planet names are DOM labels on top: always sharp, no canvas of their own.
  for (const p of PLANETS) {
    p.el = document.createElement('span');
    p.el.className = 'planet-label';
    p.el.textContent = p.name;
    sky.append(p.el);
  }

  // A few faint stars far out, fixed in the world, so they move only with the camera.
  const rnd = (a, b) => a + Math.random() * (b - a);
  const STARS = Array.from({ length: innerWidth < 700 ? 30 : 56 }, () => {
    const u = rnd(-1, 1), t = rnd(0, TAU), r = rnd(5, 10), s = Math.sqrt(1 - u * u);
    return { x: r * s * Math.cos(t), y: r * u * 0.7, z: r * s * Math.sin(t), a: rnd(0.25, 0.55), s: rnd(0.7, 1.3) };
  });
  const DOT = document.createElement('canvas');
  DOT.width = DOT.height = 16;
  {
    const g = DOT.getContext('2d');
    const r = g.createRadialGradient(8, 8, 0, 8, 8, 8);
    r.addColorStop(0, 'rgba(238,241,245,1)'); r.addColorStop(0.55, 'rgba(238,241,245,.9)'); r.addColorStop(1, 'rgba(238,241,245,0)');
    g.fillStyle = r; g.fillRect(0, 0, 16, 16);
  }

  // Reusable draw items (no closures or arrays made per frame).
  const K_STAR = 0, K_GLOW = 1, K_PIECE = 2, K_SUN = 3, K_PLANET = 4;
  const PIECES_PER_ARC = 14;
  const pool = [];
  const item = (i) => pool[i] || (pool[i] = { k: 0, d: 0, c: 0, x: 0, y: 0, r: 0, a: 0, col: '', p: null, pts: new Float32Array(44), n: 0 });
  let items = [];

  // Stops, read from the page.
  const wide = matchMedia('(min-width: 980px)');
  let stops = [], vw = 0, vh = 0, max = 0;
  function measure() {
    vw = innerWidth; vh = innerHeight;
    const dpr = Math.min(devicePixelRatio || 1, 2);
    for (const L of layers) {
      const k = dpr * L.res;
      L.c.width = Math.round(vw * k); L.c.height = Math.round(vh * k);
      L.ctx.setTransform(k, 0, 0, k, 0, 0);
      L.used = true;
    }
    max = Math.max(0, document.documentElement.scrollHeight - vh);
    stops = [...document.querySelectorAll('[data-cam]')].map((el) => {
      const [elev, az, D, K, focus, flat, labels] = el.dataset.cam.split(/\s+/);
      const r = el.getBoundingClientRect(), top = r.top + scrollY;
      // A stop is reached when its element is centred, or its top is at the top if it's taller than the screen.
      const at = el.id === 'top' ? 0 : Math.min(max, Math.max(0, top + Math.min(r.height, vh) / 2 - vh / 2));
      const slotEl = (wide.matches && el.dataset.slotWide && document.querySelector(el.dataset.slotWide)) || el.querySelector('.slot');
      let s = slotEl && slotEl.getBoundingClientRect();
      let fixed = null;
      if (!s || !s.width) s = { left: r.left + r.width * 0.25, width: r.width * 0.5, top: r.top + r.height / 2 - r.width * 0.25, height: r.width * 0.5 };
      else {
        // A slot in a sticky box sits at the same place on screen whenever that box is stuck.
        const st = slotEl.closest('[data-sticky]');
        if (st) fixed = parseFloat(getComputedStyle(st).top) + (s.top - st.getBoundingClientRect().top) + s.height / 2;
      }
      return {
        id: el.id, at, fixed,
        x: s.left + s.width / 2, docY: s.top + scrollY + s.height / 2, R: s.width / 2 / 1.05,
        el: +elev, az: +az, D: +D, K: +K, focus, flat: +flat, labels: vw > 900 ? +labels : 0,
      };
    }).sort((a, b) => a.at - b.at);
  }
  // A stop's anchor on screen for this scroll: glued to its slot, so the sun stays put behind a screenshot.
  const anchorY = (st, s) => (st.fixed != null ? st.fixed : st.docY - s);

  // Where the camera wants to be for this scroll: between two stops, resting near each.
  const smooth = (t) => t * t * (3 - 2 * t);
  let frozen = false; // Reduce Motion: the page's top, drawn once, scrolling away with the hero
  const T = {};
  function target() {
    const s = frozen ? 0 : Math.min(scrollY, max);
    let i = 0;
    while (i < stops.length - 1 && s >= stops[i + 1].at && s > 0) i++;
    const a = stops[i], b = stops[Math.min(i + 1, stops.length - 1)];
    let t = b.at > a.at ? (s - a.at) / (b.at - a.at) : 0;
    t = smooth(Math.min(1, Math.max(0, (t - 0.15) / 0.7)));
    for (const k of ['x', 'R', 'el', 'az', 'D', 'K', 'flat', 'labels']) T[k] = a[k] + (b[k] - a[k]) * t;
    const ya = anchorY(a, frozen ? 0 : scrollY), yb = anchorY(b, frozen ? 0 : scrollY);
    T.y = ya + (yb - ya) * t;
    T.focus = t < 0.5 ? a.focus : b.focus;
    // While the system travels between two places on screen it fades back, so it never sits on the copy it passes.
    const dist = Math.hypot(b.x - a.x, yb - ya);
    const dip = Math.min(1, dist / (0.6 * vh)) * (vw < 980 ? 0.85 : 0.6);
    T.dim = 1 - dip * Math.sin(Math.PI * t) ** 2;
  }

  const cam = { x: 0, y: 0, R: 1, el: 0.3, az: 0, D: 3.6, K: 1, flat: 0, labels: 0, focal: 3.6, px: 0, py: 0 };
  let ce = 1, se = 0, ca = 1, sa = 0;
  function setView() {
    const el = Math.min(Math.PI / 2, cam.el + cam.py * 0.06 * (1 - 0.7 * cam.flat));
    const az = cam.az + cam.px * 0.12 * (1 - 0.7 * cam.flat);
    ce = Math.cos(el); se = Math.sin(el); ca = Math.cos(az); sa = Math.sin(az);
  }
  // World point (ring plane is y = 0) to screen x, y, depth and scale, into o.
  function proj(x, y, z, o) {
    const x1 = x * ca - z * sa, z1 = x * sa + z * ca;
    const sy = -z1 * se - y * ce, dz = z1 * ce - y * se;
    const depth = Math.max(0.2, cam.D + dz), s = (cam.R * cam.D) / depth;
    o.X = cam.x + x1 * s; o.Y = cam.y + sy * s; o.depth = depth; o.s = s;
    return o;
  }
  const P = { X: 0, Y: 0, depth: 0, s: 0 };
  // Signed circle of confusion, in px, for something at this depth.
  const coc = (depth) => (cam.K * 9 * (depth - cam.focal)) / depth;

  const pointer = { x: -1e4, y: -1e4, in: false, mouse: false };
  let chosen = null;

  // ---------- step: move everything toward where it wants to be; say whether anything still moves ----------
  let lastScroll = -1, spin = 0;
  function step(dt, still) {
    target();
    let busy = false;
    const k = still ? 1 : 1 - Math.exp(-dt * 9);
    // Position follows the page at once (it's glued to a slot); the view eases.
    cam.x = T.x; cam.y = T.y; cam.R = T.R;
    for (const key of ['el', 'az', 'D', 'K', 'flat']) {
      cam[key] += (T[key] - cam[key]) * k;
      if (Math.abs(T[key] - cam[key]) > 1e-4) busy = true;
    }
    cam.labels += (T.labels - cam.labels) * (still ? 1 : 1 - Math.exp(-dt * 6));
    if (Math.abs(T.labels - cam.labels) > 0.004) busy = true;
    // The cursor tilts the view quickly (about a sixth of a second).
    const kp = 1 - Math.exp(-dt * 18);
    const tx = pointer.in && !still ? pointer.x / vw * 2 - 1 : 0, ty = pointer.in && !still ? pointer.y / vh * 2 - 1 : 0;
    cam.px += (tx - cam.px) * kp; cam.py += (ty - cam.py) * kp;
    if (Math.abs(tx - cam.px) + Math.abs(ty - cam.py) > 1e-3) busy = true;
    setView();

    // Scrolling quickly winds the rings a little, then they coast to a stop.
    const sy = scrollY;
    const v = lastScroll < 0 ? 0 : Math.abs(sy - lastScroll) / Math.max(dt, 1e-3);
    lastScroll = sy;
    spin += (Math.min(3, v / 700) - spin) * (1 - Math.exp(-dt * 3));
    if (spin < 0.003) spin = 0; else busy = true;
    for (const g of RINGS) {
      if (!still) g.turn += 0.6 * g.wind * spin * (1 - cam.flat) * dt * 60;
      g.turn = ((g.turn + 180) % 360 + 360) % 360 - 180;
      // Near the last stop, ease each ring back to where the mark has it.
      if (cam.flat > 0.01) {
        g.turn *= still ? 0 : Math.exp(-dt * 7 * cam.flat * cam.flat);
        if (cam.flat > 0.98 && Math.abs(g.turn) < 0.3) g.turn = 0;
        if (g.turn) busy = true;
      }
    }

    // Focus stays on the sun.
    for (const p of PLANETS) {
      const g = RINGS[p.ring];
      const a = (p.a + g.turn) * D2R;
      proj(g.r * Math.cos(a), 0, -g.r * Math.sin(a), p);
      p.rad = p.size * p.s * (0.55 + 0.45 * cam.flat) * (p.mark ? 1 : 1 - cam.flat);
    }
    const fd = cam.D;
    cam.focal += (fd - cam.focal) * (still ? 1 : 1 - Math.exp(-dt * 6));
    if (Math.abs(fd - cam.focal) > 1e-3) busy = true;

    // Planets lean toward a nearby cursor and spring back.
    let hover = null, best = 1e9;
    const h = Math.min(dt, 1 / 30);
    for (const p of PLANETS) {
      let gx = 0, gy = 0;
      const dx = pointer.x - p.X, dy = pointer.y - p.Y, d = Math.hypot(dx, dy);
      if (pointer.in && pointer.mouse && !still && d < 220 && cam.flat < 0.5) {
        const f = (1 - d / 220) ** 2;
        gx = dx * 0.16 * f; gy = dy * 0.16 * f;
        if (d < p.rad + 22 && d < best) { best = d; hover = p; }
      }
      if (still) { p.ox = p.oy = p.vx = p.vy = 0; continue; }
      p.vx += ((gx - p.ox) * 140 - p.vx * 13) * h; p.vy += ((gy - p.oy) * 140 - p.vy * 13) * h;
      p.ox += p.vx * h; p.oy += p.vy * h;
      if (Math.abs(gx - p.ox) + Math.abs(gy - p.oy) + Math.abs(p.vx) + Math.abs(p.vy) > 0.05) busy = true;
    }
    for (const p of PLANETS) {
      const want = Math.max(cam.labels, p === hover || p.name === chosen ? 1 : 0) * (1 - cam.flat) * (p.rad > 1 ? 1 : 0);
      p.label += (want - p.label) * (still ? 1 : 1 - Math.exp(-dt * 10));
      if (Math.abs(want - p.label) > 0.004) busy = true; else p.label = want;
    }
    return busy;
  }

  // ---------- draw ----------
  // Draw one item onto the two canvases either side of its blur, faded by how close it is to each.
  function put(it) {
    const c = Math.max(LEVELS[LEVELS.length - 1], Math.min(LEVELS[0], it.c));
    let i = 0, best = 1e9;
    for (let j = 0; j < LEVELS.length; j++) { const e = Math.abs(LEVELS[j] - c); if (e < best) { best = e; i = j; } }
    paint(layers[i], it, it.a);
  }
  let sunX = 0, sunY = 0, sunS = 0;
  function paint(L, it, alpha) {
    const g = L.ctx;
    L.used = true;
    g.globalAlpha = alpha;
    const flat = cam.flat;
    switch (it.k) {
      case K_STAR:
        g.drawImage(DOT, it.x - it.r, it.y - it.r, it.r * 2, it.r * 2);
        break;
      case K_GLOW: {
        const gr = g.createRadialGradient(it.x, it.y, 0, it.x, it.y, it.r);
        gr.addColorStop(0, `rgba(${AMBER},${0.2 * (1 - flat)})`); gr.addColorStop(1, `rgba(${AMBER},0)`);
        g.fillStyle = gr; g.beginPath(); g.arc(it.x, it.y, it.r, 0, TAU); g.fill();
        break;
      }
      case K_PIECE: {
        const q = it.pts;
        g.fillStyle = it.col;
        g.beginPath(); g.moveTo(q[0], q[1]);
        for (let j = 2; j < it.n; j += 2) g.lineTo(q[j], q[j + 1]);
        g.closePath(); g.fill();
        break;
      }
      case K_SUN: {
        const gr = g.createRadialGradient(it.x - it.r * 0.3, it.y - it.r * 0.3, it.r * 0.1, it.x, it.y, it.r);
        gr.addColorStop(0, flat > 0.9 ? '#F5A524' : '#FFD27A'); gr.addColorStop(1, '#F5A524');
        g.fillStyle = gr; g.beginPath(); g.arc(it.x, it.y, it.r, 0, TAU); g.fill();
        break;
      }
      case K_PLANET: {
        // A small sphere lit from the sun's side; a flat mist disc once it's the mark.
        const X = it.x, Y = it.y, r = it.r;
        const dx = sunX - X, dy = sunY - Y, dl = Math.hypot(dx, dy) || 1, lit = 0.55 * (1 - flat);
        const gr = g.createRadialGradient(X + (dx / dl) * r * lit, Y + (dy / dl) * r * lit, r * 0.05, X, Y, r);
        const m = (a, b) => Math.round(a + (b - a) * flat);
        gr.addColorStop(0, `rgb(${m(214, 238)},${m(222, 241)},${m(228, 245)})`);
        gr.addColorStop(0.5, `rgb(${m(128, 238)},${m(148, 241)},${m(160, 245)})`);
        gr.addColorStop(1, `rgb(${m(36, 238)},${m(56, 241)},${m(66, 245)})`);
        g.fillStyle = gr; g.beginPath(); g.arc(X, Y, r, 0, TAU); g.fill();
        break;
      }
    }
  }

  // Ring piece outline: outer edge, end cap, inner edge back, start cap; into it.pts.
  function piece(it, g, b0, b1, first, last) {
    let n = 0;
    const w = W * (0.24 + 0.76 * cam.flat); // a fine orbit line in 3D, the mark's weight once flat
    const q = it.pts;
    const pt = (x, z) => { proj(x, 0, z, P); q[n++] = P.X; q[n++] = P.Y; };
    const edge = (r, from, to) => {
      for (let j = 0; j <= 4; j++) { const a = (from + ((to - from) * j) / 4) * D2R; pt(r * Math.cos(a), -r * Math.sin(a)); }
    };
    const cap = (deg, dir) => {
      const ux = Math.cos(deg * D2R), uy = Math.sin(deg * D2R), tx = -uy * dir, ty = ux * dir;
      const cx = g.r * ux, cy = g.r * uy;
      for (let j = 1; j < 4; j++) {
        const ph = (Math.PI * (dir > 0 ? j : 4 - j)) / 4;
        pt(cx + (w / 2) * (ux * Math.cos(ph) + tx * Math.sin(ph)), -(cy + (w / 2) * (uy * Math.cos(ph) + ty * Math.sin(ph))));
      }
    };
    edge(g.r + w / 2, b0, b1);
    if (last) cap(b1, 1);
    edge(g.r - w / 2, b1, b0);
    if (first) cap(b0, -1);
    it.n = n;
  }

  // Whether a ring is cut at this angle (degrees) for a planet sitting there; never once it's the flat mark.
  function cut(g, deg) {
    const k = 1 - Math.min(1, cam.flat * 2); // the gaps close as it flattens into the mark
    if (k <= 0) return false;
    for (const p of PLANETS) {
      if (RINGS[p.ring] !== g) continue;
      let d = Math.abs(((deg - p.a - g.turn) % 360 + 540) % 360 - 180);
      if (d < (((p.size * 0.75) / g.r) / D2R) * k) return true; // just under the planet, so the gap never shows
    }
    return false;
  }

  let shownDim = -1;
  function draw() {
    if (Math.abs(T.dim - shownDim) > 0.005) { shownDim = T.dim; sky.style.opacity = T.dim.toFixed(3); }
    for (const L of layers) if (L.used) { L.ctx.globalAlpha = 1; L.ctx.clearRect(0, 0, vw, vh); L.used = false; }
    let n = 0;
    const flat = cam.flat;

    for (const st of STARS) {
      proj(st.x, st.y, st.z, P);
      if (P.depth < 0.6 || P.X < -20 || P.X > vw + 20 || P.Y < -20 || P.Y > vh + 20) continue;
      const it = item(n++);
      it.k = K_STAR; it.d = P.depth; it.c = Math.min(4, coc(P.depth)); it.x = P.X; it.y = P.Y; it.r = st.s * 1.6; it.a = st.a * (1 - 0.6 * flat);
    }

    proj(0, 0, 0, P);
    sunX = P.X; sunY = P.Y; sunS = P.s;
    const sd = P.depth;
    {
      const it = item(n++);
      it.k = K_GLOW; it.d = sd + 3; it.c = LEVELS[0]; it.x = sunX; it.y = sunY; it.r = sunS * 0.9; it.a = 1;
    }

    // Rings: each arc in short pieces, so each piece is blurred for its own depth.
    const base = [13, 22, 28];
    for (const g of RINGS) {
      for (let a = 0; a < 4; a++) {
        const a0 = g.from + a * 90 + g.turn;
        for (let j = 0; j < PIECES_PER_ARC; j++) {
          const b0 = a0 + (g.span * j) / PIECES_PER_ARC, b1 = a0 + (g.span * (j + 1)) / PIECES_PER_ARC;
          if (cut(g, (b0 + b1) / 2)) continue;
          const it = item(n++);
          const step = g.span / PIECES_PER_ARC;
          const first = j === 0 || cut(g, b0 - step / 2), last = j === PIECES_PER_ARC - 1 || cut(g, b1 + step / 2);
          // Each piece reaches a little into its neighbours, so their soft edges overlap instead of leaving a seam.
          piece(it, g, first ? b0 : b0 - step * 0.35, last ? b1 : b1 + step * 0.35, first, last);
          const mid = ((b0 + b1) / 2) * D2R;
          proj(g.r * Math.cos(mid), 0, -g.r * Math.sin(mid), P);
          const d = P.depth;
          const fade = 0.72 * (1 - flat) + flat; // one colour along the ring: depth shows by blur and size, not shade
          // Slate when seen in 3D, mist once it's the flat mark.
          const r = Math.round(base[0] + ((47 + (238 - 47) * (0.5 + 0.5 * flat)) - base[0]) * fade);
          const gg = Math.round(base[1] + ((74 + (241 - 74) * (0.5 + 0.5 * flat)) - base[1]) * fade);
          const b = Math.round(base[2] + ((87 + (245 - 87) * (0.5 + 0.5 * flat)) - base[2]) * fade);
          it.k = K_PIECE; it.d = d; it.c = coc(d); it.a = 1; it.col = `rgb(${r},${gg},${b})`;
        }
      }
    }
    {
      const it = item(n++);
      it.k = K_SUN; it.d = sd; it.c = coc(sd); it.x = sunX; it.y = sunY; it.r = SUN * sunS * (0.62 + 0.38 * flat); it.a = 1;
    }
    for (const p of PLANETS) {
      if (p.rad < 0.3) continue;
      const it = item(n++);
      it.k = K_PLANET; it.d = p.depth; it.c = coc(p.depth); it.x = p.X + p.ox; it.y = p.Y + p.oy; it.r = p.rad; it.a = 1;
    }

    items.length = 0;
    for (let i = 0; i < n; i++) items.push(pool[i]);
    items.sort((a, b) => b.d - a.d);
    for (const it of items) put(it);

    for (const p of PLANETS) {
      if (p.label < 0.02) { if (p.shown) { p.el.style.opacity = '0'; p.shown = false; } continue; }
      p.shown = true;
      p.el.style.opacity = p.label.toFixed(3);
      p.el.style.transform = `translate3d(${(p.X + p.ox + p.rad + 8).toFixed(1)}px,${(p.Y + p.oy).toFixed(1)}px,0) translateY(-50%)`;
      p.el.classList.toggle('is-chosen', p.name === chosen);
    }
  }

  // ---------- loop: runs only while something moves ----------
  const reduce = matchMedia('(prefers-reduced-motion: reduce)');
  let raf = 0, last = 0;
  function frame(now) {
    raf = 0;
    const dt = Math.min(0.1, last && now > last ? (now - last) / 1000 : 1 / 60);
    last = now;
    const busy = step(dt, false);
    draw();
    if (busy) kick(); else last = 0;
  }
  function kick() {
    if (!raf && !frozen && !document.hidden) raf = requestAnimationFrame(frame);
  }
  function still() {
    if (raf) cancelAnimationFrame(raf);
    raf = 0; last = 0;
    measure();
    step(0, true); draw();
  }
  function start() {
    frozen = reduce.matches;
    document.documentElement.classList.toggle('live', !frozen);
    document.documentElement.classList.toggle('still', frozen);
    measure();
    if (frozen) { still(); return; }
    step(0, true); draw();
    kick();
  }

  addEventListener('pointermove', (e) => {
    pointer.x = e.clientX; pointer.y = e.clientY; pointer.in = true; pointer.mouse = e.pointerType === 'mouse';
    kick();
  }, { passive: true });
  document.addEventListener('pointerleave', () => { pointer.in = false; kick(); });
  addEventListener('blur', () => { pointer.in = false; kick(); });
  addEventListener('scroll', kick, { passive: true });
  // Agent names pull focus to their planet and label it.
  for (const li of document.querySelectorAll('[data-agent]')) {
    const on = () => { chosen = li.dataset.agent; kick(); };
    const off = () => { if (chosen === li.dataset.agent) chosen = null; kick(); };
    li.addEventListener('pointerenter', on); li.addEventListener('focus', on);
    li.addEventListener('pointerleave', off); li.addEventListener('blur', off);
  }
  document.addEventListener('visibilitychange', () => { last = 0; kick(); });
  reduce.addEventListener?.('change', start);
  let rt = 0;
  const remeasure = () => {
    clearTimeout(rt);
    rt = setTimeout(() => { if (frozen) still(); else { measure(); kick(); } }, 60);
  };
  addEventListener('resize', remeasure);
  new ResizeObserver(remeasure).observe(document.body);
  document.fonts?.ready.then(remeasure);
  addEventListener('load', remeasure);
  start();
})();
