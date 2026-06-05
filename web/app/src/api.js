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

async function request(path, init) {
  const res = await fetch(path, init);
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(data.error || `Request failed: ${res.status}`);
  }
  return data;
}
