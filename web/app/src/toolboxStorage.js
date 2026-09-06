const DB_NAME = "scriptagent-toolbox";

// Files stay in this browser until the generation service supports toolbox jobs.
export async function toolboxDrafts(userId, operation, draft) {
  const db = await new Promise((resolve, reject) => {
    const request = indexedDB.open(DB_NAME, 1);
    request.onupgradeneeded = () => request.result.createObjectStore("drafts", { keyPath: "key" });
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error);
  });
  try {
    return await new Promise((resolve, reject) => {
      const transaction = db.transaction("drafts", operation === "list" ? "readonly" : "readwrite");
      const store = transaction.objectStore("drafts");
      const request = operation === "list" ? store.getAll()
        : operation === "delete" ? store.delete(`${userId}:${draft.id}`)
        : store.put({ ...draft, userId, key: `${userId}:${draft.id}` });
      transaction.oncomplete = () => resolve(operation === "list"
        ? request.result.filter((item) => item.userId === userId).sort((a, b) => b.updatedAt - a.updatedAt)
        : draft);
      transaction.onerror = () => reject(transaction.error);
      transaction.onabort = () => reject(transaction.error || new Error("保存被中断"));
    });
  } finally { db.close(); }
}
