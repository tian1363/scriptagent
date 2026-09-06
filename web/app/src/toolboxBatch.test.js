import test from "node:test";
import assert from "node:assert/strict";
import { createVersion, initialVersions, batchSummary, versionIssues, mergeImportedVersions, parseBulkCopies, restoreBatch } from "./toolboxBatch.js";

test("batch totals ignore unselected incomplete and duplicated rows", () => {
  const one = createVersion({ text: "新品\n即刻体验" });
  const duplicate = createVersion({ text: "新品 即刻体验" });
  const missing = createVersion({ selected: false });
  const result = batchSummary("copy", [one, duplicate, missing]);
  assert.equal(result.selected.length, 2);
  assert.equal(result.incomplete, 0);
  assert.equal(result.duplicateIds.size, 2);
  duplicate.selected = false;
  assert.equal(batchSummary("copy", [one, duplicate, missing]).duplicateIds.size, 0);
});

test("same reference with different instructions counts as differentiated", () => {
  const target = { id: "image-1" };
  const rows = [createVersion({ target, notes: "自然微笑" }), createVersion({ target, notes: "认真介绍" })];
  assert.equal(batchSummary("model", rows).duplicateIds.size, 0);
  assert.equal(versionIssues("product", createVersion({ target })).length, 1);
  assert.equal(versionIssues("product", createVersion({ target, adapt: false })).length, 0);
});

test("bulk imports replace empty placeholders and preserve existing edited versions", () => {
  const edited = createVersion({ text: "已编辑内容", selected: false });
  const rows = [edited, createVersion(), createVersion()];
  const next = mergeImportedVersions(rows, [{ text: "新文案" }]);
  assert.equal(next.length, 2);
  assert.equal(next[0], edited);
  assert.equal(next[1].text, "新文案");
  assert.equal(rows.length, 3);
  assert.throws(() => mergeImportedVersions([edited], Array.from({ length: 30 }, () => ({ text: "test" }))), /最多/);
});

test("bulk copy retains paragraph breaks and rejects oversized entries", () => {
  assert.deepEqual(parseBulkCopies("第一段\n第二段\n---\n另一版"), [{ text: "第一段\n第二段" }, { text: "另一版" }]);
  assert.throws(() => parseBulkCopies("a".repeat(3001)));
  assert.throws(() => parseBulkCopies("---"));
});

test("legacy saved plans remain restorable, and new batches preserve all versions", () => {
  const legacy = restoreBatch({ forms: { copy: { text: "旧方案" } } });
  assert.equal(legacy.copy[0].text, "旧方案");
  const versionsByMode = initialVersions();
  assert.equal(restoreBatch({ versionsByMode }), versionsByMode);
  assert.equal(new Set(Object.values(versionsByMode).flat().map((row) => row.id)).size, 9);
});
