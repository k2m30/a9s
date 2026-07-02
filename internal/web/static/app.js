// app.js — keyboard-driven action dispatcher for a9s web UI.
// Maps TUI-equivalent key presses to POST /action calls.
// No external dependencies, no build step required.

(function () {
  "use strict";

  const token = document.body.getAttribute("data-token") || "";

  // sendAction posts a semantic action to /action and swaps the #body content.
  // done, if provided, is called after the round-trip settles (success or
  // failure) — used by clickSelect to release its double-click guard.
  function sendAction(kind, arg, n, done) {
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
          // Screen changed: clear the row-click debounce so a same-index click
          // on the new screen is never suppressed. Covers clickField /
          // clickRelated / clickSelect / keyboard actions.
          lastClickIdx = -1;
          lastClickAt = 0;
        });
      })
      .catch(function (e) { console.error("action error:", e); })
      .finally(function () {
        clearTimeout(loadingTimer);
        var li = document.getElementById("loading-indicator");
        if (li) li.style.display = "none";
        if (done) done();
      });
  }

  // clickSelect navigates to the item at visible index idx and selects it,
  // via a single atomic "select-index" action (the controller sets the
  // cursor to idx directly, matching whatever visible index the template
  // rendered). Previously this replayed move-top + N×move-down + select as
  // separate round-trips; that chain landed on the wrong row whenever cursor
  // movement skips entries (e.g. the main menu's skip-unavailable stepping
  // over confirmed-empty resource types), opening the wrong resource.
  // clickBusy guards against a double-click firing the action twice: the
  // second click would run select-index on the screen the first click
  // already navigated to, drilling a level deeper ("two screens away").
  var clickBusy = false, lastClickIdx = -1, lastClickAt = 0;
  function clickSelect(idx) {
    var now = Date.now();
    if (clickBusy) return;
    // Ignore a repeat click on the same row within 500ms. lastClickIdx is reset
    // to -1 after every navigation (sendAction's DOM-swap handler) — so the
    // window only ever suppresses a double-click's second tap before the
    // screen changes, never a legitimate click on the same row index of the
    // screen we just navigated to.
    if (idx === lastClickIdx && now - lastClickAt < 500) return;
    lastClickIdx = idx;
    lastClickAt = now;
    clickBusy = true;
    sendAction("select-index", undefined, idx, function () {
      clickBusy = false;
    });
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
    // r toggles the related panel (TUI keys.go ToggleRelated="r"). Refresh
    // used to be ctrl+r, but that hijacked the browser's reload shortcut, so
    // it now lives on bare "R" (shift+r) instead.
    { key: "r",          action: { kind: "toggle-related" } },
    { key: "R",          action: { kind: "refresh" } },
    { key: "m",          action: { kind: "load-more" } },
    { key: "c",          action: { kind: "copy" } },
    { key: "w",          action: { kind: "toggle-wrap" } },
    { key: "!",          action: { kind: "open-error-log" } },
    { key: "Tab",        action: { kind: "toggle-focus" } },
    { key: "t",          action: { kind: "cloudtrail" } },

    // Page scroll
    { key: "PageUp",     action: { kind: "page-up",   n: 20 } },
    { key: "PageDown",   action: { kind: "page-down", n: 20 } },
  ];

  // filterInput holds state for the / filter input.
  var filterMode = false;
  var filterBuf = "";

  // searchInput holds state for the search input (no keyboard entry point
  // currently wired up; see the keydown handler for details).
  var searchMode = false;
  var searchBuf = "";

  function setFilter(val) {
    sendAction("set-filter", val);
  }

  function setSearch(val) {
    sendAction("search", val);
  }

  // commandMode holds state for the ":" command palette.
  var commandMode = false;
  var commandBuf = "";

  // showInputBar / hideInputBar render the client-side mode line (mirrors the
  // TUI's bottom input). prefix is "/" (filter), ":" (command), or "search: ".
  // Shown the instant the mode key is pressed so an empty buffer is still
  // visible — the fix for "/ shows nothing until the first letter".
  function showInputBar(prefix, buf) {
    var bar = document.getElementById("input-bar");
    if (!bar) return;
    bar.textContent = prefix + buf + "█"; // block cursor
    bar.style.display = "block";
  }
  function hideInputBar() {
    var bar = document.getElementById("input-bar");
    if (bar) bar.style.display = "none";
  }

  document.addEventListener("keydown", function (e) {
    // Ignore when focus is in an input/textarea.
    var tag = (document.activeElement || {}).tagName || "";
    if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return;

    // Never hijack Cmd/Meta combos (copy, paste, reload, address bar, etc.) —
    // let the browser handle them untouched.
    if (e.metaKey) return;

    // Ctrl combos: only ctrl+z (issues-only toggle) is ours. Everything else
    // (ctrl+r reload, ctrl+s save, ctrl+c/v, ...) falls through to the browser.
    if (e.ctrlKey && e.key !== "z" && e.key !== "Z") return;

    // Filter mode: / was pressed.
    if (filterMode) {
      if (e.key === "Escape") {
        filterMode = false;
        filterBuf = "";
        setFilter("");
        hideInputBar();
        e.preventDefault();
        return;
      }
      if (e.key === "Enter") {
        filterMode = false;
        hideInputBar();
        e.preventDefault();
        return;
      }
      if (e.key === "Backspace") {
        filterBuf = filterBuf.slice(0, -1);
        setFilter(filterBuf);
        showInputBar("/", filterBuf);
        e.preventDefault();
        return;
      }
      if (e.key.length === 1) {
        filterBuf += e.key;
        setFilter(filterBuf);
        showInputBar("/", filterBuf);
        e.preventDefault();
        return;
      }
      return;
    }

    // Search mode: no keyboard trigger currently enters this (ctrl+s was
    // removed so the browser's save-page shortcut works); state and handling
    // kept in case a future non-hijacking entry point is added.
    if (searchMode) {
      if (e.key === "Escape") {
        searchMode = false;
        searchBuf = "";
        sendAction("search-clear", "");
        hideInputBar();
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
        showInputBar("search: ", searchBuf);
        e.preventDefault();
        return;
      }
      if (e.key.length === 1) {
        searchBuf += e.key;
        setSearch(searchBuf);
        showInputBar("search: ", searchBuf);
        e.preventDefault();
        return;
      }
      return;
    }

    // Command mode: : was pressed. Mirrors the TUI's colon-command palette;
    // the command is collected locally and dispatched on Enter (the controller's
    // ActionCommand handles root/profile/region/theme/help/<resource-shortname>).
    if (commandMode) {
      if (e.key === "Escape") {
        commandMode = false;
        commandBuf = "";
        hideInputBar();
        e.preventDefault();
        return;
      }
      if (e.key === "Enter") {
        commandMode = false;
        hideInputBar();
        if (commandBuf) sendAction("command", commandBuf);
        commandBuf = "";
        e.preventDefault();
        return;
      }
      if (e.key === "Backspace") {
        commandBuf = commandBuf.slice(0, -1);
        showInputBar(":", commandBuf);
        e.preventDefault();
        return;
      }
      if (e.key.length === 1) {
        commandBuf += e.key;
        showInputBar(":", commandBuf);
        e.preventDefault();
        return;
      }
      return;
    }

    // Enter filter mode.
    if (e.key === "/" && !e.ctrlKey && !e.metaKey) {
      filterMode = true;
      filterBuf = "";
      showInputBar("/", "");
      e.preventDefault();
      return;
    }

    // Enter command mode (:).
    if (e.key === ":" && !e.ctrlKey && !e.metaKey) {
      commandMode = true;
      commandBuf = "";
      showInputBar(":", "");
      e.preventDefault();
      return;
    }

    // Ctrl+Z: issues-only filter. This is the only ctrl combo we intercept
    // (guarded above); everything else falls through to the browser. Must
    // still check e.ctrlKey here — a bare z/Z keypress is not a shortcut.
    if (e.ctrlKey && (e.key === "z" || e.key === "Z")) {
      sendAction("toggle-attention", "");
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
    var childKeys = { "e": "e", "L": "L", "s": "s" };
    if (childKeys[e.key] && !e.ctrlKey) {
      sendAction("child-view", e.key);
      e.preventDefault();
      return;
    }

    // Enter on a navigable, focused detail field navigates that field (the
    // controller's field-select) rather than the generic select. The navigation
    // logic lives in the controller; this only routes the key.
    if (e.key === "Enter") {
      var fcur = document.querySelector(".field-row.field-cursor.field-navigable");
      if (fcur && fcur.getAttribute("data-fidx") !== null) {
        sendAction("field-select", fcur.getAttribute("data-fidx"));
        e.preventDefault();
        return;
      }
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
