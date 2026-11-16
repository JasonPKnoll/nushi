// The hero's orrery: the mark (docs/brand/mark.svg) in 3D. Its amber sun at the centre, its broken rings (four
// arcs each) lying in a plane, and on them the agents Nushi runs as planets, lit from the sun's side and named
// when the cursor comes near. It's drawn on three stacked canvases, far, in focus and near, the far one a little
// blurred and the near one more (CSS filters), so it has a depth of field. The cursor tilts the view at once;
// scrolling turns the camera to look down on the plane, until it's the mark seen flat, and moves stars at
// every depth at their own speed. Plain canvas, no library. One still frame under Reduce Motion; paused off screen.
(() => {
  const box = document.querySelector('.field');
  if (!box) return;
  const layer = (name) => {
    const c = document.createElement('canvas');
    c.className = name;
    box.append(c);
    return c.getContext('2d');
  };
  const far = layer('far'), mid = layer('mid'), near = layer('near');
  if (!mid) return;
  const all = [far, mid, near];
  const still = matchMedia('(prefers-reduced-motion: reduce)').matches;
  const MIST = '238,241,245';
  const AMBER = '245,165,36';

  // Rings by radius (the outer is 1); planets by ring: name, start angle, size.
  const rings = [
    { r: 0.42, planets: [['Claude', 0.4, 7], ['Pi', 3.6, 5]] },
    { r: 0.7, planets: [['Codex', 1.2, 7.5], ['OpenCode', 3.3, 6], ['PiG', 5.2, 5]] },
    { r: 1.0, planets: [['Ante', 2.2, 7], ['oh-my-pi', 5.6, 6]] },
  ];
  rings.forEach((g) => { g.speed = 0.035 / g.r ** 1.5; g.turn = Math.random() * 6.28; });

  const rnd = (a, b) => a + Math.random() * (b - a);
  const stars = Array.from({ length: innerWidth < 700 ? 40 : 90 }, () => ({
    x: rnd(-0.1, 1.1), y: rnd(-0.1, 1.1), z: rnd(0, 1), // z: 0 near (big, soft, quick), 1 far (tiny, slow)
  }));

  let w = 0, h = 0, S = 1, cx = 0, cy0 = 0;
  function size() {
    const dpr = Math.min(devicePixelRatio || 1, 2);
    const r = box.getBoundingClientRect();
    w = r.width; h = r.height;
    for (const c of all) {
      c.canvas.width = Math.round(w * dpr); c.canvas.height = Math.round(h * dpr);
      c.setTransform(dpr, 0, 0, dpr, 0, 0);
    }
    const text = document.querySelector('.hero-text')?.getBoundingClientRect();
    if (w > 820 && text) {
      const left = Math.max(16, (w - 1180) / 2);
      S = Math.min(w * 0.22, 330);
      cx = Math.min(left + 1180 * 0.79, w - S * 0.8);
      cy0 = text.top - r.top + text.height * 0.5;
    } else {
      S = Math.min(w * 0.4, 170);
      cx = w / 2;
      cy0 = 150;
    }
  }

  const mouse = { x: 0, y: 0, in: false };
  addEventListener('pointermove', (e) => {
    const r = box.getBoundingClientRect();
    mouse.x = e.clientX - r.left; mouse.y = e.clientY - r.top;
    mouse.in = mouse.y > 0 && mouse.y < h;
  }, { passive: true });
  document.addEventListener('pointerleave', () => { mouse.in = false; });

  let t = 0, yaw = 0, tilt = 0.4, scrolled = 0, cy = 0;
  // A point on the ring plane at radius r and angle a: screen x, y, depth z (-1 near … 1 far), scale p.
  function place(r, a) {
    const x0 = r * Math.cos(a + yaw), z0 = r * Math.sin(a + yaw);
    const y = -z0 * Math.sin(tilt), z = z0 * Math.cos(tilt);
    const p = 2.4 / (2.4 + z);
    return { x: cx + x0 * S * p, y: cy + y * S * p, z, p };
  }
  const layerOf = (z) => (z > 0.3 ? far : z < -0.4 ? near : mid);

  function step(dt) {
    t += dt;
    const rect = box.getBoundingClientRect();
    scrolled = Math.max(0, -rect.top);
    const k = Math.min(1, scrolled / (innerHeight * 0.9)); // 0 at the top, 1 a screen down
    const tx = mouse.in ? mouse.x / w - 0.5 : 0;
    const ty = mouse.in ? mouse.y / innerHeight - 0.5 : 0;
    const ease = 1 - Math.exp(-dt * 12);
    yaw += (tx * 0.35 + k * 0.5 - yaw) * ease;
    tilt += (0.42 + k * 0.95 + ty * 0.2 - tilt) * ease; // scrolling turns the camera to look straight down
    cy = cy0 + scrolled * 0.45;                         // and the orrery falls behind the page, slower
    for (const g of rings) g.turn += g.speed * dt;
  }

  function ring(g) {
    for (let k = 0; k < 4; k++) {
      const from = g.turn * 0.5 + k * (Math.PI / 2) + 0.16, span = Math.PI / 2 - 0.32;
      let prev = place(g.r, from);
      for (let i = 1; i <= 30; i++) {
        const q = place(g.r, from + (span * i) / 30);
        const c = layerOf((q.z + prev.z) / 2);
        c.strokeStyle = `rgba(${MIST},${0.07 + 0.12 * (1 - q.z) / 2})`;
        c.lineWidth = 1.8 * q.p;
        c.lineCap = 'round';
        c.beginPath(); c.moveTo(prev.x, prev.y); c.lineTo(q.x, q.y); c.stroke();
        prev = q;
      }
    }
  }

  // A planet as a small sphere lit from the sun: its bright side faces the centre.
  function planet(c, q, r) {
    const dx = cx - q.x, dy = cy - q.y, d = Math.hypot(dx, dy) || 1;
    const hx = q.x + (dx / d) * r * 0.5, hy = q.y + (dy / d) * r * 0.5;
    const g = c.createRadialGradient(hx, hy, r * 0.1, q.x, q.y, r);
    g.addColorStop(0, `rgba(${MIST},0.85)`);
    g.addColorStop(0.7, 'rgba(120,145,158,0.55)');
    g.addColorStop(1, 'rgba(47,74,87,0.35)');
    c.fillStyle = g;
    c.beginPath(); c.arc(q.x, q.y, r, 0, 6.2832); c.fill();
  }

  function sun() {
    const R = S * 0.5;
    const glow = mid.createRadialGradient(cx, cy, 0, cx, cy, R);
    glow.addColorStop(0, `rgba(${AMBER},0.38)`);
    glow.addColorStop(0.18, `rgba(${AMBER},0.14)`);
    glow.addColorStop(1, `rgba(${AMBER},0)`);
    mid.fillStyle = glow;
    mid.beginPath(); mid.arc(cx, cy, R, 0, 6.2832); mid.fill();
    mid.fillStyle = `rgba(${AMBER},0.9)`;
    mid.beginPath(); mid.arc(cx, cy, Math.max(6, S * 0.045), 0, 6.2832); mid.fill();
  }

  function draw() {
    for (const c of all) c.clearRect(0, 0, w, h);
    for (const s of stars) {
      const c = s.z > 0.55 ? far : s.z < 0.2 ? near : mid;
      const x = s.x * w - yaw * (1 - s.z) * 120;
      const y = s.y * h * 0.8 - scrolled * (1 - s.z) * 0.5 - (tilt - 0.42) * (1 - s.z) * 60;
      c.fillStyle = `rgba(${MIST},${0.08 + 0.22 * s.z})`;
      c.beginPath(); c.arc(x, y, 0.7 + (1 - s.z) * 2.2, 0, 6.2832); c.fill();
    }
    for (const g of rings) ring(g);
    // Planets far to near; the sun among them by depth, so it hides what passes behind it.
    const bodies = [];
    for (const g of rings) for (const [name, at, size] of g.planets) {
      const q = place(g.r, at + g.turn);
      bodies.push({ q, name, r: size * q.p * (S / 330) ** 0.5 });
    }
    bodies.sort((a, b) => b.q.z - a.q.z);
    let sunDrawn = false;
    for (const b of bodies) {
      if (!sunDrawn && b.q.z <= 0) { sun(); sunDrawn = true; }
      planet(layerOf(b.q.z), b.q, b.r);
    }
    if (!sunDrawn) sun();
    mid.font = '500 13px Inter, ui-sans-serif, system-ui, sans-serif';
    mid.textBaseline = 'middle';
    for (const b of bodies) {
      const d = mouse.in ? Math.hypot(b.q.x - mouse.x, b.q.y - mouse.y) : 1e9;
      const lit = Math.max(0, 1 - d / 160);
      if (lit <= 0) continue;
      mid.fillStyle = `rgba(${MIST},${0.75 * lit})`;
      mid.fillText(b.name, b.q.x + b.r + 8, b.q.y);
    }
  }

  let running = false, visible = true, last = 0;
  function frame(now) {
    const dt = Math.min(0.033, (now - last) / 1000);
    last = now;
    step(dt); draw();
    if (running) requestAnimationFrame(frame);
  }
  function go() {
    const want = !still && visible && !document.hidden;
    if (want && !running) { running = true; last = performance.now(); requestAnimationFrame(frame); }
    if (!want) running = false;
  }

  size(); step(1); draw();
  addEventListener('resize', () => { size(); step(1); draw(); });
  document.fonts?.ready.then(() => { size(); draw(); });
  new IntersectionObserver(([e]) => { visible = e.isIntersecting; go(); }).observe(box);
  document.addEventListener('visibilitychange', go);
  go();
})();
