# 用分块编辑实现真 WYSIWYG

## 背景

ADR-0001 决定在 `kit.CodeEditor` 上用 `CodeDecoration` 做"源码即排版"。原型验收时用户要求真 WYSIWYG（标记消失、标题按标题字号），而 keel v0.4.11 的装饰能力到不了那一步：`ui/kit/code_editor.go` 里 `CodeDecoration` 的注释原文是 "Weight and Italic restyle the glyphs of a CodeDecorationText range. Glyphs keep the regular positions, so carets and clicks do not move."，`ui/kit`、`ui/el` 全仓没有 conceal、零宽字符或行内折叠；把标记染成背景色只会留下等宽空白，标题字还是正文大小。

## 决定

把文档拆成顶层块，块列里最多一块处于编辑态：

- 非活动块用 `ui/markdown.Doc` 渲染，标题是标题字号、标记符号不存在；
- 活动块换成 `kit.CodeEditor`，显示该块原文，并继续用装饰淡化标记；
- 点中某块即进入编辑，`Esc`、点中别的块或切文件时把该块原文拼回全文；
- 切块规则与 `ui/markdown` 的增量解析一致：围栏代码与 `$$` 之外的空行即块边界（`internal/mddoc.Blocks`）。

## 后果

- 视觉上接近 Typora：没有标记符号，标题、引用、列表按渲染样式显示。
- 撤销栈退化：`kit.CodeEditor` 的撤销只覆盖当前块，跨块的撤销顺序由用户自己记住。
- 点击落点只能到块首：keel 没有暴露点击坐标的 API，用户点一下进入该块，再点一下把光标放到具体位置。
- 保存、退出码、自动保存都与块无关：写入的始终是拼接后的全文。
- `ADR-0001` 仍然成立的范围缩小了：装饰只用于活动块的标记淡化，不再承担"编辑区排版"。

## 考虑过的其它选项

- **只遮盖标记**：字宽不变，标题不放大，算不上 WYSIWYG。
- **自研富文本编辑器**：要在 `kit.CodeEditor` 之外接管 Gio 的文本布局、光标与输入法，属于非目标。
- **给 CodeEditor 加隐藏字符能力**：要改 keel 上游，超出本项目范围。
