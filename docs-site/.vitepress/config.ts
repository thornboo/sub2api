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
        text: '维护',
        items: [
          { text: '合并 main 到 dev-zz', link: '/dev-zz/maintenance/merge-main' },
          { text: '上游合并记录', link: '/dev-zz/maintenance/merge-log' },
          { text: '配置与迁移索引', link: '/dev-zz/reference/configuration-and-migrations' },
          { text: '验证矩阵', link: '/dev-zz/testing/verification-matrix' }
        ]
      },
      {
        text: '功能与决策',
        items: [
          { text: '可用渠道模型广场', link: '/dev-zz/features/available-channels-model-marketplace' },
          { text: '企业 Key 成员管理', link: '/dev-zz/features/enterprise-key-member-management' },
          { text: 'API Key 用量下钻', link: '/dev-zz/features/api-key-usage-drilldown' },
          { text: '企业用量分析中心', link: '/dev-zz/features/enterprise-usage-analytics' },
          { text: 'dev-zz 接口索引', link: '/dev-zz/reference/api-surface' },
          { text: 'ADR 0001: 文档中心', link: '/dev-zz/decisions/adr-0001-docs-site-as-dev-zz-doc-hub' },
          { text: 'ADR 0002: Key 作为成员', link: '/dev-zz/decisions/adr-0002-key-as-enterprise-member' }
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
