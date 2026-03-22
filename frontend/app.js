const DEFAULT_API_BASE = "http://localhost:31101/v1";
const MAX_FILE_SIZE = 10 * 1024 * 1024;

const API_BASE = ((window.APP_CONFIG && window.APP_CONFIG.apiBaseUrl) || DEFAULT_API_BASE).replace(/\/$/, "");

const uploadForm = document.getElementById("uploadForm");
const uploadFileInput = document.getElementById("uploadFile");
const uploadTitle = document.getElementById("uploadTitle");
const fileMeta = document.getElementById("fileMeta");
const fileError = document.getElementById("fileError");
const sourceFrame = document.getElementById("sourceFrame");
const sourceImage = document.getElementById("sourceImage");
const sourcePlaceholder = document.getElementById("sourcePlaceholder");
const jobIdInput = document.getElementById("jobIdInput");
const jobIdValue = document.getElementById("jobIdValue");
const queryJobBtn = document.getElementById("queryJobBtn");
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
  const isDone = safeStatus === "done";
  downloadImageBtn.disabled = !isDone;
  downloadStlBtn.disabled = !isDone;
}

function setCurrentJobId(jobId) {
  currentJobId = jobId || "";
  jobIdInput.value = currentJobId;
  jobIdValue.textContent = currentJobId || "-";
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
    uploadTitle.textContent = "选择图片";
    fileMeta.textContent = "未选择文件";
    showFileError("");
    setFrameImage(sourceFrame, sourceImage, sourcePlaceholder, "", "选择图片后在这里显示。");
    return;
  }

  const error = validateFile(file);
  if (error) {
    uploadFileInput.value = "";
    uploadTitle.textContent = "选择图片";
    fileMeta.textContent = "未选择文件";
    showFileError(error);
    setFrameImage(sourceFrame, sourceImage, sourcePlaceholder, "", "选择图片后在这里显示。");
    return;
  }

  showFileError("");
  uploadTitle.textContent = file.name;
  fileMeta.textContent = formatSize(file.size);
  sourceObjectUrl = URL.createObjectURL(file);
  setFrameImage(sourceFrame, sourceImage, sourcePlaceholder, sourceObjectUrl, "");
}

async function requestJson(path, options = {}) {
  const response = await fetch(`${API_BASE}${path}`, options);
  const text = await response.text();
  let data;

  try {
    data = text ? JSON.parse(text) : {};
  } catch {
    data = text;
  }

  if (!response.ok) {
    const message = typeof data === "object" && data && data.error ? data.error : `${response.status} ${response.statusText}`;
    throw new Error(message);
  }

  return data;
}

async function loadResultPreview(jobId) {
  const response = await fetch(`${API_BASE}/relief/download/image/${jobId}`);
  if (!response.ok) {
    throw new Error("image 读取失败");
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
    return "任务处理中，请继续使用 jobId 查询。";
  }
  if (status === "done") {
    return "任务已完成，可以下载 image 和 STL。";
  }
  if (status === "failed") {
    return error ? `任务失败：${error}` : "任务失败。";
  }
  return "创建任务后会返回 jobId，可继续查询任务状态。";
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
  } else if (status === "failed") {
    clearResultPreview(data.error || "任务失败，当前无可下载结果。");
  } else {
    clearResultPreview(`任务 ${jobId} 当前状态：${status}`);
  }
}

async function downloadFile(jobId, kind) {
  if (!jobId) {
    throw new Error("请先输入 jobId");
  }

  const endpoint = kind === "image" ? "image" : "stl";
  const response = await fetch(`${API_BASE}/relief/download/${endpoint}/${jobId}`);
  if (!response.ok) {
    const data = await response.json().catch(() => null);
    throw new Error(data && data.error ? data.error : "下载失败");
  }

  const blob = await response.blob();
  const objectUrl = URL.createObjectURL(blob);
  const link = document.createElement("a");
  const ext = kind === "image" ? "png" : "stl";
  link.href = objectUrl;
  link.download = `${jobId}.${ext}`;
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
    const data = await requestJson("/relief", {
      method: "POST",
      body: formData,
    });
    const jobId = data.jobId || data.id;
    if (!jobId) {
      throw new Error("响应中缺少 jobId");
    }

    setCurrentJobId(jobId);
    setStatus("queued");
    statusMessage.textContent = `任务创建成功，jobId: ${jobId}`;
    clearResultPreview("状态为 done 后，这里显示 depth image，并开放下载。");
  } catch (errorMessage) {
    statusMessage.textContent = `创建任务失败：${errorMessage.message}`;
    setStatus("failed");
  }
});

queryJobBtn.addEventListener("click", async () => {
  try {
    await queryJob(jobIdInput.value.trim());
  } catch (error) {
    statusMessage.textContent = error.message;
    setStatus("idle");
  }
});

downloadImageBtn.addEventListener("click", async () => {
  try {
    await downloadFile(currentJobId || jobIdInput.value.trim(), "image");
  } catch (error) {
    statusMessage.textContent = `下载 image 失败：${error.message}`;
  }
});

downloadStlBtn.addEventListener("click", async () => {
  try {
    await downloadFile(currentJobId || jobIdInput.value.trim(), "stl");
  } catch (error) {
    statusMessage.textContent = `下载 STL 失败：${error.message}`;
  }
});

function init() {
  setCurrentJobId("");
  setStatus("idle");
  showFileError("");
  setFrameImage(sourceFrame, sourceImage, sourcePlaceholder, "", "选择图片后在这里显示。");
  clearResultPreview("状态为 done 后，这里显示 depth image，并开放下载。");
}

init();
