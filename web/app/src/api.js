const jsonHeaders = {
  "Content-Type": "application/json; charset=utf-8",
};

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

export async function createProduct(formData) {
  return request("/api/products", {
    method: "POST",
    body: formData,
  });
}

export async function getProductMarkdown(id) {
  return request(`/api/products/${id}/markdown`);
}
export async function listProductAssets(id) { return request(`/api/products/${id}/assets`); }
export async function uploadProductAsset(id, file) { const form = new FormData(); form.append("asset", file); return request(`/api/products/${id}/assets`, { method:"POST", body:form }); }
export async function updateProduct(id, input) { return request(`/api/products/${id}`, { method:"PUT", headers:jsonHeaders, body:JSON.stringify(input) }); }
export async function listSpaces() { return request("/api/spaces"); }
export async function createSpace(input) { return request("/api/spaces", { method:"POST", headers:jsonHeaders, body:JSON.stringify(input) }); }

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

export async function listChats() {
  return request("/api/chats");
}

export async function getChat(id) {
  return request(`/api/chats/${id}`);
}

export async function sendNewChatMessage(content, productId = "") {
  return request("/api/chats/messages", {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify({ content, product_id: productId }),
  });
}

export async function sendChatMessage(id, content, productId = "") {
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
  const isJSON = (res.headers.get("content-type") || "").includes("application/json");
  const data = isJSON ? await res.json().catch(() => ({})) : {};
  if (!res.ok) {
    throw new Error(data.error || `Request failed: ${res.status}`);
  }
  if (!isJSON) throw new Error("服务暂时不可用，请稍后再试");
  return data;
}
