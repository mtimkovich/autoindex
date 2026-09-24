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
  if (navEntry && navEntry.type === "reload") { save("treeOpen", []); save("treeScroll", 0); }
  let opened = load("treeOpen", []); // paths of folders that stay expanded
  // The folders leading to this page are open too; remember them so they stay
  // open after navigating elsewhere.
  [].forEach.call(tree.querySelectorAll("li.open"), function (li) {
    const p = li.querySelector("a").pathname;
    if (opened.indexOf(p) < 0) opened.push(p);
  });
  save("treeOpen", opened);

  // Expand a folder, fetching its children first if they aren't in the page.
  const expand = function (li) {
    const b = li.querySelector("button");
    if (li.querySelector("ul") || !b) { li.classList.add("open"); return Promise.resolve(); }
    const a = li.querySelector("a");
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
    });
  };

  tree.onclick = function (e) {
    const b = e.target.closest("button");
    if (!b) return;
    const li = b.parentNode, p = li.querySelector("a").pathname;
    if (li.classList.contains("open")) {
      li.classList.remove("open");
      opened = opened.filter(function (x) { return x !== p; });
    } else {
      expand(li);
      if (opened.indexOf(p) < 0) opened.push(p);
    }
    save("treeOpen", opened);
  };

  // Reopen the folders expanded on earlier pages (parents first) and restore the scroll position.
  tree.scrollTop = load("treeScroll", 0);
  opened.slice().sort(function (a, b) { return a.length - b.length; }).reduce(function (chain, p) {
    return chain.then(function () {
      const li = [].find.call(tree.querySelectorAll("li"), function (l) { return l.querySelector("a").pathname === p; });
      return li && expand(li);
    });
  }, Promise.resolve()).then(function () { tree.scrollTop = load("treeScroll", 0); });
  addEventListener("pagehide", function () { save("treeScroll", tree.scrollTop); });
}
