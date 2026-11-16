// The hero's background: a disc of linked nodes, like the mark's orbits seen at an angle, turning slowly in 3D.
// The cursor tilts it and pushes nearby nodes aside, and they spring back; amber sparks run along the links,
// messages between agents. Plain canvas, no library. Still (one frame) under Reduce Motion, paused off screen.
(() => {
  const canvas = document.querySelector('.field');
  if (!canvas || !canvas.getContext) return;
  const ctx = canvas.getContext('2d');
  const still = matchMedia('(prefers-reduced-motion: reduce)').matches;

  const MIST = '238,241,245';
  const AMBER = '245,165,36';
  const N = innerWidth < 700 ? 150 : 300;

  // Nodes on a thin disc, denser toward the middle; a few are amber.
  const nodes = [];
  for (let i = 0; i < N; i++) {
    const r = Math.sqrt(0.2 + Math.random() * 0.8); // an open middle, where the headline sits
    const a = Math.random() * Math.PI * 2;
    const g = (Math.random() + Math.random() + Math.random() - 1.5) * 0.06;
    nodes.push({
      x: Math.cos(a) * r, y: g, z: Math.sin(a) * r,
      ox: 0, oy: 0, vx: 0, vy: 0, // the cursor's push, in screen pixels
      sx: 0, sy: 0, s: 1, d: 0,   // projected place, scale, depth (0 far, 1 near)
      amber: Math.random() < 0.06,
    });
  }

  // Each node links to its 3 nearest, once.
  const links = [];
  const seen = new Set();
  nodes.forEach((n, i) => {
    nodes
      .map((m, j) => [j, (n.x - m.x) ** 2 + (n.y - m.y) ** 2 + (n.z - m.z) ** 2])
      .filter(([j]) => j !== i)
      .sort((p, q) => p[1] - q[1])
      .slice(0, 3)
      .forEach(([j]) => {
        const key = i < j ? i * N + j : j * N + i;
        if (!seen.has(key)) { seen.add(key); links.push([i, j]); }
      });
  });

  let w = 0, h = 0, dpr = 1;
  function size() {
    dpr = Math.min(devicePixelRatio || 1, 2);
    const box = canvas.getBoundingClientRect();
    w = box.width; h = box.height;
    canvas.width = Math.round(w * dpr); canvas.height = Math.round(h * dpr);
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  }

  // The cursor, relative to the canvas; tilt eases toward it.
  const mouse = { x: -1e4, y: -1e4, in: false };
  let tiltX = 0, tiltY = 0;
  addEventListener('pointermove', (e) => {
    const box = canvas.getBoundingClientRect();
    mouse.x = e.clientX - box.left; mouse.y = e.clientY - box.top;
    mouse.in = mouse.y > 0 && mouse.y < h;
  }, { passive: true });
  document.addEventListener('pointerleave', () => { mouse.in = false; });

  const sparks = [];
  let spin = 0.6, last = performance.now(), nextSpark = 0;

  function project() {
    const cx = w / 2, cy = Math.min(h * 0.3, 330);
    const R = Math.min(w * 0.55, 820);
    const yaw = spin + tiltX * 0.35;
    const pitch = 0.42 + tiltY * 0.25; // the disc seen from a little above
    const cyw = Math.cos(yaw), syw = Math.sin(yaw), cp = Math.cos(pitch), sp = Math.sin(pitch);
    for (const n of nodes) {
      const x1 = n.x * cyw - n.z * syw;
      const z1 = n.x * syw + n.z * cyw;
      const y2 = n.y * cp - z1 * sp;
      const z2 = n.y * sp + z1 * cp;
      const s = 2.2 / (2.2 + z2);
      n.s = s; n.d = (1 - z2) / 2;
      n.sx = cx + x1 * R * s + n.ox;
      n.sy = cy + y2 * R * s + n.oy;
    }
  }

  function step(dt) {
    spin += dt * 0.035;
    const tx = mouse.in ? (mouse.x / w - 0.5) * 2 : 0;
    const ty = mouse.in ? (mouse.y / h - 0.5) * 2 : 0;
    tiltX += (tx - tiltX) * Math.min(1, dt * 2);
    tiltY += (ty - tiltY) * Math.min(1, dt * 2);
    const reach = 140;
    for (const n of nodes) {
      let fx = -n.ox * 18, fy = -n.oy * 18; // spring home
      if (mouse.in) {
        const dx = n.sx - mouse.x, dy = n.sy - mouse.y;
        const dist = Math.hypot(dx, dy);
        if (dist < reach && dist > 0.1) {
          const push = (1 - dist / reach) ** 2 * 5200 * n.s;
          fx += (dx / dist) * push; fy += (dy / dist) * push;
        }
      }
      n.vx = (n.vx + fx * dt) * Math.pow(0.02, dt); // damped
      n.vy = (n.vy + fy * dt) * Math.pow(0.02, dt);
      n.ox += n.vx * dt; n.oy += n.vy * dt;
    }
    nextSpark -= dt;
    if (nextSpark <= 0 && sparks.length < 14) {
      const [a, b] = links[(Math.random() * links.length) | 0];
      sparks.push({ a, b, t: 0, hops: 2 + ((Math.random() * 4) | 0) });
      nextSpark = 0.18 + Math.random() * 0.3;
    }
    for (let i = sparks.length - 1; i >= 0; i--) {
      const k = sparks[i];
      k.t += dt * 1.6;
      if (k.t < 1) continue;
      // Hop on to a link of the node it reached, or fade out.
      const out = links.filter(([p, q]) => (p === k.b && q !== k.a) || (q === k.b && p !== k.a));
      if (--k.hops <= 0 || !out.length) { sparks.splice(i, 1); continue; }
      const [p, q] = out[(Math.random() * out.length) | 0];
      k.a = k.b; k.b = p === k.b ? q : p; k.t = 0;
    }
  }

  function draw() {
    ctx.clearRect(0, 0, w, h);
    ctx.lineWidth = 1;
    for (const [i, j] of links) {
      const a = nodes[i], b = nodes[j];
      const alpha = 0.1 + 0.24 * ((a.d + b.d) / 2);
      ctx.strokeStyle = `rgba(${MIST},${alpha})`;
      ctx.beginPath(); ctx.moveTo(a.sx, a.sy); ctx.lineTo(b.sx, b.sy); ctx.stroke();
    }
    for (const n of nodes) {
      const r = (n.amber ? 3 : 2) * n.s;
      ctx.fillStyle = n.amber ? `rgba(${AMBER},${0.5 + 0.5 * n.d})` : `rgba(${MIST},${0.35 + 0.6 * n.d})`;
      ctx.beginPath(); ctx.arc(n.sx, n.sy, r, 0, Math.PI * 2); ctx.fill();
    }
    for (const k of sparks) {
      const a = nodes[k.a], b = nodes[k.b];
      const x = a.sx + (b.sx - a.sx) * k.t, y = a.sy + (b.sy - a.sy) * k.t;
      const glow = ctx.createRadialGradient(x, y, 0, x, y, 9);
      glow.addColorStop(0, `rgba(${AMBER},0.9)`);
      glow.addColorStop(1, `rgba(${AMBER},0)`);
      ctx.fillStyle = glow;
      ctx.beginPath(); ctx.arc(x, y, 9, 0, Math.PI * 2); ctx.fill();
    }
  }

  let running = false, visible = true;
  function frame(now) {
    const dt = Math.min(0.05, (now - last) / 1000);
    last = now;
    project(); step(dt); draw();
    if (running) requestAnimationFrame(frame);
  }
  function go() {
    const want = !still && visible && !document.hidden;
    if (want && !running) { running = true; last = performance.now(); requestAnimationFrame(frame); }
    if (!want) running = false;
  }

  size();
  project(); draw();
  addEventListener('resize', () => { size(); project(); draw(); });
  new IntersectionObserver(([e]) => { visible = e.isIntersecting; go(); }).observe(canvas);
  document.addEventListener('visibilitychange', go);
  go();
})();
