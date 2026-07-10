// fork 自定义 i18n 键（zhiguofan 分支）——由上游模块深合并覆盖。
// 生成来源：合并 0.1.147 时从 fork 单体语言包提取（新增 278 / 覆写 23 键）。
// 后续新增 fork 键直接改本文件；上游同步时本文件不与上游模块冲突。
export default {
  home: {
    heroSubtitle: "Multi-Key, Multi-Group, Fine-Grained Billing Control",
    tags: {
      subscriptionToApi: "No Throttling, No Downgrade"
    },
    providers: {
      title: "Access Leading AI Models",
      description: "Unified API for top language & multimodal models from China",
      soon: "Coming Soon",
      llmLabel: "Language Models",
      multimodalLabel: "Multimodal",
      deepseek: "DeepSeek V4",
      kimi: "Kimi K2.6",
      qwen: "Qwen",
      doubao: "Doubao 2.0",
      hunyuan: "Hunyuan Hy3",
      seedance: "Seedance 2.0",
      kling: "Kling",
      hailuo: "Hailuo",
      vidu: "Vidu",
      paiwo: "PaiWo",
      doubaovideo: "Doubao Video"
    }
  },
  setup: {
    title: "TokenPanel Setup",
    description: "Configure your TokenPanel instance",
    currency: {
      title: "Currency Settings",
      description: "Choose the currency display mode for the user interface",
      usdMode: "USD Mode",
      usdModeDesc: "Show both USD and CNY prices",
      cnyMode: "CNY Mode",
      cnyModeDesc: "Show CNY prices only"
    }
  },
  nav: {
    sora: "Sora Studio",
    models: "Model Plaza",
    playground: "Playground",
    modelDiscounts: "Model Discounts"
  },
  auth: {
    errors: {
      INVITATION_CODE_REQUIRED: "New accounts require an invitation code. Please enter one and try again."
    },
    usernameLabel: "Username",
    usernamePlaceholder: "Enter your username",
    usernameRequired: "Username is required",
    phoneLoginInvitationHint: "Invitation code is enforced. A new phone number registers on first login, so an invitation code is required.",
    phoneLabel: "Phone Number",
    phonePlaceholder: "Enter phone number",
    smsCodeLabel: "SMS Code",
    sendSmsCode: "Send Code",
    resendSmsCode: "Resend",
    resendSmsCountdown: "Resend in {countdown}s",
    phoneRequired: "Phone number is required",
    invalidPhone: "Please enter a valid phone number",
    smsCodeRequired: "SMS code is required",
    switchToPasswordLogin: "Sign in with email and password",
    switchToPhoneLogin: "Sign in with phone verification code"
  },
  availableChannels: {
    pricing: {
      imageInputPrice: "Image Input"
    }
  },
  profile: {
    authBindings: {
      providers: {
        phone: "Phone"
      }
    }
  },
  admin: {
    currency: {
      setup: {
        title: "Select Currency Display Mode",
        description: "This setting affects how all prices and balances are displayed to users. You can change it later in System Settings.",
        confirm: "Confirm"
      }
    },
    modelPricings: {
      pricingUnitLabel: "Billing Unit",
      pricingUnitSecond: "Second",
      pricingUnitImage: "Image (per image)",
      upstreamPricePerSecond: "Upstream Price (per second)",
      upstreamPricePerImage: "Upstream Price (per image)",
      customPricePerSecond: "Custom Price (per second)",
      secondUnit: "sec"
    },
    dashboard: {
      providerDistribution: "Provider Usage Distribution",
      accountDistribution: "Account Usage Distribution",
      lastNDays: "Last {n} days",
      total: "Total",
      provider: "Provider",
      providersEmpty: "No provider data",
      account: "Account",
      coveredGroups: "Covered groups",
      coveredGroupsHint: "Account is m2m-attached to {n} groups (aggregated across groups)",
      accountsEmpty: "No accounts",
      profit: "Profit"
    },
    backup: {
      r2Guide: {
        step1: {
          line2: "Click \"Create bucket\", enter a name (e.g. tokenpanel-backups), choose a region"
        }
      }
    },
    users: {
      viewStats: "View Stats",
      usageStatistics: "Usage Statistics",
      last30DaysUsage: "Last 30 days usage statistics (based on actual usage days)"
    },
    groups: {
      platforms: {
        lingjing: "Lingjing",
        generic: "Generic Channel"
      }
    },
    availableChannels: {
      pricing: {
        imageInputPrice: "Image Input"
      }
    },
    channels: {
      form: {
        imageInputPrice: "Image Input"
      }
    },
    accounts: {
      platforms: {
        lingjing: "Lingjing",
        generic: "Generic Channel"
      },
      generic: {
        title: "Generic Channel",
        subtitle: "Multi-endpoint multi-protocol upstream",
        endpoints: "Endpoints",
        addEndpoint: "Add Endpoint",
        removeEndpoint: "Remove",
        endpointIndex: "Endpoint {index}",
        stableId: "Stable ID",
        stableIdPlaceholder: "Auto-generated if empty, e.g. deepseek-openai_chat",
        stableIdHint: "Cannot be changed after creation — UsageLog snapshots depend on this field",
        outboundProtocol: "Outbound Protocol",
        baseUrl: "Base URL",
        baseUrlPlaceholder: "https://api.example.com",
        authHeader: "Auth Header",
        authHeaderPlaceholder: "Authorization",
        authScheme: "Auth Scheme",
        authSchemePlaceholder: "Bearer",
        modelsSource: "Models Source",
        priority: "Scheduling Priority",
        priorityHint: "Lower number = higher priority",
        protocols: {
          openai_chat: "OpenAI Chat Completions",
          openai_responses: "OpenAI Responses API",
          anthropic_messages: "Anthropic Messages",
          gemini_v1beta: "Gemini v1beta"
        },
        modelsSources: {
          remote: "Remote (call /models)",
          manual: "Manual entry",
          static_preset: "Static preset"
        },
        noEndpoints: "Please add at least one endpoint",
        apiKeyPlaceholder: "API key (e.g. Wanjie), shared by all endpoints",
        apiKeyEditPlaceholder: "Leave blank to keep the existing key",
        apiKeyHint: "One key per account; each endpoint below carries it via its own auth header/scheme",
        supportedModels: "Supported model IDs",
        supportedModelsPlaceholder: "qwen3.7-max, glm-4.6, claude-3-5-sonnet (comma or newline separated)",
        supportedModelsHint: "Model IDs this endpoint can forward. Leave blank = not configured (admin \"Available models\" returns empty).",
        fetchModels: "Fetch from upstream",
        fetchModelsLoading: "Fetching…",
        fetchModelsSuccess: "Fetched {count} models, merged into the list and seeded into the pricing table",
        fetchModelsFailed: "Failed to fetch models",
        fetchModelsNeedBaseUrl: "Please fill in the Base URL first",
        fetchModelsNeedApiKey: "Please fill in the account API Key first",
        fetchModelsSuccessPick: "Fetched {count} models (deduped into the pricing table). Check the subset to expose below",
        modelPickerSearch: "Search models…",
        modelPickerSelectAll: "Select all",
        modelPickerClear: "Clear",
        modelPickerSelected: "Selected {n} / {total}",
        modelPickerNoMatch: "No matching models",
        modelPickerEmpty: "Click \"Fetch from upstream\" to load models, or expand manual entry below",
        modelPickerManual: "Manual entry / advanced",
        modelMappingHint: "Optional. Alias → upstream model name (request the alias; forwarded as the upstream name). Leave empty to pass supported_models through as-is; both can coexist."
      },
      bulkEdit: {
        mixedPlatformWarning: "Selected accounts span multiple platforms ({platforms}). Some platform-specific settings (such as model restriction) must be edited per platform.",
        modelRestrictionDisabledByMixedPlatform: "Selected accounts span multiple platforms. Model restriction must be set per platform — it is disabled here to avoid overwriting each platform's model mapping."
      },
      oauthSetupToken: "Setup Token",
      providers: {
        label: "Provider",
        hint: "Choose an OpenAI-compatible upstream channel; the preset auto-fills the base URL so usage logs can hit provider_pricing for accurate cost snapshots.",
        openai: "OpenAI (Official)",
        deepseek: "DeepSeek",
        doubao: "Doubao (Volcengine)",
        siliconflow: "SiliconFlow",
        custom: "Custom (manual entry)"
      },
      anthropic: {
        responseMasking: "Response Masking (Kiro Compat)",
        responseMaskingDesc: "For Anthropic-compatible upstreams (e.g. Kiro). When enabled, identity/model/tool questions are intercepted and answered with \"Claude Code + model ID\". Kiro/Kiro CLI strings in responses are also replaced. Disabled by default.",
        bedrockCompat: "Bedrock Converse Compatibility",
        bedrockCompatDesc: "Rewrites the response returned to the client into AWS Bedrock Converse shape (field remapping; both non-streaming and streaming binary EventStream are supported). For clients that consume the Converse protocol; independent of whether the account is an AWS Bedrock account. Mutually exclusive with \"Response Masking (Kiro Compat)\". Disabled by default."
      },
      poolModeInfo: "When enabled, upstream 429/403/401 errors will auto-retry without marking the account as rate-limited or errored. Suitable for upstream pointing to another tokenpanel instance."
    },
    ops: {
      errorLog: {
        clientIp: "Client IP"
      },
      errorDetail: {
        clientIp: "Client IP",
        apiKey: "API Key"
      },
      requestDetails: {
        table: {
          clientIp: "Client IP",
          apiKey: "API Key"
        }
      }
    },
    settings: {
      tabs: {
        currency: "Currency"
      },
      features: {
        showOverseasModels: {
          title: "Overseas Models Visibility",
          description: "Controls whether models from overseas AI providers (e.g. OpenAI, Anthropic, Google Gemini) are shown in the model marketplace and discount management. When disabled, only domestic provider models are displayed.",
          enabled: "Show Overseas Models",
          enabledHint: "When disabled, the model marketplace and discount page will only show models from domestic providers (e.g. DeepSeek, Zhipu, Qwen, Moonshot)."
        }
      },
      linuxdo: {
        description: "Configure LinuxDo Connect OAuth for TokenPanel end-user login"
      },
      dingtalk: {
        description: "Configure DingTalk OAuth for TokenPanel end-user login"
      },
      gatewayForwarding: {
        clientDatelineNormalizationHint: "Default on. Rewrites the \"Today's date is …\" sentence in request bodies back to a canonical ASCII apostrophe and hyphen date format, erasing steganographic fingerprint bits some clients inject when they detect a non-official base URL.",
        openaiAllowClaudeCodeCodexPlugin: "Allow using the Codex plugin in Claude Code",
        openaiAllowClaudeCodeCodexPluginDesc: "Global switch; only affects OpenAI OAuth accounts that have 'Codex official clients only' enabled. When on, all such accounts additionally allow requests from the Claude Code Codex plugin (exact match on originator=Claude Code) without per-account config; upstream requests remain pass-through."
      },
      site: {
        uiTheme: "UI Theme",
        theme_teal: "Teal",
        theme_violet: "Violet",
        theme_orange: "Orange",
        siteNamePlaceholder: "TokenPanel",
        siteSubtitlePlaceholder: "管理企业多个大模型资源，精细化管理每个用户和模型账单。",
        apiBaseUrlHint: "Used for \"Use Key\" and \"Import to CC Switch\" features. Leave empty to use current site URL."
      },
      sms: {
        title: "SMS Service",
        description: "Configure SMS for phone registration and login. Supports Volcengine, Tencent Cloud, and Alibaba Cloud.",
        phoneRegisterEnabled: "Enable Phone Registration",
        phoneRegisterEnabledHint: "When enabled, email verification will be disabled. Users register and log in with phone + SMS code",
        passwordLoginEnabled: "Allow Password Login",
        passwordLoginEnabledHint: "When disabled, users can only log in via phone verification code. Email/password login will be blocked",
        providerVolcengine: "Volcengine",
        providerTencent: "Tencent Cloud",
        providerAliyun: "Alibaba Cloud",
        accessKeyID: "Access Key ID",
        accessKeyIDPlaceholder: "Enter Access Key ID",
        accessKeySecret: "Access Key Secret",
        secretPlaceholder: "Enter Secret",
        secretConfiguredPlaceholder: "Configured. Leave blank to keep current value",
        secretHint: "Enter Secret to configure",
        secretConfiguredHint: "Secret is configured. Leave blank to keep the current value",
        smsAccountID: "SMS Account ID",
        smsAccountIDPlaceholder: "Volcengine SMS account ID",
        smsSign: "SMS Signature",
        smsSignPlaceholder: "Signature name",
        smsTemplateID: "Template ID",
        smsTemplateIDPlaceholder: "Template ID",
        tencentSecretID: "SecretId",
        tencentSecretIDPlaceholder: "Enter Tencent Cloud SecretId",
        tencentSecretKey: "SecretKey",
        tencentSdkAppID: "SMS SDK App ID (SmsSdkAppId)",
        tencentSdkAppIDPlaceholder: "1400xxxxxxx",
        aliyunTemplateCode: "Template Code",
        aliyunTemplateCodePlaceholder: "SMS_xxxxxxx"
      },
      smtp: {
        fromNamePlaceholder: "TokenPanel"
      },
      currency: {
        title: "Currency Display Mode",
        description: "Control how balances and prices are displayed to users",
        mode: "Currency Mode",
        usd: "USD Mode",
        usdDesc: "Show both USD and CNY prices",
        cny: "CNY Mode",
        cnyDesc: "Show CNY prices only",
        cnyRate: "CNY Exchange Rate",
        cnyRateHint: "1 USD = ? CNY, used for price conversion",
        save: "Save Currency Settings",
        saved: "Currency settings saved"
      }
    }
  },
  version: {
    customBuild: "Custom Build"
  },
  onboarding: {
    admin: {
      welcome: {
        title: "👋 Welcome to TokenPanel",
        description: "<div style=\"line-height: 1.8;\"><p style=\"margin-bottom: 16px;\">TokenPanel is a powerful AI service gateway platform that helps you easily manage and distribute AI services.</p><p style=\"margin-bottom: 12px;\"><b>🎯 Core Features:</b></p><ul style=\"margin-left: 20px; margin-bottom: 16px;\"><li>📦 <b>Group Management</b> - Create service tiers (VIP, Free Trial, etc.)</li><li>🔗 <b>Account Pool</b> - Connect multiple upstream AI service accounts</li><li>🔑 <b>Key Distribution</b> - Generate independent API Keys for users</li><li>💰 <b>Billing Control</b> - Flexible rate and quota management</li></ul><p style=\"color: #10b981; font-weight: 600;\">Let's complete the initial setup in 3 minutes →</p></div>"
      },
      groupManage: {
        description: "<div style=\"line-height: 1.7;\"><p style=\"margin-bottom: 12px;\"><b>What is a Group?</b></p><p style=\"margin-bottom: 12px;\">Groups are the core concept of TokenPanel, like a \"service package\":</p><ul style=\"margin-left: 20px; margin-bottom: 12px; font-size: 13px;\"><li>🎯 Each group can contain multiple upstream accounts</li><li>💰 Each group has independent billing multiplier</li><li>👥 Can be set as public or exclusive</li></ul><p style=\"margin-top: 12px; padding: 8px 12px; background: #f0fdf4; border-left: 3px solid #10b981; border-radius: 4px; font-size: 13px;\"><b>💡 Example:</b> You can create \"VIP Premium\" (high rate) and \"Free Trial\" (low rate) groups</p><p style=\"margin-top: 16px; color: #10b981; font-weight: 600;\">👉 Click \"Group Management\" on the left sidebar</p></div>"
      }
    },
    user: {
      welcome: {
        title: "👋 Welcome to TokenPanel",
        description: "<div style=\"line-height: 1.8;\"><p style=\"margin-bottom: 16px;\">Hello! Welcome to the TokenPanel AI service platform.</p><p style=\"margin-bottom: 12px;\"><b>🎯 Quick Start:</b></p><ul style=\"margin-left: 20px; margin-bottom: 16px;\"><li>🔑 Create API Key</li><li>📋 Copy key to your application</li><li>🚀 Start using AI services</li></ul><p style=\"color: #10b981; font-weight: 600;\">Just 1 minute, let's get started →</p></div>"
      }
    }
  },
  playground: {
    title: "Playground",
    description: "Chat and generate images using your own API Key",
    greeting: "What are you working on today?",
    inputPlaceholder: "Ask anything",
    send: "Send",
    stop: "Stop",
    newChat: "New chat",
    copy: "Copy",
    copied: "Copied",
    maxImages: "Up to {n} images",
    selectKey: "Select Key",
    keyGroup: "Group",
    model: "Model",
    noKey: "No available API Key yet",
    noKeyHint: "Create an API Key first to start experiencing.",
    goCreateKey: "Create API Key",
    noModel: "This Key has no available models",
    imageOnlyKey: "This Key is for image generation only — use \"Generate Image\" below.",
    imageModelPlaceholder: "Image model",
    chips: {
      generateImage: "Generate Image",
      editImage: "Image to Image"
    },
    intent: {
      chat: "Chat",
      image: "Generate Image",
      edit: "Image to Image"
    },
    imageNotAvailable: "The group of the current Key does not allow image generation. Switch to an OpenAI-type group with image generation enabled.",
    upload: "Upload image",
    uploadHint: "Up to {max} images, each ≤ {size}MB",
    imageTooLarge: "Image {name} exceeds {size}MB limit",
    imagePrompt: "Describe the image you want",
    editPrompt: "Describe how to edit the uploaded image",
    size: "Size",
    count: "Count",
    download: "Download",
    useAsEditInput: "Use as image-to-image input",
    generatedImages: "Generated {n} image(s), billed",
    reasoning: "Reasoning",
    showReasoning: "Show reasoning",
    hideReasoning: "Hide reasoning",
    thinking: "Thinking…",
    generatingImage: "Generating image, this may take a moment…",
    params: {
      title: "Parameters",
      temperature: "Temperature",
      maxTokens: "Max Tokens",
      topP: "Top P",
      systemPrompt: "System Prompt",
      enable: "Enable"
    },
    usage: {
      tokens: "{input} in / {output} out tokens",
      images: "{n} image(s)",
      cost: "Est. cost ${cost}"
    },
    risk: {
      bannerTitle: "Model experience uses real quota",
      bannerBody: "Requests use your selected API Key and consume real quota (balance / quota / rate-limit windows), same as production calls, and are non-refundable.",
      ack: "I understand, continue",
      inlineHint: "Consumes real quota",
      sendTooltip: "This request will be billed for real"
    },
    errors: {
      insufficient: "Insufficient balance or subscription quota. Please recharge or switch Key.",
      rateLimited: "Rate limit (5h/1d/7d window) or quota exhausted. Please try again later.",
      imageNotAllowed: "The current Key group does not allow image generation.",
      generic: "Request failed"
    }
  },
  models: {
    title: "Model Plaza",
    description: "View all available AI models, pricing, and features",
    refresh: "Refresh",
    modelsAvailable: "models available",
    searchPlaceholder: "Search model name...",
    allProviders: "All Providers",
    allModes: "All Modes",
    chatMode: "Chat",
    imageMode: "Image",
    noResults: "No matching models found",
    showing: "Showing {count} / {total} models",
    modelName: "Model Name",
    provider: "Provider",
    availability: "Availability",
    available: "Available",
    unavailable: "Unavailable",
    copyModelName: "Copy model name",
    copied: "Copied",
    testPassed: "Test Passed",
    testFailed: "Test Failed",
    inputPrice: "Input Price (/1K)",
    outputPrice: "Output Price (/1K)",
    pricing: {
      secondPrice: "Unit Price",
      unitPerSecond: "/sec",
      imagePrice: "Per Image",
      videoPrice: "Unit Price",
      unitPerImage: "/image",
      unitPerToken: "/token"
    },
    contextWindow: "Context Window",
    features: "Features",
    promptCaching: "Cache"
  },
  payment: {
    orders: {
      orderIdLabel: "Order ID"
    }
  }
}
