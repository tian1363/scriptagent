import { useEffect, useRef, useState } from 'react';
import { ChevronDown, Loader2, Plus, Sparkles, Upload, X } from 'lucide-react';
import { listCharacters, uploadCharacter, generateCharacter } from './api.js';
import './CharacterLibrary.css';

export const characterImage = (role) => `/api/characters/${role.id}/image`;
const defaults = { description:'自然真实的成年生活方式创作者', age:28, hair:'自然棕色中长发', outfit:'简洁日常休闲装', expression:'自然微笑', pose:'正面站立，双手自然放松', framing:'半身，完整头部入镜', background:'浅灰纯色背景', lighting:'柔和自然光', size:'720*1280', seed:42, negative:'' };
const presets = [
 {name:'北美生活方式', description:'面向北美生活方式广告的成年女性创作者，自然真实的个人风格', age:28,hair:'棕色微卷长发',outfit:'米白色针织上衣'},
 {name:'运动分享', description:'成年男性运动内容创作者，自然健康的体态',age:30,hair:'黑色短发',outfit:'无品牌标识的运动上衣'},
 {name:'家居日常', description:'成年女性家居内容创作者，亲切自然',age:38,hair:'深棕色齐肩发',outfit:'浅蓝色休闲衬衫'},
 {name:'专业讲解', description:'成年男性产品讲解者，整洁自然',age:45,hair:'整洁深色短发',outfit:'简洁深灰色衬衫'},
];
export default function CharacterLibrary({ initialTab='library', onPick, onClose }) {
 const dialog=useRef(null), upload=useRef(null);
 const [tab,setTab]=useState(initialTab),[roles,setRoles]=useState([]),[reload,setReload]=useState(0);
 const [loading,setLoading]=useState(true),[busy,setBusy]=useState(false),[error,setError]=useState('');
 const [name,setName]=useState('新角色'),[controls,setControls]=useState(defaults),[reference,setReference]=useState(null),[preview,setPreview]=useState(null);
 const [file,setFile]=useState(null),[fileURL,setFileURL]=useState(''),[query,setQuery]=useState('');
 useEffect(()=>{dialog.current.showModal();},[]);
 useEffect(()=>{if(!file){setFileURL('');return;}const url=URL.createObjectURL(file);setFileURL(url);return()=>URL.revokeObjectURL(url);},[file]);
 useEffect(()=>{
  const controller=new AbortController();let timer;
  async function refresh(){try{const next=await listCharacters(controller.signal);if(controller.signal.aborted)return;setRoles(next);setLoading(false);if(next.some(r=>r.status==='generating'))timer=setTimeout(refresh,2500);}catch(e){if(!controller.signal.aborted){setError(e.message);setLoading(false);}}}
  refresh();return()=>{controller.abort();clearTimeout(timer);};
 },[reload]);
 const ready=roles.filter(r=>r.status==='ready');
 const visible=roles.filter(r=>r.name.toLowerCase().includes(query.trim().toLowerCase()));
 const selected=roles.find(r=>r.id===preview);
 const base=selected?.reference_id ? roles.find(r=>r.id===selected.reference_id) : null;
 function field(key,value){setControls(c=>({...c,[key]:value}));}
 function changeLook(role){setReference(role);setControls({...defaults,...role.controls});setName(`${role.name} · 新造型`.slice(0,80));setTab('generate');setError('');setPreview(null);}
 async function create(){setBusy(true);setError('');try{const role=await generateCharacter({name,reference_id:reference?.id||'',controls});setTab('library');setQuery('');setPreview(role.id);setReload(x=>x+1);}catch(e){setError(e.message);}finally{setBusy(false);}}
 async function saveUpload(){if(!file)return;setBusy(true);setError('');try{const role=await uploadCharacter(name,file);setFile(null);setTab('library');setQuery('');setPreview(role.id);setReload(x=>x+1);}catch(e){setError(e.message);}finally{setBusy(false);}}
 function pick(role){onPick({id:role.id,character_id:role.id,name:role.name,kind:'image'});}
 return <dialog ref={dialog} className="character-dialog" aria-labelledby="character-library-title" onCancel={onClose}>
  <header><div><h2 id="character-library-title">角色库</h2><p>选定一个角色，在不同版本中复用。</p></div><button className="tb-icon-button" aria-label="关闭角色库" onClick={onClose}><X size={20}/></button></header>
  <nav className="character-tabs" aria-label="角色来源">{[['library','我的角色'],['generate','AI 生成角色'],['upload','上传角色']].map(([id,label])=><button key={id} aria-pressed={tab===id} disabled={busy} onClick={()=>{setTab(id);setError('');}}>{label}</button>)}</nav>
  <div className="character-body">
   {error && <div className="tb-inline-note" role="alert">{error}<button className="tb-text-button" onClick={()=>{setError('');setReload(x=>x+1);}}>重新读取角色库</button></div>}
   {tab==='library' && <>
    <label className="tb-field">查找角色<input value={query} onChange={e=>setQuery(e.target.value)} placeholder="搜索角色名称"/></label>
    {loading ? <p role="status">正在读取角色…</p> : !roles.length ? <div className="character-empty"><h3>建立你的第一个角色</h3><p>上传现有角色图，或用 AI 生成一个可反复使用的角色。</p><button className="tb-primary" onClick={()=>setTab('generate')}><Sparkles size={16}/>AI 生成角色</button></div> : <div className="character-grid">{visible.map(role=><article key={role.id} className={preview===role.id?'selected':''}>
     <button className="character-preview-button" onClick={()=>setPreview(role.id)} aria-label={`查看${role.name}`}>{role.status==='ready'?<img src={characterImage(role)} alt={role.name}/>:<div className="character-pending">{role.status==='generating'?<><Loader2 className="spin"/>生成中</>:'未生成成功'}</div>}<strong>{role.name}</strong></button>
     <small>{role.source==='upload'?'上传角色':role.source==='edit'?'角色新造型':'AI 角色'}</small>
     {role.status==='ready'?<div className="character-card-actions"><button className="tb-secondary" onClick={()=>pick(role)}>使用</button><button className="tb-text-button" onClick={()=>changeLook(role)}>改造型</button></div>:role.status==='failed'?<p role="alert">{role.error}</p>:<p>关闭窗口后仍会继续生成</p>}
    </article>)}</div>}
    {!!roles.length && !visible.length && <p className="tb-muted">没有匹配的角色。</p>}
    {selected?.status==='ready' && <section className="character-comparison" aria-label="角色预览">{base?.status==='ready'&&<figure><img src={characterImage(base)} alt="原角色"/><figcaption>基准角色</figcaption></figure>}<figure><img src={characterImage(selected)} alt={selected.name}/><figcaption>{selected.name}</figcaption></figure><div><p>{base?'检查五官、肤色与发型是否延续原角色，再采用新造型。':'确认角色外观后，加入当前换模特版本。'}</p><button className="tb-primary" onClick={()=>pick(selected)}>使用此角色</button></div></section>}
   </>}
   {tab==='upload'&&<div className="character-upload"><label className="tb-field">角色名称<input value={name} maxLength={80} onChange={e=>setName(e.target.value)}/></label><input ref={upload} type="file" hidden accept=".jpg,.jpeg,.png" onChange={e=>{setFile(e.target.files?.[0]||null);setError('');}}/><button className="tb-dropzone" onClick={()=>upload.current?.click()}><Upload size={24}/><strong>{file?file.name:'选择角色图'}</strong><small>PNG / JPG · 最大 10MB · 宽高 384–3072 像素</small></button>{fileURL&&<img className="character-upload-preview" src={fileURL} alt="待保存角色"/>}</div>}
   {tab==='generate'&&<>
    <label className="tb-field">角色名称<input maxLength={80} value={name} onChange={e=>setName(e.target.value)}/></label>
    <label className="tb-field">身份基准<select value={reference?.id||''} onChange={e=>{const role=ready.find(r=>r.id===e.target.value);setReference(role||null);}}><option value="">创建一个新角色</option>{ready.map(r=><option key={r.id} value={r.id}>沿用：{r.name}</option>)}</select></label>
    {reference?<div className="character-reference"><img src={characterImage(reference)} alt="身份基准"/><div><strong>以「{reference.name}」为基准</strong><p>保持五官、肤色、年龄与发型，仅调整下面的造型。生成后需对比确认。</p></div></div>:<>
     <div className="character-presets" aria-label="角色设定模板">{presets.map(p=><button key={p.name} onClick={()=>{setName(p.name);const {name: presetName,...values}=p;setControls({...defaults,...values});}}>{p.name}</button>)}</div><p className="tb-muted">模板只提供创作设定，点击生成后才会创建角色图。</p>
     <label className="tb-field">外观描述<textarea rows={2} maxLength={200} value={controls.description} onChange={e=>field('description',e.target.value)}/></label>
     <div className="character-fields"><label className="tb-field">年龄<input type="number" min={18} max={90} value={controls.age} onChange={e=>field('age',Number(e.target.value))}/></label><label className="tb-field">发型<input maxLength={200} value={controls.hair} onChange={e=>field('hair',e.target.value)}/></label></div>
    </>}
    <div className="character-fields">{[['outfit','服装'],['expression','表情'],['pose','姿态'],['framing','构图'],['background','背景'],['lighting','光线']].map(([key,label])=><label className="tb-field" key={key}>{label}<input maxLength={200} value={controls[key]} onChange={e=>field(key,e.target.value)}/></label>)}</div>
    <details className="tb-details"><summary>更多控制<ChevronDown size={16}/></summary><div className="character-fields"><label className="tb-field">图片尺寸<select value={controls.size} onChange={e=>field('size',e.target.value)}><option value="720*1280">9:16 · 720 × 1280</option><option value="960*1280">3:4 · 960 × 1280</option><option value="1024*1024">1:1 · 1024 × 1024</option></select></label><label className="tb-field">种子<input type="number" min={0} max={2147483647} value={controls.seed} onChange={e=>field('seed',Number(e.target.value))}/></label></div><button className="tb-text-button" onClick={()=>field('seed',Math.floor(Math.random()*2147483648))}>换一个种子</button><label className="tb-field">排除内容<input maxLength={200} value={controls.negative} onChange={e=>field('negative',e.target.value)} placeholder="例如：帽子、墨镜、夸张妆容"/></label><p className="tb-muted">相同种子有助于稳定结果，不能代替身份参考图。服装与姿态是生成约束，并非像素级锁定。</p></details>
   </>}
  </div>
  {tab!=='library'&&<footer><small>{tab==='generate'?'一次生成 1 张，使用已配置的图片模型。':'保存在当前账号的角色库中。'}</small><button className="tb-primary" disabled={busy||!name.trim()||(tab==='upload'&&!file)} onClick={tab==='upload'?saveUpload:create}>{busy?<Loader2 className="spin" size={16}/>:tab==='generate'?<Sparkles size={16}/>:<Plus size={16}/>}{busy?'正在提交…':tab==='upload'?'保存角色':reference?'生成新造型':'生成角色图'}</button></footer>}
 </dialog>;
}
