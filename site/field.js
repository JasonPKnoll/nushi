// The hero's background: a deep cloud of drifting nodes, linked to their neighbours while they're close, seen
// through a lens focused on the middle distance: nodes near the camera and far behind blur into soft discs, and
// only the ones in focus draw links. The cursor moves the camera (near nodes shift more than far ones) and pushes
// the nodes under it aside; they spring back. Amber sparks run along the links, messages between agents.
// Plain canvas, no library. One still frame under Reduce Motion; paused off screen.
(() => {
  const canvas = document.querySelector('.field');
  if (!canvas || !canvas.getContext) return;
  const ctx = canvas.getContext('2d');
  const still = matchMedia('(prefers-reduced-motion: reduce)').matches;

  const MIST = '238,241,245';
  const AMBER = '245,165,36';
  const N = innerWidth < 700 ? 90 : 160;
  const NEAR = 0.55, FAR = 3.4, FOCUS = 1.5; // depths from the camera
  const LINK = 0.6;                           // nodes closer than this (in the world) link

  const rnd = (a, b) => a + Math.random() * (b - a);
  const nodes = Array.from({ length: N }, () => {
    const z = rnd(NEAR, FAR);
    return {
      hx: rnd(-1.9, 1.9) * z / 1.6, hy: rnd(-1.1, 1.1) * z / 1.6, z, // spread wider with depth, filling the view
      ph: rnd(0, 6.3), sp: rnd(0.15, 0.4),                       // its own slow drift
      x: 0, y: 0, ox: 0, oy: 0, vx: 0, vy: 0,                    // where it is; the cursor's push and its speed
      sx: 0, sy: 0, blur: 0, focus: 1,
      amber: Math.random() < 0.07,
    };
  });

  let w = 0, h = 0, F = 1;
  function size() {
    const dpr = Math.min(devicePixelRatio || 1, 2);
    const box = canvas.getBoundingClientRect();
    w = box.width; h = box.height;
    canvas.width = Math.round(w * dpr); canvas.height = Math.round(h * dpr);
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    F = Math.min(w, 1500) * 0.42;
  }

  const mouse = { x: 0, y: 0, in: false };
  let camX = 0, camY = 0;
  addEventListener('pointermove', (e) => {
    const box = canvas.getBoundingClientRect();
    mouse.x = e.clientX - box.left; mouse.y = e.clientY - box.top;
    mouse.in = mouse.y > 0 && mouse.y < h;
  }, { passive: true });
  document.addEventListener('pointerleave', () => { mouse.in = false; });

  let t = 0;
  const cy = () => Math.min(h * 0.32, innerHeight * 0.45);

  function step(dt) {
    t += dt;
    // The camera follows the cursor quickly, with a little lag so it feels physical.
    const tx = mouse.in ? (mouse.x / w - 0.5) * 0.5 : 0;
    const ty = mouse.in ? (mouse.y / innerHeight - 0.5) * 0.35 : 0;
    const ease = 1 - Math.exp(-dt * 9);
    camX += (tx - camX) * ease; camY += (ty - camY) * ease;

    const reach = 150, K = 90, C = 2 * Math.sqrt(K) * 0.7; // a stiff spring, a little bounce
    for (const n of nodes) {
      n.x = n.hx + Math.sin(t * n.sp + n.ph) * 0.06;
      n.y = n.hy + Math.cos(t * n.sp * 0.8 + n.ph * 1.3) * 0.05;
      const s = F / n.z;
      const bx = w / 2 + (n.x - camX * (FAR - n.z) * 0.5) * s;
      const by = cy() + (n.y - camY * (FAR - n.z) * 0.5) * s;
      let fx = -K * n.ox - C * n.vx, fy = -K * n.oy - C * n.vy;
      if (mouse.in) {
        const dx = bx + n.ox - mouse.x, dy = by + n.oy - mouse.y;
        const d = Math.hypot(dx, dy);
        if (d < reach && d > 0.5) {
          const push = (1 - d / reach) ** 2 * 26000 * Math.min(1.6, FOCUS / n.z);
          fx += (dx / d) * push; fy += (dy / d) * push;
        }
      }
      n.vx += fx * dt; n.vy += fy * dt;
      n.ox += n.vx * dt; n.oy += n.vy * dt;
      n.sx = bx + n.ox; n.sy = by + n.oy;
      const off = Math.abs(n.z - FOCUS);
      n.blur = off * 9 * (n.z < FOCUS ? 1.8 : 1); // the near side blurs faster, as through a lens
      n.focus = Math.max(0, 1 - off / 1.0);
    }
    stepSparks(dt);
  }

  // Links: the pairs close enough in the world, both near enough to focus to draw.
  let links = [];
  function link() {
    links = [];
    for (let i = 0; i < N; i++) {
      const a = nodes[i];
      if (a.focus <= 0) continue;
      for (let j = i + 1; j < N; j++) {
        const b = nodes[j];
        if (b.focus <= 0) continue;
        const d = Math.hypot(a.x - b.x, a.y - b.y, a.z - b.z);
        if (d < LINK) links.push([i, j, 1 - d / LINK]);
      }
    }
  }

  const sparks = [];
  let nextSpark = 0;
  function stepSparks(dt) {
    nextSpark -= dt;
    if (nextSpark <= 0 && links.length && sparks.length < 8) {
      const [a, b] = links[(Math.random() * links.length) | 0];
      sparks.push({ a, b, t: 0, hops: 3 + ((Math.random() * 4) | 0) });
      nextSpark = rnd(0.25, 0.6);
    }
    for (let i = sparks.length - 1; i >= 0; i--) {
      const k = sparks[i];
      k.t += dt * 1.8;
      if (k.t < 1) continue;
      const out = links.filter(([p, q]) => (p === k.b && q !== k.a) || (q === k.b && p !== k.a));
      if (--k.hops <= 0 || !out.length) { sparks.splice(i, 1); continue; }
      const [p, q] = out[(Math.random() * out.length) | 0];
      k.a = k.b; k.b = p === k.b ? q : p; k.t = 0;
    }
  }

  function dot(x, y, r, blur, rgb, alpha) {
    if (blur < 0.6) {
      ctx.fillStyle = `rgba(${rgb},${alpha})`;
      ctx.beginPath(); ctx.arc(x, y, r, 0, 6.2832); ctx.fill();
      return;
    }
    // Out of focus: a soft disc, its light spread over a larger area.
    const R = r + blur;
    const g = ctx.createRadialGradient(x, y, 0, x, y, R);
    const a = alpha * Math.min(0.55, (r * 2) / R + 0.08);
    g.addColorStop(0, `rgba(${rgb},${a})`);
    g.addColorStop(0.55, `rgba(${rgb},${a * 0.75})`);
    g.addColorStop(1, `rgba(${rgb},0)`);
    ctx.fillStyle = g;
    ctx.beginPath(); ctx.arc(x, y, R, 0, 6.2832); ctx.fill();
  }

  function draw() {
    ctx.clearRect(0, 0, w, h);
    ctx.lineWidth = 1;
    for (const [i, j, near] of links) {
      const a = nodes[i], b = nodes[j];
      ctx.strokeStyle = `rgba(${MIST},${0.4 * near * Math.min(a.focus, b.focus)})`;
      ctx.beginPath(); ctx.moveTo(a.sx, a.sy); ctx.lineTo(b.sx, b.sy); ctx.stroke();
    }
    const order = nodes.slice().sort((a, b) => b.z - a.z); // far first
    for (const n of order) {
      const r = (n.amber ? 3.2 : 2.2) * (FOCUS / n.z) ** 0.9;
      const depth = Math.min(1, 0.45 + 0.48 * (FOCUS / n.z));
      dot(n.sx, n.sy, r, n.blur, n.amber ? AMBER : MIST, (n.amber ? 0.95 : 0.7) * depth);
    }
    for (const k of sparks) {
      const a = nodes[k.a], b = nodes[k.b];
      const x = a.sx + (b.sx - a.sx) * k.t, y = a.sy + (b.sy - a.sy) * k.t;
      const g = ctx.createRadialGradient(x, y, 0, x, y, 10);
      g.addColorStop(0, `rgba(${AMBER},0.95)`);
      g.addColorStop(1, `rgba(${AMBER},0)`);
      ctx.fillStyle = g;
      ctx.beginPath(); ctx.arc(x, y, 10, 0, 6.2832); ctx.fill();
    }
  }

  let running = false, visible = true, last = 0;
  function frame(now) {
    const dt = Math.min(0.033, (now - last) / 1000);
    last = now;
    step(dt); link(); draw();
    if (running) requestAnimationFrame(frame);
  }
  function go() {
    const want = !still && visible && !document.hidden;
    if (want && !running) { running = true; last = performance.now(); requestAnimationFrame(frame); }
    if (!want) running = false;
  }

  size(); step(0); link(); draw();
  addEventListener('resize', () => { size(); step(0); link(); draw(); });
  new IntersectionObserver(([e]) => { visible = e.isIntersecting; go(); }).observe(canvas);
  document.addEventListener('visibilitychange', go);
  go();
})();
