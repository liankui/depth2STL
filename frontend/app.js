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
const sourceImage = document.getElementById("sourceImage");
const previewHint = document.getElementById("previewHint");
const jobStatusBadge = document.getElementById("jobStatusBadge");
const pollingFlag = document.getElementById("pollingFlag");
const logEl = document.getElementById("log");

const API_BASE = ((window.APP_CONFIG && window.APP_CONFIG.apiBaseUrl) || DEFAULT_API_BASE).replace(/\/$/, "");

let autoRefreshTimer = null;
let jobPollTimer = null;
let currentJobId = "";
let previewObjectUrl = "";
let sourceObjectUrl = "";
let history = readHistory();

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
  currentJobId = jobId;
  jobIdInput.value = jobId;
}

function setStatusBadge(status) {
  const finalStatus = status || "idle";
  jobStatusBadge.textContent = finalStatus;
  jobStatusBadge.className = `status-badge status-${finalStatus}`;
  const done = finalStatus === "done";
  downloadImageBtn.disabled = !done;
  downloadStlBtn.disabled = !done;
}

function setPolling(active) {
  pollingFlag.textContent = active ? "轮询中..." : "未轮询";
  pollingFlag.classList.toggle("polling-active", active);
}

function setSourcePreview(file) {
  if (sourceObjectUrl) {
    URL.revokeObjectURL(sourceObjectUrl);
    sourceObjectUrl = "";
  }
  if (!file) {
    sourceImage.removeAttribute("src");
    return;
  }
  sourceObjectUrl = URL.createObjectURL(file);
  sourceImage.src = sourceObjectUrl;
}

async function setResultPreview(jobId, status) {
  if (previewObjectUrl) {
    URL.revokeObjectURL(previewObjectUrl);
    previewObjectUrl = "";
  }

  if (status !== "done") {
    previewImage.removeAttribute("src");
    previewHint.textContent = `任务 ${jobId} 当前状态：${status}`;
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
  previewHint.textContent = `任务 ${jobId} 已完成，可直接下载深度图和 STL。`;
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
    btn.addEventListener("click", async () => {
      const jobId = btn.dataset.jobId || "";
      setCurrentJobId(jobId);
      await queryAndRender(jobId);
      if (jobStatusBadge.textContent === "queued" || jobStatusBadge.textContent === "processing") {
        startPollingJob(jobId);
      }
    });
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
  queueStatusDetail.innerHTML = `
    <div class="queue-line">队列: queued=${queue.queued ?? 0}, processing=${queue.processing ?? 0}, done=${queue.done ?? 0}, failed=${queue.failed ?? 0}</div>
    <div class="queue-line">运行时: queueLength=${runtime.queueLength ?? 0}, activeWorkers=${runtime.activeWorkers ?? 0}, maxQueueSize=${runtime.maxQueueSize ?? 0}</div>
  `;
}

function getJobId() {
  return jobIdInput.value.trim();
}

async function requestJson(path, options = {}) {
  const resp = await fetch(`${API_BASE}${path}`, options);
  const text = await resp.text();
  let data;
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
      makeMetric("总任务", data.totalJobs ?? 0),
      makeMetric("排队", data.queue?.queued ?? 0),
      makeMetric("处理中", data.queue?.processing ?? 0),
      makeMetric("成功", data.queue?.done ?? 0),
      makeMetric("失败", data.queue?.failed ?? 0),
      makeMetric("队列长度", data.runtime?.queueLength ?? 0),
      makeMetric("活跃 Worker", data.runtime?.activeWorkers ?? 0),
      makeMetric("队列容量", data.runtime?.maxQueueSize ?? 0),
    ].join("");
    renderQueueStatusDetail(data);
  } catch (err) {
    log(`队列刷新失败: ${err.message}`);
  }
}

async function queryAndRender(jobId) {
  if (!jobId) {
    throw new Error("请先输入 jobId");
  }
  const data = await requestJson(`/relief/${jobId}`);
  const status = data.status || "unknown";
  setStatusBadge(status);
  upsertJob(jobId, status);
  await setResultPreview(jobId, status);
  log(`任务 ${jobId} 状态: ${status}`);
  return data;
}

async function downloadFile(jobId, kind) {
  if (!jobId) {
    throw new Error("请先输入 jobId");
  }
  const endpoint = kind === "image" ? "image" : "stl";
  const resp = await fetch(`${API_BASE}/relief/download/${endpoint}/${jobId}`);
  if (!resp.ok) {
    const data = await resp.json().catch(() => null);
    throw new Error(data?.error || `${resp.status} ${resp.statusText}`);
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

function stopPollingJob() {
  if (jobPollTimer) {
    clearInterval(jobPollTimer);
    jobPollTimer = null;
  }
  setPolling(false);
}

function startPollingJob(jobId) {
  if (!jobId) {
    return;
  }
  setCurrentJobId(jobId);
  stopPollingJob();
  setPolling(true);

  const poll = async () => {
    try {
      const data = await queryAndRender(jobId);
      if (data.status === "done" || data.status === "failed") {
        stopPollingJob();
      }
    } catch (err) {
      log(`轮询失败: ${err.message}`);
      stopPollingJob();
    }
  };

  poll();
  jobPollTimer = setInterval(poll, POLL_INTERVAL_MS);
}

uploadFileInput.addEventListener("change", () => {
  const file = uploadFileInput.files && uploadFileInput.files[0] ? uploadFileInput.files[0] : null;
  setSourcePreview(file);
});

uploadForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  const file = uploadFileInput.files && uploadFileInput.files[0] ? uploadFileInput.files[0] : null;
  if (!file) {
    log("请先选择图片");
    return;
  }

  const form = new FormData();
  form.append("file", file);

  try {
    const data = await requestJson("/relief", { method: "POST", body: form });
    const jobId = data.jobId || data.id;
    if (!jobId) {
      throw new Error("响应缺少 jobId");
    }
    setCurrentJobId(jobId);
    setStatusBadge("queued");
    upsertJob(jobId, "queued");
    previewHint.textContent = `任务 ${jobId} 已创建，等待处理...`;
    log(`任务创建成功: ${jobId}`);
    startPollingJob(jobId);
    refreshQueueStatus();
  } catch (err) {
    log(`创建任务失败: ${err.message}`);
  }
});

queryJobBtn.addEventListener("click", async () => {
  try {
    const jobId = getJobId();
    const data = await queryAndRender(jobId);
    if (data.status === "queued" || data.status === "processing") {
      startPollingJob(jobId);
    } else {
      stopPollingJob();
    }
  } catch (err) {
    log(`查询失败: ${err.message}`);
  }
});

downloadImageBtn.addEventListener("click", async () => {
  try {
    await downloadFile(getJobId(), "image");
    log(`深度图下载完成`);
  } catch (err) {
    log(`下载深度图失败: ${err.message}`);
  }
});

downloadStlBtn.addEventListener("click", async () => {
  try {
    await downloadFile(getJobId(), "stl");
    log(`STL 下载完成`);
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
    setStatusBadge("deleted");
    previewImage.removeAttribute("src");
    previewHint.textContent = `任务 ${jobId} 已删除`;
    if (currentJobId === jobId) {
      stopPollingJob();
    }
    log(`任务已删除: ${jobId}`);
    refreshQueueStatus();
  } catch (err) {
    log(`删除失败: ${err.message}`);
  }
});

refreshQueueBtn.addEventListener("click", refreshQueueStatus);
autoRefreshCheckbox.addEventListener("change", setupAutoRefresh);

function init() {
  setStatusBadge("idle");
  setPolling(false);
  renderHistory();
  refreshQueueStatus();
  setupAutoRefresh();
  log(`前端初始化完成，API Base: ${API_BASE}`);
}

init();
