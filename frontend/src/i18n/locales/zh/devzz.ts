// Generated from the dev-zz locale delta at merge time.
// Keep branch-specific copy here so upstream locale modules can evolve independently.
export default {
    // Home Page
    home: {
        nav: {
            home: '首页',
            model: '模型',
            document: '文档',
            register: '注册'
        },
        hero: {
            badge: '统一 AI 模型网关',
            titleLead: '一个接口，',
            titleHighlight: '调用全球所有 AI 模型',
            description: '统一密钥、统一格式、统一账单，一站式接入 OpenAI、Claude、Gemini、DeepSeek 等数十种模型',
            primaryCta: '立即开始',
            secondaryCta: '查看文档'
        },
        quickAccess: {
            title: '3 分钟快速接入',
            guide: '快速接入指南',
            guideDesc: '只需三步，即可接入全球所有 AI 模型，无需适配多家平台，统一格式、一键调用。',
            step1: '注册获取 API Key',
            step2: '替换请求地址为网关地址',
            step3: '直接调用，支持所有模型',
            stepDesc1: '在控制台创建或复制 API Key，按用户和分组控制可用范围。',
            stepDesc2: '把客户端 Base URL 指向网关，保留熟悉的 OpenAI 兼容请求格式。',
            stepDesc3: '选择模型名称即可调用，后续请求记录、计费和日志由平台统一处理。',
            samplePrompt: 'hi',
            fullDoc: '查看完整文档',
            compatibility: '兼容 OpenAI 格式，无需修改业务代码'
        },
        advantages: {
            title: '核心优势',
            description: '统一接入、多模型选择、用量追踪和权限控制放在同一个工作台里，不再把关键配置散落在多个平台。',
            fast: '高速转发',
            fastDesc: '全球节点，低延迟，高并发，稳定可用',
            unified: '统一接口',
            unifiedDesc: '一次接入，全模型通用，无需重复适配',
            safe: '安全可靠',
            safeDesc: '数据加密，权限可控，日志可追溯',
            format: '统一请求格式',
            stable: '稳定接入体验',
            usage: '用量和账单可追踪'
        },
        models: {
            title: '主流模型',
            description: '常用模型以统一目录呈现，业务只需要选择模型名称，兼容格式、可用范围和用量记录由平台集中呈现。',
            more: '浏览更多模型',
            hot: '热门',
            input: '输入',
            output: '输出',
            pricingNote: '实际以优惠后分组价格为准',
            gptDesc: '超强推理、代码、多模态能力',
            claudeDesc: '超长上下文，文档理解强',
            geminiDesc: '多模态、实时联网、视频理解',
            compatibilityBullet: '兼容 OpenAI、Gemini、Claude 调用',
            usageBullet: '按业务场景选择模型，统一查看请求记录和用量消耗',
            metricAccess: '统一入口接入',
            metricUsage: '消耗记录可查',
            metricModels: '按需选择模型'
        },
        testimonials: {
            title: '用户怎么说',
            description: '这些反馈来自不同类型的团队，统一做匿名化展示，只保留使用场景和角色身份。',
            first: {
                initial: '企',
                quote: '接入后，我们不再需要维护多个 AI 模型的适配代码，开发效率提升了 80%，成本降低了 30%。',
                name: '企业客户',
                role: 'CEO'
            },
            second: {
                initial: '技',
                quote: '统一的接口和账单管理让财务和技术团队都轻松了很多，而且模型响应速度比直连还快。',
                name: '技术团队',
                role: '技术架构师'
            },
            third: {
                initial: '产',
                quote: '原本担心数据安全问题，试用后发现加密和私有化部署方案完全满足我们的合规要求。',
                name: '产品团队',
                role: '产品负责人'
            },
            fourth: {
                initial: 'AI',
                quote: '把 Codex、Claude 和 Gemini 都接到同一个网关后，团队不用再到处分发不同密钥，权限和用量也终于能统一看了。',
                name: 'AI 团队',
                role: 'AI 应用团队负责人'
            },
            fifth: {
                initial: '后',
                quote: '之前每个项目都要单独适配模型接口，现在只维护一套 OpenAI 兼容调用，灰度切换上游也很轻松。',
                name: '后端团队',
                role: '后端架构师'
            },
            sixth: {
                initial: '运',
                quote: '我们最需要的是稳定转发和清晰账单，接入后能按用户、Key 和模型拆分成本，月底对账省了很多时间。',
                name: '运营团队',
                role: 'SaaS 运营负责人'
            },
            seventh: {
                initial: '客',
                quote: '统一入口比人工维护多个平台省心很多，业务侧不用关心不同供应商的接入差异，客服压力明显下降。',
                name: '客户成功团队',
                role: '客户成功经理'
            },
            eighth: {
                initial: '平',
                quote: '对于内部工具来说，统一入口非常关键。现在新人只需要拿一个 Key，就能在测试环境里试所有可用模型。',
                name: '平台团队',
                role: '平台工程师'
            },
            ninth: {
                initial: 'IT',
                quote: '我们把不同部门拆成独立分组，额度、倍率和可用模型都能分开配置，比之前用表格登记清楚很多。',
                name: '企业 IT',
                role: '企业 IT 管理员'
            },
            tenth: {
                initial: 'PM',
                quote: '模型切换速度很快，业务只改一个 model 名称就能验证效果，做 A/B 测试比直连多家平台简单很多。',
                name: '产品团队',
                role: '产品经理'
            }
        },
        faq: {
            title: '常见问题',
            description: '接入前最常见的问题集中在兼容性、成本和安全策略。这里保留核心答案，详细配置可继续查看文档。',
            q1: '平台兼容哪些接口格式？',
            a1: '兼容 OpenAI 接口格式，通常只需要替换请求地址即可；同时可按平台能力适配 Claude、Gemini 等模型。',
            q2: '使用平台会增加额外成本吗？',
            a2: '平台按实际用量和配置计费，适合统一管理多模型调用、预算和团队额度，整体成本更容易控制。',
            q3: '数据安全有保障吗？',
            a3: '请求通过网关统一转发，可结合权限、日志、额度和部署策略做安全控制，具体能力取决于你的站点配置。'
        },
        // CTA 区块
        cta: {
            title: '准备好接入全球所有 AI 模型了吗？',
            description: '注册即可体验统一接口、统一账单和多模型调用能力。',
            button: '立即注册'
        },
        footer: {
            tagline: '一个接口，调用全球所有 AI 模型。统一格式、统一密钥、统一账单。',
            navTitle: '主导航',
            supportTitle: '服务支持',
            businessTitle: '商务合作',
            quickAccess: '快速接入',
            faq: '常见问题',
            apiDocs: 'API 文档',
            contact: '联系我们',
            enterprise: '企业方案',
            partner: '渠道合作'
        }
    },
    // Navigation
    nav: {
        channelStatus: '模型状态'
    },
    // Auth
    auth: {
        copyright: '© {year} {siteName}。保留所有权利。'
    },
    // API Keys
    keys: {
        tagFilterPlaceholder: '添加标签筛选...',
        tags: '标签',
        tagsLabel: '标签',
        tagsPlaceholder: '输入标签，例如 项目一',
        tagsAddPlaceholder: '继续添加标签',
        tagsHint: '输入后点击添加或按 Enter；也支持粘贴逗号/换行分隔的标签。最多 20 个，每个不超过 40 个字符。',
        noTags: '无标签',
        noTagOptions: '暂无可选标签',
        tagsTooMany: '标签最多 {count} 个',
        tagTooLong: '单个标签不能超过 {count} 个字符',
        batchCreate: {
            title: '批量创建',
            templateMode: '名称模板',
            namesMode: '名称列表',
            nameTemplate: '名称模板',
            nameTemplatePlaceholder: '成员-{seq}',
            nameTemplateHint: '必须包含 {seq}，系统会从 001 开始自动补齐编号。',
            count: '数量',
            names: '名称列表',
            namesPlaceholder: '张三\n李四\n王五',
            namesHint: '每行一个名称，当前 {count} 个。',
            quotaHint: '本次创建的每个密钥都会使用相同额度。0 或留空 = 无限制。',
            expirationPlaceholder: '留空或 0 表示永久有效',
            expirationHint: '按创建时间向后计算天数。',
            submit: '创建密钥',
            creating: '创建中...',
            resultTitle: '批量创建结果',
            resultHint: '完整密钥只在本次结果中展示，请在关闭前完成复制或导出。',
            replayWarning: '这是幂等重放结果，完整密钥不会再次展示。',
            copyAll: '复制全部',
            exportCsv: '导出 CSV',
            copied: '批量密钥已复制',
            success: '已创建 {count} 个 API 密钥',
            failed: '批量创建失败',
            templateRequired: '名称模板必须包含 {seq}',
            countRequired: '请输入大于 0 的创建数量',
            namesRequired: '请输入至少一个密钥名称',
            namesDuplicate: '名称列表中存在重复名称',
            nameTooLong: '密钥名称不能超过 100 个字符',
            tagsHint: '本次创建的每个密钥都会写入相同标签。',
            unlimited: '无限制'
        },
        batchActions: {
            selected: '已选择 {count} 个 API 密钥',
            filtered: '当前筛选结果 {count} 个 API 密钥',
            selectPage: '选择当前页 API 密钥',
            selectOne: '选择 API 密钥 {name}',
            selectRequired: '请先选择 API 密钥',
            filtersRequired: '请先设置至少一个筛选条件',
            emptyFilteredResult: '当前筛选结果为空',
            filterLimitExceeded: '筛选结果超过 {max} 个，请缩小范围',
            clear: '清空选择',
            update: '批量修改',
            delete: '批量删除',
            updateTitle: '批量修改 API 密钥',
            updateHint: '将同一组配置同步应用到已选择的 {count} 个密钥。只有勾选的字段会被修改，未勾选字段保持不变。',
            updateFilteredHint: '将同一组配置同步应用到当前筛选条件匹配的 {count} 个密钥。只有勾选的字段会被修改，未勾选字段保持不变。',
            updateSubmit: '应用修改',
            updateSuccess: '已更新 {count} 个 API 密钥',
            updateFailed: '批量修改失败',
            deleteTitle: '批量删除 API 密钥',
            deleteConfirm: '确定要删除已选择的 {count} 个 API 密钥吗？删除后这些 Key 将无法继续使用。',
            deleteFilteredConfirm: '确定要删除当前筛选条件匹配的 {count} 个 API 密钥吗？删除后这些 Key 将无法继续使用。',
            deleteSuccess: '已删除 {count} 个 API 密钥',
            deleteFailed: '批量删除失败',
            noFields: '请至少选择一个要修改的字段',
            addQuotaRequired: '追加额度必须大于 0',
            expirationRequired: '请选择有效的过期时间',
            tagsRequired: '请输入至少一个标签',
            clearGroup: '清空分组',
            resetRateUsage: '同时清零当前限速窗口用量',
            tagsHint: '用逗号或换行分隔标签。',
            clearTagsHint: '会清空已选择密钥上的全部标签。',
            fields: {
                group: '分组',
                status: '状态',
                tags: '标签',
                quota: '额度',
                expiration: '过期时间',
                rateLimit: '限速',
                ipAccess: 'IP 访问控制'
            },
            quotaModes: {
                set: '设置为固定额度',
                add: '在现有额度上追加',
                unlimited: '设为无限制'
            },
            expirationModes: {
                clear: '永久有效',
                set: '设置过期时间'
            },
            tagModes: {
                add: '追加标签',
                set: '覆盖为这些标签',
                remove: '移除标签',
                clear: '清空全部标签'
            }
        },
        systemStatus: {
            quota_exhausted: {
                title: '额度耗尽',
                description: '提高额度或重置已用额度后，这把密钥会恢复可用。'
            },
            expired: {
                title: '已过期',
                description: '延长或清除过期时间后，这把密钥会恢复可用。'
            },
            rateChangedTitle: '分组倍率已调整',
            rateChangedDescription: '该 API Key 因分组倍率调整被系统停用。确认新倍率后，可将状态改为启用。',
            rateChangedBadge: '倍率变更停用',
            rateChangedEnableAction: '确认启用',
            manualDisableLabel: '手动禁用这把密钥',
            manualDisableHint: '即使额度或过期时间变更本可恢复，也保存为已禁用。'
        },
        usageDetails: {
            open: '查看用量详情',
            trendTab: '用量趋势',
            modelsTab: '模型分布',
            logsTab: '请求记录',
            startDate: '开始日期',
            endDate: '结束日期',
            timezone: '时区',
            bucket: '时间桶',
            trendChart: '趋势图表',
            metricCost: '消耗',
            metricTokens: 'Token',
            metricRequests: '请求',
            modelCount: '模型数',
            requestId: '请求 ID',
            recordsTotal: '共 {total} 条记录',
            loadFailed: '加载用量趋势失败',
            modelsLoadFailed: '加载模型分布失败',
            granularity: {
                hour: '按小时',
                day: '按天',
                week: '按周',
                month: '按月'
            },
        },
        status: {
            disabled: '已禁用'
        }
    },
    // Usage
    usage: {
        memberUsageTitle: '成员使用记录',
        memberUsageDescription: '查看企业成员的请求、消费、Token 与错误记录',
        memberScopeTitle: '成员筛选',
        memberScopeDescription: '选择全部成员或单个成员，统计、图表、请求明细和错误记录会同步更新。',
        member: '企业成员',
        memberFilter: '成员',
        members: {
            all: '全部成员',
            assigned: '企业成员请求',
            unassigned: '普通 Key 请求',
            unassignedShort: '普通 Key',
            option: '{name}（{code}）',
            optionArchived: '{name}（{code}，已归档）',
            archived: '已归档'
        },
        tabs: {
            analytics: '统计分析'
        },
        analytics: {
            title: '企业使用分析',
            memberTitle: '成员用量分析',
            dimensionMember: '按成员',
            dimensionKey: '按密钥',
            memberScope: '汇总所选成员在当前时间范围内的消费、请求、Token、模型和实际分组。',
            scope: '按日期和面板筛选统计当前账号名下 API 密钥；平台管理员成本和上游路由字段不会展示。',
            singleKeyScope: '跟随上方日期筛选，仅分析当前选中的 API 密钥；平台管理员成本和上游路由字段不会展示。',
            loadFailed: '加载用量分析失败',
            errors: {
                network: '无法连接到后端服务',
                endpointMissing: '接口不存在，请确认后端已更新并重启',
                server: '服务端错误（{status}），请查看后端日志'
            },
            usedKeys: '有用量的密钥',
            activeKeysNow: '当前活跃密钥',
            nearQuotaKeys: '当前接近额度',
            nearRateLimitKeys: '当前接近速率限制',
            memberCount: '成员数',
            budgetRiskMembers: '预算风险成员',
            memberActualCost: '成员实际消费',
            reservedBudget: '已预留预算',
            budgetUsed: '本期已用预算',
            memberBudget: '本期预算',
            apiKey: 'API 密钥',
            share: '全量占比',
            change: '环比',
            model: '模型',
            keyCount: '密钥数',
            filters: {
                search: '搜索',
                searchPlaceholder: '搜索密钥名称或 Key...',
                allGroups: '全部分组',
                allTags: '全部标签',
                allStatuses: '全部状态',
                limit: '显示'
            },
            tabs: {
                memberLeaderboard: '成员排行',
                leaderboard: '密钥排行',
                trend: '消耗趋势',
                models: '模型分布',
                groups: '分组分析',
                tags: '标签归因'
            }
        }
    },
    // Shared keys for channel monitor (admin + user views)
    monitorCommon: {
        status: {
            unknown: '未知'
        },
        history60pts: '近 {n} 次检测'
    },
    // Model Status (user-facing read-only view)
    channelStatus: {
        title: '模型服务状态',
        description: '查看站点对外模型的当前健康状态、可用率和近期延迟。',
        searchPlaceholder: '搜索模型...',
        loadError: '加载模型状态失败',
        tokenUsageLoadError: '加载自检 Token 统计失败',
        detailLoadError: '加载模型详情失败',
        detailTitle: '模型详情',
        unknownGroup: '未命名分组',
        groupPrefix: '分组：',
        windowTab: {
            today: '今日',
            '24h': '24 小时'
        },
        overall: {
            operational: '全部正常',
            degraded: '部分波动',
            unavailable: '存在异常',
            unknown: '暂无数据'
        },
        summary: {
            overall: '总体状态',
            models: '监控模型',
            affected: '受影响模型',
            updated: '最后更新'
        },
        metrics: {
            latency: '平均延迟',
            avgLatency7d: '7 天平均延迟',
            selfCheckTokens: '全局自检 Token',
            selfCheckTokensScope: '按模型全局聚合，不按分组拆分',
            inputTokens: '输入',
            outputTokens: '输出',
            lastChecked: '最后检测'
        },
        message: {
            normal: '服务正常',
            partial: '部分请求可能受影响',
            unavailable: '当前模型可能不可用',
            no_data: '暂无检测数据'
        },
        empty: {
            title: '暂无模型状态',
            description: '管理员尚未配置公开模型健康监控。'
        }
    },
    // Available Channels (user-facing)
    availableChannels: {
        exportExcel: '导出 Excel',
        exporting: '导出中...',
        viewMode: {
            channels: '渠道视图',
            models: '表格视图'
        },
        platformFilter: {
            label: '平台筛选',
            all: '全部平台'
        },
        billingModeFilter: {
            label: '计费模式筛选',
            all: '全部计费'
        },
        groupScopeFilter: {
            label: '分组范围筛选',
            all: '全部分组',
            publicExclusive: '公开 + 专属',
            public: '仅公开',
            exclusive: '仅专属'
        },
        priceStatusFilter: {
            label: '价格状态筛选',
            all: '全部价格',
            priced: '有价格',
            unpriced: '未配置'
        },
        exportScope: {
            label: '导出分组范围',
            publicExclusive: '导出：公开 + 专属',
            public: '导出：仅公开',
            adminHint: '管理员导出使用全量渠道目录；订阅分组默认不导出。'
        },
        exportStatus: {
            label: '导出渠道状态',
            all: '全部状态',
            active: '仅启用',
            disabled: '仅禁用'
        },
        exportSource: {
            label: '导出数据源',
            adminCatalog: '管理员全量目录',
            visibleChannels: '当前可见可用渠道'
        },
        modelTable: {
            columns: {
                model: '模型',
                platform: '平台',
                channel: '渠道',
                billingMode: '计费模式',
                interval: '区间',
                inputPrice: '输入',
                outputPrice: '输出',
                cacheWritePrice: '缓存写入',
                cacheReadPrice: '缓存读取',
                imageOutputPrice: '图片输出',
                perRequestPrice: '按次',
                groups: '我可访问的分组',
                intervals: '阶梯定价'
            },
            tooltips: {
                interval: '阶梯定价区间；无阶梯时显示默认。',
                inputPrice: '输入价格，单位：$/1M token。表格数值省略单位。',
                outputPrice: '输出价格，单位：$/1M token。表格数值省略单位。',
                cacheWritePrice: '缓存写入价格，单位：$/1M token。表格数值省略单位。',
                cacheReadPrice: '缓存读取价格，单位：$/1M token。表格数值省略单位。',
                imageOutputPrice: '图片输出价格，通常为按图片或按次模型的单次价格。',
                perRequestPrice: '按次计费模型的单次请求价格；按 Token 计费模型显示为 -。',
                intervals: '阶梯定价按区间展示；Token 价格单位仍为 $/1M token。'
            }
        },
        export: {
            dialogTitle: '导出模型价格',
            rowCount: '导出行数',
            fullCatalogUnavailableHint: '管理员全量目录暂不可用，当前可导出页面可见的可用渠道。',
            sheetName: '模型价格',
            noData: '没有可导出的模型价格',
            success: '模型价格导出成功',
            failed: '模型价格导出失败',
            columns: {
                model: '模型',
                platform: '平台',
                channel: '渠道',
                status: '状态',
                description: '描述',
                groups: '可访问分组',
                billingMode: '计费模式',
                interval: '区间',
                inputPrice: '输入（$/1M token）',
                outputPrice: '输出（$/1M token）',
                cacheWritePrice: '缓存写入（$/1M token）',
                cacheReadPrice: '缓存读取（$/1M token）',
                imageOutputPrice: '图片输出（$/次）',
                perRequestPrice: '按次（$/次）',
                intervals: '阶梯定价'
            }
        },
        pricing: {
            perRequestPrice: '按次'
        }
    },
    // Dates
    dates: {
        startTime: '开始时间',
        endTime: '结束时间'
    },
    // Admin
    admin: {
        // Dashboard
        dashboard: {
            month: '按月'
        },
        // Users Management
        users: {
            usageDetails: '用量明细',
            viewUserUsageDetails: '查看用户全部用量',
            viewApiKeyUsageDetails: '查看此 Key 用量',
            userUsageDetailsTitle: '{email} 用量明细',
            apiKeyUsageDetailsTitle: 'API Key 用量详情：{name}',
            allApiKeysUsageScope: '全部 Key',
            apiKeyUsageScope: 'API Key'
        },
        // Accounts Management
        accounts: {
            views: {
                list: '账号列表',
                archived: '已归档',
                upstreamCost: '供应商'
            },
            upstreamCost: {
                title: '上游成本',
                settingsTitle: '上游成本配置',
                description: '记录供应商充值比例和参考汇率，用来估算供应商成本折扣；账号/key 的上游分组倍率在账号编辑里维护。',
                supplierBindingTitle: '供应商',
                supplierBindingDescription: '给这把上游 key 选择供应商，并记录它在供应商侧的分组和倍率。',
                supplier: '供应商',
                supplierPlaceholder: '请选择供应商',
                supplierEmpty: '暂无可选供应商，请先到供应商标签页新增',
                upstreamGroupName: '上游分组',
                upstreamGroupNamePlaceholder: '例如：claude-sale',
                upstreamGroupNameHint: '这把 key 在供应商侧所属的分组，用于解释综合折扣。',
                upstreamGroupMultiplier: '上游分组倍率',
                upstreamGroupMultiplierHint: '这把 key 在供应商侧的计价倍率，例如 0.8。',
                priceReferenceCurrency: '分组计价基准',
                priceReferenceCurrencyPlaceholder: '请选择人民币价或美元价',
                priceReferenceCurrencyRequired: '请选择分组计价基准，确认后才能保存供应商绑定',
                priceReferenceCurrencyHint: '按上游分组价目表的币种选择，不要按供应商所在地判断。',
                priceReferenceCurrencyCNY: '人民币官方价（不除以汇率）',
                priceReferenceCurrencyUSD: '美元官方价（除以参考汇率）',
                priceReferenceFormulaCNY: '当前公式：资金池实际成本 × {multiplier}；充值比例为 1:1 时，0.8 显示为 8 折。',
                priceReferenceFormulaUSD: '当前公式：资金池实际成本 ÷ 参考汇率 × {multiplier}。',
                priceReferenceFormulaPending: '历史绑定尚未确认计价基准；当前仅保留旧美元口径，不参与成本优先调度。',
                priceReferenceShortCNY: '人民币价基准',
                priceReferenceShortUSD: '美元价基准',
                priceReferencePending: '待确认',
                priceReferencePendingLegacy: '待确认（暂按美元旧口径）',
                addSupplier: '添加供应商',
                createSupplierTitle: '新增供应商',
                newSupplierName: '新供应商名称',
                newSupplierNamePlaceholder: '例如：供应商 A',
                saveSupplier: '保存',
                supplierCreateHint: '保存后会加入供应商列表；账号归属请回到账号编辑里选择。',
                supplierNote: '供应商备注',
                supplierNotePlaceholder: '供应商备注，例如共享钱包、折扣来源',
                editSupplierTitle: '编辑供应商',
                defaultSettlementTitle: '默认结算配置',
                defaultSettlementDescription: '用于日常新增充值记录的自动计算；修改这里只影响后续记录，不会重算历史流水。',
                defaultRechargeConversion: '默认充值换算',
                defaultRechargeConversionHint: '输入每支付 1 CNY 默认到账多少 USD 额度。',
                defaultReferenceFxRate: '默认参考汇率',
                estimatedRechargeDiscount: '预计充值折扣',
                supplierNameRequired: '请输入供应商名称',
                supplierCreated: '供应商已加入列表',
                supplierUpdated: '供应商已更新',
                supplierArchived: '供应商已归档',
                supplierUnarchived: '供应商已恢复',
                supplierDeleted: '供应商已删除',
                supplierCreateFailed: '新增供应商失败',
                supplierUpdateFailed: '更新供应商失败',
                supplierDeleteFailed: '删除供应商失败',
                archive: '归档',
                unarchive: '恢复',
                archiveSupplierTitle: '归档供应商',
                archiveSupplierConfirm: '确定归档供应商「{name}」？当前仍有 {count} 个账号绑定。已有绑定会继续生效，但该供应商会从新账号绑定候选中隐藏。',
                deleteSupplierTitle: '删除供应商',
                deleteSupplierConfirm: '确定删除供应商「{name}」？只有从未绑定账号、没有充值记录、成本快照或非默认资金池的供应商可以删除；已使用过的供应商请改用归档。',
                supplierBindingUpdatePartialFailed: '账号已保存，但供应商绑定保存失败，请重试',
                supplierBindingHint: '充值金额、到账额度和参考汇率到供应商页维护；这里维护这把 key 的供应商归属、上游分组和分组倍率。',
                supplierLoadFailed: '加载上游供应商失败',
                supplierPool: '主余额池',
                supplierRechargeScope: '供应商充值',
                boundAccounts: '绑定账号',
                currentCost: '当前成本',
                supplierNoPool: '未建余额池',
                noSuppliers: '暂无供应商，请点击右上角新增。',
                status: '状态',
                completeStatus: '已配完整',
                archivedStatus: '已归档',
                needsConfig: '暂无充值成本',
                loadFailed: '加载供应商失败',
                rechargeCnyPerUsd: '充值人民币/美元额度',
                rechargeCnyPerUsdHint: '例如 1 表示 1 元人民币买到 1 美元额度',
                referenceFxRate: '参考汇率',
                referenceFxRateHint: '默认 7，可按你自己的成本口径手动修改',
                groupMultiplier: '默认分组倍率',
                groupMultiplierHint: '上游分组展示倍率，例如 codex 分组 0.5',
                note: '成本备注',
                notePlaceholder: '例如：月付套餐、限时折扣、需手动核对余额',
                rechargeDiscount: '充值折扣',
                poolDiscountUSD: '资金池折扣（美元基准）',
                defaultMultiplier: '默认倍率',
                effectiveDiscount: '综合折扣',
                displayDiscount: '折扣展示',
                discountSuffix: '折',
                missingFields: '还缺：{fields}',
                familyOverrides: '模型系列倍率',
                familyOverridesHint: '同一供应商不同系列倍率不一样时，在这里覆盖；例如 haiku、sonnet、opus。',
                addFamily: '添加系列',
                noFamilyOverrides: '暂无模型族覆盖，默认使用供应商默认倍率。',
                familyPlaceholder: '模型系列，例如 sonnet',
                familyNotePlaceholder: '系列备注',
                defaultFamily: '默认倍率',
                rechargeRatio: '充值比例',
                multiplier: '分组倍率',
                notConfigured: '未配置',
                errors: {
                    nameConflict: '已有同名的启用供应商',
                    reserved: '系统保留供应商不能修改或删除',
                    hasBoundAccounts: '该供应商仍有账号绑定，请先解绑；有历史数据时请改用归档',
                    hasBindingHistory: '该供应商已有账号绑定历史，为保留审计记录请改用归档',
                    hasCostData: '该供应商已有资金池、充值记录或成本快照，请归档而不是删除',
                    UPSTREAM_SUPPLIER_BINDING_REQUIRED: '请先在账号编辑中绑定真实供应商，再新增充值记录'
                },
                balanceQuery: {
                    title: 'Key 配额查询',
                    provider: '面板类型',
                    providerSub2Api: 'Sub2API',
                    providerNewApi: 'New API 兼容',
                    endpoint: '接口路径',
                    authMode: '认证方式',
                    authModeAccountKey: '使用当前 API Key',
                    authModeBearer: '上游 Bearer Token',
                    authModeCustomHeader: '自定义 Header',
                    authHeader: 'Header 名',
                    authToken: '查询 Token',
                    authTokenPlaceholder: '填写上游提供的 Bearer Token 或专用查询 Token',
                    authTokenConfigured: '已配置，留空则保留',
                    authTokenHint: '仅在 Bearer 或自定义 Header 模式使用，不参与模型请求转发',
                    authTokenRequired: '请填写上游查询 Token',
                    balance: 'Key 配额',
                    refresh: '刷新配额',
                    refreshShort: '配额',
                    refreshAll: '批量刷新配额',
                    refreshingAll: '刷新中',
                    refreshAllProgress: '{done}/{total}',
                    disabled: '未启用',
                    pending: '未查询',
                    notFetched: '还没有刷新过',
                    failed: '查询失败',
                    unlimited: '不限额',
                    refreshSuccess: 'Key 配额已刷新',
                    refreshFailed: 'Key 配额查询失败',
                    refreshSkipped: '账号正在刷新',
                    noRefreshableAccounts: '当前列表没有开启 Key 配额查询的账号',
                    batchRefreshSuccess: '已刷新 {count} 个 Key 配额',
                    batchRefreshPartial: 'Key 配额刷新完成：成功 {success} 个，失败 {failed} 个',
                    batchRefreshFailed: 'Key 配额刷新失败：{failed} 个账号未刷新成功'
                },
                rechargeRecords: {
                    action: '充值记录',
                    title: '上游充值记录',
                    type: '类型',
                    typeRecharge: '充值',
                    typeBonus: '赠送',
                    typeAdjustment: '调整',
                    paidAmount: '支付金额',
                    receivedCredit: '到账额度',
                    autoCalculated: '自动计算',
                    recalculateCredit: '按配置重算',
                    useDefaultCalculation: '使用默认计算',
                    overrideThisRecord: '本次与默认不同',
                    defaultConfigApplied: '默认按 {conversion}、参考汇率 {fx} 计算；特殊到账可单独覆盖。',
                    currency: '币种',
                    recordedAt: '时间',
                    addRecord: '新增记录',
                    editRecord: '编辑记录',
                    history: '记录明细',
                    records: '条记录',
                    totalPaid: '累计支付',
                    totalCredit: '累计到账',
                    weightedCost: '累计成本',
                    latestCost: '最近成本',
                    effectiveCost: '实际成本',
                    applyLatest: '应用最近成本',
                    applyWeighted: '应用累计成本',
                    poolAutoApplyHint: '保存后会自动更新该供应商当前成本。',
                    empty: '暂无充值记录',
                    notePlaceholder: '订单号、活动、到账说明等',
                    deleteTitle: '删除充值记录',
                    deleteMessage: '删除后不会参与成本汇总。',
                    loadFailed: '加载充值记录失败',
                    saveFailed: '保存充值记录失败',
                    deleteFailed: '删除充值记录失败',
                    applyFailed: '应用成本失败',
                    created: '充值记录已新增',
                    saved: '充值记录已保存',
                    deleted: '充值记录已删除',
                    applied: '成本参数已更新'
                }
            },
            archiveAccount: '归档账号',
            archiveAction: '归档',
            restoreAccount: '恢复账号',
            archiveConfirm: "确定要归档账号 '{name}' 吗？归档会保留历史用量和分组关系，但账号不会继续参与调度。",
            archiveBulkConfirm: '确定要归档已选择的账号吗？归档会保留历史用量和分组关系。',
            archiveRequiresDisabled: '请先将账号状态改为已停用，再执行归档。',
            columns: {
                archivedAt: '归档时间'
            },
            clearModelRateLimitSuccess: '已解除该模型的限流',
            clearModelRateLimitFailed: '解除模型限流失败',
            accountArchived: '账号归档成功',
            accountRestored: '账号已恢复为已停用状态',
            bulkArchiveSuccess: '成功归档 {count} 个账号',
            bulkArchivePartial: '部分账号归档成功：成功 {success} 个，失败 {failed} 个',
            status: {
                inactive: '非活跃',
                disabled: '已停用'
            },
            bulkActions: {
                delete: '批量归档'
            },
            failedToArchive: '归档账号失败',
            failedToRestore: '恢复账号失败',
            // OpenAI specific hints
            openai: {
                cacheTokenUsageMode: '缓存 Token 口径',
                cacheTokenUsageModeDesc: '决定上游 usage 里的输入 Token 是否已经包含缓存命中。OpenAI 官方一般选默认；new-api 等代理如果把 prompt_tokens 与 cached_tokens 分开返回，选“输入不含缓存”。',
                cacheTokenUsageIncludes: '输入包含缓存（OpenAI 默认）',
                cacheTokenUsageExcludes: '输入不含缓存（new-api/代理）'
            },
            probeSupportedModels: '获取支持模型',
            probingSupportedModels: '获取中...',
            probeModelsMissingBaseUrl: '请先填写 Base URL',
            probeModelsMissingApiKey: '请先填写 API Key',
            probeModelsSuccess: '已获取并添加 {count} 个模型',
            probeModelsNoNewModels: '未发现新的支持模型',
            probeModelsSummary: '获取完成：新增 {added} 个，已存在 {existing} 个，获取异常 {missing} 个',
            probeModelsSummaryWithConflicts: '获取完成：新增 {added} 个，已存在 {existing} 个，获取异常 {missing} 个，源模型冲突 {conflicts} 个',
            probeModelNew: '本次新增',
            probeModelMissing: '获取异常',
            probeModelsFailed: '获取支持模型失败，请检查 Base URL / API Key，或手动填写',
            probeModelsEndpointMissing: '获取支持模型接口不存在，请重启或更新后端服务后重试',
            fillRelatedModels: '填入相关模型',
            queryModelCatalog: '查询',
            queryingModelCatalog: '查询中',
            modelCatalogQueryRequired: '请先输入模型关键词',
            modelCatalogLoadFailed: '模型目录查询失败，请稍后重试',
            modelCatalogNoResults: '没有找到匹配模型',
            modelCatalogResultHint: '来自 models.dev 公共模型库，选择后会直接填入',
            modelCatalogDisclaimer: '来自 models.dev 公共模型库，不代表当前账号一定支持'
        },
        // Usage Records
        usage: {
            apiKeyDeletedBadge: '已删除',
            accountArchivedBadge: '已归档',
            apiKeyActiveBadge: '活跃',
            apiKeyId: 'API Key ID',
            apiKeyStatus: 'API Key 状态',
            apiKeyDeletedAt: 'API Key 删除时间',
            profile: {
                globalTitle: '全站用量分析',
                userTitle: '{user} 的用量分析',
                apiKeyTitle: 'API Key 用量分析：{apiKey}',
                userApiKeyTitle: '{user} 下的 Key 用量分析：{apiKey}',
                rangeSubtitle: '当前范围：{start} 至 {end}',
                balanceHistory: '充值记录',
                userKeys: '用户 Keys',
                clearUser: '清除用户',
                clearApiKey: '清除 Key',
                objectFilter: '分析对象',
                objectFilterPlaceholder: '选择用户或 API 密钥',
                clearObjectFilter: '清除分析对象',
                usersColumn: '用户',
                keysColumn: 'API 密钥',
                allUserKeys: '所有 Key',
                allUserKeysHelp: '查看该用户全部 Key 的用量',
                selectUserFirst: '先在左侧选择用户，再查看该用户的 API 密钥。',
                searchUsersHint: '输入邮箱搜索用户',
                noUsersFound: '未找到用户',
                noKeysFound: '暂无 API 密钥',
                includeDeletedApiKeys: '包含已删除 Key',
                loadingUser: '用户 #{id}',
                loadingApiKey: 'API Key #{id}',
                userMissing: '未找到该用户 #{id}',
                apiKeyMissing: '未找到该 API Key #{id}',
                userMissingHelp: '未找到该用户 #{id}，或当前管理员无权访问。请清除用户筛选后重新选择。',
                apiKeyMissingHelp: '未找到该 API Key #{id}，或它不属于当前用户筛选。请清除 Key 筛选后重新选择。',
                viewUsageProfile: '用量分析',
                viewUserUsageProfile: '查看用户用量分析',
                viewApiKeyUsageProfile: '查看 Key 用量分析'
            }
        },
        // Ops Monitoring
        ops: {
            customerVisibleFailures: '客户可见失败',
            totalFailures: '全部失败',
            customerSideLimits: 'SLA 排除项',
            slaErrors: '平台 SLA 失败',
            platformAvailability: '平台可用性',
            platformSlaFailures: '平台 SLA 失败',
            slaExcludedFailures: 'SLA 排除项',
            unclassifiedFailures: '未分类',
            currentState: {
                active: '当前仍在故障',
                recovered: '当前已恢复',
                quiet: '当前无平台故障',
                unknown: '当前状态待确认'
            },
            failureDomain: {
                customer: '账户与密钥策略',
                enterprise: '企业成员策略',
                client: '客户端请求与中断',
                platformRouting: '平台路由容量',
                platformInternal: '平台内部故障',
                upstream: '最终上游失败'
            },
            upstreamNonRateErrors: '非限流上游错误',
            upstreamRateOverload: '上游限流/过载',
            errorCountExcl429529: '非限流上游错误',
            sla: '平台可用性',
            businessLimited: 'SLA 排除项：',
            errorsSla: '平台 SLA 失败',
            upstreamExcl429529: '非限流上游错误',
            // Error Details Modal
            errorDetails: {
                viewErrors: '平台 SLA 失败',
                viewExcluded: 'SLA 排除项',
                viewAllFailures: '全部失败',
                statusRateOverload: '429/529 限流/过载',
                statusNonRateOverload: '非 429/529',
                filters: {
                    search: '搜索',
                    statusCode: '状态码',
                    phase: '错误阶段',
                    owner: '归属方',
                    scope: '显示范围',
                    domain: '失败域',
                    category: '失败类别',
                    resolutionOwner: '处置责任方',
                    slaImpact: 'SLA 归属'
                },
                domain: {
                    customer: '客户账户',
                    enterprise: '企业成员',
                    client: '客户端',
                    platform: '平台',
                    upstream: '上游服务',
                    unknown: '未知'
                },
                category: {
                    authentication: '认证',
                    balance: '余额',
                    budget: '预算',
                    quota: '配额',
                    rate_limit: '速率限制',
                    concurrency: '并发限制',
                    permission: '权限',
                    capability: '能力/模型',
                    protocol: '请求协议',
                    routing_capacity: '路由容量',
                    non_routing: '平台非路由故障',
                    credential: '上游凭据',
                    overload: '上游过载',
                    timeout: '超时',
                    network: '网络',
                    dependency: '依赖服务',
                    internal: '内部错误',
                    cancellation: '客户端取消',
                    unknown: '未知'
                },
                resolutionOwner: {
                    customer: '客户',
                    enterprise_admin: '企业管理员',
                    platform_ops: '平台运维',
                    client: '客户端',
                    unknown: '待确认'
                },
                poolOwnership: {
                    platform: '平台托管',
                    enterprise: '企业自管',
                    unknown: '不适用/未知'
                },
                slaImpact: {
                    included: '计入平台 SLA',
                    excluded: '不计入平台 SLA',
                    unknown: 'SLA 归属待确认'
                },
                searchPlaceholder: '搜索请求 ID、客户端请求 ID、错误信息'
            },
            // Error Detail Modal
            errorDetail: {
                businessLimited: 'SLA 排除项',
                classification: {
                    title: '结构化失败归因',
                    customerVisible: '客户可见',
                    domain: '失败域',
                    category: '失败类别',
                    reason: '原因代码',
                    resolutionOwner: '处置责任方',
                    poolOwnership: '账号池归属'
                }
            },
            alertRules: {
                metrics: {
                    errorRate: '平台 SLA 失败率 (%)'
                },
                metricDescriptions: {
                    errorRate: '统计窗口内计入平台 SLA 的最终失败占比；客户与企业限制、客户端中断和已恢复尝试不计入。'
                }
            },
            settings: {
                requestErrorRateMaxPercent: '平台 SLA 失败率最大值（%）',
                requestErrorRateMaxPercentHint: '平台 SLA 失败率高于此值时显示为红色（默认：5%）'
            },
            runtime: {
                requestErrorRateMaxPercent: '平台 SLA 失败率最大值（%）',
                requestErrorRateMaxPercentHint: '平台 SLA 失败率高于此值时显示为红色（默认：5%）'
            },
            tooltips: {
                errorTrend: '失败趋势按客户可见、平台 SLA 和 SLA 排除项使用同一结构化分类口径。',
                errorDistribution: '按状态码统计客户最终可见失败，并区分平台 SLA 与排除项。',
                upstreamErrors: '上游服务返回的错误，拆分为非限流错误和 429/529 限流/过载。',
                sla: '平台可用性只统计平台负责的最终失败；客户余额、企业预算、客户端取消和已恢复的上游尝试不计入。',
                customerVisibleFailures: '客户最终实际收到的失败响应，按客户、企业、客户端、平台和上游归因拆分。',
                errors: '平台 SLA 失败统计，未知归因会单独提示，不会静默计为健康。'
            }
        },
        // Settings
        settings: {
            features: {
                modelSelfCheck: {
                    title: '模型自检',
                    description: '按渠道定价中启用的模型定时发起极小探针，向用户展示站点模型健康状态，不暴露上游账号或渠道。',
                    configureLink: '前往 渠道管理 > 渠道定价 选择要公开自检的模型',
                    enabled: '启用模型自检',
                    enabledHint: '关闭后停止后台探针，用户端模型状态页返回空列表。',
                    defaultInterval: '默认自检间隔（秒）',
                    defaultIntervalHint: '每个去重后的模型与账号探针使用该间隔。范围 60 – 86400 秒。',
                    maxConcurrency: '全局并发上限',
                    maxConcurrencyHint: '同一时刻允许提交的自检探针数量。范围 1 – 64。',
                    maxTasksPerRound: '单轮任务上限',
                    maxTasksPerRoundHint: '每次刷新最多调度的去重探针数量，防止误开大量模型时刷爆上游。范围 1 – 10000。',
                    snapshotRetentionDays: '状态快照保留天数',
                    snapshotRetentionDaysHint: '用于保留无可用账号等用户可见状态证据。0 表示关闭自动清理；正数有效范围 30 – 3650 天，1 – 29 会按 30 天保存。',
                },
                keyRateChangeGuard: {
                    title: '分组倍率变更保护',
                    description: '修改分组默认倍率或用户专属倍率后，自动停用受影响的 API Key，要求用户确认后重新启用。',
                    enabled: '倍率变更后停用受影响 Key',
                    enabledHint: '默认关闭。开启后仅停用仍沿用被修改倍率的有效 Key；手动重新启用会清除系统停用原因。',
                }
            },
            scheduling: {
                strategy: '调度策略',
                strategyHint: '严格优先级保持现有 priority 调度；成本优先会在健康候选中优先选择综合折扣最低的账号，未配置成本池的账号靠后。',
                strategyStrictPriority: '严格优先级',
                strategyCostFirst: '成本优先'
            },
            modelRateLimit: {
                title: '模型级限流策略',
                description: '配置某个模型连续失败多少次后才触发模型级限流，以及触发后的回退冷却时长',
                enabled: '启用失败阈值',
                enabledHint: '关闭时（默认）保持历史行为：模型首次失败即限流。启用后按窗口内连续失败次数判定',
                failureThreshold: '失败阈值（次）',
                failureThresholdHint: '窗口内连续失败达到该次数才对模型限流（1-100）',
                windowMinutes: '统计窗口（分钟）',
                windowMinutesHint: '失败计数的滑动窗口时长（1-1440 分钟），窗口内无新失败则自动衰减',
                cooldownSeconds: '回退冷却（秒）',
                cooldownSecondsHint: '触发限流后的默认冷却时长（1-7200 秒）；上游返回明确 reset 时仍优先使用上游时间',
                saved: '模型级限流设置保存成功',
                saveFailed: '保存模型级限流设置失败'
            }
        }
    }
}
