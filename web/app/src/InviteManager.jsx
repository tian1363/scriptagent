import { useEffect, useState } from "react";
import { listInvites, createInvite, revokeInvite } from "./api";

const labels = { unused: "未使用", used: "已使用", expired: "已过期", revoked: "已停用" };
const time = value => value ? new Date(value).toLocaleString("zh-CN", {hour12: false}) : "—";

export default function InviteManager() {
  const [items, setItems] = useState([]);
  const [filter, setFilter] = useState("");
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [code, setCode] = useState("");
  const [copied, setCopied] = useState(false);
  const refresh = async () => { setLoading(true); try {setItems(await listInvites());} finally {setLoading(false);} };
  useEffect(() => { refresh().catch(err => setError(err.message)); }, []);
  async function generate(event) {
    event.preventDefault(); const form = new FormData(event.currentTarget);
    setBusy(true); setError("");
    try {
      const result = await createInvite({ label: form.get("label"), days: Number(form.get("days")) });
      setCode(result.code); setCopied(false); setOpen(false); await refresh();
    } catch (err) {setError(err.message);} finally {setBusy(false);}
  }
  async function stop(item) {
    if (!window.confirm(`停用“${item.label || item.id.slice(0,8)}”？停用后将不能用于注册。`)) return;
    setBusy(true); setError("");
    try {await revokeInvite(item.id); await refresh();} catch(err) {setError(err.message);} finally {setBusy(false);}
  }
  return <section className="invite-manager" aria-label="邀请码管理">
    <header className="invite-toolbar"><h2>邀请码</h2><div>
      <button type="button" className="secondary-button" disabled={loading || busy} onClick={() => {setError(""); refresh().catch(err => setError(err.message));}}>刷新</button>
      <button type="button" className="primary-button" disabled={busy} onClick={() => setOpen(!open)}>{open ? "取消" : "生成邀请码"}</button>
    </div></header>
    {open && <form className="invite-form" onSubmit={generate}>
      <label>备注<input name="label" maxLength={100} placeholder="例如：首批体验用户" /></label>
      <label>有效期<select name="days" defaultValue="7"><option value="1">1 天</option><option value="7">7 天</option><option value="30">30 天</option><option value="90">90 天</option></select></label>
      <button className="primary-button" disabled={busy}>{busy ? "生成中" : "生成"}</button>
    </form>}
    {code && <div className="invite-code" role="status"><div><strong>邀请码已生成</strong><p>仅展示一次，请复制保存。每码仅限一个账号注册。</p><code>{code}</code></div>
      <button type="button" className="secondary-button" onClick={async () => {try {await navigator.clipboard.writeText(code); setCopied(true);} catch {setError("复制失败，请手动选中邀请码复制");}}}>{copied ? "已复制" : "复制"}</button>
      <button type="button" className="text-button" onClick={() => {if(copied || window.confirm("关闭后无法找回完整邀请码，确认已保存？")) setCode("");}}>关闭</button>
    </div>}
    <label className="invite-filter">状态 <select value={filter} onChange={e => setFilter(e.target.value)}><option value="">全部</option>{Object.entries(labels).map(([key,label]) => <option key={key} value={key}>{label}</option>)}</select></label>
    {error && <p role="alert" className="error-banner">{error}</p>}
    {loading ? <p role="status">加载中</p> : <div className="invite-list">
      {items.filter(item => !filter || item.status === filter).map(item => <article key={item.id} className="invite-row">
        <div><strong>{item.label || "未备注"}</strong><small>编号 {item.id.slice(0,8)} · 创建于 {time(item.created_at)}</small></div>
        <div><strong>{labels[item.status]}</strong><small>有效期至 {item.expires_at ? time(item.expires_at) : "长期有效"}</small></div>
        <div><strong>{item.status === "used" ? (item.email || "历史记录未留存") : "尚未使用"}</strong><small>{item.used_at ? `使用于 ${time(item.used_at)}` : "—"}</small></div>
        <button type="button" className="secondary-button" disabled={busy || item.status !== "unused"} onClick={() => stop(item)}>停用</button>
      </article>)}
      {!items.some(item => !filter || item.status === filter) && <p>暂无{filter ? labels[filter] + "的" : ""}邀请码</p>}
    </div>}
  </section>;
}
