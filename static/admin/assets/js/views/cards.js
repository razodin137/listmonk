import Alpine from "alpinejs"
import {
  api,
  urls,
  i18n,
  ListTag
} from "../main.js"
import * as u from "../utils.js"

// cardsView is the signup cards list page component.
function cardsView() {
  return {
    async onDeleteCard(id, name) {
      if (!(await u.confirm(i18n.t("cards.confirmDelete")))) {
        return
      }

      await api("cards", `/cards/${id}`, "DELETE")
      u.reload({ message: i18n.ts("globals.messages.deleted", { name }) })
    },

    async onDuplicateCard(id) {
      const data = await api("cards", `/cards/${id}/duplicate`, "POST")
      u.reload({ message: i18n.ts("globals.messages.created", { name: data.name }) })
    }
  }
}

// cardForm is the signup card add/edit form page component.
function cardForm({ isNew = true } = {}) {
  const card = window._card || {}

  // Init a fresh form payload.
  const makeForm = () => ({
    id: null,
    uuid: "",
    name: "",
    slug: "",
    description: "",
    success_message: "",
    redirect_url: "",
    captcha: "inherit",
    lists: []
  })

  const fromCard = (c) => ({
    ...makeForm(),
    ...c,
    lists: (Array.isArray(c.lists) ? c.lists : []).map((l) => new ListTag(l))
  })

  return {
    form: isNew ? makeForm() : fromCard(card),

    // ===============
    // Event handlers.
    async onSubmitNew() {
      const data = await api("cards.save", "/cards", "POST", this._payload())
      u.redirect(`${urls.admin}/cards/${data.id}`, { message: i18n.ts("globals.messages.created", { name: data.name }) })
    },

    async onSubmitUpdate() {
      const data = await api("cards.save", `/cards/${this.form.id}`, "PUT", this._payload())
      this.form = fromCard(data)
      u.reload({ message: i18n.ts("globals.messages.updated", { name: data.name }) })
    },

    // ===============
    // Private functions.
    _payload() {
      return {
        name: this.form.name,
        slug: this.form.slug,
        description: this.form.description || "",
        success_message: this.form.success_message || "",
        redirect_url: this.form.redirect_url || "",
        captcha: this.form.captcha || "inherit",
        list_ids: this.form.lists.map((l) => l.id)
      }
    }
  }
}

document.addEventListener("alpine:init", () => {
  Alpine.data("cardsView", cardsView)
  Alpine.data("cardForm", cardForm)
}, { once: true })
