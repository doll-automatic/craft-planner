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
  facilityItems: [],
  // リロードすると「すべて」と「必要素材」に戻る。表示切替は同期しない。
  // summaryMode の primary は先頭レシピに前提がない 1 次素材だけ。
  summaryMode: "all",
  gameReq: 0,
  treeReq: 0,
  planReq: 0,
  resultMode: "materials",
  role: "host",
  // 未取得は -1。サーバの初期 revision 0 でも初回は適用する。
  revision: -1,
  pulling: false,
  heldQuantity: null,
  // ゲームごとの現在個数メモ。ホストとクライアントでは共有しない。
  stock: {},
};

const $ = (id) => document.getElementById(id);

function init() {
  ui.role = document.documentElement.getAttribute("data-role") === "client" ? "client" : "host";
  if (ui.role === "client") {
    document.title = "生産管理（クライアント）";
    $("app-title").textContent = "生産管理（クライアント）";
  }
  applyTheme(savedOrSystemTheme());
  loadStock();
  $("theme-toggle").addEventListener("click", toggleTheme);
  $("qr-toggle").addEventListener("click", () => {
    openJoinQR().catch((err) => console.error(err));
  });
  $("qr-close").addEventListener("click", () => $("qr-dialog").close());
  $("game-select").addEventListener("change", () => {
    changeGame($("game-select").value);
  });
  $("type-filter").addEventListener("change", () => {
    renderItemOptions();
  });
  $("name-filter").addEventListener("input", () => {
    renderItemOptions();
  });
  $("item-select").addEventListener("change", () => {
    changeItem($("item-select").value);
  });
  $("quantity-input").addEventListener("input", () => {
    ui.heldQuantity = null;
  });
  $("quantity-input").addEventListener("change", () => {
    commitQuantity();
  });
  $("quantity-input").addEventListener("blur", () => {
    // フォーカス中に届いた値は、その後に打ち直していなければここで反映する。
    if (ui.heldQuantity == null || $("quantity-input").value === ui.heldQuantity) {
      ui.heldQuantity = null;
      return;
    }
    $("quantity-input").value = ui.heldQuantity;
    ui.heldQuantity = null;
    applyQuantity(currentQuantity());
  });
  document.querySelectorAll('input[name="result-mode"]').forEach((input) => {
    input.addEventListener("change", () => {
      if (!input.checked) return;
      ui.resultMode = input.value === "materials" ? "materials" : "facilities";
      applyResultMode();
    });
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
  applyResultMode();
  clearFacilities();
  renderMaterials([]);
  loadGameList()
    .catch((err) => {
      console.error(err);
      renderErrors([{ message: "サーバに接続できませんでした" }]);
    })
    .finally(() => {
      pullState();
      // shortcut: 反映は1秒間隔の取得。同時編集は後勝ち。届ける方式が必要になったら置き換える。
      setInterval(pullState, 1000);
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
  const req = ++ui.gameReq;
  ui.treeReq++;
  ui.planReq++;
  clearItemView();
  clearRate();
  resetItemFilter();
  ui.gameId = gameId || "";
  ui.game = null;
  $("empty-game").hidden = true;
  renderTypeOptions();
  renderItemOptions();
  if (!ui.gameId) return;
  let res;
  try {
    res = await fetch("/api/games/" + encodeURIComponent(ui.gameId));
  } catch (err) {
    console.error(err);
    if (req !== ui.gameReq) return;
    $("empty-game").hidden = false;
    $("empty-game").textContent = "サーバに接続できませんでした";
    return;
  }
  if (req !== ui.gameReq) return;
  if (!res.ok) {
    $("empty-game").hidden = false;
    $("empty-game").textContent = "ゲームを読み込めませんでした";
    return;
  }
  ui.game = await res.json();
  if (req !== ui.gameReq) return;
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
  const quantity = currentQuantity();
  ui.materials = [];
  ui.facilityItems = [];
  renderMaterials([]);
  clearFacilities();
  if (!quantity.empty && !quantity.ok) {
    $("quantity-error").hidden = false;
    $("quantity-error").textContent = "目標速度が不正なため、一覧は表示していません。";
  } else {
    $("quantity-error").hidden = true;
  }
  if (ui.role !== "client") await loadTree(id);
  if (ui.itemId !== id) return;
  if (quantity.empty) await loadPlan(minimumUnit(item));
  else if (quantity.ok) await loadPlan(quantity.value);
}

async function loadTree(id) {
  const req = ++ui.treeReq;
  const gameId = ui.gameId;
  const url = "/api/games/" + encodeURIComponent(gameId) + "/tree?itemId=" + encodeURIComponent(id);
  let res;
  try {
    res = await fetch(url);
  } catch (err) {
    console.error(err);
    if (req === ui.treeReq) $("tree-notice").textContent = "サーバに接続できませんでした";
    return;
  }
  if (req !== ui.treeReq || ui.itemId !== id || ui.gameId !== gameId) return;
  if (!res.ok) {
    $("tree-notice").textContent = "生産ツリーを取得できませんでした";
    return;
  }
  const data = await res.json();
  if (req !== ui.treeReq || ui.itemId !== id || ui.gameId !== gameId) return;
  const notes = [];
  if (data.truncated) notes.push("ノード数が上限（1000）に達したため、一部が未展開です。");
  if (data.cycle && data.cycle.length) {
    notes.push("循環を検出しました: " + data.cycle.join(" → ") + "。循環ノードは再展開せずに表示しています。");
  }
  $("tree-notice").textContent = notes.join(" ");
  renderGraph(data);
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

function facilityByNode(id) {
  return ui.facilityItems.find((item) => item.itemId === id) || null;
}

// 目標速度（未入力時は最小単位）の算出後は、選択中の結果だけを出す。
function metricLine(data) {
  const facility = data.facility || "";
  const row = facilityByNode(data.id);
  if (!row) {
    if (data.outputRate == null || !Number.isFinite(Number(data.outputRate))) return facility;
    const rate = formatNum(Number(data.outputRate)) + "/秒";
    return facility ? facility + " " + rate : rate;
  }
  // 必要個数は生産時間が無くても求まる。算出不可は施設数の表示にだけ関わる。
  if (ui.resultMode === "materials") return "必要 " + displayQuantity(row.requiredQuantity) + " 個";
  if (row.uncomputable) return row.reason || "算出できません";
  return (facility ? facility + " " : "") + row.facilityCount + " 台";
}

function buildNodeCard(data) {
  const card = document.createElement("div");
  card.className = "node-card" + (data.hasInputs ? "" : " leaf");
  card.dataset.nodeId = data.id || "";
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
  mid.textContent = metricLine(data);
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

function applyResultMode() {
  const materials = ui.resultMode === "materials";
  $("materials-panel").hidden = !materials;
  $("facilities-panel").hidden = materials;
  refreshNodeMetrics();
}

function refreshNodeMetrics() {
  document.querySelectorAll("#cy .node-card").forEach((card) => {
    const mid = card.querySelector(".node-card-rate");
    const node = ui.cy && ui.cy.getElementById(card.dataset.nodeId);
    if (!mid || !node || node.empty()) return;
    mid.textContent = metricLine(node.data());
    mid.title = mid.textContent;
  });
}

function renderMaterials(list) {
  ui.materials = Array.isArray(list) ? list : [];
  const ul = $("materials");
  ul.replaceChildren();
  // 個数はサーバが目標速度倍した値。ここでは一覧の絞り込みだけ行い、グラフのノードは消さない。
  const shown = ui.summaryMode === "primary" ? ui.materials.filter((line) => line.isPrimary) : ui.materials;
  if (!ui.materials.length) {
    const li = document.createElement("li");
    li.textContent = ui.facilityItems.length ? "素材が存在しません" : "目標速度を入力すると、必要な素材を表示します";
    ul.appendChild(li);
    return;
  }
  if (!shown.length) {
    const li = document.createElement("li");
    li.textContent = "1次素材はありません";
    ul.appendChild(li);
    return;
  }
  const focused = document.activeElement;
  const focusedId = focused && focused.classList && focused.classList.contains("stock-memo") ? focused.dataset.itemId : "";
  const selStart = focusedId ? focused.selectionStart : null;
  const selEnd = focusedId ? focused.selectionEnd : null;
  for (const line of shown) {
    const li = document.createElement("li");
    li.className = "item-row";
    appendItemIcon(li, line.itemId);
    const text = document.createElement("span");
    const targetText = displayQuantity(line.quantity);
    text.textContent = line.name + " × " + targetText;
    const memo = document.createElement("input");
    memo.type = "text";
    memo.className = "stock-memo";
    memo.inputMode = "numeric";
    memo.autocomplete = "off";
    memo.placeholder = "現在";
    memo.setAttribute("aria-label", line.name + "の現在の個数");
    memo.dataset.itemId = line.itemId;
    memo.value = stockOf(line.itemId);
    memo.addEventListener("input", () => {
      writeStock(line.itemId, memo.value);
      markStock(li, memo.value, targetText);
    });
    markStock(li, memo.value, targetText);
    li.append(text, memo);
    ul.appendChild(li);
    if (focusedId && focusedId === line.itemId) {
      memo.focus();
      if (selStart != null && selEnd != null) memo.setSelectionRange(selStart, selEnd);
    }
  }
}

const STOCK_KEY = "gpm-stock";

function loadStock() {
  try {
    const raw = localStorage.getItem(STOCK_KEY);
    const parsed = raw ? JSON.parse(raw) : {};
    ui.stock = parsed && typeof parsed === "object" ? parsed : {};
  } catch (err) {
    ui.stock = {};
  }
}

function stockOf(itemId) {
  const game = ui.stock[ui.gameId];
  if (!game || game[itemId] == null) return "";
  return String(game[itemId]);
}

function writeStock(itemId, value) {
  if (!ui.gameId || !itemId) return;
  if (!ui.stock[ui.gameId] || typeof ui.stock[ui.gameId] !== "object") ui.stock[ui.gameId] = {};
  const trimmed = value.trim();
  if (trimmed === "") delete ui.stock[ui.gameId][itemId];
  else ui.stock[ui.gameId][itemId] = trimmed;
  try {
    localStorage.setItem(STOCK_KEY, JSON.stringify(ui.stock));
  } catch (err) {
    // 保存できなくても、画面上のメモはそのまま残す。
  }
}

// 現在数が、行に出している目標個数を超えたときだけ暗くする。
function markStock(li, raw, targetText) {
  const current = Number(String(raw).trim());
  const target = Number(targetText);
  const over = String(raw).trim() !== "" && Number.isFinite(current) && Number.isFinite(target) && current > target;
  li.classList.toggle("is-over", over);
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

// minimumUnit は先頭レシピが1回で出す主産物の個数。未定義や 0 以下は 1。
function minimumUnit(item) {
  const recipe = item && Array.isArray(item.recipes) ? item.recipes[0] : null;
  if (!recipe || !Array.isArray(recipe.outputs)) return 1;
  const main = recipe.outputs.find((out) => out && out.id === recipe.mainOutput);
  const qty = main ? Number(main.qty) : NaN;
  if (!Number.isInteger(qty) || qty < 1) return 1;
  return qty;
}
function currentQuantity() {
  const raw = $("quantity-input").value.trim();
  if (raw === "") return { ok: false, empty: true };
  if (!/^(?:0|[1-9]\d*)(?:\.\d+)?$|^\.\d+$/.test(raw)) return { ok: false, empty: false };
  const value = Number(raw);
  if (!Number.isFinite(value) || value <= 0 || value > 1000000000) return { ok: false, empty: false };
  return { ok: true, value, empty: false };
}

function commitQuantity() {
  // 確定した値を送る。直後の blur で、入力中に届いた古い値へ戻さない。
  ui.heldQuantity = null;
  const quantity = currentQuantity();
  applyQuantity(quantity);
  if (!quantity.empty && !quantity.ok) return;
  pushState({ quantity: $("quantity-input").value.trim() });
}

function applyQuantity(quantity) {
  if (quantity.empty) {
    $("quantity-error").hidden = true;
    const item = itemById(ui.itemId);
    if (item) loadPlan(minimumUnit(item));
    return;
  }
  if (!quantity.ok) {
    $("quantity-error").hidden = false;
    $("quantity-error").textContent = "目標速度は 0 より大きい数で入力してください。直前の表示を保持しています。";
    return;
  }
  $("quantity-error").hidden = true;
  if (ui.itemId) loadPlan(quantity.value);
}

// change は利用者の操作でだけ起きる（value の代入では起きない）ので、ここからは常に送る。
async function changeGame(gameId) {
  await selectGame(gameId);
  await pushState({ gameId: gameId || "", quantity: "" });
}

async function changeItem(id) {
  if (!id) {
    clearItemView();
    await pushState({ itemId: "" });
    return;
  }
  await selectItem(id);
  await pushState({ itemId: id });
}

// 操作の送信と重なった取得は捨てる。古い応答で入力を戻さない。
// 反映の途中で利用者が操作したら（syncEpoch が進んだら）、反映をやめて利用者の操作を残す。
let syncEpoch = 0;

async function pushState(patch) {
  const ticket = ++syncEpoch;
  let res;
  try {
    res = await fetch("/api/state", {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(patch),
    });
  } catch (err) {
    console.error(err);
    return;
  }
  if (ticket !== syncEpoch || !res.ok) return;
  const state = await res.json();
  if (ticket !== syncEpoch) return;
  if (typeof state.revision === "number" && state.revision > ui.revision) ui.revision = state.revision;
}

async function pullState() {
  if (ui.pulling) return;
  ui.pulling = true;
  const ticket = syncEpoch;
  try {
    const res = await fetch("/api/state");
    if (ticket !== syncEpoch || !res.ok) return;
    const state = await res.json();
    if (ticket !== syncEpoch) return;
    if (typeof state.revision !== "number" || state.revision <= ui.revision) {
      healQuantity(state.quantity || "");
      return;
    }
    await applyRemote(state, ticket);
  } catch (err) {
    console.error(err);
  } finally {
    ui.pulling = false;
  }
}

async function applyRemote(state, ticket) {
  const quantityFocused = document.activeElement === $("quantity-input");
  const gameId = state.gameId || "";
  const itemId = state.itemId || "";
  const quantity = state.quantity || "";
  const gameChanged = gameId !== ui.gameId;
  const itemChanged = itemId !== ui.itemId;
  const prevQuantity = $("quantity-input").value;
  if (gameChanged) {
    $("game-select").value = gameId;
    await selectGame(gameId);
    if (ticket !== syncEpoch) return;
  }
  if (!quantityFocused) {
    $("quantity-input").value = quantity;
    ui.heldQuantity = null;
  } else if (quantity !== $("quantity-input").value) {
    ui.heldQuantity = quantity;
  }
  if (itemId) {
    if (gameChanged || itemChanged) {
      revealItem(itemId);
      $("item-select").value = itemId;
      await selectItem(itemId);
    } else if (!quantityFocused && quantity !== prevQuantity) {
      applyQuantity(currentQuantity());
    }
  } else if (itemChanged || gameChanged) {
    clearItemView();
  }
  if (ticket === syncEpoch && state.revision > ui.revision) ui.revision = state.revision;
}

// 反映を見送った目標速度を、フォーカスが外れたあとに合わせる。
function healQuantity(quantity) {
  if (document.activeElement === $("quantity-input")) {
    if (quantity !== $("quantity-input").value) ui.heldQuantity = quantity;
    return;
  }
  if ($("quantity-input").value === quantity) {
    ui.heldQuantity = null;
    return;
  }
  $("quantity-input").value = quantity;
  ui.heldQuantity = null;
  applyQuantity(currentQuantity());
}

// 絞り込みで見えない相手のアイテムは、絞り込みを外して一覧に戻す。
function revealItem(id) {
  const item = itemById(id);
  if (!item) return;
  const type = $("type-filter").value;
  const query = $("name-filter").value.trim().toLowerCase();
  const hidden = (type && item.type !== type) || (query && !String(item.name || "").toLowerCase().includes(query));
  if (!hidden) return;
  $("type-filter").value = "";
  $("name-filter").value = "";
  renderItemOptions();
}

async function openJoinQR() {
  const dialog = $("qr-dialog");
  let url = "";
  try {
    const res = await fetch("/api/join");
    if (res.ok) {
      const data = await res.json();
      url = data.url || "";
    }
  } catch (err) {
    console.error(err);
  }
  $("qr-url").textContent = url || "接続URLを取得できませんでした";
  $("qr-note").hidden = !/^https?:\/\/(localhost|127\.0\.0\.1)([:/]|$)/.test(url);
  const box = $("qr-code");
  box.replaceChildren();
  if (url && typeof QRCode === "function") {
    new QRCode(box, {
      text: url,
      width: 196,
      height: 196,
      correctLevel: QRCode.CorrectLevel.M,
    });
  }
  if (!dialog.open) dialog.showModal();
}

async function loadPlan(quantity) {
  const req = ++ui.planReq;
  const gameId = ui.gameId;
  const itemId = ui.itemId;
  const url = "/api/games/" + encodeURIComponent(gameId) + "/facilities?itemId=" + encodeURIComponent(itemId) + "&quantity=" + encodeURIComponent(quantity);
  let res;
  try {
    res = await fetch(url);
  } catch (err) {
    console.error(err);
    if (req === ui.planReq) {
      $("quantity-error").hidden = false;
      $("quantity-error").textContent = "サーバに接続できませんでした";
    }
    return;
  }
  if (req !== ui.planReq || ui.itemId !== itemId || ui.gameId !== gameId) return;
  if (!res.ok) {
    $("quantity-error").hidden = false;
    $("quantity-error").textContent = "目標速度が不正なため、算出を行いませんでした。直前の表示を保持しています。";
    return;
  }
  const data = await res.json();
  if (req !== ui.planReq || ui.itemId !== itemId || ui.gameId !== gameId) return;
  ui.facilityItems = data.items || [];
  renderFacilities(ui.facilityItems);
  renderMaterials(data.materials || []);
  refreshNodeMetrics();
}

function renderFacilities(items) {
  const ul = $("facilities");
  ul.replaceChildren();
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
      text.textContent = item.name + ": " + (item.reason || "算出できません");
    } else {
      text.textContent = item.name + ": " + item.facilityCount + " 台（必要 " + displayQuantity(item.requiredQuantity) + " 個、1台あたり " + formatNum(item.outputRate) + " 個/秒）";
    }
    li.appendChild(text);
    ul.appendChild(li);
  }
}

// 整数に近い理論値はそのまま出し、端数があるときだけ切り上げて個数にする。
function displayQuantity(n) {
  if (!Number.isFinite(Number(n))) return "-";
  const value = Number(n);
  const nearest = Math.round(value);
  if (Math.abs(value - nearest) < 1e-6) return String(nearest);
  return String(Math.ceil(value - 1e-9));
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
  ui.materials = [];
  ui.facilityItems = [];
  renderMaterials([]);
  clearFacilities();
  refreshNodeMetrics();
  const sel = $("item-select");
  if (sel) sel.value = "";
}

function clearRate() {
  $("quantity-input").value = "";
  $("quantity-error").hidden = true;
}

function clearFacilities() {
  const ul = $("facilities");
  ul.replaceChildren();
  const li = document.createElement("li");
  li.textContent = "目標速度を入力すると、必要な施設数を表示します";
  ul.appendChild(li);
}

init();
