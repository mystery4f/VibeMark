# VibeMark MVP 设计稿

状态：v3（2026-11-19 签收）。两轮 grilling 已定下保存策略、装饰细则、启动行为、测试门槛与术语，可以开工。术语见根目录 `GLOSSARY.md`，架构决策见 `docs/adr/0001-decoration-based-wysiwyg.md`。

## 目标

一个桌面 markdown 编辑器，编辑区本身按 markdown 语法呈现样式，不依赖"源码 + 预览"两栏对照。定位是**轻、快、不折腾**：单文件、无数据库、无插件体系，拿一个 md 文件就能写。呈现方式是整篇一个编辑器加装饰：标记淡化但不隐藏，标题加粗加强调色，光标、撤销、输入法都交给 keel 的编辑器。
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

## 方案：整篇一个编辑器 + 装饰（2026-11-19 再修订，取代"分块编辑"）

整篇文档放在一个 `kit.CodeEditor` 里，编辑器用 `Fill()` 占满窗口，排版效果全部由 `CodeDecoration` 表达：

- `internal/mddoc.Decorations(src)` 用 goldmark 解析整篇，把标题、强调、行内代码、链接、列表与引用标记转成装饰；每次 `OnChange` 重建一次，源码哈希未变则短路
- 标题→正文加粗 + 强调色、`# ` 前缀淡化；粗体/斜体→`Weight`/`Italic`（不设 Color，保留语法高亮色）；标记符号用约 40% 不透明度淡化，**始终可见不隐藏**
- 光标、选区、撤销、输入法、多光标、Enter/Shift+Enter/Backspace/Delete 全交给 keel 编辑器；窗口只注册改写整篇的命令（加粗、标题、代码围栏、粘贴、保存、预览、侧栏）
- 全屏预览用整篇 `ui/markdown` 渲染，进入/退出只切视图

为什么退回来：分块编辑（非活动块渲染、活动块显原文）能做出真 WYSIWYG 的观感，但 keel 的编辑器只认自己的整篇缓冲区，每个文本编辑行为都要在下游模拟——零宽守卫字符、rune 钳位、`SetValue` 丢撤销栈——真实使用里连续暴露跨块退格失效、空段落退格无反应、空段落全选无效、守卫字符被顶进正文等缺陷。取舍记录见 `docs/adr/0003-one-editor-with-decorations.md`（ADR-0002 已标记 superseded）。

装饰的边界由 keel 定死：`ui/kit/code_editor.go` 的 `CodeDecoration` 注释写明 "Glyphs keep the regular positions, so carets and clicks do not move."，全仓没有 conceal/零宽/行内折叠，所以**装饰不能隐藏字符、不能改字宽**，标题字号因此无法放大。

性能（i5-14600K，`go test -bench . -benchtime 200x`）：整篇 20000 行一帧 0.097ms，2000 行 0.101ms，单次键入 0.632ms，预览一帧 0.407ms；`Decorations(500 段)` 1.93ms，是每次改动都要付的一次性代价。

装饰细则：

- 粗体 / 斜体 → `Weight` / `Italic`；删除线**不渲染**：`CodeDecoration` 没有 strikethrough
- 代码块内部不做 markdown 装饰，只把 ``` 围栏行淡化
- 链接只染 `[]()` 与 URL，链接文字保持原色
- 表格不补空格对齐，只淡化 `|` 与分隔行
- 任务列表 `- [x]` 用低对比色表示已完成

已知代价：改写整篇的命令（加粗、标题、代码围栏）会让撤销历史从头开始；行号栏与滚动条由 keel 写死，关不掉。

## MVP 范围

- 单文件模式：`vibemark <file>` 阻塞编辑，用于 `core.editor` / `$EDITOR`；保存后退进程退出码 0，用户显式取消则退出码 1（git 据此放弃本次提交）
- 退出码语义：正常退出 = 文件已落盘；`1` = 用户取消；`2` = 打不开或写盘失败，错误写 stderr
- 工作目录：`--dir <dir>` 指定，否则用上次会话记录的目录，再否则用当前工作目录；keel 没有系统文件选择器，空态里给一个路径输入框兜底
- 左侧文件树（`kit.AppShell` + `kit.Sidebar` + `kit.Tree`），右侧整篇编辑区（`kit.CodeEditor` + `Fill()`）
- 单文件模式不显示文件树与侧栏：只有编辑区 + 底部状态栏（文件名、是否有未保存改动）；路径不存在则创建空文件，父目录不存在或写盘失败则退 2 并把原因写 stderr
- 保存策略按模式分开：工作目录模式 300ms 防抖自动保存（临时文件 + rename 原子写），因此不显示"未保存"标记；单文件模式不自动保存，只认 Ctrl+S
- 预览：全屏切换（`ui/markdown` 渲染当前文件），快捷键进出，不与编辑区同屏
- 快捷键：加粗 / 斜体 / 行内代码 / 链接包裹选区，标题级别，列表与引用前缀；查找交给 `CodeEditor` 内置面板
- 主题：keel 明暗两套 + 强调色配置
- 启动恢复：`window.Options.KeepState` 记住上次文件与滚动位置

## 非目标（本版不做）

- 表格可视化编辑、脚注 UI、插件系统、多窗口、云同步、移动端
- 自研富文本编辑引擎：要接管 Gio 的文本布局与光标，成本远高于 MVP
- 原生文件对话框：改用目录型工作目录，避开 keel 空缺
- 删除线渲染、左右分栏预览、光标行才显示标记：前者 keel 没有能力，后两者与"单栏、轻快"的定位冲突

## 代码结构

- `main.go` / `app.go`：keel 脚手架原样，只改 `app.go`
- `internal/mddoc`：goldmark 解析、区间到装饰的映射、markdown 语义编辑命令（对选区做加粗/标题等）、rune 与字节位置换算
- `internal/media`：粘贴图片落到系统临时目录（`VIBEMARK_MEDIA` 可覆盖），返回绝对路径
- `internal/workspace`：目录扫描、读写、原子保存、最近文件；单文件模式复用同一读写层，不建 workspace、不写索引
- `app.go`：视图组合（侧栏、编辑区、预览、状态栏），不再有独立 `internal/ui` 包
- 依赖单向：`app.go` → `workspace` / `mddoc` / `media`；`mddoc` 不依赖 UI，可纯单测

## 图片

- 粘贴的图片写入系统临时目录下的 `vibemark`（`VIBEMARK_MEDIA` 可覆盖），markdown 里写绝对路径；文档旁不再生成 `assets/`，笔记目录不被图片污染。代价是临时目录可能被系统清理，需要长期保留时把图片挪到笔记自己的目录
- 读取顺序：绝对路径直接用；相对路径先找文档所在目录，再找工作目录，再找启动编辑器的目录；`file://`、`<尖括号>`、`%20` 转义都识别
- 远程 `https://` 与 `data:` 一律不读，打开笔记不碰网络；读不出来时状态栏指名失败的图片
- 已知 keel 行为：图片在后台加载，加载完成才重绘，首帧显示占位框属于正常

## 风险与验证

- 编辑后装饰区间漂移 → 每次改动从 AST 重建，不做增量同步；帧耗时阈值等竖线跑通后拿真实文档实测再定，降级分支先不写
- 装饰与 Chroma 色彩叠加：`Text` 装饰在染色之后绘制，只设颜色不设字重时仍会盖掉语法色 → 只对标记符号用淡化色，正文样式（粗体/斜体）不设 `Color`；用真实长文档量帧耗时再定阈值
- keel 处于 0.4.x 快速迭代期 → 固定 v0.4.11，升级单独评估，不跟随
- keel 挡不住关窗：`OnClose` 是窗口销毁后的通知，不能拦截；默认退出固定 `os.Exit(0)`，所以"取消"必须自己 `os.Exit(1)`。未保存风险靠自动保存消除，不靠提示框
- `core.editor` 的可用性前提是 `vibemark.exe` 在 PATH 上；git 调起时工作目录是仓库根，不能靠当前目录推断工作目录
- 测试用 keel 的 `windowtest` 无窗口跑 `go test ./...`；交互效果靠 `keel run` 手测

## 落地顺序

- `keel new` 脚手架（要求目录为空 → 先生成到 `.tmp/scaffold` 再挪进来，保留 `.git` 与 `.editorconfig`），装 keel CLI：`go install github.com/dyike/keel/cmd/keel@v0.4.11`
- 打通"打开工作目录 → 编辑 → 实时装饰 → 保存 → 预览"这一条竖线，先不做文件树以外的 UI
- 单文件阻塞模式 + 退出码，让 `git commit` 真能调起来——这是最短的一条可验证闭环
- 再补文件树、快捷键、主题、启动恢复

## 已定决策（2026-11-19 第二轮签收）
- 验收边界：范围与"非目标"整体签收，编码只做到竖线跑通（打开工作目录 → 编辑 → 装饰 → 保存 → 预览）就停下验收，文件树/快捷键/主题/启动恢复验收后再补
- 保存策略按模式分开（见 MVP 范围）：工作目录自动保存、单文件只认 Ctrl+S，关窗有未保存改动退 1
- 装饰细则四条（见方案）：代码块内部不装饰、链接文字不染色、表格不补空格、任务列表用低对比色
- 帧耗时阈值实测再定，先不写降级分支
- 测试门槛：`internal/mddoc` 覆盖率 ≥ 80%，`app.go` 的 UI 部分不设覆盖率数字
- markdown 语义命令：跨行选区逐行独立包裹（每行一对 `**`），不产出跨行强调块
- 术语定名见 `GLOSSARY.md`，`vault` 一词废弃改称"工作目录"；架构决策见 `docs/adr/0001-decoration-based-wysiwyg.md`（固定 keel v0.4.11 与单栏不放大两条只记在本稿，不占 ADR）

## 与真 WYSIWYG 的差距（2026-11-19 记录，最终定案）

标记（`# `、`**`、`` ` ``）虽然淡化，仍然占字宽、仍然可见；标题在编辑区只加粗加强调色，不是标题字号。排查后确认 keel 的 `CodeDecoration` 不能隐藏字符、不能改字宽，分块编辑虽然是能达成的路，但模拟层带来的编辑缺陷不可控（见 `docs/adr/0002-block-editing-wysiwyg.md` 与 `docs/adr/0003-one-editor-with-decorations.md`），最终选择整篇单编辑器 + 装饰。

其余视觉缺口都在 keel 写死的部分：行号栏宽度（`ui/kit/code_editor_paint.go:125`）与滚动条无法关闭。
