# VibeMark MVP 设计稿（待确认）

状态：草案，等你确认后再写代码。本文只定边界和取舍，不含实现细节。

## 目标

一个桌面 markdown 笔记应用，编辑区本身按 markdown 语法呈现样式（所见即所得），不依赖"源码 + 预览"两栏对照。

## 技术底座（已核实）

- Go 1.26.5，`GOPROXY=https://goproxy.cn,direct`
- UI 框架 keel 固定 **v0.4.11**（2026-11-17 发布，最新）。纯 Go 绘制，内置打过补丁的 Gio，不用 HTML/CSS/JS/WebView
- 模块路径 `github.com/mystery4f/vibemark`，应用 ID `com.mystery4f.vibemark`
- 脚手架 `keel new` 生成 `main.go`、`app.go`、`app_test.go`、`keel.json`、`appicon.png`、`README.md`、`.gitignore`
- keel 的 `native` 包只有权限、截图、合成键鼠、全局热键、通知、剪贴板、托盘，**没有原生文件对话框**

## 三个决定设计的硬约束

- `ui/markdown` 只读：`Doc` 提供 `Append` / `SetSource` / 选区复制 / 区间高亮 / 滚动定位，没有编辑能力。
- 可编辑控件只有两个：`el.TextArea`（单一样式的纯文本）和 `kit.CodeEditor`（代码编辑器：语法高亮、多光标、列选择、查找替换、折叠、软换行、撤销重做、输入法、装饰集合）。
- `kit.CodeEditor` 的 `CodeDecoration` 支持 `Text` 样式，可设 `Color`、`Underline`、`Weight`（如 `font.Bold`）、`Italic`，且"只设字重或斜体、不设颜色时保留语法高亮的颜色"；装饰随编辑自动平移/收敛区间。字宽按常规字宽定位，所以**编辑区内不能放大字号**。

## 方案：装饰式实时样式编辑

编辑区用 `kit.CodeEditor`，`Language("markdown")` 交给 Chroma 染色；同时用 goldmark 解析当前源码，把 AST 里的样式区间翻成一批 `CodeDecoration`：

- 粗体 / 斜体 / 删除线 → `Weight` / `Italic` / `Underline`
- 行内代码、链接、引用文本 → 前景色
- 语法标记符（`**`、`#`、反引号、`[]()`）→ 低对比色，作为"标记淡化"；隐藏标记留开关，默认淡化
- 标题 → 加粗 + 主题强调色（真实字号只在预览里体现）

每次 `OnChange` 重建装饰，不做增量同步：删除、搬移、撤销都由 AST 重新给出真相，比猜区间便宜。大文件按阈值降级为只染色不装饰。

这条路子的价值在于：光标、选区、输入法、多光标、撤销、查找全部由 `CodeEditor` 承担，我们不写文本布局引擎。代价是它不是 Typora 那种真正的富文本排版，是"源码即排版"。

## MVP 范围

- vault 目录：路径来自命令行参数或上次会话，首次启动扫当前目录
- 左侧文件树（`kit.AppShell` + `kit.Sidebar` + `kit.Tree`），右侧编辑区
- 单文件编辑、未保存标记、自动保存（临时文件 + rename 原子写）
- 预览切换：`ui/markdown` 渲染当前文件，快捷键切换，与编辑区同位置替换
- 快捷键：加粗 / 斜体 / 行内代码 / 链接包裹选区，标题级别，列表与引用前缀；查找交给 `CodeEditor` 内置面板
- 主题：keel 明暗两套 + 强调色配置
- 启动恢复：`window.Options.KeepState` 记住上次文件与滚动位置

## 非目标（本版不做）

- 图片粘贴入库、表格可视化编辑、脚注 UI、插件系统、多窗口、云同步、移动端
- 自研富文本编辑引擎：要接管 Gio 的文本布局与光标，成本远高于 MVP
- 原生文件对话框：改用目录型 vault，避开 keel 空缺

## 代码结构

- `main.go` / `app.go`：keel 脚手架原样，只改 `app.go`
- `internal/mddoc`：goldmark 解析、区间到装饰的映射、markdown 语义编辑命令（对选区做加粗/标题等）
- `internal/vault`：目录扫描、读写、原子保存、最近文件
- `internal/ui`：视图组合（侧栏、编辑区、预览、状态栏）
- 依赖单向：`ui` → `vault` / `mddoc`；`mddoc` 不依赖 UI，可纯单测

## 风险与验证

- 装饰与 Chroma 色彩的叠加顺序、编辑后的区间漂移 → 每帧从 AST 重建；用一份 5000 行文档量帧耗时
- 标记"隐藏"用背景色只是视觉近似，会留出空位 → 默认只淡化，隐藏做成开关，实测后再定默认值
- keel 处于 0.4.x 快速迭代期 → 固定 v0.4.11，升级单独评估，不跟随
- 测试用 keel 的 `windowtest` 无窗口跑 `go test ./...`；交互效果靠 `keel run` 手测

## 落地顺序

- `keel new` 脚手架（要求目录为空 → 先生成到 `.tmp/scaffold` 再挪进来，保留 `.git` 与 `.editorconfig`），装 keel CLI：`go install github.com/dyike/keel/cmd/keel@v0.4.11`
- 打通"打开 vault → 编辑 → 实时装饰 → 保存 → 预览"这一条竖线，先不做文件树以外的 UI
- 再补文件树、快捷键、主题、启动恢复
