import { defineConfig } from 'vitepress'

export default defineConfig({
  lang: 'zh-CN',
  title: 'Sub2API 文档中心',
  description: 'Sub2API 项目源文档与 dev-zz 二开文档',
  cleanUrls: true,
  ignoreDeadLinks: false,
  themeConfig: {
    logo: undefined,
    nav: [
      { text: '首页', link: '/' },
      { text: '项目文档', link: '/project/' },
      { text: 'dev-zz', link: '/dev-zz/' },
      { text: '接口索引', link: '/dev-zz/reference/api-surface' },
      { text: '本地开发', link: '/dev-zz/development/local-development' }
    ],
    sidebar: [
      {
        text: '项目文档',
        items: [
          { text: '项目总览', link: '/project/' },
          { text: '项目说明', link: '/project/overview' },
          { text: '支付配置', link: '/project/payment/payment-cn' },
          { text: 'Admin 支付集成 API', link: '/project/payment/admin-payment-integration-api' }
        ]
      },
      {
        text: 'dev-zz 总览',
        items: [
          { text: 'dev-zz 文档入口', link: '/dev-zz/' },
          { text: '变更地图', link: '/dev-zz/reference/change-map' },
          { text: '分支策略', link: '/dev-zz/branch-policy' },
          { text: '变更记录', link: '/dev-zz/changelog' },
          { text: '补丁记录', link: '/dev-zz/patches' }
        ]
      },
      {
        text: '开发与部署',
        items: [
          { text: '完全本地开发', link: '/dev-zz/development/local-development' },
          { text: 'dev-zz 部署', link: '/dev-zz/deployment/deploy-dev-zz' }
        ]
      },
      {
        text: '分支、复盘与旧稿',
        items: [
          { text: '同步上游 main', link: '/dev-zz/maintenance/merge-main' },
          { text: '上游合并记录', link: '/dev-zz/maintenance/merge-log' },
          { text: '前端白屏复盘', link: '/dev-zz/maintenance/frontend-white-screen-2026-06-17' },
          { text: '删除 Key 用量排查', link: '/dev-zz/maintenance/deleted-key-usage-ledger-triage-2026-06-22' },
          { text: '企业 Key 成员管理（历史方案）', link: '/dev-zz/features/enterprise-key-member-management' },
          { text: 'dev-zz-apipool 分支清单', link: '/dev-zz/maintenance/dev-zz-apipool-branch-inventory' },
          { text: 'DEV_SEED stash 设计清单', link: '/dev-zz/maintenance/stash-dev-seed-design' },
          { text: '维度化计费 stash 设计清单', link: '/dev-zz/maintenance/stash-billing-dimensional-pricing' }
        ]
      },
      {
        text: '参考索引',
        items: [
          { text: 'dev-zz 接口索引', link: '/dev-zz/reference/api-surface' },
          { text: '配置与迁移索引', link: '/dev-zz/reference/configuration-and-migrations' },
          { text: '验证矩阵', link: '/dev-zz/testing/verification-matrix' }
        ]
      },
      {
        text: '已落地功能',
        items: [
          { text: '可用渠道模型广场', link: '/dev-zz/features/available-channels-model-marketplace' },
          { text: 'API Key 用量下钻', link: '/dev-zz/features/api-key-usage-drilldown' },
          { text: '管理员用量分析下钻', link: '/dev-zz/features/admin-usage-profile-drilldown' }
        ]
      },
      {
        text: '部分落地功能',
        items: [
          { text: '企业用量分析中心', link: '/dev-zz/features/enterprise-usage-analytics' },
          { text: '用量账本与已删除 Key 证据', link: '/dev-zz/features/usage-ledger-evidence-integrity' },
          { text: '客户可见错误排障', link: '/dev-zz/features/ops-customer-visible-error-triage' },
          { text: '模型自检监控（定价驱动）', link: '/dev-zz/features/pricing-driven-self-check-monitoring-design' },
          { text: '上游供应商资金池与成本账本', link: '/dev-zz/features/upstream-cost-pools-and-ledger' }
        ]
      },
      {
        text: '方案稿（未实现）',
        items: [
          { text: '企业用户成员管理（完整目标设计）', link: '/dev-zz/features/enterprise-member-management' },
          { text: '上游成本感知调度', link: '/dev-zz/features/upstream-provider-cost-aware-scheduling' },
          { text: '模型自检 Token 消耗统计', link: '/dev-zz/features/self-check-token-usage-stats' },
          { text: '改倍率停用受影响 Key', link: '/dev-zz/features/disable-keys-on-group-rate-change' }
        ]
      },
      {
        text: '设计取舍',
        items: [
          { text: '0001：文档中心', link: '/dev-zz/decisions/adr-0001-docs-site-as-dev-zz-doc-hub' },
          { text: '0002：Key 作为成员（已取代）', link: '/dev-zz/decisions/adr-0002-key-as-enterprise-member' },
          { text: '0003：不可登录成员实体', link: '/dev-zz/decisions/adr-0003-enterprise-member-entity' }
        ]
      }
    ],
    search: {
      provider: 'local'
    },
    outline: {
      label: '本页目录',
      level: [2, 3]
    },
    docFooter: {
      prev: '上一页',
      next: '下一页'
    },
    returnToTopLabel: '回到顶部',
    sidebarMenuLabel: '菜单',
    darkModeSwitchLabel: '外观',
    lightModeSwitchTitle: '切换到浅色模式',
    darkModeSwitchTitle: '切换到深色模式',
    skipToContentLabel: '跳到内容'
  }
})
