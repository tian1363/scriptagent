const jsonHeaders = {
  "Content-Type": "application/json; charset=utf-8",
};

export async function getAuthStatus() {
  return request("/api/auth/status");
}
export async function getCurrentUser() {
  return request("/api/auth/me");
}
export async function login(input) {
  return request("/api/auth/login", {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify(input),
  });
}
export async function register(input) {
  return request("/api/auth/register", {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify(input),
  });
}
export async function logout() {
  return request("/api/auth/logout", { method: "POST", headers: jsonHeaders });
}

export async function listJobs() {
  return request("/api/jobs");
}

export async function getJob(id) {
  return request(`/api/jobs/${id}`);
}

export async function createJob(formData) {
  return request("/api/jobs", {
    method: "POST",
    body: formData,
  });
}

export async function listProducts() {
  return request("/api/products");
}

export async function listSkills() {
  return request("/api/skills");
}

export async function createSkill(input) {
  return request("/api/skills", {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify(input),
  });
}

export async function generateSkillDraft(requirement) {
  return request("/api/skills/draft", {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify({ requirement }),
  });
}

export async function updateSkill(id, input) {
  return request(`/api/skills/${id}`, {
    method: "PUT",
    headers: jsonHeaders,
    body: JSON.stringify(input),
  });
}

export async function createProduct(formData) {
  return request("/api/products", {
    method: "POST",
    body: formData,
  });
}
export async function parseProductDocument(file) {
  const form = new FormData();
  form.append("document", file);
  return request("/api/products/parse", { method: "POST", body: form });
}

export async function getProductMarkdown(id) {
  return request(`/api/products/${id}/markdown`);
}
export async function listProductAssets(id) {
  return request(`/api/products/${id}/assets`);
}
export async function uploadProductAsset(id, file) {
  const form = new FormData();
  form.append("asset", file);
  return request(`/api/products/${id}/assets`, { method: "POST", body: form });
}
export async function deleteProductAsset(id) {
  return request(`/api/assets/${id}`, { method: "DELETE" });
}
export async function updateProduct(id, input) {
  return request(`/api/products/${id}`, {
    method: "PUT",
    headers: jsonHeaders,
    body: JSON.stringify(input),
  });
}
export async function listSpaces() {
  return request("/api/spaces");
}
export async function listSuggestions() {
  return request("/api/suggestions");
}
export async function updateSuggestionStatus(id, status) {
  return request(`/api/suggestions/${id}/status`, {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify({ status }),
  });
}
export async function createSpace(input) {
  return request("/api/spaces", {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify(input),
  });
}
export async function updateSpace(id, input) {
  return request(`/api/spaces/${id}`, {
    method: "PUT",
    headers: jsonHeaders,
    body: JSON.stringify(input),
  });
}
export async function deleteSpace(id) {
  return request(`/api/spaces/${id}`, { method: "DELETE" });
}
export async function getSpaceObservability(id, limit = 100) {
  return request(`/api/spaces/${id}/observability?limit=${limit}`);
}
export async function getIntelligence(spaceId = "") {
  return request(
    `/api/intelligence${spaceId ? `?space_id=${encodeURIComponent(spaceId)}` : ""}`,
  );
}
export async function seedIntelligenceDemo(spaceId = "") {
  return request("/api/intelligence/demo", {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify({ space_id: spaceId }),
  });
}
export async function promoteIntelligenceSignal(id, spaceId) {
  return request(`/api/intelligence/signals/${id}/memory`, {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify({ space_id: spaceId }),
  });
}
export async function updateCreativeMemory(id, input) {
  return request(`/api/intelligence/memories/${id}`, {
    method: "PUT",
    headers: jsonHeaders,
    body: JSON.stringify(input),
  });
}
export async function deleteCreativeMemory(id) {
  return request(`/api/intelligence/memories/${id}`, { method: "DELETE" });
}
export async function createCompetitorMonitor(input) {
  return request("/api/intelligence/competitors", {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify(input),
  });
}
export async function scanCompetitorMonitor(id) {
  return request(`/api/intelligence/competitors/${id}/scan`, {
    method: "POST",
    headers: jsonHeaders,
  });
}
export async function getOwnerSession() {
  return request("/api/owner/session");
}
export async function getOwnerOverview() {
  return request("/api/owner/overview");
}
export async function loginOwner(username, password) {
  return request("/api/owner/login", {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify({ username, password }),
  });
}
export async function logoutOwner() {
  return request("/api/owner/logout", { method: "POST", headers: jsonHeaders });
}

export async function listCreativeReports(productId) {
  return request(`/api/products/${productId}/creative-reports`);
}

export async function createCreativeReport(productId, input) {
  return request(`/api/products/${productId}/creative-reports`, {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify(input),
  });
}

export async function getModelSettings() {
  return request("/api/settings/model");
}

export async function saveModelSettings(settings) {
  return request("/api/settings/model", {
    method: "PUT",
    headers: jsonHeaders,
    body: JSON.stringify(settings),
  });
}

export async function publishJob(id) {
  return request(`/api/jobs/${id}/publish`, {
    method: "POST",
    headers: jsonHeaders,
  });
}

export async function retryJob(id) {
  return request(`/api/jobs/${id}/retry`, {
    method: "POST",
    headers: jsonHeaders,
  });
}

export async function generateVideoPrompts(id, source = "all") {
  return request(`/api/jobs/${id}/video-prompts`, {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify({ source }),
  });
}

export async function listVideos() {
  return request("/api/videos");
}
export async function createVideo(input) {
  return request("/api/videos", {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify(input),
  });
}
export async function retryVideo(id) {
  return request(`/api/videos/${id}/retry`, {
    method: "POST",
    headers: jsonHeaders,
  });
}

export async function listChats() {
  return request("/api/chats");
}

export async function getChat(id) {
  return request(`/api/chats/${id}`);
}

export async function getChatProgress(id) {
  return request(`/api/chats/${id}/progress`);
}

export async function sendNewChatMessage(
  content,
  productId = "",
  attachment = null,
  spaceId = "",
) {
  if (attachment) {
    const form = new FormData();
    form.append("content", content);
    form.append("product_id", productId);
    form.append("space_id", spaceId);
    form.append("attachment", attachment);
    return request("/api/chats/messages", { method: "POST", body: form });
  }
  return request("/api/chats/messages", {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify({ content, product_id: productId, space_id: spaceId }),
  });
}

export async function sendChatMessage(
  id,
  content,
  productId = "",
  attachment = null,
) {
  if (attachment) {
    const form = new FormData();
    form.append("content", content);
    form.append("product_id", productId);
    form.append("attachment", attachment);
    return request(`/api/chats/${id}/messages`, { method: "POST", body: form });
  }
  return request(`/api/chats/${id}/messages`, {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify({ content, product_id: productId }),
  });
}

export async function listModelCalls(params = {}) {
  const query = new URLSearchParams();
  if (params.ref_id) query.set("ref_id", params.ref_id);
  if (params.limit) query.set("limit", String(params.limit));
  const suffix = query.toString() ? `?${query.toString()}` : "";
  return request(`/api/model-calls${suffix}`);
}

async function request(path, init) {
  const res = await fetch(path, init);
  if (res.status === 204) return null;
  const isJSON = (res.headers.get("content-type") || "").includes(
    "application/json",
  );
  const data = isJSON ? await res.json().catch(() => ({})) : {};
  if (!res.ok) {
    throw new Error(data.error || `Request failed: ${res.status}`);
  }
  if (!isJSON) throw new Error("服务暂时不可用，请稍后再试");
  return data;
}
