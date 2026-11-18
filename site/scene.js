// The orrery: Nushi's mark as a small solar system behind the page. The amber sun, the two broken rings (four
// arcs each, as in the mark) and the agents as planets on them, in real 3D (perspective, lit from the sun).
// Each [data-cam] element is a camera stop: scrolling moves between stops (orbit, tilt). Focus stays on the sun's
// plane, so what's nearer or farther softens a little, never more. Below a screenshot the sun sits just behind its top edge, so the screenshot eclipses it.
// At the last stop the camera looks straight down and the system settles into the flat mark above Install.
//
// Depth of field, on one canvas with no filters: everything is drawn with a soft edge as wide as its own blur.
// A ring arc is one shape whose soft edge widens continuously along it with depth, so nothing steps or travels
// along a line; a planet or the sun is a disc whose rim fades over its blur. Rings are drawn first, so a planet
// always sits on its line, never behind it.
// The cursor tilts the camera at once, and planets lean toward a nearby cursor and spring back. Scrolling fast
// winds the rings a little. Nothing turns on its own.
// Dust: each planet sheds a thin trail of grains along its ring behind it, in the same 3D space (projected, sorted
// among the planets, blurred by the same depth of field). The grains drift slowly back along the trail; the pointer
// (or a dragging finger) pushes them aside and drags them along its path, and they spring back. While dust is
// showing the loop keeps running (at half rate when nothing but the drift moves); once the scene is flat the dust
// has faded and the loop stops again. One still frame under Reduce Motion. Plain canvas, no library.
//
// Debug hook, for headless screenshots (nothing happens without these query params):
//   ?cam=<i>            pin the camera at stop i (and scroll the page there); 8.5 is halfway from stop 8 to 9
//   &px=&py=            a simulated pointer sweep ending at this point (0..1 of the window) ...
//   &pvx=&pvy=          ... travelling at this velocity in px/s; it runs for 0.5 s from t = 1.2 s, then the scene
//   &hold=<ms>          freezes this long after the sweep ends (default 150), so the wake can be seen
//   &bare=1             hide the page's content to see the scene alone (and don't scroll it)
//   &dust=0             no dust (for comparison)
//   &bench=<frames>     time that many frames of step+draw (&flush=1 also waits for the raster) into document.title
(() => {
  const sky = document.querySelector('.sky');
  if (!sky) return;
  const TAU = Math.PI * 2, D2R = Math.PI / 180;
  const AMBER = '245,165,36';

  const cv = document.createElement('canvas');
  sky.append(cv);
  const g = cv.getContext('2d');
  if (!g) return;

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

  // A small seeded generator (mulberry32): the same sky and dust on every visit, and comparable screenshots.
  let seed = 0x6e757368;
  const ran = () => { seed = (seed + 0x6d2b79f5) | 0; let t = Math.imul(seed ^ (seed >>> 15), 1 | seed); t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t; return ((t ^ (t >>> 14)) >>> 0) / 4294967296; };
  const rnd = (a, b) => a + ran() * (b - a);
  // A few faint stars far out, fixed in the world, so they move only with the camera.
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

  // Stops, read from the page.
  const Q = new URLSearchParams(location.search);
  const qn = (k, d) => (Q.has(k) && Q.get(k) !== '' && isFinite(+Q.get(k)) ? +Q.get(k) : d);
  const PIN = Q.has('cam') ? qn('cam', 0) : null;
  if (Q.get('bare') === '1') for (const el of document.querySelectorAll('main, footer, .bar')) el.style.visibility = 'hidden';
  const wide = matchMedia('(min-width: 980px)');
  let stops = [], vw = 0, vh = 0, max = 0, dpr = 1;
  function measure() {
    vw = innerWidth; vh = innerHeight;
    dpr = Math.min(devicePixelRatio || 1, 2);
    cv.width = Math.round(vw * dpr); cv.height = Math.round(vh * dpr);
    g.setTransform(dpr, 0, 0, dpr, 0, 0);
    max = Math.max(0, document.documentElement.scrollHeight - vh);
    sizeDust();
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
    const pin = PIN != null && stops.length ? stops[Math.min(stops.length - 1, Math.max(0, PIN | 0))] : null;
    const pn = pin && stops[Math.min(stops.length - 1, (PIN | 0) + 1)];
    const s = pin ? pin.at + (pn.at - pin.at) * (PIN - (PIN | 0)) : frozen ? 0 : Math.min(scrollY, max);
    let i = 0;
    while (i < stops.length - 1 && s >= stops[i + 1].at && s > 0) i++;
    if (pin) i = stops.indexOf(pin);
    const a = stops[i], b = stops[Math.min(i + 1, stops.length - 1)];
    let t = pin ? PIN - (PIN | 0) : b.at > a.at ? (s - a.at) / (b.at - a.at) : 0;
    if (pin) t = 0.15 + 0.7 * t; // a pinned fraction is the tween's own progress
    t = smooth(Math.min(1, Math.max(0, (t - 0.15) / 0.7)));
    for (const k of ['x', 'R', 'el', 'az', 'D', 'K', 'flat', 'labels']) T[k] = a[k] + (b[k] - a[k]) * t;
    const ya = anchorY(a, pin ? s : frozen ? 0 : scrollY), yb = anchorY(b, pin ? s : frozen ? 0 : scrollY);
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

  // ---------- dust ----------
  // Grains live in typed arrays (nothing allocated per frame). Each belongs to a planet and has an "age" that runs
  // 0..1 at a slow steady rate and wraps; it sits age^1.6 of the trail's span behind its planet, so grains bunch
  // near the planet and spread and fade further back. Scatter across and above/below the ring grows with that
  // distance. The pointer acts in screen space, on an offset per grain held by a spring.
  const DUST_ON = Q.get('dust') !== '0';
  const DUST_CAP = 1000;
  const SPAN = [88, 104]; // degrees of trail behind a planet, per ring
  const DPAT = []; // a grain's planet, interleaved so any prefix is balanced; the marked planets shed a little more
  for (let r = 0; r < 3; r++) PLANETS.forEach((p, i) => { if (r < (p.mark ? 3 : 2)) DPAT.push(i); });
  const dP = new Uint8Array(DUST_CAP), dCol = new Uint8Array(DUST_CAP), dGl = new Uint8Array(DUST_CAP), dLay = new Uint8Array(DUST_CAP);
  const dAge = new Float32Array(DUST_CAP), dRate = new Float32Array(DUST_CAP), dN1 = new Float32Array(DUST_CAP),
    dN2 = new Float32Array(DUST_CAP), dPh = new Float32Array(DUST_CAP), dSz = new Float32Array(DUST_CAP), dBr = new Float32Array(DUST_CAP);
  const dOx = new Float32Array(DUST_CAP), dOy = new Float32Array(DUST_CAP), dVx = new Float32Array(DUST_CAP), dVy = new Float32Array(DUST_CAP);
  const dX = new Float32Array(DUST_CAP), dY = new Float32Array(DUST_CAP), dD = new Float32Array(DUST_CAP),
    dR = new Float32Array(DUST_CAP), dA = new Float32Array(DUST_CAP);
  const dOrder = new Uint16Array(DUST_CAP), dStart = new Uint16Array(12), dCur = new Uint16Array(12);
  const gauss = () => (ran() + ran() + ran() - 1.5) * 2;
  for (let i = 0; i < DUST_CAP; i++) {
    dP[i] = DPAT[i % DPAT.length];
    dAge[i] = ran(); dRate[i] = rnd(0.8, 1.25) / 80; // about 80 s from the planet to the trail's end
    dN1[i] = gauss(); dN2[i] = gauss(); dPh[i] = rnd(0, TAU);
    const q = ran();
    dSz[i] = 0.6 + 0.8 * q * q; dBr[i] = rnd(0.35, 1);
    const c = ran();
    dCol[i] = c < 0.06 ? 2 : c < 0.45 ? 0 : 1; // mist, slate-mist, rarely pale amber
    if (ran() < 0.03) { dGl[i] = 1; dSz[i] *= 1.9; dBr[i] = 1; }
  }
  // A grain is one soft disc: a radial gradient made once per colour (mist, slate-mist, pale amber) at unit size,
  // drawn under a transform that places and scales it, so its falloff is smooth at any size and nothing is
  // allocated per frame (plain discs stacked to fake the falloff showed rings once blurred).
  const DCOL = ['238,241,245', '150,172,186', '255,214,140'].map((rgb) => {
    const r = g.createRadialGradient(0, 0, 0, 0, 0, 1);
    r.addColorStop(0, `rgba(${rgb},1)`); r.addColorStop(0.18, `rgba(${rgb},.85)`); r.addColorStop(0.4, `rgba(${rgb},.38)`);
    r.addColorStop(0.65, `rgba(${rgb},.1)`); r.addColorStop(1, `rgba(${rgb},0)`);
    return r;
  });
  let dustN = 0, dustA = 0, dustT = 0, dustCalm = true, RP = 150;
  // The pointer as the dust feels it: where it is, and its velocity from how far it moved since the last frame.
  const dptr = { x: 0, y: 0, lx: 0, ly: 0, vx: 0, vy: 0, in: false };
  const dustAt = (x, y) => {
    if (!dptr.in) { dptr.lx = x; dptr.ly = y; dptr.vx = dptr.vy = 0; }
    dptr.x = x; dptr.y = y; dptr.in = true;
  };
  function sizeDust() {
    // About 650 grains on a 1440 x 900 window, scaled by area: fewer on a phone, never more than the cap.
    dustN = DUST_ON ? Math.round(Math.min(DUST_CAP, Math.max(110, (vw * vh) / (1440 * 900) * 650)) * (frozen ? 0.5 : 1)) : 0;
    RP = Math.max(80, 0.11 * Math.hypot(vw, vh)); // the pointer's reach
  }

  const K_SPRING = 28, C_DAMP = 2 * 0.45 * Math.sqrt(K_SPRING); // underdamped: one soft overshoot, then rest
  function stepDust(dt, still) {
    // The dust thins as the scene flattens into the mark, and is gone by halfway.
    const shown = dustN ? 1 - smooth(Math.min(1, cam.flat / 0.5)) : 0;
    if (shown < 0.01) {
      // Gone: every grain goes home, so it comes back calm.
      if (dustA) { dOx.fill(0); dOy.fill(0); dVx.fill(0); dVy.fill(0); }
      dustA = 0; dustCalm = true;
      return false;
    }
    dustA = shown;
    const live = !still && !frozen;
    if (live) dustT += dt;
    // Pointer velocity from this frame's movement, lightly smoothed; a pointer that stops has none almost at once.
    if (live && dptr.in && dt > 0) {
      const k = 1 - Math.exp(-dt * 40);
      dptr.vx += ((dptr.x - dptr.lx) / dt - dptr.vx) * k; dptr.vy += ((dptr.y - dptr.ly) / dt - dptr.vy) * k;
      const sp = Math.hypot(dptr.vx, dptr.vy);
      if (sp > 3000) { dptr.vx *= 3000 / sp; dptr.vy *= 3000 / sp; }
    } else { dptr.vx = dptr.vy = 0; }
    dptr.lx = dptr.x; dptr.ly = dptr.y;
    const on = live && dptr.in, px = dptr.x, py = dptr.y, pvx = dptr.vx, pvy = dptr.vy;
    const speed = Math.hypot(pvx, pvy), RP2 = RP * RP;
    const sub = live ? Math.max(1, Math.min(4, Math.ceil(dt * 60 - 0.01))) : 0, h = sub ? Math.min(dt, 1 / 15) / sub : 0;
    let motion = 0;
    for (let i = 0; i < dustN; i++) {
      const p = PLANETS[dP[i]], gR = RINGS[p.ring];
      if (live) {
        let a = dAge[i] + dRate[i] * dt;
        if (a >= 1) { a -= 1; dN1[i] = gauss(); dN2[i] = gauss(); } // off the end, unseen: shed again at the planet
        dAge[i] = a;
      }
      const age = dAge[i], u = age ** 1.6, ph = dPh[i];
      const ang = (p.a + gR.turn - u * SPAN[p.ring]) * D2R;
      const r = gR.r + dN1[i] * (0.008 + 0.075 * u) + 0.004 * Math.sin(dustT * 0.37 + ph);
      const y = dN2[i] * (0.006 + 0.05 * u) + 0.003 * Math.sin(dustT * 0.29 + ph * 1.7);
      proj(r * Math.cos(ang), y, -r * Math.sin(ang), P);
      const X = P.X, Y = P.Y;
      let ox = dOx[i], oy = dOy[i], vx = dVx[i], vy = dVy[i];
      if (sub) {
        let ax = 0, ay = 0;
        if (on) {
          const dx = X + ox - px, dy = Y + oy - py, d2 = dx * dx + dy * dy;
          if (d2 < RP2) {
            const d = Math.sqrt(d2) || 1, nx = dx / d, ny = dy / d;
            // Nearer grains move a little more.
            let w = 1 - d / RP; w = w * w * Math.min(1.5, Math.max(0.6, P.s / cam.R));
            const vn = pvx * nx + pvy * ny;
            // Pushed aside (a little by a resting pointer, more by a moving one); the flow round a moving body
            // (ahead and behind carried along, the sides slipping back), which curls a sweep's wake into a pair
            // of eddies; and a drag along the path.
            const push = 220 + 1.8 * speed;
            ax = w * (nx * push + 3.6 * (2 * vn * nx - pvx) + 5.2 * pvx);
            ay = w * (ny * push + 3.6 * (2 * vn * ny - pvy) + 5.2 * pvy);
          }
        }
        for (let k = 0; k < sub; k++) {
          vx += (ax - K_SPRING * ox - C_DAMP * vx) * h; vy += (ay - K_SPRING * oy - C_DAMP * vy) * h;
          ox += vx * h; oy += vy * h;
        }
        // Back home and still: snap to rest. Held aside by a resting pointer is calm too; only movement counts.
        const sp = Math.abs(vx) + Math.abs(vy);
        if (Math.abs(ox) + Math.abs(oy) < 0.02 && sp < 0.4) ox = oy = vx = vy = 0; else if (sp > 0.4) motion += sp;
        dOx[i] = ox; dOy[i] = oy; dVx[i] = vx; dVy[i] = vy;
      } else if (still) { dOx[i] = dOy[i] = dVx[i] = dVy[i] = 0; ox = oy = 0; }
      dX[i] = X + ox; dY[i] = Y + oy; dD[i] = P.depth;
      // Depth of field as the planets have it: the blur widens a grain and spreads its light.
      const r0 = dSz[i] * (P.s / cam.R) * (cam.R / 260) ** 0.3, b = Math.abs(coc(P.depth)), rr = r0 + b;
      dR[i] = rr;
      // Fade in once clear of the planet, so grains never speckle its face; fade out down the trail.
      const fade = Math.min(1, (u * SPAN[p.ring] * D2R * gR.r) / (p.size * 1.3)) * (1 - u) ** 1.8;
      dA[i] = 0.8 * dBr[i] * fade * dustA * ((r0 + 0.5) / (rr + 0.5)) ** 1.1;
    }
    dustCalm = motion === 0 && speed < 1;
    return live;
  }

  // Draws the grains of layer L (the ones between bodies L-1 and L, far to near).
  function drawDust(L) {
    const end = dStart[L + 1];
    let col = -1;
    for (let k = dStart[L]; k < end; k++) {
      const i = dOrder[k], a = dA[i];
      if (a < 0.004) continue;
      const X = dX[i], Y = dY[i], r = dR[i] * 2.4;
      if (X < -r || Y < -r || X > vw + r || Y > vh + r) continue;
      if (dCol[i] !== col) { col = dCol[i]; g.fillStyle = DCOL[col]; }
      g.globalAlpha = a;
      g.setTransform(dpr * r, 0, 0, dpr * r, dpr * X, dpr * Y);
      g.fillRect(-1, -1, 2, 2);
      if (dGl[i]) { // a larger grain catches the light: a faint wide halo round it
        g.globalAlpha = a * 0.12;
        g.setTransform(dpr * r * 2.4, 0, 0, dpr * r * 2.4, dpr * X, dpr * Y);
        g.fillRect(-1, -1, 2, 2);
      }
    }
    g.setTransform(dpr, 0, 0, dpr, 0, 0);
  }

  // ---------- step: move everything toward where it wants to be; say whether anything still moves ----------
  let lastScroll = -1, spin = 0, sceneMoving = false;
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
    sceneMoving = busy;
    if (stepDust(dt, still)) busy = true;
    return busy;
  }

  // ---------- draw ----------
  const mix = (a, b, t) => Math.round(a + (b - a) * t);
  // A disc with a rim that fades over its blur: `mid` at the centre to `rim` at its edge, then a highlight
  // toward (lx, ly) for a lit sphere.
  function disc(x, y, r, b, mid, rim, hi, lx, ly) {
    const R = r + b, e = r / (r + 0.6 * b);
    const gr = g.createRadialGradient(x, y, 0, x, y, R);
    gr.addColorStop(0, `rgba(${mid},${e})`);
    gr.addColorStop(Math.max(0, r - b) / R, `rgba(${rim},${e})`);
    gr.addColorStop(1, `rgba(${rim},0)`);
    g.fillStyle = gr; g.beginPath(); g.arc(x, y, R, 0, TAU); g.fill();
    if (!hi) return;
    const hr = r * 0.95;
    const h = g.createRadialGradient(x + lx * r, y + ly * r, 0, x + lx * r * 0.6, y + ly * r * 0.6, hr);
    h.addColorStop(0, `rgba(${hi},${0.95 * e})`);
    h.addColorStop(1, `rgba(${hi},0)`);
    g.fillStyle = h; g.beginPath(); g.arc(x, y, r, 0, TAU); g.fill();
  }

  // One run of a ring, from b0 to b1 degrees, as a single shape: the band's two edges (and round ends where the
  // arc itself ends), each point pushed out from the centre line by `grow` times the blur at that point.
  // The soft edge is built from shapes laid over each other, widest and faintest first, close enough together
  // that no band shows between them.
  const PASSES = [[2.1, 0.05], [1.8, 0.07], [1.5, 0.09], [1.25, 0.11], [1.0, 0.13], [0.78, 0.16], [0.58, 0.2], [0.4, 0.24], [0.22, 0.3], [0, 1]];
  const MAXP = 80;
  const cx_ = new Float32Array(MAXP), cy_ = new Float32Array(MAXP), ox_ = new Float32Array(MAXP), oy_ = new Float32Array(MAXP),
    ix_ = new Float32Array(MAXP), iy_ = new Float32Array(MAXP), bl_ = new Float32Array(MAXP);
  function run(gR, b0, b1, capA, capB, col) {
    const w = W * (0.24 + 0.76 * cam.flat); // a fine orbit line in 3D, the mark's weight once flat
    const n = Math.min(MAXP - 1, Math.max(2, Math.ceil((b1 - b0) / 2.5)));
    let sum = 0, wpx = 0;
    for (let j = 0; j <= n; j++) {
      const a = (b0 + ((b1 - b0) * j) / n) * D2R, c = Math.cos(a), sn = -Math.sin(a);
      proj(gR.r * c, 0, gR.r * sn, P); cx_[j] = P.X; cy_[j] = P.Y; bl_[j] = Math.abs(coc(P.depth)); sum += bl_[j]; wpx += w * P.s;
      proj((gR.r + w / 2) * c, 0, (gR.r + w / 2) * sn, P); ox_[j] = P.X; oy_[j] = P.Y;
      proj((gR.r - w / 2) * c, 0, (gR.r - w / 2) * sn, P); ix_[j] = P.X; iy_[j] = P.Y;
    }
    const avg = sum / (n + 1); wpx /= n + 1;
    const energy = wpx / (wpx + 0.9 * avg); // a blurred line spreads its light, so it's fainter
    // Push point (x, y) away from the centre line's point j by e px, along its own direction or, if it sits on
    // the centre line (the band seen edge-on), across the line.
    const out = (x, y, j, e, side) => {
      let dx = x - cx_[j], dy = y - cy_[j], d = Math.hypot(dx, dy);
      if (d < 0.3) {
        const k = Math.min(n, j + 1), m = Math.max(0, j - 1);
        const tx = cx_[k] - cx_[m], ty = cy_[k] - cy_[m], tl = Math.hypot(tx, ty) || 1;
        dx = (-ty / tl) * side; dy = (tx / tl) * side; d = 1;
      }
      g.lineTo(x + (dx / d) * e, y + (dy / d) * e);
    };
    const cap = (deg, j, dir, e) => {
      const ux = Math.cos(deg * D2R), uy = Math.sin(deg * D2R), tx = -uy * dir, ty = ux * dir;
      for (let q = 1; q < 6; q++) {
        const ph = (Math.PI * (dir > 0 ? q : 6 - q)) / 6;
        proj(gR.r * ux + (w / 2) * (ux * Math.cos(ph) + tx * Math.sin(ph)), 0, -(gR.r * uy + (w / 2) * (uy * Math.cos(ph) + ty * Math.sin(ph))), P);
        out(P.X, P.Y, j, e, 1);
      }
    };
    g.fillStyle = col;
    for (const [grow, alpha] of PASSES) {
      if (grow && avg < 0.25) continue;
      g.globalAlpha = grow ? alpha : Math.min(1, energy + 0.15) * (avg > 0.25 ? 0.5 : 1);
      g.beginPath();
      for (let j = 0; j <= n; j++) out(ox_[j], oy_[j], j, bl_[j] * grow, 1);
      if (capB) cap(b1, n, 1, bl_[n] * grow);
      for (let j = n; j >= 0; j--) out(ix_[j], iy_[j], j, bl_[j] * grow, -1);
      if (capA) cap(b0, 0, -1, bl_[0] * grow);
      g.closePath(); g.fill();
    }
    g.globalAlpha = 1;
  }

  const bodies = [];
  let shownDim = -1;
  function draw() {
    if (Math.abs(T.dim - shownDim) > 0.005) { shownDim = T.dim; sky.style.opacity = T.dim.toFixed(3); }
    g.globalAlpha = 1;
    g.clearRect(0, 0, vw, vh);
    const flat = cam.flat;

    for (const st of STARS) {
      proj(st.x, st.y, st.z, P);
      if (P.depth < 0.6 || P.X < -20 || P.X > vw + 20 || P.Y < -20 || P.Y > vh + 20) continue;
      const r0 = st.s * 1.6, r = r0 + Math.min(4, Math.abs(coc(P.depth)));
      g.globalAlpha = st.a * (1 - 0.6 * flat) * (r0 / r);
      g.drawImage(DOT, P.X - r, P.Y - r, r * 2, r * 2);
    }
    g.globalAlpha = 1;

    proj(0, 0, 0, P);
    const sunX = P.X, sunY = P.Y, sunS = P.s, sunD = P.depth;
    {
      const R = sunS * 0.9, gr = g.createRadialGradient(sunX, sunY, 0, sunX, sunY, R);
      gr.addColorStop(0, `rgba(${AMBER},${0.2 * (1 - flat)})`); gr.addColorStop(1, `rgba(${AMBER},0)`);
      g.fillStyle = gr; g.beginPath(); g.arc(sunX, sunY, R, 0, TAU); g.fill();
    }

    // Rings: slate-mist when seen in 3D, mist once it's the flat mark.
    const t = 0.36 + 0.64 * flat;
    const col = `rgb(${mix(47, 238, t)},${mix(74, 241, t)},${mix(87, 245, t)})`;
    for (const gR of RINGS) {
      for (let a = 0; a < 4; a++) {
        const a0 = gR.from + a * 90 + gR.turn;
        run(gR, a0, a0 + gR.span, true, true, col);
      }
    }

    // The sun and the planets, far to near.
    bodies.length = 0;
    bodies.push({ sun: true, d: sunD });
    for (const p of PLANETS) if (p.rad >= 0.3) bodies.push({ p, d: p.depth });
    bodies.sort((a, b) => b.d - a.d);
    // Grains go into layers between the bodies (a counting sort): layer L is nearer than the first L bodies,
    // so it is drawn after them and before the rest.
    const nb = bodies.length;
    dStart.fill(0);
    for (let i = 0; i < dustN; i++) {
      let L = 0;
      while (L < nb && bodies[L].d > dD[i]) L++;
      dLay[i] = L; dStart[L + 1]++;
    }
    for (let L = 1; L <= nb + 1; L++) dStart[L] += dStart[L - 1];
    dCur.set(dStart);
    for (let i = 0; i < dustN; i++) dOrder[dCur[dLay[i]]++] = i;
    for (let L = 0; L <= nb; L++) {
      if (dustA) { drawDust(L); g.globalAlpha = 1; }
      if (L === nb) break;
      const b = bodies[L];
      if (b.sun) {
        disc(sunX, sunY, SUN * sunS * (0.62 + 0.38 * flat), Math.abs(coc(sunD)), AMBER, AMBER, flat > 0.9 ? null : '255,214,140', -0.3, -0.3);
        continue;
      }
      const p = b.p, X = p.X + p.ox, Y = p.Y + p.oy;
      const dx = sunX - X, dy = sunY - Y, dl = Math.hypot(dx, dy) || 1, lit = 0.5 * (1 - flat);
      disc(X, Y, p.rad, Math.abs(coc(p.depth)),
        `${mix(118, 238, flat)},${mix(138, 241, flat)},${mix(150, 245, flat)}`,
        `${mix(52, 238, flat)},${mix(72, 241, flat)},${mix(84, 245, flat)}`,
        flat > 0.9 ? null : '226,232,238', (dx / dl) * lit, (dy / dl) * lit);
    }

    for (const p of PLANETS) {
      if (p.label < 0.02) { if (p.shown) { p.el.style.opacity = '0'; p.shown = false; } continue; }
      p.shown = true;
      p.el.style.opacity = p.label.toFixed(3);
      p.el.style.transform = `translate3d(${(p.X + p.ox + p.rad + 8).toFixed(1)}px,${(p.Y + p.oy).toFixed(1)}px,0) translateY(-50%)`;
      p.el.classList.toggle('is-chosen', p.name === chosen);
    }
  }

  // ---------- loop: runs only while something moves (or dust drifts) ----------
  const reduce = matchMedia('(prefers-reduced-motion: reduce)');
  let raf = 0, last = 0, drawnAt = 0, nudged = true, t0 = 0;
  // Debug hook (see the top): a simulated pointer sweep, and a frame timer. Headless Chrome may never fire
  // requestAnimationFrame, so a sweep is driven by timers, which its virtual time does advance.
  const SWEEP = Q.has('px') ? { x: qn('px', 0.5), y: qn('py', 0.5), vx: qn('pvx', 0), vy: qn('pvy', 0), hold: qn('hold', 150) / 1000 } : null;
  const BENCH = qn('bench', 0), FLUSH = Q.get('flush') === '1';
  const later = SWEEP ? (f) => setTimeout(() => f(performance.now()), 16) : requestAnimationFrame;
  const unlater = SWEEP ? clearTimeout : cancelAnimationFrame;
  let halted = false;
  function frame(now) {
    raf = 0;
    // When only the slow drift moves, half the frame rate is plenty.
    if (!nudged && !sceneMoving && dustCalm && now - drawnAt < 30) { raf = later(frame); return; }
    nudged = false; drawnAt = now;
    const dt = Math.min(0.1, last && now > last ? (now - last) / 1000 : 1 / 60);
    last = now;
    if (SWEEP && sweepAt((now - t0) / 1000)) { halted = true; return; }
    const busy = step(dt, false);
    draw();
    if (busy) kick(); else last = 0;
  }
  // Places the simulated pointer for time t (s since start); true once the scene should hold still.
  function sweepAt(t) {
    t -= 1.2;
    if (t >= 0) {
      const k = Math.min(t, 0.5) - 0.5;
      const x = SWEEP.x * vw + SWEEP.vx * k, y = SWEEP.y * vh + SWEEP.vy * k;
      pointer.x = x; pointer.y = y; pointer.in = true; pointer.mouse = true;
      dustAt(x, y);
    }
    return t > 0.5 + SWEEP.hold;
  }
  // Times n frames of step + draw back to back (the first 10 warm up), with a pointer circling the system so
  // the dust physics runs too; &flush=1 also waits for each frame's raster.
  function bench(n) {
    let sum = 0, worst = 0;
    for (let f = 0; f < n + 10; f++) {
      const a = (f / 60) * 4;
      dustAt(cam.x + Math.cos(a) * cam.R * 0.8, cam.y + Math.sin(a) * cam.R * 0.3);
      const b0 = performance.now();
      step(1 / 60, false); draw();
      if (FLUSH) g.getImageData(0, 0, 1, 1);
      const ms = performance.now() - b0;
      if (f >= 10) { sum += ms; worst = Math.max(worst, ms); }
    }
    document.title = `bench frames=${n} mean=${(sum / n).toFixed(3)}ms max=${worst.toFixed(2)}ms grains=${dustN}`;
  }
  function kick() {
    nudged = true;
    if (!raf && !frozen && !halted && !document.hidden) raf = later(frame);
  }
  function still() {
    if (raf) unlater(raf);
    raf = 0; last = 0;
    measure();
    step(0, true); draw();
  }
  function start() {
    frozen = reduce.matches;
    document.documentElement.classList.toggle('live', !frozen);
    document.documentElement.classList.toggle('still', frozen);
    measure();
    if (PIN != null && stops[PIN | 0] && Q.get('bare') !== '1') { const a = stops[PIN | 0], b = stops[Math.min(stops.length - 1, (PIN | 0) + 1)]; scrollTo(0, a.at + (b.at - a.at) * (PIN % 1)); }
    if (frozen) { still(); return; }
    t0 = performance.now();
    step(0, true); draw();
    if (BENCH) { bench(BENCH); return; }
    kick();
  }

  addEventListener('pointermove', (e) => {
    pointer.x = e.clientX; pointer.y = e.clientY; pointer.in = true; pointer.mouse = e.pointerType === 'mouse';
    if (e.pointerType !== 'touch') dustAt(e.clientX, e.clientY); // a finger is followed by its touch events below
    kick();
  }, { passive: true });
  document.addEventListener('pointerleave', () => { pointer.in = false; dptr.in = false; kick(); });
  addEventListener('blur', () => { pointer.in = false; dptr.in = false; kick(); });
  // A finger drags the dust too. Touch events (passive) keep coming while the page scrolls under the finger,
  // where pointer events are cancelled, so scrolling is never held up.
  const touch = (e) => { const t = e.touches[0]; if (t) { dustAt(t.clientX, t.clientY); kick(); } };
  addEventListener('touchstart', touch, { passive: true });
  addEventListener('touchmove', touch, { passive: true });
  const untouch = (e) => { if (!e.touches.length) { dptr.in = false; kick(); } };
  addEventListener('touchend', untouch, { passive: true });
  addEventListener('touchcancel', untouch, { passive: true });
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
