// <code-editor> is a reusable CodeMirror 6 WebComponent.
import { EditorState } from "@codemirror/state"
import {
  EditorView, keymap, highlightActiveLine, lineNumbers, highlightActiveLineGutter
} from "@codemirror/view"
import { html } from "@codemirror/lang-html"
import { css } from "@codemirror/lang-css"
import { javascript } from "@codemirror/lang-javascript"
import { markdown } from "@codemirror/lang-markdown"
import {
  defaultKeymap, history, historyKeymap, indentWithTab
} from "@codemirror/commands"
import { defaultHighlightStyle, syntaxHighlighting, bracketMatching } from "@codemirror/language"
import { search, searchKeymap, highlightSelectionMatches } from "@codemirror/search"
import { editorExtensions } from "./editor-theme.js"

const LANGS = {
  html, css, javascript, markdown
}

const MARKDOWN_TOOLS = [
  { cmd: "heading", key: "editor.heading", label: "Heading", text: "H" },
  { cmd: "bold", key: "editor.bold", label: "Bold", text: "B" },
  { cmd: "italic", key: "editor.italic", label: "Italic", text: "I" },
  { cmd: "link", key: "globals.terms.link", label: "Link", text: "Link" },
  { cmd: "bullet", key: "editor.bulletList", label: "Bullet list", text: "List" },
  { cmd: "quote", key: "editor.blockquote", label: "Quote", text: "Quote" }
]

function translate(key, fallback) {
  const value = window._LM_I18N?.[key]
  return value ? String(value).split("|")[0].trim() : fallback
}

function replaceSelection(view, insert, anchor, head = anchor) {
  const { from, to } = view.state.selection.main
  view.dispatch({
    changes: { from, to, insert },
    selection: { anchor: from + anchor, head: from + head },
    scrollIntoView: true
  })
  view.focus()
}

function wrapSelection(view, before, after, placeholder) {
  const { from, to } = view.state.selection.main
  const selected = view.state.sliceDoc(from, to)
  const text = selected || placeholder
  const insert = `${before}${text}${after}`
  replaceSelection(view, insert, before.length, before.length + text.length)
}

function prefixLines(view, prefix) {
  const { state } = view
  const sel = state.selection.main
  const first = state.doc.lineAt(sel.from)
  let end = sel.to
  if (end > sel.from && state.doc.lineAt(end).from === end) {
    end -= 1
  }
  const last = state.doc.lineAt(end)
  const changes = []
  for (let n = first.number; n <= last.number; n += 1) {
    changes.push({ from: state.doc.line(n).from, insert: prefix })
  }
  const count = last.number - first.number + 1
  view.dispatch({
    changes,
    selection: {
      anchor: sel.from + prefix.length,
      head: sel.to + (prefix.length * count)
    },
    scrollIntoView: true
  })
  view.focus()
}

function insertHeading(view) {
  const { state } = view
  const sel = state.selection.main
  const line = state.doc.lineAt(sel.head)
  const indent = /^\s*/.exec(line.text)[0]
  const rest = line.text.slice(indent.length)
  const existing = /^(#{1,6})\s+/.exec(rest)
  const level = existing ? existing[1].length : 0
  const next = level >= 6 ? 0 : level + 1
  const marker = next === 0 ? "" : `${"#".repeat(next)} `
  const at = line.from + indent.length
  const to = at + (existing ? existing[0].length : 0)

  view.dispatch({
    changes: { from: at, to, insert: marker },
    selection: { anchor: sel.head + marker.length - (to - at) },
    scrollIntoView: true
  })
  view.focus()
}

function insertLink(view) {
  const { from, to } = view.state.selection.main
  const selected = view.state.sliceDoc(from, to)
  const text = selected || "text"
  const insert = `[${text}](url)`
  const urlStart = text.length + 3
  const anchor = selected ? urlStart : 1
  const head = selected ? urlStart + 3 : 1 + text.length
  replaceSelection(view, insert, anchor, head)
}

const MARKDOWN_ACTIONS = {
  heading: insertHeading,
  bold: (view) => wrapSelection(view, "**", "**", "bold"),
  italic: (view) => wrapSelection(view, "*", "*", "italic"),
  link: insertLink,
  bullet: (view) => prefixLines(view, "- "),
  quote: (view) => prefixLines(view, "> ")
}

function buildMarkdownToolbar(disabled) {
  const toolbar = document.createElement("div")
  toolbar.className = "md-toolbar"
  toolbar.setAttribute("role", "toolbar")
  toolbar.setAttribute("aria-label", translate("editor.toolbar", "Markdown formatting"))

  MARKDOWN_TOOLS.forEach((tool) => {
    const label = translate(tool.key, tool.label)
    const button = document.createElement("button")
    button.type = "button"
    button.className = "ghost small md-tool"
    button.dataset.cmd = tool.cmd
    button.textContent = tool.text
    button.title = label
    button.setAttribute("aria-label", label)
    button.disabled = disabled
    toolbar.append(button)
  })

  toolbar.addEventListener("mousedown", (e) => {
    if (e.target.closest("button")) {
      e.preventDefault()
    }
  })

  return toolbar
}

class CodeEditor extends HTMLElement {
  constructor() {
    super()
    this.editor = null
    this.textarea = null
    this.internalUpdate = false
  }

  connectedCallback() {
    // Handle if `.value` was assigned before the element was upgraded.
    this._upgradeProperty("value")

    this.textarea = this.querySelector("textarea")
    const initial = this.textarea
      ? this.textarea.value
      : (this.pendingValue ?? this.getAttribute("value") ?? "")
    if (this.textarea) {
      this.textarea.hidden = true
    }

    const mount = document.createElement("div")
    mount.className = "code-editor-mount"
    this.appendChild(mount)

    // Apply styles to the <code-editor> tag.
    this.classList.add("code-editor")

    const lang = this.getAttribute("lang")
    const isMarkdown = lang === "markdown"
    const langFn = LANGS[lang] || html
    const disabled = this.hasAttribute("disabled")

    if (isMarkdown) {
      this.classList.add("is-markdown")
      const toolbar = buildMarkdownToolbar(disabled)
      this.insertBefore(toolbar, mount)
      toolbar.addEventListener("click", (e) => {
        const button = e.target.closest("button[data-cmd]")
        if (!button || !this.editor) {
          return
        }
        MARKDOWN_ACTIONS[button.dataset.cmd]?.(this.editor)
      })
    }

    const onUpdate = EditorView.updateListener.of((update) => {
      if (!update.docChanged) {
        return
      }
      this.internalUpdate = true
      const val = update.state.doc.toString()
      if (this.textarea) {
        this.textarea.value = val
      }
      this.dispatchEvent(new Event("input", { bubbles: true }))
      this.internalUpdate = false
    })

    this.editor = new EditorView({
      parent: mount,
      state: EditorState.create({
        doc: initial,
        extensions: [
          langFn(),
          history(),
          highlightActiveLine(),
          bracketMatching(),
          highlightSelectionMatches(),
          ...(isMarkdown ? [] : [ lineNumbers(), highlightActiveLineGutter() ]),
          keymap.of([ ...defaultKeymap, ...historyKeymap, ...searchKeymap, indentWithTab ]),

          // Readonly?
          EditorState.readOnly.of(disabled),
          EditorView.editable.of(!disabled),

          // Syntax highlighting and theme.
          syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
          EditorView.lineWrapping,
          editorExtensions,

          search({ top: true }),

          onUpdate
        ]
      })
    })

    queueMicrotask(() => {
      this.dispatchEvent(new CustomEvent("editor-ready", { bubbles: true }))
    })
  }

  disconnectedCallback() {
    if (this.editor) {
      this.editor.destroy()
      this.editor = null
    }
  }

  get value() {
    return this.editor
      ? this.editor.state.doc.toString()
      : (this.pendingValue ?? this.getAttribute("value") ?? "")
  }

  set value(val) {
    const v = val == null ? "" : String(val)

    if (!this.editor) {
      this.pendingValue = v
      return
    }

    // Ignore and internal updates.
    if (this.internalUpdate || v === this.editor.state.doc.toString()) {
      return
    }

    this.editor.dispatch({
      changes: { from: 0, to: this.editor.state.doc.length, insert: v }
    })
  }

  // Reinstall a property that was set on the instance before the element was upgraded.
  _upgradeProperty(prop) {
    if (Object.prototype.hasOwnProperty.call(this, prop)) {
      const v = this[prop]
      delete this[prop]
      this[prop] = v
    }
  }
}

if (!customElements.get("code-editor")) {
  customElements.define("code-editor", CodeEditor)
}
