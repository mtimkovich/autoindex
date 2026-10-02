(function () {
  const box = document.getElementById("rows");
  const rows = [].slice.call(box.children);
  const heads = [].slice.call(document.querySelectorAll("[data-k]"));
  const first = {name: 1, date: -1, size: -1}; // first click: name asc, others desc
  let sort = {k: "name", o: 1};
  try {
    const s = JSON.parse(localStorage.getItem("sort"));
    if (s && first[s.k] && (s.o === 1 || s.o === -1)) sort = s;
  } catch (e) {}

  function cmp(a, b) {
    let d = b.dataset.d - a.dataset.d; // directories stay on top
    if (d) return d;
    const f = {date: "t", size: "n"}[sort.k];
    if (f && (d = a.dataset[f] - b.dataset[f])) return d * sort.o;
    const x = a.dataset.s, y = b.dataset.s;
    if (!f && x !== y) return (x < y ? -1 : 1) * sort.o;
    return x < y ? -1 : x > y ? 1 : 0;
  }

  function apply() {
    rows.sort(cmp).forEach(function (r) { box.appendChild(r); });
    heads.forEach(function (h) {
      const icon = h.lastChild; // the <i>, holding the arrow icon
      const active = h.dataset.k === sort.k;
      icon.classList.toggle("active", active);
      icon.classList.toggle("desc", active && sort.o !== 1);
    });
  }

  heads.forEach(function (h) {
    h.onclick = function (e) {
      e.preventDefault();
      const k = h.dataset.k;
      sort = {k: k, o: k === sort.k ? -sort.o : first[k]};
      try { localStorage.setItem("sort", JSON.stringify(sort)); } catch (e) {}
      apply();
    };
  });
  apply();
})();

// Folder tree: only present when the -tree flag is on.
const tree = document.getElementById("tree");
if (tree) {
  // The server adds ?show=1 to links so the sidebar stays shown without
  // JavaScript. This page has JS, so that's handled below via localStorage
  // instead - drop it to keep the address bar clean.
  if (location.search) history.replaceState(null, "", location.pathname);

  // The toggle is a real link (to "?show=..."), so the sidebar's shown/hidden
  // state still survives a click without JavaScript. With JS, keep that
  // instant and reload-free: flip the checkbox ourselves and remember it.
  const tt = document.getElementById("tt");
  document.getElementById("tt-toggle").onclick = function (e) {
    e.preventDefault();
    tt.checked = !tt.checked;
    try { localStorage.setItem("tree", tt.checked ? "1" : "0"); } catch (e) {}
  };
  // Open folders and scroll position last only for this tab's session, and a reload resets them.
  const load = function (k, def) { try { return JSON.parse(sessionStorage.getItem(k)) || def; } catch (e) { return def; } };
  const save = function (k, v) { try { sessionStorage.setItem(k, JSON.stringify(v)); } catch (e) {} };
  const navEntry = performance.getEntriesByType("navigation")[0];
  if (navEntry && navEntry.type === "reload") { save("treeOpen", []); save("treeScroll", 0); save("treeHTML", {}); }
  let opened = load("treeOpen", []); // paths of folders that stay expanded
  // Fetched children, keyed by folder path: lets a folder that was open on an
  // earlier page reopen instantly on this one instead of waiting on a fetch,
  // which otherwise shows it closed for a moment first (it isn't this page's
  // ancestor, so the server doesn't expand it) - a visible jump for any
  // folder with children.
  let htmlCache = load("treeHTML", {});
  // "cur" marks the folder this exact page is showing - never something to
  // cache and replay on a later, different page (that folder isn't "cur"
  // there, whatever a stale snapshot says), or two folders end up bold at
  // once: the real current one, plus a leftover from whenever this snapshot
  // was taken.
  const stripCur = function (ul) {
    [].forEach.call(ul.querySelectorAll(".cur"), function (el) { el.classList.remove("cur"); });
    return ul;
  };
  // The folders leading to this page are open too; remember them so they stay
  // open after navigating elsewhere. Also warm the treeHTML cache for each
  // from what the server already rendered: expand() never runs for them
  // (they're already open), so without this, the first time you navigate
  // away from one of them it would still jump once before it's cached.
  [].forEach.call(tree.querySelectorAll("li.open"), function (li) {
    const a = li.querySelector("a");
    const p = a.pathname;
    if (opened.indexOf(p) < 0) opened.push(p);
    const ul = li.querySelector(":scope > ul");
    if (ul && !htmlCache[p]) {
      // Clone it: this folder's own "cur" descendant (if any) is genuinely
      // current on THIS page and must keep showing that way here, even
      // though the cached copy about to be stripped of it is not.
      const clone = ul.cloneNode(true);
      // Resolve against the PAGE's own URL, not this folder's (li's) own
      // href: every link the server renders into the page - at any depth -
      // is relative to the page itself (the same "../../.." prefix is
      // threaded through the whole tree, unlike the AJAX ?tree=1 endpoint
      // below, which deliberately renders paths relative to the expanded
      // folder instead). Resolving a deeply-nested link (more "../"s than
      // this folder is deep) against this folder's own, shallower href
      // exhausts those "../"s early and silently clips the path - which is
      // how a cached link ends up missing a leading reverse-proxy prefix.
      [].forEach.call(clone.querySelectorAll("a"), function (x) { x.href = new URL(x.getAttribute("href"), location.href).href; });
      htmlCache[p] = stripCur(clone).outerHTML;
    }
  });
  save("treeOpen", opened);
  save("treeHTML", htmlCache);

  // Expand a folder: instantly from the cache if we've fetched its children
  // before, otherwise fetching them first.
  const expand = function (li) {
    const b = li.querySelector("button");
    if (li.querySelector("ul") || !b) { li.classList.add("open"); return Promise.resolve(); }
    const a = li.querySelector("a");
    const p = a.pathname;
    if (htmlCache[p]) {
      const d = document.createElement("div");
      d.innerHTML = htmlCache[p];
      if (d.firstChild) li.appendChild(d.firstChild);
      li.classList.add("open");
      return Promise.resolve();
    }
    const u = new URL(a.href);
    u.searchParams.set("tree", "1");
    return fetch(u).then(function (r) { return r.text(); }).then(function (html) {
      const d = document.createElement("div");
      d.innerHTML = html;
      const ul = d.firstChild;
      if (!ul || !ul.children.length) return b.replaceWith(Object.assign(document.createElement("span"), {className: "sp"}));
      [].forEach.call(ul.querySelectorAll("a"), function (x) { x.href = new URL(x.getAttribute("href"), a.href).href; });
      li.appendChild(ul);
      li.classList.add("open");
      htmlCache[p] = ul.outerHTML;
      save("treeHTML", htmlCache);
    });
  };

  tree.onclick = function (e) {
    const b = e.target.closest("button");
    if (!b) return;
    const li = b.parentNode, p = li.querySelector("a").pathname;
    if (li.classList.contains("open")) {
      li.classList.remove("open");
      opened = opened.filter(function (x) { return x !== p; });
      save("treeOpen", opened);
    } else {
      // Wait for expand() to actually finish before recording it as open:
      // doing this eagerly left a window where treeOpen already claimed the
      // folder was open while treeHTML's cache entry (set inside expand(),
      // after its fetch resolves) didn't exist yet - and for a folder with
      // no subdirectories, expand() never adds the "open" class at all, so
      // it would otherwise stay stuck in treeOpen forever despite never
      // actually being open.
      expand(li).then(function () {
        if (li.classList.contains("open") && opened.indexOf(p) < 0) {
          opened.push(p);
          save("treeOpen", opened);
        }
      });
    }
  };

  // Reopen the folders expanded on earlier pages and restore the scroll
  // position. Each folder waits only on its own parent (so its li exists to
  // expand into), not on every other open folder - unrelated folders (e.g.
  // two open siblings) expand in parallel instead of queuing behind each
  // other, which otherwise delayed an instantly-cached folder's reopen
  // behind some unrelated folder's slow, uncached fetch and caused the same
  // visible jump the cache is meant to avoid.
  tree.scrollTop = load("treeScroll", 0);
  const parentOf = function (p) { return p === "/" ? null : p.replace(/[^/]+\/$/, ""); };
  const pending = {};
  const ensureExpanded = function (p) {
    if (pending[p]) return pending[p];
    const parent = parentOf(p);
    const parentReady = (parent && opened.indexOf(parent) >= 0) ? ensureExpanded(parent) : Promise.resolve();
    return pending[p] = parentReady.then(function () {
      const li = [].find.call(tree.querySelectorAll("li"), function (l) { return l.querySelector("a").pathname === p; });
      return li && expand(li);
    });
  };
  Promise.all(opened.map(ensureExpanded)).then(function () { tree.scrollTop = load("treeScroll", 0); });
  addEventListener("pagehide", function () { save("treeScroll", tree.scrollTop); });
}
