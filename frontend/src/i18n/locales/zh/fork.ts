// fork 自定义 i18n 键（zhiguofan 分支）——由上游模块深合并覆盖。
// 生成来源：合并 0.1.147 时从 fork 单体语言包提取（新增 276 / 覆写 22 键）。
// 后续新增 fork 键直接改本文件；上游同步时本文件不与上游模块冲突。
export default {
  home: {
    heroSubtitle: "多密钥，多分组，精细化管理支付账单",
    tags: {
      subscriptionToApi: "不降智不降速"
    },
    providers: {
      title: "接入主流 AI 模型",
      description: "覆盖国内顶尖语言与多模态大模型，统一 API 一键调用",
      llmLabel: "大语言模型",
      multimodalLabel: "多模态模型",
      deepseek: "DeepSeek V4",
      kimi: "Kimi K2.6",
      qwen: "通义千问",
      doubao: "豆包 2.0",
      hunyuan: "混元 Hy3",
      seedance: "Seedance 2.0",
      kling: "可灵系列",
      hailuo: "海螺系列",
      vidu: "vidu系列",
      paiwo: "拍我系列",
      doubaovideo: "豆包系列"
    }
  },
  setup: {
    title: "TokenPanel 安装向导",
    description: "配置您的 TokenPanel 实例",
    currency: {
      title: "货币设置",
      description: "选择用户界面的货币显示模式",
      usdMode: "美元模式（USD）",
      usdModeDesc: "同时显示美元和人民币价格",
      cnyMode: "人民币模式（CNY）",
      cnyModeDesc: "仅显示人民币价格"
    }
  },
  nav: {
    models: "模型广场",
    playground: "模型体验",
    modelDiscounts: "模型折扣"
  },
  auth: {
    errors: {
      INVITATION_CODE_REQUIRED: "新用户注册需要邀请码，请填写邀请码后重试"
    },
    usernameLabel: "用户名",
    usernamePlaceholder: "请输入用户名",
    usernameRequired: "请输入用户名",
    phoneLoginInvitationHint: "当前已开启强制邀请码，新手机号首次登录即注册，需填写邀请码",
    phoneLabel: "手机号",
    phonePlaceholder: "请输入手机号",
    smsCodeLabel: "短信验证码",
    sendSmsCode: "发送验证码",
    resendSmsCode: "重新发送",
    resendSmsCountdown: "{countdown}秒后可重发",
    phoneRequired: "请输入手机号",
    invalidPhone: "请输入有效的手机号",
    smsCodeRequired: "请输入短信验证码",
    switchToPasswordLogin: "使用账号密码登录",
    switchToPhoneLogin: "使用手机号验证码登录"
  },
  availableChannels: {
    pricing: {
      imageInputPrice: "图片输入"
    }
  },
  profile: {
    authBindings: {
      providers: {
        phone: "手机号"
      }
    }
  },
  admin: {
    currency: {
      setup: {
        title: "请选择货币显示模式",
        description: "此设置影响用户端所有价格和余额的显示方式，设置后可在系统设置中修改。",
        confirm: "确认"
      }
    },
    modelPricings: {
      pricingUnitLabel: "计费单位",
      pricingUnitSecond: "秒",
      pricingUnitImage: "图片（按张）",
      upstreamPricePerSecond: "上游单价（每秒）",
      upstreamPricePerImage: "上游单价（每张）",
      customPricePerSecond: "自定义单价（每秒）",
      secondUnit: "秒"
    },
    dashboard: {
      profit: "利润",
      providerDistribution: "厂商使用分布",
      accountDistribution: "账号使用分布",
      lastNDays: "近 {n} 天",
      total: "合计",
      provider: "厂商",
      providersEmpty: "暂无厂商数据",
      account: "账号",
      coveredGroups: "覆盖分组数",
      coveredGroupsHint: "该账号通过 m2m 关联挂载到 {n} 个分组（跨 group 聚合）",
      accountsEmpty: "暂无账号"
    },
    backup: {
      r2Guide: {
        step1: {
          line2: "点击「创建存储桶」，输入名称（如 tokenpanel-backups），选择区域"
        }
      }
    },
    users: {
      viewStats: "查看统计",
      usageStatistics: "使用统计",
      last30DaysUsage: "近 30 天使用统计（基于实际使用天数）"
    },
    groups: {
      platforms: {
        lingjing: "灵境",
        generic: "通用渠道"
      }
    },
    availableChannels: {
      pricing: {
        imageInputPrice: "图片输入"
      }
    },
    channels: {
      form: {
        imageInputPrice: "图片输入"
      }
    },
    accounts: {
      platforms: {
        lingjing: "灵境",
        generic: "通用渠道"
      },
      generic: {
        title: "通用渠道",
        subtitle: "多 Endpoint 多协议上游渠道",
        endpoints: "Endpoint 列表",
        addEndpoint: "添加 Endpoint",
        removeEndpoint: "删除",
        endpointIndex: "Endpoint {index}",
        stableId: "稳定标识符",
        stableIdPlaceholder: "留空自动生成，如 deepseek-openai_chat",
        stableIdHint: "写入后不可变更，UsageLog 归因依赖此字段",
        outboundProtocol: "出站协议",
        baseUrl: "上游地址",
        baseUrlPlaceholder: "https://api.example.com",
        authHeader: "鉴权头",
        authHeaderPlaceholder: "Authorization",
        authScheme: "鉴权方案",
        authSchemePlaceholder: "Bearer",
        modelsSource: "模型列表来源",
        priority: "调度优先级",
        priorityHint: "数值越小越优先",
        protocols: {
          openai_chat: "OpenAI Chat Completions",
          openai_responses: "OpenAI Responses API",
          anthropic_messages: "Anthropic Messages",
          gemini_v1beta: "Gemini v1beta"
        },
        modelsSources: {
          remote: "远程（调用 /models）",
          manual: "手动填写",
          static_preset: "内置预设"
        },
        noEndpoints: "请至少添加一个 Endpoint",
        apiKeyPlaceholder: "万界方舟等渠道的 API Key，所有 Endpoint 共用",
        apiKeyEditPlaceholder: "留空则不修改（沿用已存密钥）",
        apiKeyHint: "一个账号一个 Key，下面每个 Endpoint 用各自的鉴权头/方案携带此 Key",
        supportedModels: "支持的模型 ID 列表",
        supportedModelsPlaceholder: "qwen3.7-max, glm-4.6, claude-3-5-sonnet（逗号或换行分隔）",
        supportedModelsHint: "该端点实际能转发的模型 ID；留空表示未配置（管理员\"可用模型\"将返回空列表）。",
        fetchModels: "从上游拉取",
        fetchModelsLoading: "拉取中…",
        fetchModelsSuccess: "已拉取 {count} 个模型并合并到列表（同时已去重写入折扣表）",
        fetchModelsFailed: "拉取失败",
        fetchModelsNeedBaseUrl: "请先填写上游地址（Base URL）",
        fetchModelsNeedApiKey: "请先填写账号 API Key",
        fetchModelsSuccessPick: "已拉取 {count} 个模型（并去重写入折扣表），请在下方勾选要暴露的子集",
        modelPickerSearch: "搜索模型…",
        modelPickerSelectAll: "全选",
        modelPickerClear: "清空",
        modelPickerSelected: "已选 {n} / {total}",
        modelPickerNoMatch: "无匹配模型",
        modelPickerEmpty: "点击\"从上游拉取\"获取模型列表，或展开下方手动输入",
        modelPickerManual: "手动输入 / 高级",
        modelMappingHint: "可选。别名 → 上游模型名（请求用别名，转发时改回上游名）。留空则直接透传 supported_models；两者可并存。"
      },
      bulkEdit: {
        mixedPlatformWarning: "所选账号跨越多个平台（{platforms}）。部分平台相关设置（如模型限制）需按平台分别编辑。",
        modelRestrictionDisabledByMixedPlatform: "所选账号跨越多个平台，模型限制需按平台分别设置（混选时已禁用，以免覆盖各平台的模型映射）。"
      },
      oauthSetupToken: "Setup Token",
      providers: {
        label: "渠道（Provider）",
        hint: "选择 OpenAI-compatible 上游渠道；预设会自动填充 base_url，便于命中 provider_pricing 上游成本快照",
        openai: "OpenAI（官方）",
        deepseek: "DeepSeek",
        doubao: "豆包（火山引擎）",
        siliconflow: "硅基流动",
        custom: "自定义（手动填写）"
      },
      anthropic: {
        responseMasking: "响应遮蔽（Kiro 兼容）",
        responseMaskingDesc: "适用于 Kiro 等 Anthropic 兼容上游。开启后，身份/模型/工具类问题将被拦截并固定回答\"Claude Code + 模型名\"；响应中的 Kiro/Kiro CLI 字样也会被替换。默认关闭。",
        bedrockCompat: "Bedrock Converse 兼容",
        bedrockCompatDesc: "把返回给客户端的响应改写为 AWS Bedrock Converse 协议形态（字段重映射；非流式与流式二进制 EventStream 均支持）。用于客户端按 Converse 协议消费的场景，与账号是否为 AWS Bedrock 类型无关。与「响应遮蔽（Kiro 兼容）」互斥。默认关闭。"
      },
      poolModeInfo: "启用后，上游 429/403/401 错误将自动重试而不标记账号限流或错误，适用于上游指向另一个 tokenpanel 实例的场景。"
    },
    ops: {
      errorLog: {
        clientIp: "来源 IP"
      },
      errorDetail: {
        clientIp: "来源 IP",
        apiKey: "API Key"
      },
      requestDetails: {
        table: {
          clientIp: "来源 IP",
          apiKey: "API Key"
        }
      }
    },
    settings: {
      tabs: {
        currency: "货币设置"
      },
      features: {
        showOverseasModels: {
          title: "海外模型显示",
          description: "控制模型广场和折扣管理中是否显示海外 AI 服务商的模型（如 OpenAI、Anthropic、Google Gemini 等）。关闭后仅展示国内服务商模型。",
          enabled: "显示海外模型",
          enabledHint: "关闭后，模型广场和折扣页面将只显示国内服务商的模型（如 DeepSeek、智谱、通义、月之暗面等）。"
        }
      },
      linuxdo: {
        description: "配置 LinuxDo Connect OAuth，用于 TokenPanel 用户登录"
      },
      dingtalk: {
        description: "配置钉钉 OAuth，用于 TokenPanel 用户登录"
      },
      gatewayForwarding: {
        clientDatelineNormalizationHint: "默认开启。将请求体中 \"Today's date is …\" 语句里的撇号与日期分隔符还原为 ASCII 撇号 + 短横线的规范形态，抹除某些客户端在检测到非官方 base URL 时注入的隐写指纹位。",
        openaiAllowClaudeCodeCodexPlugin: "允许在 Claude Code 中使用 Codex 插件",
        openaiAllowClaudeCodeCodexPluginDesc: "全局开关，仅对已开启「仅允许 Codex 官方客户端」的 OpenAI OAuth 账号生效。开启后，所有此类账号都额外放行通过 Claude Code 的 Codex 插件发起的请求（精确匹配 originator=Claude Code），无需逐账号配置；上游请求仍保持透传。"
      },
      site: {
        uiTheme: "界面主题",
        theme_teal: "青绿",
        theme_violet: "蓝紫",
        theme_orange: "橙色",
        siteNamePlaceholder: "TokenPanel",
        siteSubtitlePlaceholder: "管理企业多个大模型资源，精细化管理每个用户和模型账单。",
        apiBaseUrlHint: "用于\"使用密钥\"和\"导入到 CC Switch\"功能，留空则使用当前站点地址"
      },
      sms: {
        title: "短信服务（SMS）",
        description: "配置手机号注册和登录所需的短信服务，支持火山引擎、腾讯云、阿里云",
        phoneRegisterEnabled: "启用手机号注册",
        phoneRegisterEnabledHint: "开启后将关闭邮件验证，用户通过手机号+短信验证码注册和登录",
        passwordLoginEnabled: "允许账号密码登录",
        passwordLoginEnabledHint: "关闭后用户只能通过手机号验证码登录，邮箱密码登录将被禁用",
        providerVolcengine: "火山引擎",
        providerTencent: "腾讯云",
        providerAliyun: "阿里云",
        accessKeyID: "Access Key ID",
        accessKeyIDPlaceholder: "请输入 Access Key ID",
        accessKeySecret: "Access Key Secret",
        secretPlaceholder: "请输入 Secret",
        secretConfiguredPlaceholder: "已配置，留空以保留当前值",
        secretHint: "首次配置请输入 Secret",
        secretConfiguredHint: "Secret 已配置，留空则保留当前值",
        smsAccountID: "短信账户 ID",
        smsAccountIDPlaceholder: "火山引擎短信账户 ID",
        smsSign: "短信签名",
        smsSignPlaceholder: "签名名称",
        smsTemplateID: "验证码模板 ID",
        smsTemplateIDPlaceholder: "模板 ID",
        tencentSecretID: "SecretId",
        tencentSecretIDPlaceholder: "请输入腾讯云 SecretId",
        tencentSecretKey: "SecretKey",
        tencentSdkAppID: "短信应用 ID（SmsSdkAppId）",
        tencentSdkAppIDPlaceholder: "1400xxxxxxx",
        aliyunTemplateCode: "模板 Code",
        aliyunTemplateCodePlaceholder: "SMS_xxxxxxx"
      },
      smtp: {
        fromNamePlaceholder: "TokenPanel"
      },
      currency: {
        title: "货币显示模式",
        description: "控制用户端余额和价格的货币显示方式",
        mode: "货币模式",
        usd: "美元模式（USD）",
        usdDesc: "同时显示美元和人民币价格",
        cny: "人民币模式（CNY）",
        cnyDesc: "仅显示人民币价格",
        cnyRate: "人民币汇率",
        cnyRateHint: "1 USD = ? CNY，用于价格换算",
        save: "保存货币设置",
        saved: "货币设置已保存"
      }
    }
  },
  version: {
    customBuild: "定制版本"
  },
  onboarding: {
    admin: {
      welcome: {
        title: "👋 欢迎使用 TokenPanel",
        description: "<div style=\"line-height: 1.8;\"><p style=\"margin-bottom: 16px;\">TokenPanel 是一个强大的 AI 服务中转平台，让您轻松管理和分发 AI 服务。</p><p style=\"margin-bottom: 12px;\"><b>🎯 核心功能：</b></p><ul style=\"margin-left: 20px; margin-bottom: 16px;\"><li>📦 <b>分组管理</b> - 创建不同的服务套餐（VIP、免费试用等）</li><li>🔗 <b>账号池</b> - 连接多个上游 AI 服务商账号</li><li>🔑 <b>密钥分发</b> - 为用户生成独立的 API Key</li><li>💰 <b>计费管理</b> - 灵活的费率和配额控制</li></ul><p style=\"color: #10b981; font-weight: 600;\">接下来，我们将用 3 分钟带您完成首次配置 →</p></div>"
      },
      groupManage: {
        description: "<div style=\"line-height: 1.7;\"><p style=\"margin-bottom: 12px;\"><b>什么是分组？</b></p><p style=\"margin-bottom: 12px;\">分组是 TokenPanel 的核心概念，它就像一个\"服务套餐\"：</p><ul style=\"margin-left: 20px; margin-bottom: 12px; font-size: 13px;\"><li>🎯 每个分组可以包含多个上游账号</li><li>💰 每个分组有独立的计费倍率</li><li>👥 可以设置为公开或专属分组</li></ul><p style=\"margin-top: 12px; padding: 8px 12px; background: #f0fdf4; border-left: 3px solid #10b981; border-radius: 4px; font-size: 13px;\"><b>💡 示例：</b>您可以创建\"VIP专线\"（高倍率）和\"免费试用\"（低倍率）两个分组</p><p style=\"margin-top: 16px; color: #10b981; font-weight: 600;\">👉 点击左侧的\"分组管理\"开始</p></div>"
      }
    },
    user: {
      welcome: {
        title: "👋 欢迎使用 TokenPanel",
        description: "<div style=\"line-height: 1.8;\"><p style=\"margin-bottom: 16px;\">您好！欢迎来到 TokenPanel AI 服务平台。</p><p style=\"margin-bottom: 12px;\"><b>🎯 快速开始：</b></p><ul style=\"margin-left: 20px; margin-bottom: 16px;\"><li>🔑 创建 API 密钥</li><li>📋 复制密钥到您的应用</li><li>🚀 开始使用 AI 服务</li></ul><p style=\"color: #10b981; font-weight: 600;\">只需 1 分钟，让我们开始吧 →</p></div>"
      }
    }
  },
  playground: {
    title: "模型体验",
    description: "使用你自己的 API Key 进行对话和生图",
    greeting: "今天有什么计划?",
    inputPlaceholder: "有问题，尽管问",
    send: "发送",
    stop: "停止",
    newChat: "新对话",
    copy: "复制",
    copied: "已复制",
    maxImages: "最多上传 {n} 张图片",
    selectKey: "选择 Key",
    keyGroup: "分组",
    model: "模型",
    noKey: "暂无可用的 API Key",
    noKeyHint: "请先创建一个 API Key 再开始体验。",
    goCreateKey: "创建 API Key",
    noModel: "该 Key 无可用模型",
    imageOnlyKey: "该 Key 仅支持生图，请点下方「生成图片」。",
    imageModelPlaceholder: "生图模型",
    chips: {
      generateImage: "生成图片",
      editImage: "图生图"
    },
    intent: {
      chat: "对话",
      image: "生成图片",
      edit: "图生图"
    },
    imageNotAvailable: "当前 Key 所属分组未开启生图，请切换到 OpenAI 类且已开启生图的分组。",
    upload: "上传图片",
    uploadHint: "最多 {max} 张，每张 ≤ {size}MB",
    imageTooLarge: "图片 {name} 超过 {size}MB 限制",
    imagePrompt: "描述你想要的图片",
    editPrompt: "描述如何编辑上传的图片",
    size: "尺寸",
    count: "数量",
    download: "下载",
    useAsEditInput: "作为图生图输入",
    generatedImages: "本次生成 {n} 张图，已计费",
    reasoning: "推理过程",
    showReasoning: "展开推理",
    hideReasoning: "收起推理",
    thinking: "正在思考…",
    generatingImage: "正在生成图片，可能需要一会儿…",
    params: {
      title: "参数",
      temperature: "温度",
      maxTokens: "最大 Token",
      topP: "Top P",
      systemPrompt: "系统提示词",
      enable: "启用"
    },
    usage: {
      tokens: "输入 {input} / 输出 {output} tokens",
      images: "{n} 张图",
      cost: "预估成本 ${cost}"
    },
    risk: {
      bannerTitle: "模型体验会消耗真实额度",
      bannerBody: "体验将使用你选定的 API Key 发起真实请求，会消耗真实额度（余额 / 配额 / 限速窗口），与正式调用一致，不可退款。",
      ack: "我已知晓，继续",
      inlineHint: "消耗真实额度",
      sendTooltip: "本次将真实计费"
    },
    errors: {
      insufficient: "余额或订阅额度不足，请充值或更换 Key。",
      rateLimited: "触发限速（5h/1d/7d 窗口）或配额已用尽，请稍后再试。",
      imageNotAllowed: "当前 Key 分组未开启生图。",
      generic: "请求失败"
    }
  },
  models: {
    title: "模型广场",
    description: "查看当前账号可用的 AI 模型、价格和能力",
    refresh: "刷新",
    modelsAvailable: "个模型可用",
    searchPlaceholder: "搜索模型名称...",
    allProviders: "全部提供商",
    allModes: "全部模式",
    chatMode: "聊天",
    imageMode: "图片",
    noResults: "没有匹配的模型",
    showing: "正在显示 {count} / {total} 个模型",
    modelName: "模型名称",
    provider: "提供商",
    availability: "可用性",
    available: "可用",
    unavailable: "不可用",
    copyModelName: "复制模型名称",
    copied: "已复制",
    testPassed: "测试通过",
    testFailed: "测试失败",
    inputPrice: "输入价格（/1K）",
    outputPrice: "输出价格（/1K）",
    pricing: {
      secondPrice: "单价",
      unitPerSecond: "/秒",
      imagePrice: "单张价",
      videoPrice: "单价",
      unitPerImage: "/张",
      unitPerToken: "/token"
    },
    contextWindow: "上下文窗口",
    features: "能力",
    promptCaching: "缓存"
  }
}
