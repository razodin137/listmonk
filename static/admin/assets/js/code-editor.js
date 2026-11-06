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
import { convert } from "./content.js"

const LANGS = {
  html, css, javascript, markdown
}

// Milliseconds to wait after the last keystroke before re-rendering the preview.
const PREVIEW_DEBOUNCE = 150

// localStorage key for the preview pane's open/closed state.
const PREVIEW_STORAGE_KEY = "listmonk.markdownPreview"

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

function buildMarkdownToolbar(disabled, previewVisible) {
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

  const spacer = document.createElement("span")
  spacer.className = "md-toolbar-spacer"

  const toggle = document.createElement("button")
  toggle.type = "button"
  toggle.className = "ghost small md-tool md-preview-toggle"
  toggle.dataset.action = "toggle-preview"
  toggle.textContent = translate("globals.buttons.preview", "Preview")
  toggle.title = toggle.textContent
  toggle.setAttribute("aria-label", toggle.textContent)
  toggle.setAttribute("aria-pressed", String(previewVisible))
  toolbar.append(spacer, toggle)

  toolbar.addEventListener("mousedown", (e) => {
    if (e.target.closest("button")) {
      e.preventDefault()
    }
  })

  return toolbar
}

function buildMarkdownPreview() {
  const pane = document.createElement("div")
  pane.className = "md-preview-pane"

  const head = document.createElement("div")
  head.className = "md-preview-head"

  const label = document.createElement("span")
  label.textContent = translate("globals.buttons.preview", "Preview")

  const close = document.createElement("button")
  close.type = "button"
  close.className = "ghost small md-preview-close"
  close.dataset.action = "close-preview"
  close.textContent = "×"
  close.title = translate("globals.buttons.close", "Close")
  close.setAttribute("aria-label", close.title)

  const scroller = document.createElement("div")
  scroller.className = "md-preview"
  scroller.tabIndex = 0
  scroller.setAttribute("role", "region")
  scroller.setAttribute("aria-label", label.textContent)

  const body = document.createElement("div")
  body.className = "md-preview-body"
  scroller.append(body)

  head.append(label, close)
  pane.append(head, scroller)

  return {
    pane, scroller, body, close, toggle: null
  }
}

function readPreviewPreference() {
  try {
    return window.localStorage.getItem(PREVIEW_STORAGE_KEY) !== "false"
  } catch {
    return true
  }
}

function writePreviewPreference(visible) {
  try {
    window.localStorage.setItem(PREVIEW_STORAGE_KEY, String(visible))
    return true
  } catch {
    return false
  }
}

class CodeEditor extends HTMLElement {
  constructor() {
    super()
    this.editor = null
    this.textarea = null
    this.internalUpdate = false
    this.preview = null
    this.previewVisible = false
    this.previewTimer = null
    this.previewSyncing = false
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

    // Apply styles to the <code-editor> tag.
    this.classList.add("code-editor")

    const lang = this.getAttribute("lang")
    const isMarkdown = lang === "markdown"
    const langFn = LANGS[lang] || html
    const disabled = this.hasAttribute("disabled")
    const previewVisible = isMarkdown && readPreviewPreference()

    if (isMarkdown) {
      this.classList.add("is-markdown")
      this.preview = buildMarkdownPreview()
      this.previewVisible = previewVisible
      this.classList.toggle("is-preview-hidden", !previewVisible)

      const split = document.createElement("div")
      split.className = "md-split"
      split.append(mount, this.preview.pane)

      const toolbar = buildMarkdownToolbar(disabled, previewVisible)
      this.preview.toggle = toolbar.querySelector("[data-action='toggle-preview']")
      this.append(toolbar, split)
      this._bindToolbar(toolbar)
      this._bindPreview()
    } else {
      this.appendChild(mount)
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

      if (this.previewVisible) {
        this._schedulePreview()
      }
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

    if (isMarkdown) {
      this._bindScrollSync()
      this._renderPreview()
    }

    queueMicrotask(() => {
      this.dispatchEvent(new CustomEvent("editor-ready", { bubbles: true }))
    })
  }

  disconnectedCallback() {
    clearTimeout(this.previewTimer)
    this.previewTimer = null

    if (this.editor) {
      this.editor.destroy()
      this.editor = null
    }

    this.preview = null
  }

  _bindToolbar(toolbar) {
    toolbar.addEventListener("click", (e) => {
      const button = e.target.closest("button")
      if (!button) {
        return
      }

      if (button.dataset.action === "toggle-preview") {
        this._setPreviewVisible(!this.previewVisible)
        return
      }

      if (!button.dataset.cmd || !this.editor) {
        return
      }

      MARKDOWN_ACTIONS[button.dataset.cmd]?.(this.editor)
    })
  }

  _bindPreview() {
    this.preview.close.addEventListener("click", () => this._setPreviewVisible(false))
  }

  _bindScrollSync() {
    this.editor.scrollDOM.addEventListener("scroll", () => {
      this._syncScroll(this.editor.scrollDOM, this.preview.scroller)
    }, { passive: true })

    this.preview.scroller.addEventListener("scroll", () => {
      this._syncScroll(this.preview.scroller, this.editor.scrollDOM)
    }, { passive: true })
  }

  _syncScroll(from, to) {
    if (!this.previewVisible || this.previewSyncing) {
      return
    }

    const fromMax = from.scrollHeight - from.clientHeight
    const toMax = to.scrollHeight - to.clientHeight
    if (fromMax <= 0 || toMax <= 0) {
      return
    }

    this.previewSyncing = true
    to.scrollTop = (from.scrollTop / fromMax) * toMax
    requestAnimationFrame(() => {
      this.previewSyncing = false
    })
  }

  _setPreviewVisible(visible) {
    this.previewVisible = visible
    this.classList.toggle("is-preview-hidden", !visible)
    this.preview.toggle?.setAttribute("aria-pressed", String(visible))
    writePreviewPreference(visible)

    clearTimeout(this.previewTimer)
    if (!visible) {
      return
    }

    this._renderPreview()
    this._syncScroll(this.editor.scrollDOM, this.preview.scroller)
  }

  _schedulePreview() {
    clearTimeout(this.previewTimer)
    this.previewTimer = setTimeout(() => this._renderPreview(), PREVIEW_DEBOUNCE)
  }

  _renderPreview() {
    if (!this.editor || !this.preview) {
      return
    }

    const { body } = convert(this.editor.state.doc.toString(), "markdown", "html")
    this.preview.body.innerHTML = body
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
