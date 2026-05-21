const statusUrl = "./assets/status.json";

const healthChecks = [
  { name: "Liveness", path: "/kleos/api/healthz" },
  { name: "Readiness", path: "/kleos/api/readyz" },
];

async function loadStatus() {
  const response = await fetch(statusUrl, { cache: "no-store" });
  if (!response.ok) {
    throw new Error(`status load failed: ${response.status}`);
  }
  return response.json();
}

function renderPhases(phases) {
  const grid = document.querySelector(".phase-grid");
  for (const phase of phases) {
    const card = document.createElement("article");
    card.className = `phase-card ${phase.state}`;
    card.innerHTML = `
      <div class="phase-meta">
        <span class="status-dot ${phase.state === "complete" ? "ok" : "warn"}"></span>
        <span class="badge ${phase.state}">${phase.label}</span>
      </div>
      <h3>${phase.name}</h3>
      <p>${phase.summary}</p>
    `;
    grid.appendChild(card);
  }
}

function renderTimeline(items) {
  const list = document.querySelector("#checkpoint-list");
  list.replaceChildren(
    ...items.map((item) => {
      const row = document.createElement("article");
      row.className = "timeline-item";
      row.innerHTML = `
        <div class="timeline-date">${item.date}</div>
        <div>
          <div class="timeline-title">${item.title}</div>
          <p class="timeline-text">${item.summary}</p>
        </div>
      `;
      return row;
    }),
  );
}

function healthRow(check, state, detail) {
  const dot = state === "ok" ? "ok" : state === "loading" ? "" : "bad";
  const row = document.createElement("div");
  row.className = "health-row";
  row.innerHTML = `
    <div class="health-name">
      <span class="status-dot ${dot}"></span>
      <span>${check.name}</span>
    </div>
    <div class="health-detail">${detail}</div>
  `;
  return row;
}

async function runHealthChecks() {
  const list = document.querySelector("#health-list");
  list.replaceChildren(...healthChecks.map((check) => healthRow(check, "loading", "Checking...")));

  const rows = await Promise.all(
    healthChecks.map(async (check) => {
      const started = performance.now();
      try {
        const response = await fetch(check.path, { cache: "no-store" });
        const elapsed = Math.round(performance.now() - started);
        if (!response.ok) {
          return healthRow(check, "bad", `${response.status} after ${elapsed}ms`);
        }
        const body = await response.json().catch(() => ({}));
        const detail = body.ok === true ? `Healthy in ${elapsed}ms` : `HTTP ${response.status} in ${elapsed}ms`;
        return healthRow(check, "ok", detail);
      } catch (error) {
        return healthRow(check, "bad", error instanceof Error ? error.message : "Request failed");
      }
    }),
  );
  list.replaceChildren(...rows);
}

async function init() {
  try {
    const status = await loadStatus();
    renderPhases(status.phases);
    renderTimeline(status.checkpoints);
  } catch (error) {
    const timeline = document.querySelector("#checkpoint-list");
    timeline.textContent = error instanceof Error ? error.message : "Could not load status";
  }

  document.querySelector("#refresh-health").addEventListener("click", runHealthChecks);
  await runHealthChecks();
}

init();
