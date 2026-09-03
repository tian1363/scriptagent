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
  ChevronLeft,
  ChevronRight,
  CircleHelp,
  Loader2,
  LogOut,
  Mail,
  Mic,
  MicOff,
  MessageSquare,
  Package,
  Play,
  Plus,
  RefreshCw,
  RotateCcw,
  Radar,
  Send,
  Settings,
  ShieldCheck,
  Sparkles,
  KeyRound,
  Upload,
  Video,
  Trash2,
  X,
} from "lucide-react";
import { Fragment, useEffect, useMemo, useRef, useState } from "react";
import {
  createJob,
  createVideo,
  createCreativeReport,
  createProduct,
  parseProductDocument,
  createSpace,
  deleteSpace,
  updateSpace,
  createSkill,
  generateSkillDraft,
  updateSkill,
  updateProduct,
  generateVideoPrompts,
  getModelSettings,
  getAuthStatus,
  getCurrentUser,
  getSpaceObservability,
  getIntelligence,
  seedIntelligenceDemo,
  promoteIntelligenceSignal,
  updateCreativeMemory,
  deleteCreativeMemory,
  createCompetitorMonitor,
  scanCompetitorMonitor,
  getOwnerSession,
  getOwnerOverview,
  loginOwner,
  login,
  logout,
  logoutOwner,
  getChat,
  getChatProgress,
  getJob,
  getProductMarkdown,
  listCreativeReports,
  listProducts,
  listProductAssets,
  listSpaces,
  uploadProductAsset,
  deleteProductAsset,
  listSkills,
  listChats,
  listJobs,
  listModelCalls,
  listVideos,
  listSuggestions,
  updateSuggestionStatus,
  retryJob,
  retryVideo,
  register,
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

const marketingGoals = [
  ["conversion", "商品转化", "推动下单、留资或到店"],
  ["awareness", "品牌认知", "建立记忆点与品牌心智"],
  ["seeding", "内容种草", "强化场景、卖点与信任"],
  ["growth", "用户增长", "促进关注、互动与拉新"],
  ["campaign", "活动引爆", "集中放大活动声量与参与"],
];

const marketingStages = [
  ["reach", "认知", "先让目标人群看见并记住"],
  ["interest", "兴趣", "让用户理解价值并产生偏好"],
  ["action", "行动", "用明确利益点推动立即转化"],
];

function marketingLabel(options, value) {
  return options.find(([id]) => id === value)?.[1] || "未设置";
}

function suggestionActionLabel(actionType) {
  return (
    {
      continue_space: "继续创作",
      open_space: "完善空间",
      open_product: "添加素材",
      review_video: "查看视频",
      review_signal: "查看证据",
    }[actionType] || "查看"
  );
}

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
    items: [
      "换BGM",
      "换音效",
      "换色调/滤镜",
      "换字幕&花字",
      "换画幅",
      "换配音(语速/声线)",
    ],
  },
  {
    layer: "结构层",
    items: [
      "换开头钩子",
      "换CTA",
      "时长压缩/拉伸",
      "变速·节奏调整",
      "换首帧/封面",
      "同素材高光重剪",
    ],
  },
  {
    layer: "元素层",
    items: [
      "换局部角色/群演",
      "换局部场景贴片",
      "换局部道具/UI",
      "字幕语言本地化",
    ],
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
    const matchedJobs = safeJobs.filter(
      (job) => job.product_md_name === product.md_name,
    );
    const usableJobs = matchedJobs.filter((job) => job.status !== "failed");
    const scriptCount = usableJobs.reduce(
      (total, job) => total + 1 + Number(job.fission_count || 0),
      0,
    );
    const latestJob = matchedJobs.reduce((latest, job) => {
      if (!latest) return job;
      return new Date(job.updated_at).getTime() >
        new Date(latest.updated_at).getTime()
        ? job
        : latest;
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

function visibleAssistantContent(content = "") {
  const value = String(content).trim();
  if (!value.startsWith("{")) return value;
  try {
    const parsed = JSON.parse(value);
    return typeof parsed.answer === "string" ? parsed.answer : value;
  } catch {
    return value;
  }
}

function videoParametersFromPrompt(prompt = "") {
  const ratios = prompt.match(/(?:16:9|9:16|1:1|4:3|3:4)/g);
  const rangeEnds = [
    ...prompt.matchAll(/\d+\s*[-–—~至]\s*(\d+)\s*(?:s|秒)/gi),
  ].map((match) => Number(match[1]));
  const statedDuration = prompt.match(
    /(\d+)\s*(?:s|秒)(?:视频|成片|真人|UGC)/i,
  );
  const duration = Math.max(
    2,
    Math.min(
      30,
      rangeEnds.length
        ? Math.max(...rangeEnds)
        : Number(statedDuration?.[1] || 5),
    ),
  );
  const cid =
    prompt.match(/CID\s*:?\s*asset-([a-zA-Z0-9_-]+)/i)?.[1] || "";
  return { ratio: ratios?.[0] || "9:16", duration, cid };
}

function isDirectVideoCommand(content = "") {
  return /^(?:请)?(?:直接|立即|开始|按上文|用上文)?(?:给我)?生成(?:这个|这段|一段)?视频[。！!]?$/u.test(
    content.trim(),
  );
}

const chatQuickTasks = [
  {
    title: "生成裂变方向",
    description:
      "基于当前产品，先给我 3 个适合短视频的裂变方向，并说明每个方向改哪里。",
  },
  {
    title: "写产品 Markdown",
    description:
      "帮我把这个产品整理成可用于生成脚本的 Markdown，需要包含卖点、用户、场景、限制和素材备注。",
  },
  {
    title: "优化脚本",
    description:
      "帮我检查这条脚本哪里可以优化，重点看开头钩子、产品卖点和 CTA。",
  },
  {
    title: "拆解并复刻素材",
    description:
      "调用 material_replication_analysis skill，分析我上传的图片或视频，拆解设计方式、内容表达和视听结构，并给出视频复刻建议。",
  },
  {
    title: "生成视频提示词",
    description:
      "调用 seedance_video_prompt_writer skill，把这条分镜脚本转成 Seedance 可用的视频生成提示词。",
  },
];

export function App() {
  const [view, setView] = useState("home");
  const [currentUser, setCurrentUser] = useState(null);
  const [isCheckingAuth, setIsCheckingAuth] = useState(true);
  const [registrationAvailable, setRegistrationAvailable] = useState(false);
  const [authError, setAuthError] = useState("");
  const [jobs, setJobs] = useState([]);
  const [videos, setVideos] = useState([]);
  const [isCreatingVideo, setIsCreatingVideo] = useState(false);
  const [selectedJob, setSelectedJob] = useState(null);
  const [activeTab, setActiveTab] = useState("run_log");
  const [videoPromptContent, setVideoPromptContent] = useState("");
  const [isGeneratingVideoPrompts, setIsGeneratingVideoPrompts] =
    useState(false);
  const [isCreating, setIsCreating] = useState(false);
  const [isRetrying, setIsRetrying] = useState(false);
  const [products, setProducts] = useState([]);
  const [spaces, setSpaces] = useState([]);
  const [suggestions, setSuggestions] = useState([]);
  const [isCreatingSpace, setIsCreatingSpace] = useState(false);
  const [skills, setSkills] = useState([]);
  const [selectedProductId, setSelectedProductId] = useState("");
  const [productPreview, setProductPreview] = useState(null);
  const [creativeReports, setCreativeReports] = useState([]);
  const [selectedCreativeReportId, setSelectedCreativeReportId] = useState("");
  const [isLoadingProductPreview, setIsLoadingProductPreview] = useState(false);
  const [isLoadingCreativeReports, setIsLoadingCreativeReports] =
    useState(false);
  const [isCreatingCreativeReport, setIsCreatingCreativeReport] =
    useState(false);
  const [isCreatingProduct, setIsCreatingProduct] = useState(false);
  const [isSavingProduct, setIsSavingProduct] = useState(false);
  const [jobInitialRequirement, setJobInitialRequirement] = useState("");
  const [modelSettings, setModelSettings] = useState(null);
  const [isSavingSettings, setIsSavingSettings] = useState(false);
  const [showModelOnboarding, setShowModelOnboarding] = useState(false);
  const [showDebugPanel, setShowDebugPanel] = useState(
    () => window.localStorage.getItem("scriptagent:debug-panel") === "true",
  );
  const [ownerSession, setOwnerSession] = useState({
    configured: false,
    authenticated: false,
  });
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
  const [isMainSidebarCollapsed, setIsMainSidebarCollapsed] = useState(() => {
    try {
      const saved = window.localStorage.getItem(
        "scriptagent:main-sidebar-collapsed",
      );
      return saved === null ? true : saved === "true";
    } catch {
      return true;
    }
  });
  const [isChatHistoryCollapsed, setIsChatHistoryCollapsed] = useState(() => {
    try {
      return (
        window.localStorage.getItem("scriptagent:chat-history-collapsed") ===
        "true"
      );
    } catch {
      return false;
    }
  });
  useEffect(() => {
    try {
      window.localStorage.setItem(
        "scriptagent:main-sidebar-collapsed",
        String(isMainSidebarCollapsed),
      );
    } catch {}
  }, [isMainSidebarCollapsed]);
  useEffect(() => {
    try {
      window.localStorage.setItem(
        "scriptagent:chat-history-collapsed",
        String(isChatHistoryCollapsed),
      );
    } catch {}
  }, [isChatHistoryCollapsed]);

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
    const targetId =
      nextSelectedId || selectedChat?.conversation?.id || items[0]?.id;
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
  async function refreshSpaces() {
    const items = await listSpaces();
    setSpaces(Array.isArray(items) ? items : []);
  }
  async function refreshVideos() {
    const items = await listVideos();
    setVideos(Array.isArray(items) ? items : []);
  }
  async function refreshSuggestions() {
    const items = await listSuggestions();
    setSuggestions(Array.isArray(items) ? items : []);
  }

  async function handleSuggestion(suggestion, status = "accepted") {
    setError("");
    try {
      await updateSuggestionStatus(suggestion.id, status);
      setSuggestions((items) =>
        items.filter((item) => item.id !== suggestion.id),
      );
      if (status !== "accepted") return;
      if (
        suggestion.action_type === "continue_space" ||
        suggestion.action_type === "open_space"
      ) {
        const space = spaces.find(
          (item) => item.id === suggestion.action_target_id,
        );
        if (space) await handleStartSpaceAgent(space);
        else setView("spaces");
      } else if (suggestion.action_type === "open_product") {
        setSelectedProductId(
          suggestion.action_target_id || suggestion.product_id,
        );
        setView("products");
      } else if (suggestion.action_type === "review_video") {
        setView("history");
      } else if (suggestion.action_type === "review_signal") {
        setView("intelligence");
      }
    } catch (err) {
      setError(err.message);
    }
  }

  async function refreshSkills() {
    setSkills(await listSkills());
  }

  async function handleSaveSkill(input, id = "") {
    const skill = id ? await updateSkill(id, input) : await createSkill(input);
    await refreshSkills();
    return skill;
  }

  async function refreshModelSettings() {
    setModelSettings(await getModelSettings());
  }

  function finishModelOnboarding() {
    try {
      window.localStorage.setItem("scriptagent:model-onboarding-v1", "done");
    } catch {}
    setShowModelOnboarding(false);
  }

  async function handleModelOnboardingSave(selection) {
    setError("");
    setIsSavingSettings(true);
    try {
      const profiles = ["text", "multimodal"].map((capability) => {
        const catalog = modelCapabilityCatalog.find(
          (item) => item.id === capability,
        );
        const saved = modelSettings?.profiles?.find(
          (item) => item.capability === capability,
        );
        return {
          capability,
          mode: saved?.mode || "managed",
          api_key: "",
          provider: saved?.provider || "dashscope",
          endpoint: saved?.endpoint || catalog.endpoint,
          model: selection[capability] || catalog.models[0].id,
        };
      });
      setModelSettings(await saveModelSettings({ profiles }));
      finishModelOnboarding();
    } catch (err) {
      setError(err.message);
    } finally {
      setIsSavingSettings(false);
    }
  }

  async function refreshCurrent() {
    setError("");
    if (view === "jobs") await refreshJobs();
    if (view === "chat") await refreshChats();
    if (view === "calls" && showDebugPanel)
      await Promise.all([refreshModelCalls(), refreshJobs(), refreshSpaces()]);
    if (view === "products") await refreshProducts();
    if (view === "spaces") await refreshSpaces();
    if (view === "history")
      await Promise.all([refreshJobs(), refreshChats(), refreshVideos()]);
    if (view === "settings") await refreshModelSettings();
    if (view === "admin" && ownerSession.authenticated)
      setOwnerOverview(await getOwnerOverview());
  }

  async function handleOwnerLogin(event) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setError("");
    setIsOwnerLoading(true);
    try {
      const session = await loginOwner(
        String(form.get("username") || ""),
        String(form.get("password") || ""),
      );
      setOwnerSession(session);
      setOwnerOverview(await getOwnerOverview());
      setView("admin");
      event.currentTarget.reset();
    } catch (err) {
      setError(err.message);
    } finally {
      setIsOwnerLoading(false);
    }
  }

  async function handleOwnerLogout() {
    setIsOwnerLoading(true);
    setError("");
    try {
      const session = await logoutOwner();
      setOwnerSession(session);
      setOwnerOverview(null);
      setView("settings");
    } catch (err) {
      setError(err.message);
    } finally {
      setIsOwnerLoading(false);
    }
  }

  async function handleAuthSubmit(mode, values) {
    setAuthError("");
    try {
      const user = await (mode === "register"
        ? register(values)
        : login(values));
      setCurrentUser(user);
      setRegistrationAvailable(false);
      setView("home");
      return true;
    } catch (err) {
      setAuthError(err.message);
      return false;
    }
  }

  async function handleLogout() {
    await logout().catch(() => {});
    setCurrentUser(null);
    setSelectedJob(null);
    setSelectedChat(null);
    setJobs([]);
    setProducts([]);
    setSpaces([]);
    setSuggestions([]);
    setChats([]);
    setSkills([]);
    setModelCalls([]);
    setAuthError("");
  }

  useEffect(() => {
    Promise.all([getAuthStatus(), getCurrentUser().catch(() => null)])
      .then(([status, user]) => {
        setRegistrationAvailable(Boolean(status?.registration_available));
        setCurrentUser(user);
      })
      .catch((err) => setAuthError(err.message))
      .finally(() => setIsCheckingAuth(false));
  }, []);

  useEffect(() => {
    if (!currentUser) return;
    refreshJobs().catch((err) => setError(err.message));
    refreshChats().catch(() => {});
    if (showDebugPanel) refreshModelCalls().catch(() => {});
    refreshProducts().catch(() => {});
    refreshSpaces().catch(() => {});
    refreshSkills().catch(() => {});
    refreshVideos().catch(() => {});
    refreshSuggestions().catch(() => {});
    refreshModelSettings().catch(() => {});
    getOwnerSession()
      .then(setOwnerSession)
      .catch(() => {});
  }, [currentUser?.id]);

  useEffect(() => {
    if (!modelSettings) return;
    try {
      setShowModelOnboarding(
        window.localStorage.getItem("scriptagent:model-onboarding-v1") !==
          "done",
      );
    } catch {
      setShowModelOnboarding(false);
    }
  }, [Boolean(modelSettings)]);

  useEffect(() => {
    window.localStorage.setItem(
      "scriptagent:debug-panel",
      showDebugPanel ? "true" : "false",
    );
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
    if (
      !videos.some((item) =>
        ["pending", "submitted", "running"].includes(item.status),
      )
    )
      return undefined;
    const timer = window.setInterval(
      () => refreshVideos().catch(() => {}),
      3000,
    );
    return () => window.clearInterval(timer);
  }, [videos.map((item) => `${item.id}:${item.status}`).join("|")]);

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
    if (
      !typingMessage ||
      typingMessage.visible.length >= typingMessage.content.length
    ) {
      return undefined;
    }
    const timer = window.setTimeout(() => {
      setTypingMessage((current) => {
        if (!current) return current;
        const nextLength = Math.min(
          current.content.length,
          current.visible.length + typingStep(current.content.length),
        );
        return { ...current, visible: current.content.slice(0, nextLength) };
      });
    }, 18);
    return () => window.clearTimeout(timer);
  }, [typingMessage]);

  useEffect(() => {
    if (
      !typingMessage ||
      typingMessage.visible.length < typingMessage.content.length
    ) {
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

  async function handleCreateVideo(input) {
    setError("");
    setIsCreatingVideo(true);
    try {
      await createVideo(input);
      await refreshVideos();
      return true;
    } catch (err) {
      setError(err.message);
      return false;
    } finally {
      setIsCreatingVideo(false);
    }
  }

  async function handleRetryVideo(id) {
    setError("");
    try {
      await retryVideo(id);
      await refreshVideos();
    } catch (err) {
      setError(err.message);
    }
  }

  async function handleOpenVideoHistory(videoID, conversationID) {
    if (conversationID) {
      await handleSelectChat(conversationID);
      setView("chat");
      return;
    }
    window.open(
      `/api/videos/${videoID}/file`,
      "_blank",
      "noopener,noreferrer",
    );
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
    const thread = await getChat(id);
    setSelectedChat(thread);
    const sourceSpace = spaces.find(
      (space) => space.id === thread.conversation?.space_id,
    );
    setChatProductId(
      sourceSpace?.product_id || thread.conversation?.product_id || "",
    );
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
    setChatProductId("");
  }

  async function sendChatToAgent(
    content,
    productID,
    conversationID = "",
    attachment = null,
    spaceID = "",
  ) {
    content = content.trim();
    if (!content && !attachment) return;
    setError("");
    setIsSending(true);
    setIsChatThinking(true);
    setTypingMessage(null);
    setChatCitations([]);
    setChatAgentSteps([]);
    const displayContent = [
      content || "请分析我上传的素材。",
      attachment ? `附件：${attachment.name}` : "",
    ]
      .filter(Boolean)
      .join("\n\n");
    const tempUserMessage = {
      id: `temp-user-${Date.now()}`,
      role: "user",
      content: displayContent,
      created_at: new Date().toISOString(),
    };
    const continuingCurrentChat = Boolean(
      conversationID && selectedChat?.conversation?.id === conversationID,
    );
    const baseMessages = continuingCurrentChat
      ? selectedChat?.messages || []
      : [];
    setOptimisticChatMessages([...baseMessages, tempUserMessage]);
    setChatDraft("");
    setChatAttachment(null);
    let progressTimer;
    const refreshChatProgress = async () => {
      if (!conversationID) return;
      try {
        const progress = await getChatProgress(conversationID);
        setChatAgentSteps(progress.steps || []);
      } catch {
        // Progress is optional; the message request remains authoritative.
      }
    };
    try {
      const messageRequest = conversationID
        ? sendChatMessage(conversationID, content, productID, attachment)
        : sendNewChatMessage(content, productID, attachment, spaceID);
      if (conversationID) {
        progressTimer = window.setInterval(refreshChatProgress, 250);
      }
      const thread = await messageRequest;
      const assistantMessage = lastAssistantMessage(thread.messages);
      setSelectedChat(thread);
      setChatCitations(thread.citations || []);
      setChatAgentSteps(thread.agent_steps || []);
      if (assistantMessage) {
        setOptimisticChatMessages(
          thread.messages.filter(
            (message) => message.id !== assistantMessage.id,
          ),
        );
        setTypingMessage({ ...assistantMessage, visible: "" });
      } else {
        setOptimisticChatMessages(null);
      }
      await refreshChats(thread.conversation.id);
      if (showDebugPanel) {
        await refreshModelCalls();
      }
    } catch (err) {
      setError("");
      setChatAttachment(attachment);
      setTypingMessage(null);
      setOptimisticChatMessages((current) => [
        ...(current || [tempUserMessage]),
        {
          id: `temp-error-${Date.now()}`,
          role: "assistant",
          content: `暂时无法生成：${err.message}`,
          created_at: new Date().toISOString(),
        },
      ]);
    } finally {
      if (progressTimer) window.clearInterval(progressTimer);
      setIsChatThinking(false);
      setIsSending(false);
    }
  }

  async function handleSendChat(event) {
    event.preventDefault();
    const conversationID = selectedChat?.conversation?.id || "";
    if (conversationID && isDirectVideoCommand(chatDraft)) {
      const source = lastAssistantMessage(selectedChat?.messages || []);
      const prompt = visibleAssistantContent(source?.content || "");
      if (!prompt) {
        setError("上文还没有可用于生成视频的提示词");
        return;
      }
      const inferred = videoParametersFromPrompt(prompt);
      let sourceAssetID = "";
      let mode = "text";
      if (chatProductId && inferred.cid) {
        try {
          const assets = await listProductAssets(chatProductId);
          const asset = (Array.isArray(assets) ? assets : []).find(
            (item) => item.id === inferred.cid,
          );
          if (asset) {
            sourceAssetID = asset.id;
            mode = asset.kind;
          }
        } catch {
          // The prompt remains usable without a reference asset.
        }
      }
      setChatDraft("");
      const created = await handleCreateVideo({
        product_id: chatProductId,
        conversation_id: conversationID,
        space_id: selectedChat?.conversation?.space_id || "",
        source_asset_id: sourceAssetID,
        source_asset_ids: sourceAssetID ? [sourceAssetID] : [],
        mode,
        prompt,
        negative_prompt:
          "广告感过强，棚拍感，过度磨皮，文字乱码，水印，品牌标识变形，产品外观不一致",
        model: "wan3.0-video-prime",
        resolution: "720P",
        ratio: inferred.ratio,
        duration: inferred.duration,
        sound_enabled: true,
      });
      if (created) await handleSelectChat(conversationID);
      return;
    }
    await sendChatToAgent(
      chatDraft,
      chatProductId,
      conversationID,
      chatAttachment,
      selectedChat?.conversation?.space_id || "",
    );
  }

  async function handleStartAgentChat(productID, goal) {
    setView("chat");
    setSelectedChat(null);
    setChatProductId(productID || "");
    await sendChatToAgent(goal, productID || "");
  }

  async function handleStartSpaceAgent(space) {
    if (!space) return;
    setError("");
    setView("chat");
    setSelectedChat({
      conversation: {
        space_id: space.id,
        product_id: space.product_id || "",
        title: space.title,
      },
      messages: [],
    });
    setChatProductId(space.product_id || "");
    setOptimisticChatMessages(null);
    setTypingMessage(null);
    setChatCitations([]);
    setChatAgentSteps([]);
    setIsChatThinking(false);
    setChatAttachment(null);
    setChatDraft("");
  }

  async function handleCreateProduct(eventOrForm) {
    const isFormData = Boolean(
      eventOrForm &&
      typeof eventOrForm.append === "function" &&
      typeof eventOrForm.get === "function",
    );
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
      const profiles = modelCapabilityCatalog.map((item) => ({
        capability: item.id,
        mode: String(form.get(`${item.id}_mode`) || "byok"),
        api_key: String(form.get(`${item.id}_api_key`) || ""),
        provider: String(form.get(`${item.id}_provider`) || "dashscope"),
        endpoint: String(form.get(`${item.id}_endpoint`) || item.endpoint),
        model: String(form.get(`${item.id}_model`) || item.models[0].id),
      }));
      const next = await saveModelSettings({ profiles });
      setModelSettings(next);
    } catch (err) {
      setError(err.message);
    } finally {
      setIsSavingSettings(false);
    }
  }

  const visibleContent =
    activeTab === "video_prompts"
      ? videoPromptContent
      : selectedJob?.[activeTab] || "";
  const canRetry = selectedJob && !runningStatuses.has(selectedJob.status);
  const selectedCall =
    modelCalls.find((call) => call.id === selectedCallId) || modelCalls[0];
  const productStats = useMemo(
    () => buildProductStats(products, jobs),
    [products, jobs],
  );

  function handleStartProductJob(productId, requirement = "", spaceId = "") {
    setSelectedProductId(productId);
    setJobInitialRequirement(requirement);
    window.sessionStorage.setItem("scriptagent:space-id", spaceId);
    setView("jobs");
  }

  async function handleCreateCreativeReport(event) {
    event.preventDefault();
    const selectedProduct = products.find(
      (product) => product.id === selectedProductId,
    );
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
    setError("");
    setIsSavingProduct(true);
    try {
      await updateProduct(id, { title, content });
      await refreshProducts(id);
      setProductPreview(await getProductMarkdown(id));
    } catch (err) {
      setError(err.message);
      throw err;
    } finally {
      setIsSavingProduct(false);
    }
  }
  async function handleCreateSpace(event) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setError("");
    setIsCreatingSpace(true);
    const formEl = event.currentTarget;
    try {
      await createSpace({
        title: String(form.get("title") || ""),
        summary: String(form.get("summary") || ""),
        product_id: String(form.get("product_id") || ""),
        agent_brief: String(form.get("agent_brief") || ""),
        marketing_goal: String(form.get("marketing_goal") || "conversion"),
        goal_stage: String(form.get("goal_stage") || "action"),
      });
      formEl.reset();
      await refreshSpaces();
      return true;
    } catch (err) {
      setError(err.message);
      return false;
    } finally {
      setIsCreatingSpace(false);
    }
  }

  async function handleUpdateSpace(id, input) {
    setError("");
    const updated = await updateSpace(id, input);
    await refreshSpaces();
    if (selectedChat?.conversation?.space_id === id)
      setChatProductId(updated.product_id || "");
    return updated;
  }

  async function handleDeleteSpace(id) {
    setError("");
    try {
      await deleteSpace(id);
      await refreshSpaces();
      if (selectedChat?.conversation?.space_id === id) handleNewChat();
    } catch (err) {
      setError(err.message);
      throw err;
    }
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

  function handleCreateIntelligenceTask(space, signal) {
    setError("");
    setView("chat");
    setSelectedChat({
      conversation: {
        space_id: space.id,
        product_id: space.product_id || "",
        title: space.title,
      },
      messages: [],
    });
    setChatProductId(space.product_id || "");
    setOptimisticChatMessages(null);
    setTypingMessage(null);
    setChatCitations([]);
    setChatAgentSteps([]);
    setIsChatThinking(false);
    setChatAttachment(null);
    setChatDraft(
      `基于创意情报“${signal.title}”设计下一轮可验证的素材实验。证据：${signal.summary}`,
    );
  }

  if (isCheckingAuth)
    return (
      <div className="auth-shell">
        <div className="auth-card auth-loading">
          <img src={agentIcon} alt="" />
          <Loader2 className="spin" size={20} />
          <p>正在检查登录状态</p>
        </div>
      </div>
    );
  if (!currentUser)
    return (
      <AuthGate
        error={authError}
        registrationAvailable={registrationAvailable}
        onSubmit={handleAuthSubmit}
      />
    );

  return (
    <div className="app-shell agent-app-shell">
      {showModelOnboarding ? (
        <ModelOnboarding
          modelSettings={modelSettings}
          isSaving={isSavingSettings}
          error={error}
          onSave={handleModelOnboardingSave}
          onLater={finishModelOnboarding}
          onSettings={() => {
            finishModelOnboarding();
            setView("settings");
          }}
        />
      ) : null}
      <AppSidebar
        view={view}
        onChange={setView}
        icon={agentIcon}
        collapsed={isMainSidebarCollapsed}
        onToggle={() => setIsMainSidebarCollapsed((value) => !value)}
        showDebugPanel={showDebugPanel}
        ownerAuthenticated={ownerSession.authenticated}
        currentUser={currentUser}
        onLogout={handleLogout}
      />
      <div className="agent-frame">
        <main
          className={`workspace ${view === "products" || view === "calls" || view === "admin" ? "workspace-home" : ""} ${view === "chat" && isChatHistoryCollapsed ? "chat-history-collapsed" : ""}`}
        >
          {view === "jobs" ? (
            <JobsSidebar
              jobs={jobs}
              selectedJob={selectedJob}
              onSelect={handleSelectJob}
            />
          ) : null}
          {view === "chat" ? (
            <ChatSidebar
              chats={chats}
              selectedChat={selectedChat}
              collapsed={isChatHistoryCollapsed}
              onSelect={handleSelectChat}
              onNew={handleNewChat}
              onToggle={() =>
                setIsChatHistoryCollapsed(!isChatHistoryCollapsed)
              }
            />
          ) : null}
          {view === "settings" ? (
            <SettingsSidebar modelSettings={modelSettings} />
          ) : null}

          <section
            className={`main-pane ${view === "products" || view === "calls" || view === "admin" ? "main-pane-home" : ""}`}
          >
            {view === "home" ? (
              <AgentStart
                products={products}
                jobs={jobs}
                spaces={spaces}
                suggestions={suggestions}
                isSending={isSending}
                onSend={handleStartAgentChat}
                onSpaces={() => setView("spaces")}
                onHistory={() => setView("history")}
                onSuggestion={handleSuggestion}
              />
            ) : null}
            {view === "history" ? (
              <HistoryWorkspace
                jobs={jobs}
                chats={chats}
                videos={videos}
                products={products}
                onJob={(id) => handleSelectJob(id).then(() => setView("jobs"))}
                onChat={(id) =>
                  handleSelectChat(id).then(() => setView("chat"))
                }
                onVideo={handleOpenVideoHistory}
              />
            ) : null}
            {view === "spaces" ? (
              <SpacesWorkspace
                spaces={spaces}
                products={products}
                jobs={jobs}
                suggestions={suggestions}
                isCreating={isCreatingSpace}
                isSending={isSending}
                error={error}
                onCreate={handleCreateSpace}
                onUpdate={handleUpdateSpace}
                onDelete={handleDeleteSpace}
                onStart={handleStartSpaceAgent}
                onSuggestion={handleSuggestion}
              />
            ) : null}
            {view === "intelligence" ? (
              <IntelligenceWorkspace
                spaces={spaces}
                onCreateTask={handleCreateIntelligenceTask}
              />
            ) : null}
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
                videos={videos}
                isCreatingVideo={isCreatingVideo}
                onDraft={setChatDraft}
                onAttachment={setChatAttachment}
                onProduct={setChatProductId}
                sourceSpace={spaces.find(
                  (space) => space.id === selectedChat?.conversation?.space_id,
                )}
                historyCollapsed={isChatHistoryCollapsed}
                onHistoryToggle={() => setIsChatHistoryCollapsed(false)}
                onEditSpace={() => setView("spaces")}
                onCreateSkill={handleSaveSkill}
                onCreateVideo={handleCreateVideo}
                onRetryVideo={handleRetryVideo}
                onSend={handleSendChat}
              />
            ) : null}
            {view === "calls" && showDebugPanel ? (
              <ModelCallsWorkspace
                calls={modelCalls}
                jobs={jobs}
                spaces={spaces}
                error={error}
              />
            ) : null}
            {view === "admin" && ownerSession.authenticated ? (
              <OwnerDashboard
                overview={ownerOverview}
                isLoading={isOwnerLoading}
                error={error}
                onRefresh={() =>
                  refreshCurrent().catch((err) => setError(err.message))
                }
                onLogout={handleOwnerLogout}
              />
            ) : null}
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
        </main>
      </div>
    </div>
  );
}

function AuthGate({ error, registrationAvailable, onSubmit }) {
  const [mode, setMode] = useState("login");
  const [isSubmitting, setIsSubmitting] = useState(false);

  useEffect(() => {
    if (!registrationAvailable) setMode("login");
  }, [registrationAvailable]);

  async function submit(event) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setIsSubmitting(true);
    await onSubmit(mode, {
      name: String(form.get("name") || ""),
      email: String(form.get("email") || ""),
      password: String(form.get("password") || ""),
    });
    setIsSubmitting(false);
  }

  return (
    <main className="auth-shell">
      <section className="auth-card" aria-labelledby="auth-title">
        <div className="auth-brand">
          <img src={agentIcon} alt="" />
          <div>
            <strong>ScriptAgent</strong>
            <span>让每次创作都有目标、有依据</span>
          </div>
        </div>
        <div className="auth-heading">
          <span className="eyebrow">
            {mode === "register" ? "首次设置" : "欢迎回来"}
          </span>
          <h1 id="auth-title">
            {mode === "register" ? "创建管理员账号" : "登录工作台"}
          </h1>
          <p>
            {mode === "register"
              ? "创建首个账号，保护当前工作区与模型配置。"
              : "登录后继续使用你的创意空间和产品资料。"}
          </p>
        </div>
        {registrationAvailable ? (
          <div className="auth-tabs" role="tablist">
            <button
              type="button"
              className={mode === "login" ? "active" : ""}
              onClick={() => setMode("login")}
            >
              登录
            </button>
            <button
              type="button"
              className={mode === "register" ? "active" : ""}
              onClick={() => setMode("register")}
            >
              注册
            </button>
          </div>
        ) : null}
        <form className="auth-form" onSubmit={submit}>
          {mode === "register" ? (
            <label>
              <span>称呼</span>
              <input name="name" autoComplete="name" placeholder="怎么称呼你" />
            </label>
          ) : null}
          <label>
            <span>邮箱</span>
            <div className="auth-input">
              <Mail size={18} />
              <input
                name="email"
                type="email"
                autoComplete="email"
                placeholder="name@example.com"
                required
              />
            </div>
          </label>
          <label>
            <span>密码</span>
            <div className="auth-input">
              <ShieldCheck size={18} />
              <input
                name="password"
                type="password"
                autoComplete={
                  mode === "register" ? "new-password" : "current-password"
                }
                placeholder="至少 8 位"
                minLength={8}
                required
              />
            </div>
          </label>
          {error ? (
            <div className="auth-error" role="alert">
              {error}
            </div>
          ) : null}
          <button
            className="primary auth-submit"
            type="submit"
            disabled={isSubmitting}
          >
            {isSubmitting ? (
              <>
                <Loader2 className="spin" size={18} />
                处理中
              </>
            ) : mode === "register" ? (
              "创建账号并进入"
            ) : (
              "登录"
            )}
          </button>
        </form>
        <p className="auth-note">
          每个账号拥有独立的产品资料、创作空间、对话与模型配置。
        </p>
      </section>
    </main>
  );
}

function AppSidebar({
  view,
  onChange,
  icon,
  collapsed,
  onToggle,
  showDebugPanel,
  ownerAuthenticated,
  currentUser,
  onLogout,
}) {
  const items = [
    ["home", "开始", House],
    ["history", "历史", History],
    ["spaces", "创意空间", FolderKanban],
    ["intelligence", "创意雷达", Radar],
    ["products", "产品资料", Package],
  ];
  const navButton = (id, label, Icon) => (
    <button
      key={id}
      className={view === id ? "active" : ""}
      type="button"
      title={collapsed ? label : undefined}
      aria-label={label}
      onClick={() => onChange(id)}
    >
      <Icon size={19} />
      <span>{label}</span>
    </button>
  );
  return (
    <aside className={`app-sidebar ${collapsed ? "collapsed" : ""}`}>
      <button
        className="agent-brand"
        type="button"
        onClick={onToggle}
        title={collapsed ? "展开主菜单" : "收起主菜单"}
        aria-label={collapsed ? "展开主菜单" : "收起主菜单"}
      >
        <img src={icon} alt="" />
        <span>ScriptAgent</span>
        {collapsed ? null : (
          <ChevronLeft className="sidebar-collapse-mark" size={16} />
        )}
      </button>
      <nav>{items.map(([id, label, Icon]) => navButton(id, label, Icon))}</nav>
      <div className="sidebar-bottom">
        {ownerAuthenticated
          ? navButton("admin", "运营后台", ChartNoAxesCombined)
          : null}
        {showDebugPanel ? navButton("calls", "开发者模式", Activity) : null}
        {navButton("settings", "设置", Settings)}
        <div className="sidebar-account">
          <span className="account-avatar">
            {(currentUser?.name || currentUser?.email || "A")
              .slice(0, 1)
              .toUpperCase()}
          </span>
          <span className="account-copy">
            <strong>{currentUser?.name || "管理员"}</strong>
            <small>{currentUser?.email}</small>
          </span>
          <button
            className="account-logout"
            type="button"
            onClick={onLogout}
            title="退出登录"
            aria-label="退出登录"
          >
            <LogOut size={17} />
          </button>
        </div>
      </div>
    </aside>
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
              <span className={`status-pill ${statusTone(job.status)}`}>
                {statusLabel(job.status)}
              </span>
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
          <span
            className={`status-pill ${modelSettings?.configured ? "success" : "danger"}`}
          >
            {modelSettings?.configured ? "模型已配置" : "模型未配置"}
          </span>
          <p>{modelSettings?.profiles?.length || 0} 类能力已设置</p>
        </div>
      </div>
    </aside>
  );
}

function ChatSidebar({
  chats,
  selectedChat,
  collapsed,
  onSelect,
  onNew,
  onToggle,
}) {
  if (collapsed) {
    return null;
  }
  return (
    <aside className="history-pane chat-history-pane">
      <div className="pane-heading split">
        <span>
          <MessageSquare size={16} />
          <b>对话记录</b>
        </span>
        <div className="chat-history-actions">
          <button className="mini-button" type="button" onClick={onNew}>
            新对话
          </button>
          <button
            className="history-toggle"
            type="button"
            onClick={onToggle}
            title={collapsed ? "展开历史对话" : "收起历史对话"}
            aria-label={collapsed ? "展开历史对话" : "收起历史对话"}
          >
            <ChevronLeft size={17} />
          </button>
        </div>
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
              <time className="history-meta">
                {formatChatTime(chat.updated_at)}
              </time>
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
              <span className="history-meta">
                {call.scope} · {call.total_tokens || 0} tokens
              </span>
              <span
                className={`status-pill ${call.error_message ? "danger" : "success"}`}
              >
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
  const [selectedProductID, setSelectedProductID] = useState(
    props.initialProductId || "",
  );
  const [requirementDraft, setRequirementDraft] = useState(
    props.initialRequirement || "",
  );

  useEffect(() => {
    setSelectedProductID(props.initialProductId || "");
  }, [props.initialProductId]);

  useEffect(() => {
    setRequirementDraft(props.initialRequirement || "");
  }, [props.initialRequirement]);

  function handleFissionCountChange(event) {
    const next = Number.parseInt(event.target.value, 10);
    setFissionCount(
      Number.isFinite(next) ? Math.min(20, Math.max(1, next)) : 1,
    );
  }

  return (
    <>
      <section className="composer">
        <div>
          <h1>创建脚本任务</h1>
          <p>先确认产品资产，再上传参考视频生成复刻脚本与裂变脚本。</p>
        </div>
        <form className="task-form" onSubmit={props.onCreate}>
          <input
            type="hidden"
            name="space_id"
            value={window.sessionStorage.getItem("scriptagent:space-id") || ""}
          />
          <div className="form-section">
            <div className="section-heading">
              <span>1. 产品资产</span>
              <small>产品信息决定脚本是否有卖点和细节</small>
            </div>
            <label>
              <span>选择产品</span>
              <select
                name="product_id"
                value={selectedProductID}
                onChange={(event) => setSelectedProductID(event.target.value)}
              >
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
                  <input
                    name="product_title"
                    type="text"
                    placeholder="默认使用 Markdown 文件名"
                  />
                </label>
                <FileField
                  name="product_md"
                  label="产品 Markdown"
                  accept=".md,.markdown,text/markdown"
                  icon={<FileText size={16} />}
                  required={!selectedProductID}
                />
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
                <input
                  name="title"
                  type="text"
                  placeholder="默认使用产品 Markdown 文件名"
                />
              </label>
              <FileField
                name="video"
                label="参考视频"
                accept="video/mp4,video/quicktime,video/webm"
                icon={<Video size={16} />}
              />
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
                <input
                  name="fission_count"
                  type="number"
                  min="1"
                  max="20"
                  value={fissionCount}
                  onChange={handleFissionCountChange}
                />
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
            <button
              className="primary-button"
              type="submit"
              disabled={props.isCreating}
            >
              {props.isCreating ? (
                <Loader2 className="spin" size={16} />
              ) : (
                <Play size={16} />
              )}
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
            <button
              key={key}
              className={props.activeTab === key ? "active" : ""}
              type="button"
              onClick={() => props.onTab(key)}
            >
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

function ChatWorkspace({
  thread,
  optimisticMessages,
  typingMessage,
  citations,
  agentSteps,
  isThinking,
  draft,
  products,
  skills,
  selectedProductId,
  sourceSpace,
  historyCollapsed,
  isSending,
  error,
  attachment,
  videos,
  isCreatingVideo,
  onDraft,
  onAttachment,
  onProduct,
  onCreateSkill,
  onCreateVideo,
  onRetryVideo,
  onHistoryToggle,
  onEditSpace,
  onSend,
}) {
  const messages = optimisticMessages || thread?.messages || [];
  const messagesRef = useRef(null);
  const [showSkillMenu, setShowSkillMenu] = useState(false);
  const [showVideoComposer, setShowVideoComposer] = useState(false);
  const [isListening, setIsListening] = useState(false);
  const [voiceError, setVoiceError] = useState("");
  const recognitionRef = useRef(null);
  const voiceBaseRef = useRef("");
  const SpeechRecognition =
    typeof window !== "undefined"
      ? window.SpeechRecognition || window.webkitSpeechRecognition
      : null;

  useEffect(
    () => () => {
      if (recognitionRef.current) recognitionRef.current.abort();
    },
    [],
  );

  function toggleVoiceInput() {
    if (!SpeechRecognition) {
      setVoiceError("当前浏览器不支持语音输入，请使用最新版 Chrome 或 Edge。");
      return;
    }
    if (isListening && recognitionRef.current) {
      recognitionRef.current.stop();
      return;
    }
    const recognition = new SpeechRecognition();
    recognition.lang = "zh-CN";
    recognition.continuous = true;
    recognition.interimResults = true;
    voiceBaseRef.current = draft.trimEnd();
    recognition.onstart = () => {
      setIsListening(true);
      setVoiceError("");
    };
    recognition.onresult = (event) => {
      let transcript = "";
      for (let index = 0; index < event.results.length; index += 1)
        transcript += event.results[index][0].transcript;
      const base = voiceBaseRef.current;
      onDraft(`${base}${base && transcript ? " " : ""}${transcript}`);
    };
    recognition.onerror = (event) => {
      const messages = {
        "not-allowed": "没有麦克风权限，请在浏览器中允许访问。",
        "audio-capture": "未检测到可用麦克风。",
        "no-speech": "没有识别到语音，请重试。",
        network: "语音识别网络不可用，请稍后重试。",
      };
      setVoiceError(messages[event.error] || "语音识别失败，请重试。");
      setIsListening(false);
    };
    recognition.onend = () => {
      setIsListening(false);
      recognitionRef.current = null;
    };
    recognitionRef.current = recognition;
    recognition.start();
  }
  let lastAssistantIndex = -1;
  messages.forEach((message, index) => {
    if (message.role === "assistant") lastAssistantIndex = index;
  });
  const showCompletedTrace = Boolean(
    agentSteps?.length && !isThinking && !typingMessage,
  );
  const conversationId = thread?.conversation?.id || "";
  const conversationVideos = conversationId
    ? (Array.isArray(videos) ? videos : []).filter(
        (item) => item.conversation_id === conversationId,
      )
    : [];

  useEffect(() => {
    const messageList = messagesRef.current;
    if (messageList) messageList.scrollTop = messageList.scrollHeight;
  }, [
    messages.length,
    conversationVideos.length,
    isThinking,
    typingMessage?.visible,
  ]);

  function handleSkill(skill) {
    onDraft(skill.invocation_prompt || `调用 ${skill.name} skill。`);
    setShowSkillMenu(false);
  }

  return (
    <section className="chat-pane">
      <div className="chat-context-bar">
        {historyCollapsed ? (
          <button
            className="chat-history-inline"
            type="button"
            onClick={onHistoryToggle}
            title="展开历史对话"
            aria-label="展开历史对话"
          >
            <MessageSquare size={18} />
          </button>
        ) : null}
        <label className="chat-product-select">
          <Package size={15} />
          <span>产品资料</span>
          <select
            value={selectedProductId}
            disabled={Boolean(sourceSpace)}
            onChange={(event) => onProduct(event.target.value)}
          >
            <option value="">未选择</option>
            {products.map((product) => (
              <option key={product.id} value={product.id}>
                {product.title}
              </option>
            ))}
          </select>
          {sourceSpace ? (
            <>
              <span className="space-context-lock">
                由「{sourceSpace.title}」管理
              </span>
              <button
                className="space-context-edit"
                type="button"
                onClick={onEditSpace}
              >
                回空间修改
              </button>
            </>
          ) : null}
        </label>
      </div>
      <div className="chat-messages" ref={messagesRef}>
        {messages.length ||
        conversationVideos.length ||
        isThinking ||
        typingMessage ? (
          <>
            {citations?.length ? <CitationPanel citations={citations} /> : null}
            {messages.map((message, index) => {
              const persistedSteps =
                thread?.agent_traces?.[message.id] || [];
              const traceSteps = persistedSteps.length
                ? persistedSteps
                : showCompletedTrace && index === lastAssistantIndex
                  ? agentSteps
                  : [];
              return (
                <Fragment key={message.id}>
                  {message.role === "assistant" && traceSteps.length ? (
                    <AgentExecutionTrace steps={traceSteps} />
                  ) : null}
                  <ChatMessageBubble message={message} />
                </Fragment>
              );
            })}
            {isThinking || typingMessage ? (
              <AgentExecutionTrace
                steps={agentSteps}
                isRunning={isThinking}
                hasProduct={Boolean(selectedProductId)}
                hasAttachment={Boolean(attachment)}
              />
            ) : null}
            {typingMessage ? (
              <ChatMessageBubble
                message={{ ...typingMessage, content: typingMessage.visible }}
                isTyping
              />
            ) : null}
            {conversationVideos.length ? (
              <ChatVideoResults
                videos={conversationVideos}
                onRetry={onRetryVideo}
              />
            ) : null}
          </>
        ) : (
          <ChatTaskStarter onSelect={onDraft} />
        )}
      </div>
      <form className="chat-form" onSubmit={onSend}>
        {error ? <div className="error-banner">{error}</div> : null}
        {showSkillMenu ? (
          <SkillCommandMenu
            skills={skills || []}
            onSelect={handleSkill}
            onCreate={onCreateSkill}
            onClose={() => setShowSkillMenu(false)}
          />
        ) : null}
        {showVideoComposer ? (
          <VideoComposerPanel
            productId={selectedProductId}
            conversationId={conversationId}
            spaceId={sourceSpace?.id || thread?.conversation?.space_id || ""}
            draft={draft}
            onCreate={onCreateVideo}
            isCreating={isCreatingVideo}
            onClose={() => setShowVideoComposer(false)}
          />
        ) : null}
        {attachment ? (
          <AttachmentPreview
            attachment={attachment}
            onRemove={() => onAttachment(null)}
          />
        ) : null}
        <textarea
          value={draft}
          onChange={(event) => onDraft(event.target.value)}
          rows="3"
          placeholder="发消息或创建任务… / 使用技能，添加素材"
        />
        <div className="chat-composer-toolbar">
          <div className="composer-tools">
            <button
              className={`composer-tool voice-input ${isListening ? "active listening" : ""}`}
              type="button"
              onClick={toggleVoiceInput}
              title={isListening ? "停止语音输入" : "语音输入"}
              aria-label={isListening ? "停止语音输入" : "开始语音输入"}
              aria-pressed={isListening}
            >
              {isListening ? <MicOff size={16} /> : <Mic size={16} />}
              <span>{isListening ? "正在听…" : "语音"}</span>
            </button>
            <button
              className={`composer-tool ${showSkillMenu ? "active" : ""}`}
              type="button"
              onClick={() => setShowSkillMenu((value) => !value)}
            >
              <Sparkles size={16} />
              <span>技能</span>
              <ChevronRight size={14} />
            </button>
            <label
              className={`composer-tool composer-connector ${attachment ? "active" : ""}`}
              title="添加图片或视频素材"
            >
              <Upload size={15} />
              <span>{attachment ? "已添加素材" : "素材"}</span>
              <ChevronRight size={14} />
              <input
                type="file"
                accept="image/png,image/jpeg,image/webp,image/gif,video/mp4,video/quicktime,video/webm"
                onChange={(event) =>
                  onAttachment(event.target.files?.[0] || null)
                }
              />
            </label>
            <button
              className={`composer-tool ${showVideoComposer ? "active" : ""}`}
              type="button"
              onClick={() => {
                setShowSkillMenu(false);
                setShowVideoComposer((value) => !value);
              }}
            >
              <Video size={16} />
              <span>生成视频</span>
              <ChevronRight size={14} />
            </button>
          </div>
          <button
            className="composer-send"
            type="submit"
            disabled={isSending || (!draft.trim() && !attachment)}
            aria-label={isSending ? "发送中" : "发送"}
          >
            {isSending ? (
              <Loader2 className="spin" size={18} />
            ) : (
              <Send size={18} />
            )}
          </button>
        </div>
        {voiceError ? (
          <div className="voice-input-message" role="status">
            {voiceError}
          </div>
        ) : isListening ? (
          <div className="voice-input-message listening" role="status">
            <span />
            正在识别，点击“正在听”即可停止
          </div>
        ) : null}
      </form>
    </section>
  );
}

const videoStatusText = {
  pending: "准备生成",
  submitted: "已提交",
  running: "生成中",
  completed: "已完成",
  failed: "生成失败",
};

function VideoComposerPanel({
  productId,
  conversationId,
  spaceId,
  draft,
  onCreate,
  isCreating,
  onClose,
}) {
  const [mode, setMode] = useState("text");
  const [assets, setAssets] = useState([]);
  const [sourceAssetIds, setSourceAssetIds] = useState([]);
  const [prompt, setPrompt] = useState(draft || "");
  const [resolution, setResolution] = useState("720P");
  const [duration, setDuration] = useState(5);
  const [ratio, setRatio] = useState("9:16");
  const [soundEnabled, setSoundEnabled] = useState(true);

  useEffect(() => {
    if (!productId) {
      setAssets([]);
      setSourceAssetIds([]);
      return undefined;
    }
    let active = true;
    listProductAssets(productId)
      .then((items) => {
        if (!active) return;
        setAssets(
          (Array.isArray(items) ? items : []).filter(
            (item) =>
              item.kind === "image" ||
              (item.kind === "video" &&
                /\.(mp4|mov)$/i.test(item.original_name)),
          ),
        );
      })
      .catch(() => {
        if (active) setAssets([]);
      });
    return () => {
      active = false;
    };
  }, [productId]);

  async function generate() {
    const selectedAssets = sourceAssetIds
      .map((id) => assets.find((asset) => asset.id === id))
      .filter(Boolean);
    const references = selectedAssets.map(
      (asset, index) =>
        `${asset.kind === "image" ? "图" : "视频"}${index + 1}（${asset.original_name}）`,
    );
    const referencedPrompt = references.length
      ? `参考素材对应关系：${references.join("、")}。\n${prompt.trim()}`
      : prompt.trim();
    const created = await onCreate({
      product_id: productId,
      conversation_id: conversationId,
      space_id: spaceId,
      source_asset_id: mode === "text" ? "" : sourceAssetIds[0] || "",
      source_asset_ids: mode === "text" ? [] : sourceAssetIds,
      mode,
      prompt: referencedPrompt,
      negative_prompt:
        "广告感过强，棚拍感，过度磨皮，文字乱码，水印，品牌标识变形，产品外观不一致",
      model: "wan3.0-video-prime",
      resolution,
      ratio,
      duration: Number(duration),
      sound_enabled: soundEnabled,
    });
    if (created) onClose();
  }

  const visibleAssets = assets.filter((asset) => asset.kind === mode);
  const selectionLimit = mode === "image" ? 10 : 5;
  const canGenerate =
    prompt.trim() && (mode === "text" || sourceAssetIds.length) && !isCreating;

  function toggleAsset(assetId) {
    setSourceAssetIds((current) => {
      if (current.includes(assetId)) {
        return current.filter((id) => id !== assetId);
      }
      if (current.length >= selectionLimit) return current;
      return [...current, assetId];
    });
  }
  return (
    <section className="video-composer-panel">
      <div className="video-composer-head">
        <div>
          <strong>生成视频</strong>
          <small>写下想要的画面和动作</small>
        </div>
        <button type="button" onClick={onClose} aria-label="关闭视频设置">
          <X size={17} />
        </button>
      </div>
      <div className="ugc-mode-switch">
        <button
          type="button"
          className={mode === "text" ? "active" : ""}
          onClick={() => {
            setMode("text");
            setSourceAssetIds([]);
          }}
        >
          纯文本
        </button>
        <button
          type="button"
          className={mode === "image" ? "active" : ""}
          onClick={() => {
            setMode("image");
            setSourceAssetIds([]);
          }}
        >
          图片参考
        </button>
        <button
          type="button"
          className={mode === "video" ? "active" : ""}
          onClick={() => {
            setMode("video");
            setSourceAssetIds([]);
          }}
        >
          视频参考
        </button>
      </div>
      {mode !== "text" ? (
        <div className="ugc-reference compact">
          <span>
            从当前产品资料选择{mode === "image" ? "图片" : "视频"} · 已选 {sourceAssetIds.length}/{selectionLimit}
          </span>
          {visibleAssets.length ? (
            <div className="ugc-asset-strip">
              {visibleAssets.map((asset) => (
                <button
                  type="button"
                  className={sourceAssetIds.includes(asset.id) ? "active" : ""}
                  onClick={() => toggleAsset(asset.id)}
                  key={asset.id}
                >
                  {sourceAssetIds.includes(asset.id) ? (
                    <b className="asset-reference-index">
                      {mode === "image" ? "图" : "视频"}
                      {sourceAssetIds.indexOf(asset.id) + 1}
                    </b>
                  ) : null}
                  {asset.kind === "video" ? (
                    <video
                      src={`/api/assets/${asset.id}/file`}
                      muted
                      preload="metadata"
                    />
                  ) : (
                    <img
                      src={`/api/assets/${asset.id}/file`}
                      alt={asset.original_name}
                    />
                  )}
                  <small>{asset.original_name}</small>
                </button>
              ))}
            </div>
          ) : (
            <p>
              {productId
                ? `当前产品资料还没有可用${mode === "image" ? "图片" : "视频"}。`
                : "请先选择产品资料。"}
            </p>
          )}
        </div>
      ) : null}
      <label className="video-prompt">
        <span>提示词</span>
        <textarea
          value={prompt}
          onChange={(event) => setPrompt(event.target.value)}
          placeholder="描述人物、场景、动作、镜头运动、声音与画面风格…"
        />
        {sourceAssetIds.length ? (
          <small className="video-reference-help">
            可在提示词中直接使用“{mode === "image" ? "图1、图2" : "视频1、视频2"}”；提交时会自动附上素材对应关系。
          </small>
        ) : null}
      </label>
      <div className="video-composer-options">
        <label>
          <span>分辨率</span>
          <select
            value={resolution}
            onChange={(event) => setResolution(event.target.value)}
          >
            <option value="480P">480P</option>
            <option value="720P">720P</option>
            <option value="1080P">1080P</option>
          </select>
        </label>
        <label>
          <span>时长</span>
          <select
            value={duration}
            onChange={(event) => setDuration(Number(event.target.value))}
          >
            {[2, 3, 4, 5, 6, 8, 10, 15, 20, 30].map((value) => (
              <option value={value} key={value}>
                {value} 秒
              </option>
            ))}
          </select>
        </label>
        <label>
          <span>画幅</span>
          <select
            value={ratio}
            onChange={(event) => setRatio(event.target.value)}
          >
            <option value="9:16">9:16 竖屏</option>
            <option value="16:9">16:9 横屏</option>
            <option value="1:1">1:1 方形</option>
            <option value="adaptive">跟随素材</option>
          </select>
        </label>
        <label className="video-sound">
          <input
            type="checkbox"
            checked={soundEnabled}
            onChange={(event) => setSoundEnabled(event.target.checked)}
          />
          <span>生成声音</span>
        </label>
      </div>
      <div className="video-composer-actions">
        <small>生成后可离开当前对话，任务会继续运行。</small>
        <button
          className="primary-button"
          type="button"
          disabled={!canGenerate}
          onClick={generate}
        >
          {isCreating ? (
            <>
              <Loader2 className="spin" size={15} />
              提交中
            </>
          ) : (
            <>
              <Video size={15} />
              生成
            </>
          )}
        </button>
      </div>
    </section>
  );
}

function ChatVideoResults({ videos, onRetry }) {
  return (
    <section className="chat-video-results">
      <div className="chat-video-results-title">
        <Video size={16} />
        <strong>视频结果</strong>
      </div>
      {videos.map((item) => (
        <article className="chat-video-card" key={item.id}>
          <div className="chat-video-preview">
            {item.status === "completed" ? (
              <video
                src={`/api/videos/${item.id}/file`}
                controls
                preload="metadata"
              />
            ) : (
              <div className={`ugc-video-state ${item.status}`}>
                {["pending", "submitted", "running"].includes(item.status) ? (
                  <Loader2 className="spin" size={22} />
                ) : (
                  <Video size={22} />
                )}
                <strong>{videoStatusText[item.status] || item.status}</strong>
                <small>
                  {item.error_message ||
                    (item.status === "running"
                      ? "可继续对话，完成后自动显示"
                      : `${item.duration} 秒 · ${item.resolution}`)}
                </small>
              </div>
            )}
          </div>
          <div className="chat-video-meta">
            <div>
              <strong>{item.prompt}</strong>
              <small>
                {item.resolution} · {item.duration} 秒 ·{" "}
                {item.sound_enabled ? "有声" : "无声"} ·{" "}
                {formatTime(item.created_at)}
              </small>
            </div>
            {item.status === "completed" ? (
              <a
                className="secondary-button"
                href={`/api/videos/${item.id}/file`}
                download={`ugc-${item.id}.mp4`}
              >
                下载
              </a>
            ) : item.status === "failed" ? (
              <button
                className="secondary-button"
                type="button"
                onClick={() => onRetry(item.id)}
              >
                重试
              </button>
            ) : null}
          </div>
        </article>
      ))}
    </section>
  );
}

function AttachmentPreview({ attachment, onRemove }) {
  const [previewURL, setPreviewURL] = useState("");
  const isVideo = attachment?.type?.startsWith("video/");

  useEffect(() => {
    if (!attachment) return undefined;
    const url = URL.createObjectURL(attachment);
    setPreviewURL(url);
    return () => URL.revokeObjectURL(url);
  }, [attachment]);

  return (
    <div className="attachment-preview">
      <div className="attachment-cover">
        {previewURL ? (
          isVideo ? (
            <video
              src={previewURL}
              muted
              playsInline
              preload="metadata"
              aria-label={attachment.name}
            />
          ) : (
            <img src={previewURL} alt={attachment.name} />
          )
        ) : null}
        {isVideo ? (
          <span className="attachment-video-badge">
            <Video size={13} />
            视频
          </span>
        ) : null}
      </div>
      <div className="attachment-meta">
        <strong>{attachment.name}</strong>
        <small>{isVideo ? "视频素材" : "图片素材"}</small>
      </div>
      <button type="button" onClick={onRemove}>
        移除
      </button>
    </div>
  );
}

function AgentExecutionTrace({ steps = [], isRunning = false }) {
  const [expanded, setExpanded] = useState(false);
  const completedActions = steps.map((step) => ({
    label: agentStepLabel(step),
    detail: step.error ? "此步骤未完成" : step.reason || "",
    status: step.status || "completed",
    error: Boolean(step.error),
  }));

  useEffect(() => {
    if (isRunning && steps.length > 0) setExpanded(true);
  }, [isRunning, steps.length]);

  const status = isRunning
    ? "处理中"
    : `已完成${completedActions.length > 1 ? ` · ${completedActions.length} 个步骤` : ""}`;

  return (
    <section
      className={`agent-execution-trace ${isRunning ? "running" : "complete"}`}
      aria-live="polite"
    >
      <button
        className="agent-trace-toggle"
        type="button"
        onClick={() => setExpanded((value) => !value)}
        aria-expanded={expanded}
      >
        <span>
          {isRunning ? (
            <Loader2 className="spin" size={15} />
          ) : (
            <CheckCircle2 size={15} />
          )}
          {status}
        </span>
        {completedActions.length ? (
          <ChevronDown className={expanded ? "expanded" : ""} size={15} />
        ) : null}
      </button>
      {expanded && completedActions.length ? (
        <ol className="agent-trace-steps">
          {completedActions.map((item, index) => {
            const state = item.error || item.status === "error"
              ? "error"
              : item.status === "running"
                ? "active"
                : "done";
            return (
              <li className={state} key={`${item.label}-${index}`}>
                <i />{" "}
                <div>
                  <strong>{item.label}</strong>
                  {item.detail ? <small>{item.detail}</small> : null}
                </div>
              </li>
            );
          })}
        </ol>
      ) : null}
    </section>
  );
}

function agentStepLabel(step) {
  if (step.kind === "model") return "判断下一步";
  if (step.kind === "final") return "整理并校验结果";
  return (
    {
      list_products: "检查可用产品资料",
      retrieve_product_sections: "检索相关产品内容",
      read_product_markdown: "读取产品完整资料",
      call_skill: "调用专业创作技能",
    }[step.tool] || "执行 Agent 工具"
  );
}

function ChatTaskStarter({ onSelect }) {
  return (
    <section className="chat-starter">
      <div>
        <h3>从一个具体任务开始</h3>
        <p>
          选一个常见任务，我会把问题放进输入框，你可以补充产品、平台或素材限制。
        </p>
      </div>
      <div className="starter-list">
        {chatQuickTasks.map((task) => (
          <button
            key={task.title}
            type="button"
            onClick={() => onSelect(task.description)}
          >
            <strong>{task.title}</strong>
            <span>{task.description}</span>
          </button>
        ))}
      </div>
    </section>
  );
}

function SkillCommandMenu({ skills, onSelect, onCreate, onClose }) {
  const [selectedSkill, setSelectedSkill] = useState(skills[0] || null);
  const [isCreating, setIsCreating] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [createError, setCreateError] = useState("");
  const [creatorPrompt, setCreatorPrompt] = useState("");
  const [editingSkillId, setEditingSkillId] = useState("");
  const [isGeneratingDraft, setIsGeneratingDraft] = useState(false);
  const [draft, setDraft] = useState({
    name: "",
    title: "",
    description: "",
    category: "自定义",
    invocation_prompt: "",
    content: "",
  });

  function updateDraft(key, value) {
    setDraft((current) => ({ ...current, [key]: value }));
  }

  async function saveSkill() {
    setCreateError("");
    setIsSaving(true);
    try {
      const skill = await onCreate(draft, editingSkillId);
      setSelectedSkill(skill);
      setIsCreating(false);
      setEditingSkillId("");
      setCreatorPrompt("");
      setDraft({
        name: "",
        title: "",
        description: "",
        category: "自定义",
        invocation_prompt: "",
        content: "",
      });
    } catch (err) {
      setCreateError(err.message);
    } finally {
      setIsSaving(false);
    }
  }

  function startCreate() {
    setCreateError("");
    setEditingSkillId("");
    setCreatorPrompt("");
    setDraft({
      name: "",
      title: "",
      description: "",
      category: "自定义",
      invocation_prompt: "",
      content: "",
    });
    setIsCreating(true);
  }

  async function createFromPrompt() {
    if (!creatorPrompt.trim()) return;
    setCreateError("");
    setIsGeneratingDraft(true);
    try {
      setDraft(await generateSkillDraft(creatorPrompt));
    } catch (err) {
      setCreateError(err.message);
    } finally {
      setIsGeneratingDraft(false);
    }
  }

  function startEdit(skill) {
    if (skill.source !== "custom") return;
    setCreateError("");
    setEditingSkillId(skill.id);
    setCreatorPrompt("");
    setDraft({
      name: skill.name,
      title: skill.title,
      description: skill.description,
      category: skill.category || "自定义",
      invocation_prompt: skill.invocation_prompt || "",
      content: skill.content || "",
    });
    setIsCreating(true);
  }

  return (
    <section className="skill-command-menu">
      <div className="skill-command-head">
        <span>技能</span>
        <small>{skills.length}</small>
        <button
          className="skill-create-entry"
          type="button"
          onClick={startCreate}
        >
          <Plus size={14} />
          新建技能
        </button>
        <button type="button" onClick={onClose}>
          关闭
        </button>
      </div>
      <div className="skill-command-body">
        <div className="skill-command-list">
          {skills.map((skill) => (
            <button
              className={
                selectedSkill?.name === skill.name && !isCreating
                  ? "active"
                  : ""
              }
              key={skill.name}
              type="button"
              onClick={() => {
                setSelectedSkill(skill);
                setIsCreating(false);
              }}
            >
              <span className="skill-command-icon">
                <Sparkles size={17} />
              </span>
              <strong>{skill.title}</strong>
              <small>{skill.description}</small>
              <em>{skill.category}</em>
            </button>
          ))}
        </div>
        {isCreating ? (
          <div className="skill-create-panel">
            <div className="skill-preview-heading">
              <div>
                <span>Skill Creator</span>
                <strong>
                  {editingSkillId ? "编辑自定义技能" : "创建自定义技能"}
                </strong>
              </div>
            </div>
            {!editingSkillId ? (
              <div className="skill-creator-quick">
                <label>
                  <span>一句话描述你想要的技能</span>
                  <textarea
                    value={creatorPrompt}
                    onChange={(event) => setCreatorPrompt(event.target.value)}
                    placeholder="例如：检查短视频前三秒钩子，并给出三个可直接替换的方案"
                  />
                </label>
                <button
                  className="secondary-button"
                  type="button"
                  disabled={!creatorPrompt.trim() || isGeneratingDraft}
                  onClick={createFromPrompt}
                >
                  {isGeneratingDraft ? (
                    <Loader2 className="spin" size={15} />
                  ) : (
                    <Sparkles size={15} />
                  )}
                  {isGeneratingDraft ? "生成中" : "生成草稿"}
                </button>
              </div>
            ) : null}
            <label>
              <span>名称</span>
              <input
                value={draft.title}
                onChange={(event) => updateDraft("title", event.target.value)}
                placeholder="例如：小红书标题优化"
              />
            </label>
            <label>
              <span>标识</span>
              <input
                value={draft.name}
                onChange={(event) =>
                  updateDraft(
                    "name",
                    event.target.value
                      .toLowerCase()
                      .replace(/[^a-z0-9-]/g, "-"),
                  )
                }
                placeholder="xiaohongshu-title-review"
              />
              <small>小写字母、数字和连字符</small>
            </label>
            <label>
              <span>描述</span>
              <textarea
                value={draft.description}
                onChange={(event) =>
                  updateDraft("description", event.target.value)
                }
                placeholder="说明它做什么，以及什么情况下应该使用。"
              />
            </label>
            <label>
              <span>分类</span>
              <input
                value={draft.category}
                onChange={(event) =>
                  updateDraft("category", event.target.value)
                }
              />
            </label>
            <label>
              <span>调用提示</span>
              <input
                value={draft.invocation_prompt}
                onChange={(event) =>
                  updateDraft("invocation_prompt", event.target.value)
                }
                placeholder="可留空，由 Skill Creator 自动生成"
              />
            </label>
            <label>
              <span>正文</span>
              <textarea
                className="skill-content-editor"
                value={draft.content}
                onChange={(event) => updateDraft("content", event.target.value)}
                placeholder={
                  "# 工作流程\n\n1. 读取输入...\n2. 按规则处理...\n\n## 输出格式\n..."
                }
              />
            </label>
            {createError ? (
              <div className="error-banner">{createError}</div>
            ) : null}
            <button
              className="primary-button"
              type="button"
              disabled={
                isSaving ||
                !draft.name ||
                !draft.title ||
                !draft.description ||
                !draft.content
              }
              onClick={saveSkill}
            >
              {isSaving ? "保存中" : editingSkillId ? "保存修改" : "创建并启用"}
            </button>
          </div>
        ) : selectedSkill ? (
          <div className="skill-preview-panel">
            <div className="skill-preview-heading">
              <div>
                <span>
                  {selectedSkill.source === "custom"
                    ? "自定义 Skill"
                    : "内置 Skill"}
                </span>
                <strong>{selectedSkill.title}</strong>
              </div>
              <em>{selectedSkill.category}</em>
            </div>
            <dl className="skill-metadata">
              <div>
                <dt>名称</dt>
                <dd>{selectedSkill.name}</dd>
              </div>
              <div>
                <dt>描述</dt>
                <dd>{selectedSkill.description}</dd>
              </div>
            </dl>
            <div className="skill-preview-content">
              <MarkdownContent content={selectedSkill.content || "暂无正文"} />
            </div>
            <div className="skill-preview-actions">
              <button
                className="primary-button"
                type="button"
                onClick={() => onSelect(selectedSkill)}
              >
                使用这个 Skill
              </button>
              {selectedSkill.source === "custom" ? (
                <button
                  className="secondary-button"
                  type="button"
                  onClick={() => startEdit(selectedSkill)}
                >
                  编辑技能
                </button>
              ) : (
                <span>系统预置技能，仅支持查看和使用</span>
              )}
            </div>
          </div>
        ) : (
          <div className="skill-preview-empty">
            还没有 Skill，可以创建一个。
          </div>
        )}
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
          <details
            key={`${step.index}-${step.kind}-${step.tool || "final"}`}
            className="agent-step-card"
          >
            <summary>
              <span>{String(step.index).padStart(2, "0")}</span>
              <strong>{step.kind === "tool" ? step.tool : "final"}</strong>
              <small>
                {step.reason ||
                  (step.kind === "tool" ? "调用工具" : "最终回答")}
              </small>
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
    <details className="citation-panel">
      <summary>参考了 {citations.length} 条产品资料</summary>
      <div className="citation-list">
        {citations.map((citation, index) => (
          <article
            key={`${citation.chunk_id || citation.chunk_index}-${index}`}
            className="citation-card"
          >
            <div>
              <strong>{citation.heading || "产品资料"}</strong>
              <span>{citation.product_name}</span>
            </div>
          </article>
        ))}
      </div>
    </details>
  );
}

function ChatMessageBubble({ message, isTyping = false }) {
  const content = displayChatMessageContent(message);
  return (
    <div className={`chat-message ${message.role} ${isTyping ? "typing" : ""}`}>
      <span>{message.role === "assistant" ? "助手" : "用户"}</span>
      <p>
        {content}
        {isTyping ? <b className="typing-cursor" /> : null}
      </p>
    </div>
  );
}

function displayChatMessageContent(message) {
  let content = (message?.content || "")
    .replace(/<think>[\s\S]*?<\/think>/gi, "")
    .replace(/（当前已选产品 ID：[^）]+）/g, "")
    .trim();
  if (message?.role === "assistant") {
    content = visibleAssistantContent(content);
  }
  if (
    message?.role !== "user" ||
    !["继续推进创作空间「", "继续推进创意空间「"].some((prefix) =>
      content.startsWith(prefix),
    )
  )
    return content;
  return content.split("\n", 1)[0];
}

function FissionDirectionPicker({ count }) {
  const rows = Array.from({ length: count }, (_, index) => index);
  const [directions, setDirections] = useState(() =>
    rows.map((index) => defaultFissionDirection(index)),
  );

  useEffect(() => {
    setDirections((current) =>
      rows.map((index) => current[index] || defaultFissionDirection(index)),
    );
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
            <input
              type="hidden"
              name="fission_directions"
              value={directions[index] || defaultFissionDirection(index)}
            />
            <div className="direction-card-title">
              <span className="direction-index">
                {String(index + 1).padStart(2, "0")}
              </span>
              <div>
                <strong>第 {index + 1} 条裂变脚本</strong>
                <small>
                  {directions[index] || defaultFissionDirection(index)}
                </small>
              </div>
            </div>
            <div className="direction-option-board">
              {fissionDirectionGroups.map((group) => (
                <div key={group.layer} className="direction-option-group">
                  <span>{group.layer}</span>
                  <div>
                    {group.items.map((item) => {
                      const value = `${group.layer}-${item}`;
                      const active =
                        (directions[index] ||
                          defaultFissionDirection(index)) === value;
                      return (
                        <button
                          key={value}
                          className={active ? "active" : ""}
                          type="button"
                          onClick={() => updateDirection(index, value)}
                        >
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

function AgentStart({
  products,
  jobs,
  spaces,
  suggestions,
  isSending,
  onSend,
  onSpaces,
  onHistory,
  onSuggestion,
}) {
  const [goal, setGoal] = useState("");
  const [productID, setProductID] = useState(products[0]?.id || "");
  const safeJobs = Array.isArray(jobs) ? jobs : [];
  const safeSpaces = Array.isArray(spaces) ? spaces : [];
  const active = safeJobs.filter((job) => runningStatuses.has(job.status));
  return (
    <section className="agent-start">
      <div className="agent-intro">
        <span className="eyebrow">ScriptAgent</span>
        <h1>今天想完成什么？</h1>
        <p>说出目标，助手会读取资料、规划步骤并执行。</p>
      </div>
      <div className="agent-goal-card">
        <textarea
          value={goal}
          onChange={(event) => setGoal(event.target.value)}
          placeholder="例如：为夏季活动生成 8 条短视频脚本，优先测试前三秒钩子"
        />
        <div>
          <select
            value={productID}
            onChange={(event) => setProductID(event.target.value)}
          >
            <option value="">暂不选择资料</option>
            {products.map((product) => (
              <option key={product.id} value={product.id}>
                {product.title}
              </option>
            ))}
          </select>
          <button
            className="primary-button"
            type="button"
            disabled={!goal.trim() || isSending}
            onClick={() => onSend(productID, goal)}
          >
            <Play size={15} />
            {isSending ? "处理中" : "发送"}
          </button>
        </div>
      </div>
      {suggestions?.length ? (
        <section className="proactive-panel">
          <div className="section-heading">
            <span>
              <Sparkles size={15} /> Agent 建议
            </span>
            <small>根据当前进度生成，执行前由你确认</small>
          </div>
          <div className="proactive-list">
            {suggestions.slice(0, 3).map((item) => (
              <article key={item.id}>
                <div>
                  <strong>{item.title}</strong>
                  <p>{item.summary}</p>
                </div>
                <div className="proactive-actions">
                  <button
                    className="mini-button"
                    type="button"
                    onClick={() => onSuggestion(item, "dismissed")}
                  >
                    忽略
                  </button>
                  <button
                    className="primary-button"
                    type="button"
                    onClick={() => onSuggestion(item, "accepted")}
                  >
                    {suggestionActionLabel(item.action_type)}
                  </button>
                </div>
              </article>
            ))}
          </div>
        </section>
      ) : null}
      <div className="agent-start-grid">
        <section>
          <h2>正在进行</h2>
          {active.length ? (
            active.map((job) => (
              <button
                className="agent-list-row"
                key={job.id}
                onClick={onHistory}
              >
                <span className="task-state-dot running" />
                <strong>{job.title}</strong>
                <small>{statusLabel(job.status)}</small>
              </button>
            ))
          ) : (
            <p>当前没有执行中的任务。</p>
          )}
        </section>
        <section>
          <div className="section-heading">
            <span>最近空间</span>
            <button className="mini-button" type="button" onClick={onSpaces}>
              查看全部
            </button>
          </div>
          {safeSpaces.length ? (
            safeSpaces.slice(0, 3).map((space) => (
              <button
                className="agent-list-row"
                key={space.id}
                onClick={onSpaces}
              >
                <FolderKanban size={16} />
                <strong>{space.title}</strong>
                <small>{space.summary || "继续创作"}</small>
              </button>
            ))
          ) : (
            <button className="agent-list-row" onClick={onSpaces}>
              <FolderKanban size={16} />
              <strong>创建第一个空间</strong>
              <small>把目标和历史放在一起</small>
            </button>
          )}
        </section>
      </div>
    </section>
  );
}

function HistoryWorkspace({
  jobs,
  chats,
  videos,
  products,
  onJob,
  onChat,
  onVideo,
}) {
  const [query, setQuery] = useState("");
  const [kind, setKind] = useState("all");
  const safeJobs = Array.isArray(jobs) ? jobs : [],
    safeChats = Array.isArray(chats) ? chats : [],
    safeVideos = Array.isArray(videos) ? videos : [],
    safeProducts = Array.isArray(products) ? products : [];
  const items = [
    ...safeJobs.map((job) => ({
      id: job.id,
      kind: "job",
      title: job.title,
      detail: `${statusLabel(job.status)} · ${safeProducts.find((p) => p.md_name === job.product_md_name)?.title || job.product_md_name}`,
      at: job.updated_at,
    })),
    ...safeChats.map((chat) => ({
      id: chat.id,
      kind: "chat",
      title: chat.title,
      detail: chat.summary || "和 ScriptAgent 的对话",
      at: chat.updated_at,
    })),
    ...safeVideos.map((video) => ({
      id: video.id,
      kind: "video",
      video,
      conversationId: video.conversation_id || "",
      title: video.prompt || "UGC 视频",
      detail: `${videoStatusText[video.status] || video.status} · ${video.resolution} · ${video.duration} 秒 · ${video.sound_enabled ? "有声" : "无声"}`,
      at: video.updated_at || video.created_at,
    })),
  ]
    .filter(
      (item) =>
        (kind === "all" || item.kind === kind) &&
        `${item.title} ${item.detail}`
          .toLowerCase()
          .includes(query.toLowerCase()),
    )
    .sort((a, b) => new Date(b.at) - new Date(a.at));
  return (
    <section className="history-workspace">
      <span className="eyebrow">过去的工作都在这里</span>
      <h1>继续，不必重新开始</h1>
      <p>对话、执行任务和视频结果统一保存。</p>
      <div className="history-tools">
        <label>
          <Search size={15} />
          <input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="搜索标题或产品"
          />
        </label>
        <div>
          {[
            ["all", "全部"],
            ["job", "执行任务"],
            ["chat", "对话"],
            ["video", "视频结果"],
          ].map(([key, label]) => (
            <button
              key={key}
              className={kind === key ? "active" : ""}
              onClick={() => setKind(key)}
            >
              {label}
            </button>
          ))}
        </div>
      </div>
      {items.length && kind === "video" ? (
        <div className="history-video-grid">
          {items.map((item) => (
            <HistoryVideoCard
              key={item.id}
              video={item.video}
              onConversation={() => onVideo(item.id, item.conversationId)}
            />
          ))}
        </div>
      ) : items.length ? (
        <div className="history-list">
          {items.map((item) => (
            <button
              key={`${item.kind}-${item.id}`}
              onClick={() =>
                item.kind === "job"
                  ? onJob(item.id)
                  : item.kind === "chat"
                    ? onChat(item.id)
                    : onVideo(item.id, item.conversationId)
              }
            >
              <span>
                {item.kind === "job" ? (
                  <Bot size={16} />
                ) : item.kind === "video" ? (
                  <Video size={16} />
                ) : (
                  <MessageSquare size={16} />
                )}
              </span>
              <div>
                <strong>{item.title}</strong>
                <small>{item.detail}</small>
              </div>
              <time>{formatTime(item.at)}</time>
              <ChevronRight size={16} />
            </button>
          ))}
        </div>
      ) : (
        <EmptyState text="还没有历史记录" />
      )}
    </section>
  );
}

function HistoryVideoCard({ video, onConversation }) {
  const [expanded, setExpanded] = useState(false);
  const completed = video.status === "completed";
  const failed = video.status === "failed";
  const estimatedCost = failed ? 0 : Number(video.estimated_cost_cny || 0);
  return (
    <article className="history-video-card">
      <header>
        <span className={`status-pill ${statusTone(video.status)}`}>
          {videoStatusText[video.status] || video.status}
        </span>
        <strong className="history-video-cost">
          预估费用 ¥{estimatedCost.toFixed(2)}
        </strong>
      </header>
      <div className="history-video-player">
        {completed ? (
          <video src={`/api/videos/${video.id}/file`} controls preload="metadata" />
        ) : (
          <div className={`ugc-video-state ${video.status}`}>
            {failed ? <Video size={24} /> : <Loader2 className="spin" size={24} />}
            <strong>{videoStatusText[video.status] || video.status}</strong>
            <small>{video.error_message || "完成后可在这里直接预览"}</small>
          </div>
        )}
      </div>
      <section className="history-video-prompt">
        <span>生成提示词</span>
        <p className={expanded ? "expanded" : ""}>{video.prompt}</p>
        <button type="button" onClick={() => setExpanded((value) => !value)}>
          {expanded ? "收起详情" : "查看详情"}
          <ChevronDown className={expanded ? "expanded" : ""} size={14} />
        </button>
      </section>
      <dl className="history-video-params">
        <div><dt>模型</dt><dd>{video.model}</dd></div>
        <div><dt>分辨率</dt><dd>{video.resolution}</dd></div>
        <div><dt>画幅</dt><dd>{video.ratio}</dd></div>
        <div><dt>时长</dt><dd>{video.duration} 秒</dd></div>
        <div><dt>声音</dt><dd>{video.sound_enabled ? "开启" : "关闭"}</dd></div>
        <div><dt>参考素材</dt><dd>{video.mode === "text" ? "无" : video.mode === "video" ? "视频" : "图片"}</dd></div>
      </dl>
      <footer>
        <small>费用按公开原价估算，优惠、免费额度及参考视频输入费用以服务商账单为准。</small>
        {video.conversation_id ? (
          <button className="secondary-button" type="button" onClick={onConversation}>
            查看完整对话 <ChevronRight size={14} />
          </button>
        ) : null}
      </footer>
    </article>
  );
}

function SpacesWorkspace({
  spaces,
  products,
  jobs,
  suggestions,
  isCreating,
  isSending,
  error,
  onCreate,
  onUpdate,
  onDelete,
  onStart,
  onSuggestion,
}) {
  const [showCreate, setShowCreate] = useState(false);
  async function submit(event) {
    const created = await onCreate(event);
    if (created) setShowCreate(false);
  }
  return (
    <section className="spaces-workspace">
      <div className="spaces-head">
        <div>
          <span className="eyebrow">长期创作</span>
          <h1>创意空间</h1>
          <p>集中管理长期目标、产品资料和后续创作。</p>
        </div>
        <button
          className={showCreate ? "secondary-button" : "primary-button"}
          type="button"
          onClick={() => setShowCreate((value) => !value)}
        >
          {showCreate ? <X size={16} /> : <Plus size={16} />}{" "}
          {showCreate ? "收起" : "新建空间"}
        </button>
      </div>
      {showCreate ? (
        <section className="space-create">
          <form onSubmit={submit}>
            <div className="space-create-fields">
              <input name="title" placeholder="空间名称" required />
              <select name="product_id" defaultValue="">
                <option value="">暂不选择产品资料</option>
                {products.map((product) => (
                  <option key={product.id} value={product.id}>
                    {product.title}
                  </option>
                ))}
              </select>
              <select
                name="marketing_goal"
                defaultValue="conversion"
                aria-label="广告创作目标"
              >
                {marketingGoals.map(([id, label, description]) => (
                  <option key={id} value={id}>
                    {label} · {description}
                  </option>
                ))}
              </select>
              <select
                name="goal_stage"
                defaultValue="action"
                aria-label="营销阶段"
              >
                {marketingStages.map(([id, label, description]) => (
                  <option key={id} value={id}>
                    {label} · {description}
                  </option>
                ))}
              </select>
              <textarea
                name="summary"
                placeholder="这个空间要持续完成什么？"
                required
              />
              <textarea
                name="agent_brief"
                placeholder="受众、风格、渠道或合规限制（可选）"
              />
            </div>
            <div className="space-create-actions">
              <small>创作目标会持续约束后续内容，修改不会影响历史记录。</small>
              <button className="primary-button" disabled={isCreating}>
                {isCreating ? "创建中" : "创建空间"}
              </button>
            </div>
          </form>
          {error ? <div className="error-banner">{error}</div> : null}
        </section>
      ) : null}
      <section className="spaces-list">
        {spaces.length ? (
          spaces.map((space) => (
            <SpaceCard
              key={space.id}
              space={space}
              products={products}
              jobCount={jobs.filter((job) => job.space_id === space.id).length}
              isSending={isSending}
              suggestion={suggestions?.find(
                (item) => item.space_id === space.id,
              )}
              onUpdate={onUpdate}
              onDelete={onDelete}
              onStart={onStart}
              onSuggestion={onSuggestion}
            />
          ))
        ) : (
          <EmptyState text="还没有空间。点击右上角新建。" />
        )}
      </section>
    </section>
  );
}

function IntelligenceWorkspace({ spaces, onCreateTask }) {
  const [spaceId, setSpaceId] = useState(spaces[0]?.id || "");
  const [data, setData] = useState({
    connections: [],
    signals: [],
    memories: [],
    monitors: [],
  });
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState("");
  const [loadError, setLoadError] = useState("");
  const activeSpace = spaces.find((space) => space.id === spaceId);
  const load = async (target = spaceId) => {
    setLoading(true);
    setLoadError("");
    try {
      setData(await getIntelligence(target));
    } catch (err) {
      setLoadError(err.message);
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => {
    load(spaceId);
  }, [spaceId]);
  async function demo() {
    setBusy("demo");
    setLoadError("");
    try {
      setData(await seedIntelligenceDemo(spaceId));
    } catch (err) {
      setLoadError(err.message);
    } finally {
      setBusy("");
    }
  }
  async function remember(signal) {
    if (!spaceId) return;
    setBusy(signal.id);
    try {
      await promoteIntelligenceSignal(signal.id, spaceId);
      await load(spaceId);
    } catch (err) {
      setLoadError(err.message);
    } finally {
      setBusy("");
    }
  }
  async function saveMemory(id, input) {
    setBusy(id);
    setLoadError("");
    try {
      await updateCreativeMemory(id, input);
      await load(spaceId);
      return true;
    } catch (err) {
      setLoadError(err.message);
      return false;
    } finally {
      setBusy("");
    }
  }
  async function removeMemory(id) {
    setBusy(id);
    setLoadError("");
    try {
      await deleteCreativeMemory(id);
      await load(spaceId);
      return true;
    } catch (err) {
      setLoadError(err.message);
      return false;
    } finally {
      setBusy("");
    }
  }
  async function addMonitor(event) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setBusy("monitor");
    try {
      await createCompetitorMonitor({
        space_id: spaceId,
        name: String(form.get("name") || ""),
        platform: String(form.get("platform") || "xiaohongshu"),
        account_url: String(form.get("account_url") || ""),
        keywords: String(form.get("keywords") || ""),
        source_type: "demo",
        schedule: "manual",
      });
      event.currentTarget.reset();
      await load(spaceId);
    } catch (err) {
      setLoadError(err.message);
    } finally {
      setBusy("");
    }
  }
  async function scanMonitor(monitor) {
    setBusy(monitor.id);
    try {
      await scanCompetitorMonitor(monitor.id);
      await load(spaceId);
    } catch (err) {
      setLoadError(err.message);
    } finally {
      setBusy("");
    }
  }
  const labels = {
    market_opportunity: "市场机会",
    winning_creative: "优胜信号",
    fatigue: "疲劳预警",
    audience_voice: "用户声音",
  };
  return (
    <section className="intelligence-workspace">
      <header className="intelligence-head">
        <div>
          <span className="eyebrow">市场 × 竞品 × 自有数据</span>
          <h1>创意雷达</h1>
          <p>连接外部数据，把证据转成可验证的素材实验和长期创意记忆。</p>
        </div>
        <div className="intelligence-actions">
          <select value={spaceId} onChange={(e) => setSpaceId(e.target.value)}>
            <option value="">暂不绑定空间</option>
            {spaces.map((space) => (
              <option key={space.id} value={space.id}>
                {space.title}
              </option>
            ))}
          </select>
          <button
            className="primary-button"
            type="button"
            disabled={busy === "demo"}
            onClick={demo}
          >
            {busy === "demo" ? (
              <Loader2 className="spin" size={16} />
            ) : (
              <Play size={16} />
            )}
            载入演示数据
          </button>
        </div>
      </header>
      <div className="connector-strip">
        <div>
          <Radar size={18} />
          <span>
            <strong>数据连接</strong>
            <small>
              当前可用演示数据；文件导入与真实平台 OAuth/API 正在接入。
            </small>
          </span>
        </div>
        {data.connections.map((item) => (
          <div className="connector-card" key={item.id}>
            <span className="status-dot" />
            <span>
              <strong>{item.name}</strong>
              <small>
                {item.status === "connected" ? "已连接" : item.status} ·{" "}
                {item.last_synced_at
                  ? formatTime(item.last_synced_at)
                  : "未同步"}
              </small>
            </span>
          </div>
        ))}
      </div>
      <section className="competitor-monitor-panel">
        <div className="section-heading">
          <span>竞品监控</span>
          <small>添加账号或关键词；当前扫描生成明确标记的演示结果</small>
        </div>
        <form onSubmit={addMonitor}>
          <input name="name" placeholder="竞品名称" required />
          <select name="platform" defaultValue="xiaohongshu">
            <option value="xiaohongshu">小红书</option>
            <option value="douyin">抖音</option>
            <option value="tiktok">TikTok</option>
            <option value="meta">Meta</option>
            <option value="other">其他平台</option>
          </select>
          <input
            name="account_url"
            type="url"
            placeholder="竞品账号或广告库链接（可选）"
          />
          <input name="keywords" placeholder="品类、品牌、卖点关键词" />
          <button className="secondary-button" disabled={busy === "monitor"}>
            {busy === "monitor" ? "添加中" : "添加监控"}
          </button>
        </form>
        {data.monitors?.length ? (
          <div className="monitor-list">
            {data.monitors.map((monitor) => (
              <article key={monitor.id}>
                <div>
                  <strong>{monitor.name}</strong>
                  <small>
                    {monitor.platform} · {monitor.keywords || "未设置关键词"}
                  </small>
                </div>
                <span>
                  {monitor.last_scanned_at
                    ? `上次扫描 ${formatTime(monitor.last_scanned_at)}`
                    : "尚未扫描"}
                </span>
                <button
                  className="mini-button"
                  type="button"
                  disabled={busy === monitor.id}
                  onClick={() => scanMonitor(monitor)}
                >
                  {busy === monitor.id ? "扫描中" : "演示扫描"}
                </button>
              </article>
            ))}
          </div>
        ) : (
          <p className="monitor-empty">
            尚未添加竞品。Demo 阶段不会抓取真实平台数据。
          </p>
        )}
      </section>
      {loadError ? <div className="error-banner">{loadError}</div> : null}
      {loading ? (
        <div className="intelligence-empty">
          <Loader2 className="spin" />
          <span>正在读取创意证据</span>
        </div>
      ) : data.signals.length ? (
        <div className="intelligence-layout">
          <section>
            <div className="section-heading">
              <span>最新信号</span>
              <small>仅保存来源明确、带时间窗口的证据</small>
            </div>
            <div className="signal-grid">
              {data.signals.map((signal) => (
                <article key={signal.id}>
                  <div>
                    <span className={`signal-kind ${signal.signal_type}`}>
                      {labels[signal.signal_type] || signal.signal_type}
                    </span>
                    <span className="confidence">
                      置信度 {Math.round(signal.confidence * 100)}%
                    </span>
                  </div>
                  <h3>{signal.title}</h3>
                  <p>{signal.summary}</p>
                  <footer>
                    <button
                      className="mini-button"
                      type="button"
                      disabled={
                        !spaceId ||
                        busy === signal.id ||
                        data.memories.some((m) => m.signal_id === signal.id)
                      }
                      onClick={() => remember(signal)}
                    >
                      {data.memories.some((m) => m.signal_id === signal.id)
                        ? "已进入记忆"
                        : "确认进入记忆"}
                    </button>
                    <button
                      className="secondary-button"
                      type="button"
                      disabled={!activeSpace}
                      onClick={() => onCreateTask(activeSpace, signal)}
                    >
                      生成实验任务
                    </button>
                  </footer>
                </article>
              ))}
            </div>
          </section>
          <aside>
            <div className="section-heading">
              <span>长期创意记忆</span>
              <small>只注入已由用户确认的结论</small>
            </div>
            {data.memories.length ? (
              data.memories.map((memory) => (
                <CreativeMemoryCard
                  memory={memory}
                  busy={busy === memory.id}
                  onSave={saveMemory}
                  onDelete={removeMemory}
                  key={memory.id}
                />
              ))
            ) : (
              <div className="memory-empty">
                选择一条有价值的信号并确认，之后空间对话会读取它。
              </div>
            )}
          </aside>
        </div>
      ) : (
        <div className="intelligence-empty">
          <Radar size={30} />
          <strong>还没有数据连接</strong>
          <span>
            无需广告账户，先载入演示数据验证“发现—创作—投放反馈—再创作”闭环。
          </span>
        </div>
      )}
    </section>
  );
}

function CreativeMemoryCard({ memory, busy, onSave, onDelete }) {
  const [editing, setEditing] = useState(false);
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const [title, setTitle] = useState(memory.title);
  const [finding, setFinding] = useState(memory.finding);
  const [confidence, setConfidence] = useState(
    Math.round(memory.confidence * 100),
  );
  async function save(event) {
    event.preventDefault();
    if (
      await onSave(memory.id, {
        title,
        finding,
        confidence: Number(confidence) / 100,
      })
    )
      setEditing(false);
  }
  if (editing)
    return (
      <form className="memory-card memory-editor" onSubmit={save}>
        <label>
          <span>标题</span>
          <input
            value={title}
            onChange={(event) => setTitle(event.target.value)}
            required
          />
        </label>
        <label>
          <span>结论</span>
          <textarea
            value={finding}
            onChange={(event) => setFinding(event.target.value)}
            required
          />
        </label>
        <label>
          <span>置信度</span>
          <div className="memory-confidence-input">
            <input
              type="range"
              min="0"
              max="100"
              value={confidence}
              onChange={(event) => setConfidence(event.target.value)}
            />
            <output>{confidence}%</output>
          </div>
        </label>
        <div className="memory-card-actions">
          <button
            className="mini-button"
            type="button"
            onClick={() => {
              setTitle(memory.title);
              setFinding(memory.finding);
              setConfidence(Math.round(memory.confidence * 100));
              setEditing(false);
            }}
          >
            取消
          </button>
          <button className="primary-button" disabled={busy}>
            保存
          </button>
        </div>
      </form>
    );
  return (
    <article className="memory-card">
      <CheckCircle2 size={16} />
      <div className="memory-card-copy">
        <strong>{memory.title}</strong>
        <p>{memory.finding}</p>
        <small>
          置信度 {Math.round(memory.confidence * 100)}% ·{" "}
          {formatTime(memory.last_verified_at)}
        </small>
      </div>
      <div className="memory-card-menu">
        {confirmingDelete ? (
          <>
            <span>确认删除？</span>
            <button
              type="button"
              disabled={busy}
              onClick={() => onDelete(memory.id)}
            >
              删除
            </button>
            <button type="button" onClick={() => setConfirmingDelete(false)}>
              取消
            </button>
          </>
        ) : (
          <>
            <button
              type="button"
              title="编辑记忆"
              aria-label={`编辑 ${memory.title}`}
              onClick={() => setEditing(true)}
            >
              <FileText size={14} />
            </button>
            <button
              type="button"
              title="删除记忆"
              aria-label={`删除 ${memory.title}`}
              onClick={() => setConfirmingDelete(true)}
            >
              <Trash2 size={14} />
            </button>
          </>
        )}
      </div>
    </article>
  );
}

function SpaceCard({
  space,
  products,
  jobCount,
  isSending,
  suggestion,
  onUpdate,
  onDelete,
  onStart,
  onSuggestion,
}) {
  const [editing, setEditing] = useState(false);
  const [productID, setProductID] = useState(space.product_id || "");
  const [marketingGoal, setMarketingGoal] = useState(
    space.marketing_goal || "conversion",
  );
  const [goalStage, setGoalStage] = useState(space.goal_stage || "action");
  const [saving, setSaving] = useState(false);
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const product = products.find((item) => item.id === space.product_id);

  async function save() {
    setSaving(true);
    try {
      await onUpdate(space.id, {
        title: space.title,
        summary: space.summary || "",
        agent_brief: space.agent_brief || "",
        product_id: productID,
        marketing_goal: marketingGoal,
        goal_stage: goalStage,
      });
      setEditing(false);
    } finally {
      setSaving(false);
    }
  }

  async function remove() {
    setDeleting(true);
    try {
      await onDelete(space.id);
    } finally {
      setDeleting(false);
    }
  }

  return (
    <article className={editing ? "editing" : ""}>
      <span className="space-card-icon">
        <FolderKanban size={18} />
      </span>
      <div className="space-card-copy">
        <strong>{space.title}</strong>
        <p>{space.summary || "继续上次的创作"}</p>
        {editing ? (
          <div className="space-product-editor">
            <select
              value={productID}
              onChange={(event) => setProductID(event.target.value)}
            >
              <option value="">暂不选择产品资料</option>
              {products.map((item) => (
                <option key={item.id} value={item.id}>
                  {item.title}
                </option>
              ))}
            </select>
            <select
              value={marketingGoal}
              aria-label="广告创作目标"
              onChange={(event) => setMarketingGoal(event.target.value)}
            >
              {marketingGoals.map(([id, label]) => (
                <option key={id} value={id}>
                  {label}
                </option>
              ))}
            </select>
            <select
              value={goalStage}
              aria-label="营销阶段"
              onChange={(event) => setGoalStage(event.target.value)}
            >
              {marketingStages.map(([id, label]) => (
                <option key={id} value={id}>
                  {label}
                </option>
              ))}
            </select>
            <small>保存后只约束后续对话，历史内容保持不变。</small>
          </div>
        ) : (
          <>
            <div className="space-goal-tags">
              <span>
                {marketingLabel(marketingGoals, space.marketing_goal)}
              </span>
              <span>{marketingLabel(marketingStages, space.goal_stage)}</span>
            </div>
            <small>
              {product?.title || "暂未关联资料"} · {jobCount} 次执行
            </small>
            {suggestion ? (
              <button
                className="space-next-action"
                type="button"
                onClick={() => onSuggestion(suggestion, "accepted")}
              >
                <Sparkles size={13} /> 下一步：{suggestion.title}{" "}
                <ChevronRight size={13} />
              </button>
            ) : null}
          </>
        )}
      </div>
      <div className="space-card-actions">
        {confirmingDelete ? (
          <div className="space-delete-confirm">
            <span>确认删除？</span>
            <button
              className="mini-button"
              type="button"
              onClick={() => setConfirmingDelete(false)}
            >
              取消
            </button>
            <button
              className="danger-button"
              type="button"
              disabled={deleting}
              onClick={remove}
            >
              {deleting ? "删除中" : "删除"}
            </button>
          </div>
        ) : editing ? (
          <>
            <button
              className="mini-button"
              type="button"
              onClick={() => {
                setProductID(space.product_id || "");
                setMarketingGoal(space.marketing_goal || "conversion");
                setGoalStage(space.goal_stage || "action");
                setEditing(false);
              }}
            >
              取消
            </button>
            <button
              className="primary-button"
              type="button"
              disabled={saving}
              onClick={save}
            >
              {saving ? "保存中" : "保存"}
            </button>
          </>
        ) : (
          <>
            <button
              className="icon-button subtle"
              title="删除空间"
              aria-label={`删除 ${space.title}`}
              type="button"
              onClick={() => setConfirmingDelete(true)}
            >
              <Trash2 size={15} />
            </button>
            <button
              className="mini-button"
              type="button"
              onClick={() => setEditing(true)}
            >
              设置
            </button>
            <button
              className="secondary-button"
              type="button"
              disabled={isSending}
              onClick={() => onStart(space)}
            >
              {isSending ? "处理中" : "继续创作"}
            </button>
          </>
        )}
      </div>
    </article>
  );
}

function ProductKnowledgeWorkspace({
  products,
  selectedProductId,
  productPreview,
  isLoadingProductPreview,
  isCreatingProduct,
  isSavingProduct,
  error,
  onSelect,
  onStartJob,
  onCreate,
  onUpdate,
}) {
  const [query, setQuery] = useState("");
  const [editing, setEditing] = useState(false);
  const [isNew, setIsNew] = useState(false);
  const [isParsingDocument, setIsParsingDocument] = useState(false);
  const [documentParseError, setDocumentParseError] = useState("");
  const [parsedFilename, setParsedFilename] = useState("");
  const [assets, setAssets] = useState([]);
  const [isUploadingAsset, setIsUploadingAsset] = useState(false);
  const [previewAsset, setPreviewAsset] = useState(null);
  const [deletingAssetId, setDeletingAssetId] = useState("");
  const [title, setTitle] = useState("");
  const [content, setContent] = useState("");
  const selected =
    products.find((item) => item.id === selectedProductId) || products[0];
  const visible = products.filter((item) =>
    item.title.toLowerCase().includes(query.toLowerCase()),
  );
  useEffect(() => {
    if (!editing) {
      setTitle(selected?.title || "");
      setContent(productPreview?.content || "");
    }
  }, [selected?.id, productPreview?.content, editing]);
  useEffect(() => {
    if (!selected?.id) {
      setAssets([]);
      return;
    }
    listProductAssets(selected.id)
      .then((items) => setAssets(Array.isArray(items) ? items : []))
      .catch(() => setAssets([]));
  }, [selected?.id]);
  async function save(event) {
    event.preventDefault();
    if (selected) {
      await onUpdate(selected.id, title, content);
      setEditing(false);
    }
  }
  async function addAsset(event) {
    const file = event.target.files?.[0];
    if (!file || !selected) return;
    setIsUploadingAsset(true);
    try {
      const asset = await uploadProductAsset(selected.id, file);
      setAssets((items) => [asset, ...items]);
    } catch (err) {
      window.alert(err.message);
    } finally {
      event.target.value = "";
      setIsUploadingAsset(false);
    }
  }
  async function parseDocument(event) {
    const file = event.target.files?.[0];
    if (!file) return;
    setIsParsingDocument(true);
    setDocumentParseError("");
    try {
      const result = await parseProductDocument(file);
      setTitle(result.title || title);
      setContent(result.content || "");
      setParsedFilename(result.filename || file.name);
    } catch (err) {
      setDocumentParseError(err.message);
    } finally {
      event.target.value = "";
      setIsParsingDocument(false);
    }
  }
  async function removeAsset(asset) {
    setDeletingAssetId(asset.id);
    try {
      await deleteProductAsset(asset.id);
      setAssets((items) => items.filter((item) => item.id !== asset.id));
      if (previewAsset?.id === asset.id) setPreviewAsset(null);
    } catch (err) {
      window.alert(err.message);
    } finally {
      setDeletingAssetId("");
    }
  }
  return (
    <section className="knowledge-workspace">
      {previewAsset ? (
        <AssetPreviewModal
          asset={previewAsset}
          deleting={deletingAssetId === previewAsset.id}
          onClose={() => setPreviewAsset(null)}
          onDelete={removeAsset}
        />
      ) : null}
      <header>
        <div>
          <span className="eyebrow">可持续更新的资料</span>
          <h1>产品资料</h1>
          <p>把产品说清楚一次，后续创作和执行都会自动带上它。</p>
        </div>
        <button
          className="primary-button"
          type="button"
          onClick={() => {
            setEditing(true);
            setIsNew(true);
            setTitle("");
            setContent(
              "# 产品资料\n\n## 卖点\n\n## 目标用户\n\n## 使用场景\n\n## 表达边界\n",
            );
            setParsedFilename("");
            setDocumentParseError("");
          }}
        >
          {" "}
          <Plus size={16} />
          新建
        </button>
      </header>
      <div className="knowledge-grid">
        <aside className="knowledge-list">
          <label className="knowledge-search">
            <Search size={15} />
            <input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="搜索资料"
            />
          </label>
          <div className="knowledge-entry-note">
            你可以上传 Markdown、粘贴文字，或在这里直接修改。
          </div>
          {visible.length ? (
            visible.map((item) => (
              <button
                className={selected?.id === item.id ? "active" : ""}
                type="button"
                key={item.id}
                onClick={() => {
                  setEditing(false);
                  setIsNew(false);
                  onSelect(item.id);
                }}
              >
                <Package size={16} />
                <span>
                  <strong>{item.title}</strong>
                  <small>更新 {formatTime(item.updated_at)}</small>
                </span>
                <ChevronRight size={15} />
              </button>
            ))
          ) : (
            <EmptyState text="还没有匹配的资料" compact />
          )}
        </aside>
        <main className="knowledge-reader">
          {editing ? (
            <form
              className="living-product-editor"
              onSubmit={async (event) => {
                if (selected && !isNew) return save(event);
                event.preventDefault();
                const data = new FormData();
                data.append("title", title);
                data.append(
                  "product_md",
                  new File([content], `${title || "产品资料"}.md`, {
                    type: "text/markdown",
                  }),
                );
                await onCreate(data);
                setIsNew(false);
                setEditing(false);
              }}
            >
              <div className="knowledge-editor-head">
                <input
                  value={title}
                  onChange={(event) => setTitle(event.target.value)}
                  placeholder="资料名称"
                  required
                />
                <button
                  className="secondary-button"
                  type="button"
                  onClick={() => {
                    setEditing(false);
                    setIsNew(false);
                    setTitle(selected?.title || "");
                    setContent(productPreview?.content || "");
                  }}
                >
                  取消
                </button>
                <button
                  className="primary-button"
                  disabled={isSavingProduct || isCreatingProduct}
                  type="submit"
                >
                  {isSavingProduct || isCreatingProduct ? "保存中" : "保存资料"}
                </button>
              </div>
              {isNew ? (
                <section className="product-document-import">
                  <div>
                    <strong>从文件解析</strong>
                    <small>支持 MD、PDF、DOC、DOCX，解析后可继续编辑</small>
                  </div>
                  <label className="secondary-button">
                    {isParsingDocument ? (
                      <Loader2 className="spin" size={15} />
                    ) : (
                      <Upload size={15} />
                    )}
                    {isParsingDocument ? "解析中" : "上传并解析"}
                    <input
                      type="file"
                      accept=".md,.markdown,.pdf,.doc,.docx,text/markdown,application/pdf,application/msword,application/vnd.openxmlformats-officedocument.wordprocessingml.document"
                      disabled={isParsingDocument}
                      onChange={parseDocument}
                    />
                  </label>
                  {parsedFilename ? (
                    <span className="document-parse-success">
                      <CheckCircle2 size={14} />
                      已解析 {parsedFilename}
                    </span>
                  ) : null}
                  {documentParseError ? (
                    <div className="error-banner">{documentParseError}</div>
                  ) : null}
                </section>
              ) : null}
              <textarea
                value={content}
                onChange={(event) => setContent(event.target.value)}
                aria-label="产品资料内容"
                required
              />
            </form>
          ) : selected ? (
            <>
              <div className="knowledge-reader-head">
                <div>
                  <span className="eyebrow">当前资料</span>
                  <h2>{selected.title}</h2>
                  <small>
                    {selected.md_name} · 更新 {formatTime(selected.updated_at)}
                  </small>
                </div>
                <div>
                  <button
                    className="secondary-button"
                    type="button"
                    onClick={() => setEditing(true)}
                  >
                    <FileText size={15} />
                    修改
                  </button>
                  <button
                    className="primary-button"
                    type="button"
                    onClick={() => onStartJob(selected.id)}
                  >
                    <Play size={15} />
                    开始创作
                  </button>
                </div>
              </div>
              {isLoadingProductPreview ? (
                <EmptyState text="正在读取资料" />
              ) : productPreview?.content ? (
                <MarkdownContent content={productPreview.content} />
              ) : (
                <EmptyState text="资料为空，点击修改补充内容" />
              )}
              <section className="product-assets">
                <div>
                  <strong>图片与视频素材</strong>
                  <small>任务执行时可作为产品参考素材</small>
                </div>
                <label className="asset-upload">
                  <Upload size={15} />
                  {isUploadingAsset ? "上传中" : "添加素材"}
                  <input
                    type="file"
                    accept="image/png,image/jpeg,image/webp,image/gif,video/mp4,video/quicktime,video/webm"
                    onChange={addAsset}
                  />
                </label>
                {assets.length ? (
                  <div className="asset-gallery">
                    {assets.map((asset) => (
                      <ProductMediaCard
                        asset={asset}
                        deleting={deletingAssetId === asset.id}
                        onPreview={setPreviewAsset}
                        onDelete={removeAsset}
                        key={asset.id}
                      />
                    ))}
                  </div>
                ) : (
                  <p className="asset-empty">
                    还没有素材。添加产品图、包装图、演示视频或可用镜头。
                  </p>
                )}
              </section>
            </>
          ) : (
            <EmptyState text="从左侧选择资料，或新建一份" />
          )}
        </main>
        <aside className="knowledge-checks">
          <span className="eyebrow">创作前检查</span>
          <h2>资料是否够用？</h2>
          {["核心卖点", "目标用户", "使用场景", "表达边界", "可用素材"].map(
            (label) => (
              <div key={label}>
                <span
                  className={
                    content.includes(label) ||
                    productPreview?.content?.includes(label) ||
                    (label === "可用素材" && assets.length)
                      ? "check-ok"
                      : "check-wait"
                  }
                />
                <strong>{label}</strong>
                <small>
                  {content.includes(label) ||
                  productPreview?.content?.includes(label) ||
                  (label === "可用素材" && assets.length)
                    ? "已记录"
                    : "建议补充"}
                </small>
              </div>
            ),
          )}
          <p>资料不完整也能开始。助手会在执行前提醒你缺少什么。</p>
          {error ? <div className="error-banner">{error}</div> : null}
        </aside>
      </div>
    </section>
  );
}

function ProductMediaCard({ asset, deleting, onPreview, onDelete }) {
  const [confirming, setConfirming] = useState(false);
  return (
    <figure className="product-media-card">
      <button
        className="asset-preview-trigger"
        type="button"
        onClick={() => onPreview(asset)}
        aria-label={`预览 ${asset.original_name}`}
      >
        {asset.kind === "video" ? (
          <>
            <video
              src={`/api/assets/${asset.id}/file`}
              muted
              preload="metadata"
            />
            <span className="asset-play">
              <Play size={18} />
            </span>
          </>
        ) : (
          <img src={`/api/assets/${asset.id}/file`} alt={asset.original_name} />
        )}
      </button>
      <figcaption title={asset.original_name}>{asset.original_name}</figcaption>
      <div className="asset-card-actions">
        {confirming ? (
          <>
            <button
              className="danger"
              type="button"
              disabled={deleting}
              onClick={() => onDelete(asset)}
            >
              {deleting ? "删除中" : "确认"}
            </button>
            <button type="button" onClick={() => setConfirming(false)}>
              取消
            </button>
          </>
        ) : (
          <>
            <button type="button" onClick={() => onPreview(asset)}>
              预览
            </button>
            <button
              className="danger"
              type="button"
              onClick={() => setConfirming(true)}
            >
              删除
            </button>
          </>
        )}
      </div>
    </figure>
  );
}

function AssetPreviewModal({ asset, deleting, onClose, onDelete }) {
  const [confirming, setConfirming] = useState(false);
  return (
    <div
      className="asset-preview-backdrop"
      role="presentation"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <section
        className="asset-preview-modal"
        role="dialog"
        aria-modal="true"
        aria-label={`预览 ${asset.original_name}`}
      >
        <header>
          <div>
            <strong>{asset.original_name}</strong>
            <small>
              {asset.kind === "video" ? "视频" : "图片"} ·{" "}
              {formatBytes(asset.size_bytes)}
            </small>
          </div>
          <button type="button" onClick={onClose} aria-label="关闭预览">
            <X size={20} />
          </button>
        </header>
        <div className="asset-preview-stage">
          {asset.kind === "video" ? (
            <video
              src={`/api/assets/${asset.id}/file`}
              controls
              autoPlay
              preload="metadata"
            />
          ) : (
            <img
              src={`/api/assets/${asset.id}/file`}
              alt={asset.original_name}
            />
          )}
        </div>
        <footer>
          {confirming ? (
            <>
              <span>删除后无法恢复。</span>
              <button
                className="secondary-button"
                type="button"
                onClick={() => setConfirming(false)}
              >
                取消
              </button>
              <button
                className="danger-button"
                type="button"
                disabled={deleting}
                onClick={() => onDelete(asset)}
              >
                {deleting ? "删除中" : "确认删除"}
              </button>
            </>
          ) : (
            <button
              className="danger-button"
              type="button"
              onClick={() => setConfirming(true)}
            >
              <Trash2 size={15} />
              删除素材
            </button>
          )}
        </footer>
      </section>
    </div>
  );
}

function ProductCover({ product, compact = false }) {
  const variant = productCoverVariant(product);
  return (
    <div
      className={`product-cover cover-${variant} ${compact ? "compact" : ""}`}
    >
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
    <article
      className={`product-asset-card ${isActive ? "active" : ""}`}
      onClick={() => onSelect(product.id)}
    >
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
          <small>
            {stats?.latestAt
              ? `最近生成 ${formatTime(stats.latestAt)}`
              : `更新 ${formatTime(product.updated_at)}`}
          </small>
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
  const selectedProduct =
    products.find((product) => product.id === selectedProductId) || products[0];
  const selectedStats = selectedProduct
    ? productStats.get(selectedProduct.id)
    : null;
  const selectedReport =
    creativeReports.find((report) => report.id === selectedCreativeReportId) ||
    creativeReports[0];
  const totalScripts = Array.from(productStats.values()).reduce(
    (total, stats) => total + (stats.scriptCount || 0),
    0,
  );
  useEffect(() => {
    if (!editing) {
      setEditTitle(selectedProduct?.title || "");
      setEditContent(productPreview?.content || "");
    }
  }, [selectedProduct?.id, productPreview?.content, editing]);
  return (
    <section className="product-home">
      <div className="product-home-hero">
        <div>
          <span className="eyebrow">产品资产库</span>
          <h1>先把产品放进来，再批量裂变脚本</h1>
          <p>
            每个产品都对应一份 Markdown
            资料、历史任务和可复用脚本。运营同学进来先看到自己的产品，而不是一张空表单。
          </p>
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
            <FileField
              name="product_md"
              label="产品 Markdown"
              accept=".md,.markdown,text/markdown"
              icon={<FileText size={16} />}
              required
            />
            <button
              className="primary-button wide-button"
              type="submit"
              disabled={isCreatingProduct}
            >
              {isCreatingProduct ? (
                <Loader2 className="spin" size={16} />
              ) : (
                <Plus size={16} />
              )}
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
                <p>
                  {selectedProduct.md_name} · 更新{" "}
                  {formatTime(selectedProduct.updated_at)}
                </p>
              </div>
              <div className="dossier-actions">
                <span className="status-pill success">可用于任务</span>
                <button
                  className="secondary-button"
                  type="button"
                  onClick={() => setEditing((value) => !value)}
                >
                  <FileText size={15} />
                  <span>{editing ? "阅读资料" : "编辑资料"}</span>
                </button>
                <button
                  className="secondary-button"
                  type="button"
                  onClick={() => onStartJob(selectedProduct.id)}
                >
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
                <strong>
                  {selectedStats?.latestAt
                    ? formatTime(selectedStats.latestAt)
                    : "-"}
                </strong>
              </div>
            </div>
            <section className="creative-report-panel">
              <div className="section-heading">
                <span>创意策略报告</span>
                <small>配置 DataEye 来源，生成后可转为裂变脚本任务</small>
              </div>
              <div className="creative-report-grid">
                <form
                  className="task-form compact-form"
                  onSubmit={onCreateCreativeReport}
                >
                  <div className="upload-grid">
                    <label>
                      <span>DataEye URL</span>
                      <input
                        name="dataeye_url"
                        type="url"
                        placeholder="https://..."
                      />
                    </label>
                    <label>
                      <span>DataEye 产品 ID</span>
                      <input name="dataeye_id" type="text" placeholder="可选" />
                    </label>
                  </div>
                  <div className="upload-grid">
                    <label>
                      <span>产品名</span>
                      <input
                        name="product_name"
                        type="text"
                        defaultValue={selectedProduct.title}
                      />
                    </label>
                    <label>
                      <span>时间范围</span>
                      <input
                        name="date_range"
                        type="text"
                        defaultValue="近 30 天"
                      />
                    </label>
                  </div>
                  <div className="upload-grid three-cols">
                    <label>
                      <span>媒体</span>
                      <input
                        name="media"
                        type="text"
                        placeholder="TikTok / Meta"
                      />
                    </label>
                    <label>
                      <span>国家/地区</span>
                      <input
                        name="country"
                        type="text"
                        placeholder="美国 / 日本"
                      />
                    </label>
                    <label>
                      <span>样本数</span>
                      <input
                        name="sample_count"
                        type="number"
                        min="5"
                        max="200"
                        defaultValue="50"
                      />
                    </label>
                  </div>
                  <label>
                    <span>排序指标</span>
                    <input
                      name="sort_metric"
                      type="text"
                      defaultValue="热度/曝光/播放优先"
                    />
                  </label>
                  <label>
                    <span>补充要求</span>
                    <textarea
                      name="requirement"
                      rows="3"
                      placeholder="例如：重点看开头钩子、节奏和可复制素材结构。"
                    />
                  </label>
                  <label>
                    <span>素材备注</span>
                    <textarea
                      name="material_note"
                      rows="3"
                      placeholder="如已有素材观察、DataEye 导出摘要，可贴在这里。"
                    />
                  </label>
                  <button
                    className="primary-button wide-button"
                    type="submit"
                    disabled={isCreatingCreativeReport}
                  >
                    {isCreatingCreativeReport ? (
                      <Loader2 className="spin" size={16} />
                    ) : (
                      <Activity size={16} />
                    )}
                    <span>
                      {isCreatingCreativeReport ? "生成中" : "生成创意策略报告"}
                    </span>
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
                            className={
                              selectedReport?.id === report.id ? "active" : ""
                            }
                            type="button"
                            onClick={() => onReportSelect(report.id)}
                          >
                            <strong>{formatTime(report.created_at)}</strong>
                            <span>
                              {reportConfigLabel(report.source_config_json)}
                            </span>
                          </button>
                        ))}
                      </div>
                      {selectedReport ? (
                        <article className="creative-report-preview">
                          <div className="report-actions">
                            <div>
                              <strong>报告摘要</strong>
                              <p>
                                {selectedReport.report_summary ||
                                  "报告已生成，可查看完整内容。"}
                              </p>
                            </div>
                            <button
                              className="secondary-button"
                              type="button"
                              onClick={() => onReportToJob(selectedReport)}
                            >
                              <Play size={15} />
                              <span>转裂变任务</span>
                            </button>
                          </div>
                          <MarkdownContent
                            content={selectedReport.report_markdown}
                          />
                        </article>
                      ) : null}
                    </>
                  ) : (
                    <EmptyState
                      text="暂无报告，先生成一份创意策略报告"
                      compact
                    />
                  )}
                </div>
              </div>
            </section>
            <section className="product-preview">
              <div className="section-heading">
                <span>Markdown 预览</span>
                <small>
                  {isLoadingProductPreview
                    ? "读取中"
                    : productPreview?.md_name || selectedProduct.md_name}
                </small>
              </div>
              {editing ? (
                <form
                  className="living-product-editor"
                  onSubmit={async (event) => {
                    event.preventDefault();
                    await onUpdate(selectedProduct.id, editTitle, editContent);
                    setEditing(false);
                  }}
                >
                  <input
                    value={editTitle}
                    onChange={(event) => setEditTitle(event.target.value)}
                    required
                    aria-label="产品名称"
                  />
                  <textarea
                    value={editContent}
                    onChange={(event) => setEditContent(event.target.value)}
                    required
                    aria-label="产品资料内容"
                  />
                  <button
                    className="primary-button"
                    type="submit"
                    disabled={isSavingProduct}
                  >
                    {isSavingProduct ? "保存中" : "保存更新"}
                  </button>
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

const modelCapabilityCatalog = [
  {
    id: "text",
    title: "文本与脚本",
    description: "脚本生成、裂变策略、检查和普通对话",
    endpoint:
      "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation",
    models: [
      { id: "qwen3.8-flash", label: "Qwen 3.8 Flash（推荐）" },
      { id: "qwen3.7-plus", label: "Qwen 3.7 Plus" },
      { id: "qwen3.6-plus", label: "Qwen 3.6 Plus" },
    ],
  },
  {
    id: "multimodal",
    title: "图片 / 视频理解",
    description: "素材分析、广告复刻、镜头和音画拆解",
    endpoint:
      "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation",
    models: [
      { id: "qwen3.5-omni-flash", label: "Qwen 3.5 Omni Flash（推荐）" },
      { id: "qwen3.5-omni-plus", label: "Qwen 3.5 Omni Plus" },
      { id: "qwen3-vl-8b-instruct", label: "Qwen3-VL 8B（仅视觉低成本）" },
      { id: "qwen3-vl-30b-a3b-instruct", label: "Qwen3-VL 30B" },
      { id: "qwen3-omni-30b-a3b-captioner", label: "Omni Captioner" },
    ],
  },
  {
    id: "image_generation",
    title: "图片生成",
    description: "分镜参考图、封面和产品场景图",
    endpoint:
      "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation",
    models: [
      { id: "qwen-image-2.0", label: "Qwen Image 2.0（推荐）" },
      { id: "qwen-image-2.0-pro", label: "Qwen Image 2.0 Pro" },
    ],
  },
  {
    id: "image_edit",
    title: "图片编辑",
    description: "换背景、局部修改和商品素材调整",
    endpoint:
      "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation",
    models: [
      { id: "qwen-image-edit-plus", label: "Qwen Image Edit Plus（推荐）" },
      { id: "qwen-image-edit-max", label: "Qwen Image Edit Max" },
    ],
  },
  {
    id: "video_generation",
    title: "视频生成",
    description: "UGC 文生视频、图片参考、视频参考和有声成片",
    endpoint:
      "https://dashscope.aliyuncs.com/api/v1/services/aigc/video-generation/video-synthesis",
    models: [
      { id: "wan3.0-video-prime", label: "Wan 3.0 Prime（推荐）" },
      { id: "wan3.0-video", label: "Wan 3.0 标准版" },
    ],
  },
  {
    id: "embedding",
    title: "多模态素材检索",
    description: "素材库相似搜索和知识检索",
    endpoint:
      "https://dashscope.aliyuncs.com/api/v1/services/embeddings/multimodal-embedding/multimodal-embedding",
    models: [{ id: "qwen3-vl-embedding", label: "Qwen3-VL Embedding（推荐）" }],
  },
];

function ModelOnboarding({
  modelSettings,
  isSaving,
  error,
  onSave,
  onLater,
  onSettings,
}) {
  const savedProfiles = Object.fromEntries(
    (modelSettings?.profiles || []).map((item) => [item.capability, item]),
  );
  const textCatalog = modelCapabilityCatalog.find((item) => item.id === "text");
  const multimodalCatalog = modelCapabilityCatalog.find(
    (item) => item.id === "multimodal",
  );
  const [textModel, setTextModel] = useState(
    savedProfiles.text?.model || textCatalog.models[0].id,
  );
  const [multimodalModel, setMultimodalModel] = useState(
    savedProfiles.multimodal?.model || multimodalCatalog.models[0].id,
  );
  return (
    <div className="model-onboarding-backdrop" role="presentation">
      <section
        className="model-onboarding"
        role="dialog"
        aria-modal="true"
        aria-labelledby="model-onboarding-title"
      >
        <header>
          <div>
            <span className="eyebrow">开始前设置</span>
            <h2 id="model-onboarding-title">选择常用模型</h2>
            <p>先选两个常用能力，之后可在设置中更改。</p>
          </div>
          <button
            className="model-onboarding-later"
            type="button"
            onClick={onLater}
          >
            稍后设置
          </button>
        </header>
        <div className="model-onboarding-fields">
          <label>
            <span>
              <Bot size={18} />
              <b>对话与脚本</b>
              <small>日常对话、策略和脚本生成</small>
            </span>
            <select
              value={textModel}
              onChange={(event) => setTextModel(event.target.value)}
            >
              {textCatalog.models.map((model) => (
                <option key={model.id} value={model.id}>
                  {model.label}
                </option>
              ))}
            </select>
          </label>
          <label>
            <span>
              <Video size={18} />
              <b>图片与视频理解</b>
              <small>素材拆解、镜头和内容分析</small>
            </span>
            <select
              value={multimodalModel}
              onChange={(event) => setMultimodalModel(event.target.value)}
            >
              {multimodalCatalog.models.map((model) => (
                <option key={model.id} value={model.id}>
                  {model.label}
                </option>
              ))}
            </select>
          </label>
        </div>
        {error ? <div className="error-banner">{error}</div> : null}
        <footer>
          <button
            className="model-settings-link"
            type="button"
            onClick={onSettings}
          >
            需要连接自己的 API？前往设置
          </button>
          <button
            className="primary-button"
            type="button"
            disabled={isSaving}
            onClick={() =>
              onSave({ text: textModel, multimodal: multimodalModel })
            }
          >
            {isSaving ? (
              <Loader2 className="spin" size={16} />
            ) : (
              <CheckCircle2 size={16} />
            )}{" "}
            {isSaving ? "保存中" : "保存并开始"}
          </button>
        </footer>
      </section>
    </div>
  );
}

function SettingsWorkspace({
  modelSettings,
  showDebugPanel,
  isSaving,
  error,
  onDebugPanel,
  onSave,
  ownerSession,
  isOwnerLoading,
  onOwnerLogin,
}) {
  const savedProfiles = Object.fromEntries(
    (modelSettings?.profiles || []).map((item) => [item.capability, item]),
  );
  return (
    <section className="result-pane full-height">
      <div className="result-header">
        <div>
          <h2>模型配置</h2>
          <p>
            分别配置脚本、多模态理解、图片和视频能力；可使用平台托管额度，也可由用户接入自己的
            API。
          </p>
        </div>
        <span
          className={`status-pill ${modelSettings?.configured ? "success" : "danger"}`}
        >
          <KeyRound size={13} />
          {modelSettings?.configured ? "已配置" : "未配置"}
        </span>
      </div>
      <form className="settings-form" onSubmit={onSave}>
        {error ? <div className="error-banner">{error}</div> : null}
        <details className="settings-security-note">
          <summary><KeyRound size={15} /> 密钥如何保存</summary>
          <p>API Key 保存在当前部署的本地数据库。请勿分享数据库文件；团队部署建议启用密钥加密。</p>
        </details>
        {modelCapabilityCatalog.map((capability) => {
          const saved = savedProfiles[capability.id] || {};
          const selectedModel = capability.models.find(
            (model) => model.id === (saved.model || capability.models[0].id),
          );
          return (
            <details className="settings-capability" key={capability.id}>
              <summary>
                <span className={`settings-status-dot ${saved.configured ? "ready" : "missing"}`} />
                <span className="settings-capability-copy">
                  <strong>{capability.title}</strong>
                  <small>{capability.description}</small>
                </span>
                <span className="settings-current-model">
                  <strong>{selectedModel?.label || saved.model || "未选择模型"}</strong>
                  <small>{saved.mode === "byok" ? "自己的 API" : "平台额度"}</small>
                </span>
                <span className={`settings-state ${saved.configured ? "ready" : "missing"}`}>
                  {saved.configured ? "可用" : "未配置"}
                </span>
                <span className="settings-edit-label">编辑</span>
                <ChevronDown size={16} />
              </summary>
              <div className="settings-capability-editor">
                <div className="upload-grid">
                  <label>
                    <span>额度来源</span>
                    <select name={`${capability.id}_mode`} defaultValue={saved.mode || "managed"}>
                      <option value="managed">平台托管</option>
                      <option value="byok">用户自己的 API Key</option>
                    </select>
                  </label>
                  <label>
                    <span>服务商</span>
                    <select name={`${capability.id}_provider`} defaultValue={saved.provider || "dashscope"}>
                      <option value="dashscope">阿里云百炼</option>
                      {["text", "multimodal"].includes(capability.id) ? <option value="openai">OpenAI 兼容接口</option> : null}
                    </select>
                  </label>
                </div>
                <label>
                  <span>模型</span>
                  <select name={`${capability.id}_model`} defaultValue={saved.model || capability.models[0].id}>
                    {capability.models.map((model) => <option key={model.id} value={model.id}>{model.label}</option>)}
                  </select>
                </label>
                <label>
                  <span>API Key</span>
                  <input name={`${capability.id}_api_key`} type="password" placeholder={saved.api_key_mask ? `留空保留 ${saved.api_key_mask}` : "使用自己的 API 时填写"} autoComplete="new-password" />
                </label>
                <details className="settings-advanced">
                  <summary>自定义接口地址</summary>
                  <label>
                    <span>接口地址</span>
                    <input name={`${capability.id}_endpoint`} defaultValue={saved.endpoint || capability.endpoint} />
                  </label>
                </details>
              </div>
            </details>
          );
        })}
        <details className="settings-secondary-section">
          <summary><Activity size={16} /><span><strong>开发者选项</strong><small>{showDebugPanel ? "已开启" : "默认关闭"}</small></span><ChevronDown size={16} /></summary>
          <div className="settings-secondary-body">
          <label className="toggle-row">
            <span>
              <strong>显示开发者模式</strong>
              <small>查看模型输入输出、token、原始响应。</small>
            </span>
            <input
              type="checkbox"
              checked={showDebugPanel}
              onChange={(event) => onDebugPanel(event.target.checked)}
            />
          </label>
          </div>
        </details>
        <details className="settings-secondary-section owner-login-section">
          <summary><ShieldCheck size={16} /><span><strong>运营后台</strong><small>{ownerSession?.authenticated ? "管理员已登录" : "仅所有者可见"}</small></span><ChevronDown size={16} /></summary>
          <div className="settings-secondary-body">
          <div className="section-heading">
            <span>运营后台</span>
            <small>
              {ownerSession?.authenticated ? "管理员已登录" : "仅所有者可见"}
            </small>
          </div>
          {ownerSession?.authenticated ? (
            <div className="owner-authenticated">
              <CheckCircle2 size={18} />
              <div>
                <strong>身份验证有效</strong>
                <small>运营后台入口已显示在侧栏。</small>
              </div>
            </div>
          ) : ownerSession?.configured ? (
            <div className="owner-login-fields">
              <label>
                <span>管理员账号</span>
                <input name="username" autoComplete="username" />
              </label>
              <label>
                <span>管理员密码</span>
                <input
                  name="password"
                  type="password"
                  autoComplete="current-password"
                />
              </label>
              <button
                className="secondary-button"
                type="button"
                disabled={isOwnerLoading}
                onClick={(event) => {
                  const form = event.currentTarget.closest("form");
                  onOwnerLogin({ preventDefault() {}, currentTarget: form });
                }}
              >
                {isOwnerLoading ? "验证中" : "管理员登录"}
              </button>
            </div>
          ) : (
            <div className="security-callout">
              <KeyRound size={18} />
              <div>
                <strong>尚未配置管理员账号</strong>
                <p>
                  在服务器设置 SCRIPT_AGENT_OWNER_USERNAME 和
                  SCRIPT_AGENT_OWNER_PASSWORD 后重启。
                </p>
              </div>
            </div>
          )}
          </div>
        </details>
        <div className="submit-row">
          <div>
            <span>运行时配置</span>
            <small>每类能力独立生效，用户 Key 不会覆盖其他模型</small>
          </div>
          <button className="primary-button" type="submit" disabled={isSaving}>
            {isSaving ? (
              <Loader2 className="spin" size={16} />
            ) : (
              <Settings size={16} />
            )}
            <span>{isSaving ? "保存中" : "保存配置"}</span>
          </button>
        </div>
      </form>
    </section>
  );
}

function OwnerDashboard({ overview, isLoading, error, onRefresh, onLogout }) {
  const totals = overview?.totals || {};
  const metrics = [
    ["创意空间", totals.spaces || 0],
    ["任务总数", totals.jobs || 0],
    ["Agent Runs", totals.runs || 0],
    ["模型调用", totals.model_calls || 0],
    ["Token 总量", formatNumber(totals.total_tokens || 0)],
    ["已发布脚本", totals.published_jobs || 0],
  ];
  return (
    <section className="owner-dashboard">
      <header className="owner-dashboard-head">
        <div>
          <span>OWNER ONLY</span>
          <h1>产品运营总览</h1>
          <p>跨空间查看任务产出、Agent 运行和模型消耗。</p>
        </div>
        <div>
          <button
            className="secondary-button"
            type="button"
            onClick={onRefresh}
            disabled={isLoading}
          >
            <RefreshCw size={15} />
            刷新
          </button>
          <button
            className="secondary-button"
            type="button"
            onClick={onLogout}
            disabled={isLoading}
          >
            退出登录
          </button>
        </div>
      </header>
      {error ? <div className="error-banner">{error}</div> : null}
      {!overview || isLoading ? (
        <div className="debug-empty">
          <Loader2 className="spin" size={26} />
          <strong>正在读取运营数据</strong>
        </div>
      ) : (
        <>
          <div className="owner-metrics">
            {metrics.map(([label, value]) => (
              <article key={label}>
                <span>{label}</span>
                <strong>{value}</strong>
              </article>
            ))}
          </div>
          <div className="owner-dashboard-grid">
            <section>
              <div className="debug-section-head">
                <h2>空间表现</h2>
                <span>{overview.spaces?.length || 0} 个空间</span>
              </div>
              <div className="owner-space-table">
                {overview.spaces?.map((space) => (
                  <article key={space.id}>
                    <div>
                      <strong>{space.title}</strong>
                      <small>
                        {space.runs} 次运行 · {space.model_calls} 次调用
                      </small>
                    </div>
                    <span>{formatNumber(space.total_tokens)} tokens</span>
                    <span
                      className={
                        space.failed_runs ? "owner-failed" : "owner-healthy"
                      }
                    >
                      {space.failed_runs
                        ? `${space.failed_runs} 次失败`
                        : "运行正常"}
                    </span>
                  </article>
                ))}
              </div>
            </section>
            <section>
              <div className="debug-section-head">
                <h2>最近运行</h2>
                <span>{overview.recent_runs?.length || 0} 条</span>
              </div>
              <div className="owner-run-list">
                {overview.recent_runs?.length ? (
                  overview.recent_runs.map((run) => (
                    <article key={run.ID}>
                      <div>
                        <strong>{run.SpaceTitle}</strong>
                        <small>{formatTime(run.StartedAt)}</small>
                      </div>
                      <span
                        className={`debug-call-status ${run.Status === "failed" ? "danger" : "success"}`}
                      >
                        {run.Status === "failed"
                          ? "失败"
                          : run.Status === "completed"
                            ? "完成"
                            : "运行中"}
                      </span>
                    </article>
                  ))
                ) : (
                  <div className="debug-empty compact">暂无运行记录</div>
                )}
              </div>
            </section>
          </div>
        </>
      )}
    </section>
  );
}

function ModelCallsWorkspace({ calls = [], spaces = [], error }) {
  const [expandedCallId, setExpandedCallId] = useState("");
  const [selectedSpaceId, setSelectedSpaceId] = useState("");
  const [observability, setObservability] = useState({
    runs: [],
    steps: [],
    model_calls: [],
    memory_events: [],
  });
  const [isLoadingObservability, setIsLoadingObservability] = useState(false);
  const [observabilityError, setObservabilityError] = useState("");
  const activeSpaceId = spaces.some((space) => space.id === selectedSpaceId)
    ? selectedSpaceId
    : "";
  const visibleCalls = activeSpaceId ? observability.model_calls || [] : calls;
  const runs = observability.runs || [];
  const runSteps = observability.steps || [];
  const memoryEvents = observability.memory_events || [];
  const totalInput = visibleCalls.reduce(
    (total, call) => total + Number(call.prompt_tokens || 0),
    0,
  );
  const totalOutput = visibleCalls.reduce(
    (total, call) => total + Number(call.output_tokens || 0),
    0,
  );
  const totalLatency = visibleCalls.reduce(
    (total, call) => total + Number(call.latency_ms || 0),
    0,
  );
  const runCount = activeSpaceId
    ? runs.length
    : new Set(visibleCalls.map((call) => call.run_id).filter(Boolean)).size;

  useEffect(() => {
    if (!activeSpaceId) {
      setObservability({
        runs: [],
        steps: [],
        model_calls: [],
        memory_events: [],
      });
      setObservabilityError("");
      setIsLoadingObservability(false);
      return undefined;
    }
    let cancelled = false;
    setIsLoadingObservability(true);
    setObservabilityError("");
    getSpaceObservability(activeSpaceId)
      .then((result) => {
        if (!cancelled) setObservability(result);
      })
      .catch((err) => {
        if (!cancelled) setObservabilityError(err.message);
      })
      .finally(() => {
        if (!cancelled) setIsLoadingObservability(false);
      });
    return () => {
      cancelled = true;
    };
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
        <div className="debug-observer-title">
          <Activity size={20} />
          <strong>开发者模式</strong>
        </div>
        <div className="debug-space-control">
          <label htmlFor="debug-space">运行范围</label>
          <select
            id="debug-space"
            value={activeSpaceId}
            onChange={(event) => {
              setSelectedSpaceId(event.target.value);
              setExpandedCallId("");
            }}
          >
            <option value="">全部运行（含对话）</option>
            {spaces.map((space) => (
              <option value={space.id} key={space.id}>
                {space.title}
              </option>
            ))}
          </select>
          <span>
            {runCount} 次运行 · {visibleCalls.length} 次调用
          </span>
        </div>
      </header>
      {error ? <div className="error-banner">{error}</div> : null}
      {observabilityError ? (
        <div className="error-banner">{observabilityError}</div>
      ) : null}

      <div className="debug-observer-body">
        <div className="debug-intro">
          <h1>Agent Loop 观测</h1>
          <p>查看每次模型请求、响应、Token 与耗时。敏感信息会在服务端脱敏。</p>
        </div>

        <div className="debug-metric-grid">
          {metrics.map(([label, value]) => (
            <div className="debug-metric" key={label}>
              <span>{label}</span>
              <strong>{value}</strong>
            </div>
          ))}
        </div>

        <section className="debug-section">
          <div className="debug-section-head">
            <h2>模型调用</h2>
            <span>{visibleCalls.length} 次</span>
          </div>
          <div className="debug-call-list">
            {isLoadingObservability ? (
              <div className="debug-empty compact">
                <Loader2 className="spin" size={24} />
                <strong>正在加载空间运行记录</strong>
              </div>
            ) : visibleCalls.length ? (
              visibleCalls.map((call, index) => {
                const expanded = expandedCallId === call.id;
                return (
                  <article
                    className={`debug-call ${expanded ? "expanded" : ""}`}
                    key={call.id}
                  >
                    <button
                      type="button"
                      aria-expanded={expanded}
                      onClick={() => setExpandedCallId(expanded ? "" : call.id)}
                    >
                      {expanded ? (
                        <ChevronDown size={18} />
                      ) : (
                        <ChevronRight size={18} />
                      )}
                      <strong>第 {index + 1} 次模型请求</strong>
                      <span className="debug-call-model">
                        {call.scope || "agent"} · {call.step || "模型调用"}
                      </span>
                      <span
                        className={`debug-call-status ${call.error_message ? "danger" : "success"}`}
                      >
                        {call.error_message ? "失败" : "完成"}
                      </span>
                    </button>
                    {expanded ? (
                      <div className="debug-call-detail">
                        <div className="debug-call-meta">
                          <span>
                            输入 {formatNumber(call.prompt_tokens || 0)}
                          </span>
                          <span>
                            输出 {formatNumber(call.output_tokens || 0)}
                          </span>
                          <span>
                            耗时 {formatDuration(call.latency_ms || 0)}
                          </span>
                          <span>{formatTime(call.created_at)}</span>
                        </div>
                        {call.error_message ? (
                          <div className="error-banner">
                            {call.error_message}
                          </div>
                        ) : null}
                        <section>
                          <h3>模型输入</h3>
                          <pre className="result-output json">
                            {pretty(call.input_json)}
                          </pre>
                        </section>
                        <section>
                          <h3>模型输出</h3>
                          <pre className="result-output markdown">
                            {call.output_text || "-"}
                          </pre>
                        </section>
                        <section>
                          <h3>原始响应</h3>
                          <pre className="result-output json">
                            {pretty(call.response_json)}
                          </pre>
                        </section>
                      </div>
                    ) : null}
                  </article>
                );
              })
            ) : (
              <div className="debug-empty compact">
                <CircleHelp size={24} />
                <strong>暂无模型调用记录</strong>
                <p>
                  {activeSpaceId
                    ? "这个创意空间还没有运行记录。"
                    : "运行 Agent 后，请求详情会实时显示在这里。"}
                </p>
              </div>
            )}
          </div>
        </section>

        <section className="debug-section memory-section">
          <div className="debug-section-head">
            <h2>运行步骤</h2>
            <span>{runSteps.length} 条</span>
          </div>
          {runSteps.length ? (
            <div className="debug-memory-list">
              {runSteps.map((step) => (
                <article key={step.id}>
                  <strong>
                    {step.index}. {step.key}
                  </strong>
                  <span>
                    {step.output_summary || step.input_summary || "-"}
                  </span>
                  <time>
                    {step.status} · {formatTime(step.started_at)}
                  </time>
                </article>
              ))}
            </div>
          ) : (
            <div className="debug-empty">
              <CircleHelp size={28} />
              <strong>暂无运行步骤</strong>
              <p>
                {activeSpaceId
                  ? "任务开始后会按 Run 记录工作流步骤。"
                  : "选择创意空间后，可查看该空间的工作流步骤。"}
              </p>
            </div>
          )}
        </section>

        <section className="debug-section memory-section">
          <div className="debug-section-head">
            <h2>Memory 行为</h2>
            <span>{memoryEvents.length} 条</span>
          </div>
          {memoryEvents.length ? (
            <div className="debug-memory-list">
              {memoryEvents.map((event) => (
                <article key={event.id}>
                  <strong>{event.kind}</strong>
                  <span>{event.payload || "-"}</span>
                  <time>{formatTime(event.created_at)}</time>
                </article>
              ))}
            </div>
          ) : (
            <div className="debug-empty">
              <CircleHelp size={28} />
              <strong>本次运行没有 Memory 事件</strong>
              <p>
                {activeSpaceId
                  ? "挂载、提取、Dream、同步和冲突会在这里实时显示。"
                  : "选择创意空间后，可查看该空间的 Memory 行为。"}
              </p>
            </div>
          )}
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
            {runningStatuses.has(job.status) ? (
              <Clock3 size={13} />
            ) : (
              <CheckCircle2 size={13} />
            )}
            {statusLabel(job.status)}
          </span>
          <button
            className="secondary-button"
            type="button"
            onClick={onRetry}
            disabled={!canRetry || isRetrying}
          >
            {isRetrying ? (
              <Loader2 className="spin" size={16} />
            ) : (
              <RotateCcw size={16} />
            )}
            <span>重试</span>
          </button>
        </div>
      ) : null}
    </div>
  );
}

function ResultContent({ job, content, activeTab, isGeneratingVideoPrompts }) {
  if (!job) return <EmptyState text="暂无任务结果" />;
  if (job.error_message && job.status === "failed")
    return (
      <pre className="result-output error-output">{job.error_message}</pre>
    );
  if (activeTab === "video_prompts" && isGeneratingVideoPrompts)
    return <EmptyState text="正在生成 Seedance 视频提示词" />;
  if (!content)
    return (
      <EmptyState
        text={runningStatuses.has(job.status) ? "任务执行中" : "当前页暂无内容"}
      />
    );
  if (activeTab === "run_log") return <TimelineContent content={content} />;
  if (activeTab === "analysis_markdown")
    return <MarkdownContent content={content} />;
  if (activeTab === "video_prompts")
    return <MarkdownContent content={content} />;
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
                <h3>
                  {script.metadata?.title ||
                    script.title ||
                    (activeTab === "replica_script_json"
                      ? "复刻脚本"
                      : "裂变脚本")}
                </h3>
                <p>
                  {script.metadata?.fission_dimension ||
                    script.metadata?.industry ||
                    "分镜脚本"}
                </p>
              </div>
              <strong>
                {Array.isArray(script.storyboards)
                  ? script.storyboards.length
                  : 0}{" "}
                镜
              </strong>
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
      blocks.push({
        type: "heading",
        level: heading[1].length,
        text: heading[2],
      });
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
  if (activeTab === "replica_script_json" && Array.isArray(parsed.storyboards))
    return [parsed];
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

function formatBytes(value) {
  const bytes = Number(value || 0);
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

function formatChatTime(value) {
  if (!value) return "-";
  const date = new Date(value);
  const part = (number) => String(number).padStart(2, "0");
  return `${date.getFullYear()}/${part(date.getMonth() + 1)}/${part(date.getDate())} ${part(date.getHours())}:${part(date.getMinutes())}:${part(date.getSeconds())}`;
}

function reportConfigLabel(raw) {
  const config = parseJSONValue(raw) || {};
  return (
    [config.date_range, config.media, config.country, config.sort_metric]
      .filter(Boolean)
      .join(" · ") || "创意策略报告"
  );
}
