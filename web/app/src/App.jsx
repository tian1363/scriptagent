import {
  Activity,
  Bot,
  CheckCircle2,
  Clock3,
  FileText,
  History,
  Loader2,
  MessageSquare,
  Package,
  Play,
  Plus,
  RefreshCw,
  RotateCcw,
  Send,
  Settings,
  KeyRound,
  Upload,
  Video,
} from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import {
  createJob,
  createProduct,
  getModelSettings,
  getChat,
  getJob,
  getProductMarkdown,
  listProducts,
  listChats,
  listJobs,
  listModelCalls,
  publishJob,
  retryJob,
  saveModelSettings,
  sendChatMessage,
  sendNewChatMessage,
} from "./api.js";
import logo from "./assets/logo-scriptagent.svg";

const resultTabs = [
  ["run_log", "运行日志"],
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

const fissionDirectionGroups = [
  {
    layer: "视听层",
    items: ["换BGM", "换音效", "换色调/滤镜", "换字幕&花字", "换画幅", "换配音(语速/声线)"],
  },
  {
    layer: "结构层",
    items: ["换开头钩子", "换CTA", "时长压缩/拉伸", "变速·节奏调整", "换首帧/封面", "同素材高光重剪"],
  },
  {
    layer: "元素层",
    items: ["换局部角色/群演", "换局部场景贴片", "换局部道具/UI", "字幕语言本地化"],
  },
];

const fissionDirections = fissionDirectionGroups.flatMap((group) =>
  group.items.map((item) => ({
    layer: group.layer,
    label: item,
    value: `${group.layer}-${item}`,
  })),
);

function defaultFissionDirection(index) {
  return fissionDirections[index % fissionDirections.length]?.value || "";
}

function typingStep(length) {
  if (length > 1800) return 8;
  if (length > 900) return 5;
  if (length > 360) return 3;
  return 2;
}

function lastAssistantMessage(messages = []) {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    if (messages[index].role === "assistant") return messages[index];
  }
  return null;
}

export function App() {
  const [view, setView] = useState("jobs");
  const [jobs, setJobs] = useState([]);
  const [selectedJob, setSelectedJob] = useState(null);
  const [activeTab, setActiveTab] = useState("run_log");
  const [isCreating, setIsCreating] = useState(false);
  const [isPublishing, setIsPublishing] = useState(false);
  const [isRetrying, setIsRetrying] = useState(false);
  const [products, setProducts] = useState([]);
  const [selectedProductId, setSelectedProductId] = useState("");
  const [productPreview, setProductPreview] = useState(null);
  const [isLoadingProductPreview, setIsLoadingProductPreview] = useState(false);
  const [isCreatingProduct, setIsCreatingProduct] = useState(false);
  const [modelSettings, setModelSettings] = useState(null);
  const [isSavingSettings, setIsSavingSettings] = useState(false);

  const [chats, setChats] = useState([]);
  const [selectedChat, setSelectedChat] = useState(null);
  const [chatDraft, setChatDraft] = useState("");
  const [chatProductId, setChatProductId] = useState("");
  const [optimisticChatMessages, setOptimisticChatMessages] = useState(null);
  const [typingMessage, setTypingMessage] = useState(null);
  const [isChatThinking, setIsChatThinking] = useState(false);
  const [isSending, setIsSending] = useState(false);

  const [modelCalls, setModelCalls] = useState([]);
  const [selectedCallId, setSelectedCallId] = useState("");
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

  async function refreshChats(nextSelectedId) {
    const items = await listChats();
    setChats(items);
    const targetId = nextSelectedId || selectedChat?.conversation?.id || items[0]?.id;
    if (targetId) {
      setSelectedChat(await getChat(targetId));
    }
  }

  async function refreshModelCalls() {
    const items = await listModelCalls({ limit: 100 });
    setModelCalls(items);
    if (!selectedCallId && items[0]) {
      setSelectedCallId(items[0].id);
    }
  }

  async function refreshProducts(nextSelectedId) {
    const items = await listProducts();
    setProducts(items);
    if (nextSelectedId || (!selectedProductId && items[0])) {
      setSelectedProductId(nextSelectedId || items[0]?.id || "");
    }
  }

  async function refreshModelSettings() {
    setModelSettings(await getModelSettings());
  }

  async function refreshCurrent() {
    setError("");
    if (view === "jobs") await refreshJobs();
    if (view === "chat") await refreshChats();
    if (view === "calls") await refreshModelCalls();
    if (view === "products") await refreshProducts();
    if (view === "settings") await refreshModelSettings();
  }

  useEffect(() => {
    refreshJobs().catch((err) => setError(err.message));
    refreshChats().catch(() => {});
    refreshModelCalls().catch(() => {});
    refreshProducts().catch(() => {});
    refreshModelSettings().catch(() => {});
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

  useEffect(() => {
    if (view !== "products" || !selectedProductId) {
      setProductPreview(null);
      return undefined;
    }
    let cancelled = false;
    setIsLoadingProductPreview(true);
    getProductMarkdown(selectedProductId)
      .then((preview) => {
        if (!cancelled) setProductPreview(preview);
      })
      .catch((err) => {
        if (!cancelled) setProductPreview({ error: err.message });
      })
      .finally(() => {
        if (!cancelled) setIsLoadingProductPreview(false);
      });
    return () => {
      cancelled = true;
    };
  }, [view, selectedProductId]);

  useEffect(() => {
    if (!typingMessage || typingMessage.visible.length >= typingMessage.content.length) {
      return undefined;
    }
    const timer = window.setTimeout(() => {
      setTypingMessage((current) => {
        if (!current) return current;
        const nextLength = Math.min(current.content.length, current.visible.length + typingStep(current.content.length));
        return { ...current, visible: current.content.slice(0, nextLength) };
      });
    }, 18);
    return () => window.clearTimeout(timer);
  }, [typingMessage]);

  useEffect(() => {
    if (!typingMessage || typingMessage.visible.length < typingMessage.content.length) {
      return undefined;
    }
    const timer = window.setTimeout(() => {
      setTypingMessage(null);
      setOptimisticChatMessages(null);
    }, 220);
    return () => window.clearTimeout(timer);
  }, [typingMessage]);

  async function handleCreate(event) {
    event.preventDefault();
    const formEl = event.currentTarget;
    setError("");
    setIsCreating(true);
    const form = new FormData(formEl);
    try {
      const created = await createJob(form);
      formEl.reset();
      setActiveTab("run_log");
      await refreshJobs(created.job_id);
      await refreshProducts();
    } catch (err) {
      setError(err.message);
    } finally {
      setIsCreating(false);
    }
  }

  async function handleSelectJob(id) {
    setError("");
    setActiveTab("run_log");
    setSelectedJob(await getJob(id));
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

  async function handleRetry() {
    if (!selectedJob) return;
    setError("");
    setIsRetrying(true);
    try {
      const retried = await retryJob(selectedJob.id);
      setActiveTab("run_log");
      await refreshJobs(retried.job_id);
    } catch (err) {
      setError(err.message);
    } finally {
      setIsRetrying(false);
    }
  }

  async function handleSelectChat(id) {
    setError("");
    setOptimisticChatMessages(null);
    setTypingMessage(null);
    setIsChatThinking(false);
    setSelectedChat(await getChat(id));
  }

  function handleNewChat() {
    setError("");
    setSelectedChat(null);
    setOptimisticChatMessages(null);
    setTypingMessage(null);
    setIsChatThinking(false);
  }

  async function handleSendChat(event) {
    event.preventDefault();
    const content = chatDraft.trim();
    if (!content) return;
    setError("");
    setIsSending(true);
    setIsChatThinking(true);
    setTypingMessage(null);
    const tempUserMessage = {
      id: `temp-user-${Date.now()}`,
      role: "user",
      content,
      created_at: new Date().toISOString(),
    };
    const baseMessages = selectedChat?.messages || [];
    setOptimisticChatMessages([...baseMessages, tempUserMessage]);
    setChatDraft("");
    try {
      const thread = selectedChat?.conversation?.id
        ? await sendChatMessage(selectedChat.conversation.id, content, chatProductId)
        : await sendNewChatMessage(content, chatProductId);
      const assistantMessage = lastAssistantMessage(thread.messages);
      setSelectedChat(thread);
      if (assistantMessage) {
        setOptimisticChatMessages(thread.messages.filter((message) => message.id !== assistantMessage.id));
        setTypingMessage({ ...assistantMessage, visible: "" });
      } else {
        setOptimisticChatMessages(null);
      }
      await refreshChats(thread.conversation.id);
      await refreshModelCalls();
    } catch (err) {
      setError(err.message);
      setTypingMessage(null);
      setOptimisticChatMessages((current) => [
        ...(current || [tempUserMessage]),
        {
          id: `temp-error-${Date.now()}`,
          role: "assistant",
          content: `模型响应失败：${err.message}`,
          created_at: new Date().toISOString(),
        },
      ]);
    } finally {
      setIsChatThinking(false);
      setIsSending(false);
    }
  }

  async function handleCreateProduct(event) {
    event.preventDefault();
    const formEl = event.currentTarget;
    setError("");
    setIsCreatingProduct(true);
    try {
      const product = await createProduct(new FormData(formEl));
      formEl.reset();
      await refreshProducts(product.id);
    } catch (err) {
      setError(err.message);
    } finally {
      setIsCreatingProduct(false);
    }
  }

  async function handleSaveModelSettings(event) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setError("");
    setIsSavingSettings(true);
    try {
      const next = await saveModelSettings({
        api_key: String(form.get("api_key") || ""),
        endpoint: String(form.get("endpoint") || ""),
        model: String(form.get("model") || ""),
      });
      setModelSettings(next);
      event.currentTarget.reset();
    } catch (err) {
      setError(err.message);
    } finally {
      setIsSavingSettings(false);
    }
  }

  const visibleContent = selectedJob?.[activeTab] || "";
  const canPublish = selectedJob?.status === "completed" || selectedJob?.status === "published";
  const canRetry = selectedJob && !runningStatuses.has(selectedJob.status);
  const selectedCall = modelCalls.find((call) => call.id === selectedCallId) || modelCalls[0];

  return (
    <div className="app-shell">
      <header className="topbar">
        <div className="brand">
          <img src={logo} alt="ScriptAgent" />
        </div>
        <div className="view-switcher">
          <button className={view === "jobs" ? "active" : ""} type="button" onClick={() => setView("jobs")}>
            <FileText size={15} />
            <span>脚本任务</span>
          </button>
          <button className={view === "products" ? "active" : ""} type="button" onClick={() => setView("products")}>
            <Package size={15} />
            <span>产品库</span>
          </button>
          <button className={view === "chat" ? "active" : ""} type="button" onClick={() => setView("chat")}>
            <MessageSquare size={15} />
            <span>通用对话</span>
          </button>
          <button className={view === "calls" ? "active" : ""} type="button" onClick={() => setView("calls")}>
            <Activity size={15} />
            <span>模型调用</span>
          </button>
          <button className={view === "settings" ? "active" : ""} type="button" onClick={() => setView("settings")}>
            <Settings size={15} />
            <span>配置</span>
          </button>
        </div>
        <button className="icon-button" type="button" onClick={() => refreshCurrent().catch((err) => setError(err.message))} title="刷新">
          <RefreshCw size={16} />
        </button>
      </header>

      <main className="workspace">
        {view === "jobs" ? (
          <JobsSidebar jobs={jobs} selectedJob={selectedJob} onSelect={handleSelectJob} />
        ) : null}
        {view === "products" ? (
          <ProductsSidebar products={products} selectedProductId={selectedProductId} onSelect={setSelectedProductId} />
        ) : null}
        {view === "chat" ? (
          <ChatSidebar chats={chats} selectedChat={selectedChat} onSelect={handleSelectChat} onNew={handleNewChat} />
        ) : null}
        {view === "calls" ? (
          <CallsSidebar calls={modelCalls} selectedCallId={selectedCall?.id} onSelect={setSelectedCallId} />
        ) : null}
        {view === "settings" ? <SettingsSidebar modelSettings={modelSettings} /> : null}

        <section className="main-pane">
          {view === "jobs" ? (
            <JobsWorkspace
              selectedJob={selectedJob}
              activeTab={activeTab}
              visibleContent={visibleContent}
              isCreating={isCreating}
              isPublishing={isPublishing}
              isRetrying={isRetrying}
              canPublish={canPublish}
              canRetry={canRetry}
              products={products}
              error={error}
              onCreate={handleCreate}
              onPublish={handlePublish}
              onRetry={handleRetry}
              onTab={setActiveTab}
            />
          ) : null}
          {view === "products" ? (
            <ProductsWorkspace
              products={products}
              selectedProductId={selectedProductId}
              productPreview={productPreview}
              isLoadingProductPreview={isLoadingProductPreview}
              isCreatingProduct={isCreatingProduct}
              error={error}
              onCreate={handleCreateProduct}
            />
          ) : null}
          {view === "chat" ? (
            <ChatWorkspace
              thread={selectedChat}
              optimisticMessages={optimisticChatMessages}
              typingMessage={typingMessage}
              isThinking={isChatThinking}
              draft={chatDraft}
              products={products}
              selectedProductId={chatProductId}
              isSending={isSending}
              error={error}
              onDraft={setChatDraft}
              onProduct={setChatProductId}
              onSend={handleSendChat}
            />
          ) : null}
          {view === "calls" ? <ModelCallsWorkspace call={selectedCall} error={error} /> : null}
          {view === "settings" ? (
            <SettingsWorkspace modelSettings={modelSettings} isSaving={isSavingSettings} error={error} onSave={handleSaveModelSettings} />
          ) : null}
        </section>
      </main>
    </div>
  );
}

function JobsSidebar({ jobs, selectedJob, onSelect }) {
  return (
    <aside className="history-pane">
      <div className="pane-heading">
        <History size={16} />
        <span>历史记录</span>
      </div>
      <div className="history-list">
        {jobs.length ? (
          jobs.map((job) => (
            <button
              key={job.id}
              className={`history-item ${selectedJob?.id === job.id ? "active" : ""}`}
              type="button"
              onClick={() => onSelect(job.id).catch(() => {})}
            >
              <span className="history-title">{job.title}</span>
              <span className="history-meta">{job.video_original_name}</span>
              <span className={`status-pill ${statusTone(job.status)}`}>{statusLabel(job.status)}</span>
            </button>
          ))
        ) : (
          <EmptyState text="暂无生成记录" compact />
        )}
      </div>
    </aside>
  );
}

function ProductsSidebar({ products, selectedProductId, onSelect }) {
  return (
    <aside className="history-pane">
      <div className="pane-heading">
        <Package size={16} />
        <span>产品资料库</span>
      </div>
      <div className="history-list">
        {products.length ? (
          products.map((product) => (
            <button
              key={product.id}
              className={`history-item ${selectedProductId === product.id ? "active" : ""}`}
              type="button"
              onClick={() => onSelect(product.id)}
            >
              <span className="history-title">{product.title}</span>
              <span className="history-meta">{product.md_name}</span>
              <span className="history-meta">{formatTime(product.updated_at)}</span>
            </button>
          ))
        ) : (
          <EmptyState text="暂无产品" compact />
        )}
      </div>
    </aside>
  );
}

function SettingsSidebar({ modelSettings }) {
  return (
    <aside className="history-pane">
      <div className="pane-heading">
        <Settings size={16} />
        <span>配置中心</span>
      </div>
      <div className="history-list">
        <div className="config-card">
          <span className={`status-pill ${modelSettings?.configured ? "success" : "danger"}`}>
            {modelSettings?.configured ? "模型已配置" : "模型未配置"}
          </span>
          <p>{modelSettings?.source === "user" ? "使用用户配置" : "使用环境变量"}</p>
        </div>
      </div>
    </aside>
  );
}

function ChatSidebar({ chats, selectedChat, onSelect, onNew }) {
  return (
    <aside className="history-pane">
      <div className="pane-heading split">
        <span>
          <MessageSquare size={16} />
          对话记录
        </span>
        <button className="mini-button" type="button" onClick={onNew}>
          新对话
        </button>
      </div>
      <div className="history-list">
        {chats.length ? (
          chats.map((chat) => (
            <button
              key={chat.id}
              className={`history-item ${selectedChat?.conversation?.id === chat.id ? "active" : ""}`}
              type="button"
              onClick={() => onSelect(chat.id).catch(() => {})}
            >
              <span className="history-title">{chat.title}</span>
              <span className="history-meta">{formatTime(chat.updated_at)}</span>
            </button>
          ))
        ) : (
          <EmptyState text="暂无对话" compact />
        )}
      </div>
    </aside>
  );
}

function CallsSidebar({ calls, selectedCallId, onSelect }) {
  return (
    <aside className="history-pane">
      <div className="pane-heading">
        <Activity size={16} />
        <span>模型调用</span>
      </div>
      <div className="history-list">
        {calls.length ? (
          calls.map((call) => (
            <button
              key={call.id}
              className={`history-item ${selectedCallId === call.id ? "active" : ""}`}
              type="button"
              onClick={() => onSelect(call.id)}
            >
              <span className="history-title">{call.step || call.scope}</span>
              <span className="history-meta">{call.scope} · {call.total_tokens || 0} tokens</span>
              <span className={`status-pill ${call.error_message ? "danger" : "success"}`}>
                {call.error_message ? "失败" : "成功"}
              </span>
            </button>
          ))
        ) : (
          <EmptyState text="暂无调用记录" compact />
        )}
      </div>
    </aside>
  );
}

function JobsWorkspace(props) {
  const [fissionCount, setFissionCount] = useState(5);
  const [selectedProductID, setSelectedProductID] = useState("");

  function handleFissionCountChange(event) {
    const next = Number.parseInt(event.target.value, 10);
    setFissionCount(Number.isFinite(next) ? Math.min(20, Math.max(1, next)) : 1);
  }

  return (
    <>
      <section className="composer">
        <div>
          <h1>创建脚本任务</h1>
          <p>上传参考视频和产品 Markdown，生成复刻脚本与裂变脚本。</p>
        </div>
        <form className="task-form" onSubmit={props.onCreate}>
          <div className="form-section">
            <div className="section-heading">
              <span>素材输入</span>
              <small>视频 + 产品信息是脚本生成的核心上下文</small>
            </div>
            <label>
              <span>任务标题</span>
              <input name="title" type="text" placeholder="默认使用产品 Markdown 文件名" />
            </label>
            <div className="upload-grid">
              <FileField name="video" label="参考视频" accept="video/mp4,video/quicktime,video/webm" icon={<Video size={16} />} />
              <label>
                <span>选择产品</span>
                <select name="product_id" value={selectedProductID} onChange={(event) => setSelectedProductID(event.target.value)}>
                  <option value="">上传新产品 Markdown</option>
                  {props.products.map((product) => (
                    <option key={product.id} value={product.id}>
                      {product.title}
                    </option>
                  ))}
                </select>
              </label>
            </div>
            {selectedProductID ? (
              <div className="selected-product-note">
                <Package size={15} />
                <span>已选择产品库资料，本次任务会复用该产品 Markdown。</span>
              </div>
            ) : (
              <div className="upload-grid">
                <label>
                  <span>新产品名称</span>
                  <input name="product_title" type="text" placeholder="默认使用 Markdown 文件名" />
                </label>
                <FileField name="product_md" label="产品 Markdown" accept=".md,.markdown,text/markdown" icon={<FileText size={16} />} required={!selectedProductID} />
              </div>
            )}
          </div>

          <div className="form-section split-section">
            <label className="requirement-field">
              <span>补充要求</span>
              <textarea name="requirement" rows="4" placeholder="例如：面向 TikTok，节奏更快，避免夸大功效。" />
            </label>
            <div className="settings-panel">
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
                <input name="fission_count" type="number" min="1" max="20" value={fissionCount} onChange={handleFissionCountChange} />
              </label>
            </div>
          </div>

          <FissionDirectionPicker count={fissionCount} />

          <div className="submit-row">
            <div>
              <span>生成流程</span>
              <small>视频理解 / 复刻脚本 / {fissionCount} 条裂变脚本</small>
            </div>
            <button className="primary-button" type="submit" disabled={props.isCreating}>
              {props.isCreating ? <Loader2 className="spin" size={16} /> : <Play size={16} />}
              <span>{props.isCreating ? "创建中" : "生成"}</span>
            </button>
          </div>
        </form>
        {props.error ? <div className="error-banner">{props.error}</div> : null}
      </section>

      <section className="result-pane">
        <ResultHeader {...props} job={props.selectedJob} />
        <div className="tabs">
          {resultTabs.map(([key, label]) => (
            <button key={key} className={props.activeTab === key ? "active" : ""} type="button" onClick={() => props.onTab(key)}>
              {label}
            </button>
          ))}
        </div>
        <ResultContent job={props.selectedJob} content={props.visibleContent} activeTab={props.activeTab} />
      </section>
    </>
  );
}

function ChatWorkspace({ thread, optimisticMessages, typingMessage, isThinking, draft, products, selectedProductId, isSending, error, onDraft, onProduct, onSend }) {
  const messages = optimisticMessages || thread?.messages || [];
  const selectedProduct = products.find((product) => product.id === selectedProductId);
  const messagesEndRef = useRef(null);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ block: "end" });
  }, [messages.length, isThinking, typingMessage?.visible]);

  return (
    <section className="chat-pane">
      <div className="result-header">
        <div>
          <h2>{thread?.conversation?.title || "通用对话"}</h2>
          <p>用于讨论脚本策略、裂变方向、发布问题和一般创作问题，可调用产品库资料。</p>
        </div>
        <span className="status-pill neutral">
          <Bot size={13} />
          非 ReAct 工作流
        </span>
      </div>
      <div className="chat-context-bar">
        <label>
          <span>产品上下文</span>
          <select value={selectedProductId} onChange={(event) => onProduct(event.target.value)}>
            <option value="">不调用产品库</option>
            {products.map((product) => (
              <option key={product.id} value={product.id}>
                {product.title}
              </option>
            ))}
          </select>
        </label>
        <div className={`context-chip ${selectedProduct ? "active" : ""}`}>
          <Package size={14} />
          <span>{selectedProduct ? `本轮会引用 ${selectedProduct.md_name}` : "普通对话"}</span>
        </div>
      </div>
      <div className="chat-messages">
        {messages.length || isThinking || typingMessage ? (
          <>
            {messages.map((message) => (
              <ChatMessageBubble key={message.id} message={message} />
            ))}
            {isThinking ? (
              <div className="chat-message assistant thinking">
                <span>助手</span>
                <p>
                  <span className="thinking-dots" aria-label="模型思考中">
                    <i />
                    <i />
                    <i />
                  </span>
                  <span>模型思考中</span>
                </p>
              </div>
            ) : null}
            {typingMessage ? <ChatMessageBubble message={{ ...typingMessage, content: typingMessage.visible }} isTyping /> : null}
            <div ref={messagesEndRef} />
          </>
        ) : (
          <EmptyState text="输入第一条消息开始对话" />
        )}
      </div>
      <form className="chat-form" onSubmit={onSend}>
        {error ? <div className="error-banner">{error}</div> : null}
        <textarea value={draft} onChange={(event) => onDraft(event.target.value)} rows="4" placeholder="输入你的问题，例如：帮我设计 3 个视听层裂变方向。" />
        <button className="primary-button" type="submit" disabled={isSending || !draft.trim()}>
          {isSending ? <Loader2 className="spin" size={16} /> : <Send size={16} />}
          <span>{isSending ? "发送中" : "发送"}</span>
        </button>
      </form>
    </section>
  );
}

function ChatMessageBubble({ message, isTyping = false }) {
  return (
    <div className={`chat-message ${message.role} ${isTyping ? "typing" : ""}`}>
      <span>{message.role === "assistant" ? "助手" : "用户"}</span>
      <p>
        {message.content}
        {isTyping ? <b className="typing-cursor" /> : null}
      </p>
    </div>
  );
}

function FissionDirectionPicker({ count }) {
  const rows = Array.from({ length: count }, (_, index) => index);

  return (
    <div className="direction-panel">
      <div className="direction-heading">
        <span>裂变方向</span>
        <small>每条脚本只选择 1 个裂变元素，可重复使用同一元素</small>
      </div>
      <div className="direction-list">
        {rows.map((index) => (
          <label key={index} className="direction-row">
            <span className="direction-index">{String(index + 1).padStart(2, "0")}</span>
            <span className="direction-title">裂变脚本</span>
            <select name="fission_directions" defaultValue={defaultFissionDirection(index)} required>
              {fissionDirectionGroups.map((group) => (
                <optgroup key={group.layer} label={group.layer}>
                  {group.items.map((item) => {
                    const value = `${group.layer}-${item}`;
                    return (
                      <option key={value} value={value}>
                        {item}
                      </option>
                    );
                  })}
                </optgroup>
              ))}
            </select>
          </label>
        ))}
      </div>
    </div>
  );
}

function ProductsWorkspace({ products, selectedProductId, productPreview, isLoadingProductPreview, isCreatingProduct, error, onCreate }) {
  const selectedProduct = products.find((product) => product.id === selectedProductId) || products[0];
  return (
    <>
      <section className="composer product-composer">
        <div>
          <h1>产品资料库</h1>
          <p>统一维护产品和产品 Markdown。创建脚本任务时可直接选择已有产品。</p>
        </div>
        <form className="task-form" onSubmit={onCreate}>
          <div className="form-section split-section">
            <label>
              <span>产品名称</span>
              <input name="title" type="text" placeholder="例如：无尽冬日 - 游戏信息总结" />
            </label>
            <FileField name="product_md" label="产品 Markdown" accept=".md,.markdown,text/markdown" icon={<FileText size={16} />} required />
          </div>
          <div className="submit-row">
            <div>
              <span>保存产品</span>
              <small>Markdown 会保存到本地资料库，后续任务可复用</small>
            </div>
            <button className="primary-button" type="submit" disabled={isCreatingProduct}>
              {isCreatingProduct ? <Loader2 className="spin" size={16} /> : <Plus size={16} />}
              <span>{isCreatingProduct ? "保存中" : "新增产品"}</span>
            </button>
          </div>
        </form>
        {error ? <div className="error-banner">{error}</div> : null}
      </section>

      <section className="result-pane">
        <div className="result-header">
          <div>
            <h2>{selectedProduct?.title || "产品详情"}</h2>
            <p>{selectedProduct ? `${selectedProduct.md_name} · ${formatTime(selectedProduct.updated_at)}` : "保存产品后可在这里查看详情。"}</p>
          </div>
          {selectedProduct ? <span className="status-pill success">可用于任务</span> : null}
        </div>
        {selectedProduct ? (
          <div className="product-detail">
            <div className="product-meta-grid">
              <div className="detail-row">
                <span>产品 ID</span>
                <strong>{selectedProduct.id}</strong>
              </div>
              <div className="detail-row">
                <span>Markdown 文件</span>
                <strong>{selectedProduct.md_name}</strong>
              </div>
              <div className="detail-row">
                <span>更新时间</span>
                <strong>{formatTime(selectedProduct.updated_at)}</strong>
              </div>
            </div>
            <section className="product-preview">
              <div className="section-heading">
                <span>Markdown 预览</span>
                <small>{isLoadingProductPreview ? "读取中" : productPreview?.md_name || selectedProduct.md_name}</small>
              </div>
              {productPreview?.error ? (
                <div className="error-banner">{productPreview.error}</div>
              ) : isLoadingProductPreview ? (
                <EmptyState text="正在读取产品 Markdown" compact />
              ) : productPreview?.content ? (
                <MarkdownContent content={productPreview.content} />
              ) : (
                <EmptyState text="暂无可预览内容" compact />
              )}
            </section>
          </div>
        ) : (
          <EmptyState text="暂无产品详情" />
        )}
      </section>
    </>
  );
}

function SettingsWorkspace({ modelSettings, isSaving, error, onSave }) {
  return (
    <section className="result-pane full-height">
      <div className="result-header">
        <div>
          <h2>模型配置</h2>
          <p>部署给其他用户使用时，每个用户可在这里配置自己的 DashScope 模型。</p>
        </div>
        <span className={`status-pill ${modelSettings?.configured ? "success" : "danger"}`}>
          <KeyRound size={13} />
          {modelSettings?.configured ? "已配置" : "未配置"}
        </span>
      </div>
      <form className="settings-form" onSubmit={onSave}>
        {error ? <div className="error-banner">{error}</div> : null}
        <div className="form-section">
          <div className="section-heading">
            <span>DashScope</span>
            <small>{modelSettings?.api_key_mask ? `当前 Key：${modelSettings.api_key_mask}` : "保存后立即生效"}</small>
          </div>
          <label>
            <span>API Key</span>
            <input name="api_key" type="password" placeholder={modelSettings?.configured ? "留空则保留当前 Key" : "请输入 DashScope API Key"} autoComplete="new-password" />
          </label>
          <div className="upload-grid">
            <label>
              <span>模型</span>
              <input name="model" type="text" defaultValue={modelSettings?.model || "qwen3.6-plus"} />
            </label>
            <label>
              <span>Endpoint</span>
              <input name="endpoint" type="text" defaultValue={modelSettings?.endpoint || "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation"} />
            </label>
          </div>
        </div>
        <div className="config-note">
          <p>API Key 会保存在当前部署的 SQLite 数据库中；不要把这个数据库文件公开分享。生产环境建议增加用户登录和密钥加密。</p>
        </div>
        <div className="submit-row">
          <div>
            <span>运行时配置</span>
            <small>用户配置优先于服务器环境变量</small>
          </div>
          <button className="primary-button" type="submit" disabled={isSaving}>
            {isSaving ? <Loader2 className="spin" size={16} /> : <Settings size={16} />}
            <span>{isSaving ? "保存中" : "保存配置"}</span>
          </button>
        </div>
      </form>
    </section>
  );
}

function ModelCallsWorkspace({ call, error }) {
  if (!call) {
    return (
      <section className="result-pane full-height">
        <EmptyState text="暂无模型调用记录" />
      </section>
    );
  }
  return (
    <section className="result-pane full-height">
      <div className="result-header">
        <div>
          <h2>{call.step || "模型调用"}</h2>
          <p>{call.scope} · {call.ref_id || "-"} · {formatTime(call.created_at)}</p>
        </div>
        <div className="token-strip">
          <span>输入 {call.prompt_tokens || 0}</span>
          <span>输出 {call.output_tokens || 0}</span>
          <span>合计 {call.total_tokens || 0}</span>
          <span>{call.latency_ms || 0}ms</span>
        </div>
      </div>
      {error ? <div className="error-banner">{error}</div> : null}
      {call.error_message ? <div className="error-banner">{call.error_message}</div> : null}
      <div className="call-detail">
        <section>
          <h3>模型输入</h3>
          <pre className="result-output json">{pretty(call.input_json)}</pre>
        </section>
        <section>
          <h3>模型输出</h3>
          <pre className="result-output markdown">{call.output_text || "-"}</pre>
        </section>
        <section>
          <h3>原始响应</h3>
          <pre className="result-output json">{pretty(call.response_json)}</pre>
        </section>
      </div>
    </section>
  );
}

function FileField({ name, label, accept, icon, required = true }) {
  return (
    <label className="file-field">
      <span>{label}</span>
      <div className="file-input">
        {icon}
        <input name={name} type="file" accept={accept} required={required} />
        <Upload size={16} />
      </div>
    </label>
  );
}

function ResultHeader({ job, isPublishing, isRetrying, canPublish, canRetry, onPublish, onRetry }) {
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
          <button className="secondary-button" type="button" onClick={onRetry} disabled={!canRetry || isRetrying}>
            {isRetrying ? <Loader2 className="spin" size={16} /> : <RotateCcw size={16} />}
            <span>重试</span>
          </button>
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
  if (!job) return <EmptyState text="暂无任务结果" />;
  if (job.error_message && job.status === "failed") return <pre className="result-output error-output">{job.error_message}</pre>;
  if (!content) return <EmptyState text={runningStatuses.has(job.status) ? "任务执行中" : "当前页暂无内容"} />;
  if (activeTab === "run_log") return <TimelineContent content={content} />;
  if (activeTab === "analysis_markdown") return <MarkdownContent content={content} />;
  return <ScriptJSONContent content={content} activeTab={activeTab} />;
}

function TimelineContent({ content }) {
  const lines = content
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean);
  return (
    <div className="timeline-output">
      {lines.map((line, index) => (
        <div key={`${line}-${index}`} className="timeline-item">
          <span>{String(index + 1).padStart(2, "0")}</span>
          <p>{line}</p>
        </div>
      ))}
    </div>
  );
}

function MarkdownContent({ content }) {
  const blocks = markdownBlocks(content);
  return (
    <div className="markdown-output">
      {blocks.map((block, index) => {
        if (block.type === "heading") {
          const Tag = block.level <= 2 ? "h3" : "h4";
          return <Tag key={index}>{block.text}</Tag>;
        }
        if (block.type === "table") {
          return (
            <div key={index} className="table-wrap">
              <table>
                <tbody>
                  {block.rows.map((row, rowIndex) => (
                    <tr key={rowIndex}>
                      {row.map((cell, cellIndex) => {
                        const Cell = rowIndex === 0 ? "th" : "td";
                        return <Cell key={cellIndex}>{cell}</Cell>;
                      })}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          );
        }
        return <p key={index}>{block.text}</p>;
      })}
    </div>
  );
}

function ScriptJSONContent({ content, activeTab }) {
  const parsed = parseJSONValue(content);
  const scripts = extractScripts(parsed, activeTab);
  return (
    <div className="json-review">
      {scripts.length ? (
        <div className="script-card-list">
          {scripts.map((script, index) => (
            <article key={index} className="script-card">
              <div>
                <span>{String(index + 1).padStart(2, "0")}</span>
                <h3>{script.metadata?.title || script.title || (activeTab === "replica_script_json" ? "复刻脚本" : "裂变脚本")}</h3>
                <p>{script.metadata?.fission_dimension || script.metadata?.industry || "分镜脚本"}</p>
              </div>
              <strong>{Array.isArray(script.storyboards) ? script.storyboards.length : 0} 镜</strong>
            </article>
          ))}
        </div>
      ) : null}
      <details className="raw-json" open={!scripts.length}>
        <summary>原始 JSON</summary>
        <pre className="result-output json">{pretty(content)}</pre>
      </details>
    </div>
  );
}

function EmptyState({ text, compact = false }) {
  return (
    <div className={`empty-state ${compact ? "compact" : ""}`}>
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

function markdownBlocks(content) {
  const lines = content.split("\n");
  const blocks = [];
  for (let index = 0; index < lines.length; index += 1) {
    const line = lines[index].trim();
    if (!line) continue;
    if (line.startsWith("|")) {
      const rows = [];
      while (index < lines.length && lines[index].trim().startsWith("|")) {
        const row = splitMarkdownRow(lines[index]);
        if (!row.every((cell) => /^:?-{2,}:?$/.test(cell))) {
          rows.push(row);
        }
        index += 1;
      }
      index -= 1;
      if (rows.length) blocks.push({ type: "table", rows });
      continue;
    }
    const heading = /^(#{1,4})\s+(.+)$/.exec(line);
    if (heading) {
      blocks.push({ type: "heading", level: heading[1].length, text: heading[2] });
      continue;
    }
    blocks.push({ type: "paragraph", text: line.replace(/^\s*[-*]\s+/, "") });
  }
  return blocks;
}

function splitMarkdownRow(line) {
  return line
    .trim()
    .replace(/^\|/, "")
    .replace(/\|$/, "")
    .split("|")
    .map((cell) => cell.trim() || "-");
}

function parseJSONValue(value) {
  try {
    return JSON.parse(value);
  } catch {
    return null;
  }
}

function extractScripts(parsed, activeTab) {
  if (!parsed) return [];
  if (Array.isArray(parsed)) return parsed;
  if (Array.isArray(parsed.fission_scripts)) return parsed.fission_scripts;
  if (parsed.replica_script) return [parsed.replica_script];
  if (activeTab === "replica_script_json" && Array.isArray(parsed.storyboards)) return [parsed];
  return [];
}

function pretty(value) {
  if (!value) return "-";
  try {
    return JSON.stringify(JSON.parse(value), null, 2);
  } catch {
    return value;
  }
}

function formatTime(value) {
  if (!value) return "-";
  return new Date(value).toLocaleString("zh-CN", { hour12: false });
}
