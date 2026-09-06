export const MAX_VERSIONS = 30;
export const MODE_LABELS = { model: "换模特", copy: "换文案", product: "换产品" };
export function createVersion(patch = {}) {
  return { id: crypto.randomUUID(), selected: true, title: "", target: null, notes: "", text: "", scope: "both", name: "", claims: "", adapt: true, ...patch };
}
export function initialVersions() {
  return Object.fromEntries(Object.keys(MODE_LABELS).map((mode) => [mode, Array.from({ length: 3 }, () => createVersion())]));
}
export function versionIssues(mode, version) {
  const issues = [];
  if (mode !== "copy" && !version.target) issues.push(mode === "model" ? "缺模特图" : "缺商品图");
  if (mode === "copy" && !version.text.trim()) issues.push("缺文案");
  if (mode === "product" && version.adapt && !version.text.trim()) issues.push("缺商品文案");
  return issues;
}
function fingerprint(mode, row) {
  const media = row.target?.id || (row.target?.file ? `${row.target.file.name}:${row.target.file.size}:${row.target.file.lastModified}` : "");
  const clean = (value = "") => value.trim().replace(/\s+/g, " ");
  if (mode === "model") return JSON.stringify([media, clean(row.notes)]);
  if (mode === "copy") return JSON.stringify([clean(row.text), row.scope]);
  return JSON.stringify([media, clean(row.name), clean(row.claims), row.adapt, row.adapt ? clean(row.text) : ""]);
}
export function batchSummary(mode, rows) {
  const selected = rows.filter((row) => row.selected);
  const complete = selected.filter((row) => !versionIssues(mode, row).length);
  const seen = new Map();
  const duplicateIds = new Set();
  for (const row of complete) {
    const key = fingerprint(mode, row);
    if (seen.has(key)) { duplicateIds.add(row.id); duplicateIds.add(seen.get(key)); }
    else seen.set(key, row.id);
  }
  return { selected, complete, incomplete: selected.length - complete.length, duplicateIds };
}
export function restoreBatch(draft) {
  if (draft.versionsByMode) return draft.versionsByMode;
  const result = initialVersions();
  if (draft.forms) {
    for (const mode of Object.keys(MODE_LABELS)) result[mode] = [createVersion(draft.forms[mode] || {})];
  }
  return result;
}

export function mergeImportedVersions(rows, imported) {
  const empty = (row) => !row.target && !row.text.trim() && !row.title.trim() && !row.notes.trim() && !row.name.trim() && !row.claims.trim();
  if (rows.filter((row) => !empty(row)).length + imported.length > MAX_VERSIONS) throw new Error(`每批最多 ${MAX_VERSIONS} 个版本，请减少导入数量。`);
  let cursor = 0;
  const next = rows.flatMap((row) => {
    if (!empty(row)) return [row];
    return cursor < imported.length ? [{ ...row, ...imported[cursor++], selected: true }] : [];
  });
  return [...next, ...imported.slice(cursor).map((patch) => createVersion(patch))];
}

export function parseBulkCopies(value) {
  const texts = value.split(/^\s*---\s*$/m).map((text) => text.trim()).filter(Boolean);
  if (!texts.length || texts.some((text) => text.length > 3000)) throw new Error("每版文案需要 1–3000 个字符。");
  return texts.map((text) => ({ text }));
}
