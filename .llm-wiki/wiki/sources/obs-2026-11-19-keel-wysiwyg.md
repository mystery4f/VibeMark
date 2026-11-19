---
type: source
title: "Observation: keel 装饰不能隐藏字符，真 WYSIWYG 只能靠分块编辑"
tags:
  - keel
  - wysiwyg
  - editor
  - adr
status: observation
created: 2026-11-19
updated: 2026-11-19
slug: obs-2026-11-19-keel-wysiwyg
relevance: high
observed_at: 2026-11-19T04:13:55.285Z
source_context: VibeMark 分块编辑 WYSIWYG 实现
---

# ⭐ Observation: keel 装饰不能隐藏字符，真 WYSIWYG 只能靠分块编辑

keel v0.4.11 的 kit.CodeDecoration 只能改 Color/Weight/Italic/Underline，且注释明说 "Glyphs keep the regular positions, so carets and clicks do not move" —— 装饰既不能隐藏字符也不能改字宽，全仓 ui/kit、ui/el 都没有 conceal 或零宽折叠（Hidden(true) 是整块 div 隐藏，Fold 是整行折叠）。因此"源码即排版"的行内补丁式 WYSIWYG 无法做到真 WYSIWYG：标记删不掉、标题放不大。可行路径是按顶层块用 ui/markdown 渲染、把光标所在块换成 kit.CodeEditor 的原文（分块编辑）。代价：撤销只覆盖活动块、点击只落块首、活动块会露出 keel 写死的行号 gutter（code_editor_paint.go:125 `m.gutter = m.foldW + len(...)*digit + gtx.Dp(14)`，无开关）。VibeMark 在 docs/adr/0001 与 0002 里记录了这一反转。参考源码在 .tmp/keel-repo（tag v0.4.11）。

*Relevance: high*
*Context: VibeMark 分块编辑 WYSIWYG 实现*
*Tags: keel wysiwyg editor adr*

---
*Observed: 2026-11-19T04:13:55.285Z*
