// The hero's background: the mark as an orrery. Its amber sun sits at the centre, its broken rings (four arcs
// each, as in docs/brand/mark.svg) lie in a plane seen from a little above, and on them the agents Nushi runs
// orbit as planets, each named. The cursor tilts the view at once; a planet's name brightens as the cursor nears
// it (names show only then). What's near the camera is a little out of focus; behind, a few stars at every depth shift with the view.
// Plain canvas, no library. One still frame under Reduce Motion; paused off screen.
(() => {
  const canvas = document.querySelector('.field');
  if (!canvas || !canvas.getContext) return;
  const ctx = canvas.getContext('2d');
  const still = matchMedia('(prefers-reduced-motion: reduce)').matches;
  const MIST = '238,241,245';
  const AMBER = '245,165,36';

  // Rings by radius (the outer is 1); planets by ring, a start angle and a size.
  const rings = [
    { r: 0.42, planets: [['Claude', 0.4, 5], ['Pi', 3.6, 4]] },
    { r: 0.7, planets: [['Codex', 1.2, 5.5], ['OpenCode', 3.3, 4.5], ['PiG', 5.2, 4]] },
    { r: 1.0, planets: [['Ante', 2.2, 5.5], ['oh-my-pi', 5.6, 4.5]] },
  ];
  rings.forEach((g) => { g.speed = 0.03 / g.r ** 1.5; g.turn = Math.random() * 6.28; });

  const rnd = (a, b) => a + Math.random() * (b - a);
  const stars = Array.from({ length: innerWidth < 700 ? 30 : 60 }, () => ({
    x: rnd(-1, 1), y: rnd(-1, 1), z: rnd(0.2, 1), // z: 1 far, small and still; 0.2 near, soft and quick
  }));

  let w = 0, h = 0, S = 1, cx = 0, cy = 0;
  function size() {
    const dpr = Math.min(devicePixelRatio || 1, 2);
    const box = canvas.getBoundingClientRect();
    w = box.width; h = box.height;
    canvas.width = Math.round(w * dpr); canvas.height = Math.round(h * dpr);
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    S = Math.min(w * 0.47, 720);
    cx = w / 2;
    // The sun rises from behind the top edge of the app's screenshot.
    const shot = document.querySelector('.hero-shot');
    cy = shot ? shot.getBoundingClientRect().top - box.top + 8 : h * 0.4;
  }

  const mouse = { x: 0, y: 0, in: false };
  addEventListener('pointermove', (e) => {
    const box = canvas.getBoundingClientRect();
    mouse.x = e.clientX - box.left; mouse.y = e.clientY - box.top;
    mouse.in = mouse.y > 0 && mouse.y < h;
  }, { passive: true });
  document.addEventListener('pointerleave', () => { mouse.in = false; });

  let t = 0, yaw = 0, tilt = 0.3;
  // A point on the ring plane at radius r and angle a, to the screen: x, y, depth (-1 near … 1 far), scale.
  function place(r, a) {
    const x0 = r * Math.cos(a + yaw), z0 = r * Math.sin(a + yaw);
    const y = -z0 * Math.sin(tilt), z = z0 * Math.cos(tilt);
    const p = 3 / (3 + z);
    return { x: cx + x0 * S * p, y: cy + y * S * p, z, p };
  }

  function step(dt) {
    t += dt;
    const tx = mouse.in ? mouse.x / w - 0.5 : 0;
    const ty = mouse.in ? mouse.y / h - 0.5 : 0;
    const ease = 1 - Math.exp(-dt * 10);
    yaw += (tx * 0.3 - yaw) * ease;
    tilt += (0.3 + ty * 0.2 - tilt) * ease;
    for (const g of rings) g.turn += g.speed * dt;
  }

  function soft(x, y, r, blur, rgb, alpha) {
    if (blur < 0.5) {
      ctx.fillStyle = `rgba(${rgb},${alpha})`;
      ctx.beginPath(); ctx.arc(x, y, r, 0, 6.2832); ctx.fill();
      return;
    }
    const R = r + blur;
    const g = ctx.createRadialGradient(x, y, 0, x, y, R);
    const a = alpha * Math.min(1, (r * 1.8) / R + 0.15);
    g.addColorStop(0, `rgba(${rgb},${a})`);
    g.addColorStop(0.6, `rgba(${rgb},${a * 0.7})`);
    g.addColorStop(1, `rgba(${rgb},0)`);
    ctx.fillStyle = g;
    ctx.beginPath(); ctx.arc(x, y, R, 0, 6.2832); ctx.fill();
  }

  // A ring's four arcs, as in the mark, turning with its planets; only the half on one side of the sun.
  function ring(g, far) {
    ctx.lineCap = 'round';
    for (let k = 0; k < 4; k++) {
      const from = g.turn * 0.5 + k * (Math.PI / 2) + 0.16, span = Math.PI / 2 - 0.32;
      let prev = null;
      for (let i = 0; i <= 28; i++) {
        const q = place(g.r, from + (span * i) / 28);
        if (prev && (q.z > 0) === far) {
          ctx.strokeStyle = `rgba(${MIST},${0.05 + 0.08 * (1 - q.z) / 2})`;
          ctx.lineWidth = 1.3 * q.p;
          ctx.beginPath(); ctx.moveTo(prev.x, prev.y); ctx.lineTo(q.x, q.y); ctx.stroke();
        }
        prev = q;
      }
    }
  }

  function planets(far) {
    ctx.font = '500 13px Inter, ui-sans-serif, system-ui, sans-serif';
    ctx.textBaseline = 'middle';
    for (const g of rings) {
      for (const [name, at, size] of g.planets) {
        const q = place(g.r, at + g.turn);
        if ((q.z > 0) !== far) continue;
        const r = size * q.p;
        const blur = Math.max(0, -q.z) * 5; // the near side of the plane is a little out of focus
        soft(q.x, q.y, r, blur, MIST, 0.3 + 0.25 * (1 - q.z) / 2);
        const d = mouse.in ? Math.hypot(q.x - mouse.x, q.y - mouse.y) : 1e9;
        const lit = Math.max(0, 1 - d / 220);
        ctx.fillStyle = `rgba(${MIST},${(0.55 * lit) * (0.6 + 0.4 * (1 - q.z) / 2)})`;
        ctx.fillText(name, q.x + r + 8, q.y);
      }
    }
  }

  function sun() {
    const glow = ctx.createRadialGradient(cx, cy, 0, cx, cy, S * 0.3);
    glow.addColorStop(0, `rgba(${AMBER},0.3)`);
    glow.addColorStop(0.25, `rgba(${AMBER},0.06)`);
    glow.addColorStop(1, `rgba(${AMBER},0)`);
    ctx.fillStyle = glow;
    ctx.beginPath(); ctx.arc(cx, cy, S * 0.3, 0, 6.2832); ctx.fill();
    ctx.fillStyle = `rgba(${AMBER},0.75)`;
    ctx.beginPath(); ctx.arc(cx, cy, Math.max(7, S * 0.018), 0, 6.2832); ctx.fill();
  }

  function draw() {
    ctx.clearRect(0, 0, w, h);
    for (const s of stars) {
      const x = cx + (s.x - yaw * (1 - s.z) * 0.6) * w * 0.6;
      const y = cy * 0.9 + (s.y - (tilt - 0.3) * (1 - s.z) * 0.8) * h * 0.55;
      soft(x, y, 1.1 + (1 - s.z) * 1.4, (1 - s.z) * 5, MIST, 0.06 + 0.16 * s.z);
    }
    for (const g of rings) ring(g, true);
    planets(true);
    sun();
    for (const g of rings) ring(g, false);
    planets(false);
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

  size(); draw();
  addEventListener('resize', () => { size(); draw(); });
  document.fonts?.ready.then(draw);
  new IntersectionObserver(([e]) => { visible = e.isIntersecting; go(); }).observe(canvas);
  document.addEventListener('visibilitychange', go);
  go();
})();
