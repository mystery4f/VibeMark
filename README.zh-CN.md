# VibeMark

[English](README.md) | 简体中文

一个桌面 WYSIWYG Markdown 编辑器，用 Go 写，界面基于
[keel](https://github.com/dyike/keel)。目标是能当 `$EDITOR` 用：把
`git config core.editor` 指向它，提交时打开的是真编辑器而不是终端里的缓冲区。

## 渲染方式

整篇文档就是一个文本编辑器，`internal/mddoc` 把 Markdown 解析成装饰：标题加粗并用
强调色，粗体、斜体按字重字形显示，标记符号淡化到 40% 不透明度而不是消失。按
`Ctrl+P` 看完整排版后的样子。

取舍记在
[`docs/adr/0003-one-editor-with-decorations.md`](docs/adr/0003-one-editor-with-decorations.md)：
keel 的装饰既不能隐藏字符也不能改字宽，所以标题只能是正文字号、标记始终可见。保住整篇
一个编辑器，是为了让输入、撤销、选区、输入法都走控件原生实现——这比字形完全一致更重要。

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
| `Ctrl+V` | 粘贴文字、文件或截图：图片复制进临时目录并直接显示，其它文件变成链接 |
| `Ctrl+S` | 保存 |
| `Ctrl+P` | 全屏预览开关 |
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

键位沿用 Typora 默认，肌肉记忆可以直接带过来。输入、`Enter`、`Shift+Enter`、`Backspace`、`Delete`、`Ctrl+A`、`Ctrl+Z` 都是编辑器自己的，跟任何文本框一样。

## 图片

粘贴的图片写到 `%TEMP%\vibemark`（可用 `VIBEMARK_MEDIA` 改），不会落在笔记旁边，链接是绝对路径：

```markdown
![shot.png](C:/Users/you/AppData/Local/Temp/vibemark/shot.png)
```

那个目录是系统临时目录，Windows 可能清理它：要长期保留的图片，复制到笔记自己的目录里。

其余图片按写法从磁盘读取：相对路径先找文档所在目录，再找工作目录，再找启动编辑器时的目录；`file://` 地址、`<尖括号>`、`%20` 转义都能识别。渲染器只读本地文件；`https://` 这类远程地址会被跳过而不是去下载，打开笔记不会碰网络。读不出来的图片会在状态栏指名。

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

- 标记只淡化、不消失，标题是正文字号：keel 的装饰不能隐藏字符、不能改字宽。
- 行号栏与滚动条属于编辑器控件本身，关不掉。
- 加粗、标题、代码围栏这类改写整篇的命令会让撤销历史从头开始，因为控件没有写缓冲区的接口。
- 图片只能通过剪贴板进来：没有文件对话框，也不支持拖入。
