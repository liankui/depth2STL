const DEFAULT_API_BASE = "http://localhost:31101/v1";
const STORAGE_HISTORY_KEY = "depth2stl_job_history";
const POLL_INTERVAL_MS = 2500;

const uploadForm = document.getElementById("uploadForm");
const uploadFileInput = document.getElementById("uploadFile");
const jobIdInput = document.getElementById("jobIdInput");
const queryJobBtn = document.getElementById("queryJobBtn");
const downloadImageBtn = document.getElementById("downloadImageBtn");
const downloadStlBtn = document.getElementById("downloadStlBtn");
const deleteJobBtn = document.getElementById("deleteJobBtn");
const refreshQueueBtn = document.getElementById("refreshQueueBtn");
const autoRefreshCheckbox = document.getElementById("autoRefresh");
const queueMetrics = document.getElementById("queueMetrics");
const queueStatusDetail = document.getElementById("queueStatusDetail");
const historyBody = document.getElementById("historyBody");
const previewImage = document.getElementById("previewImage");
const previewHint = document.getElementById("previewHint");
const logEl = document.getElementById("log");

let autoRefreshTimer = null;
let jobPollTimer = null;
let currentJobId = "";
let previewObjectUrl = "";
let history = readHistory();
const API_BASE = ((window.APP_CONFIG && window.APP_CONFIG.apiBaseUrl) || DEFAULT_API_BASE).replace(/\/$/, "");

function nowText() {
  return new Date().toLocaleString();
}

function log(message) {
  const line = document.createElement("div");
  line.textContent = `[${nowText()}] ${message}`;
  logEl.prepend(line);
}

function readHistory() {
  try {
    const raw = localStorage.getItem(STORAGE_HISTORY_KEY);
    return raw ? JSON.parse(raw) : [];
  } catch {
    return [];
  }
}

function saveHistory() {
  localStorage.setItem(STORAGE_HISTORY_KEY, JSON.stringify(history.slice(0, 100)));
}

function upsertJob(jobId, status = "unknown") {
  const found = history.find((item) => item.jobId === jobId);
  if (found) {
    found.status = status;
    found.updatedAt = nowText();
  } else {
    history.unshift({ jobId, status, updatedAt: nowText() });
  }
  saveHistory();
  renderHistory();
}

function setCurrentJobId(jobId) {
  jobIdInput.value = jobId;
  currentJobId = jobId;
}

async function setPreview(jobId, status) {
  if (previewObjectUrl) {
    URL.revokeObjectURL(previewObjectUrl);
    previewObjectUrl = "";
  }

  if (status !== "done") {
    previewImage.removeAttribute("src");
    previewHint.textContent = `任务 ${jobId} 当前状态: ${status}`;
    return;
  }

  const resp = await fetch(`${API_BASE}/relief/download/image/${jobId}`);
  if (!resp.ok) {
    previewImage.removeAttribute("src");
    previewHint.textContent = `任务 ${jobId} 已完成，但图片读取失败`;
    return;
  }
  const blob = await resp.blob();
  previewObjectUrl = URL.createObjectURL(blob);
  previewImage.src = previewObjectUrl;
  previewHint.textContent = `任务 ${jobId} 已完成，已渲染生成图片。`;
}

function renderHistory() {
  historyBody.innerHTML = "";
  history.forEach((item) => {
    const tr = document.createElement("tr");
    tr.innerHTML = `
      <td><button type="button" data-job-id="${item.jobId}" class="link-btn">${item.jobId}</button></td>
      <td>${item.status}</td>
      <td>${item.updatedAt}</td>
    `;
    historyBody.appendChild(tr);
  });

  historyBody.querySelectorAll(".link-btn").forEach((btn) => {
    btn.addEventListener("click", () => setCurrentJobId(btn.dataset.jobId || ""));
  });
}

function makeMetric(label, value) {
  return `
    <div class="metric-item">
      <div class="label">${label}</div>
      <div class="value">${value}</div>
    </div>
  `;
}

function renderQueueStatusDetail(data) {
  const queue = data.queue || {};
  const runtime = data.runtime || {};
  const lines = [
    `队列状态: queued=${queue.queued ?? 0}, processing=${queue.processing ?? 0}, done=${queue.done ?? 0}, failed=${queue.failed ?? 0}`,
    `运行状态: queueLength=${runtime.queueLength ?? 0}, activeWorkers=${runtime.activeWorkers ?? 0}, maxQueueSize=${runtime.maxQueueSize ?? 0}`,
  ];
  queueStatusDetail.innerHTML = lines.map((item) => `<div>${item}</div>`).join("");
}

function getJobId() {
  return jobIdInput.value.trim();
}

async function requestJson(path, options = {}) {
  const url = `${API_BASE}${path}`;
  const resp = await fetch(url, options);
  const text = await resp.text();
  let data = null;
  try {
    data = text ? JSON.parse(text) : {};
  } catch {
    data = text;
  }

  if (!resp.ok) {
    const msg = typeof data === "object" && data && data.error ? data.error : resp.statusText;
    throw new Error(`${resp.status} ${msg}`);
  }
  return data;
}

async function refreshQueueStatus() {
  try {
    const data = await requestJson("/relief/queue/status");
    queueMetrics.innerHTML = [
      makeMetric("总任务", data.totalJobs),
      makeMetric("排队", data.queue?.queued ?? 0),
      makeMetric("处理中", data.queue?.processing ?? 0),
      makeMetric("成功", data.queue?.done ?? 0),
      makeMetric("失败", data.queue?.failed ?? 0),
      makeMetric("当前队列长度", data.runtime?.queueLength ?? 0),
      makeMetric("活跃 Worker", data.runtime?.activeWorkers ?? 0),
      makeMetric("队列容量", data.runtime?.maxQueueSize ?? 0),
    ].join("");
    renderQueueStatusDetail(data);
  } catch (err) {
    log(`队列状态刷新失败: ${err.message}`);
  }
}

async function queryJob(jobId) {
  if (!jobId) {
    throw new Error("请先输入 jobId");
  }
  const data = await requestJson(`/relief/${jobId}`);
  const status = data.status || "unknown";
  upsertJob(jobId, status);
  await setPreview(jobId, status);
  log(`任务 ${jobId} 状态: ${status}`);
  return data;
}

async function downloadFile(jobId, kind) {
  if (!jobId) {
    throw new Error("请先输入 jobId");
  }
  const endpoint = kind === "image" ? "image" : "stl";
  const url = `${API_BASE}/relief/download/${endpoint}/${jobId}`;
  const resp = await fetch(url);

  if (!resp.ok) {
    const data = await resp.json().catch(() => null);
    const msg = data?.error || `${resp.status} ${resp.statusText}`;
    throw new Error(msg);
  }

  const blob = await resp.blob();
  const ext = kind === "image" ? "png" : "stl";
  const a = document.createElement("a");
  const objectUrl = URL.createObjectURL(blob);
  a.href = objectUrl;
  a.download = `${jobId}.${ext}`;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(objectUrl);
}

function setupAutoRefresh() {
  if (autoRefreshTimer) {
    clearInterval(autoRefreshTimer);
    autoRefreshTimer = null;
  }
  if (autoRefreshCheckbox.checked) {
    autoRefreshTimer = setInterval(refreshQueueStatus, 5000);
  }
}

function startPollingJob(jobId) {
  if (!jobId) {
    return;
  }
  currentJobId = jobId;
  if (jobPollTimer) {
    clearInterval(jobPollTimer);
    jobPollTimer = null;
  }

  const poll = async () => {
    try {
      const data = await queryJob(jobId);
      if (data.status === "done" || data.status === "failed") {
        clearInterval(jobPollTimer);
        jobPollTimer = null;
      }
    } catch (err) {
      log(`轮询任务失败: ${err.message}`);
    }
  };

  poll();
  jobPollTimer = setInterval(poll, POLL_INTERVAL_MS);
}

uploadForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  if (!uploadFileInput.files || uploadFileInput.files.length === 0) {
    log("请先选择图片文件");
    return;
  }
  const form = new FormData();
  form.append("file", uploadFileInput.files[0]);

  try {
    const data = await requestJson("/relief", {
      method: "POST",
      body: form,
    });
    const jobId = data.jobId || data.id;
    if (jobId) {
      upsertJob(jobId, "queued");
      setCurrentJobId(jobId);
      log(`任务创建成功: ${jobId}`);
      startPollingJob(jobId);
    }
    uploadForm.reset();
    refreshQueueStatus();
  } catch (err) {
    log(`创建任务失败: ${err.message}`);
  }
});

queryJobBtn.addEventListener("click", async () => {
  try {
    const jobId = getJobId();
    await queryJob(jobId);
    startPollingJob(jobId);
  } catch (err) {
    log(`查询任务失败: ${err.message}`);
  }
});

downloadImageBtn.addEventListener("click", async () => {
  const jobId = getJobId();
  try {
    await downloadFile(jobId, "image");
    log(`图片下载完成: ${jobId}.png`);
  } catch (err) {
    log(`下载图片失败: ${err.message}`);
  }
});

downloadStlBtn.addEventListener("click", async () => {
  const jobId = getJobId();
  try {
    await downloadFile(jobId, "stl");
    log(`STL 下载完成: ${jobId}.stl`);
  } catch (err) {
    log(`下载 STL 失败: ${err.message}`);
  }
});

deleteJobBtn.addEventListener("click", async () => {
  const jobId = getJobId();
  if (!jobId) {
    log("请先输入 jobId");
    return;
  }

  try {
    await requestJson(`/relief/queue/${jobId}`, { method: "DELETE" });
    upsertJob(jobId, "deleted");
    if (currentJobId === jobId) {
      if (previewObjectUrl) {
        URL.revokeObjectURL(previewObjectUrl);
        previewObjectUrl = "";
      }
      previewImage.removeAttribute("src");
      previewHint.textContent = `任务 ${jobId} 已删除`;
    }
    log(`任务已删除: ${jobId}`);
    refreshQueueStatus();
  } catch (err) {
    log(`删除任务失败: ${err.message}`);
  }
});

refreshQueueBtn.addEventListener("click", refreshQueueStatus);
autoRefreshCheckbox.addEventListener("change", setupAutoRefresh);

function init() {
  renderHistory();
  refreshQueueStatus();
  setupAutoRefresh();
  log(`前端已初始化，API Base: ${API_BASE}`);
}

init();
