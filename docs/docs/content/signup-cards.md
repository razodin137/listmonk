# Signup cards

A signup card is a named public signup form with its own slug and address. It
adds everyone who uses it to a fixed set of labels. The labels remain ordinary
lists underneath; a card is just the thing you hand out.

Create, edit, duplicate and delete cards under **Signup cards** in the admin
sidebar. Each card row shows its slug, its labels and how many people joined
through it, split into new subscribers and existing members who gained labels.
A card can also carry its own description, success message, redirect URL and
captcha setting.

## Labels

A label is a list that a subscriber belongs to. A card pins one or more public
lists, and every signup through the card adds exactly those labels. A visitor
never sees or chooses them, so the form has no list picker or checkboxes.

Only public lists can be pinned to a card. If a label on an existing card is
later made private, the card refuses submissions instead of enrolling anyone.

## Three ways to hand out a card

Every card has its own slug, such as `autumn-newsletter`. Each of these uses
that slug.

### Hosted page

`/signup/<slug>` renders the card on its own, using the same public styles as
the rest of the site. It shows no list picker and no other card.

The public subscription page must be enabled in settings for the card pages to
be available.

### Embed snippet

The card's admin page shows an iframe snippet with a copy button. Paste it into
any page:

```html
<iframe src="https://list.example.com/signup/autumn-newsletter/embed"
  title="Autumn newsletter" width="100%" height="400"
  style="border:0;max-width:100%;" loading="lazy"></iframe>
```

The snippet points at `/signup/<slug>/embed`, which renders the bare form with
no page chrome. The iframe boundary keeps the host page's styles from leaking
in. The height is calculated to fit the card's description and captcha, so the
form is not cut off.

### Standalone HTML file

The card's admin page has an **Export HTML** button, and
`/signup/<slug>/export` downloads the same file. It is a single self-contained
page with the public styles inlined and the card's labels baked in. The form
posts back to this listmonk install, so the file can be dropped on any static
site and still subscribe people.

## What happens to an existing member

The email address decides. A new address is subscribed as usual, including any
double opt-in confirmation the labels require.

If the address already belongs to a subscriber, the card only adds its labels.
Their name, other labels and status are left alone, and the response is a
success rather than a duplicate-email error.

Three cases follow from the member's current state:

- If they are enabled and already confirmed on every label the card adds, they
  are subscribed immediately. No email is sent and no confirmation screen is
  shown.
- If any card label needs confirmation and they have not confirmed it, one
  opt-in email is sent for those labels only. The success screen says the
  confirmation is pending.

A card join is counted when the subscriber is new, or when an existing member
gains at least one of the card's labels. After a successful signup the card
shows its success message, or redirects to its redirect URL if one is set.
