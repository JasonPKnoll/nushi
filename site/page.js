// The page around the orrery: copy buttons, the top bar's backdrop, the tour's screenshot, and the few things
// scrolling moves (the big screenshot standing up as it arrives, the agents' names sliding sideways).
// Screenshots lean a degree or two toward the cursor, and go flat while the pointer is on them.
// One rAF loop that runs only while something is easing, then stops. Rect reads come before writes in a frame.
// Without JS or under Reduce Motion the page is plain and still; nothing waits on an animation to show.
(() => {
  const root = document.documentElement;
  const $ = (s, el = document) => el.querySelector(s);
  const $$ = (s, el = document) => [...el.querySelectorAll(s)];
  const clamp = (v, a, b) => Math.max(a, Math.min(b, v));

  // Copy buttons.
  for (const b of $$('[data-copy]')) {
    b.addEventListener('click', async () => {
      const label = $('.copied', b);
      try { await navigator.clipboard.writeText(b.dataset.copy); label.textContent = 'Copied'; }
      catch { label.textContent = 'Select it'; }
      setTimeout(() => { label.textContent = 'Copy'; }, 1600);
    });
  }

  // The top bar is see-through at the very top and gets its backdrop once anything can run under it.
  const bar = $('.bar');
  const barState = () => bar.classList.toggle('at-top', scrollY < 8);
  addEventListener('scroll', barState, { passive: true });
  barState();

  // Tour: the step crossing 45% of the screen picks the screenshot (or the one clicked or focused).
  const steps = $$('.step');
  const shots = $$('.frame-imgs img');
  let on = 0;
  const show = (i) => {
    if (i === on) return;
    steps.forEach((s, j) => s.classList.toggle('is-on', j === i));
    shots.forEach((s, j) => {
      s.classList.toggle('was', j === on);
      s.classList.toggle('is-on', j === i);
    });
    on = i;
  };
  steps.forEach((s, i) => {
    const b = $('.step-b', s);
    b.addEventListener('click', () => show(i));
    b.addEventListener('focus', () => show(i));
  });
  let picking = false;
  const pick = () => {
    picking = false;
    const line = innerHeight * 0.45;
    let i = 0;
    steps.forEach((s, j) => { if (s.getBoundingClientRect().top < line) i = j; });
    show(i);
  };
  addEventListener('scroll', () => { if (!picking) { picking = true; requestAnimationFrame(pick); } }, { passive: true });
  if (steps.length) pick();

  if (!root.classList.contains('motion')) return;

  // ---------- Motion ----------
  let W = innerWidth, H = innerHeight;
  const hero = $('.hero-shot img');
  const roster = $('.roster'), track = $('.track');
  const leaners = $$('.frame, .phone-shot img, .hero-shot img').map((el) => ({ el, rx: 0, ry: 0, trx: 0, try: 0, over: false }));
  const finePointer = matchMedia('(hover: hover) and (pointer: fine)');
  let mx = 0, my = 0, raf = 0, lastT = 0, lastScroll = -1;
  const near = (a, b) => Math.abs(a - b) < 0.002;

  function frame(t) {
    raf = 0;
    const dt = lastT ? Math.min(0.05, (t - lastT) / 1000) : 1 / 60;
    lastT = t;
    const k = 1 - Math.exp(-dt * 18);
    const sy = scrollY;
    let busy = sy !== lastScroll;
    lastScroll = sy;

    // Read.
    const hr = hero && hero.getBoundingClientRect();
    const rr = roster && roster.getBoundingClientRect();
    const trackOver = track ? track.scrollWidth - W : 0;

    // Write. The big screenshot lies back and stands up as it comes into view.
    for (const L of leaners) {
      const tx = L.over || !finePointer.matches ? 0 : -my * 1.5, ty = L.over || !finePointer.matches ? 0 : mx * 2;
      L.rx += (tx - L.rx) * k; L.ry += (ty - L.ry) * k;
      if (!near(L.rx, tx) || !near(L.ry, ty)) busy = true;
      let lie = 0;
      if (L.el === hero && hr) lie = 14 * (1 - clamp((H - hr.top) / (H * 0.75), 0, 1));
      L.el.style.transform = `perspective(1800px) rotateX(${(lie + L.rx).toFixed(3)}deg) rotateY(${L.ry.toFixed(3)}deg)`;
    }
    // The agents' names slide sideways as the section passes.
    if (rr && rr.bottom > 0 && rr.top < H && trackOver > 0) {
      const prog = clamp((H - rr.top) / (H + rr.height), 0, 1);
      track.style.transform = `translate3d(${(-prog * trackOver).toFixed(1)}px,0,0)`;
    }
    if (busy) kick(); else lastT = 0;
  }
  function kick() { if (!raf && !document.hidden) raf = requestAnimationFrame(frame); }

  addEventListener('scroll', kick, { passive: true });
  addEventListener('pointermove', (e) => {
    if (e.pointerType === 'touch') return;
    mx = clamp(e.clientX / W * 2 - 1, -1, 1);
    my = clamp(e.clientY / H * 2 - 1, -1, 1);
    kick();
  }, { passive: true });
  document.addEventListener('pointerleave', () => { mx = my = 0; kick(); });
  for (const L of leaners) {
    L.el.addEventListener('pointerenter', () => { L.over = true; kick(); });
    L.el.addEventListener('pointerleave', () => { L.over = false; kick(); });
  }
  addEventListener('resize', () => { W = innerWidth; H = innerHeight; kick(); });
  document.addEventListener('visibilitychange', kick);
  addEventListener('load', kick);
  kick();
})();
