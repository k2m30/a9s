// app.js — keyboard-driven action dispatcher for a9s web UI.
// Maps TUI-equivalent key presses to POST /action calls.
// No external dependencies, no build step required.

(function () {
  "use strict";

  const token = document.body.getAttribute("data-token") || "";

  // sendAction posts a semantic action to /action and swaps the #body content.
  function sendAction(kind, arg, n) {
    const body = { kind: kind };
    if (arg !== undefined && arg !== "") body.arg = String(arg);
    if (n !== undefined && n !== 0) body.n = n;

    // Only reveal the loading indicator if the round-trip is actually slow.
    // Most actions return within a frame, so showing it immediately made every
    // keypress flash "loading". Delay it ~180ms and cancel on completion.
    var loadingTimer = setTimeout(function () {
      var li = document.getElementById("loading-indicator");
      if (li) li.style.display = "block";
    }, 180);

    fetch("/action", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-A9S-Token": token,
      },
      body: JSON.stringify(body),
    })
      .then(function (r) {
        if (!r.ok) {
          console.error("action failed:", r.status);
          return r.text().then(function (t) { console.error(t); });
        }
        return r.text().then(function (html) {
          var el = document.getElementById("main");
          if (el) el.innerHTML = html;
        });
      })
      .catch(function (e) { console.error("action error:", e); })
      .finally(function () {
        clearTimeout(loadingTimer);
        var li = document.getElementById("loading-indicator");
        if (li) li.style.display = "none";
      });
  }

  // clickSelect navigates to item at index idx and selects it.
  // Sends move-up (to top) then the right number of move-downs, then select.
  // Simpler: just send a special "goto" by looping move-down from 0. Instead,
  // use move-top + N×move-down + select chained sequentially via async.
  // clickBusy guards against a double-click firing the move-chain twice: the
  // second click would run move-top/down/select on the screen the first click
  // already navigated to, drilling a level deeper ("two screens away").
  var clickBusy = false, lastClickIdx = -1, lastClickAt = 0;
  function clickSelect(idx) {
    var now = Date.now();
    if (clickBusy) return;
    // Ignore a repeat click on the same row within 500ms: a double-click would
    // otherwise navigate, then re-run the move-chain on the new screen, landing
    // two levels deep.
    if (idx === lastClickIdx && now - lastClickAt < 500) return;
    lastClickIdx = idx;
    lastClickAt = now;
    clickBusy = true;
    // Chain: move-top → N × move-down → select (all sequential).
    var steps = [{ kind: "move-top" }];
    for (var i = 0; i < idx; i++) {
      steps.push({ kind: "move-down" });
    }
    steps.push({ kind: "select" });
    chainActions(steps, 0, function () { clickBusy = false; });
  }

  function chainActions(steps, i, done) {
    if (i >= steps.length) { if (done) done(); return; }
    var s = steps[i];
    var body = { kind: s.kind };
    if (s.arg) body.arg = s.arg;

    fetch("/action", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-A9S-Token": token,
      },
      body: JSON.stringify(body),
    })
      .then(function (r) {
        if (!r.ok) { if (done) done(); return; }
        return r.text().then(function (html) {
          var el = document.getElementById("main");
          if (el) el.innerHTML = html;
          chainActions(steps, i + 1, done);
        });
      })
      .catch(function (e) { console.error(e); if (done) done(); });
  }

  // clickRelated navigates to the related-panel row at visible index idx.
  // Dead-end rows (count <= 0 and not loading) have no onclick so this is
  // only called for navigable rows, but the controller guards as well.
  function clickRelated(idx) {
    sendAction("related-select", String(idx));
  }

  // clickField navigates to the resource linked by the navigable detail field
  // at visible index idx (matches $i in {{range $i, $f := .Fields}}).
  function clickField(idx) {
    sendAction("field-select", String(idx));
  }

  // Expose for use in onclick handlers in templates.
  window.sendAction = sendAction;
  window.clickSelect = clickSelect;
  window.clickRelated = clickRelated;
  window.clickField = clickField;

  // Keyboard map: TUI key → Action
  var keyMap = [
    // Navigation
    { key: "ArrowUp",    action: { kind: "move-up" } },
    { key: "ArrowDown",  action: { kind: "move-down" } },
    { key: "ArrowLeft",  action: { kind: "scroll-left" } },
    { key: "ArrowRight", action: { kind: "scroll-right" } },
    { key: "k",          action: { kind: "move-up" } },
    { key: "j",          action: { kind: "move-down" } },
    { key: "h",          action: { kind: "scroll-left" } },
    { key: "l",          action: { kind: "scroll-right" } },
    { key: "g",          action: { kind: "move-top" } },
    { key: "G",          action: { kind: "move-bottom" } },
    { key: "Enter",      action: { kind: "select" } },
    { key: "Escape",     action: { kind: "back" } },
    { key: "Backspace",  action: { kind: "back" } },

    // Views
    { key: "d",          action: { kind: "open-detail" } },
    { key: "y",          action: { kind: "open-yaml" } },
    { key: "J",          action: { kind: "open-json" } },
    { key: "?",          action: { kind: "open-help" } },
    { key: "i",          action: { kind: "open-identity" } },

    // List actions
    { key: "r",          action: { kind: "refresh" } },
    { key: "m",          action: { kind: "load-more" } },
    { key: "c",          action: { kind: "copy" } },
    { key: "w",          action: { kind: "toggle-wrap" } },
    { key: "!",          action: { kind: "toggle-attention" } },
    { key: "R",          action: { kind: "toggle-related" } },
    { key: "Tab",        action: { kind: "toggle-focus" } },

    // Page scroll
    { key: "PageUp",     action: { kind: "page-up",   n: 20 } },
    { key: "PageDown",   action: { kind: "page-down", n: 20 } },
  ];

  // filterInput holds state for the / filter input.
  var filterMode = false;
  var filterBuf = "";

  // searchInput holds state for Ctrl+S search input.
  var searchMode = false;
  var searchBuf = "";

  function setFilter(val) {
    sendAction("set-filter", val);
  }

  function setSearch(val) {
    sendAction("search", val);
  }

  document.addEventListener("keydown", function (e) {
    // Ignore when focus is in an input/textarea.
    var tag = (document.activeElement || {}).tagName || "";
    if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return;

    // Filter mode: / was pressed.
    if (filterMode) {
      if (e.key === "Escape") {
        filterMode = false;
        filterBuf = "";
        setFilter("");
        e.preventDefault();
        return;
      }
      if (e.key === "Enter") {
        filterMode = false;
        e.preventDefault();
        return;
      }
      if (e.key === "Backspace") {
        filterBuf = filterBuf.slice(0, -1);
        setFilter(filterBuf);
        e.preventDefault();
        return;
      }
      if (e.key.length === 1) {
        filterBuf += e.key;
        setFilter(filterBuf);
        e.preventDefault();
        return;
      }
      return;
    }

    // Search mode: ctrl+s was pressed.
    if (searchMode) {
      if (e.key === "Escape") {
        searchMode = false;
        searchBuf = "";
        sendAction("search-clear", "");
        e.preventDefault();
        return;
      }
      if (e.key === "Enter") {
        sendAction("search-next", "");
        e.preventDefault();
        return;
      }
      if (e.key === "Backspace") {
        searchBuf = searchBuf.slice(0, -1);
        setSearch(searchBuf);
        e.preventDefault();
        return;
      }
      if (e.key.length === 1) {
        searchBuf += e.key;
        setSearch(searchBuf);
        e.preventDefault();
        return;
      }
      return;
    }

    // Enter filter mode.
    if (e.key === "/" && !e.ctrlKey && !e.metaKey) {
      filterMode = true;
      filterBuf = "";
      e.preventDefault();
      return;
    }

    // Enter search mode (Ctrl+S).
    if (e.key === "s" && (e.ctrlKey || e.metaKey)) {
      searchMode = true;
      searchBuf = "";
      e.preventDefault();
      return;
    }

    // Search navigation.
    if (e.key === "n" && !e.ctrlKey) {
      sendAction("search-next", "");
      e.preventDefault();
      return;
    }
    if (e.key === "N" && !e.ctrlKey) {
      sendAction("search-prev", "");
      e.preventDefault();
      return;
    }

    // Child-view triggers.
    var childKeys = { "e": "e", "L": "L", "s": "s", "t": "t" };
    if (childKeys[e.key] && !e.ctrlKey) {
      sendAction("child-view", e.key);
      e.preventDefault();
      return;
    }

    // Look up in key map.
    for (var i = 0; i < keyMap.length; i++) {
      var entry = keyMap[i];
      if (entry.key === e.key) {
        var a = entry.action;
        sendAction(a.kind, a.arg || "", a.n || 0);
        e.preventDefault();
        return;
      }
    }
  });

  // SSE: listen for state-changed events and reload the body.
  function connectSSE() {
    var url = "/events?token=" + encodeURIComponent(token);
    var evtSrc = new EventSource(url);

    evtSrc.addEventListener("update", function () {
      // Fetch the body fragment directly — no POST, no notifySubscribers,
      // no risk of triggering another "update" event.
      fetch("/body", {
        headers: { "X-A9S-Token": token },
      })
        .then(function (r) { return r.text(); })
        .then(function (html) {
          var el = document.getElementById("main");
          if (el && html) el.innerHTML = html;
        })
        .catch(function () {});
    });

    evtSrc.onerror = function () {
      evtSrc.close();
      // Reconnect after 3 seconds.
      setTimeout(connectSSE, 3000);
    };
  }

  connectSSE();
})();
