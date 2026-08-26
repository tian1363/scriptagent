import {
  Activity,
  ChartNoAxesCombined,
  Bot,
  CheckCircle2,
  Clock3,
  FileText,
  History,
  House,
  FolderKanban,
  Search,
  ChevronDown,
  ChevronRight,
  CircleHelp,
  Loader2,
  MessageSquare,
  Package,
  Play,
  Plus,
  RefreshCw,
  RotateCcw,
  Send,
  Settings,
  Sparkles,
  KeyRound,
  Upload,
  Video,
} from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import {
  createJob,
  createCreativeReport,
  createProduct,
  createSpace,

  updateProduct,
  generateVideoPrompts,
  getModelSettings,
  getSpaceObservability,
  getOwnerSession,
  getOwnerOverview,
  loginOwner,
  logoutOwner,
  getChat,
  getJob,
  getProductMarkdown,
  listCreativeReports,
  listProducts,
  listProductAssets,
  listSpaces,
  uploadProductAsset,
  listSkills,
  listChats,
  listJobs,
  listModelCalls,
  retryJob,
  saveModelSettings,
  sendChatMessage,
  sendNewChatMessage,
} from "./api.js";
import agentIcon from "./assets/scriptagent-agent-v2.png";

const resultTabs = [
  ["run_log", "运行日志"],
  ["analysis_markdown", "视频分析"],
  ["replica_script_json", "复刻脚本"],
  ["fission_scripts_json", "裂变脚本"],
  ["video_prompts", "视频提示词"],
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

function hashText(text = "") {
  return Array.from(text).reduce((sum, char) => sum + char.charCodeAt(0), 0);
}

function productCoverVariant(product) {
  return hashText(`${product?.id || ""}${product?.title || ""}`) % 5;
}

function productInitial(title = "") {
  return title.trim().slice(0, 1).toUpperCase() || "S";
}

function buildProductStats(products = [], jobs = []) {
  const result = new Map();
  const safeProducts = Array.isArray(products) ? products : [];
  const safeJobs = Array.isArray(jobs) ? jobs : [];
  safeProducts.forEach((product) => {
    const matchedJobs = safeJobs.filter((job) => job.product_md_name === product.md_name);
    const usableJobs = matchedJobs.filter((job) => job.status !== "failed");
    const scriptCount = usableJobs.reduce((total, job) => total + 1 + Number(job.fission_count || 0), 0);
    const latestJob = matchedJobs.reduce((latest, job) => {
      if (!latest) return job;
      return new Date(job.updated_at).getTime() > new Date(latest.updated_at).getTime() ? job : latest;
    }, null);
    result.set(product.id, {
      taskCount: matchedJobs.length,
      scriptCount,
      latestJob,
      latestAt: latestJob?.updated_at || "",
    });
  });
  return result;
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

const chatQuickTasks = [
  {
    title: "生成裂变方向",
    description: "基于当前产品，先给我 3 个适合短视频的裂变方向，并说明每个方向改哪里。",
  },
  {
    title: "写产品 Markdown",
    description: "帮我把这个产品整理成可用于生成脚本的 Markdown，需要包含卖点、用户、场景、限制和素材备注。",
  },
  {
    title: "优化脚本",
    description: "帮我检查这条脚本哪里可以优化，重点看开头钩子、产品卖点和 CTA。",
  },
  {
    title: "拆解并复刻素材",
    description: "调用 material_replication_analysis skill，分析我上传的图片或视频，拆解设计方式、内容表达和视听结构，并给出视频复刻建议。",
  },
  {
    title: "生成视频提示词",
    description: "调用 seedance_video_prompt_writer skill，把这条分镜脚本转成 Seedance 可用的视频生成提示词。",
  },
];

export function App() {
  const [view, setView] = useState("home");
  const [jobs, setJobs] = useState([]);
  const [selectedJob, setSelectedJob] = useState(null);
  const [activeTab, setActiveTab] = useState("run_log");
  const [videoPromptContent, setVideoPromptContent] = useState("");
  const [isGeneratingVideoPrompts, setIsGeneratingVideoPrompts] = useState(false);
  const [isCreating, setIsCreating] = useState(false);
  const [isRetrying, setIsRetrying] = useState(false);
  const [products, setProducts] = useState([]);
  const [spaces, setSpaces] = useState([]);
  const [isCreatingSpace, setIsCreatingSpace] = useState(false);
  const [skills, setSkills] = useState([]);
  const [selectedProductId, setSelectedProductId] = useState("");
  const [productPreview, setProductPreview] = useState(null);
  const [creativeReports, setCreativeReports] = useState([]);
  const [selectedCreativeReportId, setSelectedCreativeReportId] = useState("");
  const [isLoadingProductPreview, setIsLoadingProductPreview] = useState(false);
  const [isLoadingCreativeReports, setIsLoadingCreativeReports] = useState(false);
  const [isCreatingCreativeReport, setIsCreatingCreativeReport] = useState(false);
  const [isCreatingProduct, setIsCreatingProduct] = useState(false);
  const [isSavingProduct, setIsSavingProduct] = useState(false);
  const [jobInitialRequirement, setJobInitialRequirement] = useState("");
  const [modelSettings, setModelSettings] = useState(null);
  const [isSavingSettings, setIsSavingSettings] = useState(false);
  const [showDebugPanel, setShowDebugPanel] = useState(() => window.localStorage.getItem("scriptagent:debug-panel") === "true");
  const [ownerSession, setOwnerSession] = useState({ configured: false, authenticated: false });
  const [ownerOverview, setOwnerOverview] = useState(null);
  const [isOwnerLoading, setIsOwnerLoading] = useState(false);

  const [chats, setChats] = useState([]);
  const [selectedChat, setSelectedChat] = useState(null);
  const [chatDraft, setChatDraft] = useState("");
  const [chatAttachment, setChatAttachment] = useState(null);
  const [chatProductId, setChatProductId] = useState("");
  const [optimisticChatMessages, setOptimisticChatMessages] = useState(null);
  const [typingMessage, setTypingMessage] = useState(null);
  const [chatCitations, setChatCitations] = useState([]);
  const [chatAgentSteps, setChatAgentSteps] = useState([]);
  const [isChatThinking, setIsChatThinking] = useState(false);
  const [isSending, setIsSending] = useState(false);

  const [modelCalls, setModelCalls] = useState([]);
  const [selectedCallId, setSelectedCallId] = useState("");
  const [error, setError] = useState("");

  async function refreshJobs(nextSelectedId) {
    const items = await listJobs();
    setJobs(Array.isArray(items) ? items : []);
    const targetId = nextSelectedId || selectedJob?.id || items[0]?.id;
    if (targetId) {
      const current = await getJob(targetId);
      setSelectedJob(current);
    }
  }

  async function refreshChats(nextSelectedId) {
    const items = await listChats();
    setChats(Array.isArray(items) ? items : []);
    const targetId = nextSelectedId || selectedChat?.conversation?.id || items[0]?.id;
    if (targetId) {
      setSelectedChat(await getChat(targetId));
    }
  }

  async function refreshModelCalls() {
    const items = await listModelCalls({ limit: 1000 });
    setModelCalls(items);
    if (!selectedCallId && items[0]) {
      setSelectedCallId(items[0].id);
    }
  }

  async function refreshProducts(nextSelectedId) {
    const items = await listProducts();
    setProducts(Array.isArray(items) ? items : []);
    if (nextSelectedId || (!selectedProductId && items[0])) {
      setSelectedProductId(nextSelectedId || items[0]?.id || "");
    }
  }
  async function refreshSpaces() { const items = await listSpaces(); setSpaces(Array.isArray(items) ? items : []); }

  async function refreshSkills() {
    setSkills(await listSkills());
  }

  async function refreshModelSettings() {
    setModelSettings(await getModelSettings());
  }

  async function refreshCurrent() {
    setError("");
    if (view === "jobs") await refreshJobs();
    if (view === "chat") await refreshChats();
    if (view === "calls" && showDebugPanel) await Promise.all([refreshModelCalls(), refreshJobs(), refreshSpaces()]);
    if (view === "products") await refreshProducts();
    if (view === "spaces") await refreshSpaces();
    if (view === "history") await Promise.all([refreshJobs(), refreshChats()]);
    if (view === "settings") await refreshModelSettings();
    if (view === "admin" && ownerSession.authenticated) setOwnerOverview(await getOwnerOverview());
  }

  async function handleOwnerLogin(event) {
    event.preventDefault(); const form = new FormData(event.currentTarget); setError(""); setIsOwnerLoading(true);
    try { const session = await loginOwner(String(form.get("username") || ""), String(form.get("password") || "")); setOwnerSession(session); setOwnerOverview(await getOwnerOverview()); setView("admin"); event.currentTarget.reset(); }
    catch (err) { setError(err.message); } finally { setIsOwnerLoading(false); }
  }

  async function handleOwnerLogout() {
    setIsOwnerLoading(true); setError("");
    try { const session = await logoutOwner(); setOwnerSession(session); setOwnerOverview(null); setView("settings"); }
    catch (err) { setError(err.message); } finally { setIsOwnerLoading(false); }
  }

  useEffect(() => {
    refreshJobs().catch((err) => setError(err.message));
    refreshChats().catch(() => {});
    if (showDebugPanel) refreshModelCalls().catch(() => {});
    refreshProducts().catch(() => {});
    refreshSpaces().catch(() => {});
    refreshSkills().catch(() => {});
    refreshModelSettings().catch(() => {});
    getOwnerSession().then(setOwnerSession).catch(() => {});
  }, []);

  useEffect(() => {
    window.localStorage.setItem("scriptagent:debug-panel", showDebugPanel ? "true" : "false");
    if (!showDebugPanel && view === "calls") {
      setView("settings");
    }
    if (showDebugPanel) {
      refreshModelCalls().catch(() => {});
    }
  }, [showDebugPanel]);

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
    if (activeTab !== "video_prompts") return undefined;
    if (!selectedJob?.id) return undefined;
    let cancelled = false;
    setError("");
    setIsGeneratingVideoPrompts(true);
    generateVideoPrompts(selectedJob.id)
      .then((result) => {
        if (!cancelled) setVideoPromptContent(result.content || "");
      })
      .catch((err) => {
        if (!cancelled) {
          setVideoPromptContent("");
          setError(err.message);
        }
      })
      .finally(() => {
        if (!cancelled) setIsGeneratingVideoPrompts(false);
      });
    return () => {
      cancelled = true;
    };
  }, [activeTab, selectedJob?.id]);

  useEffect(() => {
    if (view !== "products" || !selectedProductId) {
      setProductPreview(null);
      setCreativeReports([]);
      setSelectedCreativeReportId("");
      return undefined;
    }
    let cancelled = false;
    setIsLoadingProductPreview(true);
    setIsLoadingCreativeReports(true);
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
    listCreativeReports(selectedProductId)
      .then((reports) => {
        if (!cancelled) {
          setCreativeReports(reports);
          setSelectedCreativeReportId(reports[0]?.id || "");
        }
      })
      .catch(() => {
        if (!cancelled) setCreativeReports([]);
      })
      .finally(() => {
        if (!cancelled) setIsLoadingCreativeReports(false);
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
      setVideoPromptContent("");
      setJobInitialRequirement("");
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
    setVideoPromptContent("");
    setSelectedJob(await getJob(id));
  }

  async function handleRetry() {
    if (!selectedJob) return;
    setError("");
    setIsRetrying(true);
    try {
      const retried = await retryJob(selectedJob.id);
      setActiveTab("run_log");
      setVideoPromptContent("");
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
    setChatCitations([]);
    setChatAgentSteps([]);
    setIsChatThinking(false);
    setChatAttachment(null);
    setSelectedChat(await getChat(id));
  }

  function handleNewChat() {
    setError("");
    setSelectedChat(null);
    setOptimisticChatMessages(null);
    setTypingMessage(null);
    setChatCitations([]);
    setChatAgentSteps([]);
    setIsChatThinking(false);
    setChatAttachment(null);
  }

  async function sendChatToAgent(content, productID, conversationID = "", attachment = null) {
    content = content.trim();
    if (!content && !attachment) return;
    setError("");
    setIsSending(true);
    setIsChatThinking(true);
    setTypingMessage(null);
    setChatCitations([]);
    setChatAgentSteps([]);
    const displayContent = [content || "请分析我上传的素材。", attachment ? `附件：${attachment.name}` : ""].filter(Boolean).join("\n\n");
    const tempUserMessage = {
      id: `temp-user-${Date.now()}`,
      role: "user",
      content: displayContent,
      created_at: new Date().toISOString(),
    };
    const continuingCurrentChat = Boolean(conversationID && selectedChat?.conversation?.id === conversationID);
    const baseMessages = continuingCurrentChat ? selectedChat?.messages || [] : [];
    setOptimisticChatMessages([...baseMessages, tempUserMessage]);
    setChatDraft("");
    setChatAttachment(null);
    try {
      const thread = conversationID
        ? await sendChatMessage(conversationID, content, productID, attachment)
        : await sendNewChatMessage(content, productID, attachment);
      const assistantMessage = lastAssistantMessage(thread.messages);
      setSelectedChat(thread);
      setChatCitations(thread.citations || []);
      setChatAgentSteps(thread.agent_steps || []);
      if (assistantMessage) {
        setOptimisticChatMessages(thread.messages.filter((message) => message.id !== assistantMessage.id));
        setTypingMessage({ ...assistantMessage, visible: "" });
      } else {
        setOptimisticChatMessages(null);
      }
      await refreshChats(thread.conversation.id);
      if (showDebugPanel) {
        await refreshModelCalls();
      }
    } catch (err) {
      setError(err.message);
      setChatAttachment(attachment);
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

  async function handleSendChat(event) {
    event.preventDefault();
    await sendChatToAgent(chatDraft, chatProductId, selectedChat?.conversation?.id || "", chatAttachment);
  }

  async function handleStartAgentChat(productID, goal) {
    setView("chat");
    setSelectedChat(null);
    setChatProductId(productID || "");
    await sendChatToAgent(goal, productID || "");
  }

  async function handleStartSpaceAgent(space) {
    if (!space) return;
    const spaceContext = [
      `继续推进创作空间「${space.title}」。`,
      space.summary ? `空间目标：${space.summary}` : "",
      space.agent_brief ? `长期要求：${space.agent_brief}` : "",
      "请结合这些长期上下文判断当前最合适的下一步；需要资料时主动读取，需要补充信息时直接提问。不要自动启动固定工作流。",
    ].filter(Boolean).join("\n");
    await handleStartAgentChat(space.product_id || "", spaceContext);
  }

  async function handleCreateProduct(eventOrForm) {
    const isFormData = Boolean(eventOrForm && typeof eventOrForm.append === "function" && typeof eventOrForm.get === "function");
    if (!isFormData) eventOrForm.preventDefault();
    const formEl = isFormData ? null : eventOrForm.currentTarget;
    const form = isFormData ? eventOrForm : new FormData(formEl);
    setError("");
    setIsCreatingProduct(true);
    try {
      const product = await createProduct(form);
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
        provider: String(form.get("provider") || "dashscope"),
        endpoint: String(form.get("endpoint") || ""),
        model: String(form.get("model") || ""),
      });
      setModelSettings(next);
    } catch (err) {
      setError(err.message);
    } finally {
      setIsSavingSettings(false);
    }
  }

  const visibleContent = activeTab === "video_prompts" ? videoPromptContent : selectedJob?.[activeTab] || "";
  const canRetry = selectedJob && !runningStatuses.has(selectedJob.status);
  const selectedCall = modelCalls.find((call) => call.id === selectedCallId) || modelCalls[0];
  const productStats = useMemo(() => buildProductStats(products, jobs), [products, jobs]);

  function handleStartProductJob(productId, requirement = "", spaceId = "") {
    setSelectedProductId(productId);
    setJobInitialRequirement(requirement);
    window.sessionStorage.setItem("scriptagent:space-id", spaceId);
    setView("jobs");
  }

  async function handleCreateCreativeReport(event) {
    event.preventDefault();
    const selectedProduct = products.find((product) => product.id === selectedProductId);
    if (!selectedProduct) return;
    const form = new FormData(event.currentTarget);
    setError("");
    setIsCreatingCreativeReport(true);
    try {
      const report = await createCreativeReport(selectedProduct.id, {
        source_type: "dataeye",
        dataeye_url: form.get("dataeye_url") || "",
        dataeye_id: form.get("dataeye_id") || "",
        product_name: form.get("product_name") || selectedProduct.title,
        date_range: form.get("date_range") || "近 30 天",
        media: form.get("media") || "",
        country: form.get("country") || "",
        sort_metric: form.get("sort_metric") || "热度/曝光/播放优先",
        sample_count: Number(form.get("sample_count") || 50),
        requirement: form.get("requirement") || "",
        material_note: form.get("material_note") || "",
      });
      const reports = await listCreativeReports(selectedProduct.id);
      setCreativeReports(reports);
      setSelectedCreativeReportId(report.id);
    } catch (err) {
      setError(err.message);
    } finally {
      setIsCreatingCreativeReport(false);
    }
  }

  async function handleUpdateProduct(id, title, content) {
    setError(""); setIsSavingProduct(true);
    try { await updateProduct(id, { title, content }); await refreshProducts(id); setProductPreview(await getProductMarkdown(id)); }
    catch (err) { setError(err.message); throw err; }
    finally { setIsSavingProduct(false); }
  }
  async function handleCreateSpace(event) {
    event.preventDefault(); const form = new FormData(event.currentTarget); setError(""); setIsCreatingSpace(true);
    const formEl = event.currentTarget;
    try { await createSpace({ title:String(form.get("title")||""), summary:String(form.get("summary")||""), product_id:String(form.get("product_id")||""), agent_brief:String(form.get("agent_brief")||"") }); formEl.reset(); await refreshSpaces(); }
    catch (err) { setError(err.message); } finally { setIsCreatingSpace(false); }
  }

  function handleReportToJob(report) {
    if (!report) return;
    const requirement = [
      "基于产品创意策略报告生成裂变脚本。",
      report.report_summary ? `报告摘要：${report.report_summary}` : "",
      "优先沿用报告中的创意方向、钩子、卖点呈现和验收指标；每条裂变脚本仍只能选择 1 个裂变元素。",
    ]
      .filter(Boolean)
      .join("\n");
    handleStartProductJob(report.product_id, requirement);
  }

  return (
    <div className="app-shell agent-app-shell">
      <AppSidebar view={view} onChange={setView} icon={agentIcon} showDebugPanel={showDebugPanel} ownerAuthenticated={ownerSession.authenticated} />
      <div className="agent-frame">
        <header className="workspace-topbar">
          <div><span className="workspace-kicker">ScriptAgent</span><strong>{pageTitle(view)}</strong></div>
          <button className="icon-button" type="button" onClick={() => refreshCurrent().catch((err) => setError(err.message))} title="刷新"><RefreshCw size={16} /></button>
        </header>
      <main className={`workspace ${view === "products" || view === "calls" || view === "admin" ? "workspace-home" : ""}`}>
        {view === "jobs" ? (
          <JobsSidebar jobs={jobs} selectedJob={selectedJob} onSelect={handleSelectJob} />
        ) : null}
        {view === "chat" ? (
          <ChatSidebar chats={chats} selectedChat={selectedChat} onSelect={handleSelectChat} onNew={handleNewChat} />
        ) : null}
        {view === "settings" ? <SettingsSidebar modelSettings={modelSettings} /> : null}

        <section className={`main-pane ${view === "products" || view === "calls" || view === "admin" ? "main-pane-home" : ""}`}>
          {view === "home" ? <AgentStart products={products} jobs={jobs} spaces={spaces} isSending={isSending} onSend={handleStartAgentChat} onSpaces={() => setView("spaces")} onHistory={() => setView("history")} /> : null}
          {view === "history" ? <HistoryWorkspace jobs={jobs} chats={chats} products={products} onJob={(id) => handleSelectJob(id).then(() => setView("jobs"))} onChat={(id) => handleSelectChat(id).then(() => setView("chat"))} /> : null}
          {view === "spaces" ? <SpacesWorkspace spaces={spaces} products={products} jobs={jobs} isCreating={isCreatingSpace} isSending={isSending} error={error} onCreate={handleCreateSpace} onStart={handleStartSpaceAgent} /> : null}
          {view === "jobs" ? (
            <JobsWorkspace
              selectedJob={selectedJob}
              activeTab={activeTab}
              visibleContent={visibleContent}
              isGeneratingVideoPrompts={isGeneratingVideoPrompts}
              isCreating={isCreating}
              isRetrying={isRetrying}
              canRetry={canRetry}
              products={products}
              initialProductId={selectedProductId}
              initialRequirement={jobInitialRequirement}
              error={error}
              onCreate={handleCreate}
              onRetry={handleRetry}
              onTab={setActiveTab}
            />
          ) : null}
          {view === "products" ? (
            <ProductKnowledgeWorkspace
              products={products}
              selectedProductId={selectedProductId}
              productPreview={productPreview}
              creativeReports={creativeReports}
              selectedCreativeReportId={selectedCreativeReportId}
              isLoadingProductPreview={isLoadingProductPreview}
              isLoadingCreativeReports={isLoadingCreativeReports}
              isCreatingCreativeReport={isCreatingCreativeReport}
              isCreatingProduct={isCreatingProduct}
              isSavingProduct={isSavingProduct}
              productStats={productStats}
              error={error}
              onSelect={setSelectedProductId}
              onReportSelect={setSelectedCreativeReportId}
              onStartJob={handleStartProductJob}
              onCreateCreativeReport={handleCreateCreativeReport}
              onReportToJob={handleReportToJob}
              onCreate={handleCreateProduct}
              onUpdate={handleUpdateProduct}
            />
          ) : null}
          {view === "chat" ? (
            <ChatWorkspace
              thread={selectedChat}
              optimisticMessages={optimisticChatMessages}
              typingMessage={typingMessage}
              citations={chatCitations}
              agentSteps={chatAgentSteps}
              isThinking={isChatThinking}
              draft={chatDraft}
              products={products}
              skills={skills}
              selectedProductId={chatProductId}
              isSending={isSending}
              error={error}
              attachment={chatAttachment}
              onDraft={setChatDraft}
              onAttachment={setChatAttachment}
              onProduct={setChatProductId}
              onSend={handleSendChat}
            />
          ) : null}
          {view === "calls" && showDebugPanel ? <ModelCallsWorkspace calls={modelCalls} jobs={jobs} spaces={spaces} error={error} /> : null}
          {view === "admin" && ownerSession.authenticated ? <OwnerDashboard overview={ownerOverview} isLoading={isOwnerLoading} error={error} onRefresh={() => refreshCurrent().catch((err) => setError(err.message))} onLogout={handleOwnerLogout} /> : null}
          {view === "settings" ? (
            <SettingsWorkspace
              modelSettings={modelSettings}
              showDebugPanel={showDebugPanel}
              isSaving={isSavingSettings}
              error={error}
              onDebugPanel={setShowDebugPanel}
              onSave={handleSaveModelSettings}
              ownerSession={ownerSession}
              isOwnerLoading={isOwnerLoading}
              onOwnerLogin={handleOwnerLogin}
            />
          ) : null}
        </section>
      </main></div>
    </div>
  );
}

function pageTitle(view) {
  return ({ home: "开始创作", history: "历史记录", spaces: "创作空间", products: "产品资料", jobs: "执行任务", chat: "对话", settings: "设置", calls: "调试台", admin: "运营后台" })[view] || "ScriptAgent";
}

function AppSidebar({ view, onChange, icon, showDebugPanel, ownerAuthenticated }) {
  const items = [["home", "开始", House], ["history", "历史", History], ["spaces", "创作空间", FolderKanban], ["products", "产品资料", Package]];
  return <aside className="app-sidebar"><button className="agent-brand" type="button" onClick={() => onChange("home")}><img src={icon} alt="ScriptAgent"/><span>ScriptAgent</span></button><nav>{items.map(([id, label, Icon]) => <button key={id} className={view === id ? "active" : ""} type="button" onClick={() => onChange(id)}><Icon size={18}/><span>{label}</span></button>)}</nav><div className="sidebar-bottom">{ownerAuthenticated ? <button className={view === "admin" ? "active" : ""} type="button" onClick={() => onChange("admin")}><ChartNoAxesCombined size={18}/><span>运营后台</span></button> : null}{showDebugPanel ? <button className={view === "calls" ? "active" : ""} type="button" onClick={() => onChange("calls")}><Activity size={18}/><span>运行调试</span></button> : null}<button className={view === "settings" ? "active" : ""} type="button" onClick={() => onChange("settings")}><Settings size={18}/><span>设置</span></button></div></aside>;
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
  const [selectedProductID, setSelectedProductID] = useState(props.initialProductId || "");
  const [requirementDraft, setRequirementDraft] = useState(props.initialRequirement || "");

  useEffect(() => {
    setSelectedProductID(props.initialProductId || "");
  }, [props.initialProductId]);

  useEffect(() => {
    setRequirementDraft(props.initialRequirement || "");
  }, [props.initialRequirement]);

  function handleFissionCountChange(event) {
    const next = Number.parseInt(event.target.value, 10);
    setFissionCount(Number.isFinite(next) ? Math.min(20, Math.max(1, next)) : 1);
  }

  return (
    <>
      <section className="composer">
        <div>
          <h1>创建脚本任务</h1>
          <p>先确认产品资产，再上传参考视频生成复刻脚本与裂变脚本。</p>
        </div>
        <form className="task-form" onSubmit={props.onCreate}>
          <input type="hidden" name="space_id" value={window.sessionStorage.getItem("scriptagent:space-id") || ""} />
          <div className="form-section">
            <div className="section-heading">
              <span>1. 产品资产</span>
              <small>产品信息决定脚本是否有卖点和细节</small>
            </div>
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
            <div className="settings-panel">
              <div className="section-heading compact-heading">
                <span>2. 参考视频</span>
                <small>上传要复刻结构和节奏的视频</small>
              </div>
              <label>
                <span>任务标题</span>
                <input name="title" type="text" placeholder="默认使用产品 Markdown 文件名" />
              </label>
              <FileField name="video" label="参考视频" accept="video/mp4,video/quicktime,video/webm" icon={<Video size={16} />} />
            </div>
            <div className="settings-panel">
              <div className="section-heading compact-heading">
                <span>3. 生成设置</span>
                <small>不确定时保持默认</small>
              </div>
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

          <div className="form-section">
            <div className="section-heading">
              <span>4. 补充要求</span>
              <small>可写平台、风格、禁用表达或目标人群</small>
            </div>
            <label className="requirement-field">
              <span>补充要求</span>
              <textarea
                name="requirement"
                rows="4"
                value={requirementDraft}
                onChange={(event) => setRequirementDraft(event.target.value)}
                placeholder="例如：面向 TikTok，节奏更快，避免夸大功效。"
              />
            </label>
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
        <ResultContent
          job={props.selectedJob}
          content={props.visibleContent}
          activeTab={props.activeTab}
          isGeneratingVideoPrompts={props.isGeneratingVideoPrompts}
        />
      </section>
    </>
  );
}

function ChatWorkspace({ thread, optimisticMessages, typingMessage, citations, agentSteps, isThinking, draft, products, skills, selectedProductId, isSending, error, attachment, onDraft, onAttachment, onProduct, onSend }) {
  const messages = optimisticMessages || thread?.messages || [];
  const selectedProduct = products.find((product) => product.id === selectedProductId);
  const messagesEndRef = useRef(null);
  const [showSkillMenu, setShowSkillMenu] = useState(false);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ block: "end" });
  }, [messages.length, isThinking, typingMessage?.visible]);

  function handleSkill(skill) {
    onDraft(skill.invocation_prompt || `调用 ${skill.name} skill。`);
    setShowSkillMenu(false);
  }

  return (
    <section className="chat-pane">
      <div className="result-header">
        <div>
          <h2>{thread?.conversation?.title || "通用对话"}</h2>
          <p>用于讨论脚本策略、裂变方向、发布问题和一般创作问题，可调用产品库资料。</p>
        </div>
        <span className="status-pill neutral">
          <Bot size={13} />
          ReAct 工作流
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
            {agentSteps?.length ? <AgentStepsPanel steps={agentSteps} /> : null}
            {citations?.length ? <CitationPanel citations={citations} /> : null}
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
          <ChatTaskStarter onSelect={onDraft} />
        )}
      </div>
      <form className="chat-form" onSubmit={onSend}>
        {error ? <div className="error-banner">{error}</div> : null}
        {showSkillMenu && skills?.length ? <SkillCommandMenu skills={skills} onSelect={handleSkill} onClose={() => setShowSkillMenu(false)} /> : null}
        {attachment ? <div className="attachment-chip"><span>{attachment.name}</span><button type="button" onClick={() => onAttachment(null)}>移除</button></div> : null}
        <textarea value={draft} onChange={(event) => onDraft(event.target.value)} rows="3" placeholder="发消息或创建任务… / 使用技能，添加素材" />
        <div className="chat-composer-toolbar">
          <div className="composer-tools">
            <label className="composer-add" title="添加图片或视频素材">
              <Plus size={22} />
              <input type="file" accept="image/png,image/jpeg,image/webp,image/gif,video/mp4,video/quicktime,video/webm" onChange={(event) => onAttachment(event.target.files?.[0] || null)} />
            </label>
            <button className={`composer-tool ${showSkillMenu ? "active" : ""}`} type="button" onClick={() => setShowSkillMenu((value) => !value)}>
              <Sparkles size={16} /><span>技能</span><ChevronRight size={14} />
            </button>
            <label className={`composer-tool composer-connector ${attachment ? "active" : ""}`} title="添加图片或视频素材">
              <Upload size={15} /><span>{attachment ? "已添加素材" : "素材"}</span><ChevronRight size={14} />
              <input type="file" accept="image/png,image/jpeg,image/webp,image/gif,video/mp4,video/quicktime,video/webm" onChange={(event) => onAttachment(event.target.files?.[0] || null)} />
            </label>
          </div>
          <button className="composer-send" type="submit" disabled={isSending || (!draft.trim() && !attachment)} aria-label={isSending ? "发送中" : "发送"}>
            {isSending ? <Loader2 className="spin" size={18} /> : <Send size={18} />}
          </button>
        </div>
      </form>
    </section>
  );
}

function ChatTaskStarter({ onSelect }) {
  return (
    <section className="chat-starter">
      <div>
        <h3>从一个具体任务开始</h3>
        <p>选一个常见任务，我会把问题放进输入框，你可以补充产品、平台或素材限制。</p>
      </div>
      <div className="starter-list">
        {chatQuickTasks.map((task) => (
          <button key={task.title} type="button" onClick={() => onSelect(task.description)}>
            <strong>{task.title}</strong>
            <span>{task.description}</span>
          </button>
        ))}
      </div>
    </section>
  );
}

function SkillCommandMenu({ skills, onSelect, onClose }) {
  return (
    <section className="skill-command-menu">
      <div className="skill-command-head">
        <span>技能</span>
        <small>{skills.length}</small>
        <button type="button" onClick={onClose}>关闭</button>
      </div>
      <div className="skill-command-list">
        {skills.map((skill) => (
          <button key={skill.name} type="button" onClick={() => onSelect(skill)}>
            <span className="skill-command-icon"><Sparkles size={17} /></span>
            <strong>{skill.title}</strong>
            <small>{skill.description}</small>
            <em>{skill.category}</em>
          </button>
        ))}
      </div>
    </section>
  );
}

function AgentStepsPanel({ steps }) {
  return (
    <section className="agent-steps-panel">
      <div className="section-heading">
        <span>本轮 Agent 步骤</span>
        <small>{steps.length} 步</small>
      </div>
      <div className="agent-step-list">
        {steps.map((step) => (
          <details key={`${step.index}-${step.kind}-${step.tool || "final"}`} className="agent-step-card">
            <summary>
              <span>{String(step.index).padStart(2, "0")}</span>
              <strong>{step.kind === "tool" ? step.tool : "final"}</strong>
              <small>{step.reason || (step.kind === "tool" ? "调用工具" : "最终回答")}</small>
            </summary>
            {step.input ? (
              <div>
                <b>输入</b>
                <pre>{pretty(step.input)}</pre>
              </div>
            ) : null}
            {step.observation ? (
              <div>
                <b>{step.error ? "错误" : "观察"}</b>
                <pre>{step.observation}</pre>
              </div>
            ) : null}
          </details>
        ))}
      </div>
    </section>
  );
}

function CitationPanel({ citations }) {
  return (
    <section className="citation-panel">
      <div className="section-heading">
        <span>本轮引用</span>
        <small>{citations.length} 个产品章节</small>
      </div>
      <div className="citation-list">
        {citations.map((citation, index) => (
          <article key={`${citation.chunk_id || citation.chunk_index}-${index}`} className="citation-card">
            <div>
              <strong>{citation.heading || "产品资料"}</strong>
              <span>{citation.product_name} · {citation.source === "embedding" ? `相似度 ${Number(citation.score || 0).toFixed(3)}` : citation.source}</span>
            </div>
            <p>{citation.snippet}</p>
          </article>
        ))}
      </div>
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
  const [directions, setDirections] = useState(() => rows.map((index) => defaultFissionDirection(index)));

  useEffect(() => {
    setDirections((current) => rows.map((index) => current[index] || defaultFissionDirection(index)));
  }, [count]);

  function updateDirection(index, value) {
    setDirections((current) => {
      const next = [...current];
      next[index] = value;
      return next;
    });
  }

  return (
    <div className="direction-panel">
      <div className="direction-heading">
        <span>5. 裂变方向</span>
        <small>每一行对应一条脚本，只使用这一行选择的 1 个裂变元素</small>
      </div>
      <div className="direction-list">
        {rows.map((index) => (
          <section key={index} className="direction-card-row">
            <input type="hidden" name="fission_directions" value={directions[index] || defaultFissionDirection(index)} />
            <div className="direction-card-title">
              <span className="direction-index">{String(index + 1).padStart(2, "0")}</span>
              <div>
                <strong>第 {index + 1} 条裂变脚本</strong>
                <small>{directions[index] || defaultFissionDirection(index)}</small>
              </div>
            </div>
            <div className="direction-option-board">
              {fissionDirectionGroups.map((group) => (
                <div key={group.layer} className="direction-option-group">
                  <span>{group.layer}</span>
                  <div>
                    {group.items.map((item) => {
                      const value = `${group.layer}-${item}`;
                      const active = (directions[index] || defaultFissionDirection(index)) === value;
                      return (
                        <button key={value} className={active ? "active" : ""} type="button" onClick={() => updateDirection(index, value)}>
                          {item}
                        </button>
                      );
                    })}
                  </div>
                </div>
              ))}
            </div>
          </section>
        ))}
      </div>
    </div>
  );
}

function AgentStart({ products, jobs, spaces, isSending, onSend, onSpaces, onHistory }) {
  const [goal, setGoal] = useState("");
  const [productID, setProductID] = useState(products[0]?.id || "");
  const safeJobs = Array.isArray(jobs) ? jobs : [];
  const safeSpaces = Array.isArray(spaces) ? spaces : [];
  const active = safeJobs.filter((job) => runningStatuses.has(job.status));
  return <section className="agent-start">
    <div className="agent-intro"><span className="eyebrow">ScriptAgent</span><h1>今天想完成什么？</h1><p>说出目标，助手会读取资料、规划步骤并执行。</p></div>
    <div className="agent-goal-card"><textarea value={goal} onChange={(event) => setGoal(event.target.value)} placeholder="例如：为夏季活动生成 8 条短视频脚本，优先测试前三秒钩子" /><div><select value={productID} onChange={(event) => setProductID(event.target.value)}><option value="">暂不选择资料</option>{products.map((product)=><option key={product.id} value={product.id}>{product.title}</option>)}</select><button className="primary-button" type="button" disabled={!goal.trim() || isSending} onClick={() => onSend(productID, goal)}><Play size={15}/>{isSending ? "Agent 思考中" : "发送给 Agent"}</button></div></div>
    <div className="agent-start-grid"><section><h2>正在进行</h2>{active.length ? active.map((job)=><button className="agent-list-row" key={job.id} onClick={onHistory}><span className="task-state-dot running"/><strong>{job.title}</strong><small>{statusLabel(job.status)}</small></button>) : <p>当前没有执行中的任务。</p>}</section><section><div className="section-heading"><span>最近空间</span><button className="mini-button" type="button" onClick={onSpaces}>查看全部</button></div>{safeSpaces.length ? safeSpaces.slice(0,3).map((space)=><button className="agent-list-row" key={space.id} onClick={onSpaces}><FolderKanban size={16}/><strong>{space.title}</strong><small>{space.summary || "继续创作"}</small></button>) : <button className="agent-list-row" onClick={onSpaces}><FolderKanban size={16}/><strong>创建第一个空间</strong><small>把目标和历史放在一起</small></button>}</section></div>
  </section>;
}

function HistoryWorkspace({ jobs, chats, products, onJob, onChat }) {
  const [query,setQuery]=useState(""); const [kind,setKind]=useState("all");
  const safeJobs=Array.isArray(jobs)?jobs:[],safeChats=Array.isArray(chats)?chats:[],safeProducts=Array.isArray(products)?products:[];
  const items=[...safeJobs.map((job)=>({id:job.id,kind:"job",title:job.title,detail:`${statusLabel(job.status)} · ${safeProducts.find((p)=>p.md_name===job.product_md_name)?.title || job.product_md_name}`,at:job.updated_at})),...safeChats.map((chat)=>({id:chat.id,kind:"chat",title:chat.title,detail:chat.summary||"和 ScriptAgent 的对话",at:chat.updated_at}))].filter((item)=>(kind==="all"||item.kind===kind)&&`${item.title} ${item.detail}`.toLowerCase().includes(query.toLowerCase())).sort((a,b)=>new Date(b.at)-new Date(a.at));
  return <section className="history-workspace"><span className="eyebrow">过去的工作都在这里</span><h1>继续，不必重新开始</h1><p>对话和执行任务放在一起；创作空间中的版本仍会保留在项目里。</p><div className="history-tools"><label><Search size={15}/><input value={query} onChange={(event)=>setQuery(event.target.value)} placeholder="搜索标题或产品"/></label><div>{[["all","全部"],["job","执行任务"],["chat","对话"]].map(([key,label])=><button key={key} className={kind===key?"active":""} onClick={()=>setKind(key)}>{label}</button>)}</div></div>{items.length?<div className="history-list">{items.map((item)=><button key={`${item.kind}-${item.id}`} onClick={()=>item.kind==="job"?onJob(item.id):onChat(item.id)}><span>{item.kind==="job"?<Bot size={16}/>:<MessageSquare size={16}/>}</span><div><strong>{item.title}</strong><small>{item.detail}</small></div><time>{formatTime(item.at)}</time><ChevronRight size={16}/></button>)}</div>:<EmptyState text="还没有历史记录"/>}</section>;
}

function SpacesWorkspace({ spaces, products, jobs, isCreating, isSending, error, onCreate, onStart }) {
  return <section className="spaces-workspace"><div className="spaces-head"><div><span className="eyebrow">长期创作</span><h1>创作空间</h1><p>空间保存目标、长期要求和产品上下文；Agent 会先理解任务，再决定下一步。</p></div></div><div className="spaces-grid"><section className="spaces-list">{spaces.length?spaces.map((space)=>{const count=jobs.filter((job)=>job.space_id===space.id).length;const product=products.find((item)=>item.id===space.product_id);return <article key={space.id}><FolderKanban size={18}/><div><strong>{space.title}</strong><p>{space.summary||"继续上次的创作"}</p><small>{product?.title||"暂未关联资料"} · {count} 次执行</small></div><button className="secondary-button" type="button" disabled={isSending} onClick={()=>onStart(space)}>{isSending?"Agent 思考中":"和 Agent 继续"}</button></article>}):<EmptyState text="还没有空间，创建一个长期创作项目。"/>}</section><aside className="space-create"><span className="eyebrow">新空间</span><h2>开始一个长期创作</h2><p>先保存稳定目标，之后通过 Agent 对话逐步推进，不会自动套用固定工作流。</p><form onSubmit={onCreate}><input name="title" placeholder="例如：夏季活动脚本" required/><select name="product_id" defaultValue=""><option value="">暂不选择产品资料</option>{products.map((product)=><option key={product.id} value={product.id}>{product.title}</option>)}</select><textarea name="summary" placeholder="长期目标：最终希望达成什么？" required/><textarea name="agent_brief" placeholder="长期要求：风格、受众、边界和偏好（可选）"/><button className="primary-button" disabled={isCreating}>{isCreating?"创建中":"创建空间"}</button></form>{error?<div className="error-banner">{error}</div>:null}</aside></div></section>;
}

function ProductKnowledgeWorkspace({ products, selectedProductId, productPreview, isLoadingProductPreview, isCreatingProduct, isSavingProduct, error, onSelect, onStartJob, onCreate, onUpdate }) {
  const [query, setQuery] = useState("");
  const [editing, setEditing] = useState(false);
  const [isNew, setIsNew] = useState(false);
  const [assets, setAssets] = useState([]);
  const [isUploadingAsset, setIsUploadingAsset] = useState(false);
  const [title, setTitle] = useState("");
  const [content, setContent] = useState("");
  const selected = products.find((item) => item.id === selectedProductId) || products[0];
  const visible = products.filter((item) => item.title.toLowerCase().includes(query.toLowerCase()));
  useEffect(() => { if (!editing) { setTitle(selected?.title || ""); setContent(productPreview?.content || ""); } }, [selected?.id, productPreview?.content, editing]);
  useEffect(() => { if (!selected?.id) { setAssets([]); return; } listProductAssets(selected.id).then((items)=>setAssets(Array.isArray(items)?items:[])).catch(()=>setAssets([])); }, [selected?.id]);
  async function save(event) { event.preventDefault(); if (selected) { await onUpdate(selected.id, title, content); setEditing(false); } }
  async function addAsset(event) { const file = event.target.files?.[0]; if (!file || !selected) return; setIsUploadingAsset(true); try { const asset=await uploadProductAsset(selected.id,file); setAssets((items)=>[asset,...items]); } catch (err) { window.alert(err.message); } finally { event.target.value=""; setIsUploadingAsset(false); } }
  return <section className="knowledge-workspace"><header><div><span className="eyebrow">可持续更新的资料</span><h1>产品资料</h1><p>把产品说清楚一次，后续创作和执行都会自动带上它。</p></div><button className="primary-button" type="button" onClick={() => { setEditing(true); setIsNew(true); setTitle(""); setContent("# 产品资料\n\n## 卖点\n\n## 目标用户\n\n## 使用场景\n\n## 表达边界\n"); }}> <Plus size={16}/>新建</button></header><div className="knowledge-grid"><aside className="knowledge-list"><label className="knowledge-search"><Search size={15}/><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索资料"/></label><div className="knowledge-entry-note">你可以上传 Markdown、粘贴文字，或在这里直接修改。</div>{visible.length ? visible.map((item) => <button className={selected?.id===item.id?"active":""} type="button" key={item.id} onClick={() => { setEditing(false); setIsNew(false); onSelect(item.id); }}><Package size={16}/><span><strong>{item.title}</strong><small>更新 {formatTime(item.updated_at)}</small></span><ChevronRight size={15}/></button>) : <EmptyState text="还没有匹配的资料" compact/>}</aside><main className="knowledge-reader">{editing ? <form className="living-product-editor" onSubmit={async (event) => { if (selected && !isNew) return save(event); event.preventDefault(); const data=new FormData(); data.append("title", title); data.append("product_md", new File([content], `${title || "产品资料"}.md`, {type:"text/markdown"})); await onCreate(data); setIsNew(false); setEditing(false); }}><div className="knowledge-editor-head"><input value={title} onChange={(event) => setTitle(event.target.value)} placeholder="资料名称" required/><button className="secondary-button" type="button" onClick={() => { setEditing(false); setIsNew(false); setTitle(selected?.title || ""); setContent(productPreview?.content || ""); }}>取消</button><button className="primary-button" disabled={isSavingProduct || isCreatingProduct} type="submit">{isSavingProduct || isCreatingProduct ? "保存中" : "保存资料"}</button></div><textarea value={content} onChange={(event) => setContent(event.target.value)} aria-label="产品资料内容" required/></form> : selected ? <><div className="knowledge-reader-head"><div><span className="eyebrow">当前资料</span><h2>{selected.title}</h2><small>{selected.md_name} · 更新 {formatTime(selected.updated_at)}</small></div><div><button className="secondary-button" type="button" onClick={() => setEditing(true)}><FileText size={15}/>修改</button><button className="primary-button" type="button" onClick={() => onStartJob(selected.id)}><Play size={15}/>开始创作</button></div></div>{isLoadingProductPreview?<EmptyState text="正在读取资料"/>:productPreview?.content?<MarkdownContent content={productPreview.content}/>:<EmptyState text="资料为空，点击修改补充内容"/>}<section className="product-assets"><div><strong>图片与视频素材</strong><small>任务执行时可作为产品参考素材</small></div><label className="asset-upload"><Upload size={15}/>{isUploadingAsset?"上传中":"添加素材"}<input type="file" accept="image/png,image/jpeg,image/webp,image/gif,video/mp4,video/quicktime,video/webm" onChange={addAsset}/></label>{assets.length?<div className="asset-gallery">{assets.map((asset)=><figure key={asset.id}>{asset.kind==="video"?<video src={`/api/assets/${asset.id}/file`} controls preload="metadata"/>:<img src={`/api/assets/${asset.id}/file`} alt={asset.original_name}/>}<figcaption>{asset.original_name}</figcaption></figure>)}</div>:<p className="asset-empty">还没有素材。添加产品图、包装图、演示视频或可用镜头。</p>}</section></> : <EmptyState text="从左侧选择资料，或新建一份"/>}</main><aside className="knowledge-checks"><span className="eyebrow">创作前检查</span><h2>资料是否够用？</h2>{["核心卖点","目标用户","使用场景","表达边界","可用素材"].map((label) => <div key={label}><span className={(content.includes(label) || productPreview?.content?.includes(label) || (label==="可用素材"&&assets.length)) ? "check-ok" : "check-wait"}/><strong>{label}</strong><small>{(content.includes(label) || productPreview?.content?.includes(label) || (label==="可用素材"&&assets.length)) ? "已记录" : "建议补充"}</small></div>)}<p>资料不完整也能开始。助手会在执行前提醒你缺少什么。</p>{error?<div className="error-banner">{error}</div>:null}</aside></div></section>;
}

function ProductCover({ product, compact = false }) {
  const variant = productCoverVariant(product);
  return (
    <div className={`product-cover cover-${variant} ${compact ? "compact" : ""}`}>
      <div className="cover-noise" />
      <div className="cover-frame">
        <span>{productInitial(product?.title)}</span>
        <strong>{product?.title || "新产品"}</strong>
      </div>
      <div className="cover-strip">
        <i />
        <i />
        <i />
      </div>
    </div>
  );
}

function ProductAssetCard({ product, stats, isActive, onSelect, onStartJob }) {
  return (
    <article className={`product-asset-card ${isActive ? "active" : ""}`} onClick={() => onSelect(product.id)}>
      <ProductCover product={product} />
      <div className="asset-card-body">
        <div>
          <h3>{product.title}</h3>
          <p>{product.md_name}</p>
        </div>
        <div className="asset-metrics">
          <span>
            <strong>{stats?.scriptCount || 0}</strong>
            脚本
          </span>
          <span>
            <strong>{stats?.taskCount || 0}</strong>
            任务
          </span>
        </div>
        <div className="asset-card-footer">
          <small>{stats?.latestAt ? `最近生成 ${formatTime(stats.latestAt)}` : `更新 ${formatTime(product.updated_at)}`}</small>
          <button
            className="primary-button compact-action"
            type="button"
            onClick={(event) => {
              event.stopPropagation();
              onStartJob(product.id);
            }}
          >
            <Play size={15} />
            <span>生成新脚本</span>
          </button>
        </div>
      </div>
    </article>
  );
}

function ProductsWorkspace({
  products,
  selectedProductId,
  productPreview,
  creativeReports,
  selectedCreativeReportId,
  isLoadingProductPreview,
  isLoadingCreativeReports,
  isCreatingCreativeReport,
  isCreatingProduct,
  isSavingProduct,
  productStats,
  error,
  onSelect,
  onReportSelect,
  onStartJob,
  onCreateCreativeReport,
  onReportToJob,
  onCreate,
  onUpdate,
}) {
  const [editing, setEditing] = useState(false);
  const [editTitle, setEditTitle] = useState("");
  const [editContent, setEditContent] = useState("");
  const selectedProduct = products.find((product) => product.id === selectedProductId) || products[0];
  const selectedStats = selectedProduct ? productStats.get(selectedProduct.id) : null;
  const selectedReport = creativeReports.find((report) => report.id === selectedCreativeReportId) || creativeReports[0];
  const totalScripts = Array.from(productStats.values()).reduce((total, stats) => total + (stats.scriptCount || 0), 0);
  useEffect(() => { if (!editing) { setEditTitle(selectedProduct?.title || ""); setEditContent(productPreview?.content || ""); } }, [selectedProduct?.id, productPreview?.content, editing]);
  return (
    <section className="product-home">
      <div className="product-home-hero">
        <div>
          <span className="eyebrow">产品资产库</span>
          <h1>先把产品放进来，再批量裂变脚本</h1>
          <p>每个产品都对应一份 Markdown 资料、历史任务和可复用脚本。运营同学进来先看到自己的产品，而不是一张空表单。</p>
        </div>
        <div className="home-stats">
          <div>
            <strong>{products.length}</strong>
            <span>产品</span>
          </div>
          <div>
            <strong>{totalScripts}</strong>
            <span>脚本</span>
          </div>
        </div>
      </div>

      <div className="product-home-grid">
        <section className="asset-board">
          <div>
            <h2>我的产品</h2>
            <p>选择一个产品查看资料，或直接生成新的复刻/裂变脚本。</p>
          </div>
          {products.length ? (
            <div className="product-asset-grid">
              {products.map((product) => (
                <ProductAssetCard
                  key={product.id}
                  product={product}
                  stats={productStats.get(product.id)}
                  isActive={selectedProduct?.id === product.id}
                  onSelect={onSelect}
                  onStartJob={onStartJob}
                />
              ))}
            </div>
          ) : (
            <EmptyState text="暂无产品，先上传一份产品 Markdown" />
          )}
        </section>

        <aside className="new-product-panel">
          <div>
            <h2>新增产品</h2>
            <p>上传一份产品 Markdown，后续任务直接复用。</p>
          </div>
          <form className="task-form" onSubmit={onCreate}>
            <label>
              <span>产品名称</span>
              <input name="title" type="text" placeholder="例如：无尽冬日" />
            </label>
            <FileField name="product_md" label="产品 Markdown" accept=".md,.markdown,text/markdown" icon={<FileText size={16} />} required />
            <button className="primary-button wide-button" type="submit" disabled={isCreatingProduct}>
              {isCreatingProduct ? <Loader2 className="spin" size={16} /> : <Plus size={16} />}
              <span>{isCreatingProduct ? "保存中" : "保存产品"}</span>
            </button>
          </form>
          {error ? <div className="error-banner">{error}</div> : null}
        </aside>
      </div>

      <section className="product-dossier">
        {selectedProduct ? (
          <>
            <div className="dossier-summary">
              <ProductCover product={selectedProduct} compact />
              <div>
                <span className="eyebrow">产品档案</span>
                <h2>{selectedProduct.title}</h2>
                <p>{selectedProduct.md_name} · 更新 {formatTime(selectedProduct.updated_at)}</p>
              </div>
              <div className="dossier-actions">
                <span className="status-pill success">可用于任务</span>
                <button className="secondary-button" type="button" onClick={() => setEditing((value) => !value)}><FileText size={15} /><span>{editing ? "阅读资料" : "编辑资料"}</span></button>
                <button className="secondary-button" type="button" onClick={() => onStartJob(selectedProduct.id)}>
                  <Play size={15} />
                  <span>生成新脚本</span>
                </button>
              </div>
            </div>
            <div className="product-meta-grid">
              <div className="detail-row">
                <span>已生成脚本</span>
                <strong>{selectedStats?.scriptCount || 0}</strong>
              </div>
              <div className="detail-row">
                <span>历史任务</span>
                <strong>{selectedStats?.taskCount || 0}</strong>
              </div>
              <div className="detail-row">
                <span>最近生成</span>
                <strong>{selectedStats?.latestAt ? formatTime(selectedStats.latestAt) : "-"}</strong>
              </div>
            </div>
            <section className="creative-report-panel">
              <div className="section-heading">
                <span>创意策略报告</span>
                <small>配置 DataEye 来源，生成后可转为裂变脚本任务</small>
              </div>
              <div className="creative-report-grid">
                <form className="task-form compact-form" onSubmit={onCreateCreativeReport}>
                  <div className="upload-grid">
                    <label>
                      <span>DataEye URL</span>
                      <input name="dataeye_url" type="url" placeholder="https://..." />
                    </label>
                    <label>
                      <span>DataEye 产品 ID</span>
                      <input name="dataeye_id" type="text" placeholder="可选" />
                    </label>
                  </div>
                  <div className="upload-grid">
                    <label>
                      <span>产品名</span>
                      <input name="product_name" type="text" defaultValue={selectedProduct.title} />
                    </label>
                    <label>
                      <span>时间范围</span>
                      <input name="date_range" type="text" defaultValue="近 30 天" />
                    </label>
                  </div>
                  <div className="upload-grid three-cols">
                    <label>
                      <span>媒体</span>
                      <input name="media" type="text" placeholder="TikTok / Meta" />
                    </label>
                    <label>
                      <span>国家/地区</span>
                      <input name="country" type="text" placeholder="美国 / 日本" />
                    </label>
                    <label>
                      <span>样本数</span>
                      <input name="sample_count" type="number" min="5" max="200" defaultValue="50" />
                    </label>
                  </div>
                  <label>
                    <span>排序指标</span>
                    <input name="sort_metric" type="text" defaultValue="热度/曝光/播放优先" />
                  </label>
                  <label>
                    <span>补充要求</span>
                    <textarea name="requirement" rows="3" placeholder="例如：重点看开头钩子、节奏和可复制素材结构。" />
                  </label>
                  <label>
                    <span>素材备注</span>
                    <textarea name="material_note" rows="3" placeholder="如已有素材观察、DataEye 导出摘要，可贴在这里。" />
                  </label>
                  <button className="primary-button wide-button" type="submit" disabled={isCreatingCreativeReport}>
                    {isCreatingCreativeReport ? <Loader2 className="spin" size={16} /> : <Activity size={16} />}
                    <span>{isCreatingCreativeReport ? "生成中" : "生成创意策略报告"}</span>
                  </button>
                </form>
                <div className="creative-report-list">
                  {isLoadingCreativeReports ? (
                    <EmptyState text="正在读取创意策略报告" compact />
                  ) : creativeReports.length ? (
                    <>
                      <div className="report-switcher">
                        {creativeReports.map((report) => (
                          <button
                            key={report.id}
                            className={selectedReport?.id === report.id ? "active" : ""}
                            type="button"
                            onClick={() => onReportSelect(report.id)}
                          >
                            <strong>{formatTime(report.created_at)}</strong>
                            <span>{reportConfigLabel(report.source_config_json)}</span>
                          </button>
                        ))}
                      </div>
                      {selectedReport ? (
                        <article className="creative-report-preview">
                          <div className="report-actions">
                            <div>
                              <strong>报告摘要</strong>
                              <p>{selectedReport.report_summary || "报告已生成，可查看完整内容。"}</p>
                            </div>
                            <button className="secondary-button" type="button" onClick={() => onReportToJob(selectedReport)}>
                              <Play size={15} />
                              <span>转裂变任务</span>
                            </button>
                          </div>
                          <MarkdownContent content={selectedReport.report_markdown} />
                        </article>
                      ) : null}
                    </>
                  ) : (
                    <EmptyState text="暂无报告，先生成一份创意策略报告" compact />
                  )}
                </div>
              </div>
            </section>
            <section className="product-preview">
              <div className="section-heading">
                <span>Markdown 预览</span>
                <small>{isLoadingProductPreview ? "读取中" : productPreview?.md_name || selectedProduct.md_name}</small>
              </div>
              {editing ? (
                <form className="living-product-editor" onSubmit={async (event) => { event.preventDefault(); await onUpdate(selectedProduct.id, editTitle, editContent); setEditing(false); }}>
                  <input value={editTitle} onChange={(event) => setEditTitle(event.target.value)} required aria-label="产品名称" />
                  <textarea value={editContent} onChange={(event) => setEditContent(event.target.value)} required aria-label="产品资料内容" />
                  <button className="primary-button" type="submit" disabled={isSavingProduct}>{isSavingProduct ? "保存中" : "保存更新"}</button>
                </form>
              ) : productPreview?.error ? (
                <div className="error-banner">{productPreview.error}</div>
              ) : isLoadingProductPreview ? (
                <EmptyState text="正在读取产品 Markdown" compact />
              ) : productPreview?.content ? (
                <MarkdownContent content={productPreview.content} />
              ) : (
                <EmptyState text="暂无可预览内容" compact />
              )}
            </section>
          </>
        ) : (
          <EmptyState text="保存产品后可在这里查看档案" />
        )}
      </section>
    </section>
  );
}

function SettingsWorkspace({ modelSettings, showDebugPanel, isSaving, error, onDebugPanel, onSave, ownerSession, isOwnerLoading, onOwnerLogin }) {
  return (
    <section className="result-pane full-height">
      <div className="result-header">
        <div>
          <h2>模型配置</h2>
          <p>选择模型厂商，或接入任意兼容 OpenAI Chat Completions 协议的模型服务。</p>
        </div>
        <span className={`status-pill ${modelSettings?.configured ? "success" : "danger"}`}>
          <KeyRound size={13} />
          {modelSettings?.configured ? "已配置" : "未配置"}
        </span>
      </div>
      <form className="settings-form" onSubmit={onSave}>
        {error ? <div className="error-banner">{error}</div> : null}
        <div className="security-callout">
          <KeyRound size={18} />
          <div>
            <strong>API Key 只保存在当前部署的本地数据库</strong>
            <p>不要公开分享 `data/scriptagent.db`。给团队使用时，建议增加登录和密钥加密。</p>
          </div>
        </div>
        <div className="form-section">
          <div className="section-heading">
            <span>模型服务</span>
            <small>{modelSettings?.api_key_mask ? `当前 Key：${modelSettings.api_key_mask}` : "保存后立即生效"}</small>
          </div>
          <label>
            <span>接口类型</span>
            <select name="provider" defaultValue={modelSettings?.provider || "dashscope"}>
              <option value="dashscope">DashScope（支持本地视频理解）</option>
              <option value="openai">OpenAI 兼容接口（文本任务）</option>
            </select>
            <small className="field-hint">兼容接口可用于 OpenAI、DeepSeek、Moonshot、OpenRouter、Groq、Ollama、vLLM 等服务。</small>
          </label>
          <label>
            <span>API Key</span>
            <input name="api_key" type="password" placeholder={modelSettings?.configured ? "留空则保留当前 Key" : "输入对应厂商的 API Key"} autoComplete="new-password" />
          </label>
          <div className="upload-grid">
            <label>
              <span>模型</span>
              <input name="model" type="text" defaultValue={modelSettings?.model || "qwen3.6-plus"} />
            </label>
            <label>
              <span>接口地址</span>
              <input name="endpoint" type="text" defaultValue={modelSettings?.endpoint || "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation"} placeholder="完整的 Chat Completions 地址" />
            </label>
          </div>
        </div>
        <div className="form-section developer-options">
          <div className="section-heading">
            <span>开发者选项</span>
            <small>普通运营用户不需要开启</small>
          </div>
          <label className="toggle-row">
            <span>
              <strong>显示调试台</strong>
              <small>查看模型输入输出、token、原始响应。</small>
            </span>
            <input type="checkbox" checked={showDebugPanel} onChange={(event) => onDebugPanel(event.target.checked)} />
          </label>
        </div>
        <div className="form-section owner-login-section">
          <div className="section-heading"><span>运营后台</span><small>{ownerSession?.authenticated ? "管理员已登录" : "仅所有者可见"}</small></div>
          {ownerSession?.authenticated ? <div className="owner-authenticated"><CheckCircle2 size={18}/><div><strong>身份验证有效</strong><small>运营后台入口已显示在侧栏。</small></div></div> : ownerSession?.configured ? <div className="owner-login-fields"><label><span>管理员账号</span><input name="username" autoComplete="username" /></label><label><span>管理员密码</span><input name="password" type="password" autoComplete="current-password" /></label><button className="secondary-button" type="button" disabled={isOwnerLoading} onClick={(event) => { const form = event.currentTarget.closest("form"); onOwnerLogin({ preventDefault() {}, currentTarget: form }); }}>{isOwnerLoading ? "验证中" : "管理员登录"}</button></div> : <div className="security-callout"><KeyRound size={18}/><div><strong>尚未配置管理员账号</strong><p>在服务器设置 SCRIPT_AGENT_OWNER_USERNAME 和 SCRIPT_AGENT_OWNER_PASSWORD 后重启。</p></div></div>}
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

function OwnerDashboard({ overview, isLoading, error, onRefresh, onLogout }) {
  const totals = overview?.totals || {};
  const metrics = [["创作空间", totals.spaces || 0], ["任务总数", totals.jobs || 0], ["Agent Runs", totals.runs || 0], ["模型调用", totals.model_calls || 0], ["Token 总量", formatNumber(totals.total_tokens || 0)], ["已发布脚本", totals.published_jobs || 0]];
  return <section className="owner-dashboard">
    <header className="owner-dashboard-head"><div><span>OWNER ONLY</span><h1>产品运营总览</h1><p>跨空间查看任务产出、Agent 运行和模型消耗。</p></div><div><button className="secondary-button" type="button" onClick={onRefresh} disabled={isLoading}><RefreshCw size={15}/>刷新</button><button className="secondary-button" type="button" onClick={onLogout} disabled={isLoading}>退出登录</button></div></header>
    {error ? <div className="error-banner">{error}</div> : null}
    {!overview || isLoading ? <div className="debug-empty"><Loader2 className="spin" size={26}/><strong>正在读取运营数据</strong></div> : <>
      <div className="owner-metrics">{metrics.map(([label,value]) => <article key={label}><span>{label}</span><strong>{value}</strong></article>)}</div>
      <div className="owner-dashboard-grid"><section><div className="debug-section-head"><h2>空间表现</h2><span>{overview.spaces?.length || 0} 个空间</span></div><div className="owner-space-table">{overview.spaces?.map((space) => <article key={space.id}><div><strong>{space.title}</strong><small>{space.runs} 次运行 · {space.model_calls} 次调用</small></div><span>{formatNumber(space.total_tokens)} tokens</span><span className={space.failed_runs ? "owner-failed" : "owner-healthy"}>{space.failed_runs ? `${space.failed_runs} 次失败` : "运行正常"}</span></article>)}</div></section><section><div className="debug-section-head"><h2>最近运行</h2><span>{overview.recent_runs?.length || 0} 条</span></div><div className="owner-run-list">{overview.recent_runs?.length ? overview.recent_runs.map((run) => <article key={run.ID}><div><strong>{run.SpaceTitle}</strong><small>{formatTime(run.StartedAt)}</small></div><span className={`debug-call-status ${run.Status === "failed" ? "danger" : "success"}`}>{run.Status === "failed" ? "失败" : run.Status === "completed" ? "完成" : "运行中"}</span></article>) : <div className="debug-empty compact">暂无运行记录</div>}</div></section></div>
    </>}
  </section>;
}

function ModelCallsWorkspace({ spaces = [], error }) {
  const [expandedCallId, setExpandedCallId] = useState("");
  const [selectedSpaceId, setSelectedSpaceId] = useState(() => spaces[0]?.id || "");
  const [observability, setObservability] = useState({ runs: [], steps: [], model_calls: [], memory_events: [] });
  const [isLoadingObservability, setIsLoadingObservability] = useState(false);
  const [observabilityError, setObservabilityError] = useState("");
  const activeSpaceId = spaces.some((space) => space.id === selectedSpaceId) ? selectedSpaceId : spaces[0]?.id || "";
  const visibleCalls = observability.model_calls || [];
  const runs = observability.runs || [];
  const runSteps = observability.steps || [];
  const memoryEvents = observability.memory_events || [];
  const totalInput = visibleCalls.reduce((total, call) => total + Number(call.prompt_tokens || 0), 0);
  const totalOutput = visibleCalls.reduce((total, call) => total + Number(call.output_tokens || 0), 0);
  const totalLatency = visibleCalls.reduce((total, call) => total + Number(call.latency_ms || 0), 0);
  const runCount = runs.length;

  useEffect(() => {
    if (!activeSpaceId) {
      setObservability({ runs: [], steps: [], model_calls: [], memory_events: [] });
      return undefined;
    }
    let cancelled = false;
    setIsLoadingObservability(true);
    setObservabilityError("");
    getSpaceObservability(activeSpaceId).then((result) => {
      if (!cancelled) setObservability(result);
    }).catch((err) => {
      if (!cancelled) setObservabilityError(err.message);
    }).finally(() => {
      if (!cancelled) setIsLoadingObservability(false);
    });
    return () => { cancelled = true; };
  }, [activeSpaceId]);

  const metrics = [
    ["输入 Token", formatNumber(totalInput)],
    ["输出 Token", formatNumber(totalOutput)],
    ["缓存读取", "0"],
    ["缓存写入", "0"],
    ["总耗时", formatDuration(totalLatency)],
    ["API 耗时", visibleCalls.length ? formatDuration(totalLatency) : "未返回"],
    ["首 Token", "未返回"],
    ["缓存命中率", "0%"],
  ];

  return (
    <section className="debug-observer">
      <header className="debug-observer-head">
        <div className="debug-observer-title"><Activity size={20} /><strong>运行调试</strong></div>
        <div className="debug-space-control">
          <label htmlFor="debug-space">创作空间</label>
          <select id="debug-space" value={activeSpaceId} onChange={(event) => { setSelectedSpaceId(event.target.value); setExpandedCallId(""); }}>
            {spaces.length ? spaces.map((space) => <option value={space.id} key={space.id}>{space.title}</option>) : <option value="">暂无空间</option>}
          </select>
          <span>{runCount} 次运行</span>
        </div>
      </header>
      {error ? <div className="error-banner">{error}</div> : null}
      {observabilityError ? <div className="error-banner">{observabilityError}</div> : null}

      <div className="debug-observer-body">
        <div className="debug-intro">
          <h1>Agent Loop 观测</h1>
          <p>查看每次模型请求、响应、Token 与耗时。敏感信息会在服务端脱敏。</p>
        </div>

        <div className="debug-metric-grid">
          {metrics.map(([label, value]) => <div className="debug-metric" key={label}><span>{label}</span><strong>{value}</strong></div>)}
        </div>

        <section className="debug-section">
          <div className="debug-section-head"><h2>模型调用</h2><span>{visibleCalls.length} 次</span></div>
          <div className="debug-call-list">
            {isLoadingObservability ? <div className="debug-empty compact"><Loader2 className="spin" size={24}/><strong>正在加载空间运行记录</strong></div> : visibleCalls.length ? visibleCalls.map((call, index) => {
              const expanded = expandedCallId === call.id;
              return <article className={`debug-call ${expanded ? "expanded" : ""}`} key={call.id}>
                <button type="button" aria-expanded={expanded} onClick={() => setExpandedCallId(expanded ? "" : call.id)}>
                  {expanded ? <ChevronDown size={18}/> : <ChevronRight size={18}/>}
                  <strong>第 {index + 1} 次模型请求</strong>
                  <span className="debug-call-model">{call.scope || "agent"} · {call.step || "模型调用"}</span>
                  <span className={`debug-call-status ${call.error_message ? "danger" : "success"}`}>{call.error_message ? "失败" : "完成"}</span>
                </button>
                {expanded ? <div className="debug-call-detail">
                  <div className="debug-call-meta">
                    <span>输入 {formatNumber(call.prompt_tokens || 0)}</span><span>输出 {formatNumber(call.output_tokens || 0)}</span><span>耗时 {formatDuration(call.latency_ms || 0)}</span><span>{formatTime(call.created_at)}</span>
                  </div>
                  {call.error_message ? <div className="error-banner">{call.error_message}</div> : null}
                  <section><h3>模型输入</h3><pre className="result-output json">{pretty(call.input_json)}</pre></section>
                  <section><h3>模型输出</h3><pre className="result-output markdown">{call.output_text || "-"}</pre></section>
                  <section><h3>原始响应</h3><pre className="result-output json">{pretty(call.response_json)}</pre></section>
                </div> : null}
              </article>;
            }) : <div className="debug-empty compact"><CircleHelp size={24}/><strong>暂无模型调用记录</strong><p>运行 Agent 后，请求详情会实时显示在这里。</p></div>}
          </div>
        </section>

        <section className="debug-section memory-section">
          <div className="debug-section-head"><h2>运行步骤</h2><span>{runSteps.length} 条</span></div>
          {runSteps.length ? <div className="debug-memory-list">{runSteps.map((step) => <article key={step.id}><strong>{step.index}. {step.key}</strong><span>{step.output_summary || step.input_summary || "-"}</span><time>{step.status} · {formatTime(step.started_at)}</time></article>)}</div> : <div className="debug-empty"><CircleHelp size={28}/><strong>暂无运行步骤</strong><p>任务开始后会按 Run 记录工作流步骤。</p></div>}
        </section>

        <section className="debug-section memory-section">
          <div className="debug-section-head"><h2>Memory 行为</h2><span>{memoryEvents.length} 条</span></div>
          {memoryEvents.length ? <div className="debug-memory-list">{memoryEvents.map((event) => <article key={event.id}><strong>{event.kind}</strong><span>{event.payload || "-"}</span><time>{formatTime(event.created_at)}</time></article>)}</div> : <div className="debug-empty"><CircleHelp size={28}/><strong>本次运行没有 Memory 事件</strong><p>挂载、提取、Dream、同步和冲突会在这里实时显示。</p></div>}
        </section>
      </div>
    </section>
  );
}

function formatNumber(value) {
  return new Intl.NumberFormat("zh-CN").format(Number(value || 0));
}

function formatDuration(milliseconds) {
  const value = Number(milliseconds || 0);
  if (!value) return "0 秒";
  if (value < 1000) return `${value} 毫秒`;
  const seconds = Math.round(value / 1000);
  if (seconds < 60) return `${seconds} 秒`;
  return `${Math.floor(seconds / 60)} 分 ${seconds % 60} 秒`;
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

function ResultHeader({ job, isRetrying, canRetry, onRetry }) {
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
        </div>
      ) : null}
    </div>
  );
}

function ResultContent({ job, content, activeTab, isGeneratingVideoPrompts }) {
  if (!job) return <EmptyState text="暂无任务结果" />;
  if (job.error_message && job.status === "failed") return <pre className="result-output error-output">{job.error_message}</pre>;
  if (activeTab === "video_prompts" && isGeneratingVideoPrompts) return <EmptyState text="正在生成 Seedance 视频提示词" />;
  if (!content) return <EmptyState text={runningStatuses.has(job.status) ? "任务执行中" : "当前页暂无内容"} />;
  if (activeTab === "run_log") return <TimelineContent content={content} />;
  if (activeTab === "analysis_markdown") return <MarkdownContent content={content} />;
  if (activeTab === "video_prompts") return <MarkdownContent content={content} />;
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

function reportConfigLabel(raw) {
  const config = parseJSONValue(raw) || {};
  return [config.date_range, config.media, config.country, config.sort_metric].filter(Boolean).join(" · ") || "创意策略报告";
}
