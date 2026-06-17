# 2026-06-17 前端白屏事故分析审查包

> 目的：给第二审查者复核本次白屏根因与修复方案。本文区分已确认事实、可复现实验、推断链、反证项和剩余不确定性。

## 结论摘要

当前最高置信结论：本次正式环境白屏由前端生产构建的手写 `manualChunks` 拆包引发。该拆包把 Vue/UI/杂项依赖切成了互相静态 import 的 ESM chunk，事故构建中出现 `vendor-vue <-> vendor-misc` 循环，浏览器在模块初始化阶段触发 temporal dead zone 错误：

```text
ReferenceError: Cannot access 'W' before initialization
```

由于错误发生在 Vue `app.mount('#app')` 之前，页面保持空的 `<div id="app"></div>`，用户看到纯白屏。

置信度：高。原因是事故 HTML 的 asset hash 与本地未修复构建产物一致，并且同 hash 入口模块在本地导入时复现了初始化错误。

非 100% 的原因：事故当时没有保存浏览器 Console 原始截图；最终判断依赖线上 HTML/容器日志、本地同 hash 构建复现、bundle import graph 分析。

## 事故输入证据

### 现场执行顺序

以下顺序按事故时操作者贴出的终端和截图整理，时间以原始输出为准。

1. 查看生产容器状态：

   ```bash
   docker ps --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}'
   ```

   输出显示：

   ```text
   NAMES                   IMAGE                               STATUS                   PORTS
   sub2api                 thornboo/sub2api:latest             Up 4 minutes (healthy)   0.0.0.0:8080->8080/tcp
   1Panel-openresty-bRpQ   1panel/openresty:1.31.1.1-0-noble   Up 6 days
   sub2api-postgres        postgres:18-alpine                  Up 6 days (healthy)      5432/tcp
   sub2api-redis           redis:8-alpine                      Up 6 days (healthy)      6379/tcp
   ```

   结论：新 `sub2api` 容器启动成功，健康检查通过，Postgres/Redis 未重启且健康。

2. 在生产机本地直连应用端口检查首页：

   ```bash
   curl -I http://127.0.0.1:8080/
   curl -sS http://127.0.0.1:8080/ | head -40
   ```

   结果：

   ```text
   HTTP/1.1 200 OK
   Cache-Control: no-cache
   Content-Type: text/html; charset=utf-8
   X-Request-Id: d2ad3bb6-e99a-49d2-9cb5-90aa126afbfe
   Date: Wed, 17 Jun 2026 02:43:59 GMT
   ```

   HTML 中引用事故构建资源：

   ```html
   <script type="module" crossorigin src="/assets/index-Bpdsxip0.js"></script>
   <link rel="modulepreload" crossorigin href="/assets/vendor-ui-bNV8SlBa.js">
   <link rel="modulepreload" crossorigin href="/assets/vendor-misc-BHYy-3GM.js">
   <link rel="modulepreload" crossorigin href="/assets/vendor-vue-LkQH8kTv.js">
   <link rel="modulepreload" crossorigin href="/assets/vendor-i18n-BrBN-qJG.js">
   <link rel="stylesheet" crossorigin href="/assets/vendor-misc-DB0Q8XAf.css">
   <link rel="stylesheet" crossorigin href="/assets/index-DOJft3-i.css">
   ```

   结论：源站直接返回 200 HTML，且 HTML 已经包含本次事故的坏 hash 资源。

3. 通过浏览器打开裸域名：

   ```text
   https://zedrouter.top
   ```

   现象：浏览器首屏纯白，无任何可见内容。

   结论：服务端 HTML 可达，但前端没有完成渲染。因为 Vue app 的挂载点初始只有空的 `<div id="app"></div>`，如果入口 JS 在挂载前失败，用户看到的就是纯白。

4. 域名检查：

   ```bash
   curl -I https://zedrouter/
   ```

   输出：

   ```text
   curl: (6) Could not resolve host: zedrouter
   ```

   该命令使用了无效主机名，不参与事故判断。

   随后检查正式 www 域名：

   ```bash
   curl -I https://www.zedrouter.top/
   ```

   输出：

   ```text
   HTTP/2 200
   server: cloudflare
   content-type: text/html; charset=utf-8
   cf-cache-status: DYNAMIC
   strict-transport-security: max-age=31536000; includeSubDomains
   ```

   结论：Cloudflare 到源站的首页请求也是 200，不是 502/523/源站不可达。

5. 查看当时最新 `sub2api` 容器日志。

   操作者贴出的最新日志集中在 `2026-06-17T11:05:02+0800` 到 `2026-06-17T11:06:38+0800`。日志主要显示 API 请求仍在正常处理，另有一次首页访问和一次鉴权接口访问。

6. 操作者随后执行回滚。回滚后再次检查线上 HTML，资源 hash 已变为另一组，例如：

   ```text
   /assets/index-Bk9ucR4j.js
   /assets/vendor-vue-BBcbT6nF.js
   /assets/vendor-i18n-Dkod-nXX.js
   /assets/vendor-misc-CfSnIdM4.js
   /assets/index-BvsMLyo8.css
   ```

   结论：回滚恢复是通过避开事故构建资源完成的，而不是通过修复数据库、Redis、后端配置或 Cloudflare 配置完成的。

### 当时最新容器日志摘录

以下不是完整日志，只保留和“白屏是否由前端 bundle 导致”相关的代表性行。完整日志中大量 `/v1/responses` 请求为 API 业务流量。

1. 事故期间 API 仍大量 200：

   ```text
   2026-06-17T11:05:04.030+0800 INFO http request completed
   {"path":"/v1/responses","method":"POST","status_code":200,"latency_ms":30970,"client_ip":"117.139.103.13","platform":"openai","model":"gpt-5.4"}

   2026-06-17T11:05:08.128+0800 INFO http request completed
   {"path":"/v1/responses","method":"POST","status_code":200,"latency_ms":22527,"client_ip":"117.139.103.13","platform":"openai","model":"gpt-5.5"}

   2026-06-17T11:06:38.351+0800 INFO http request completed
   {"path":"/v1/responses","method":"POST","status_code":200,"latency_ms":68808,"client_ip":"117.139.103.13","platform":"openai","model":"gpt-5.5"}
   ```

   结论：后端请求处理链路没有整体故障。

2. 首页请求返回 200：

   ```text
   2026-06-17T11:05:34.230+0800 INFO http request completed
   {"path":"/","method":"GET","status_code":200,"latency_ms":0,"client_ip":"58.152.106.240"}
   ```

   结论：白屏不是因为首页 HTML 500/404。

3. 浏览器侧已经执行到登录态接口：

   ```text
   2026-06-17T11:05:47.078+0800 INFO http request completed
   {"path":"/api/v1/auth/me","method":"GET","status_code":200,"latency_ms":7,"client_ip":"112.18.239.238"}
   ```

   结论：前端资源至少有部分代码路径能够触发 API 请求，或浏览器缓存/已有页面逻辑仍在请求鉴权接口；但页面仍白屏，说明不能仅用“后端接口不可用”解释。

4. 与前端白屏无直接关系的流式请求断开：

   ```text
   2026-06-17T11:05:17.631+0800 INFO Client disconnected during streaming, continuing to drain upstream for billing
   2026-06-17T11:05:32.263+0800 WARN openai.ws_bind_response_account_failed {"error":"context canceled"}
   ```

   结论：这些是流式 API 客户端断开或上下文取消日志，不能解释浏览器首页纯白。

### 反向代理日志观察

事故前后贴出的 openresty 日志包含大量公网扫描和无效路径请求，例如 `zgrab`、`visionheight.com/scan`、`timeclock.php`、Mongo/RDP 探测字节、以及针对 `www.panel.zedrouter.top` 的静态文件 404。它们多发生在 `2026-06-15` 到 `2026-06-16`，与 `2026-06-17 11:05 +0800` 的白屏现场不是同一个直接时间窗口。

因此 openresty 扫描噪音目前不作为本次白屏根因证据，只作为“公网暴露服务会持续收到扫描请求”的背景信息。

## 本地复现实验

### 坏包导入复现

在同 hash 事故构建产物上执行 Node + jsdom 入口导入 smoke，入口导入失败：

```text
UNCAUGHT ReferenceError: Cannot access 'W' before initialization
    at Po (.../vendor-vue-LkQH8kTv.js:13:10430)
    at .../vendor-misc-BHYy-3GM.js:84:44386
```

该错误发生在模块初始化阶段，早于 `frontend/src/main.ts` 中的 `bootstrap()` 业务流程完成。

### 坏包静态 import 图

事故构建的关键静态依赖关系：

```text
vendor-vue-LkQH8kTv.js  -> vendor-misc-BHYy-3GM.js
vendor-misc-BHYy-3GM.js -> vendor-vue-LkQH8kTv.js
vendor-misc-BHYy-3GM.js -> vendor-ui-bNV8SlBa.js
vendor-ui-bNV8SlBa.js   -> vendor-vue-LkQH8kTv.js
```

核心循环：

```text
vendor-vue <-> vendor-misc
```

这与 `Cannot access ... before initialization` 类型错误一致。

## 根因定位

未修复前 `frontend/vite.config.ts` 中有手写 vendor 拆包：

```ts
manualChunks(id: string) {
  if (id.includes('node_modules')) {
    if (
      id.includes('/vue/') ||
      id.includes('/vue-router/') ||
      id.includes('/pinia/') ||
      id.includes('/@vue/')
    ) {
      return 'vendor-vue'
    }

    if (id.includes('/@vueuse/') || id.includes('/xlsx/')) {
      return 'vendor-ui'
    }

    if (id.includes('/chart.js/') || id.includes('/vue-chartjs/')) {
      return 'vendor-chart'
    }

    if (id.includes('/vue-i18n/') || id.includes('/@intlify/')) {
      return 'vendor-i18n'
    }

    return 'vendor-misc'
  }
}
```

风险点：

1. `id.includes('/vue/')` 过宽，不只匹配 Vue 核心包，也可能匹配 Vue adapter 或 pnpm 路径中带 Vue peer 依赖标记的包。
2. UI 组件和浮层相关依赖可能跨 `vendor-vue`、`vendor-ui`、`vendor-misc` 三个 chunk。
3. 当 `vendor-vue` 需要 `vendor-misc` 中的 DOM/platform 工具，同时 `vendor-misc` 又静态 import Vue runtime，ESM 初始化顺序会形成 TDZ 风险。

因此，手写拆包是必要解释项；没有它，默认 Rollup chunk graph 不再产生该静态循环。

## 当前修复方案

### 1. 删除手写 `manualChunks`

当前补丁删除了 `frontend/vite.config.ts` 中的 `manualChunks`，保留默认 Rollup/Vite 拆分：

```ts
build: {
  outDir: '../backend/internal/web/dist',
  // Keep Rollup's default chunk graph. A previous manual vendor split forced
  // Vue/UI dependencies into circular chunks and caused production white screens.
  emptyOutDir: true
}
```

这是根因修复。它放弃人工 vendor 分包，避免再次把第三方依赖切出循环初始化图。

### 2. 添加 HTML 级启动失败兜底

`frontend/index.html` 在 `<div id="app"></div>` 后、入口 module 前增加一个带 nonce placeholder 的 inline script：

```html
<script nonce="__CSP_NONCE_VALUE__">
  ...
</script>
```

作用：

1. 监听 `error`，覆盖入口模块初始化错误和 script load 错误。
2. 监听 `unhandledrejection`，覆盖异步启动失败。
3. 监听 `sub2api:bootstrap-error`，接收 `main.ts` 主动转发的启动失败。
4. 如果 10 秒后 `#app` 仍为空，显示“前端加载失败”而不是纯白屏。

注意：生产构建后 Vite 会把 module 入口移动到 `<head>`，但 module script 是 defer 语义；body 中的经典 inline script 会在文档解析期间执行，早于 module 入口执行。第二审查已确认该执行顺序成立。

### 3. `main.ts` 捕获 bootstrap 错误

`frontend/src/main.ts` 现在将：

```ts
bootstrap()
```

改为：

```ts
bootstrap().catch((error: unknown) => {
  console.error('[sub2api] frontend bootstrap failed', error)
  window.dispatchEvent(new CustomEvent('sub2api:bootstrap-error', { detail: error }))
})
```

注意：这不能捕获静态 import 初始化错误，因此不是本次根因修复；它只是防止后续异步启动失败继续表现为纯白屏。

## 修复后验证

已执行：

```bash
pnpm --dir frontend build
pnpm --dir frontend lint:check
cd backend && go test -tags=embed ./internal/web
git diff --check
```

结果：

```text
pnpm --dir frontend build          passed
pnpm --dir frontend lint:check     passed
go test -tags=embed ./internal/web passed
git diff --check                   passed
```

修复后构建产物入口：

```text
/assets/index-C7MJMXR9.js
/assets/index-9G87QxLR.css
```

修复后静态 import 图检查：

```text
js chunks 128
static import edges 599
cycles 0
```

这直接覆盖了事故根因中的 `vendor-vue <-> vendor-misc` 静态循环。

## 反证与排除项

### 后端不是主因

证据：`GET /`、`GET /api/v1/auth/me` 和大量 `/v1/responses` 当时均返回 200，容器 healthy。

### CSP nonce 不是主因

现象：`curl -I` 与 `curl body` 中 nonce 不同。

解释：这是两个独立请求，每次请求生成不同 nonce 是预期行为。后端 `embed_on.go` 会把 `__CSP_NONCE_VALUE__` 替换为当前请求 nonce；`security_headers.go` 会把同一 nonce 写入 CSP。

当前补丁新增的 inline 兜底脚本也使用 `nonce="__CSP_NONCE_VALUE__"`，并由既有后端测试覆盖 nonce 替换链路。

第二审查补充确认：`replaceNoncePlaceholder` 使用 `bytes.ReplaceAll`，已有测试覆盖多个 `__CSP_NONCE_VALUE__` 同时替换；兜底脚本也没有 `onclick=` 这类 inline 事件处理器，按钮使用 `addEventListener`，因此 nonce-based CSP 下是合规的。

### Cloudflare 缓存不是主因

Cloudflare 可能影响旧 HTML/旧 asset 混用，但事故期间本机 `127.0.0.1:8080` 已经能返回坏 hash HTML，说明源站构建产物本身存在问题。

## 剩余不确定性

1. 没有事故现场浏览器 Console 截图，因此无法 100% 证明用户浏览器里的第一条错误就是上述 ReferenceError。
2. Node + jsdom smoke 能证明同 hash 入口模块存在初始化错误，不能完全替代真实浏览器 E2E。
3. 当前补丁的 HTML 兜底 UI 需要在测试环境真实浏览器中验证一次，包括 CSP、Cloudflare 和生产构建后的真实渲染行为。module/inline script 执行顺序已由第二审查确认，但仍应做浏览器 smoke 作为发布门禁。

## 建议测试环境验证步骤

构建：

```bash
pnpm --dir frontend build
```

部署到测试环境后检查 HTML：

```bash
curl -sS https://<test-domain>/ | head -100
```

预期：

1. 不再出现事故资源：

   ```text
   index-Bpdsxip0.js
   vendor-vue-LkQH8kTv.js
   vendor-misc-BHYy-3GM.js
   ```

2. 出现新构建入口，例如：

   ```text
   /assets/index-C7MJMXR9.js
   ```

浏览器验证：

1. 打开首页不白屏。
2. Console 无 `ReferenceError: Cannot access ... before initialization`。
3. 登录态场景访问 `/api/v1/auth/me` 正常。
4. 控制台 Network 中入口 JS 与 CSS 均为 200。
5. 可选：临时阻断入口 JS 或某个 chunk，页面应显示“前端加载失败”，不是纯白。

建议发布门禁补充一个真实浏览器 smoke：

1. 正常加载首页，断言 `#app` 非空且没有白屏。
2. 拦截入口 JS 返回错误，断言页面显示“前端加载失败”。
3. 对测试环境开启与生产一致的 CSP/Cloudflare 路径时重复上述检查。

## 第二审查结论

Claude 对根因和三处改动的审查结论：

1. 根因判断成立：坏 hash 与本地未修复构建产物一致，同 hash jsdom 导入复现 TDZ 错误，静态 import 图存在 `vendor-vue <-> vendor-misc` 循环。
2. 删除 `manualChunks` 是合适的根因修复，比继续微调 include 规则更稳妥；代价是失去自定义 vendor 缓存粒度，对正确性无影响。
3. nonce 链路成立：新增 inline 兜底脚本会和 `__APP_CONFIG__` 脚本一样被替换为同一个请求级 nonce，不会被 CSP 拦截。
4. 兜底脚本执行顺序成立：生产构建后 module 入口位于 `<head>`，但 module script 具备 defer 语义；body 内经典 inline 脚本会先注册 `error`、`unhandledrejection` 和自定义事件监听。
5. `main.ts` 的 `.catch` 定位准确：它不能捕获静态 import 初始化错误，只覆盖异步 bootstrap 失败；本次根因修复仍是删除错误拆包和 HTML 级兜底。
6. 唯一仍建议补强的是上线前真实浏览器 smoke，尤其验证正常首页和入口 JS 失败时的兜底 UI。

## 给 Claude 的审查问题

请重点审查以下问题：

1. 上述证据是否足以支持“手写 `manualChunks` 导致 ESM 循环初始化错误”这个根因？
2. 是否存在更强的替代解释，例如 CSP、Cloudflare 缓存、后端 HTML 注入、资源 404、浏览器兼容性？
3. 删除 `manualChunks` 是否是比“继续微调拆包规则”更合适的生产修复？
4. 新增 `index.html` 兜底脚本是否可能与 CSP、Vite HTML transform、Tailwind purge、Cloudflare 或浏览器 module 执行顺序冲突？
5. 当前验证是否还缺一个真实浏览器 smoke，是否应在发布门禁里补 Playwright 或等价检查？
