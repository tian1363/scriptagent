import { useEffect, useRef, useState } from "react";
import { Check, Copy, Download, FileText, Film, FolderOpen, History, ImagePlus, Layers, Loader2, Package, Plus, Save, ScanFace, Trash2, Upload, X } from "lucide-react";
import { listProductAssets } from "./api.js";
import { toolboxDrafts } from "./toolboxStorage.js";
import "./EcommerceToolbox.css";
import { MAX_VERSIONS, MODE_LABELS, createVersion, initialVersions, versionIssues, batchSummary, restoreBatch, mergeImportedVersions, parseBulkCopies } from "./toolboxBatch.js";

const tools = [
  { id: "model", title: "换模特", Icon: ScanFace, description: "换一个出镜人物，延续原来的创意。", target: "目标模特", hint: "上传清晰的单人参考图" },
  { id: "copy", title: "换文案", Icon: FileText, description: "换一种表达，让卖点更贴近受众。" },
  { id: "product", title: "换产品", Icon: Package, description: "沿用视频创意，展示你的商品。", target: "目标产品", hint: "上传能看清外观和包装的商品图" },
];
const assetURL = (asset) => asset?.id ? `/api/assets/${asset.id}/file` : "";
const mediaName = (media) => media?.file?.name || media?.original_name || media?.name || "素材";

function useMediaURL(media) {
  const [url, setURL] = useState("");
  useEffect(() => {
    if (!media?.file) { setURL(assetURL(media)); return; }
    const next = URL.createObjectURL(media.file);
    setURL(next);
    return () => URL.revokeObjectURL(next);
  }, [media]);
  return url;
}

function MediaUpload({ label, kind, value, onChange, onLibrary, onError }) {
  const input = useRef(null);
  const [dragging, setDragging] = useState(false);
  const preview = useMediaURL(value);
  function receive(file) {
    if (!file) return;
    const valid = kind === "video" ? /\.(mp4|mov)$/i.test(file.name) : /\.(png|jpe?g|webp)$/i.test(file.name);
    const limit = kind === "video" ? 100 : 20;
    if (!valid) { onError(kind === "video" ? "请选择 MP4 或 MOV 视频。" : "请选择 JPG、PNG 或 WebP 图片。"); return; }
    if (file.size > limit * 1024 * 1024 || file.size === 0) { onError(`请选择非空且小于 ${limit}MB 的文件。`); return; }
    onError(""); onChange({ file, kind });
  }
  return <div className="tb-upload-wrap">
    <input ref={input} type="file" hidden accept={kind === "video" ? ".mp4,.mov" : ".jpg,.jpeg,.png,.webp"} aria-label={label} onChange={(e) => { receive(e.target.files?.[0]); e.target.value = ""; }} />
    {value ? <div className="tb-selected-media">
      {kind === "image" ? <img src={preview} alt={label} /> : <span className="tb-file-icon"><Film size={22} /></span>}
      <div><strong>{mediaName(value)}</strong><small>{value.file ? "本地素材" : "产品资料"}</small></div>
      <button className="tb-icon-button" type="button" aria-label={`移除${label}`} onClick={() => onChange(null)}><X size={16} /></button>
    </div> : <button type="button" className={`tb-dropzone ${dragging ? "dragging" : ""}`} onClick={() => input.current?.click()} onDragOver={(e) => { e.preventDefault(); setDragging(true); }} onDragLeave={() => setDragging(false)} onDrop={(e) => { e.preventDefault(); setDragging(false); receive(e.dataTransfer.files[0]); }}>
      <span className="tb-upload-icon">{kind === "video" ? <Upload size={21} /> : <ImagePlus size={21} />}</span>
      <strong>{kind === "video" ? "点击上传或拖入原视频" : `上传${label}参考图`}</strong>
      <small>{kind === "video" ? "MP4 / MOV · 最大 100MB" : "JPG / PNG / WebP · 最大 20MB"}</small>
    </button>}
    <div className="tb-upload-actions">{value && <button type="button" onClick={() => input.current?.click()}>重新上传</button>}<button type="button" onClick={onLibrary}><FolderOpen size={14} />从产品资料选择</button></div>
  </div>;
}

function AssetPicker({ products, kind, onPick, onClose }) {
  const dialog = useRef(null);
  const [productId, setProductId] = useState(products[0]?.id || "");
  const [assets, setAssets] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  useEffect(() => { dialog.current.showModal(); }, []);
  useEffect(() => {
    let active = true;
    setAssets([]); setError("");
    if (!productId) return;
    setLoading(true);
    listProductAssets(productId).then((items) => { if (active) setAssets(items.filter((item) => item.kind === kind)); })
      .catch((e) => { if (active) setError(e.message); }).finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [productId, kind]);
  return <dialog ref={dialog} className="tb-dialog" onCancel={onClose} onClick={(e) => { if (e.target === dialog.current) onClose(); }} aria-labelledby="tb-library-title">
    <header><div><small>素材库</small><h2 id="tb-library-title">从产品资料选择{kind === "video" ? "视频" : "图片"}</h2></div><button className="tb-icon-button" onClick={onClose} aria-label="关闭素材库"><X size={20} /></button></header>
    {products.length ? <><label className="tb-field">产品<select value={productId} onChange={(e) => setProductId(e.target.value)}>{products.map((p) => <option key={p.id} value={p.id}>{p.title || p.name}</option>)}</select></label>
      {loading ? <p role="status">正在读取素材…</p> : error ? <p role="alert">{error}</p> : assets.length ? <div className="tb-asset-grid">{assets.map((asset) => <button key={asset.id} onClick={() => onPick(asset)}>{kind === "image" ? <img src={assetURL(asset)} alt="" /> : <video src={assetURL(asset)} preload="metadata" muted />}<span>{mediaName(asset)}</span></button>)}</div> : <p className="tb-muted">这个产品还没有可选的{kind === "video" ? "视频" : "图片"}。你也可以关闭窗口，直接上传。</p>}</> : <p className="tb-muted">还没有产品资料。关闭窗口后可直接上传，无需先创建产品。</p>}
  </dialog>;
}

function VersionThumbnail({ row, mode }) {
  const url = useMediaURL(row.target);
  return url && mode !== "copy" ? <img className="tb-row-thumbnail" src={url} alt="目标参考图" /> : <span className="tb-row-thumbnail">{mode === "copy" ? <FileText size={18} /> : <ImagePlus size={18} />}</span>;
}

function BatchDialog({ title, children, onClose }) {
  const ref = useRef(null);
  useEffect(() => { ref.current.showModal(); }, []);
  return <dialog ref={ref} className="tb-dialog" onCancel={onClose} aria-label={title} onClick={(e) => { if (e.target === ref.current) onClose(); }}>
    <header><h2>{title}</h2><button className="tb-icon-button" onClick={onClose} aria-label="关闭窗口"><X size={20} /></button></header>{children}
  </dialog>;
}

export default function EcommerceToolbox({ products = [], userId, active = true }) {
  const [mode, setMode] = useState(() => {
    try { const saved = localStorage.getItem(`scriptagent:toolbox-mode:${userId}`); return MODE_LABELS[saved] ? saved : "model"; } catch { return "model"; }
  });
  const [source, setSource] = useState(null);
  const [versionsByMode, setVersions] = useState(initialVersions);
  const [activeId, setActiveId] = useState(null);
  const [batchName, setBatchName] = useState("");
  const [resolution, setResolution] = useState("720P");
  const [duration, setDuration] = useState(0);
  const [mediaError, setMediaError] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [drafts, setDrafts] = useState([]);
  const [draftId, setDraftId] = useState(null);
  const [saving, setSaving] = useState(false);
  const [dialog, setDialog] = useState(null);
  const [picker, setPicker] = useState(null);
  const [bulkCopy, setBulkCopy] = useState("");
  const videoRef = useRef(null);
  const importRef = useRef(null);
  const sourceURL = useMediaURL(source);
  const rows = versionsByMode[mode];
  const current = rows.find((row) => row.id === activeId) || rows[0];
  const currentIndex = rows.findIndex((row) => row.id === current?.id);
  const summary = batchSummary(mode, rows);
  const tool = tools.find((item) => item.id === mode);
  const targetURL = useMediaURL(current?.target);
  const canReview = Boolean(source && !mediaError && summary.selected.length > 0);
  const ready = canReview && !summary.incomplete && !summary.duplicateIds.size;
  const selectedAll = rows.length > 0 && rows.every((row) => row.selected);
  const estimates = current?.text ? Math.ceil(current.text.replace(/\s/g, "").length / 4) : 0;

  useEffect(() => { if (!active) videoRef.current?.pause(); }, [active]);
  useEffect(() => { try { localStorage.setItem(`scriptagent:toolbox-mode:${userId}`, mode); } catch {} }, [mode, userId]);
  useEffect(() => {
    let mounted = true;
    toolboxDrafts(userId, "list").then((items) => { if (mounted) setDrafts(items); }).catch(() => { if (mounted) setError("无法读取本地批次，请检查浏览器存储权限。"); });
    return () => { mounted = false; };
  }, [userId]);

  function updateRows(fn) { setVersions((all) => ({ ...all, [mode]: fn(all[mode]) })); setNotice(""); }
  function updateRow(id, patch) { updateRows((items) => items.map((row) => row.id === id ? { ...row, ...patch } : row)); }
  function changeSource(value) { setSource(value); setDuration(0); setMediaError(""); setNotice(""); }
  function addVersion(base = {}) {
    if (rows.length >= MAX_VERSIONS) return;
    const row = createVersion(base); updateRows((items) => [...items, row]); setActiveId(row.id);
  }
  function addImported(imported) {
    let next;
    try { next = mergeImportedVersions(rows, imported); }
    catch (e) { setError(e.message); return false; }
    updateRows(() => next);
    setActiveId(next[0]?.id);
    setNotice(`已导入 ${imported.length} 个差异化版本，请检查并勾选本次需要的版本。`);
    return true;
  }
  function importImages(files) {
    const items = Array.from(files || []);
    if (!items.length) return;
    if (items.some((file) => !/\.(jpe?g|png|webp)$/i.test(file.name) || file.size === 0 || file.size > 20 * 1024 * 1024)) { setError("请上传 JPG、PNG 或 WebP 图片，每张非空且不超过 20MB。此次未导入任何图片。"); return; }
    addImported(items.map((file) => ({ target: { file, kind: "image" }, title: file.name.replace(/\.[^.]+$/, "") })));
  }
  async function save() {
    if (saving) return;
    setSaving(true); setError("");
    const draft = { id: draftId || crypto.randomUUID(), schemaVersion: 2, mode, source, batchName, versionsByMode, resolution, updatedAt: Date.now() };
    try { await toolboxDrafts(userId, "save", draft); setDraftId(draft.id); setDrafts(await toolboxDrafts(userId, "list")); setNotice("整批配置与素材已保存到当前浏览器。"); return true; }
    catch { setError("保存失败，请检查浏览器存储空间。当前输入仍保留。"); return false; }
    finally { setSaving(false); }
  }
  function restore(draft) {
    setMode(draft.mode); setVersions(restoreBatch(draft)); changeSource(draft.source); setBatchName(draft.batchName || "");
    setResolution(draft.resolution || "720P"); setDraftId(draft.id); setActiveId(null); setDialog(null); setError("");
  }
  function download() {
    const text = [`# ${batchName || `${tool.title}批量素材`}`, `原视频：${mediaName(source)}\n\n规格：${resolution}\n\n选中版本：${summary.selected.length}`, ...summary.selected.map((row, i) => `## ${i + 1}. ${row.title || `版本 ${i + 1}`}\n\n${mode === "copy" ? `修改范围：${{ both: "口播与字幕", voice: "口播", subtitle: "字幕" }[row.scope]}\n\n${row.text}` : `参考图：${row.target ? mediaName(row.target) : "未选择"}\n\n${mode === "model" ? row.notes : `产品：${row.name}\n卖点：${row.claims}\n文案：${row.adapt ? row.text : "沿用原文案，需核对产品信息"}`}`}\n\n检查：${versionIssues(mode, row).join("、") || "输入已齐全"}${summary.duplicateIds.has(row.id) ? "；与其他选中版本重复" : ""}`), "状态：配置草稿，尚未提交生成。\n参考原片生成新版本，其他画面不保证逐帧一致。"].join("\n\n");
    const url = URL.createObjectURL(new Blob([text], { type: "text/markdown;charset=utf-8" }));
    const a = document.createElement("a"); a.href = url; a.download = "批量素材方案.md"; a.click(); setTimeout(() => URL.revokeObjectURL(url), 1000);
  }

  return <div className="ecommerce-toolbox tb-batch-workspace">
    <header className="tb-page-header"><div><div className="tb-eyebrow">素材创作 / 批量改编</div><h1>电商人工具箱</h1><p>一条原素材，一批差异化版本。</p></div><div className="tb-header-actions"><button className="tb-secondary" onClick={() => setDialog("drafts")}><History size={16} />已存批次{drafts.length ? ` (${drafts.length})` : ""}</button><button className="tb-secondary" disabled={saving} onClick={save}>{saving ? <Loader2 size={16} className="spin" /> : <Save size={16} />}保存草稿</button></div></header>
    <div className="tb-tabs" role="tablist" aria-label="选择批量工具">{tools.map(({ id, title, Icon }, index) => <button key={id} role="tab" id={`tb-tab-${id}`} aria-controls="tb-batch-panel" aria-selected={mode === id} tabIndex={mode === id ? 0 : -1} onClick={() => { setMode(id); setActiveId(null); setError(""); }} onKeyDown={(e) => { const next = e.key === "ArrowRight" ? (index + 1) % 3 : e.key === "ArrowLeft" ? (index + 2) % 3 : e.key === "Home" ? 0 : e.key === "End" ? 2 : null; if (next !== null) { e.preventDefault(); setMode(tools[next].id); setActiveId(null); document.getElementById(`tb-tab-${tools[next].id}`)?.focus(); } }}><Icon size={18} />{title}<small>批量</small></button>)}</div>
    <div className="tb-batch-grid" id="tb-batch-panel" role="tabpanel" aria-labelledby={`tb-tab-${mode}`}>
      <aside className="tb-source-column" aria-label="共用原素材">
        <div className="tb-column-title"><h2>原素材</h2><span>所有版本共用</span></div>
        {sourceURL && <div className="tb-source-video"><video ref={videoRef} key={sourceURL} src={sourceURL} controls preload="metadata" onLoadedMetadata={(e) => { setDuration(Number.isFinite(e.currentTarget.duration) ? e.currentTarget.duration : 0); setMediaError(""); }} onError={() => setMediaError("无法预览，请换用可播放的 MP4 视频。")} /></div>}
        <MediaUpload label="原视频" kind="video" value={source} onChange={changeSource} onLibrary={() => setPicker("source")} onError={setError} />
        {mediaError && <p className="tb-inline-note" role="alert">{mediaError}</p>}
        {duration > 0 && <p className="tb-helper">原片时长 {Math.round(duration)} 秒</p>}
        <div className="tb-shared-settings"><h3>统一设置</h3><label className="tb-field">批次名称<input value={batchName} onChange={(e) => setBatchName(e.target.value)} placeholder="例如：秋季上新 · 模特测试" maxLength={80} /></label><label className="tb-field">输出分辨率<select value={resolution} onChange={(e) => setResolution(e.target.value)}><option>480P</option><option>720P</option><option>1080P</option></select></label></div>
        <div className="tb-preserve"><h3>本批次保留要求</h3><div>{(mode === "model" ? ["产品", "文案", "场景"] : mode === "copy" ? ["人物", "产品", "场景"] : ["人物", "视频创意"]).map((item) => <span key={item}><Check size={13} />{item}</span>)}</div></div><p className="tb-helper">参考原片生成新版本，画面细节可能变化。</p>
      </aside>
      <section className="tb-version-column" aria-label="差异化版本清单">
        <div className="tb-column-title"><div><h2>差异化版本 <span className="tb-count">{rows.length}</span></h2><p>每行一个版本，勾选后一批处理。</p></div><button className="tb-icon-button" disabled={rows.length >= MAX_VERSIONS} onClick={() => addVersion()} aria-label="添加版本"><Plus size={19} /></button></div>
        <div className="tb-list-toolbar"><label className="tb-check"><input type="checkbox" checked={selectedAll} onChange={(e) => updateRows((items) => items.map((row) => ({ ...row, selected: e.target.checked })))} />全选 <span>{summary.selected.length}/{rows.length}</span></label><button className="tb-text-button" onClick={() => mode === "copy" ? (setError(""), setDialog("import")) : importRef.current?.click()}><Upload size={14} />{mode === "copy" ? "批量粘贴文案" : "批量导入图片"}</button><input ref={importRef} type="file" hidden multiple accept=".jpg,.jpeg,.png,.webp" aria-label="批量导入参考图" onChange={(e) => { importImages(e.target.files); e.target.value = ""; }} /></div>
        <div className="tb-version-list">{rows.map((row, index) => {
          const issues = versionIssues(mode, row);
          const duplicate = summary.duplicateIds.has(row.id);
          return <article className={`tb-version-row ${current?.id === row.id ? "active" : ""}`} key={row.id}>
            <input type="checkbox" checked={row.selected} aria-label={`选择版本 ${index + 1}`} onChange={(e) => updateRow(row.id, { selected: e.target.checked })} />
            <button className="tb-version-select" aria-label={`编辑版本 ${index + 1}`} aria-pressed={current?.id === row.id} onClick={() => setActiveId(row.id)}><VersionThumbnail row={row} mode={mode} /><span className="tb-row-copy"><strong><span className="tb-version-number">{String(index + 1).padStart(2, "0")}</span>{row.title || `版本 ${index + 1}`}</strong><span>{mode === "copy" ? row.text || "添加一版独立文案" : row.target ? mediaName(row.target) : mode === "model" ? "添加不同的模特参考图" : "添加目标商品参考图"}</span><small className={duplicate ? "tb-duplicate" : ""}>{duplicate ? "内容重复，请调整差异" : issues.length ? issues.join(" · ") : "已配置"}</small></span></button>
            <div className="tb-row-actions"><button className="tb-icon-button" aria-label={`复制版本 ${index + 1}`} disabled={rows.length >= MAX_VERSIONS} onClick={() => addVersion({ ...row, id: crypto.randomUUID(), title: `${row.title || `版本 ${index + 1}`} 副本`, selected: true })}><Copy size={14} /></button><button className="tb-icon-button" aria-label={`移除版本 ${index + 1}`} onClick={() => updateRows((items) => items.filter((item) => item.id !== row.id))}><Trash2 size={14} /></button></div>
          </article>;
        })}</div>
        <button className="tb-add-version" disabled={rows.length >= MAX_VERSIONS} onClick={() => addVersion()}><Plus size={17} />添加一个差异化版本<span>{rows.length}/{MAX_VERSIONS}</span></button>
        <div className="tb-batch-hint"><Layers size={16} /><p>{mode === "model" ? "导入多张模特图，每张图对应一个版本。" : mode === "copy" ? "不同钩子、卖点或 CTA，各写成一个版本。" : "导入不同商品或包装图，逐版确认对应文案。"}</p></div>
        {summary.duplicateIds.size > 0 && <p className="tb-inline-note" role="status">{summary.duplicateIds.size} 个选中版本内容重复。修改差异或取消勾选，避免重复生成。</p>}
      </section>
      <aside className="tb-editor-column" aria-label="当前版本设置">
        {current ? <><div className="tb-column-title"><h2>版本 {String(currentIndex + 1).padStart(2, "0")}</h2><span>仅修改此版本</span></div><label className="tb-field">版本名称<input value={current.title} onChange={(e) => updateRow(current.id, { title: e.target.value })} placeholder={`版本 ${currentIndex + 1}`} maxLength={80} /></label>
          {mode === "copy" ? <><label className="tb-field">修改范围<select value={current.scope} onChange={(e) => updateRow(current.id, { scope: e.target.value })}><option value="both">口播与字幕</option><option value="voice">仅口播</option><option value="subtitle">仅字幕</option></select></label><label className="tb-field">新文案<textarea rows={9} value={current.text} onChange={(e) => updateRow(current.id, { text: e.target.value })} maxLength={3000} placeholder="写下这一版的口播或字幕…" /></label><div className="tb-input-meta"><span>{current.text.length}/3000</span>{current.scope !== "subtitle" && <span>口播约 {estimates} 秒</span>}</div>{duration > 0 && estimates > duration && current.scope !== "subtitle" && <p className="tb-inline-note">预计口播超过原片时长，建议精简。</p>}<p className="tb-helper">支持手动编辑；自动提取与改写待接入。</p></> : <><h3 className="tb-editor-label">{tool.target}</h3>{targetURL && <img className="tb-editor-image" src={targetURL} alt={tool.target} />}<MediaUpload label={tool.target} kind="image" value={current.target} onChange={(target) => updateRow(current.id, { target })} onLibrary={() => setPicker(current.id)} onError={setError} />
            {mode === "model" ? <label className="tb-field">此版本的差异要求<textarea rows={4} value={current.notes} onChange={(e) => updateRow(current.id, { notes: e.target.value })} placeholder="例如：自然分享语气、微笑、居家穿搭" maxLength={2000} /></label> : <><label className="tb-field">商品名称<input value={current.name} onChange={(e) => updateRow(current.id, { name: e.target.value })} placeholder="目标商品名称" maxLength={100} /></label><label className="tb-field">已确认的卖点<textarea rows={2} value={current.claims} onChange={(e) => updateRow(current.id, { claims: e.target.value })} placeholder="材质、规格、功能等" maxLength={2000} /></label><label className="tb-check"><input type="checkbox" checked={current.adapt} onChange={(e) => updateRow(current.id, { adapt: e.target.checked })} />同时替换商品文案</label>{current.adapt ? <label className="tb-field">商品文案<textarea rows={4} value={current.text} onChange={(e) => updateRow(current.id, { text: e.target.value })} placeholder="确认这版商品对应的口播／字幕" maxLength={3000} /></label> : <p className="tb-inline-note">沿用原文案前，请核对规格和功效。</p>}</>}
          </>}<p className="tb-editor-footnote"><Check size={14} />修改自动保留在当前批次，点击保存草稿可稍后恢复。</p></> : <div className="tb-empty-editor"><Layers size={28} /><h2>添加第一个版本</h2><p>也可以一次导入多个参考图或多版文案。</p><button className="tb-secondary" onClick={() => addVersion()}><Plus size={16} />添加版本</button></div>}
      </aside>
    </div>
    <footer className="tb-batch-footer"><div className="tb-batch-total"><Layers size={20} /><div><strong>本批次 <b>{summary.selected.length}</b> 个版本</strong><small>{!source ? "先添加共用原视频" : `${summary.complete.length} 个已配置 · ${summary.incomplete} 个待完善`}{summary.duplicateIds.size ? ` · ${summary.duplicateIds.size} 个重复` : ""}</small></div></div><div className="tb-footer-actions"><button className="tb-secondary" disabled={!summary.selected.length} onClick={download}><Download size={15} />导出清单</button><button className="tb-primary" disabled={!canReview} onClick={() => setDialog("review")}>检查整批配置 <span>{summary.selected.length}</span></button></div><p>批量生成服务待接入，当前可配置、检查和保存整批素材。</p></footer>
    {error && <div className="tb-feedback tb-error" role="alert"><span>{error}</span><button className="tb-icon-button" aria-label="关闭提示" onClick={() => setError("")}><X size={16} /></button></div>}
    {notice && <div className="tb-feedback" role="status"><Check size={16} /><span>{notice}</span><button className="tb-icon-button" aria-label="关闭保存提示" onClick={() => setNotice("")}><X size={16} /></button></div>}
    {picker && <AssetPicker products={products} kind={picker === "source" ? "video" : "image"} onClose={() => setPicker(null)} onPick={(asset) => { if (picker === "source") changeSource(asset); else updateRow(picker, { target: asset }); setPicker(null); }} />}
    {dialog === "import" && <BatchDialog title="批量粘贴文案" onClose={() => setDialog(null)}>{error && <p className="tb-inline-note" role="alert">{error}</p>}<p className="tb-muted">每个版本之间用单独一行 --- 分隔。段内换行会保留。</p><label className="tb-field">多版文案<textarea autoFocus rows={12} value={bulkCopy} onChange={(e) => setBulkCopy(e.target.value)} placeholder={"第一版文案\n---\n第二版文案\n---\n第三版文案"} /></label><button className="tb-primary" disabled={!bulkCopy.trim()} onClick={() => { let imported; try { imported = parseBulkCopies(bulkCopy); } catch (e) { setError(e.message); return; } if (addImported(imported)) { setDialog(null); setBulkCopy(""); } }}>导入 {bulkCopy.split(/^\s*---\s*$/m).filter((text) => text.trim()).length} 个版本</button></BatchDialog>}
    {dialog === "drafts" && <BatchDialog title="已存批次" onClose={() => setDialog(null)}><p className="tb-muted">保存在当前浏览器，包含原素材及各工具的版本配置。</p><div className="tb-draft-list">{drafts.length ? drafts.map((draft) => <article key={draft.id}><button onClick={() => restore(draft)}><Layers size={18} /><span><strong>{draft.batchName || `${MODE_LABELS[draft.mode]}批次`}</strong><small>{draft.versionsByMode?.[draft.mode]?.length || 1} 个版本 · {new Date(draft.updatedAt).toLocaleString("zh-CN")}</small></span></button></article>) : <p className="tb-muted">暂无保存的批次。</p>}</div></BatchDialog>}
    {dialog === "review" && <BatchDialog title={`检查 ${summary.selected.length} 个版本`} onClose={() => setDialog(null)}>{error && <p className="tb-inline-note" role="alert">{error}</p>}<p className="tb-muted">原视频：{mediaName(source)} · {resolution}</p><div className="tb-review-list">{summary.selected.map((row) => <div key={row.id}><strong>{row.title || `版本 ${rows.indexOf(row) + 1}`}</strong><span>{versionIssues(mode, row).join("、") || (summary.duplicateIds.has(row.id) ? "内容重复，请调整" : "已配置")}</span></div>)}</div><p className="tb-inline-note">{ready ? "整批配置已就绪。批量生成服务尚未接入，可先保存。" : "请返回补齐缺项，或取消勾选重复版本。"}</p><div className="tb-plan-actions"><button className="tb-secondary" onClick={() => setDialog(null)}>返回编辑</button><button className="tb-primary" disabled={saving} onClick={async () => { if (await save()) setDialog(null); }}><Save size={16} />保存整批配置</button><button className="tb-secondary" disabled title="批量生成服务尚未接入">生成 {summary.selected.length} 条 · 待接入</button></div></BatchDialog>}
  </div>;
}
