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

export async function createProduct(formData) {
  return request("/api/products", {
    method: "POST",
    body: formData,
  });
}

export async function getProductMarkdown(id) {
  return request(`/api/products/${id}/markdown`);
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
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(data.error || `Request failed: ${res.status}`);
  }
  return data;
}
