// "Try it" in the Changes chapter: a few lines of a diff you can comment on, then send the comments to the agent
// as one message, as the Changes tab does. It's a small mock; the screenshot beside it is the real app.
(() => {
  const box = document.querySelector('.demo');
  if (!box) return;
  const diff = box.querySelector('.demo-diff');
  const count = box.querySelector('.demo-count');
  const send = box.querySelector('.demo-send');
  const foot = box.querySelector('.demo-foot');
  const comments = new Map(); // line button -> text
  let draft = null;

  const esc = (s) => s.replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
  function refresh() {
    const n = comments.size;
    count.textContent = n ? `${n} comment${n > 1 ? 's' : ''}` : 'Click a line to comment';
    send.disabled = !n;
    send.textContent = n ? `Send ${n} to Claude` : 'Send to Claude';
  }
  function closeDraft() { draft?.remove(); draft = null; }
  function open(line) {
    closeDraft();
    const form = document.createElement('div');
    form.className = 'cmt';
    form.innerHTML = `<textarea aria-label="Comment on line ${line.dataset.n}" rows="2" placeholder="Leave a comment for the agent">${esc(comments.get(line)?.text || '')}</textarea><div class="row"><button type="button" class="btn-grey" data-act="cancel">Cancel</button><button type="button" class="btn-blue" data-act="add">Comment</button></div>`;
    (comments.get(line)?.el || line).after(form);
    if (comments.get(line)) comments.get(line).el.hidden = true;
    form.line = line;
    draft = form;
    const ta = form.querySelector('textarea');
    ta.focus({ preventScroll: true });
    ta.addEventListener('keydown', (e) => {
      if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); form.querySelector('[data-act="add"]').click(); }
      if (e.key === 'Escape') { form.querySelector('[data-act="cancel"]').click(); line.focus(); }
    });
  }
  diff.addEventListener('click', (e) => {
    const line = e.target.closest('.dl');
    if (line) { open(line); return; }
    const act = e.target.closest('[data-act]')?.dataset.act;
    if (!act || !draft) return;
    const line2 = draft.line;
    const old = comments.get(line2);
    if (act === 'add') {
      const v = draft.querySelector('textarea').value.trim();
      old?.el.remove();
      comments.delete(line2);
      if (v) {
        const el = document.createElement('div');
        el.className = 'cmt';
        el.innerHTML = `<div class="who">You · line ${line2.dataset.n}</div>${esc(v)}`;
        line2.after(el);
        comments.set(line2, { text: v, el });
      }
    } else if (old) old.el.hidden = false;
    closeDraft();
    refresh();
  });
  send.addEventListener('click', () => {
    const n = comments.size;
    if (!n) return;
    for (const c of comments.values()) c.el.remove();
    comments.clear();
    closeDraft();
    refresh();
    const sent = document.createElement('div');
    sent.className = 'demo-sent';
    sent.setAttribute('role', 'status');
    sent.innerHTML = `<p class="you">${n} review comment${n > 1 ? 's' : ''} on feed/handler.go</p><p class="agent"><span class="ag">Cl</span>On it. ${n > 1 ? `Going through all ${n}.` : 'Fixing that line.'}</p><button type="button" class="btn-grey">Again</button>`;
    foot.hidden = true;
    box.append(sent);
    sent.querySelector('button').addEventListener('click', () => { sent.remove(); foot.hidden = false; diff.querySelector('.dl').focus(); });
  });
  refresh();
})();
