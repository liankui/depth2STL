const API_BASE = "/v1";
const MAX_FILE_SIZE = 10 * 1024 * 1024;
const POLL_INTERVAL_MS = 2500;
const POLLABLE_STATUSES = new Set(["queued", "processing"]);

const uploadForm = document.getElementById("uploadForm");
const uploadFileInput = document.getElementById("uploadFile");
const fileMeta = document.getElementById("fileMeta");
const fileError = document.getElementById("fileError");
const sourceFrame = document.getElementById("sourceFrame");
const sourceImage = document.getElementById("sourceImage");
const sourcePlaceholder = document.getElementById("sourcePlaceholder");
const createJobBtn = document.getElementById("createJobBtn");
const jobIdInput = document.getElementById("jobIdInput");
const jobStatusBadge = document.getElementById("jobStatusBadge");
const statusMessage = document.getElementById("statusMessage");
const resultFrame = document.getElementById("resultFrame");
const previewImage = document.getElementById("previewImage");
const previewHint = document.getElementById("previewHint");
const downloadImageBtn = document.getElementById("downloadImageBtn");
const downloadStlBtn = document.getElementById("downloadStlBtn");

let currentJobId = "";
let sourceObjectUrl = "";
let resultObjectUrl = "";
let jobPollTimer = null;

function formatSize(size) {
  if (size < 1024 * 1024) {
    return `${Math.max(1, Math.round(size / 1024))} KB`;
  }
  return `${(size / (1024 * 1024)).toFixed(2)} MB`;
}

function setFrameImage(frame, img, placeholder, objectUrl, emptyText) {
  if (objectUrl) {
    img.src = objectUrl;
    frame.classList.add("has-image");
    placeholder.textContent = "";
    return;
  }

  img.removeAttribute("src");
  frame.classList.remove("has-image");
  placeholder.textContent = emptyText;
}

function setStatus(status) {
  const safeStatus = status || "idle";
  jobStatusBadge.textContent = safeStatus;
  jobStatusBadge.className = `badge badge-${safeStatus}`;
  const done = safeStatus === "done";
  downloadImageBtn.disabled = !done;
  downloadStlBtn.disabled = !done;
  if (!done) {
    downloadImageBtn.classList.remove("is-downloaded");
    downloadStlBtn.classList.remove("is-downloaded");
  }
}

function setCurrentJobId(jobId) {
  currentJobId = jobId || "";
  jobIdInput.value = currentJobId;
}

function showFileError(message) {
  if (!message) {
    fileError.textContent = "";
    fileError.classList.add("hidden");
    return;
  }
  fileError.textContent = message;
  fileError.classList.remove("hidden");
}

function stopPolling() {
  if (jobPollTimer) {
    clearInterval(jobPollTimer);
    jobPollTimer = null;
  }
}

function clearResultPreview(message) {
  if (resultObjectUrl) {
    URL.revokeObjectURL(resultObjectUrl);
    resultObjectUrl = "";
  }
  setFrameImage(resultFrame, previewImage, previewHint, "", message);
}

function validateFile(file) {
  if (!file) {
    return "请先选择图片";
  }
  if (file.size > MAX_FILE_SIZE) {
    return "图片大小不能超过 10MB";
  }
  return "";
}

function updateSelectedFile() {
  const file = uploadFileInput.files && uploadFileInput.files[0] ? uploadFileInput.files[0] : null;

  if (sourceObjectUrl) {
    URL.revokeObjectURL(sourceObjectUrl);
    sourceObjectUrl = "";
  }

  if (!file) {
    fileMeta.textContent = "未选择文件";
    showFileError("");
    setFrameImage(sourceFrame, sourceImage, sourcePlaceholder, "", "选择图片后这里显示原图。");
    return;
  }

  const error = validateFile(file);
  if (error) {
    uploadFileInput.value = "";
    fileMeta.textContent = "未选择文件";
    showFileError(error);
    setFrameImage(sourceFrame, sourceImage, sourcePlaceholder, "", "选择图片后这里显示原图。");
    return;
  }

  showFileError("");
  fileMeta.textContent = formatSize(file.size);
  sourceObjectUrl = URL.createObjectURL(file);
  setFrameImage(sourceFrame, sourceImage, sourcePlaceholder, sourceObjectUrl, "");
}

async function requestJson(path, options = {}) {
  let response;
  try {
    response = await fetch(`${API_BASE}${path}`, options);
  } catch {
    throw new Error("接口无法连接，请确认服务已启动");
  }

  const text = await response.text();
  let data;
  try {
    data = text ? JSON.parse(text) : {};
  } catch {
    data = text;
  }

  if (!response.ok) {
    let message = typeof data === "object" && data && data.error ? data.error : `${response.status} ${response.statusText}`;
    if (response.status === 404) {
      message = "404 Not Found，请确认启动的是 Web 服务入口：go run ./cmd";
    }
    throw new Error(message);
  }

  return data;
}

async function loadResultPreview(jobId) {
  let response;
  try {
    response = await fetch(`${API_BASE}/relief/download/image/${jobId}`);
  } catch {
    throw new Error("生成图接口无法连接");
  }

  if (!response.ok) {
    const data = await response.json().catch(() => null);
    throw new Error(data && data.error ? data.error : "image 读取失败");
  }

  if (resultObjectUrl) {
    URL.revokeObjectURL(resultObjectUrl);
  }

  const blob = await response.blob();
  resultObjectUrl = URL.createObjectURL(blob);
  setFrameImage(resultFrame, previewImage, previewHint, resultObjectUrl, "");
}

function getStatusText(status, error) {
  if (status === "queued") {
    return "任务已创建，等待处理。";
  }
  if (status === "processing") {
    return "任务处理中，页面会继续自动查询。";
  }
  if (status === "done") {
    return "任务已完成，可以查看生成图并下载 image 与 STL。";
  }
  if (status === "failed") {
    return error ? `任务失败：${error}` : "任务失败。";
  }
  return "创建任务后会返回 jobId，并自动开始查询。";
}

async function queryJob(jobId) {
  if (!jobId) {
    throw new Error("请先输入 jobId");
  }

  const data = await requestJson(`/relief/${jobId}`);
  const status = data.status || "idle";

  setCurrentJobId(jobId);
  setStatus(status);
  statusMessage.textContent = getStatusText(status, data.error);

  if (status === "done") {
    await loadResultPreview(jobId);
    stopPolling();
  } else if (status === "failed") {
    clearResultPreview(data.error || "任务失败，当前无可下载结果。");
    stopPolling();
  } else {
    clearResultPreview(`任务 ${jobId} 当前状态：${status}`);
  }

  return data;
}

function startPolling(jobId) {
  stopPolling();

  const poll = async () => {
    try {
      const data = await queryJob(jobId);
      if (!POLLABLE_STATUSES.has(data.status)) {
        stopPolling();
      }
    } catch (error) {
      statusMessage.textContent = `查询失败：${error.message}`;
      stopPolling();
    }
  };

  poll();
  jobPollTimer = setInterval(poll, POLL_INTERVAL_MS);
}

async function downloadFile(jobId, kind) {
  if (!jobId) {
    throw new Error("请先输入 jobId");
  }

  const endpoint = kind === "image" ? "image" : "stl";
  let response;
  try {
    response = await fetch(`${API_BASE}/relief/download/${endpoint}/${jobId}`);
  } catch {
    throw new Error("下载接口无法连接");
  }

  if (!response.ok) {
    const data = await response.json().catch(() => null);
    throw new Error(data && data.error ? data.error : "下载失败");
  }

  const blob = await response.blob();
  const objectUrl = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = objectUrl;
  link.download = `${jobId}.${kind === "image" ? "png" : "stl"}`;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(objectUrl);
}

uploadFileInput.addEventListener("change", updateSelectedFile);

uploadForm.addEventListener("submit", async (event) => {
  event.preventDefault();

  const file = uploadFileInput.files && uploadFileInput.files[0] ? uploadFileInput.files[0] : null;
  const error = validateFile(file);
  if (error) {
    showFileError(error);
    return;
  }

  const formData = new FormData();
  formData.append("file", file);

  try {
    const data = await requestJson("/relief", { method: "POST", body: formData });
    const jobId = data.jobId || data.id;
    if (!jobId) {
      throw new Error("响应中缺少 jobId");
    }

    setCurrentJobId(jobId);
    setStatus("queued");
    createJobBtn.classList.add("is-downloaded");
    statusMessage.textContent = `任务创建成功，jobId: ${jobId}`;
    clearResultPreview("正在生成，请稍候。");
    startPolling(jobId);
  } catch (requestError) {
    setStatus("failed");
    statusMessage.textContent = `创建任务失败：${requestError.message}`;
    stopPolling();
  }
});

downloadImageBtn.addEventListener("click", async () => {
  try {
    await downloadFile(currentJobId || jobIdInput.value.trim(), "image");
    downloadImageBtn.classList.add("is-downloaded");
  } catch (downloadError) {
    statusMessage.textContent = `下载 image 失败：${downloadError.message}`;
  }
});

downloadStlBtn.addEventListener("click", async () => {
  try {
    await downloadFile(currentJobId || jobIdInput.value.trim(), "stl");
    downloadStlBtn.classList.add("is-downloaded");
  } catch (downloadError) {
    statusMessage.textContent = `下载 STL 失败：${downloadError.message}`;
  }
});

function init() {
  setCurrentJobId("");
  setStatus("idle");
  createJobBtn.classList.remove("is-downloaded");
  showFileError("");
  setFrameImage(sourceFrame, sourceImage, sourcePlaceholder, "", "选择图片后这里显示原图。");
  clearResultPreview("状态为 done 后，这里显示生成图。");
}

init();
