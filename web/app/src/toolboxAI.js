export const AI_ACTIONS = {
  model: [{ id: "direction", label: "表现要求", field: "notes" }],
  copy: [
    { id: "rewrite", label: "生成 / 改写", field: "text" },
    { id: "shorten", label: "精简文案", field: "text" },
    { id: "variants", label: "生成三版", field: "text" },
  ],
  product: [
    { id: "product_copy", label: "商品文案", field: "text" },
    { id: "claims", label: "整理卖点", field: "claims" },
  ],
};

export function aiCandidatePatch(mode, action, candidate) {
  const option = AI_ACTIONS[mode]?.find((item) => item.id === action);
  if (!option || typeof candidate?.text !== "string" || !candidate.text.trim()) throw new Error("AI 结果无效，请重新生成。");
  return { [option.field]: candidate.text, ...(mode === "product" && action === "product_copy" ? { adapt: true } : {}) };
}
