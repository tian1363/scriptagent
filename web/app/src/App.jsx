import {
  CheckCircle2,
  Clock3,
  FileText,
  History,
  Loader2,
  Play,
  RefreshCw,
  Send,
  Upload,
  Video,
} from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { createJob, getJob, listJobs, publishJob } from "./api.js";
import logo from "./assets/logo-scriptagent.svg";

const tabs = [
  ["analysis_markdown", "视频分析"],
  ["replica_script_json", "复刻脚本"],
  ["fission_scripts_json", "裂变脚本"],
  ["creatibi_result_json", "CreatiBI"],
];

const runningStatuses = new Set([
  "pending",
  "running",
  "analyzing_video",
  "extracting_structure",
  "generating_replica",
  "generating_fission",
  "validating",
  "publishing",
]);

export function App() {
  const [jobs, setJobs] = useState([]);
  const [selectedJob, setSelectedJob] = useState(null);
  const [activeTab, setActiveTab] = useState("analysis_markdown");
  const [isCreating, setIsCreating] = useState(false);
  const [isPublishing, setIsPublishing] = useState(false);
  const [error, setError] = useState("");

  async function refreshJobs(nextSelectedId) {
    const items = await listJobs();
    setJobs(items);
    const targetId = nextSelectedId || selectedJob?.id || items[0]?.id;
    if (targetId) {
      const current = await getJob(targetId);
      setSelectedJob(current);
    }
  }

  useEffect(() => {
    refreshJobs().catch((err) => setError(err.message));
  }, []);

  useEffect(() => {
    if (!selectedJob || !runningStatuses.has(selectedJob.status)) {
      return undefined;
    }
    const timer = window.setInterval(() => {
      refreshJobs(selectedJob.id).catch((err) => setError(err.message));
    }, 1500);
    return () => window.clearInterval(timer);
  }, [selectedJob?.id, selectedJob?.status]);

  async function handleCreate(event) {
    event.preventDefault();
    const formEl = event.currentTarget;
    setError("");
    setIsCreating(true);
    const form = new FormData(formEl);
    try {
      const created = await createJob(form);
      formEl.reset();
      setActiveTab("analysis_markdown");
      await refreshJobs(created.job_id);
    } catch (err) {
      setError(err.message);
    } finally {
      setIsCreating(false);
    }
  }

  async function handleSelect(id) {
    setError("");
    setActiveTab("analysis_markdown");
    const job = await getJob(id);
    setSelectedJob(job);
  }

  async function handlePublish() {
    if (!selectedJob) return;
    setError("");
    setIsPublishing(true);
    try {
      await publishJob(selectedJob.id);
      setActiveTab("creatibi_result_json");
      await refreshJobs(selectedJob.id);
    } catch (err) {
      setError(err.message);
      await refreshJobs(selectedJob.id);
    } finally {
      setIsPublishing(false);
    }
  }

  const visibleContent = selectedJob?.[activeTab] || "";
  const canPublish = selectedJob?.status === "completed" || selectedJob?.status === "published";
  const hasJobs = jobs.length > 0;

  return (
    <div className="app-shell">
      <header className="topbar">
        <div className="brand">
          <img src={logo} alt="ScriptAgent" />
        </div>
        <button className="icon-button" type="button" onClick={() => refreshJobs().catch((err) => setError(err.message))} title="刷新">
          <RefreshCw size={16} />
        </button>
      </header>

      <main className="workspace">
        <aside className="history-pane">
          <div className="pane-heading">
            <History size={16} />
            <span>历史记录</span>
          </div>
          <div className="history-list">
            {hasJobs ? (
              jobs.map((job) => (
                <button
                  key={job.id}
                  className={`history-item ${selectedJob?.id === job.id ? "active" : ""}`}
                  type="button"
                  onClick={() => handleSelect(job.id).catch((err) => setError(err.message))}
                >
                  <span className="history-title">{job.title}</span>
                  <span className="history-meta">{job.video_original_name}</span>
                  <span className={`status-pill ${statusTone(job.status)}`}>
                    {statusLabel(job.status)}
                  </span>
                </button>
              ))
            ) : (
              <div className="empty-history">
                <FileText size={20} />
                <span>暂无生成记录</span>
              </div>
            )}
          </div>
        </aside>

        <section className="main-pane">
          <section className="composer">
            <div>
              <h1>创建脚本任务</h1>
              <p>上传参考视频和产品 Markdown，生成复刻脚本与裂变脚本。</p>
            </div>
            <form className="task-form" onSubmit={handleCreate}>
              <label>
                <span>任务标题</span>
                <input name="title" type="text" placeholder="默认使用产品 Markdown 文件名" />
              </label>
              <div className="upload-grid">
                <FileField name="video" label="参考视频" accept="video/mp4,video/quicktime,video/webm" icon={<Video size={16} />} />
                <FileField name="product_md" label="产品 Markdown" accept=".md,.markdown,text/markdown" icon={<FileText size={16} />} />
              </div>
              <label>
                <span>补充要求</span>
                <textarea name="requirement" rows="4" placeholder="例如：面向 TikTok，节奏更快，避免夸大功效。" />
              </label>
              <div className="settings-row">
                <label>
                  <span>行业</span>
                  <select name="industry" defaultValue="auto">
                    <option value="auto">自动判断</option>
                    <option value="game">游戏</option>
                    <option value="ecommerce">电商</option>
                  </select>
                </label>
                <label>
                  <span>裂变数量</span>
                  <input name="fission_count" type="number" min="1" max="20" defaultValue="5" />
                </label>
                <button className="primary-button" type="submit" disabled={isCreating}>
                  {isCreating ? <Loader2 className="spin" size={16} /> : <Play size={16} />}
                  <span>{isCreating ? "创建中" : "生成"}</span>
                </button>
              </div>
            </form>
            {error ? <div className="error-banner">{error}</div> : null}
          </section>

          <section className="result-pane">
            <ResultHeader job={selectedJob} isPublishing={isPublishing} canPublish={canPublish} onPublish={handlePublish} />
            <div className="tabs">
              {tabs.map(([key, label]) => (
                <button key={key} className={activeTab === key ? "active" : ""} type="button" onClick={() => setActiveTab(key)}>
                  {label}
                </button>
              ))}
            </div>
            <ResultContent job={selectedJob} content={visibleContent} activeTab={activeTab} />
          </section>
        </section>
      </main>
    </div>
  );
}

function FileField({ name, label, accept, icon }) {
  return (
    <label className="file-field">
      <span>{label}</span>
      <div className="file-input">
        {icon}
        <input name={name} type="file" accept={accept} required />
        <Upload size={16} />
      </div>
    </label>
  );
}

function ResultHeader({ job, isPublishing, canPublish, onPublish }) {
  const subtitle = useMemo(() => {
    if (!job) return "选择历史记录或创建新任务后查看结果。";
    return `${job.video_original_name} · ${job.product_md_name}`;
  }, [job]);

  return (
    <div className="result-header">
      <div>
        <h2>{job?.title || "任务结果"}</h2>
        <p>{subtitle}</p>
      </div>
      {job ? (
        <div className="result-actions">
          <span className={`status-pill ${statusTone(job.status)}`}>
            {runningStatuses.has(job.status) ? <Clock3 size={13} /> : <CheckCircle2 size={13} />}
            {statusLabel(job.status)}
          </span>
          <button className="secondary-button" type="button" onClick={onPublish} disabled={!canPublish || isPublishing}>
            {isPublishing ? <Loader2 className="spin" size={16} /> : <Send size={16} />}
            <span>{job.status === "published" ? "重新发布" : "发布至 CreatiBI"}</span>
          </button>
        </div>
      ) : null}
    </div>
  );
}

function ResultContent({ job, content, activeTab }) {
  if (!job) {
    return <EmptyState text="暂无任务结果" />;
  }
  if (job.error_message && job.status === "failed") {
    return <pre className="result-output error-output">{job.error_message}</pre>;
  }
  if (!content) {
    return <EmptyState text={runningStatuses.has(job.status) ? "任务执行中" : "当前页暂无内容"} />;
  }
  const mode = activeTab === "analysis_markdown" ? "markdown" : "json";
  return <pre className={`result-output ${mode}`}>{content}</pre>;
}

function EmptyState({ text }) {
  return (
    <div className="empty-state">
      <FileText size={24} />
      <span>{text}</span>
    </div>
  );
}

function statusLabel(status) {
  const labels = {
    pending: "等待中",
    running: "运行中",
    analyzing_video: "分析视频",
    extracting_structure: "提取结构",
    generating_replica: "生成复刻",
    generating_fission: "生成裂变",
    validating: "校验中",
    completed: "已完成",
    publishing: "发布中",
    published: "已发布",
    failed: "失败",
  };
  return labels[status] || status;
}

function statusTone(status) {
  if (status === "failed") return "danger";
  if (status === "completed" || status === "published") return "success";
  return "neutral";
}
