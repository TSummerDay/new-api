const PRO_RESOLUTIONS = ["480P", "720P", "1080P", "2K", "4K"];
// Lite bills input images separately, serves 768P in place of 720P, and fails
// with InternalError on asset:// materials, so real-person retries cannot help.
const VIDEO_MODELS = {
  "wand-vega-video-1.0-lite": {
    version: "1.0-lite",
    resolutions: ["480P", "768P", "1080P", "2K", "4K"],
    defaultResolution: "768P",
    minDuration: 4,
    maxDuration: 15,
    billsInputImages: true,
    rejectsAssets: true,
  },
  "wand-vega-video-1.0-pro": { version: "1.0-pro", resolutions: PRO_RESOLUTIONS, defaultResolution: "720P", minDuration: 4, maxDuration: 15 },
  "wand-vega-video-1.0-pro-fast": { version: "1.0-pro-fast", resolutions: PRO_RESOLUTIONS, defaultResolution: "720P", minDuration: 4, maxDuration: 15 },
  "wand-vega-video-1.0-pro-mini": { version: "1.0-pro-mini", resolutions: PRO_RESOLUTIONS, defaultResolution: "720P", minDuration: 4, maxDuration: 15 },
  "wand-vega-video-1.5-pro": { version: "1.5-pro", resolutions: PRO_RESOLUTIONS, defaultResolution: "720P", minDuration: 4, maxDuration: 30 },
};
const MODEL_PREFIX = "wand-vega-video-";
const MATERIAL_MODEL = "wand-vega-material";
const ASPECT_RATIOS = ["21:9", "16:9", "4:3", "1:1", "3:4", "9:16", "adaptive"];
const VOD_VERSION = "2018-07-17";
const DEFAULT_DURATION = 5;
const DEFAULT_MATERIAL_GROUP = "new-api-vega";
const REAL_PERSON_ERROR = "may contain real person";

function secondsField() {
  return { type: "number", unit: "second", description: { en: "Video generation unit price", zh: "视频生成单价" } };
}

function usageSchema(profile) {
  const labels = {};
  for (const resolution of profile.resolutions) labels[resolution] = { en: resolution, zh: resolution };
  const schema = {
    seconds: secondsField(),
    resolution: {
      enum: profile.resolutions,
      enumLabels: labels,
      description: { en: "Output video resolution", zh: "输出视频分辨率" },
    },
    generate_audio: {
      type: "boolean",
      description: { en: "Whether audio is generated", zh: "是否生成音频" },
    },
  };
  if (profile.billsInputImages) {
    schema.image_count = {
      type: "number",
      unit: "count",
      unitLabel: { en: "image", zh: "张" },
      description: { en: "Input image unit price", zh: "输入图片单价" },
    };
    return schema;
  }
  schema.video_input = {
    enum: ["none", "video"],
    enumLabels: { none: { en: "No reference video", zh: "无参考视频" }, video: { en: "With reference video", zh: "有参考视频" } },
    description: { en: "Reference video input", zh: "参考视频输入" },
  };
  return schema;
}

function usageExamples(profile) {
  const base = profile.defaultResolution;
  const extra = profile.billsInputImages ? { image_count: 0 } : { video_input: "none" };
  const facts = (overrides) => Object.assign({ seconds: 5, resolution: base, generate_audio: false }, extra, overrides);
  const examples = [
    { label: base + " 5s", facts: facts({}) },
    { label: "1080P 5s", facts: facts({ resolution: "1080P" }) },
    { label: base + " 5s · audio", facts: facts({ generate_audio: true }) },
    { label: "480P 5s", facts: facts({ resolution: "480P" }) },
  ];
  if (profile.billsInputImages) {
    examples.push({ label: base + " 5s · 6 images", facts: facts({ image_count: 6 }) });
    examples.push({ label: base + " 5s in + 5s out · reference video", facts: facts({ seconds: 10 }) });
  } else {
    examples.push({ label: base + " 5s in + 5s out · reference video", facts: facts({ seconds: 10, video_input: "video" }) });
  }
  return examples;
}

function capabilityUsageProfiles() {
  const profiles = [];
  for (const model of Object.keys(VIDEO_MODELS)) {
    const schema = usageSchema(VIDEO_MODELS[model]);
    const examples = usageExamples(VIDEO_MODELS[model]);
    const shape = JSON.stringify([schema, examples]);
    let profile = profiles.find((entry) => entry.shape === shape);
    if (!profile) {
      profile = { shape: shape, models: [], schema: schema, examples: examples };
      profiles.push(profile);
    }
    profile.models.push(model);
  }
  const listed = profiles.map((profile) => ({ models: profile.models, schema: profile.schema, examples: profile.examples }));
  listed.push({
    models: [MATERIAL_MODEL],
    schema: materialUsageSchema(),
    examples: [{ label: "1 material", facts: { material_count: 1 } }],
  });
  return listed;
}

function materialUsageSchema() {
  return {
    material_count: {
      type: "number",
      unit: "count",
      description: { en: "Material registration unit price", zh: "素材注册单价" },
    },
  };
}

export const meta = {
  apiVersion: 1,
  key: "tencent-vega",
  name: "WAND Vega",
  icon: "Hunyuan.Color",
  description: {
    en: "Tencent Cloud VOD WAND Vega video generation",
    zh: "腾讯云点播 WAND Vega 视频生成",
  },
  version: "1.0.0",
  author: { name: "QuantumNous" },
  website: "https://cloud.tencent.com/document/product/266",
  baseUrl: "https://vod.tencentcloudapi.com",
  allowedHosts: ["vod.intl.tencentcloudapi.com"],
  models: Object.keys(VIDEO_MODELS).concat([MATERIAL_MODEL]),
  fetchMode: "per_task",
  upstreams: ["vendor", "new_api"],
  usageSchema: usageSchema(VIDEO_MODELS["wand-vega-video-1.0-pro"]),
  usageExamples: usageExamples(VIDEO_MODELS["wand-vega-video-1.0-pro"]),
  usageProfiles: capabilityUsageProfiles(),
  routes: [
    { method: "POST", path: "/wand-vega/aigc-video-task", type: "submit", decode: "createTask", render: "taskCreated" },
    { method: "POST", path: "/wand-vega/aigc-material", type: "submit", decode: "createMaterial", render: "taskCreated" },
    { method: "GET", path: "/wand-vega/task/:task_id", type: "query", render: "taskStatus" },
  ],
  protocols: [
    { name: "openai_responses", supports: ["stream", "sync", "background"] },
    { name: "openai_video", models: Object.keys(VIDEO_MODELS) },
  ],
};

function trimmed(value) {
  return String(value || "").trim();
}

function plainObject(value) {
  return value && typeof value === "object" && !Array.isArray(value) ? value : null;
}

function viaGateway(ctx) {
  return !!(ctx && ctx.upstream && ctx.upstream.kind === "new_api");
}

function modelProfile(model) {
  const name = clientModel(model);
  return VIDEO_MODELS[name] || VIDEO_MODELS["wand-vega-video-1.0-pro"];
}

function clientModel(model) {
  const name = trimmed(model);
  if (VIDEO_MODELS[name]) return name;
  if (name && VIDEO_MODELS[MODEL_PREFIX + name]) return MODEL_PREFIX + name;
  return name;
}

function modelVersion(model) {
  const name = clientModel(model);
  if (VIDEO_MODELS[name]) return VIDEO_MODELS[name].version;
  if (name.indexOf(MODEL_PREFIX) === 0) return name.slice(MODEL_PREFIX.length);
  return name || "1.0-pro";
}

function resolutionForShortSide(shortSide) {
  if (shortSide >= 2160) return "4K";
  if (shortSide >= 1440) return "2K";
  if (shortSide >= 1080) return "1080P";
  if (shortSide >= 720) return "720P";
  if (shortSide > 0) return "480P";
  return "";
}

// 720P and 768P are one price tier; each model accepts only its own label.
function normalizeResolution(value, model) {
  let raw = trimmed(value).toUpperCase().replace(/\s+/g, "");
  if (!raw) return "";
  if (raw === "480" || raw === "720" || raw === "768" || raw === "1080") raw += "P";
  const size = raw.split("X");
  if (size.length === 2 && Number(size[0]) > 0 && Number(size[1]) > 0) {
    raw = resolutionForShortSide(Math.min(Number(size[0]), Number(size[1])));
  }
  const resolutions = modelProfile(model).resolutions;
  if (raw === "720P" && resolutions.indexOf("720P") < 0 && resolutions.indexOf("768P") >= 0) return "768P";
  if (raw === "768P" && resolutions.indexOf("768P") < 0 && resolutions.indexOf("720P") >= 0) return "720P";
  return raw;
}

function nativeBody(ctx) {
  if (!ctx.body || ctx.body.kind !== "json") throw new Error("JSON body required");
  const body = plainObject(ctx.body.value);
  if (!body) throw new Error("request body must be an object");
  return body;
}

function materialSourceURL(body) {
  const file = plainObject(body && body.FileInfo) || {};
  return trimmed((body && (body.Url || body.url)) || file.Url || file.url);
}

function materialAssetType(body, url) {
  const raw = trimmed((body && (body.AssetType || body.asset_type)) || (plainObject(body && body.FileInfo) || {}).Category).toLowerCase();
  if (raw === "image" || raw === "img") return "Image";
  if (raw === "video") return "Video";
  if (/\.(mp4|mov|webm|m4v)(\?|$)/i.test(url)) return "Video";
  if (/\.(png|jpe?g|webp|bmp|gif)(\?|$)/i.test(url)) return "Image";
  return "";
}

function decodeMaterial(body) {
  const url = materialSourceURL(body);
  if (!url) throw new Error("Url is required");
  if (url.indexOf("asset://") === 0) throw new Error("Url is already an asset reference");
  if (url.indexOf("http://") !== 0 && url.indexOf("https://") !== 0) throw new Error("Url must be an http or https URL");
  const assetType = materialAssetType(body, url);
  if (!assetType) throw new Error("AssetType must be Image or Video");
  const requestBody = { model: MATERIAL_MODEL, Url: url, AssetType: assetType };
  const name = trimmed(body.AssetName || body.asset_name);
  if (name) requestBody.AssetName = name;
  const groupId = trimmed(body.GroupId || body.group_id);
  if (groupId) requestBody.GroupId = groupId;
  if (body.IsRealPerson === true || body.IsRealPerson === "True" || body.IsRealPerson === "true" || body.is_real_person === true) {
    requestBody.IsRealPerson = true;
  }
  return { kind: "submit", model: MATERIAL_MODEL, action: "register_material", requestBody: requestBody };
}

function fileURL(item, key) {
  if (!item || typeof item !== "object" || Array.isArray(item)) return "";
  if (trimmed(item[key])) {
    const value = item[key];
    if (typeof value === "string") return trimmed(value);
    if (value && typeof value === "object" && !Array.isArray(value)) return trimmed(value.url);
  }
  if (trimmed(item.url)) return trimmed(item.url);
  return "";
}

function usageForRole(role, category) {
  const name = trimmed(role).toLowerCase();
  if (name === "first_frame" || name === "firstframe") return "FirstFrame";
  if (name === "last_frame" || name === "lastframe") return "LastFrame";
  if (category === "Video" || category === "Audio") return "Reference";
  return "Reference";
}

function categoryForType(type) {
  const name = trimmed(type).toLowerCase();
  if (name === "video_url" || name === "video") return "Video";
  if (name === "audio_url" || name === "audio") return "Audio";
  return "Image";
}

function contentToFileInfos(content) {
  if (!Array.isArray(content)) return [];
  const files = [];
  for (const item of content) {
    if (!item || typeof item !== "object" || Array.isArray(item)) continue;
    const type = trimmed(item.type).toLowerCase();
    if (type === "text") continue;
    const category = categoryForType(type || item.Category);
    const url = fileURL(item, type === "video_url" ? "video_url" : type === "audio_url" ? "audio_url" : "image_url") || trimmed(item.Url);
    if (!url) continue;
    files.push({ Type: "Url", Category: category, Url: url, Usage: usageForRole(item.role || item.Usage, category) });
  }
  return files;
}

function metadataFileInfos(metadata) {
  if (Array.isArray(metadata.FileInfos)) return metadata.FileInfos;
  const files = [];
  if (trimmed(metadata.first_frame_image) || trimmed(metadata.first_frame)) {
    files.push({ Type: "Url", Category: "Image", Url: trimmed(metadata.first_frame_image || metadata.first_frame), Usage: "FirstFrame" });
  }
  if (trimmed(metadata.last_frame_image) || trimmed(metadata.last_frame)) {
    files.push({ Type: "Url", Category: "Image", Url: trimmed(metadata.last_frame_image || metadata.last_frame), Usage: "LastFrame" });
  }
  return files;
}

function requestFileInfos(req) {
  const metadata = plainObject(req.metadata) || {};
  const fromContent = contentToFileInfos(metadata.content);
  if (fromContent.length) return fromContent;
  if (Array.isArray(req.FileInfos)) return req.FileInfos;
  return metadataFileInfos(metadata);
}

function hasVideoInput(files) {
  return files.some((file) => file && (file.Category === "Video" || trimmed(file.Category).toLowerCase() === "video"));
}

function actionForFiles(files) {
  const usages = files.map((file) => trimmed(file && file.Usage));
  if (usages.indexOf("FirstFrame") >= 0 && usages.indexOf("LastFrame") >= 0) return "first_last_to_video";
  if (usages.indexOf("FirstFrame") >= 0 || usages.indexOf("LastFrame") >= 0) return "image_to_video";
  if (files.length) return "reference_to_video";
  return "text_to_video";
}

function parseSeconds(value) {
  if (value === undefined || value === null || value === "") return undefined;
  const n = Number(value);
  if (!Number.isFinite(n)) throw new Error("seconds must be -1 or a positive number");
  return n;
}

function validateRequest(model, seconds, resolution, files) {
  const profile = modelProfile(model);
  const name = clientModel(model) || model;
  if (seconds !== undefined && seconds !== -1 && (seconds < profile.minDuration || seconds > profile.maxDuration)) {
    throw new Error(name + " duration must be -1 or between " + profile.minDuration + " and " + profile.maxDuration);
  }
  if (resolution && profile.resolutions.indexOf(resolution) < 0) {
    throw new Error(name + " resolution must be " + profile.resolutions.join(", "));
  }
  let images = 0,
    videos = 0,
    audios = 0;
  for (const file of files) {
    const category = trimmed(file && file.Category);
    if (category === "Video") videos += 1;
    else if (category === "Audio") audios += 1;
    else images += 1;
  }
  if (profile.version.indexOf("1.5") === 0) {
    if (images > 30 || videos > 10 || audios > 10 || files.length > 50) throw new Error("too many reference files for " + name);
  } else if (images > 9 || videos > 3 || audios > 3 || files.length > 12) {
    throw new Error("too many reference files for " + name);
  }
  if (audios > 0 && images + videos === 0) throw new Error("audio references must be paired with an image or video");
  if (profile.rejectsAssets && files.some((file) => trimmed(file && file.Url).indexOf("asset://") === 0)) {
    throw new Error(name + " does not accept asset:// materials");
  }
}

function draftTaskIds(req) {
  const metadata = plainObject(req.metadata) || {};
  const id = trimmed(metadata.draft_task_id || req.draft_task_id);
  return id ? [id] : [];
}

function decodeSubmit(req, model) {
  const metadata = Object.assign({}, plainObject(req.metadata) || {});
  if (Array.isArray(req.content) && !metadata.content) metadata.content = req.content;
  const seconds = parseSeconds(req.seconds !== undefined ? req.seconds : req.duration !== undefined ? req.duration : metadata.duration);
  const resolution = normalizeResolution(req.size || req.resolution || metadata.resolution, model);
  const requestBody = { model: model, prompt: trimmed(req.prompt || req.Prompt), metadata: metadata };
  if (seconds !== undefined) requestBody.seconds = seconds;
  if (resolution) requestBody.resolution = resolution;
  if (Array.isArray(req.FileInfos)) requestBody.FileInfos = req.FileInfos;
  const files = requestFileInfos(requestBody);
  validateRequest(model, seconds, resolution, files);
  if (!requestBody.prompt && files.length === 0 && !trimmed(metadata.draft_task_id)) throw new Error("prompt or reference files are required");
  const originTaskIds = draftTaskIds(requestBody);
  const intent = {
    kind: "submit",
    model: model,
    action: actionForFiles(files),
    requestBody: requestBody,
  };
  if (originTaskIds.length) intent.originTaskIds = originTaskIds;
  return intent;
}

function responsesInput(req) {
  const texts = [];
  const files = [];
  const input = req.input;
  if (typeof input === "string") texts.push(input);
  else if (Array.isArray(input)) {
    for (const item of input) {
      if (typeof item === "string") {
        texts.push(item);
        continue;
      }
      if (!item || typeof item !== "object" || Array.isArray(item)) continue;
      const content = item.content === undefined ? [item] : Array.isArray(item.content) ? item.content : [item.content];
      for (const part of content) {
        if (typeof part === "string") {
          texts.push(part);
          continue;
        }
        if (!part || typeof part !== "object" || Array.isArray(part)) continue;
        if (["input_text", "text"].includes(part.type) && typeof part.text === "string") texts.push(part.text);
        if (["input_image", "image_url"].includes(part.type)) {
          const url = fileURL(part, "image_url");
          if (url) files.push({ type: "image_url", image_url: { url: url }, role: "reference" });
        }
      }
    }
  }
  return { prompt: texts.filter(trimmed).join("\n"), content: files };
}

export const native = {
  createTask: function (ctx) {
    const body = nativeBody(ctx);
    const model = clientModel(body.model || body.ModelVersion || ctx.model);
    if (!model) throw new Error("model is required");
    if (model === MATERIAL_MODEL) throw new Error("use POST /wand-vega/aigc-material to register materials");
    const req = Object.assign({}, body, {
      prompt: body.prompt || body.Prompt,
      metadata: Object.assign({}, plainObject(body.metadata) || {}, {
        FileInfos: body.FileInfos,
        resolution: body.resolution || (plainObject(body.OutputConfig) || {}).Resolution,
        duration: body.duration || (plainObject(body.OutputConfig) || {}).Duration,
        generate_audio: (plainObject(body.OutputConfig) || {}).AudioGeneration === "Enabled" ? true : body.generate_audio,
        draft_task_id: (plainObject(body.metadata) || {}).draft_task_id,
      }),
    });
    return decodeSubmit(req, model);
  },
  createMaterial: function (ctx) {
    return decodeMaterial(nativeBody(ctx));
  },
  taskCreated: function (ctx, task) {
    const data = plainObject(task.data) || {};
    const requestId = data.Response && data.Response.RequestId ? data.Response.RequestId : data.RequestId || "";
    return { Response: { RequestId: requestId, TaskId: task.task_id } };
  },
  taskStatus: function (ctx, task) {
    const data = plainObject(task.data) || {};
    const response = Object.assign({}, plainObject(data.Response) || data);
    const material = plainObject(response.CreateAigcMaterialTask);
    if (material && !response.AigcVideoTask) {
      if (task.status === "FAILURE") {
        material.Status = "FAIL";
        material.Message = task.fail_reason || material.Message || "task failed";
        response.Status = "FAIL";
      } else if (!response.Status) {
        response.Status = task.status === "SUCCESS" ? "FINISH" : task.status === "IN_PROGRESS" ? "PROCESSING" : "WAITING";
      }
      const output = Object.assign({}, plainObject(material.Output) || {});
      if (trimmed(output.AssetId) && !trimmed(output.AssetUrl)) output.AssetUrl = "asset://" + trimmed(output.AssetId);
      material.Output = output;
      material.TaskId = task.task_id;
      response.CreateAigcMaterialTask = material;
      response.TaskId = task.task_id;
      return { Response: response };
    }
    if (task.status === "FAILURE") {
      const video = plainObject(response.AigcVideoTask) || {};
      video.Status = "FAIL";
      video.Message = task.fail_reason || video.Message || "task failed";
      response.AigcVideoTask = video;
      response.Status = "FAIL";
    } else if (!response.Status) {
      response.Status = task.status === "SUCCESS" ? "FINISH" : task.status === "IN_PROGRESS" ? "PROCESSING" : "WAITING";
    }
    if (response.AigcVideoTask) response.AigcVideoTask.TaskId = task.task_id;
    response.TaskId = task.task_id;
    return { Response: response };
  },
  error: function (ctx, error) {
    return { Response: { Error: { Code: error.code || "FailedOperation", Message: error.message } } };
  },
};

function vendorCredentials(ctx) {
  const parts = String(ctx.apiKey || "").split("|");
  if (parts.length < 3 || !trimmed(parts[0]) || !trimmed(parts[1]) || !trimmed(parts[2])) {
    throw new Error("invalid api key format for wand-vega: expected 'SecretId|SecretKey|SubAppId'");
  }
  const subAppId = Number(parts[2]);
  if (!Number.isInteger(subAppId) || subAppId <= 0) throw new Error("SubAppId must be a positive integer");
  const host = String(ctx.baseUrl || "").toLowerCase();
  const region = trimmed(parts[3]);
  return {
    secretId: trimmed(parts[0]),
    secretKey: trimmed(parts[1]),
    subAppId: subAppId,
    region: region || (host.indexOf("intl.tencentcloudapi.com") >= 0 ? "ap-singapore" : ""),
    groupId: trimmed(parts[4]) || DEFAULT_MATERIAL_GROUP,
  };
}

function signedHeaders(ctx, method, url, action, bodyText) {
  if (viaGateway(ctx)) {
    return { "Content-Type": "application/json", Accept: "application/json", Authorization: "Bearer " + ctx.apiKey };
  }
  const creds = vendorCredentials(ctx);
  const signed = utils.tencentSignTC3({
    Method: method,
    URL: url,
    Headers: { "Content-Type": "application/json", "X-TC-Action": action },
    Body: bodyText,
    SecretId: creds.secretId,
    SecretKey: creds.secretKey,
    Service: "vod",
  });
  const headers = {
    Accept: "application/json",
    "X-TC-Action": action,
    "X-TC-Version": VOD_VERSION,
  };
  if (creds.region) headers["X-TC-Region"] = creds.region;
  for (const key of Object.keys(signed)) headers[key] = signed[key];
  return headers;
}

function extInfo(metadata, originTasks) {
  const additional = {};
  const wand = { ReturnVegaUsage: true };
  if (metadata.return_last_frame === true) additional.return_last_frame = true;
  if (metadata.draft === true) additional.draft = true;
  const draftId = trimmed(metadata.draft_task_id);
  if (draftId) {
    const origin = Array.isArray(originTasks) ? originTasks.find((task) => task && (task.taskId === draftId || task.upstreamTaskId === draftId)) : null;
    additional.draft_task_id = origin && origin.upstreamTaskId ? origin.upstreamTaskId : draftId;
  }
  if (trimmed(metadata.omni_reference_task_type)) additional.omni_reference_task_type = trimmed(metadata.omni_reference_task_type);
  if (trimmed(metadata.EnhancePromptMode || metadata.enhance_prompt_mode)) {
    wand.EnhancePromptMode = trimmed(metadata.EnhancePromptMode || metadata.enhance_prompt_mode);
  }
  if (trimmed(metadata.ModerationMode || metadata.moderation_mode)) {
    wand.ModerationMode = trimmed(metadata.ModerationMode || metadata.moderation_mode);
  }
  const info = { WandVegaParameters: JSON.stringify(wand) };
  if (Object.keys(additional).length) info.AdditionalParameters = JSON.stringify(additional);
  return JSON.stringify(info);
}

function outboundSeconds(req, model) {
  const metadata = plainObject(req.metadata) || {};
  const seconds = parseSeconds(req.seconds !== undefined ? req.seconds : req.duration !== undefined ? req.duration : metadata.duration);
  if (seconds === undefined) return DEFAULT_DURATION;
  if (seconds === -1) return modelProfile(model).maxDuration;
  return seconds;
}

function outboundResolution(req, model) {
  const metadata = plainObject(req.metadata) || {};
  return normalizeResolution(req.resolution || req.size || metadata.resolution, model) || modelProfile(model).defaultResolution;
}

function inputImageCount(files) {
  return files.filter((file) => {
    const category = trimmed(file && file.Category);
    return category !== "Video" && category !== "Audio";
  }).length;
}

function generateAudio(req) {
  const metadata = plainObject(req.metadata) || {};
  if (metadata.generate_audio === true || metadata.AudioGeneration === "Enabled") return true;
  if (req.generate_audio === true) return true;
  return false;
}

function vendorBody(ctx) {
  const req = ctx.requestBody || {};
  const metadata = plainObject(req.metadata) || {};
  const model = ctx.upstreamModel || req.model;
  const files = requestFileInfos(req);
  const output = { StorageMode: metadata.StorageMode || "Temporary" };
  output.Duration = outboundSeconds(req, model);
  output.Resolution = outboundResolution(req, model);
  const ratio = trimmed(metadata.ratio || metadata.aspect_ratio || req.aspect_ratio);
  if (ASPECT_RATIOS.indexOf(ratio) >= 0) output.AspectRatio = ratio;
  if (metadata.generate_audio === true || req.generate_audio === true) output.AudioGeneration = "Enabled";
  if (metadata.generate_audio === false || req.generate_audio === false) output.AudioGeneration = "Disabled";
  const body = {
    ModelName: "wand-vega-video",
    ModelVersion: modelVersion(model),
    Prompt: trimmed(req.prompt || req.Prompt || metadata.prompt),
    OutputConfig: output,
    ExtInfo: extInfo(metadata, ctx.originTasks),
  };
  if (!viaGateway(ctx)) body.SubAppId = vendorCredentials(ctx).subAppId;
  if (files.length) body.FileInfos = files;
  if (metadata.Seed !== undefined) body.Seed = metadata.Seed;
  else if (req.seed !== undefined) body.Seed = req.seed;
  return body;
}

function materialSubmitPayload(ctx) {
  const creds = vendorCredentials(ctx);
  const req = plainObject(ctx.requestBody) || {};
  const groupId = trimmed(req.GroupId) || creds.groupId;
  const name = trimmed(req.AssetName) || trimmed(ctx.publicTaskId || "vega") + "_" + String(utils.uuid()).replace(/-/g, "").slice(0, 8);
  return {
    SubAppId: creds.subAppId,
    FileInfo: { Type: "Url", Url: trimmed(req.Url) },
    AssetType: trimmed(req.AssetType) || "Image",
    GroupId: groupId,
    GroupName: groupId,
    AssetName: name,
    IsRealPerson: req.IsRealPerson === true ? "True" : "False",
  };
}

export function buildSubmitRequest(ctx) {
  if (ctx.action === "register_material") {
    if (viaGateway(ctx)) {
      return {
        url: ctx.baseUrl + "/wand-vega/aigc-material",
        method: "POST",
        headers: { "Content-Type": "application/json", Accept: "application/json", Authorization: "Bearer " + ctx.apiKey },
        body: ctx.requestBody || {},
      };
    }
    return vendorActionRequest(ctx, "CreateAigcMaterial", materialSubmitPayload(ctx));
  }
  if (viaGateway(ctx)) {
    return {
      url: ctx.baseUrl + "/wand-vega/aigc-video-task",
      method: "POST",
      headers: { "Content-Type": "application/json", Accept: "application/json", Authorization: "Bearer " + ctx.apiKey },
      body: ctx.requestBody || {},
    };
  }
  const bodyText = JSON.stringify(vendorBody(ctx));
  const url = ctx.baseUrl.replace(/\/+$/, "") + "/";
  return {
    url: url,
    method: "POST",
    headers: signedHeaders(ctx, "POST", url, "CreateAigcVideoTask", bodyText),
    body: bodyText,
    action: actionForFiles(requestFileInfos(ctx.requestBody || {})),
  };
}

function responseError(body) {
  const response = plainObject(body && body.Response) || plainObject(body) || {};
  const error = plainObject(response.Error);
  if (!error) return "";
  return trimmed(error.Message || error.Code);
}

function taskIdFrom(body) {
  const response = plainObject(body && body.Response) || plainObject(body) || {};
  return trimmed(response.TaskId || body.task_id || body.id);
}

export function parseSubmitResponse(ctx, resp) {
  const body = resp.body || {};
  const message = responseError(body);
  if (message) throw new Error(message);
  const taskId = taskIdFrom(body);
  if (!taskId) throw new Error("missing TaskId");
  return { taskId: taskId, taskData: body };
}

export function extractUsage(ctx) {
  if (ctx.usagePurpose === "billing_ratios") return null;
  if (ctx.action === "register_material") return { material_count: 1 };
  const req = ctx.requestBody || {};
  const model = ctx.upstreamModel || req.model;
  const files = requestFileInfos(req);
  const facts = {
    seconds: outboundSeconds(req, model),
    resolution: outboundResolution(req, model),
    generate_audio: generateAudio(req),
  };
  if (modelProfile(model).billsInputImages) facts.image_count = inputImageCount(files);
  else facts.video_input = hasVideoInput(files) ? "video" : "none";
  return facts;
}

function vendorActionRequest(ctx, action, payload) {
  const bodyText = JSON.stringify(payload);
  const url = ctx.baseUrl.replace(/\/+$/, "") + "/";
  return {
    url: url,
    method: "POST",
    headers: signedHeaders(ctx, "POST", url, action, bodyText),
    body: bodyText,
  };
}

function materialFile(state) {
  const pending = Array.isArray(state && state.pending) ? state.pending : [];
  return pending[0] && typeof pending[0] === "object" ? pending[0] : null;
}

function retryVideoPayload(state, subAppId) {
  const input = plainObject(state && state.videoInput) || {};
  const assets = {};
  for (const item of state && state.assets ? state.assets : []) {
    if (item && item.url && item.asset) assets[item.url] = item.asset;
  }
  const files = [];
  for (const file of Array.isArray(input.FileInfos) ? input.FileInfos : []) {
    if (!file) continue;
    const url = trimmed(file.Url);
    files.push({
      Type: trimmed(file.Type) || "Url",
      Category: trimmed(file.Category),
      Url: assets[url] || url,
      Usage: trimmed(file.Usage),
    });
  }
  const body = {
    SubAppId: subAppId,
    ModelName: trimmed(input.ModelName) || "wand-vega-video",
    ModelVersion: trimmed(input.ModelVersion),
    Prompt: trimmed(input.Prompt),
    OutputConfig: Object.assign({}, plainObject(input.OutputConfig) || {}),
    FileInfos: files,
  };
  if (trimmed(input.ExtInfo)) body.ExtInfo = input.ExtInfo;
  return body;
}

export function buildQueryRequest(ctx) {
  if (viaGateway(ctx)) {
    return {
      url: ctx.baseUrl + "/wand-vega/task/" + encodeURIComponent(ctx.taskId),
      method: "GET",
      headers: { Accept: "application/json", Authorization: "Bearer " + ctx.apiKey },
    };
  }
  const creds = vendorCredentials(ctx);
  const state = plainObject(ctx.state) || {};
  if (state.phase === "create_material") {
    const file = materialFile(state);
    if (!file || !trimmed(file.Url)) throw new Error("missing material file");
    return vendorActionRequest(ctx, "CreateAigcMaterial", {
      SubAppId: creds.subAppId,
      FileInfo: { Type: "Url", Url: trimmed(file.Url) },
      AssetType: trimmed(file.Category) || "Image",
      GroupId: creds.groupId,
      GroupName: creds.groupId,
      AssetName: trimmed(ctx.publicTaskId || "vega") + "_" + String(utils.uuid()).replace(/-/g, "").slice(0, 8),
      IsRealPerson: "False",
    });
  }
  if (state.phase === "wait_material") {
    return vendorActionRequest(ctx, "DescribeTaskDetail", { TaskId: state.materialTaskId, SubAppId: creds.subAppId });
  }
  if (state.phase === "retry_video") {
    return vendorActionRequest(ctx, "CreateAigcVideoTask", retryVideoPayload(state, creds.subAppId));
  }
  return vendorActionRequest(ctx, "DescribeTaskDetail", {
    TaskId: trimmed(state.videoTaskId) || ctx.taskId,
    SubAppId: creds.subAppId,
  });
}

function videoTask(body) {
  const response = plainObject(body && body.Response) || plainObject(body) || {};
  return plainObject(response.AigcVideoTask) || plainObject(body && body.AigcVideoTask);
}

function videoFileURL(task) {
  const output = plainObject(task && task.Output) || {};
  const files = Array.isArray(output.FileInfos) ? output.FileInfos : [];
  for (const file of files) {
    if (!file || trimmed(file.UsageType) === "last_frame_url") continue;
    if (trimmed(file.FileUrl)) return trimmed(file.FileUrl);
  }
  return "";
}

function materialTask(body) {
  const response = plainObject(body && body.Response) || plainObject(body) || {};
  return plainObject(response.CreateAigcMaterialTask) || {};
}

function snapshotVideoInput(task) {
  const input = plainObject(task && task.Input) || {};
  const output = plainObject(input.OutputConfig) || {};
  const files = [];
  for (const file of Array.isArray(input.FileInfos) ? input.FileInfos : []) {
    if (!file) continue;
    files.push({
      Type: trimmed(file.Type) || "Url",
      Category: trimmed(file.Category),
      Url: trimmed(file.Url),
      Usage: trimmed(file.Usage),
    });
  }
  const snapshot = {
    ModelName: trimmed(input.ModelName) || "wand-vega-video",
    ModelVersion: trimmed(input.ModelVersion),
    Prompt: trimmed(input.Prompt),
    FileInfos: files,
    OutputConfig: {},
  };
  if (trimmed(output.StorageMode)) snapshot.OutputConfig.StorageMode = trimmed(output.StorageMode);
  if (output.Duration !== undefined && output.Duration !== null) snapshot.OutputConfig.Duration = output.Duration;
  if (trimmed(output.Resolution)) snapshot.OutputConfig.Resolution = trimmed(output.Resolution);
  if (trimmed(output.AspectRatio)) snapshot.OutputConfig.AspectRatio = trimmed(output.AspectRatio);
  if (trimmed(output.AudioGeneration)) snapshot.OutputConfig.AudioGeneration = trimmed(output.AudioGeneration);
  if (trimmed(input.ExtInfo)) snapshot.ExtInfo = input.ExtInfo;
  return snapshot;
}

function pendingMaterials(files) {
  const pending = [];
  for (const file of files || []) {
    if (!file) continue;
    const category = trimmed(file.Category);
    const url = trimmed(file.Url);
    if ((category !== "Image" && category !== "Video") || !url || url.indexOf("asset://") === 0) continue;
    pending.push({ Category: category, Url: url, Usage: trimmed(file.Usage), Type: trimmed(file.Type) || "Url" });
  }
  return pending;
}

function realPersonMessage(value) {
  return trimmed(value).toLowerCase().indexOf(REAL_PERSON_ERROR) >= 0;
}

function realPersonRetry(ctx, task) {
  if (viaGateway(ctx)) return null;
  if (modelProfile(ctx && (ctx.upstreamModel || ctx.model)).rejectsAssets) return null;
  const state = plainObject(ctx && ctx.state) || {};
  if (state.retried || state.phase) return null;
  const pending = pendingMaterials((plainObject(task && task.Input) || {}).FileInfos);
  if (!pending.length) return null;
  return {
    status: "IN_PROGRESS",
    progress: "50%",
    state: { phase: "create_material", pending: pending, assets: [], videoInput: snapshotVideoInput(task), retried: true },
  };
}

function continueAfterMaterial(state, sourceUrl, assetId) {
  const assets = (state.assets || []).concat([{ url: sourceUrl, asset: "asset://" + assetId }]);
  const pending = (state.pending || []).slice(1);
  if (pending.length) {
    return { status: "IN_PROGRESS", progress: "50%", state: Object.assign({}, state, { phase: "create_material", pending: pending, assets: assets, materialTaskId: "" }) };
  }
  return { status: "IN_PROGRESS", progress: "50%", state: Object.assign({}, state, { phase: "retry_video", pending: [], assets: assets, materialTaskId: "" }) };
}

export function parseTaskResult(ctx, body) {
  const message = responseError(body);
  const state = plainObject(ctx && ctx.state) || {};
  if (state.phase === "create_material") {
    if (message) return { status: "FAILURE", progress: "100%", reason: message };
    const materialTaskId = taskIdFrom(body);
    if (!materialTaskId) return { status: "FAILURE", progress: "100%", reason: "missing material TaskId" };
    return { status: "IN_PROGRESS", progress: "50%", state: Object.assign({}, state, { phase: "wait_material", materialTaskId: materialTaskId }) };
  }
  if (state.phase === "wait_material") {
    if (message) return { status: "FAILURE", progress: "100%", reason: message };
    const event = materialTask(body);
    const statusText = trimmed(event.Status || (plainObject(body && body.Response) || {}).Status).toUpperCase();
    if (statusText === "FAIL" || Number(event.ErrCode || 0) !== 0) {
      return { status: "FAILURE", progress: "100%", reason: trimmed(event.Message) || "material registration failed" };
    }
    if (statusText !== "FINISH") return { status: "IN_PROGRESS", progress: "50%", state: state };
    const assetId = trimmed((plainObject(event.Output) || {}).AssetId);
    if (!assetId) return { status: "FAILURE", progress: "100%", reason: "material has no AssetId" };
    const file = materialFile(state);
    return continueAfterMaterial(state, file ? trimmed(file.Url) : "", assetId);
  }
  if (state.phase === "retry_video") {
    if (message) return { status: "FAILURE", progress: "100%", reason: message };
    const videoTaskId = taskIdFrom(body);
    if (!videoTaskId) return { status: "FAILURE", progress: "100%", reason: "missing TaskId" };
    return { status: "IN_PROGRESS", progress: "50%", state: { phase: "video", videoTaskId: videoTaskId, retried: true } };
  }
  if (ctx && ctx.action === "register_material") {
    if (message) return { status: "FAILURE", progress: "100%", reason: message };
    const event = materialTask(body);
    const materialStatus = trimmed(event.Status || (plainObject(body && body.Response) || {}).Status).toUpperCase();
    const mapped = { WAITING: "QUEUED", PROCESSING: "IN_PROGRESS", FINISH: "SUCCESS", FAIL: "FAILURE", ABORT: "FAILURE" };
    const status = mapped[materialStatus];
    if (!status) return { status: "UNKNOWN", reason: "unrecognized status: " + materialStatus };
    if (status === "FAILURE") return { status: "FAILURE", progress: "100%", reason: trimmed(event.Message) || "material registration failed" };
    if (status === "SUCCESS") {
      const assetId = trimmed((plainObject(event.Output) || {}).AssetId);
      if (!assetId) return { status: "FAILURE", progress: "100%", reason: "material has no AssetId" };
      return { status: "SUCCESS", progress: "100%", url: "asset://" + assetId };
    }
    return { status: status, progress: status === "QUEUED" ? "10%" : "50%" };
  }
  if (message) return { status: "FAILURE", progress: "100%", reason: message };
  const task = videoTask(body);
  const statusText = trimmed((task && task.Status) || (plainObject(body && body.Response) || {}).Status || body.status).toUpperCase();
  const statuses = { WAITING: "QUEUED", PROCESSING: "IN_PROGRESS", FINISH: "SUCCESS", FAIL: "FAILURE", ABORT: "FAILURE" };
  const status = statuses[statusText];
  if (!status) return { status: "UNKNOWN", reason: "unrecognized status: " + statusText };
  const failMessage = trimmed(task && task.Message);
  const url = videoFileURL(task);
  if (status === "SUCCESS" && !url) {
    if (realPersonMessage(failMessage)) {
      const retry = realPersonRetry(ctx, task);
      if (retry) return retry;
    }
    return { status: "FAILURE", progress: "100%", reason: failMessage || "task failed" };
  }
  if (status === "FAILURE" && realPersonMessage(failMessage)) {
    const retry = realPersonRetry(ctx, task);
    if (retry) return retry;
  }
  const result = { status: status, progress: status === "QUEUED" ? "10%" : status === "IN_PROGRESS" ? "50%" : "100%" };
  if (status === "SUCCESS") result.url = url;
  if (status === "FAILURE") result.reason = failMessage || "task failed";
  return result;
}

function artifactSnapshot(ctx) {
  return videoTask(ctx && ctx.data) || plainObject(ctx && ctx.data) || {};
}

export function listArtifacts(task) {
  if (task.status !== "SUCCESS") return [];
  const snapshot = artifactSnapshot(task);
  const artifacts = [];
  if (videoFileURL(snapshot)) artifacts.push({ key: "video", type: "video", mimeType: "video/mp4" });
  const output = plainObject(snapshot.Output) || {};
  const files = Array.isArray(output.FileInfos) ? output.FileInfos : [];
  if (files.some((file) => file && trimmed(file.UsageType) === "last_frame_url" && trimmed(file.FileUrl))) {
    artifacts.push({ key: "last_frame", type: "image", mimeType: "image/png" });
  }
  return artifacts;
}

export function buildContentRequest(ctx) {
  const snapshot = artifactSnapshot(ctx);
  if (ctx.artifactKey === "video") {
    const url = videoFileURL(snapshot);
    if (!url) throw new Error("artifact_not_found");
    return { url: url, method: ctx.clientRequest.method, credentialless: true };
  }
  if (ctx.artifactKey === "last_frame") {
    const output = plainObject(snapshot.Output) || {};
    const files = Array.isArray(output.FileInfos) ? output.FileInfos : [];
    const file = files.find((item) => item && trimmed(item.UsageType) === "last_frame_url");
    const url = file ? trimmed(file.FileUrl) : "";
    if (!url) throw new Error("artifact_not_found");
    return { url: url, method: ctx.clientRequest.method, credentialless: true };
  }
  throw new Error("artifact_not_found");
}

export function extractUsageOnComplete(task, _result, data) {
  if ((task && task.action === "register_material") || (materialTask(data).Status && !videoTask(data))) {
    return { material_count: 1 };
  }
  const parsed = videoTask(data) || plainObject(data) || {};
  const output = plainObject(parsed.Output) || {};
  const files = Array.isArray(output.FileInfos) ? output.FileInfos : [];
  const video = files.find((file) => file && trimmed(file.UsageType) !== "last_frame_url");
  const meta = plainObject(video && video.MetaData) || {};
  const usage = plainObject(output.Usage) || {};
  const facts = {};
  // Reference-video tasks bill input plus output seconds at the output
  // resolution; VOD reports that sum as TotalSeconds.
  let seconds = Number(usage.TotalSeconds);
  if (!(seconds > 0)) seconds = Number(usage.InputSeconds || 0) + Number(usage.OutputSeconds || 0);
  if (!(seconds > 0)) seconds = Number(meta.Duration || meta.VideoDuration);
  if (Number.isFinite(seconds) && seconds > 0 && seconds <= 3600) facts.seconds = seconds;
  const model = task && (task.upstreamModel || task.model);
  const width = Number(meta.Width);
  const height = Number(meta.Height);
  const shortSide = width > 0 && height > 0 ? Math.min(width, height) : height;
  const resolution = normalizeResolution(resolutionForShortSide(shortSide), model);
  if (resolution) facts.resolution = resolution;
  // A zero InputImageCount is also what VOD returns when it omits the count,
  // so only a positive count replaces the images counted at submission.
  const images = Number(usage.InputImageCount);
  if (modelProfile(model).billsInputImages && Number.isInteger(images) && images > 0 && images <= 128) facts.image_count = images;
  return Object.keys(facts).length ? facts : null;
}

function responsesVideoText(ctx) {
  const artifact = ctx && ctx.artifacts && ctx.artifacts.video;
  const url = trimmed(artifact && artifact.url);
  if (!url) throw new Error("video artifact is unavailable");
  const escaped = url.replace(/&/g, "&amp;").replace(/"/g, "&quot;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
  return '<video controls src="' + escaped + '"></video>';
}

const legacyRenderers = {
  openai_video: function (task) {
    const statuses = { NOT_START: "queued", SUBMITTED: "queued", QUEUED: "queued", IN_PROGRESS: "in_progress", SUCCESS: "completed", FAILURE: "failed" };
    const output = {
      id: task.task_id,
      object: "video",
      model: task.properties && task.properties.origin_model_name ? task.properties.origin_model_name : "",
      status: statuses[task.status] || "unknown",
      progress: Number(String(task.progress || "0").replace("%", "")),
      created_at: task.created_at,
    };
    if (task.updated_at) output.completed_at = task.updated_at;
    const message = responseError(task.data) || trimmed(task.fail_reason);
    if (task.status === "FAILURE" && message) output.error = { message: message, code: "task_failed" };
    return output;
  },
};

export const protocols = {
  openai_responses: {
    decodeRequest: function (ctx) {
      if (!ctx.body || ctx.body.kind !== "json") throw new Error("JSON body required");
      const req = ctx.body.value;
      if (!req || typeof req !== "object" || Array.isArray(req)) throw new Error("request body must be an object");
      const model = trimmed(req.model);
      if (!model) throw new Error("model is required");
      if (clientModel(model) === MATERIAL_MODEL) throw new Error("use POST /wand-vega/aigc-material to register materials");
      if (req.input !== undefined && typeof req.input !== "string" && !Array.isArray(req.input)) throw new Error("input must be a string or array");
      if (req.metadata !== undefined && (!req.metadata || typeof req.metadata !== "object" || Array.isArray(req.metadata))) {
        throw new Error("metadata must be an object");
      }
      const input = responsesInput(req);
      const metadata = Object.assign({}, req.metadata || {});
      if (input.content.length && !metadata.content) metadata.content = input.content;
      return decodeSubmit(Object.assign({}, req, { prompt: input.prompt || req.prompt, metadata: metadata }), model);
    },
    renderEvents: function (ctx, task, previousState) {
      const status = String(task.status || "UNKNOWN").toUpperCase();
      const value = Number(String(task.progress || "").replace("%", ""));
      const progress = Number.isFinite(value) && value >= 0 && value <= 100 ? value : null;
      const state = { status: status, progress: progress };
      if (status === "SUCCESS") {
        const text = responsesVideoText(ctx);
        const events = previousState && previousState.status === status ? [] : [{ type: "output", data: text }];
        return { events: events, state: state, done: true };
      }
      if (status === "FAILURE") {
        return { events: [{ type: "error", code: "task_failed", message: task.fail_reason || "task failed" }], state: state, done: true };
      }
      if (previousState && previousState.status === status && previousState.progress === progress) return { events: [], state: state, done: false };
      const event = { type: "progress", message: status.toLowerCase() };
      if (progress !== null) event.progress = progress;
      return { events: [event], state: state, done: false };
    },
    renderFinal: function (ctx) {
      return {
        output: [
          {
            type: "message",
            status: "completed",
            role: "assistant",
            content: [{ type: "output_text", text: responsesVideoText(ctx), annotations: [], logprobs: [] }],
          },
        ],
        metadata: { vendor: "tencent-vega" },
      };
    },
  },
  openai_video: {
    decodeRequest: function (ctx) {
      if (!ctx.body || (ctx.body.kind !== "json" && ctx.body.kind !== "multipart")) throw new Error("JSON or multipart body required");
      if (ctx.body.kind !== "json") throw new Error("WAND Vega requires image and video references to be URLs inside metadata.content");
      if (!ctx.body.value || Array.isArray(ctx.body.value)) throw new Error("JSON object required");
      const req = Object.assign({}, ctx.body.value);
      const seconds = req.seconds === undefined ? req.duration : req.seconds;
      if (seconds !== undefined && Number(seconds) !== -1 && (!Number.isFinite(Number(seconds)) || Number(seconds) <= 0 || Number(seconds) > 3600)) {
        throw new Error("seconds must be -1 or between 1 and 3600");
      }
      if (seconds !== undefined) req.seconds = seconds;
      return decodeSubmit(req, ctx.model);
    },
    render: function (ctx, task) {
      return legacyRenderers.openai_video(task);
    },
  },
};
