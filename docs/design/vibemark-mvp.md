# VibeMark MVP 设计稿（待确认）

状态：草案 v2（2026-11-19 按三项决策收敛——标记只淡化、单栏不放大标题、定位轻快不折腾）。你签收后再写代码。

## 目标

一个桌面 markdown 编辑器，编辑区本身按 markdown 语法呈现样式，不依赖"源码 + 预览"两栏对照。定位是**轻、快、不折腾**：单文件、无数据库、无插件体系，拿一个 md 文件就能写。它不是 Typora 那种真富文本排版，是"半实时"：标记可见、字号不变。
- 能被 git 和任意命令行工具当编辑器调起：`git config --global core.editor vibemark`，或 `EDITOR=vibemark`；接一个文件路径就进入阻塞编辑，退出即回传结果。

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

- 粗体 / 斜体 → `Weight` / `Italic`；删除线**不渲染**：`CodeDecoration` 没有 strikethrough，拿下划线冒充删除线观感是错的
- 行内代码、链接、引用文本 → 前景色
- 语法标记符（`**`、`#`、反引号、`[]()`）→ 低对比前景色（约 40% 不透明度）的"淡化"，标记始终可见。**不做背景色遮盖，也不做"光标行才显示标记"**：装饰不改字宽、不删字符，遮盖只会在行内留空洞，光标穿过就露馅
- 标题 → 加粗 + 主题强调色，**字号不变**；接受单栏，不做左右分栏预览（真字号只出现在全屏预览里）

每次 `OnChange` 重建装饰，不做增量同步：删除、搬移、撤销都由 AST 重新给出真相，比猜区间便宜。大文件按阈值降级为只染色不装饰。

这条路子的价值在于：光标、选区、输入法、多光标、撤销、查找全部由 `CodeEditor` 承担，我们不写文本布局引擎。代价是它不是 Typora 那种真正的富文本排版，是"源码即排版"。

## MVP 范围

- 单文件模式：`vibemark <file>` 阻塞编辑，用于 `core.editor` / `$EDITOR`；保存后退进程退出码 0，用户显式取消则退出码 1（git 据此放弃本次提交）
- 退出码语义：正常退出 = 文件已落盘；`1` = 用户取消；`2` = 打不开或写盘失败，错误写 stderr
- vault 目录：路径来自命令行参数或上次会话，首次启动扫当前目录
- 左侧文件树（`kit.AppShell` + `kit.Sidebar` + `kit.Tree`），右侧编辑区
- 单文件编辑、未保存标记、自动保存（防抖约 300ms 后再临时文件 + rename 原子写）
- 预览：全屏切换（`ui/markdown` 渲染当前文件），快捷键进出，不与编辑区同屏
- 快捷键：加粗 / 斜体 / 行内代码 / 链接包裹选区，标题级别，列表与引用前缀；查找交给 `CodeEditor` 内置面板
- 主题：keel 明暗两套 + 强调色配置
- 启动恢复：`window.Options.KeepState` 记住上次文件与滚动位置

## 非目标（本版不做）

- 图片粘贴入库、表格可视化编辑、脚注 UI、插件系统、多窗口、云同步、移动端
- 自研富文本编辑引擎：要接管 Gio 的文本布局与光标，成本远高于 MVP
- 原生文件对话框：改用目录型 vault，避开 keel 空缺
- 删除线渲染、左右分栏预览、光标行才显示标记：前者 keel 没有能力，后两者与"单栏、轻快"的定位冲突

## 代码结构

- `main.go` / `app.go`：keel 脚手架原样，只改 `app.go`
- `internal/mddoc`：goldmark 解析、区间到装饰的映射、markdown 语义编辑命令（对选区做加粗/标题等）
- `internal/vault`：目录扫描、读写、原子保存、最近文件；单文件模式复用同一读写层，不建 vault、不写索引
- `internal/ui`：视图组合（侧栏、编辑区、预览、状态栏）
- 依赖单向：`ui` → `vault` / `mddoc`；`mddoc` 不依赖 UI，可纯单测

## 风险与验证

- 编辑后装饰区间漂移 → 每帧从 AST 重建，不做增量同步；5000 行文档设帧耗时门槛，超了降级为只染色
- 装饰与 Chroma 色彩叠加：`Text` 装饰在染色之后绘制，只设颜色不设字重时仍会盖掉语法色 → 只对标记符号用淡化色，正文样式（粗体/斜体）不设 `Color`；用 5000 行文档量帧耗时
- keel 处于 0.4.x 快速迭代期 → 固定 v0.4.11，升级单独评估，不跟随
- keel 挡不住关窗：`OnClose` 是窗口销毁后的通知，不能拦截；默认退出固定 `os.Exit(0)`，所以"取消"必须自己 `os.Exit(1)`。未保存风险靠自动保存消除，不靠提示框
- `core.editor` 的可用性前提是 `vibemark.exe` 在 PATH 上；git 调起时工作目录是仓库根，不能依赖当前目录找 vault
- 测试用 keel 的 `windowtest` 无窗口跑 `go test ./...`；交互效果靠 `keel run` 手测

## 落地顺序

- `keel new` 脚手架（要求目录为空 → 先生成到 `.tmp/scaffold` 再挪进来，保留 `.git` 与 `.editorconfig`），装 keel CLI：`go install github.com/dyike/keel/cmd/keel@v0.4.11`
- 打通"打开 vault → 编辑 → 实时装饰 → 保存 → 预览"这一条竖线，先不做文件树以外的 UI
- 单文件阻塞模式 + 退出码，让 `git commit` 真能调起来——这是最短的一条可验证闭环
- 再补文件树、快捷键、主题、启动恢复
