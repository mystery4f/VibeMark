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
| `Ctrl+S` | 保存 |
| `Esc` | 提交当前正在编辑的块 |
| `Ctrl+P` | 全屏预览开关 |
| `Ctrl+N` | 在工作目录里新建 `untitled.md`（仅工作目录模式） |
| `Ctrl+B` / `Ctrl+I` / `Ctrl+E` | 选区加粗 / 斜体 / 行内代码 |
| `Ctrl+K` | 选区包成链接 |
| `Ctrl+1` / `Ctrl+2` / `Ctrl+3` | 一级 / 二级 / 三级标题 |
| `Ctrl+Shift+8` / `Ctrl+Shift+9` / `Ctrl+Shift+\` | 无序列表 / 有序列表 / 引用 |

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

每个克隆启用一次钩子：`git config core.hooksPath .githooks`。

设计记录在 [`docs/design/vibemark-mvp.md`](docs/design/vibemark-mvp.md)、
[`GLOSSARY.md`](GLOSSARY.md) 与 [`docs/adr/`](docs/adr/)。

## 已知限制

- 正在编辑的块会露出 keel 的行号栏和代码底色。这个栏是 keel 写死的，没有开关；
  换成普通文本框又会丢掉包裹类命令需要的光标与选区接口。
- 撤销只覆盖当前正在编辑的块。
- 点一下是选中一个块，不是把光标定位到某个字；要在块里定位请再点一次。
- 没有系统文件对话框、不支持文件拖入、不显示图片：keel v0.4.11 在桌面端都没有。
