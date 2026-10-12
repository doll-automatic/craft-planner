// 画面はサーバの算出結果を描画するだけである。施設数や素材合計はここでは計算しない。
const THEME_KEY = "gpm-theme";

// 初期倍率の上限とホイール感度。要件の「40%〜100%」は目安。
// ノードがカード化して大きくなったため、単一ノードでも fit が拡大しすぎないよう初期倍率は 1 で頭打ちにする。
const treeZoom = {
  wheelSensitivity: 0.2,
  minZoom: 0.05,
  maxZoom: 2.5,
  maxInitialZoom: 1,
  padding: 28,
};

// Node_Card のモデル寸法。CSS の padding・行高と揃える（ズーム前の px）。
const cardMetrics = {
  width: 280,
  chrome: 66,
  row: 26,
  rowGap: 6,
  sectionGap: 6,
};

const ui = {
  games: [],
  gameId: "",
  game: null,
  itemId: "",
  cy: null,
  materials: [],
  // リロードすると初期の「1 次＋中間」に戻る。
  summaryMode: "all",
};

const $ = (id) => document.getElementById(id);

function init() {
  applyTheme(savedOrSystemTheme());
  $("theme-toggle").addEventListener("click", toggleTheme);
  $("game-select").addEventListener("change", () => {
    selectGame($("game-select").value);
  });
  $("type-filter").addEventListener("change", () => {
    renderItemOptions();
  });
  $("name-filter").addEventListener("input", () => {
    renderItemOptions();
  });
  $("item-select").addEventListener("change", () => {
    const id = $("item-select").value;
    if (!id) {
      clearItemView();
      return;
    }
    selectItem(id);
  });
  $("rate-input").addEventListener("change", () => {
    commitRate();
  });
  document.querySelectorAll('input[name="summary-mode"]').forEach((input) => {
    input.addEventListener("change", () => {
      if (!input.checked) return;
      ui.summaryMode = input.value === "primary" ? "primary" : "all";
      renderMaterials(ui.materials);
    });
  });
  $("tree-fullscreen").addEventListener("click", () => toggleFullscreen("tree-panel"));
  $("lists-fullscreen").addEventListener("click", () => toggleFullscreen("lists-panel"));
  document.addEventListener("keydown", onFullscreenKey);
  clearFacilities();
  renderMaterials([]);
  loadGameList().catch((err) => {
    console.error(err);
  });
}

function savedOrSystemTheme() {
  try {
    const saved = localStorage.getItem(THEME_KEY);
    if (saved === "light" || saved === "dark") return saved;
  } catch (err) {
    // 保存値を読めないときは OS の配色に従う。
  }
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

function applyTheme(theme) {
  document.documentElement.setAttribute("data-theme", theme);
  const btn = $("theme-toggle");
  if (!btn) return;
  const dark = theme === "dark";
  btn.textContent = dark ? "ライト" : "ダーク";
  btn.setAttribute("aria-pressed", dark ? "true" : "false");
  btn.setAttribute("aria-label", dark ? "ライトテーマに切り替える" : "ダークテーマに切り替える");
  updateGraphTheme();
}

function toggleTheme() {
  const next = document.documentElement.getAttribute("data-theme") === "dark" ? "light" : "dark";
  try {
    localStorage.setItem(THEME_KEY, next);
  } catch (err) {
    // 保存に失敗しても画面の切替は行う。
  }
  applyTheme(next);
}

function cssVar(name) {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
}

function graphStylesheet() {
  const edge = cssVar("--edge");
  const edgeLabel = cssVar("--edge-label");
  const paper = cssVar("--paper");
  return [
    {
      selector: "node",
      style: {
        label: "",
        "background-opacity": 0,
        "border-width": 0,
        width: "data(w)",
        height: "data(h)",
        shape: "round-rectangle",
      },
    },
    {
      selector: "edge",
      style: {
        width: 2,
        "line-color": edge,
        // 矢じりは source（親・上）側。素材から成果物へ、画面では下から上へ向ける。
        "source-arrow-color": edge,
        "source-arrow-shape": "triangle",
        "target-arrow-shape": "none",
        // 折れ線は親から子へ下向きのままにする。upward にすると親の上へ一度出てから子へ戻り、上向きの矢印と食い違う。
        "curve-style": "taxi",
        "taxi-direction": "downward",
        label: "data(label)",
        "font-size": 12,
        color: edgeLabel,
        "text-background-color": paper,
        "text-background-opacity": 0.95,
        "text-background-padding": 3,
      },
    },
  ];
}

function updateGraphTheme() {
  if (!ui.cy) return;
  ui.cy.style().fromJson(graphStylesheet()).update();
}

async function loadGameList() {
  const res = await fetch("/api/games");
  if (!res.ok) throw new Error("games");
  const data = await res.json();
  ui.games = data.games || [];
  renderGameOptions();
  renderErrors(data.errors || []);
}

function renderGameOptions() {
  const sel = $("game-select");
  sel.replaceChildren();
  const first = document.createElement("option");
  first.value = "";
  first.textContent = "ゲームを選択";
  sel.appendChild(first);
  for (const game of ui.games) {
    const opt = document.createElement("option");
    opt.value = game.gameId;
    opt.textContent = game.name;
    sel.appendChild(opt);
  }
}

function renderErrors(errors) {
  const box = $("errors");
  box.replaceChildren();
  if (!errors.length) {
    box.hidden = true;
    return;
  }
  box.hidden = false;
  const title = document.createElement("h2");
  title.textContent = "読み込めなかったゲーム（選択できません）";
  box.appendChild(title);
  for (const err of errors) {
    const p = document.createElement("p");
    const parts = [err.fileName, err.itemId, err.refItemId, (err.cycle || []).join(" → "), err.message].filter(Boolean);
    p.textContent = parts.join(" / ");
    box.appendChild(p);
  }
}

async function selectGame(gameId) {
  clearItemView();
  clearRate();
  resetItemFilter();
  ui.gameId = gameId || "";
  ui.game = null;
  $("empty-game").hidden = true;
  renderTypeOptions();
  renderItemOptions();
  if (!ui.gameId) return;
  const res = await fetch("/api/games/" + encodeURIComponent(ui.gameId));
  if (!res.ok) {
    $("empty-game").hidden = false;
    $("empty-game").textContent = "ゲームを読み込めませんでした";
    return;
  }
  ui.game = await res.json();
  renderTypeOptions();
  renderItemOptions();
  const items = ui.game.items || [];
  $("empty-game").hidden = items.length !== 0;
  $("empty-game").textContent = "表示可能なアイテムがありません";
}

function resetItemFilter() {
  $("name-filter").value = "";
}

function renderTypeOptions() {
  const sel = $("type-filter");
  const seen = Object.create(null);
  const types = [];
  for (const item of (ui.game && ui.game.items) || []) {
    const type = item.type || "";
    if (!type || seen[type]) continue;
    seen[type] = true;
    types.push(type);
  }
  types.sort((a, b) => a.localeCompare(b, "ja"));
  sel.replaceChildren();
  const all = document.createElement("option");
  all.value = "";
  all.textContent = "すべて";
  sel.appendChild(all);
  for (const type of types) {
    const opt = document.createElement("option");
    opt.value = type;
    opt.textContent = type;
    sel.appendChild(opt);
  }
  const ready = !!ui.game;
  sel.disabled = !ready;
  sel.value = "";
  $("name-filter").disabled = !ready;
}

function filteredItems() {
  const type = $("type-filter").value;
  const query = $("name-filter").value.trim().toLowerCase();
  return ((ui.game && ui.game.items) || []).filter((item) => {
    if (type && item.type !== type) return false;
    if (query && !String(item.name || "").toLowerCase().includes(query)) return false;
    return true;
  }).sort((a, b) => a.id.localeCompare(b.id, "ja"));
}

function renderItemOptions() {
  const sel = $("item-select");
  const all = (ui.game && ui.game.items) || [];
  const items = filteredItems();
  sel.replaceChildren();
  const first = document.createElement("option");
  first.value = "";
  first.textContent = "アイテムを選択";
  sel.appendChild(first);
  for (const item of items) {
    const opt = document.createElement("option");
    opt.value = item.id;
    opt.textContent = item.name;
    sel.appendChild(opt);
  }
  sel.disabled = items.length === 0;
  sel.value = ui.itemId || "";
  $("filter-empty").hidden = !(ui.game && all.length > 0 && items.length === 0);
}

function itemById(id) {
  const items = (ui.game && ui.game.items) || [];
  return items.find((item) => item.id === id) || null;
}

async function selectItem(id) {
  ui.itemId = id;
  const item = itemById(id);
  if (!item) {
    clearItemView();
    return;
  }
  await loadTree(id);
  const rate = currentRate();
  if (rate.ok) await loadFacilities(rate.value);
}

async function loadTree(id) {
  const url = "/api/games/" + encodeURIComponent(ui.gameId) + "/tree?itemId=" + encodeURIComponent(id);
  const res = await fetch(url);
  if (!res.ok) {
    $("tree-notice").textContent = "生産ツリーを取得できませんでした";
    return;
  }
  const data = await res.json();
  const notes = [];
  if (data.truncated) notes.push("ノード数が上限（1000）に達したため、一部が未展開です。");
  if (data.cycle && data.cycle.length) {
    notes.push("循環を検出しました: " + data.cycle.join(" → ") + "。循環ノードは再展開せずに表示しています。");
  }
  $("tree-notice").textContent = notes.join(" ");
  renderGraph(data);
  renderMaterials(data.materials || []);
}

function fitTree(cy) {
  cy.resize();
  cy.fit(treeZoom.padding);
  if (cy.zoom() > treeZoom.maxInitialZoom) {
    cy.zoom(treeZoom.maxInitialZoom);
    cy.center();
  }
}

let treeResizeTimer = 0;
function scheduleFitTree() {
  clearTimeout(treeResizeTimer);
  treeResizeTimer = setTimeout(() => {
    const box = $("cy");
    if (!ui.cy || !box || box.hidden) return;
    const rect = box.getBoundingClientRect();
    if (rect.width < 10 || rect.height < 10) return;
    fitTree(ui.cy);
  }, 80);
}

if (typeof ResizeObserver === "function") {
  new ResizeObserver(scheduleFitTree).observe($("cy"));
}
window.addEventListener("resize", scheduleFitTree);

// 全画面は localStorage に載せない。同時に広げられるのは 1 つ。
const fullscreenButtons = {
  "tree-panel": "tree-fullscreen",
  "lists-panel": "lists-fullscreen",
};

function setFullscreen(panelId, on) {
  const panel = $(panelId);
  const button = $(fullscreenButtons[panelId]);
  panel.classList.toggle("is-fullscreen", on);
  button.setAttribute("aria-pressed", on ? "true" : "false");
  button.textContent = on ? "全画面解除" : "全画面表示";
}

function toggleFullscreen(panelId) {
  const opening = !$(panelId).classList.contains("is-fullscreen");
  let canvasChanged = panelId === "tree-panel";
  if (opening) {
    Object.keys(fullscreenButtons).forEach((id) => {
      if (id === panelId || !$(id).classList.contains("is-fullscreen")) return;
      setFullscreen(id, false);
      if (id === "tree-panel") canvasChanged = true;
    });
  }
  setFullscreen(panelId, opening);
  if (canvasChanged) scheduleFitTree();
}

function onFullscreenKey(event) {
  if (event.key !== "Escape") return;
  let closed = false;
  let canvasChanged = false;
  Object.keys(fullscreenButtons).forEach((id) => {
    if (!$(id).classList.contains("is-fullscreen")) return;
    setFullscreen(id, false);
    closed = true;
    if (id === "tree-panel") canvasChanged = true;
  });
  if (!closed) return;
  event.preventDefault();
  if (canvasChanged) scheduleFitTree();
}

function cardSize(data) {
  const inputs = data.hasInputs && Array.isArray(data.inputs) ? data.inputs.length : 0;
  const rows = inputs > 0 ? Math.ceil(inputs / 2) : 0;
  let height = cardMetrics.chrome;
  if (rows > 0) {
    height += cardMetrics.sectionGap + rows * cardMetrics.row + (rows - 1) * cardMetrics.rowGap;
  }
  return { w: cardMetrics.width, h: height };
}

function cardTitle(data) {
  const name = data.label || "";
  if (data.output == null) return name;
  return name + " (" + data.output + ")";
}

function rateLine(data) {
  const facility = data.facility || "";
  if (data.outputRate == null || !Number.isFinite(Number(data.outputRate))) return facility;
  const rate = formatNum(Number(data.outputRate)) + "/秒";
  return facility ? facility + " " + rate : rate;
}

function buildNodeCard(data) {
  const card = document.createElement("div");
  card.className = "node-card" + (data.hasInputs ? "" : " leaf");
  const head = document.createElement("div");
  head.className = "node-card-head";
  const name = document.createElement("div");
  name.className = "node-card-name";
  name.textContent = cardTitle(data);
  name.title = name.textContent;
  const type = document.createElement("div");
  type.className = "node-card-type";
  type.textContent = data.type || "";
  head.append(name, type);
  const mid = document.createElement("div");
  mid.className = "node-card-rate";
  mid.textContent = rateLine(data);
  mid.title = mid.textContent;
  card.append(head, mid);
  if (data.hasInputs && Array.isArray(data.inputs) && data.inputs.length) {
    const grid = document.createElement("div");
    grid.className = "node-card-inputs";
    for (const input of data.inputs) {
      const cell = document.createElement("div");
      cell.className = "node-card-input";
      cell.textContent = (input.name || "") + " (" + input.qty + ")";
      cell.title = cell.textContent;
      grid.appendChild(cell);
    }
    card.appendChild(grid);
  }
  return card;
}

function clearNodeCards() {
  document.querySelectorAll("#cy .node-card-layer").forEach((el) => el.remove());
}

// ノード座標に HTML カードを重ねる。ズームとパンは Cytoscape の描画座標へ追従する。
function mountNodeCards(cy) {
  clearNodeCards();
  const layer = document.createElement("div");
  layer.className = "node-card-layer";
  const cards = new Map();
  cy.nodes().forEach((node) => {
    const el = buildNodeCard(node.data());
    cards.set(node.id(), el);
    layer.appendChild(el);
  });
  cy.container().appendChild(layer);
  const sync = () => {
    const zoom = cy.zoom();
    cy.nodes().forEach((node) => {
      const el = cards.get(node.id());
      if (!el) return;
      const w = node.width();
      const h = node.height();
      const pos = node.renderedPosition();
      el.style.width = w + "px";
      el.style.height = h + "px";
      el.style.transform = "translate(" + (pos.x - (w * zoom) / 2) + "px," + (pos.y - (h * zoom) / 2) + "px) scale(" + zoom + ")";
    });
  };
  cy.on("render", sync);
  sync();
}

function renderGraph(data) {
  clearNodeCards();
  if (ui.cy) {
    ui.cy.destroy();
    ui.cy = null;
  }
  const nodes = data.nodes || [];
  $("tree-placeholder").hidden = nodes.length > 0;
  $("cy").hidden = nodes.length === 0;
  if (!nodes.length || typeof cytoscape !== "function") return;
  const elements = [];
  const nameById = new Map();
  nodes.forEach((node) => {
    const raw = node.data || {};
    const size = cardSize(raw);
    nameById.set(raw.id, raw.label || "");
    elements.push({
      data: {
        id: raw.id,
        label: raw.label || "",
        type: raw.type || "",
        output: raw.output,
        facility: raw.facility || "",
        outputRate: raw.outputRate,
        inputs: raw.inputs || [],
        hasInputs: !!raw.hasInputs,
        w: size.w,
        h: size.h,
      },
      classes: raw.hasInputs ? "" : "leaf",
    });
  });
  (data.edges || []).forEach((edge, index) => {
    const raw = edge.data || {};
    const childName = nameById.get(raw.target) || raw.target || "";
    elements.push({
      data: {
        id: "e" + index,
        source: raw.source,
        target: raw.target,
        // 表示用の転記。個数はサーバの qty をそのまま使う。
        label: childName + " ×" + raw.qty,
      },
    });
  });
  ui.cy = cytoscape({
    container: $("cy"),
    elements,
    wheelSensitivity: treeZoom.wheelSensitivity,
    minZoom: treeZoom.minZoom,
    maxZoom: treeZoom.maxZoom,
    layout: {
      name: "breadthfirst",
      directed: true,
      fit: false,
      padding: 16,
      // カード幅ぶん横に離す。縦は openVerticalLanes でラベルの通り道だけ空ける。
      spacingFactor: 1.55,
      avoidOverlap: true,
    },
    style: graphStylesheet(),
  });
  openVerticalLanes(ui.cy);
  mountNodeCards(ui.cy);
  fitTree(ui.cy);
}

// breadthfirst はカードの幅を縦間隔にも使う。直角エッジとラベルが通る分だけ縦を詰め、初期表示が極端に小さくならないようにする。
function openVerticalLanes(cy) {
  const nodes = cy.nodes();
  if (nodes.length < 2) return;
  const buckets = [];
  nodes.forEach((node) => {
    const y = node.position("y");
    let bucket = null;
    for (const item of buckets) {
      if (Math.abs(item.y - y) < 1) {
        bucket = item;
        break;
      }
    }
    if (!bucket) {
      bucket = { y, nodes: [] };
      buckets.push(bucket);
    }
    bucket.nodes.push(node);
  });
  buckets.sort((a, b) => a.y - b.y);
  const lane = 84;
  let center = buckets[0].y;
  buckets.forEach((bucket, index) => {
    let height = 0;
    bucket.nodes.forEach((node) => {
      node.position("y", center);
      height = Math.max(height, node.height());
    });
    if (index + 1 < buckets.length) {
      let nextHeight = 0;
      buckets[index + 1].nodes.forEach((node) => {
        nextHeight = Math.max(nextHeight, node.height());
      });
      center += height / 2 + lane + nextHeight / 2;
    }
  });
}

function renderMaterials(list) {
  ui.materials = Array.isArray(list) ? list : [];
  const ul = $("materials");
  ul.replaceChildren();
  const shown = ui.summaryMode === "primary" ? ui.materials.filter((line) => line.isPrimary) : ui.materials;
  if (!ui.materials.length) {
    const li = document.createElement("li");
    li.textContent = ui.itemId ? "素材が存在しません" : "アイテムを選択すると必要素材を表示します";
    ul.appendChild(li);
    return;
  }
  if (!shown.length) {
    const li = document.createElement("li");
    li.textContent = "1 次素材はありません";
    ul.appendChild(li);
    return;
  }
  for (const line of shown) {
    const li = document.createElement("li");
    li.className = "item-row";
    appendItemIcon(li, line.itemId);
    const text = document.createElement("span");
    text.textContent = line.name + " × " + line.quantity;
    li.appendChild(text);
    ul.appendChild(li);
  }
}

// アイコンは data/icon/<gameId>/<itemId>.png。無いときは要素を外して名称だけ残す。
function appendItemIcon(parent, itemId) {
  if (!ui.gameId || !itemId) return;
  const img = document.createElement("img");
  img.className = "item-icon";
  img.alt = "";
  img.src = "/data/icon/" + encodeURIComponent(ui.gameId) + "/" + encodeURIComponent(itemId) + ".png";
  img.addEventListener("error", () => {
    img.remove();
  });
  parent.appendChild(img);
}

function currentRate() {
  const raw = $("rate-input").value.trim();
  if (raw === "") return { ok: false, empty: true };
  const value = Number(raw);
  if (!Number.isFinite(value) || value <= 0) return { ok: false, empty: false };
  return { ok: true, value };
}

function commitRate() {
  const rate = currentRate();
  if (!rate.ok) {
    $("rate-error").hidden = false;
    $("rate-error").textContent = "目標生産速度は 0 より大きい数値で入力してください。直前の表示を保持しています。";
    return;
  }
  $("rate-error").hidden = true;
  if (ui.itemId) loadFacilities(rate.value);
}

async function loadFacilities(rate) {
  const url = "/api/games/" + encodeURIComponent(ui.gameId) + "/facilities?itemId=" + encodeURIComponent(ui.itemId) + "&rate=" + encodeURIComponent(rate);
  const res = await fetch(url);
  if (!res.ok) {
    $("rate-error").hidden = false;
    $("rate-error").textContent = "目標生産速度が不正なため、算出を行いませんでした。直前の表示を保持しています。";
    return;
  }
  const data = await res.json();
  const ul = $("facilities");
  ul.replaceChildren();
  const items = data.items || [];
  if (!items.length) {
    const li = document.createElement("li");
    li.textContent = "算出結果がありません";
    ul.appendChild(li);
    return;
  }
  for (const item of items) {
    const li = document.createElement("li");
    li.className = "item-row";
    if (item.itemId === ui.itemId) li.classList.add("root");
    appendItemIcon(li, item.itemId);
    const text = document.createElement("span");
    if (item.uncomputable) {
      text.textContent = item.name + ": 循環により算出できません";
    } else {
      text.textContent = item.name + ": 必要 " + formatNum(item.requiredRate) + " 個/秒 / 施設 " + item.facilityCount + " 台（1台あたり " + formatNum(item.outputRate) + " 個/秒）";
    }
    li.appendChild(text);
    ul.appendChild(li);
  }
}

function formatNum(n) {
  if (!Number.isFinite(n)) return "-";
  return String(Math.round(n * 1000) / 1000);
}

function clearItemView() {
  ui.itemId = "";
  clearNodeCards();
  $("tree-notice").textContent = "";
  $("tree-placeholder").hidden = false;
  $("cy").hidden = true;
  if (ui.cy) {
    ui.cy.destroy();
    ui.cy = null;
  }
  renderMaterials([]);
  clearFacilities();
  const sel = $("item-select");
  if (sel) sel.value = "";
}

function clearRate() {
  $("rate-input").value = "";
  $("rate-error").hidden = true;
}

function clearFacilities() {
  const ul = $("facilities");
  ul.replaceChildren();
  const li = document.createElement("li");
  li.textContent = "目標生産速度を入力すると、必要な施設数を表示します";
  ul.appendChild(li);
}

init();
