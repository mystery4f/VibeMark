# VibeMark

[English](README.md) | 简体中文

一个桌面 WYSIWYG Markdown 编辑器，用 Go 写，界面基于
[keel](https://github.com/dyike/keel)。目标是能当 `$EDITOR` 用：把
`git config core.editor` 指向它，提交时打开的是真编辑器而不是终端里的缓冲区。

## 渲染方式

每个顶层块都由 `ui/markdown` 渲染，所以标记默认看不见、标题是真字号。点一下某个
块，只有这个块变成它的原文；按 `Esc` 或点别的块就收回去。任何时刻最多只有一个块
以源码形态出现。

这是有意的取舍，记在
[`docs/adr/0002-block-editing-wysiwyg.md`](docs/adr/0002-block-editing-wysiwyg.md)：
keel 的装饰既不能隐藏字符也不能改字宽，所以真 WYSIWYG 只能靠分块编辑，而不是行内
改样式。

## 三种启动方式

- `vibemark <文件>` —— 单文件模式，阻塞式，给 `$EDITOR` 用。只有 `Ctrl+S` 才写盘；
  带未保存改动关窗时退出码为 `1`，git 会因此放弃这次提交。
- `vibemark --dir <目录>` —— 左边一棵树，浏览目录里的 Markdown。停止输入 300 毫秒
  后自动保存。
- `vibemark` —— 回到上次的目录；没有记录就用当前工作目录。

单文件模式退出码：`0` 已保存、`1` 用户取消、`2` 打不开或写盘失败（原因写 stderr）。

## 快捷键

| 组合 | 作用 |
| --- | --- |
| `Enter` | 列表项内续写同一个标记、另起一块，或插入空行 |
| `Shift+Enter` | 在同一块内换行 |
| `Ctrl+V` | 粘贴文字、文件或截图：图片复制进 `assets/` 并直接显示，其它文件变成链接 |
| 块首/块尾 `Backspace`、`Delete` | 与相邻块合并 |
| `Esc` | 提交当前正在编辑的块 |
| `Ctrl+S` | 保存 |
| `Ctrl+P` | 全屏预览开关 |
| `Ctrl+/` | 整篇切到源码模式 |
| `Ctrl+Shift+L` | 文件树开关 |
| `Ctrl+B` / `Ctrl+I` | 选区加粗 / 斜体 |
| ``Ctrl+Shift+` `` | 选区行内代码 |
| `Alt+Shift+5` | 选区删除线 |
| `Ctrl+K` | 选区包成链接 |
| `Ctrl+1` ... `Ctrl+6` | 一级 ... 六级标题 |
| `Ctrl+0` | 变回普通段落 |
| `Ctrl+Shift+K` | 围栏代码块开关 |
| `Ctrl+Shift+Q` | 引用 |
| `Ctrl+Shift+[` / `Ctrl+Shift+]` | 有序 / 无序列表 |

键位沿用 Typora 默认，肌肉记忆可以直接带过来。

## 图片

粘贴或引用的图片存在文档旁边的 `assets/` 目录里，笔记和图片一起搬家：

```markdown
![shot.png](assets/shot.png)
```

渲染器只读本地文件；`https://` 这类远程地址会被忽略而不是去下载，打开笔记不会碰网络。

## 当作 git 编辑器

```sh
go build -o vibemark.exe .        # 或者 keel build
# 把 vibemark 放进 PATH，然后：
git config --global core.editor vibemark
```

## 开发

```sh
go build ./...
go test ./...       # 界面测试走 keel 的 windowtest，不开真窗口
gofmt -l .          # .githooks/pre-commit 会连同 vet 和 test 一起检查
```

`windowtest` 只合成按键事件，碰不到真实键盘和剪贴板，所以依赖系统的那部分用 keel
自带的自动化服务器 `keel-mcp` 验证（`go install github.com/dyike/keel/cmd/keel-mcp@v0.4.11`，
配置在 [`.pi/mcp.json`](.pi/mcp.json)）：

```sh
go build -o .tmp/vibemark.exe .
# 然后通过 MCP 工具驱动真实窗口：
#   launch    {"command": "…/.tmp/vibemark.exe --dir .tmp/demo"}
#   click     {"text": "正文"}
#   press_key {"key": "backspace"}
#   press_key {"key": "ctrl+v"}
```

默认离屏运行，`visible: true` 则显示在屏幕上。跨块删除、剪贴板粘贴、自动保存都靠它
端到端验过。

每个克隆启用一次钩子：`git config core.hooksPath .githooks`。

设计记录在 [`docs/design/vibemark-mvp.md`](docs/design/vibemark-mvp.md)、
[`GLOSSARY.md`](GLOSSARY.md) 与 [`docs/adr/`](docs/adr/)。

## 已知限制

- 正在编辑的那一段会显示原文，并带行号；其余部分保持排版后的样子。
- 正在编辑的那一段长过窗口高度后，会在自己的框里滚动。
- 撤销只覆盖正在编辑的那一段。
- 点一下打开某一段，再点一次才把光标放到想放的位置。
- 图片只能通过剪贴板进来：没有文件对话框，也不支持拖入。
- 全选要按两次：第一次选当前段，第二次选中整篇，接着打字就替换整篇。
