import { useEffect, useRef, useState } from "react";
import { Loader2, Sparkles, X } from "lucide-react";
import { generateToolboxDraft } from "./api.js";
import { AI_ACTIONS } from "./toolboxAI.js";

export default function ToolboxAI({ mode, row, remaining, onApply, onClose }) {
  const dialog = useRef(null);
  const controller = useRef(null);
  const [action, setAction] = useState(AI_ACTIONS[mode][0].id);
  const [instruction, setInstruction] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [result, setResult] = useState(null);
  useEffect(() => {
    dialog.current.showModal();
    return () => controller.current?.abort();
  }, []);
  const hasContext = Boolean([row.text, row.name, row.claims, row.notes, instruction].some((value) => value?.trim()));
  async function generate() {
    if (controller.current) return;
    const request = new AbortController();
    controller.current = request;
    setBusy(true); setError(""); setResult(null);
    try {
      const data = await generateToolboxDraft({ mode, action, text: row.text, name: row.name, claims: row.claims, notes: row.notes, instruction }, request.signal);
      if (!Array.isArray(data?.candidates) || !data.candidates.length) throw new Error("AI 未返回可用内容，请重试。");
      if (!request.signal.aborted) setResult({ action, candidates: data.candidates });
    } catch (e) {
      if (!request.signal.aborted) setError(e.message || "生成失败，请稍后重试。");
    } finally {
      if (controller.current === request) controller.current = null;
      if (!request.signal.aborted) setBusy(false);
    }
  }
  function cancel() {
    controller.current?.abort(); controller.current = null; setBusy(false);
  }
  return <dialog ref={dialog} className="tb-dialog tb-ai-dialog" aria-labelledby="tb-ai-title" onCancel={onClose} onClick={(e) => { if (e.target === dialog.current) onClose(); }}>
    <header><h2 id="tb-ai-title"><Sparkles size={20} />AI 创作助手</h2><button className="tb-icon-button" aria-label="关闭 AI 助手" onClick={onClose}><X size={20} /></button></header>
    {AI_ACTIONS[mode].length > 1 && <div className="tb-ai-actions" aria-label="选择 AI 操作">{AI_ACTIONS[mode].map((item) => <button key={item.id} disabled={busy} className={item.id === action ? "active" : ""} aria-pressed={item.id === action} onClick={() => { setAction(item.id); setResult(null); setError(""); }}>{item.label}</button>)}</div>}
    <p className="tb-muted">基于当前版本的文字生成，暂不读取图片和视频。结果由你确认后采用。</p>
    <label className="tb-field">{hasContext ? "补充要求" : "告诉 AI 你的想法"}<textarea autoFocus rows={3} maxLength={3000} disabled={busy} value={instruction} onChange={(e) => { setInstruction(e.target.value); setResult(null); }} placeholder={mode === "model" ? "例如：居家场景，亲切自然地介绍产品，希望动作更有生活感" : mode === "copy" ? "粘贴原文，或说明已确认的卖点、受众和语气…" : "提供已确认的商品信息、目标人群和表达要求…"} /></label>
    {action === "shorten" && !row.text.trim() && <p className="tb-muted">请先返回操作台填写需要精简的文案。</p>}
    {error && <p className="tb-inline-note" role="alert">{error}</p>}
    <div className="tb-plan-actions"><button className="tb-primary" disabled={busy || !hasContext || (action === "shorten" && !row.text.trim())} onClick={generate}>{busy ? <Loader2 className="spin" size={16} /> : <Sparkles size={16} />}{busy ? "正在生成…" : result ? "重新生成" : "生成建议"}</button>{busy && <button className="tb-secondary" onClick={cancel}>取消生成</button>}</div>
    {busy && <p className="tb-helper" role="status">正在组织表达，原内容会保留。</p>}
    {result && <section className="tb-ai-results" aria-label="AI 生成建议" aria-live="polite">{result.candidates.map((candidate, index) => <article key={index}><h3>{candidate.title}</h3><p>{candidate.text}</p><button className="tb-secondary" onClick={() => onApply(result.action, [candidate], false)}>采用这一版</button></article>)}{result.candidates.length > 1 && <><button className="tb-primary" disabled={remaining < result.candidates.length} onClick={() => onApply(result.action, result.candidates, true)}>全部添加为新版本</button>{remaining < result.candidates.length && <p className="tb-muted">本批次最多 30 个版本，请先移除部分版本。</p>}</>}</section>}
  </dialog>;
}
